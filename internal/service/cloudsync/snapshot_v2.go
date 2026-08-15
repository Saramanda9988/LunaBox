package cloudsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/service/cloudprovider/batchupload"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrManifestNotFound 在远端未找到 manifest.json 时返回（区别于其他网络错误，便于上层走迁移分支）。
var ErrManifestNotFound = errors.New("cloud sync manifest not found")

// ErrManifestSchemaTooNew 表示远端 manifest 的 schema_version 比当前客户端能识别的版本更新。
// 触发条件：未来 v3+ 客户端写入新格式后，当前 v2 客户端读到 → 必须停止同步并提示用户升级，
// 否则可能误覆盖远端高版本数据。
var ErrManifestSchemaTooNew = errors.New("cloud sync manifest schema is newer than this client supports, please update LunaBox")

// EnsureSyncDirs 在 SyncNow 启动时一次性确保所有 v2 子目录存在。
// 这样后续上传桶不需要重复调用 EnsureDir（OneDrive 路径成本最大化收敛）。
// S3 的 EnsureDir 是 no-op，调用无副作用。
func (h *Helper) EnsureSyncDirs(provider cloudprovider.CloudStorageProvider) error {
	libraryPath := provider.GetCloudPath(h.config.BackupUserID, LibraryDir)
	if err := provider.EnsureDir(h.ctx, libraryPath); err != nil {
		return fmt.Errorf("ensure library dir: %w", err)
	}
	for _, sub := range EntitySubDirs {
		dirPath := provider.GetCloudPath(h.config.BackupUserID, filepath.ToSlash(filepath.Join(LibraryDir, sub)))
		if err := provider.EnsureDir(h.ctx, dirPath); err != nil {
			return fmt.Errorf("ensure %s dir: %w", sub, err)
		}
	}
	coverPath := provider.GetCloudPath(h.config.BackupUserID, CoverDir)
	if err := provider.EnsureDir(h.ctx, coverPath); err != nil {
		return fmt.Errorf("ensure cover dir: %w", err)
	}
	return nil
}

// LoadRemoteManifest 下载并解析 sync/library/manifest.json。
// 未找到时返回 (zero, false, nil)；高版本 schema 时返回 ErrManifestSchemaTooNew。
func (h *Helper) LoadRemoteManifest(provider cloudprovider.CloudStorageProvider) (Manifest, bool, error) {
	key := provider.GetCloudPath(h.config.BackupUserID, ManifestKey)
	raw, exists, err := h.downloadToBytes(provider, key)
	if err != nil {
		return Manifest{}, false, fmt.Errorf("download manifest: %w", err)
	}
	if !exists {
		return Manifest{}, false, nil
	}
	m, err := DecodeManifest(raw)
	if err != nil {
		return Manifest{}, false, err
	}
	if m.SchemaVersion > SchemaVersion {
		return Manifest{}, true, ErrManifestSchemaTooNew
	}
	return m, true, nil
}

