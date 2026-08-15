package umbra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/service/cloudprovider/batchupload"
)

const (
	syncSpaceName       = "library"
	syncNamespace       = "lunabox.library"
	syncRootCollection  = "root"
	syncSchemaVersion   = 1
	syncPageLimit       = 500
	syncMutationLimit   = 500
	syncPayloadLimit    = 256 * 1024
	syncBatchTargetSize = 3_500_000
)

var syncKeyPartPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type syncSnapshot struct {
	records map[umbrsdk.SyncRecordKey]umbrsdk.SyncChange
	cursor  string
}

type syncUpload struct {
	cloudPath string
	key       umbrsdk.SyncRecordKey
	payload   json.RawMessage
}

func isSyncLibrarySubPath(subPath string) bool {
	clean := strings.Trim(strings.ReplaceAll(subPath, "\\", "/"), "/")
	return clean == "sync/library" || strings.HasPrefix(clean, "sync/library/")
}

func (p *Provider) syncKeyForCloudPath(cloudPath string) (umbrsdk.SyncRecordKey, bool, error) {
	subPath, err := p.subPath(cloudPath)
	if err != nil {
		return umbrsdk.SyncRecordKey{}, false, err
	}
	return syncKeyForSubPath(subPath)
}

func syncKeyForSubPath(subPath string) (umbrsdk.SyncRecordKey, bool, error) {
	clean, err := normalizeSubPath(subPath)
	if err != nil {
		return umbrsdk.SyncRecordKey{}, false, err
	}
	if !isSyncLibrarySubPath(clean) {
		return umbrsdk.SyncRecordKey{}, false, nil
	}
	parts := strings.Split(clean, "/")
	collection := syncRootCollection
	fileName := ""
	switch len(parts) {
	case 3:
		fileName = parts[2]
	case 4:
		collection = parts[2]
		fileName = parts[3]
	default:
		return umbrsdk.SyncRecordKey{}, false, fmt.Errorf("Umbra 不支持的同步路径: %s", subPath)
	}
	if !strings.HasSuffix(fileName, ".json") {
		return umbrsdk.SyncRecordKey{}, false, fmt.Errorf("Umbra 同步路径必须是 JSON 文件: %s", subPath)
	}
	recordID := strings.TrimSuffix(fileName, ".json")
	if !syncKeyPartPattern.MatchString(collection) || !syncKeyPartPattern.MatchString(recordID) || len(collection) > 64 {
		return umbrsdk.SyncRecordKey{}, false, fmt.Errorf("Umbra 同步路径标识无效: %s", subPath)
	}
	return umbrsdk.SyncRecordKey{Namespace: syncNamespace, Collection: collection, RecordID: recordID}, true, nil
}

func subPathForSyncKey(key umbrsdk.SyncRecordKey) (string, bool) {
	if key.Namespace != syncNamespace || key.RecordID == "" {
		return "", false
	}
	if key.Collection == syncRootCollection {
		return "sync/library/" + key.RecordID + ".json", true
	}
	if key.Collection == "" {
		return "", false
	}
	return "sync/library/" + key.Collection + "/" + key.RecordID + ".json", true
}

func (p *Provider) loadSyncSnapshot(ctx context.Context) (syncSnapshot, error) {
	result := syncSnapshot{records: make(map[umbrsdk.SyncRecordKey]umbrsdk.SyncChange)}
	cursor := ""
	for {
		page, err := p.client.Sync.Snapshot(ctx, umbrsdk.SyncSnapshotInput{
			SpaceName: syncSpaceName,
			Cursor:    cursor,
			Limit:     syncPageLimit,
		})
		if err != nil {
			return syncSnapshot{}, fmt.Errorf("Umbra 读取同步快照失败: %w", err)
		}
		for _, record := range page.Records {
			if record.Key.Namespace == syncNamespace {
				result.records[record.Key] = record
			}
		}
		result.cursor = page.ExchangeCursor
		if !page.HasMore {
			return result, nil
		}
		if page.NextCursor == "" || page.NextCursor == cursor {
			return syncSnapshot{}, fmt.Errorf("Umbra 同步快照分页游标无效")
		}
		cursor = page.NextCursor
	}
}

