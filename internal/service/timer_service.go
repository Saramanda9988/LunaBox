package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os/exec"
	"path/filepath"
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
	// 设置工作目录为游戏所在目录，确保汉化补丁等资源能正确加载
	cmd.Dir = filepath.Dir(path)
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

// AddPlaySession 手动添加游玩记录
// startTime: 开始时间
// durationMinutes: 游玩时长（分钟）
func (s *TimerService) AddPlaySession(gameID string, startTime time.Time, durationMinutes int) (models.PlaySession, error) {
	// 验证游戏是否存在
	var exists bool
	err := s.db.QueryRowContext(s.ctx, "SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)", gameID).Scan(&exists)
	if err != nil {
		runtime.LogErrorf(s.ctx, "AddPlaySession: failed to check game existence: %v", err)
		return models.PlaySession{}, fmt.Errorf("检查游戏是否存在失败: %w", err)
	}
	if !exists {
		return models.PlaySession{}, fmt.Errorf("游戏不存在: %s", gameID)
	}

	// 转换为秒
	durationSeconds := durationMinutes * 60
	endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

	session := models.PlaySession{
		ID:        uuid.New().String(),
		GameID:    gameID,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  durationSeconds,
	}

	_, err = s.db.ExecContext(
		s.ctx,
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration)
		 VALUES (?, ?, ?, ?, ?)`,
		session.ID,
		session.GameID,
		session.StartTime,
		session.EndTime,
		session.Duration,
	)
	if err != nil {
		runtime.LogErrorf(s.ctx, "AddPlaySession: failed to insert play session: %v", err)
		return models.PlaySession{}, fmt.Errorf("添加游玩记录失败: %w", err)
	}

	runtime.LogInfof(s.ctx, "AddPlaySession: added play session for game %s, duration: %d minutes", gameID, durationMinutes)
	return session, nil
}

// GetPlaySessions 获取指定游戏的所有游玩记录
func (s *TimerService) GetPlaySessions(gameID string) ([]models.PlaySession, error) {
	rows, err := s.db.QueryContext(
		s.ctx,
		`SELECT id, game_id, start_time, COALESCE(end_time, start_time), duration 
		 FROM play_sessions 
		 WHERE game_id = ? 
		 ORDER BY start_time DESC`,
		gameID,
	)
	if err != nil {
		runtime.LogErrorf(s.ctx, "GetPlaySessions: failed to query play sessions: %v", err)
		return nil, fmt.Errorf("查询游玩记录失败: %w", err)
	}
	defer rows.Close()

	var sessions []models.PlaySession
	for rows.Next() {
		var session models.PlaySession
		if err := rows.Scan(&session.ID, &session.GameID, &session.StartTime, &session.EndTime, &session.Duration); err != nil {
			runtime.LogErrorf(s.ctx, "GetPlaySessions: failed to scan play session: %v", err)
			return nil, fmt.Errorf("读取游玩记录失败: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeletePlaySession 删除指定的游玩记录
func (s *TimerService) DeletePlaySession(sessionID string) error {
	result, err := s.db.ExecContext(s.ctx, "DELETE FROM play_sessions WHERE id = ?", sessionID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "DeletePlaySession: failed to delete play session: %v", err)
		return fmt.Errorf("删除游玩记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("游玩记录不存在: %s", sessionID)
	}

	runtime.LogInfof(s.ctx, "DeletePlaySession: deleted play session %s", sessionID)
	return nil
}

// UpdatePlaySession 更新游玩记录
func (s *TimerService) UpdatePlaySession(session models.PlaySession) error {
	// 重新计算结束时间
	endTime := session.StartTime.Add(time.Duration(session.Duration) * time.Second)

	result, err := s.db.ExecContext(
		s.ctx,
		`UPDATE play_sessions SET start_time = ?, end_time = ?, duration = ? WHERE id = ?`,
		session.StartTime,
		endTime,
		session.Duration,
		session.ID,
	)
	if err != nil {
		runtime.LogErrorf(s.ctx, "UpdatePlaySession: failed to update play session: %v", err)
		return fmt.Errorf("更新游玩记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("游玩记录不存在: %s", session.ID)
	}

	runtime.LogInfof(s.ctx, "UpdatePlaySession: updated play session %s", session.ID)
	return nil
}

// BatchAddPlaySessions 批量添加游玩记录（用于导入）
func (s *TimerService) BatchAddPlaySessions(sessions []models.PlaySession) error {
	if len(sessions) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(s.ctx,
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	for _, session := range sessions {
		_, err = stmt.ExecContext(s.ctx, session.ID, session.GameID, session.StartTime, session.EndTime, session.Duration)
		if err != nil {
			runtime.LogErrorf(s.ctx, "BatchAddPlaySessions: failed to insert session: %v", err)
			return fmt.Errorf("插入游玩记录失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	runtime.LogInfof(s.ctx, "BatchAddPlaySessions: added %d play sessions", len(sessions))
	return nil
}
