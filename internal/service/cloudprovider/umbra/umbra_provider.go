package umbra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	umbraSDK "github.com/Umbrae-Labs/umbra-core/sdk/umbra-go"
)

const (
	DefaultRedirectURI = "http://127.0.0.1:1420/auth/callback"
	DefaultScope       = "openid offline_access"

	pathPrefixDB     = "db/"
	pathPrefixFull   = "full/"
	pathPrefixGame   = "game/"
	pathPrefixAsset  = "asset/"
	legacyPrefixDB   = "database/"
	legacyPrefixGame = "saves/"

	subjectSyncLibrary = "lunabox_sync_library"
	subjectSyncCovers  = "lunabox_sync_covers"
)

type UmbraConfig struct {
	BaseURL               string
	APIBaseURL            string
	AuthorizationEndpoint string
	TokenEndpoint         string
	RevocationEndpoint    string
	ClientID              string
	RedirectURI           string
	Scope                 string
}

type UmbraProvider struct {
	client *umbraSDK.Client
	store  *configTokenStore
}

func NewUmbraProvider(cfg UmbraConfig, appConfig *appconf.AppConfig, openURL func(string) error) (*UmbraProvider, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("Umbra Base URL 未配置")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, fmt.Errorf("Umbra Client ID 未配置")
	}

	store, err := newConfigTokenStore(appConfig)
	if err != nil {
		return nil, fmt.Errorf("解析 Umbra token 失败: %w", err)
	}

	redirectURI := strings.TrimSpace(cfg.RedirectURI)
	if redirectURI == "" {
		redirectURI = DefaultRedirectURI
	}
	scope := strings.TrimSpace(cfg.Scope)
	if scope == "" {
		scope = DefaultScope
	}

	client, err := umbraSDK.New(umbraSDK.Config{
		BaseURL:               strings.TrimSpace(cfg.BaseURL),
		APIBaseURL:            strings.TrimSpace(cfg.APIBaseURL),
		AuthorizationEndpoint: strings.TrimSpace(cfg.AuthorizationEndpoint),
		TokenEndpoint:         strings.TrimSpace(cfg.TokenEndpoint),
		RevocationEndpoint:    strings.TrimSpace(cfg.RevocationEndpoint),
		ClientID:              strings.TrimSpace(cfg.ClientID),
		RedirectURI:           redirectURI,
		Scope:                 scope,
		TokenStore:            store,
		BrowserOpener:         browserOpenerFunc(openURL),
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 Umbra 客户端失败: %w", err)
	}

	return &UmbraProvider{
		client: client,
		store:  store,
	}, nil
}