// LoadRemoteBuckets 并发下载 toPull 列表中的桶文件。
// 返回 map[entityKey][bucketChar]*BucketContent；未在 toPull 中的桶不存在于返回 map。
func (h *Helper) LoadRemoteBuckets(provider cloudprovider.CloudStorageProvider, bucketKeys []string) (map[string]map[string]*BucketContent, error) {
	if len(bucketKeys) == 0 {
		return map[string]map[string]*BucketContent{}, nil
	}

	out := make(map[string]map[string]*BucketContent, len(EntityKeys()))
	for _, entityKey := range EntityKeys() {
		out[entityKey] = make(map[string]*BucketContent)
	}
	var mu sync.Mutex

	err := runConcurrent(h.ctx, bucketKeys, ConcurrencyFor(provider), func(ctx context.Context, key string) error {
		entity, ch, ok := splitBucketKey(key)
		if !ok {
			return fmt.Errorf("invalid bucket key %q", key)
		}
		subDir, ok := EntitySubDirs[entity]
		if !ok {
			return fmt.Errorf("unknown entity for bucket key %q", key)
		}
		cloudKey := provider.GetCloudPath(h.config.BackupUserID, filepath.ToSlash(filepath.Join(LibraryDir, subDir, ch+".json")))
		raw, exists, err := h.downloadToBytesCtx(ctx, provider, cloudKey)
		if err != nil {
			return fmt.Errorf("download bucket %s: %w", key, err)
		}
		if !exists {
			// 远端 manifest 引用了它但实际文件缺失：当作空桶处理（merge 会按 LWW 自然处理）
			mu.Lock()
			out[entity][ch] = &BucketContent{}
			mu.Unlock()
			return nil
		}
		_, _, bc, err := UnmarshalBucketFile(raw)
		if err != nil {
			return fmt.Errorf("decode bucket %s: %w", key, err)
		}
		mu.Lock()
		out[entity][ch] = &bc
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LoadRemoteSingletons 下载指定的单文件（"categories" / "tombstones"）。
// 返回值的 key 是 singleton 名，value 是已解析的内容。
func (h *Helper) LoadRemoteSingletons(provider cloudprovider.CloudStorageProvider, names []string) (categories []Category, tombstones []Tombstone, fetched map[string]bool, err error) {
	fetched = make(map[string]bool, len(names))
	for _, name := range names {
		key, ok := singletonCloudKey(name)
		if !ok {
			return nil, nil, nil, fmt.Errorf("unknown singleton: %s", name)
		}
		cloudKey := provider.GetCloudPath(h.config.BackupUserID, key)
		raw, exists, dErr := h.downloadToBytes(provider, cloudKey)
		if dErr != nil {
			return nil, nil, nil, fmt.Errorf("download singleton %s: %w", name, dErr)
		}
		if !exists {
			fetched[name] = false
			continue
		}
		fetched[name] = true
		var file BucketFile
		if uErr := json.Unmarshal(raw, &file); uErr != nil {
			return nil, nil, nil, fmt.Errorf("decode singleton %s: %w", name, uErr)
		}
		switch name {
		case SingletonCategories:
			categories = file.Categories
		case SingletonTombstones:
			tombstones = file.Tombstones
		}
	}
	return categories, tombstones, fetched, nil
}

// SaveRemoteBuckets 并发上传 toPush 列表中的桶。
// 调用方需要保证 buckets 中已经包含 toPush 所有桶的最新内容。
func (h *Helper) SaveRemoteBuckets(provider cloudprovider.CloudStorageProvider, buckets map[string]map[string]*BucketContent, bucketKeys []string) error {
	return h.SaveRemoteLibraryFiles(provider, buckets, bucketKeys, nil, nil, nil)
}

// SaveRemoteSingletons uploads the selected categories/tombstones files.
func (h *Helper) SaveRemoteSingletons(provider cloudprovider.CloudStorageProvider, categories []Category, tombstones []Tombstone, names []string) error {
	return h.SaveRemoteLibraryFiles(provider, nil, nil, categories, tombstones, names)
}

// SaveRemoteLibraryFiles materializes buckets and singletons together so a
// batch-capable provider can upload them in as few control-plane requests as
// possible. The manifest remains a separate, final commit point.
func (h *Helper) SaveRemoteLibraryFiles(
	provider cloudprovider.CloudStorageProvider,
	buckets map[string]map[string]*BucketContent,
	bucketKeys []string,
	categories []Category,
	tombstones []Tombstone,
	singletonNames []string,
) error {
	if len(bucketKeys) == 0 && len(singletonNames) == 0 {
		return nil
	}

	items := make([]batchupload.Item, 0, len(bucketKeys)+len(singletonNames))
	tempPaths := make([]string, 0, cap(items))
	defer func() {
		for _, tempPath := range tempPaths {
			_ = os.Remove(tempPath)
		}
	}()
	addPayload := func(cloudKey string, payload []byte) error {
		tempPath, err := writeUploadTempFile(payload)
		if err != nil {
			return err
		}
		tempPaths = append(tempPaths, tempPath)
		items = append(items, batchupload.Item{CloudPath: cloudKey, LocalPath: tempPath})
		return nil
	}

	for _, key := range bucketKeys {
		entity, ch, ok := splitBucketKey(key)
		if !ok {
			return fmt.Errorf("invalid bucket key %q", key)
		}
		subDir, ok := EntitySubDirs[entity]
		if !ok {
			return fmt.Errorf("unknown entity for bucket key %q", key)
		}
		bc := buckets[entity][ch]
		payload, err := MarshalBucketFile(entity, ch, bc)
		if err != nil {
			return fmt.Errorf("marshal bucket %s: %w", key, err)
		}
		if int64(len(payload)) > BucketSizeWarnBytes {
			applog.LogWarningf(h.ctx, "CloudSync: bucket %s payload is %d bytes (> %d); consider re-bucketing in future versions", key, len(payload), BucketSizeWarnBytes)
		}
		cloudKey := provider.GetCloudPath(h.config.BackupUserID, filepath.ToSlash(filepath.Join(LibraryDir, subDir, ch+".json")))
		if err := addPayload(cloudKey, payload); err != nil {
			return fmt.Errorf("prepare bucket %s upload: %w", key, err)
		}
	}

	for _, name := range singletonNames {
		key, ok := singletonCloudKey(name)
		if !ok {
			return fmt.Errorf("unknown singleton: %s", name)
		}
		file := BucketFile{
			SchemaVersion: SchemaVersion,
			BucketKey:     "_singleton/" + name,
		}
		switch name {
		case SingletonCategories:
			file.Categories = categories
		case SingletonTombstones:
			file.Tombstones = tombstones
		}
		payload, err := json.MarshalIndent(file, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal singleton %s: %w", name, err)
		}
		cloudKey := provider.GetCloudPath(h.config.BackupUserID, key)
		if err := addPayload(cloudKey, payload); err != nil {
			return fmt.Errorf("prepare singleton %s upload: %w", name, err)
		}
	}

	startedAt := time.Now()
	applog.LogInfof(h.ctx, "CloudSync: library upload started provider=%T buckets=%d singletons=%d concurrency=%d", provider, len(bucketKeys), len(singletonNames), ConcurrencyFor(provider))
	if err := h.uploadFileItems(provider, items); err != nil {
		applog.LogWarningf(h.ctx, "CloudSync: library upload failed provider=%T buckets=%d singletons=%d failed=1 elapsed=%s: %v", provider, len(bucketKeys), len(singletonNames), time.Since(startedAt), err)
		return err
	}
	applog.LogInfof(h.ctx, "CloudSync: library upload finished provider=%T buckets=%d singletons=%d failed=0 elapsed=%s", provider, len(bucketKeys), len(singletonNames), time.Since(startedAt))
	return nil
}

// SaveRemoteManifest 写入 sync/library/manifest.json。
// 必须严格在所有桶/单文件上传成功后才调用，否则远端处于半新半旧状态。
func (h *Helper) SaveRemoteManifest(provider cloudprovider.CloudStorageProvider, manifest Manifest) error {
	payload, err := EncodeManifest(manifest)
	if err != nil {
		return err
	}
	cloudKey := provider.GetCloudPath(h.config.BackupUserID, ManifestKey)
	if err := h.uploadBytes(provider, cloudKey, payload); err != nil {
		return fmt.Errorf("upload manifest: %w", err)
	}
	return nil
}

// CleanOrphans 列出 v2 子目录里的全部文件，与 manifest 引用集合做 diff，删除不在引用集中的孤儿文件。
// 目前 SyncNow 默认不调用这个函数（见 design.md Open Question #4），仅作为可扩展入口预留。
func (h *Helper) CleanOrphans(provider cloudprovider.CloudStorageProvider, manifest Manifest) error {
	expected := make(map[string]struct{})
	expected[provider.GetCloudPath(h.config.BackupUserID, ManifestKey)] = struct{}{}
	expected[provider.GetCloudPath(h.config.BackupUserID, CategoriesFileKey)] = struct{}{}
	expected[provider.GetCloudPath(h.config.BackupUserID, TombstonesFileKey)] = struct{}{}
	for entity, sub := range EntitySubDirs {
		for ch := range manifest.Buckets[entity] {
			key := provider.GetCloudPath(h.config.BackupUserID, filepath.ToSlash(filepath.Join(LibraryDir, sub, ch+".json")))
			expected[key] = struct{}{}
		}
	}

	// 列出每个子目录与 library/ 自身
	libraryPath := provider.GetCloudPath(h.config.BackupUserID, LibraryDir)
	rootKeys, err := provider.ListObjects(h.ctx, libraryPath)
	if err != nil {
		return fmt.Errorf("list library dir: %w", err)
	}
	allKeys := append([]string{}, rootKeys...)
	for _, sub := range EntitySubDirs {
		dir := provider.GetCloudPath(h.config.BackupUserID, filepath.ToSlash(filepath.Join(LibraryDir, sub)))
		subKeys, err := provider.ListObjects(h.ctx, dir)
		if err != nil {
			return fmt.Errorf("list %s dir: %w", sub, err)
		}
		allKeys = append(allKeys, subKeys...)
	}

	for _, key := range allKeys {
		if _, ok := expected[key]; ok {
			continue
		}
		// 旧 v1 latest.json 由迁移路径单独删除，这里不动；其他子目录/不认识的文件视为 orphan
		if strings.HasSuffix(key, "/latest.json") {
			continue
		}
		// 跳过明显的目录项（OneDrive children 可能返回子目录路径本身）
		if !strings.HasSuffix(key, ".json") {
			continue
		}
		if err := provider.DeleteObject(h.ctx, key); err != nil {
			applog.LogWarningf(h.ctx, "CloudSync: failed to delete orphan %s: %v", key, err)
		}
	}
	return nil
}

// ---- private helpers ----

func singletonCloudKey(name string) (string, bool) {
	switch name {
	case SingletonCategories:
		return CategoriesFileKey, true
	case SingletonTombstones:
		return TombstonesFileKey, true
	}
	return "", false
}

func splitBucketKey(key string) (entity, ch string, ok bool) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// downloadToBytes 把远端文件下载到临时文件再读回字节；不存在时返回 (nil, false, nil)。
// 不存在的判定基于文件下载报错（两个 provider 都不暴露干净的 NotFound，只能根据错误文本兜底）。
func (h *Helper) downloadToBytes(provider cloudprovider.CloudStorageProvider, cloudKey string) ([]byte, bool, error) {
	return h.downloadToBytesCtx(h.ctx, provider, cloudKey)
}

func (h *Helper) downloadToBytesCtx(ctx context.Context, provider cloudprovider.CloudStorageProvider, cloudKey string) ([]byte, bool, error) {
	tempFile, err := os.CreateTemp("", "lunabox_cloud_v2_*.json")
	if err != nil {
		return nil, false, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempPath)

	// provider 接口没有 ctx 版本的 download；用 h.ctx（外层 SyncNow 持有），不影响超时
	_ = ctx
	if err := provider.DownloadFile(h.ctx, cloudKey, tempPath); err != nil {
		if isNotFoundErr(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	raw, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, false, fmt.Errorf("read temp file: %w", err)
	}
	return raw, true, nil
}

func (h *Helper) uploadBytes(provider cloudprovider.CloudStorageProvider, cloudKey string, payload []byte) error {
	return h.uploadBytesCtx(h.ctx, provider, cloudKey, payload)
}

func (h *Helper) uploadBytesCtx(ctx context.Context, provider cloudprovider.CloudStorageProvider, cloudKey string, payload []byte) error {
	tempFile, err := os.CreateTemp("", "lunabox_cloud_v2_upload_*.json")
	if err != nil {
		return fmt.Errorf("create upload temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(payload); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write upload temp file: %w", err)
	}
	tempFile.Close()
	defer os.Remove(tempPath)

	_ = ctx
	if err := provider.UploadFile(h.ctx, cloudKey, tempPath); err != nil {
		return err
	}
	return nil
}

func (h *Helper) uploadFileItems(provider cloudprovider.CloudStorageProvider, items []batchupload.Item) error {
	if len(items) == 0 {
		return nil
	}
	if batchProvider, ok := provider.(cloudprovider.BatchUploadProvider); ok {
		return batchProvider.UploadFiles(h.ctx, items)
	}
	return runConcurrent(h.ctx, items, ConcurrencyFor(provider), func(ctx context.Context, item batchupload.Item) error {
		if err := provider.UploadFile(ctx, item.CloudPath, item.LocalPath); err != nil {
			return fmt.Errorf("upload %s: %w", item.CloudPath, err)
		}
		return nil
	})
}

func writeUploadTempFile(payload []byte) (string, error) {
	tempFile, err := os.CreateTemp("", "lunabox_cloud_v2_upload_*.json")
	if err != nil {
		return "", fmt.Errorf("create upload temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(payload); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("write upload temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close upload temp file: %w", err)
	}
	return tempPath, nil
}

// isNotFoundErr 用 substring 兜底匹配 provider 报错文本中的 NotFound / 404 字样。
// 现有 provider 接口没有暴露干净的 NotFound 错误，只能这样判断；后续如有重构可改为类型断言。
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "notfound") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "404") ||
		strings.Contains(msg, "nosuchkey")
}
