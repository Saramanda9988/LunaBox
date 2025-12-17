package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

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

// GetBackupDir 获取备份根目录
func (s *BackupService) GetBackupDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	backupDir := filepath.Join(filepath.Dir(execPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}
	return backupDir, nil
}

// GetGameBackups 获取游戏的备份历史
func (s *BackupService) GetGameBackups(gameID string) ([]models.GameBackup, error) {
	query := `SELECT id, game_id, backup_path, size, created_at 
		FROM game_backups WHERE game_id = ? ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(s.ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query backups: %w", err)
	}
	defer rows.Close()

	var backups []models.GameBackup
	for rows.Next() {
		var backup models.GameBackup
		err := rows.Scan(&backup.ID, &backup.GameID, &backup.BackupPath, &backup.Size, &backup.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backup: %w", err)
		}
		backups = append(backups, backup)
	}
	return backups, nil
}

// CreateBackup 创建游戏存档备份
func (s *BackupService) CreateBackup(gameID string) (*models.GameBackup, error) {
	// 获取游戏信息
	var savePath string
	err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", gameID).Scan(&savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}
	if savePath == "" {
		return nil, fmt.Errorf("存档目录未设置")
	}

	// 检查存档目录是否存在
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("存档目录不存在: %s", savePath)
	}

	// 创建备份目录
	backupDir, err := s.GetBackupDir()
	if err != nil {
		return nil, err
	}
	gameBackupDir := filepath.Join(backupDir, gameID)
	if err := os.MkdirAll(gameBackupDir, 0755); err != nil {
		return nil, err
	}

	// 生成备份文件名
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupFileName := fmt.Sprintf("%s.zip", timestamp)
	backupPath := filepath.Join(gameBackupDir, backupFileName)

	// 压缩存档目录
	size, err := s.zipDirectory(savePath, backupPath)
	if err != nil {
		return nil, fmt.Errorf("备份失败: %w", err)
	}

	// 保存备份记录
	backup := &models.GameBackup{
		ID:         uuid.New().String(),
		GameID:     gameID,
		BackupPath: backupPath,
		Size:       size,
		CreatedAt:  time.Now(),
	}

	_, err = s.db.ExecContext(s.ctx,
		"INSERT INTO game_backups (id, game_id, backup_path, size, created_at) VALUES (?, ?, ?, ?, ?)",
		backup.ID, backup.GameID, backup.BackupPath, backup.Size, backup.CreatedAt)
	if err != nil {
		os.Remove(backupPath)
		return nil, fmt.Errorf("failed to save backup record: %w", err)
	}

	return backup, nil
}

// RestoreBackup 恢复备份到指定时间点
func (s *BackupService) RestoreBackup(backupID string) error {
	// 获取备份信息
	var backup models.GameBackup
	var gameID string
	err := s.db.QueryRowContext(s.ctx,
		"SELECT id, game_id, backup_path, size, created_at FROM game_backups WHERE id = ?", backupID).
		Scan(&backup.ID, &backup.GameID, &backup.BackupPath, &backup.Size, &backup.CreatedAt)
	if err != nil {
		return fmt.Errorf("备份记录不存在")
	}
	gameID = backup.GameID

	// 获取游戏存档目录
	var savePath string
	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", gameID).Scan(&savePath)
	if err != nil || savePath == "" {
		return fmt.Errorf("存档目录未设置")
	}

	// 检查备份文件是否存在
	if _, err := os.Stat(backup.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backup.BackupPath)
	}

	// 先备份当前存档（恢复前备份）
	if _, err := os.Stat(savePath); err == nil {
		backupDir, _ := s.GetBackupDir()
		preRestoreDir := filepath.Join(backupDir, gameID, "pre_restore")
		os.MkdirAll(preRestoreDir, 0755)
		preRestorePath := filepath.Join(preRestoreDir, fmt.Sprintf("%s_before_restore.zip", time.Now().Format("2006-01-02T15-04-05")))
		s.zipDirectory(savePath, preRestorePath)
	}

	// 清空目标目录
	if err := os.RemoveAll(savePath); err != nil {
		return fmt.Errorf("清空存档目录失败: %w", err)
	}
	if err := os.MkdirAll(savePath, 0755); err != nil {
		return fmt.Errorf("创建存档目录失败: %w", err)
	}

	// 解压备份
	if err := s.unzipFile(backup.BackupPath, savePath); err != nil {
		return fmt.Errorf("恢复失败: %w", err)
	}

	return nil
}

// DeleteBackup 删除备份
func (s *BackupService) DeleteBackup(backupID string) error {
	var backupPath string
	err := s.db.QueryRowContext(s.ctx, "SELECT backup_path FROM game_backups WHERE id = ?", backupID).Scan(&backupPath)
	if err != nil {
		return fmt.Errorf("备份记录不存在")
	}

	// 删除文件
	os.Remove(backupPath)

	// 删除记录
	_, err = s.db.ExecContext(s.ctx, "DELETE FROM game_backups WHERE id = ?", backupID)
	return err
}

// OpenBackupFolder 打开备份文件夹
func (s *BackupService) OpenBackupFolder(gameID string) error {
	backupDir, err := s.GetBackupDir()
	if err != nil {
		return err
	}
	gameBackupDir := filepath.Join(backupDir, gameID)
	os.MkdirAll(gameBackupDir, 0755)

	// 根据操作系统使用不同命令打开文件夹
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", gameBackupDir)
	case "darwin":
		cmd = exec.Command("open", gameBackupDir)
	default: // linux
		cmd = exec.Command("xdg-open", gameBackupDir)
	}
	return cmd.Start()
}

// zipDirectory 压缩目录
func (s *BackupService) zipDirectory(source, target string) (int64, error) {
	zipFile, err := os.Create(target)
	if err != nil {
		return 0, err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(source, path)
		header.Name = strings.ReplaceAll(relPath, "\\", "/")

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
		}
		return err
	})

	stat, _ := os.Stat(target)
	return stat.Size(), nil
}

// unzipFile 解压文件
func (s *BackupService) unzipFile(source, target string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(target, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