func (p *UmbraProvider) UploadFile(ctx context.Context, cloudPath, localPath string) error {
	address, err := parseCloudPath(cloudPath)
	if err != nil {
		return err
	}

	_, err = p.uploadFileWithDiagnostics(ctx, address, localPath)
	if err != nil {
		if allowOverwriteRetry(err) {
			if _, deleteErr := p.client.Backup.Delete(ctx, umbraSDK.BackupTarget{Address: address}); deleteErr == nil {
				_, err = p.uploadFileWithDiagnostics(ctx, address, localPath)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("上传 Umbra 备份失败: %w", err)
	}
	return nil
}

func (p *UmbraProvider) uploadFileWithDiagnostics(ctx context.Context, address umbraSDK.BackupAddress, localPath string) (*umbraSDK.UploadResult, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < 0 {
		return nil, fmt.Errorf("无效的 Umbra 上传文件大小")
	}

	contentType := contentTypeForPath(localPath)
	contentHash, err := hashOpenFile(file)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	applog.LogInfof(ctx, "Umbra upload presign start: category=%s subject=%s version=%s local_path=%s size=%d content_type=%s sha256=%s",
		address.Category, address.Subject, address.Version, localPath, info.Size(), contentType, contentHash)

	presign, err := p.client.Backup.PresignUpload(ctx, umbraSDK.PresignUploadInput{
		Address:     address,
		FileSize:    uint64(info.Size()),
		ContentType: contentType,
		ContentHash: contentHash,
	})
	if err != nil {
		return nil, err
	}

	applog.LogInfof(ctx, "Umbra upload presign success: backup_id=%d expires_in=%d url=%s",
		presign.BackupID, presign.ExpiresIn, sanitizePresignedURL(presign.PresignedURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presign.PresignedURL, file)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = info.Size()

	applog.LogInfof(ctx, "Umbra object PUT start: backup_id=%d url=%s content_length=%d content_type=%s",
		presign.BackupID, sanitizePresignedURL(presign.PresignedURL), req.ContentLength, contentType)

	res, err := p.client.HTTPClient().Do(req)
	if err != nil {
		applog.LogErrorf(ctx, "Umbra object PUT network failed: backup_id=%d url=%s error=%v",
			presign.BackupID, sanitizePresignedURL(presign.PresignedURL), err)
		return nil, &umbraSDK.UmbraError{Kind: umbraSDK.ErrNetwork, Message: err.Error(), Cause: err}
	}
	defer res.Body.Close()

	bodySnippet, _ := readLimitedString(res.Body, 4096)
	applog.LogInfof(ctx, "Umbra object PUT completed: backup_id=%d status=%d content_type=%s body=%q",
		presign.BackupID, res.StatusCode, res.Header.Get("Content-Type"), bodySnippet)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &umbraSDK.UmbraError{
			Kind:       umbraSDK.ErrStorageUnavailable,
			HTTPStatus: res.StatusCode,
			Message:    fmt.Sprintf("object storage upload failed: status=%d body=%s", res.StatusCode, bodySnippet),
		}
	}

	confirmed, err := p.client.Backup.ConfirmUpload(ctx, umbraSDK.BackupTarget{BackupID: presign.BackupID})
	if err != nil {
		applog.LogErrorf(ctx, "Umbra upload confirm failed: backup_id=%d error=%v", presign.BackupID, err)
		return nil, err
	}
	applog.LogInfof(ctx, "Umbra upload confirm success: backup_id=%d size=%d etag=%s quota_used=%d quota_total=%d",
		confirmed.BackupID, confirmed.SizeBytes, confirmed.ETag, confirmed.Quota.UsedBytes, confirmed.Quota.QuotaBytes)

	return &umbraSDK.UploadResult{
		BackupID:  confirmed.BackupID,
		SizeBytes: confirmed.SizeBytes,
		ETag:      confirmed.ETag,
		Quota:     confirmed.Quota,
	}, nil
}

func (p *UmbraProvider) DownloadFile(ctx context.Context, cloudPath, localPath string) error {
	address, err := parseCloudPath(cloudPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil && filepath.Dir(localPath) != "." {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}

	if _, err := p.client.Backup.DownloadFile(ctx, umbraSDK.BackupTarget{Address: address}, localPath, umbraSDK.DownloadOptions{
		Overwrite: true,
	}); err != nil {
		return fmt.Errorf("下载 Umbra 备份失败: %w", err)
	}
	return nil
}

func (p *UmbraProvider) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	parsed, err := parsePrefix(prefix)
	if err != nil {
		return nil, err
	}

	filter := umbraSDK.BackupListFilter{
		Category: parsed.Category,
		Subject:  parsed.Subject,
	}
	applog.LogInfof(ctx, "Umbra list start: prefix=%s path_prefix=%s category=%s subject=%s",
		prefix, parsed.PathPrefix, filter.Category, filter.Subject)

	records, err := p.listBackupRecords(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("列出 Umbra 备份失败: %w", err)
	}
	applog.LogInfof(ctx, "Umbra list returned: prefix=%s records=%d", prefix, len(records))

	sort.Slice(records, func(i, j int) bool {
		return records[i].UploadedAt.After(records[j].UploadedAt)
	})

	keys := make([]string, 0, len(records))
	for _, record := range records {
		applog.LogInfof(ctx, "Umbra list record: backup_id=%d category=%s subject=%s version=%s size=%d uploaded_at=%s",
			record.BackupID, record.Category, record.Subject, record.Version, record.SizeBytes, record.UploadedAt.Format(time.RFC3339))
		key, ok := recordToPath(record)
		if !ok {
			continue
		}
		if strings.HasPrefix(key, parsed.PathPrefix) {
			keys = append(keys, key)
		}
	}
	applog.LogInfof(ctx, "Umbra list mapped: prefix=%s keys=%d values=%s", prefix, len(keys), strings.Join(keys, ","))

	return keys, nil
}

func (p *UmbraProvider) listBackupRecords(ctx context.Context, filter umbraSDK.BackupListFilter) ([]umbraSDK.BackupRecord, error) {
	records, err := p.client.Backup.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(records) > 0 {
		return records, nil
	}

	compatRecords, err := p.listBackupRecordsCompat(ctx, filter)
	if err != nil {
		applog.LogWarningf(ctx, "Umbra compat list failed after SDK returned empty list: %v", err)
		return records, nil
	}
	if len(compatRecords) > 0 {
		applog.LogInfof(ctx, "Umbra compat list recovered records: count=%d", len(compatRecords))
		return compatRecords, nil
	}
	return records, nil
}

func (p *UmbraProvider) listBackupRecordsCompat(ctx context.Context, filter umbraSDK.BackupListFilter) ([]umbraSDK.BackupRecord, error) {
	token, err := p.client.Auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	if strings.TrimSpace(string(filter.Category)) != "" {
		query.Set("category", string(filter.Category))
	}
	if strings.TrimSpace(filter.Subject) != "" {
		query.Set("subject", filter.Subject)
	}

	listURL := strings.TrimRight(p.client.APIBaseURL(), "/") + "/client/backup/list"
	if len(query) > 0 {
		listURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("User-Agent", "umbra-go")

	res, err := p.client.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	applog.LogInfof(ctx, "Umbra compat list response: status=%d body=%s", res.StatusCode, truncateForLog(string(body), 4096))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("list returned status %d", res.StatusCode)
	}

	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("list returned code %d: %s", env.Code, env.Msg)
	}

	var out struct {
		Files   []compatBackupRecord `json:"files"`
		Items   []compatBackupRecord `json:"items"`
		Records []compatBackupRecord `json:"records"`
		Data    []compatBackupRecord `json:"data"`
		Total   int                  `json:"total"`
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil, nil
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		var direct []compatBackupRecord
		if directErr := json.Unmarshal(env.Data, &direct); directErr == nil {
			return compatRecordsToSDK(direct), nil
		}
		return nil, err
	}
	switch {
	case len(out.Files) > 0:
		return compatRecordsToSDK(out.Files), nil
	case len(out.Items) > 0:
		return compatRecordsToSDK(out.Items), nil
	case len(out.Records) > 0:
		return compatRecordsToSDK(out.Records), nil
	case len(out.Data) > 0:
		return compatRecordsToSDK(out.Data), nil
	default:
		return nil, nil
	}
}

type compatBackupRecord struct {
	umbraSDK.BackupRecord
	Key       string `json:"key,omitempty"`
	ObjectKey string `json:"object_key,omitempty"`
	Path      string `json:"path,omitempty"`
}

func compatRecordsToSDK(records []compatBackupRecord) []umbraSDK.BackupRecord {
	out := make([]umbraSDK.BackupRecord, 0, len(records))
	for _, record := range records {
		converted := record.BackupRecord
		fillRecordFromObjectKey(&converted, firstNonEmpty(record.Key, record.ObjectKey, record.Path))
		out = append(out, converted)
	}
	return out
}

func fillRecordFromObjectKey(record *umbraSDK.BackupRecord, objectKey string) {
	if objectKey == "" {
		return
	}

	parts := strings.Split(strings.Trim(objectKey, "/"), "/")
	if len(parts) < 4 {
		return
	}
	category := parts[1]
	subject := parts[2]
	version := strings.Join(parts[3:], "/")
	if record.Category == "" {
		record.Category = category
	}
	if record.Subject == "" && subject != "-" {
		record.Subject = subject
	}
	if record.Version == "" {
		record.Version = version
	}
}

func (p *UmbraProvider) DeleteObject(ctx context.Context, key string) error {
	address, err := parseCloudPath(key)
	if err != nil {
		return err
	}

	if _, err := p.client.Backup.Delete(ctx, umbraSDK.BackupTarget{Address: address}); err != nil {
		var umbraErr *umbraSDK.UmbraError
		if errors.As(err, &umbraErr) && umbraErr.Kind == umbraSDK.ErrFileNotFound {
			return nil
		}
		return fmt.Errorf("删除 Umbra 备份失败: %w", err)
	}
	return nil
}

func (p *UmbraProvider) TestConnection(ctx context.Context) error {
	if _, err := p.client.User.Quota(ctx); err != nil {
		return fmt.Errorf("测试 Umbra 连接失败: %w", err)
	}
	return nil
}

func (p *UmbraProvider) Profile(ctx context.Context) (*umbraSDK.UserProfile, error) {
	profile, err := p.client.User.Profile(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Umbra 用户信息失败: %w", err)
	}
	return profile, nil
}

func (p *UmbraProvider) EnsureDir(ctx context.Context, path string) error {
	_ = ctx
	_ = path
	return nil
}

func (p *UmbraProvider) GetCloudPath(userID, subPath string) string {
	_ = userID
	normalized := normalizeCloudPath(subPath)
	switch {
	case normalized == "database":
		return strings.TrimSuffix(pathPrefixDB, "/")
	case strings.HasPrefix(normalized, legacyPrefixDB):
		return pathPrefixDB + strings.TrimPrefix(normalized, legacyPrefixDB)
	case normalized == "saves":
		return strings.TrimSuffix(pathPrefixGame, "/")
	case strings.HasPrefix(normalized, legacyPrefixGame):
		parts := strings.Split(normalized, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return strings.TrimSuffix(pathPrefixGame, "/")
		}
		mapped := []string{"game", saveSubject(parts[1])}
		if len(parts) > 2 {
			mapped = append(mapped, parts[2:]...)
		}
		return strings.Join(mapped, "/")
	case normalized == "sync/library":
		return pathPrefixAsset + subjectSyncLibrary
	case normalized == "sync/library/latest.json":
		return pathPrefixAsset + subjectSyncLibrary + "/latest.json"
	case normalized == "sync/covers":
		return pathPrefixAsset + subjectSyncCovers
	case strings.HasPrefix(normalized, "sync/covers/"):
		fileName := strings.TrimPrefix(normalized, "sync/covers/")
		version, err := coverPathToVersion(fileName)
		if err != nil {
			version = sanitizeSegment(fileName)
		}
		return pathPrefixAsset + subjectSyncCovers + "/" + version
	default:
		return normalized
	}
}

func (p *UmbraProvider) Login(ctx context.Context) (*umbraSDK.Session, error) {
	session, err := p.client.Auth.Login(ctx)
	if err != nil {
		return nil, fmt.Errorf("Umbra 授权失败: %w", err)
	}
	return session, nil
}

func (p *UmbraProvider) Logout(ctx context.Context) error {
	if err := p.client.Auth.Logout(ctx); err != nil {
		return fmt.Errorf("Umbra 退出登录失败: %w", err)
	}
	return nil
}

type prefixMapping struct {
	PathPrefix string
	Category   umbraSDK.BackupCategory
	Subject    string
}

func parsePrefix(prefix string) (prefixMapping, error) {
	normalized := normalizeCloudPath(prefix)
	switch {
	case hasPathPrefix(normalized, "db"), hasPathPrefix(normalized, "database"):
		return prefixMapping{PathPrefix: pathPrefixDB, Category: umbraSDK.CategoryDB, Subject: ""}, nil
	case hasPathPrefix(normalized, "full"):
		return prefixMapping{PathPrefix: pathPrefixFull, Category: umbraSDK.CategoryFull, Subject: ""}, nil
	case hasPathPrefix(normalized, "game"):
		parts := strings.Split(normalized, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return prefixMapping{PathPrefix: pathPrefixGame, Category: umbraSDK.CategoryGame, Subject: ""}, nil
		}
		subject := saveSubject(parts[1])
		return prefixMapping{PathPrefix: pathPrefixGame + subject + "/", Category: umbraSDK.CategoryGame, Subject: subject}, nil
	case hasPathPrefix(normalized, "saves"):
		parts := strings.Split(normalized, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return prefixMapping{PathPrefix: pathPrefixGame, Category: umbraSDK.CategoryGame, Subject: ""}, nil
		}
		subject := saveSubject(parts[1])
		return prefixMapping{PathPrefix: pathPrefixGame + subject + "/", Category: umbraSDK.CategoryGame, Subject: subject}, nil
	case hasPathPrefix(normalized, "asset"):
		parts := strings.Split(normalized, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return prefixMapping{PathPrefix: pathPrefixAsset, Category: umbraSDK.CategoryAsset, Subject: ""}, nil
		}
		subject := sanitizeSegment(parts[1])
		return prefixMapping{PathPrefix: pathPrefixAsset + subject + "/", Category: umbraSDK.CategoryAsset, Subject: subject}, nil
	case strings.HasPrefix(normalized, "sync/library"):
		return prefixMapping{PathPrefix: pathPrefixAsset + subjectSyncLibrary + "/", Category: umbraSDK.CategoryAsset, Subject: subjectSyncLibrary}, nil
	case strings.HasPrefix(normalized, "sync/covers"):
		return prefixMapping{PathPrefix: pathPrefixAsset + subjectSyncCovers + "/", Category: umbraSDK.CategoryAsset, Subject: subjectSyncCovers}, nil
	default:
		return prefixMapping{}, fmt.Errorf("不支持的 Umbra 前缀: %s", prefix)
	}
}

func parseCloudPath(cloudPath string) (umbraSDK.BackupAddress, error) {
	normalized := normalizeCloudPath(cloudPath)

	switch {
	case strings.HasPrefix(normalized, pathPrefixDB):
		name := strings.TrimPrefix(normalized, pathPrefixDB)
		version, err := zipNameToVersion(name)
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.DBBackup(version), nil
	case strings.HasPrefix(normalized, legacyPrefixDB):
		name := strings.TrimPrefix(normalized, legacyPrefixDB)
		version, err := zipNameToVersion(name)
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.DBBackup(version), nil
	case strings.HasPrefix(normalized, pathPrefixFull):
		name := strings.TrimPrefix(normalized, pathPrefixFull)
		version, err := zipNameToVersion(name)
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.FullBackup(version), nil
	case strings.HasPrefix(normalized, pathPrefixGame):
		parts := strings.Split(normalized, "/")
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" {
			return umbraSDK.BackupAddress{}, fmt.Errorf("无效的 Umbra 游戏存档路径: %s", cloudPath)
		}
		version, err := zipNameToVersion(parts[2])
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.GameBackup(saveSubject(parts[1]), version), nil
	case strings.HasPrefix(normalized, legacyPrefixGame):
		parts := strings.Split(normalized, "/")
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" {
			return umbraSDK.BackupAddress{}, fmt.Errorf("无效的 Umbra 游戏存档路径: %s", cloudPath)
		}
		version, err := zipNameToVersion(parts[2])
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.GameBackup(saveSubject(parts[1]), version), nil
	case normalized == pathPrefixAsset+subjectSyncLibrary+"/latest.json":
		return umbraSDK.AssetBackup(subjectSyncLibrary, "latest"), nil
	case strings.HasPrefix(normalized, pathPrefixAsset):
		parts := strings.Split(normalized, "/")
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			return umbraSDK.BackupAddress{}, fmt.Errorf("无效的 Umbra 资源路径: %s", cloudPath)
		}
		return umbraSDK.AssetBackup(sanitizeSegment(parts[1]), sanitizeSegment(parts[2])), nil
	case normalized == "sync/library/latest.json":
		return umbraSDK.AssetBackup(subjectSyncLibrary, "latest"), nil
	case strings.HasPrefix(normalized, "sync/covers/"):
		fileName := strings.TrimPrefix(normalized, "sync/covers/")
		version, err := coverPathToVersion(fileName)
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.AssetBackup(subjectSyncCovers, version), nil
	default:
		return umbraSDK.BackupAddress{}, fmt.Errorf("不支持的 Umbra 路径: %s", cloudPath)
	}
}

