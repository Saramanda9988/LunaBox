package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
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
		return fmt.Errorf("invalid config")
	}
	err := appconf.SaveConfig(&newConfig)
	if err != nil {
		return err
	}
	// 更新应用配置 in-memory
	s.config.BangumiAccessToken = newConfig.BangumiAccessToken
	s.config.VNDBAccessToken = newConfig.VNDBAccessToken
	s.config.Theme = newConfig.Theme
	s.config.Language = newConfig.Language
	return nil
}
