package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/models"
	"time"

	"github.com/google/uuid"
)

type SessionService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewSessionService() *SessionService {
	return &SessionService{}
}

func (s *SessionService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

// CreatePendingSession 创建待完成的游戏会话（用于开始游戏时）
// 返回创建的会话ID
func (s *SessionService) CreatePendingSession(gameID string, startTime time.Time) (string, error) {
	sessionID := uuid.New().String()

	_, err := s.db.ExecContext(
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
		applog.LogErrorf(s.ctx, "CreatePendingSession: failed to create session: %v", err)
		return "", fmt.Errorf("创建游玩会话失败: %w", err)
	}

	return sessionID, nil
}

// AddPlaySession 手动添加游玩记录
// startTime: 开始时间
// durationMinutes: 游玩时长（分钟）
func (s *SessionService) AddPlaySession(gameID string, startTime time.Time, durationMinutes int) (models.PlaySession, error) {
	// 验证游戏是否存在
	var exists bool
	err := s.db.QueryRowContext(s.ctx, "SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)", gameID).Scan(&exists)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddPlaySession: failed to check game existence: %v", err)
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
		applog.LogErrorf(s.ctx, "AddPlaySession: failed to insert play session: %v", err)
		return models.PlaySession{}, fmt.Errorf("添加游玩记录失败: %w", err)
	}

	applog.LogInfof(s.ctx, "AddPlaySession: added play session for game %s, duration: %d minutes", gameID, durationMinutes)
	return session, nil
}

// GetPlaySessions 获取指定游戏的所有游玩记录
func (s *SessionService) GetPlaySessions(gameID string) ([]models.PlaySession, error) {
	rows, err := s.db.QueryContext(
		s.ctx,
		`SELECT id, game_id, start_time, COALESCE(end_time, start_time), duration 
		 FROM play_sessions 
		 WHERE game_id = ? 
		 ORDER BY start_time DESC`,
		gameID,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "GetPlaySessions: failed to query play sessions: %v", err)
		return nil, fmt.Errorf("查询游玩记录失败: %w", err)
	}
	defer rows.Close()

	var sessions []models.PlaySession
	for rows.Next() {
		var session models.PlaySession
		if err := rows.Scan(&session.ID, &session.GameID, &session.StartTime, &session.EndTime, &session.Duration); err != nil {
			applog.LogErrorf(s.ctx, "GetPlaySessions: failed to scan play session: %v", err)
			return nil, fmt.Errorf("读取游玩记录失败: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeletePlaySession 删除指定的游玩记录
func (s *SessionService) DeletePlaySession(sessionID string) error {
	result, err := s.db.ExecContext(s.ctx, "DELETE FROM play_sessions WHERE id = ?", sessionID)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeletePlaySession: failed to delete play session: %v", err)
		return fmt.Errorf("删除游玩记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("游玩记录不存在: %s", sessionID)
	}

	applog.LogInfof(s.ctx, "DeletePlaySession: deleted play session %s", sessionID)
	return nil
}

// UpdatePlaySession 更新游玩记录
func (s *SessionService) UpdatePlaySession(session models.PlaySession) error {
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
		applog.LogErrorf(s.ctx, "UpdatePlaySession: failed to update play session: %v", err)
		return fmt.Errorf("更新游玩记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("游玩记录不存在: %s", session.ID)
	}

	applog.LogInfof(s.ctx, "UpdatePlaySession: updated play session %s", session.ID)
	return nil
}

// BatchAddPlaySessions 批量添加游玩记录（用于导入）
func (s *SessionService) BatchAddPlaySessions(sessions []models.PlaySession) error {
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
			applog.LogErrorf(s.ctx, "BatchAddPlaySessions: failed to insert session: %v", err)
			return fmt.Errorf("插入游玩记录失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	applog.LogInfof(s.ctx, "BatchAddPlaySessions: added %d play sessions", len(sessions))
	return nil
}