func recordToPath(record umbraSDK.BackupRecord) (string, bool) {
	switch {
	case record.Category == string(umbraSDK.CategoryGame):
		return fmt.Sprintf("game/%s/%s.zip", record.Subject, record.Version), true
	case record.Category == string(umbraSDK.CategoryDB):
		return fmt.Sprintf("db/%s.zip", record.Version), true
	case record.Category == string(umbraSDK.CategoryFull):
		return fmt.Sprintf("full/%s.zip", record.Version), true
	case record.Category == string(umbraSDK.CategoryAsset) && record.Subject == subjectSyncLibrary && record.Version == "latest":
		return "asset/" + subjectSyncLibrary + "/latest.json", true
	case record.Category == string(umbraSDK.CategoryAsset):
		return fmt.Sprintf("asset/%s/%s", record.Subject, record.Version), true
	default:
		return "", false
	}
}

func normalizeCloudPath(path string) string {
	return strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
}

func hasPathPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func zipNameToVersion(name string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return "", fmt.Errorf("无效的 Umbra zip 文件名: %s", name)
	}
	version := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("无效的 Umbra 版本号: %s", name)
	}
	return version, nil
}

func saveSubject(gameID string) string {
	return sanitizeSegment(gameID)
}

func coverPathToVersion(fileName string) (string, error) {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if base == "" || ext == "" {
		return "", fmt.Errorf("无效的 Umbra 封面文件名: %s", fileName)
	}
	return sanitizeSegment(base) + "__" + sanitizeSegment(ext), nil
}

