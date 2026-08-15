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

//wails:ignore
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
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID,
		gameID,
		startTime,
		nil, // end_time 为 NULL 表示会话仍在进行中
		0,   // 初始时长为 0，后续由心跳定期保存
		time.Now(),
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "CreatePendingSession: failed to create session: %v", err)
		return "", fmt.Errorf("创建游玩会话失败: %w", err)
	}

	return sessionID, nil
}

// saveSessionHeartbeat 保存运行中会话的最新计时快照。
// end_time 为 NULL 是运行中会话的标记；心跳不得设置 end_time，也不得覆盖已经结束的会话。
func (s *SessionService) saveSessionHeartbeat(sessionID string, duration int, heartbeatAt time.Time) error {
	if duration < 0 {
		duration = 0
	}

	_, err := s.db.ExecContext(
		s.ctx,
		`UPDATE play_sessions
		 SET duration = ?, updated_at = ?
		 WHERE id = ? AND end_time IS NULL`,
		duration,
		heartbeatAt,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("保存游玩会话心跳失败: %w", err)
	}

	return nil
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
		UpdatedAt: time.Now(),
	}

	_, err = s.db.ExecContext(
		s.ctx,
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		session.ID,
		session.GameID,
		session.StartTime,
		session.EndTime,
		session.Duration,
		session.UpdatedAt,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddPlaySession: failed to insert play session: %v", err)
		return models.PlaySession{}, fmt.Errorf("添加游玩记录失败: %w", err)
	}

	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityPlaySession, session.ID); err != nil {
		applog.LogWarningf(s.ctx, "AddPlaySession: failed to clear play_session tombstone %s: %v", session.ID, err)
	}

	applog.LogInfof(s.ctx, "AddPlaySession: added play session for game %s, duration: %d minutes", gameID, durationMinutes)
	return session, nil
}

