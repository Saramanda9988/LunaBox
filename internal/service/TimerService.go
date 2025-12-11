package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"os/exec"
	"time"

	"github.com/google/uuid"
)

type TimerService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewTimerService() *TimerService {
	return &TimerService{}
}

func (s *TimerService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

// StartGameWithTracking 启动游戏并自动追踪游玩时长
// 当游戏进程退出时，自动保存游玩记录到数据库
func (s *TimerService) StartGameWithTracking(userID, gameID string) error {
	//获取游戏路径
	path, err := s.GetGamePath(gameID)
	if err != nil {
		return fmt.Errorf("failed to get game path: %w", err)
	}

	if path == "" {
		return fmt.Errorf("game path is empty for game: %s", gameID)
	}

	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start game: %w", err)
	}

	sessionID := uuid.New().String()
	startTime := time.Now()

	_, err = s.db.ExecContext(
		s.ctx,
		`INSERT INTO play_sessions (id, user_id, game_id, start_time, end_time, duration)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID,
		userID,
		gameID,
		startTime,
		startTime, // 临时占位，等游戏结束后更新
		0,         // 初始时长为 0
	)
	if err != nil {
		return fmt.Errorf("failed to create play session: %w", err)
	}

	go s.waitForGameExit(cmd, sessionID, startTime)

	return nil
}

// waitForGameExit 等待游戏进程退出并更新游玩记录
func (s *TimerService) waitForGameExit(cmd *exec.Cmd, sessionID string, startTime time.Time) {
	_ = cmd.Wait()

	endTime := time.Now()
	duration := int(endTime.Sub(startTime).Seconds())

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
		fmt.Printf("Failed to update play session %s: %v\n", sessionID, err)
	}
}

func (s *TimerService) GetGamePath(gameID string) (string, error) {
	var path string
	err := s.db.QueryRowContext(
		s.ctx,
		"SELECT path FROM games WHERE id = ?",
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

// ReportPlaySession 手动上报一条完整的游玩记录
// 用于测试
func (s *TimerService) ReportPlaySession(session models.PlaySession) error {
	// 如果没有提供 ID，生成一个新的
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	// 如果没有提供时长，根据开始和结束时间计算
	if session.Duration == 0 && !session.EndTime.IsZero() && !session.StartTime.IsZero() {
		session.Duration = int(session.EndTime.Sub(session.StartTime).Seconds())
	}

	query := `INSERT INTO play_sessions (id, user_id, game_id, start_time, end_time, duration)
		      VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(s.ctx, query,
		session.ID,
		session.UserID,
		session.GameID,
		session.StartTime,
		session.EndTime,
		session.Duration,
	)

	if err != nil {
		return fmt.Errorf("failed to report play session: %w", err)
	}

	return nil
}

func (s *TimerService) GetPlaySessions(userID string) ([]models.PlaySession, error) {
	query := `SELECT id, user_id, game_id, start_time, end_time, duration
			  FROM play_sessions
			  WHERE user_id = ?
			  ORDER BY start_time DESC`

	rows, err := s.db.QueryContext(s.ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query play sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.PlaySession
	for rows.Next() {
		var session models.PlaySession
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.GameID,
			&session.StartTime,
			&session.EndTime,
			&session.Duration,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan play session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating play sessions: %w", err)
	}

	return sessions, nil
}

func (s *TimerService) GetPlaySessionsByGameID(gameID string) ([]models.PlaySession, error) {
	query := `SELECT id, user_id, game_id, start_time, end_time, duration
			  FROM play_sessions
			  WHERE game_id = ?
			  ORDER BY start_time DESC`

	rows, err := s.db.QueryContext(s.ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query play sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.PlaySession
	for rows.Next() {
		var session models.PlaySession
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.GameID,
			&session.StartTime,
			&session.EndTime,
			&session.Duration,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan play session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating play sessions: %w", err)
	}

	return sessions, nil
}

// GetTotalPlayTime 获取指定游戏的总游玩时长（秒）
func (s *TimerService) GetTotalPlayTime(gameID string) (int, error) {
	var totalDuration sql.NullInt64
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT SUM(duration) FROM play_sessions WHERE game_id = ?`,
		gameID,
	).Scan(&totalDuration)

	if err != nil {
		return 0, fmt.Errorf("failed to get total play time: %w", err)
	}

	if !totalDuration.Valid {
		return 0, nil
	}

	return int(totalDuration.Int64), nil
}
