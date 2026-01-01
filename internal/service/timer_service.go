package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type TimerService struct {
	ctx           context.Context
	db            *sql.DB
	config        *appconf.AppConfig
	backupService *BackupService
}

func NewTimerService() *TimerService {
	return &TimerService{}
}

func (s *TimerService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

// SetBackupService 设置备份服务（用于自动备份）
func (s *TimerService) SetBackupService(backupService *BackupService) {
	s.backupService = backupService
}

// StartGameWithTracking 启动游戏并自动追踪游玩时长
// 当游戏进程退出时，自动保存游玩记录到数据库
func (s *TimerService) StartGameWithTracking(gameID string) (bool, error) {
	//获取游戏路径
	path, err := s.getGamePath(gameID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("failed to get game path: %w", err)
	}

	if path == "" {
		runtime.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("game path is empty for game: %s", gameID)
	}

	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		runtime.LogErrorf(s.ctx, "failed to start game: %v", err)
		return false, fmt.Errorf("failed to start game: %w", err)
	}

	sessionID := uuid.New().String()
	startTime := time.Now()

	_, err = s.db.ExecContext(
		s.ctx,
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID,
		gameID,
		startTime,
		startTime, // 临时占位，等游戏结束后更新
		0,         // 初始时长为 0
	)
	if err != nil {
		return false, fmt.Errorf("failed to create play session: %w", err)
	}

	go s.waitForGameExit(cmd, sessionID, gameID, startTime)

	// 启动成功，返回 true 给前端
	return true, nil
}

// waitForGameExit 等待游戏进程退出并更新游玩记录
func (s *TimerService) waitForGameExit(cmd *exec.Cmd, sessionID string, gameID string, startTime time.Time) {
	_ = cmd.Wait()

	endTime := time.Now()
	duration := int(endTime.Sub(startTime).Seconds())

	// If play time is less than 1 minute, remove the temporary session record
	if duration < 60 {
		_, err := s.db.ExecContext(
			s.ctx,
			`DELETE FROM play_sessions WHERE id = ?`,
			sessionID,
		)
		if err != nil {
			runtime.LogErrorf(s.ctx, "Failed to delete short play session %s: %v", sessionID, err)
			fmt.Printf("Failed to delete short play session %s: %v\n", sessionID, err)
		}
		return
	}

	_, err := s.db.ExecContext(
		s.ctx,
		`UPDATE play_sessions
		 SET end_time = ?, duration = ?
		 WHERE id = ?`,
		endTime,
		duration,
		sessionID,
	)
	if err != nil {
		runtime.LogErrorf(s.ctx, "Failed to update play session %s: %v", sessionID, err)
		fmt.Printf("Failed to update play session %s: %v\n", sessionID, err)
		return
	}

	// 自动备份游戏存档
	if s.config.AutoBackupGameSave && s.backupService != nil {
		s.autoBackupGameSave(gameID)
	}
}

// autoBackupGameSave 自动备份游戏存档
func (s *TimerService) autoBackupGameSave(gameID string) {
	// 检查是否设置了存档目录
	var savePath string
	err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", gameID).Scan(&savePath)
	if err != nil || savePath == "" {
		runtime.LogDebugf(s.ctx, "Game %s has no save path configured, skipping auto backup", gameID)
		return
	}

	// 执行备份
	runtime.LogInfof(s.ctx, "Auto backing up game save for: %s", gameID)
	backup, err := s.backupService.CreateBackup(gameID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "Failed to auto backup game save: %v", err)
		return
	}

	// 如果启用了自动上传到云端
	if s.config.AutoUploadToCloud && s.config.CloudBackupEnabled && s.config.BackupUserID != "" {
		runtime.LogInfof(s.ctx, "Auto uploading backup to cloud: %s", backup.ID)
		err = s.backupService.UploadGameBackupToCloud(gameID, backup.ID)
		if err != nil {
			runtime.LogErrorf(s.ctx, "Failed to auto upload backup to cloud: %v", err)
		} else {
			runtime.LogInfof(s.ctx, "Successfully uploaded backup to cloud: %s", backup.ID)
		}
	}
	runtime.LogInfof(s.ctx, "Auto backup completed for game: %s", gameID)
}

func (s *TimerService) getGamePath(gameID string) (string, error) {
	var path string
	err := s.db.QueryRowContext(
		s.ctx,
		"SELECT COALESCE(path, '') FROM games WHERE id = ?",
		gameID,
	).Scan(&path)

	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("game not found: %s", gameID)
	}
	if err != nil {
		return "", err
	}

	return path, nil
}
