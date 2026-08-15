package umbra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/utils/httputils"
	"lunabox/internal/utils/proxyutils"
)

const defaultRedirectURI = "http://127.0.0.1:0/auth/callback"

type Config struct {
	BaseURL           string
	ClientID          string
	RegistrationToken string
	ProxyConfig       proxyutils.ProxyConfigProvider
}

type Provider struct {
	client *umbrsdk.Client
	userID string
}

var _ interface {
	UploadFile(context.Context, string, string) error
	DownloadFile(context.Context, string, string) error
	ListObjects(context.Context, string) ([]string, error)
	DeleteObject(context.Context, string) error
	TestConnection(context.Context) error
	EnsureDir(context.Context, string) error
	GetCloudPath(string, string) string
} = (*Provider)(nil)

func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
	client, _, _, err := newClient(cfg, nil, nil)
	if err != nil {
		return nil, err
	}
	return newProviderWithClient(ctx, client)
}

func newProviderWithClient(ctx context.Context, client *umbrsdk.Client) (*Provider, error) {
	profile, err := client.User.Profile(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Umbra 用户信息失败: %w", err)
	}
	if profile.ID == 0 {
		return nil, fmt.Errorf("Umbra 用户 ID 无效")
	}
	return &Provider{client: client, userID: strconv.FormatUint(profile.ID, 10)}, nil
}

func (p *Provider) UploadFile(ctx context.Context, cloudPath, localPath string) error {
	if key, ok, err := p.syncKeyForCloudPath(cloudPath); err != nil {
		return err
	} else if ok {
		return p.uploadSyncFile(ctx, key, localPath)
	}
	address, err := p.addressForCloudPath(cloudPath)
	if err != nil {
		return err
	}
	if _, err := p.client.Backup.UploadFile(ctx, address, localPath, umbrsdk.UploadOptions{ComputeHash: true}); err != nil {
		return fmt.Errorf("Umbra 上传失败: %w", err)
	}
	return nil
}

func (p *Provider) DownloadFile(ctx context.Context, cloudPath, localPath string) error {
	if key, ok, err := p.syncKeyForCloudPath(cloudPath); err != nil {
		return err
	} else if ok {
		return p.downloadSyncFile(ctx, key, localPath)
	}
	address, err := p.addressForCloudPath(cloudPath)
	if err != nil {
		return err
	}
	_, err = p.client.Backup.DownloadFile(ctx, umbrsdk.BackupTarget{Address: address}, localPath, umbrsdk.DownloadOptions{Overwrite: true})
	if err == nil {
		return nil
	}
	var sdkErr *umbrsdk.UmbraError
	if errors.As(err, &sdkErr) && sdkErr.Kind == umbrsdk.ErrFileNotFound {
		return fmt.Errorf("Umbra 文件 not found: %w", err)
	}
	return fmt.Errorf("Umbra 下载失败: %w", err)
}

func (p *Provider) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	subPath, err := p.subPath(prefix)
	if err != nil {
		return nil, err
	}
	if isSyncLibrarySubPath(subPath) {
		return p.listSyncObjects(ctx, subPath)
	}
	query, err := listQueryForSubPath(subPath)
	if err != nil {
		return nil, err
	}
	records, err := p.client.Backup.List(ctx, query.filter)
	if err != nil {
		return nil, fmt.Errorf("Umbra 列出备份失败: %w", err)
	}

	keys := make([]string, 0, len(records))
	for _, record := range records {
		recordSubPath, ok := subPathForRecord(record)
		if !ok || !strings.HasPrefix(recordSubPath, query.prefix) {
			continue
		}
		keys = append(keys, p.cloudPathForSubPath(recordSubPath))
	}
	sort.Strings(keys)
	return keys, nil
}

func (p *Provider) DeleteObject(ctx context.Context, key string) error {
	if syncKey, ok, err := p.syncKeyForCloudPath(key); err != nil {
		return err
	} else if ok {
		return p.deleteSyncObject(ctx, syncKey)
	}
	address, err := p.addressForCloudPath(key)
	if err != nil {
		return err
	}
	if _, err := p.client.Backup.Delete(ctx, umbrsdk.BackupTarget{Address: address}); err != nil {
		return fmt.Errorf("Umbra 删除失败: %w", err)
	}
	return nil
}

func (p *Provider) TestConnection(ctx context.Context) error {
	if _, err := p.client.User.Quota(ctx); err != nil {
		return fmt.Errorf("Umbra 账户连接失败: %w", err)
	}
	if _, err := p.client.Sync.Snapshot(ctx, umbrsdk.SyncSnapshotInput{SpaceName: syncSpaceName, Limit: syncPageLimit}); err != nil {
		return fmt.Errorf("Umbra 设备签名验证失败: %w", err)
	}
	return nil
}

func (p *Provider) EnsureDir(context.Context, string) error { return nil }

func (p *Provider) GetCloudPath(_ string, subPath string) string {
	return p.cloudPathForSubPath(subPath)
}

func (p *Provider) cloudPathForSubPath(subPath string) string {
	return fmt.Sprintf("v1/%s/%s", p.userID, strings.TrimLeft(filepath.ToSlash(subPath), "/"))
}