func versionToCoverPath(version string) (string, error) {
	parts := strings.SplitN(version, "__", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("无效的 Umbra 封面版本: %s", version)
	}
	return parts[0] + "." + parts[1], nil
}

func sanitizeSegment(value string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ".", "_", " ", "_").Replace(strings.TrimSpace(value))
}

func contentTypeForPath(path string) string {
	if ext := filepath.Ext(path); ext != "" {
		if ct := mime.TypeByExtension(ext); ct != "" {
			return ct
		}
	}
	return "application/octet-stream"
}

func hashOpenFile(file *os.File) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sanitizePresignedURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid-url>"
	}
	queryCount := len(parsed.Query())
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if queryCount > 0 {
		return fmt.Sprintf("%s?%d_query_params_redacted", parsed.String(), queryCount)
	}
	return parsed.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readLimitedString(reader io.Reader, limit int64) (string, error) {
	if limit <= 0 {
		return "", nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit {
		return string(data[:limit]) + "...<truncated>", nil
	}
	return string(data), nil
}

func truncateForLog(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...<truncated>"
}

func allowOverwriteRetry(err error) bool {
	var umbraErr *umbraSDK.UmbraError
	return errors.As(err, &umbraErr) && umbraErr.Kind == umbraSDK.ErrFileAlreadyExists
}

type browserOpenerFunc func(string) error

func (f browserOpenerFunc) OpenURL(ctx context.Context, url string) error {
	_ = ctx
	return f(url)
}

type configTokenStore struct {
	config *appconf.AppConfig
	token  *umbraSDK.TokenSet
}

func newConfigTokenStore(config *appconf.AppConfig) (*configTokenStore, error) {
	store := &configTokenStore{config: config}
	if config == nil {
		return store, nil
	}
	if strings.TrimSpace(config.UmbraAccessToken) == "" && strings.TrimSpace(config.UmbraRefreshToken) == "" {
		return store, nil
	}

	var expiresAt time.Time
	if strings.TrimSpace(config.UmbraTokenExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, config.UmbraTokenExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("解析 Umbra token 过期时间失败: %w", err)
		}
		expiresAt = parsed
	}

	store.token = &umbraSDK.TokenSet{
		AccessToken:  strings.TrimSpace(config.UmbraAccessToken),
		RefreshToken: strings.TrimSpace(config.UmbraRefreshToken),
		TokenType:    strings.TrimSpace(config.UmbraTokenType),
		Scope:        strings.TrimSpace(config.UmbraTokenScope),
		ExpiresAt:    expiresAt,
	}
	return store, nil
}

