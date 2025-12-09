package models

import "time"

type User struct {
	ID                  string    `json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	DefaultBackupTarget string    `json:"default_backup_target"` // "s3" or "docker"
}
