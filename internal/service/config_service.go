package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ConfigService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewConfigService() *ConfigService {
	return &ConfigService{}
}

func (s *ConfigService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *ConfigService) GetAppConfig() (appconf.AppConfig, error) {
	return *s.config, nil
}

func (s *ConfigService) UpdateAppConfig(newConfig appconf.AppConfig) error {
	if newConfig.Theme == "" || newConfig.Language == "" {
		runtime.LogErrorf(s.ctx, "invalid config")
		return fmt.Errorf("invalid config")
	}

	// 如果备份密码有变化，重新生成 user-id
	if newConfig.BackupPassword != "" && newConfig.BackupPassword != s.config.BackupPassword {
		newConfig.BackupUserID = utils.GenerateUserID(newConfig.BackupPassword)
	}

	err := appconf.SaveConfig(&newConfig)
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to save config: %v", err)
		return err
	}

	// 更新应用配置 in-memory
	s.config.BangumiAccessToken = newConfig.BangumiAccessToken
	s.config.VNDBAccessToken = newConfig.VNDBAccessToken
	s.config.Theme = newConfig.Theme
	s.config.Language = newConfig.Language
	s.config.AIProvider = newConfig.AIProvider
	s.config.AIBaseURL = newConfig.AIBaseURL
	s.config.AIAPIKey = newConfig.AIAPIKey
	s.config.AIModel = newConfig.AIModel
	// 云备份配置
	s.config.CloudBackupEnabled = newConfig.CloudBackupEnabled
	s.config.CloudBackupProvider = newConfig.CloudBackupProvider
	s.config.BackupPassword = newConfig.BackupPassword
	s.config.BackupUserID = newConfig.BackupUserID
	s.config.S3Endpoint = newConfig.S3Endpoint
	s.config.S3Region = newConfig.S3Region
	s.config.S3Bucket = newConfig.S3Bucket
	s.config.S3AccessKey = newConfig.S3AccessKey
	s.config.S3SecretKey = newConfig.S3SecretKey
	s.config.CloudBackupRetention = newConfig.CloudBackupRetention
	// OneDrive OAuth
	s.config.OneDriveRefreshToken = newConfig.OneDriveRefreshToken
	return nil
}
