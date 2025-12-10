package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
)

// BackupService TODO: 目前先不实现
type BackupService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewBackupService() *BackupService {
	return &BackupService{}
}

func (s *BackupService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *BackupService) GetBackupPresignedURL(filename string) (string, error) {
	return "", nil
}
