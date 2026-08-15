package cloudprovider

import (
	"context"
	"fmt"
	"strings"

	"lunabox/internal/appconf"
	"lunabox/internal/service/cloudprovider/onedrive"
	"lunabox/internal/service/cloudprovider/s3"
	"lunabox/internal/service/cloudprovider/umbra"
	"lunabox/internal/service/cloudprovider/webdav"
	"lunabox/internal/version"
)

// ProviderType 云存储提供商类型
type ProviderType string

var _ BatchUploadProvider = (*umbra.Provider)(nil)

const (
	ProviderS3       ProviderType = "s3"
	ProviderOneDrive ProviderType = "onedrive"
	ProviderUmbra    ProviderType = "umbra"
	ProviderWebDAV   ProviderType = "webdav"
)

// HasRequiredBackupUserID reports whether the selected provider has the local backup identity it requires.
func HasRequiredBackupUserID(config *appconf.AppConfig) bool {
	return ProviderType(config.CloudBackupProvider) == ProviderUmbra || strings.TrimSpace(config.BackupUserID) != ""
}

// NewCloudProvider 根据配置创建云存储提供商
func NewCloudProvider(ctx context.Context, config *appconf.AppConfig) (CloudStorageProvider, error) {
	if !config.CloudBackupEnabled {
		return nil, fmt.Errorf("云备份未启用")
	}

	switch ProviderType(config.CloudBackupProvider) {
	case ProviderOneDrive:
		return newOneDriveProviderFromConfig(config)
	case ProviderS3:
		return newS3ProviderFromConfig(config)
	case ProviderUmbra:
		return newUmbraProviderFromConfig(ctx, config)
	case ProviderWebDAV:
		return newWebDAVProviderFromConfig(config)
	default:
		return nil, fmt.Errorf("未知的云备份提供商: %s", config.CloudBackupProvider)
	}
}

// newWebDAVProviderFromConfig 从配置创建 WebDAV Provider
func newWebDAVProviderFromConfig(config *appconf.AppConfig) (*webdav.Provider, error) {
	return webdav.NewProvider(webdav.Config{
		URL:         config.WebDAVURL,
		Username:    config.WebDAVUsername,
		Password:    config.WebDAVPassword,
		ProxyConfig: config,
	})
}

func newUmbraProviderFromConfig(ctx context.Context, config *appconf.AppConfig) (*umbra.Provider, error) {
	return umbra.NewProvider(ctx, umbra.Config{
		BaseURL:     config.UmbraBaseURL,
		ClientID:    version.UmbraOAuthClientID,
		ProxyConfig: config,
	})
}

// newS3ProviderFromConfig 从配置创建 S3 Provider
func newS3ProviderFromConfig(config *appconf.AppConfig) (*s3.S3Provider, error) {
	return s3.NewS3Provider(s3.S3Config{
		Endpoint:    config.S3Endpoint,
		Region:      config.S3Region,
		Bucket:      config.S3Bucket,
		AccessKey:   config.S3AccessKey,
		SecretKey:   config.S3SecretKey,
		ProxyConfig: config,
	})
}

// newOneDriveProviderFromConfig 从配置创建 OneDrive Provider
func newOneDriveProviderFromConfig(config *appconf.AppConfig) (*onedrive.OneDriveProvider, error) {
	return onedrive.NewOneDriveProvider(onedrive.OneDriveConfig{
		ClientID:     config.OneDriveClientID,
		RefreshToken: config.OneDriveRefreshToken,
		ProxyConfig:  config,
	})
}

// TestConnection 测试云存储连接
func TestConnection(ctx context.Context, providerType ProviderType, config *appconf.AppConfig) error {
	switch providerType {
	case ProviderS3:
		provider, err := newS3ProviderFromConfig(config)
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	case ProviderOneDrive:
		provider, err := newOneDriveProviderFromConfig(config)
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	case ProviderUmbra:
		provider, err := newUmbraProviderFromConfig(ctx, config)
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	case ProviderWebDAV:
		provider, err := newWebDAVProviderFromConfig(config)
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	default:
		return fmt.Errorf("未知的云备份提供商: %s", providerType)
	}
}

// IsConfigured 检查云备份是否已配置
func IsConfigured(config *appconf.AppConfig) bool {
	if !config.CloudBackupEnabled {
		return false
	}
	switch ProviderType(config.CloudBackupProvider) {
	case ProviderOneDrive:
		return config.OneDriveClientID != "" && config.OneDriveRefreshToken != "" && config.BackupUserID != ""
	case ProviderS3:
		return config.S3Endpoint != "" && config.S3AccessKey != "" && config.BackupUserID != ""
	case ProviderWebDAV:
		return config.WebDAVURL != "" && config.BackupUserID != ""
	case ProviderUmbra:
		if !config.UmbraAuthenticated || config.UmbraBaseURL == "" || strings.TrimSpace(version.UmbraOAuthClientID) == "" {
			return false
		}
		return umbra.HasStoredCredentials(context.Background(), umbra.Config{
			BaseURL:  config.UmbraBaseURL,
			ClientID: version.UmbraOAuthClientID,
		})
	default:
		return false
	}
}