func (p *Provider) uploadSyncFile(ctx context.Context, key umbrsdk.SyncRecordKey, localPath string) error {
	upload, err := readSyncUpload(key, "", localPath)
	if err != nil {
		return err
	}
	snapshot, err := p.loadSyncSnapshot(ctx)
	if err != nil {
		return err
	}
	return p.exchangeSyncUploads(ctx, snapshot, []syncUpload{upload})
}

func (p *Provider) downloadSyncFile(ctx context.Context, key umbrsdk.SyncRecordKey, localPath string) error {
	snapshot, err := p.loadSyncSnapshot(ctx)
	if err != nil {
		return err
	}
	record, ok := snapshot.records[key]
	if !ok || record.Operation == umbrsdk.SyncOperationDelete {
		return fmt.Errorf("Umbra 同步文件 not found: %s/%s", key.Collection, key.RecordID)
	}
	if !json.Valid(record.Payload) {
		return fmt.Errorf("Umbra 同步文件内容无效: %s/%s", key.Collection, key.RecordID)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("创建 Umbra 下载目录失败: %w", err)
	}
	if err := os.WriteFile(localPath, record.Payload, 0o600); err != nil {
		return fmt.Errorf("写入 Umbra 同步文件失败: %w", err)
	}
	return nil
}

func (p *Provider) listSyncObjects(ctx context.Context, subPath string) ([]string, error) {
	snapshot, err := p.loadSyncSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	prefix := strings.Trim(strings.ReplaceAll(subPath, "\\", "/"), "/") + "/"
	keys := make([]string, 0, len(snapshot.records))
	for _, record := range snapshot.records {
		if record.Operation == umbrsdk.SyncOperationDelete {
			continue
		}
		recordSubPath, ok := subPathForSyncKey(record.Key)
		if !ok || !strings.HasPrefix(recordSubPath, prefix) {
			continue
		}
		keys = append(keys, p.cloudPathForSubPath(recordSubPath))
	}
	sort.Strings(keys)
	return keys, nil
}

func (p *Provider) deleteSyncObject(ctx context.Context, key umbrsdk.SyncRecordKey) error {
	snapshot, err := p.loadSyncSnapshot(ctx)
	if err != nil {
		return err
	}
	current, ok := snapshot.records[key]
	if !ok || current.Operation == umbrsdk.SyncOperationDelete {
		return nil
	}
	mutation := umbrsdk.NewDeleteMutation(syncMutationID(key, current.RecordVersion, nil, umbrsdk.SyncOperationDelete), key, syncSchemaVersion, current.RecordVersion)
	return p.exchangeSyncMutations(ctx, snapshot.cursor, []umbrsdk.SyncMutation{mutation})
}

func (p *Provider) UploadFiles(ctx context.Context, items []batchupload.Item) error {
	if len(items) == 0 {
		return nil
	}
	syncUploads := make([]syncUpload, 0, len(items))
	backupItems := make([]batchupload.Item, 0, len(items))
	seen := make(map[umbrsdk.SyncRecordKey]struct{})
	for _, item := range items {
		key, ok, err := p.syncKeyForCloudPath(item.CloudPath)
		if err != nil {
			return err
		}
		if !ok {
			backupItems = append(backupItems, item)
			continue
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("Umbra 批量同步包含重复路径: %s", item.CloudPath)
		}
		seen[key] = struct{}{}
		upload, err := readSyncUpload(key, item.CloudPath, item.LocalPath)
		if err != nil {
			return err
		}
		syncUploads = append(syncUploads, upload)
	}
	if err := p.uploadBackupFiles(ctx, backupItems); err != nil {
		return err
	}
	if len(syncUploads) == 0 {
		return nil
	}
	snapshot, err := p.loadSyncSnapshot(ctx)
	if err != nil {
		return err
	}
	return p.exchangeSyncUploads(ctx, snapshot, syncUploads)
}

