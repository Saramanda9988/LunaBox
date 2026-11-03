package cloudbackup

import "lunabox/backend/appconf"

// ProviderConfigured 对应 utils/cloudSync.ts 的 isCloudProviderConfigured：云服务已启用，并且当前提供商的
// 授权与备份密码（Umbra 不需要密码）都已设置。自动备份、数据库备份等面板共用。
func ProviderConfigured(cfg *appconf.AppConfig) bool {
	if cfg == nil || !cfg.CloudBackupEnabled {
		return false
	}
	if cfg.CloudBackupProvider == "umbra" {
		return cfg.UmbraBaseURL != "" && cfg.UmbraAuthenticated
	}
	if cfg.BackupUserID == "" {
		return false
	}
	switch cfg.CloudBackupProvider {
	case "onedrive":
		return cfg.OneDriveRefreshToken != ""
	case "webdav":
		return cfg.WebDAVURL != ""
	}
	return cfg.S3Endpoint != "" && cfg.S3AccessKey != ""
}

// SyncIntervalSeconds 对应 utils/cloudSync.ts 的 getSyncIntervalSeconds：未设置时 60 秒，最少 15 秒。
func SyncIntervalSeconds(cfg *appconf.AppConfig) int {
	if cfg == nil || cfg.CloudSyncIntervalSec <= 0 {
		return 60
	}
	return max(15, cfg.CloudSyncIntervalSec)
}
