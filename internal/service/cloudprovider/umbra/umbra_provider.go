package umbra

import (
	"context"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"mime"
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

	pathPrefixDatabase = "database/"
	pathPrefixSaves    = "saves/"
	pathPrefixSync     = "sync/"

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

	_, err = p.client.Backup.UploadFile(ctx, address, localPath, umbraSDK.UploadOptions{
		ContentType: contentTypeForPath(localPath),
		ComputeHash: true,
	})
	if err != nil {
		if allowOverwriteRetry(err) {
			if _, deleteErr := p.client.Backup.Delete(ctx, umbraSDK.BackupTarget{Address: address}); deleteErr == nil {
				_, err = p.client.Backup.UploadFile(ctx, address, localPath, umbraSDK.UploadOptions{
					ContentType: contentTypeForPath(localPath),
					ComputeHash: true,
				})
			}
		}
	}
	if err != nil {
		return fmt.Errorf("上传 Umbra 备份失败: %w", err)
	}
	return nil
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

	records, err := p.client.Backup.List(ctx, umbraSDK.BackupListFilter{
		Category: parsed.Category,
		Subject:  parsed.Subject,
	})
	if err != nil {
		return nil, fmt.Errorf("列出 Umbra 备份失败: %w", err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].UploadedAt.After(records[j].UploadedAt)
	})

	keys := make([]string, 0, len(records))
	for _, record := range records {
		key, ok := recordToPath(record)
		if !ok {
			continue
		}
		if strings.HasPrefix(key, parsed.PathPrefix) {
			keys = append(keys, key)
		}
	}

	return keys, nil
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

func (p *UmbraProvider) EnsureDir(ctx context.Context, path string) error {
	_ = ctx
	_ = path
	return nil
}

func (p *UmbraProvider) GetCloudPath(userID, subPath string) string {
	_ = userID
	return strings.Trim(strings.ReplaceAll(subPath, "\\", "/"), "/")
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
	case strings.HasPrefix(normalized, pathPrefixDatabase):
		return prefixMapping{PathPrefix: pathPrefixDatabase, Category: umbraSDK.CategoryDB, Subject: ""}, nil
	case strings.HasPrefix(normalized, pathPrefixSaves):
		parts := strings.Split(normalized, "/")
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return prefixMapping{}, fmt.Errorf("无效的 Umbra 游戏存档前缀: %s", prefix)
		}
		return prefixMapping{PathPrefix: strings.Join(parts[:2], "/") + "/", Category: umbraSDK.CategoryGame, Subject: saveSubject(parts[1])}, nil
	case strings.HasPrefix(normalized, "sync/library"):
		return prefixMapping{PathPrefix: "sync/library/", Category: umbraSDK.CategoryAsset, Subject: subjectSyncLibrary}, nil
	case strings.HasPrefix(normalized, "sync/covers"):
		return prefixMapping{PathPrefix: "sync/covers/", Category: umbraSDK.CategoryAsset, Subject: subjectSyncCovers}, nil
	default:
		return prefixMapping{}, fmt.Errorf("不支持的 Umbra 前缀: %s", prefix)
	}
}

func parseCloudPath(cloudPath string) (umbraSDK.BackupAddress, error) {
	normalized := normalizeCloudPath(cloudPath)

	switch {
	case strings.HasPrefix(normalized, pathPrefixDatabase):
		name := strings.TrimPrefix(normalized, pathPrefixDatabase)
		version, err := zipNameToVersion(name)
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.DBBackup(version), nil
	case strings.HasPrefix(normalized, pathPrefixSaves):
		parts := strings.Split(normalized, "/")
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" {
			return umbraSDK.BackupAddress{}, fmt.Errorf("无效的 Umbra 游戏存档路径: %s", cloudPath)
		}
		version, err := zipNameToVersion(parts[2])
		if err != nil {
			return umbraSDK.BackupAddress{}, err
		}
		return umbraSDK.GameBackup(saveSubject(parts[1]), version), nil
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
		return fmt.Sprintf("saves/%s/%s.zip", record.Subject, record.Version), true
	case record.Category == string(umbraSDK.CategoryDB):
		return fmt.Sprintf("database/%s.zip", record.Version), true
	case record.Category == string(umbraSDK.CategoryAsset) && record.Subject == subjectSyncLibrary && record.Version == "latest":
		return "sync/library/latest.json", true
	case record.Category == string(umbraSDK.CategoryAsset) && record.Subject == subjectSyncCovers:
		coverPath, err := versionToCoverPath(record.Version)
		if err != nil {
			return "", false
		}
		return "sync/covers/" + coverPath, true
	default:
		return "", false
	}
}

func normalizeCloudPath(path string) string {
	return strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
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