func readSyncUpload(key umbrsdk.SyncRecordKey, cloudPath, localPath string) (syncUpload, error) {
	payload, err := os.ReadFile(localPath)
	if err != nil {
		return syncUpload{}, fmt.Errorf("读取 Umbra 同步文件 %s 失败: %w", cloudPath, err)
	}
	if !json.Valid(payload) {
		return syncUpload{}, fmt.Errorf("Umbra 同步文件不是有效 JSON: %s", cloudPath)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil {
		return syncUpload{}, fmt.Errorf("压缩 Umbra 同步 JSON 失败: %s: %w", cloudPath, err)
	}
	payload = compact.Bytes()
	if len(payload) > syncPayloadLimit {
		return syncUpload{}, fmt.Errorf("Umbra 同步文件超过 256 KiB 限制: %s (%d bytes)", cloudPath, len(payload))
	}
	return syncUpload{cloudPath: cloudPath, key: key, payload: json.RawMessage(payload)}, nil
}

func (p *Provider) exchangeSyncUploads(ctx context.Context, snapshot syncSnapshot, uploads []syncUpload) error {
	cursor := snapshot.cursor
	mutations := make([]umbrsdk.SyncMutation, 0, min(len(uploads), syncMutationLimit))
	payloadBytes := 0
	flush := func() error {
		if len(mutations) == 0 {
			return nil
		}
		if err := p.exchangeSyncMutations(ctx, cursor, mutations); err != nil {
			return err
		}
		mutations = mutations[:0]
		payloadBytes = 0
		return nil
	}
	for _, upload := range uploads {
		if len(mutations) == syncMutationLimit || payloadBytes+len(upload.payload) > syncBatchTargetSize {
			if err := flush(); err != nil {
				return err
			}
		}
		baseVersion := uint64(0)
		if current, ok := snapshot.records[upload.key]; ok {
			baseVersion = current.RecordVersion
		}
		mutations = append(mutations, umbrsdk.SyncMutation{
			MutationID:    syncMutationID(upload.key, baseVersion, upload.payload, umbrsdk.SyncOperationUpsert),
			Key:           upload.key,
			SchemaVersion: syncSchemaVersion,
			BaseVersion:   baseVersion,
			Operation:     umbrsdk.SyncOperationUpsert,
			Payload:       upload.payload,
		})
		payloadBytes += len(upload.payload)
	}
	return flush()
}

func (p *Provider) exchangeSyncMutations(ctx context.Context, cursor string, mutations []umbrsdk.SyncMutation) error {
	result, err := p.client.Sync.Exchange(ctx, umbrsdk.SyncExchangeInput{
		Space:     umbrsdk.SyncSpace{Name: syncSpaceName},
		Cursor:    cursor,
		Mutations: mutations,
		PullLimit: syncPageLimit,
	})
	if err != nil {
		return fmt.Errorf("Umbra 同步交换失败: %w", err)
	}
	if result.ResetRequired {
		return fmt.Errorf("Umbra 同步游标需要重置: %s", result.Reason)
	}
	if len(result.Conflicts) > 0 {
		return fmt.Errorf("Umbra 同步冲突 (%s): %s", result.Conflicts[0].MutationID, result.Conflicts[0].Reason)
	}
	if len(result.Rejected) > 0 {
		return fmt.Errorf("Umbra 同步请求被拒绝 (%s): %s", result.Rejected[0].MutationID, result.Rejected[0].Reason)
	}
	if len(result.Accepted) != len(mutations) {
		return fmt.Errorf("Umbra 同步接受 %d 项，期望 %d 项", len(result.Accepted), len(mutations))
	}
	return nil
}

func syncMutationID(key umbrsdk.SyncRecordKey, baseVersion uint64, payload []byte, operation umbrsdk.SyncOperation) string {
	hasher := sha256.New()
	_, _ = fmt.Fprintf(hasher, "%s\x00%s\x00%s\x00%d\x00%s\x00", key.Namespace, key.Collection, key.RecordID, baseVersion, operation)
	_, _ = hasher.Write(payload)
	return "lunabox-" + hex.EncodeToString(hasher.Sum(nil))[:40]
}