func (p *Provider) addressForCloudPath(cloudPath string) (umbrsdk.BackupAddress, error) {
	subPath, err := p.subPath(cloudPath)
	if err != nil {
		return umbrsdk.BackupAddress{}, err
	}
	return addressForSubPath(subPath)
}

func (p *Provider) subPath(cloudPath string) (string, error) {
	normalized := strings.TrimLeft(filepath.ToSlash(cloudPath), "/")
	prefix := "v1/" + p.userID + "/"
	if !strings.HasPrefix(normalized, prefix) {
		return "", fmt.Errorf("Umbra 云端对象键不属于当前账户")
	}
	return strings.TrimPrefix(normalized, prefix), nil
}

type BrowserOpenerFunc func(context.Context, string) error

func (fn BrowserOpenerFunc) OpenURL(ctx context.Context, url string) error { return fn(ctx, url) }

func Authenticate(ctx context.Context, cfg Config, appVersion string, opener BrowserOpenerFunc) error {
	_, deviceStore, err := newCredentialStores(cfg)
	if err != nil {
		return err
	}
	credentials, err := deviceStore.Load(ctx)
	if err != nil {
		return err
	}
	needsRegistration := credentials == nil || credentials.DeviceID == "" || credentials.DeviceSecret == ""
	if needsRegistration && strings.TrimSpace(cfg.RegistrationToken) == "" {
		return fmt.Errorf("Umbra 安装令牌未注入，请在构建时配置 LUNABOX_UMBRA_REGISTRATION_TOKEN")
	}

	var registration *umbrsdk.DeviceRegistrationOptions
	if needsRegistration {
		installPath, err := installIDPath()
		if err != nil {
			return fmt.Errorf("获取 Umbra install ID 路径失败: %w", err)
		}
		device, err := umbrsdk.DetectDeviceMetadata(umbrsdk.DeviceMetadataOptions{
			AppVersion:    appVersion,
			InstallIDPath: installPath,
		})
		if err != nil {
			return fmt.Errorf("检测 Umbra 设备信息失败: %w", err)
		}
		registration = &umbrsdk.DeviceRegistrationOptions{
			RegistrationToken: strings.TrimSpace(cfg.RegistrationToken),
			Device:            device,
		}
	}

	client, _, _, err := newClient(cfg, opener, registration)
	if err != nil {
		return err
	}
	if _, err := client.Login(ctx); err != nil {
		return fmt.Errorf("Umbra 授权失败: %w", err)
	}
	return nil
}

func Logout(ctx context.Context, cfg Config) error {
	client, _, _, err := newClient(cfg, nil, nil)
	if err != nil {
		return err
	}
	if err := client.Logout(ctx); err != nil {
		return fmt.Errorf("退出 Umbra 授权失败: %w", err)
	}
	return nil
}

func GetUserProfile(ctx context.Context, cfg Config) (*umbrsdk.UserProfile, error) {
	client, _, _, err := newClient(cfg, nil, nil)
	if err != nil {
		return nil, err
	}
	profile, err := client.User.Profile(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 Umbra 用户信息失败: %w", err)
	}
	return profile, nil
}

func HasStoredCredentials(ctx context.Context, cfg Config) bool {
	tokens, devices, err := newCredentialStores(cfg)
	if err != nil {
		return false
	}
	token, err := tokens.Load(ctx)
	if err != nil || token == nil || (token.AccessToken == "" && token.RefreshToken == "") {
		return false
	}
	device, err := devices.Load(ctx)
	return err == nil && device != nil && device.DeviceID != "" && device.DeviceSecret != ""
}

func newClient(cfg Config, opener BrowserOpenerFunc, registration *umbrsdk.DeviceRegistrationOptions) (*umbrsdk.Client, umbrsdk.TokenStore, umbrsdk.DeviceStore, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, nil, nil, fmt.Errorf("Umbra Base URL 不能为空")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, nil, nil, fmt.Errorf("Umbra OAuth Client ID 未注入，请在构建时配置 LUNABOX_UMBRA_CLIENT_ID")
	}
	httpClient, _, err := httputils.NewClient(httputils.ClientOptions{
		Timeout:     60 * time.Second,
		ProxyConfig: cfg.ProxyConfig,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 Umbra HTTP 客户端失败: %w", err)
	}
	return newClientWithHTTPClient(cfg, httpClient, opener, registration)
}

func newClientWithHTTPClient(cfg Config, httpClient *http.Client, opener BrowserOpenerFunc, registration *umbrsdk.DeviceRegistrationOptions) (*umbrsdk.Client, umbrsdk.TokenStore, umbrsdk.DeviceStore, error) {
	tokens, devices, err := newCredentialStores(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	sdkConfig := umbrsdk.Config{
		BaseURL:            cfg.BaseURL,
		ClientID:           cfg.ClientID,
		RedirectURI:        defaultRedirectURI,
		HTTPClient:         httpClient,
		TokenStore:         tokens,
		DeviceStore:        devices,
		DeviceRegistration: registration,
	}
	if opener != nil {
		sdkConfig.BrowserOpener = opener
	}
	client, err := umbrsdk.New(sdkConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("创建 Umbra SDK 客户端失败: %w", err)
	}
	return client, tokens, devices, nil
}