// GetPlaySessions 获取指定游戏的所有游玩记录
func (s *SessionService) GetPlaySessions(gameID string) ([]models.PlaySession, error) {
	rows, err := s.db.QueryContext(
		s.ctx,
		`SELECT id, game_id, start_time, COALESCE(end_time, start_time), duration, COALESCE(updated_at, end_time, start_time) 
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
		if err := rows.Scan(&session.ID, &session.GameID, &session.StartTime, &session.EndTime, &session.Duration, &session.UpdatedAt); err != nil {
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

	if err := upsertSyncTombstone(s.ctx, s.db, cloudSyncEntityPlaySession, sessionID, time.Now()); err != nil {
		return err
	}

	applog.LogInfof(s.ctx, "DeletePlaySession: deleted play session %s", sessionID)
	return nil
}

// UpdatePlaySession 更新游玩记录
func (s *SessionService) UpdatePlaySession(session models.PlaySession) error {
	// 重新计算结束时间
	endTime := session.StartTime.Add(time.Duration(session.Duration) * time.Second)
	session.UpdatedAt = time.Now()

	result, err := s.db.ExecContext(
		s.ctx,
		`UPDATE play_sessions SET start_time = ?, end_time = ?, duration = ?, updated_at = ? WHERE id = ?`,
		session.StartTime,
		endTime,
		session.Duration,
		session.UpdatedAt,
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

	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityPlaySession, session.ID); err != nil {
		applog.LogWarningf(s.ctx, "UpdatePlaySession: failed to clear play_session tombstone %s: %v", session.ID, err)
	}

	applog.LogInfof(s.ctx, "UpdatePlaySession: updated play session %s", session.ID)
	return nil
}

func (s *SessionService) completeUnfinishedSession(sessionID string, endTime time.Time, duration int) (bool, error) {
	if duration < 60 {
		_, err := s.db.ExecContext(s.ctx, "DELETE FROM play_sessions WHERE id = ?", sessionID)
		if err != nil {
			return true, fmt.Errorf("delete short unfinished session: %w", err)
		}
		if err := upsertSyncTombstone(s.ctx, s.db, cloudSyncEntityPlaySession, sessionID, endTime); err != nil {
			return true, err
		}
		return true, nil
	}

	result, err := s.db.ExecContext(
		s.ctx,
		`UPDATE play_sessions SET end_time = ?, duration = ?, updated_at = ? WHERE id = ?`,
		endTime,
		duration,
		endTime,
		sessionID,
	)
	if err != nil {
		return false, fmt.Errorf("update unfinished session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if rowsAffected == 0 {
		return false, fmt.Errorf("游玩记录不存在: %s", sessionID)
	}

	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityPlaySession, sessionID); err != nil {
		applog.LogWarningf(s.ctx, "completeUnfinishedSession: failed to clear play_session tombstone %s: %v", sessionID, err)
	}
	return false, nil
}

// completeUnfinishedSessionWithDuration 使用指定结束时间和时长完成一个未完成会话。
// duration 是实际应计入统计的秒数；在仅记录活跃窗口时，它可能小于墙钟时间。
func (s *SessionService) completeUnfinishedSessionWithDuration(sessionID string, endTime time.Time, duration int) error {
	_, err := s.completeUnfinishedSession(sessionID, endTime, duration)
	if err != nil {
		applog.LogErrorf(s.ctx, "completeUnfinishedSessionWithDuration: failed to complete session %s: %v", sessionID, err)
		return fmt.Errorf("完成游玩会话失败: %w", err)
	}
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
		`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	for _, session := range sessions {
		if session.UpdatedAt.IsZero() {
			session.UpdatedAt = time.Now()
		}
		_, err = stmt.ExecContext(s.ctx, session.ID, session.GameID, session.StartTime, session.EndTime, session.Duration, session.UpdatedAt)
		if err != nil {
			applog.LogErrorf(s.ctx, "BatchAddPlaySessions: failed to insert session: %v", err)
			return fmt.Errorf("插入游玩记录失败: %w", err)
		}
		if clearErr := deleteSyncTombstone(s.ctx, tx, cloudSyncEntityPlaySession, session.ID); clearErr != nil {
			return clearErr
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	applog.LogInfof(s.ctx, "BatchAddPlaySessions: added %d play sessions", len(sessions))
	return nil
}

// CleanupUnfinishedSessions 清理所有未完成的会话（程序启动或关闭时调用）。
// 新式运行中会话使用 end_time IS NULL 标记，并以最后一次心跳保存的
// duration/updated_at 恢复，避免把断电后的时间误算为游玩时间。
// 同时兼容旧版本使用 duration == 0 且 end_time == start_time 的待完成记录。
func (s *SessionService) CleanupUnfinishedSessions() error {
	rows, err := s.db.QueryContext(
		s.ctx,
		`SELECT
			id,
			game_id,
			start_time,
			COALESCE(duration, 0),
			COALESCE(updated_at, start_time),
			end_time IS NULL
		 FROM play_sessions
		 WHERE end_time IS NULL
			OR (COALESCE(duration, 0) = 0 AND end_time = start_time)`,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "CleanupUnfinishedSessions: failed to query unfinished sessions: %v", err)
		return fmt.Errorf("查询未完成会话失败: %w", err)
	}
	defer rows.Close()

	type unfinishedSession struct {
		ID              string
		GameID          string
		StartTime       time.Time
		Duration        int
		LastHeartbeatAt time.Time
		IsRunning       bool
	}

	var sessions []unfinishedSession
	for rows.Next() {
		var session unfinishedSession
		if err := rows.Scan(
			&session.ID,
			&session.GameID,
			&session.StartTime,
			&session.Duration,
			&session.LastHeartbeatAt,
			&session.IsRunning,
		); err != nil {
			applog.LogErrorf(s.ctx, "CleanupUnfinishedSessions: failed to scan session: %v", err)
			continue
		}
		sessions = append(sessions, session)
	}

	if len(sessions) == 0 {
		applog.LogInfof(s.ctx, "CleanupUnfinishedSessions: no unfinished sessions found")
		return nil
	}

	applog.LogInfof(s.ctx, "CleanupUnfinishedSessions: found %d unfinished sessions", len(sessions))

	// 处理每个未完成的会话。旧式记录没有心跳快照，只能沿用原来的墙钟恢复方式。
	cleanupTime := time.Now()
	var deleted, updated int

	for _, session := range sessions {
		endTime := session.LastHeartbeatAt
		duration := session.Duration
		if !session.IsRunning {
			endTime = cleanupTime
			duration = int(endTime.Sub(session.StartTime).Seconds())
		}
		if endTime.Before(session.StartTime) {
			endTime = session.StartTime
		}

		sessionDeleted, err := s.completeUnfinishedSession(session.ID, endTime, duration)
		if err != nil {
			applog.LogErrorf(s.ctx, "CleanupUnfinishedSessions: failed to complete session %s: %v", session.ID, err)
			continue
		}
		if sessionDeleted {
			deleted++
			applog.LogDebugf(s.ctx, "Deleted short session %s (duration: %d seconds)", session.ID, duration)
		} else {
			updated++
			applog.LogDebugf(s.ctx, "Updated unfinished session %s (duration: %d seconds)", session.ID, duration)
		}
	}

	applog.LogInfof(s.ctx, "CleanupUnfinishedSessions: deleted %d short sessions, updated %d sessions", deleted, updated)
	return nil
}
