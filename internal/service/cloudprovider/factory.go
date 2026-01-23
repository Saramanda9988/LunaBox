package cloudprovider

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/service/cloudprovider/onedrive"
	"lunabox/internal/service/cloudprovider/s3"
)

// ProviderType 云存储提供商类型
type ProviderType string

const (
	ProviderS3       ProviderType = "s3"
	ProviderOneDrive ProviderType = "onedrive"
)

// NewCloudProvider 根据配置创建云存储提供商
func NewCloudProvider(config *appconf.AppConfig) (CloudStorageProvider, error) {
	if !config.CloudBackupEnabled {
		return nil, fmt.Errorf("云备份未启用")
	}

	switch ProviderType(config.CloudBackupProvider) {
	case ProviderOneDrive:
		return newOneDriveProviderFromConfig(config)
	case ProviderS3:
		return newS3ProviderFromConfig(config)
	default:
		return nil, fmt.Errorf("未知的云备份提供商: %s", config.CloudBackupProvider)
	}
}

// newS3ProviderFromConfig 从配置创建 S3 Provider
func newS3ProviderFromConfig(config *appconf.AppConfig) (*s3.S3Provider, error) {
	return s3.NewS3Provider(s3.S3Config{
		Endpoint:  config.S3Endpoint,
		Region:    config.S3Region,
		Bucket:    config.S3Bucket,
		AccessKey: config.S3AccessKey,
		SecretKey: config.S3SecretKey,
	})
}

// newOneDriveProviderFromConfig 从配置创建 OneDrive Provider
func newOneDriveProviderFromConfig(config *appconf.AppConfig) (*onedrive.OneDriveProvider, error) {
	return onedrive.NewOneDriveProvider(onedrive.OneDriveConfig{
		ClientID:     config.OneDriveClientID,
		RefreshToken: config.OneDriveRefreshToken,
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
		return config.OneDriveRefreshToken != "" && config.BackupUserID != ""
	case ProviderS3:
		return config.S3Endpoint != "" && config.S3AccessKey != "" && config.BackupUserID != ""
	default:
		return false
	}
}
