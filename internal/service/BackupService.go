package service

import (
	"context"
	"database/sql"
)

type BackupService struct {
	ctx context.Context
	db  *sql.DB
}

func NewBackupService() *BackupService {
	return &BackupService{}
}

func (s *BackupService) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

func (s *BackupService) GetBackupPresignedURL(filename string) (string, error) {
	return "", nil
}
