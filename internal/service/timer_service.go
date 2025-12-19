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
func (s *TimerService) StartGameWithTracking(gameID string) (bool, error) {
	//获取游戏路径
	path, err := s.getGamePath(gameID)
	if err != nil {
		return false, fmt.Errorf("failed to get game path: %w", err)
	}

	if path == "" {
		return false, fmt.Errorf("game path is empty for game: %s", gameID)
	}

	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
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

	go s.waitForGameExit(cmd, sessionID, startTime)

	// 启动成功，返回 true 给前端
	return true, nil
}

// waitForGameExit 等待游戏进程退出并更新游玩记录
func (s *TimerService) waitForGameExit(cmd *exec.Cmd, sessionID string, startTime time.Time) {
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
		fmt.Printf("Failed to update play session %s: %v\n", sessionID, err)
	}
}

func (s *TimerService) getGamePath(gameID string) (string, error) {
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