func (s *configTokenStore) Load(ctx context.Context) (*umbraSDK.TokenSet, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.token == nil {
		return nil, nil
	}
	copyToken := *s.token
	return &copyToken, nil
}

func (s *configTokenStore) Save(ctx context.Context, token *umbraSDK.TokenSet) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if token == nil {
		s.token = nil
		if s.config != nil {
			s.config.UmbraAccessToken = ""
			s.config.UmbraRefreshToken = ""
			s.config.UmbraTokenType = ""
			s.config.UmbraTokenScope = ""
			s.config.UmbraTokenExpiresAt = ""
		}
		return nil
	}

	copyToken := *token
	s.token = &copyToken
	if s.config != nil {
		s.config.UmbraAccessToken = copyToken.AccessToken
		s.config.UmbraRefreshToken = copyToken.RefreshToken
		s.config.UmbraTokenType = copyToken.TokenType
		s.config.UmbraTokenScope = copyToken.Scope
		if copyToken.ExpiresAt.IsZero() {
			s.config.UmbraTokenExpiresAt = ""
		} else {
			s.config.UmbraTokenExpiresAt = copyToken.ExpiresAt.Format(time.RFC3339)
		}
	}
	return nil
}

func (s *configTokenStore) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.token = nil
	if s.config != nil {
		s.config.UmbraAccessToken = ""
		s.config.UmbraRefreshToken = ""
		s.config.UmbraTokenType = ""
		s.config.UmbraTokenScope = ""
		s.config.UmbraTokenExpiresAt = ""
	}
	return nil
}
