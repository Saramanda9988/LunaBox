package service

import (
	"context"
	"database/sql"
	"log"
	"lunabox/internal/utils"
	"sync"
	"time"
)

// TrackingSession 正在追踪的会话
type TrackingSession struct {
	SessionID          string
	GameID             string
	ProcessID          uint32
	StartTime          time.Time
	cancel             context.CancelFunc
	accumulatedSeconds int // 累加的活跃秒数
	mu                 sync.Mutex
}

// ActiveTimeTracker 活跃时间追踪服务
// 使用轮询方式检测窗口焦点状态，仅当游戏窗口处于前台时记录游玩时长
type ActiveTimeTracker struct {
	ctx      context.Context
	db       *sql.DB
	mu       sync.RWMutex
	sessions map[string]*TrackingSession // gameID -> session
}

// NewActiveTimeTracker 创建活跃时间追踪器
func NewActiveTimeTracker() *ActiveTimeTracker {
	return &ActiveTimeTracker{
		sessions: make(map[string]*TrackingSession),
	}
}

func (s *ActiveTimeTracker) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

// StartTracking 开始追踪指定游戏的活跃游玩时间
// sessionID: play_session 记录 ID
// processID: 游戏进程 ID
// returns: 追踪会话 ID 和可能的错误
func (s *ActiveTimeTracker) StartTracking(sessionID string, gameID string, processID uint32) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有追踪中的会话
	if _, exists := s.sessions[gameID]; exists {
		log.Printf("[ActiveTimeTracker] Game %s is already being tracked", gameID)
		return gameID, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	session := &TrackingSession{
		SessionID: sessionID,
		GameID:    gameID,
		ProcessID: processID,
		StartTime: time.Now().UTC(),
		cancel:    cancel,
	}
	s.sessions[gameID] = session

	// 启动追踪 goroutine
	go s.trackActiveTime(ctx, session)

	log.Printf("[ActiveTimeTracker] Started tracking for game %s (PID: %d)", gameID, processID)
	return gameID, nil
}

// StopTracking 停止追踪指定游戏，返回累加的活跃秒数
func (s *ActiveTimeTracker) StopTracking(gameID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[gameID]
	if !exists {
		return 0
	}

	session.cancel()
	delete(s.sessions, gameID)

	session.mu.Lock()
	accumulated := session.accumulatedSeconds
	session.mu.Unlock()

	log.Printf("[ActiveTimeTracker] Stopped tracking for game %s, accumulated %d seconds", gameID, accumulated)
	return accumulated
}

// trackActiveTime 追踪活跃时间（核心逻辑）
func (s *ActiveTimeTracker) trackActiveTime(ctx context.Context, session *TrackingSession) {
	// 创建焦点追踪器
	tracker := utils.NewFocusTracker(session.ProcessID)
	focusChan, err := tracker.Start()
	if err != nil {
		log.Printf("[ActiveTimeTracker] Failed to start focus tracker for game %s: %v", session.GameID, err)
		// 降级到轮询模式
		s.fallbackPolling(ctx, session)
		return
	}
	defer tracker.Stop()

	// 创建计时 ticker
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 创建定期校验 ticker (每 30 秒校验一次焦点状态，防止漏掉事件)
	validationTicker := time.NewTicker(30 * time.Second)
	defer validationTicker.Stop()

	// 获取当前焦点状态
	isFocused := tracker.IsFocused()

	for {
		select {
		case <-ctx.Done():
			// 上下文取消，结束追踪
			return

		case info, ok := <-focusChan:
			if !ok {
				// channel 关闭，结束追踪
				return
			}
			// 焦点状态发生变化,使用通知中的状态信息
			if info.IsFocused != isFocused {
				isFocused = info.IsFocused
				if isFocused {
					log.Printf("[ActiveTimeTracker] Game %s gained focus", session.GameID)
				} else {
					log.Printf("[ActiveTimeTracker] Game %s lost focus", session.GameID)
				}
			}

		case <-ticker.C:
			if isFocused {
				// 窗口有焦点，累加时间
				s.incrementPlayTime(session.GameID, 1)
			}

		case <-validationTicker.C:
			// 定期校验焦点状态（防止漏掉事件）
			currentFocus := tracker.IsFocused()
			if currentFocus != isFocused {
				isFocused = currentFocus
				log.Printf("[ActiveTimeTracker] Game %s focus state corrected to %v", session.GameID, isFocused)
			}
		}
	}
}

// fallbackPolling 降级轮询模式（当事件 Hook 失败时使用）
func (s *ActiveTimeTracker) fallbackPolling(ctx context.Context, session *TrackingSession) {
	log.Printf("[ActiveTimeTracker] Falling back to polling mode for game %s", session.GameID)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if utils.IsProcessFocused(session.ProcessID) {
				s.incrementPlayTime(session.GameID, 1)
			}
		}
	}
}

// incrementPlayTime 增加游玩时间（秒）- 仅在内存中累加
func (s *ActiveTimeTracker) incrementPlayTime(gameID string, seconds int) {
	// 获取当前会话
	s.mu.RLock()
	session, exists := s.sessions[gameID]
	s.mu.RUnlock()

	if !exists {
		log.Printf("[ActiveTimeTracker] No active session found for game %s", gameID)
		return
	}

	// 在内存中累加时长
	session.mu.Lock()
	session.accumulatedSeconds += seconds
	session.mu.Unlock()
}

// GetCurrentSession 获取当前追踪的会话信息
func (s *ActiveTimeTracker) GetCurrentSession(gameID string) (*TrackingSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[gameID]
	if !exists {
		return nil, false
	}
	return session, true
}

// GetAllActiveSessions 获取所有活跃的追踪会话
func (s *ActiveTimeTracker) GetAllActiveSessions() []*TrackingSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*TrackingSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// IsTracking 检查指定游戏是否正在追踪中
func (s *ActiveTimeTracker) IsTracking(gameID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.sessions[gameID]
	return exists
}
