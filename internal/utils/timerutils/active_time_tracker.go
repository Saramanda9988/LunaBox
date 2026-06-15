package timerutils

import (
	"context"
	"database/sql"
	"log"
	"lunabox/internal/applog"
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/timerutils/focusing"
	"sync"
	"time"
)

type ActiveTrackKind string

const (
	ActiveTrackDefault     ActiveTrackKind = ""
	ActiveTrackBundlePath  ActiveTrackKind = "bundle-path"
	ActiveTrackWineRootPID ActiveTrackKind = "wine-root-pid"
	ActiveTrackLauncherPID ActiveTrackKind = "launcher-pid"
)

type ActiveTrack struct {
	Kind        ActiveTrackKind
	BundlePath  string
	RootPID     uint32
	LauncherPID uint32
}

var (
	isBundlePathFocused    = focusing.IsBundlePathFocused
	getForegroundProcessID = focusing.GetForegroundProcessID
	getDescendantProcesses = processutils.GetDescendantProcesses
	isProcessFocused       = focusing.IsProcessFocused
)

// TrackingSession 正在追踪的会话
type TrackingSession struct {
	SessionID          string
	GameID             string
	ProcessID          uint32
	ActiveTrack        ActiveTrack
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

// NewActiveTimeTracker 创建活跃时间追踪器（内部服务，由 StartService 管理）
func NewActiveTimeTracker(ctx context.Context, db *sql.DB) *ActiveTimeTracker {
	return &ActiveTimeTracker{
		ctx:      ctx,
		db:       db,
		sessions: make(map[string]*TrackingSession),
	}
}

// StartTracking 开始追踪指定游戏的活跃游玩时间
// sessionID: play_session 记录 ID
// processID: 游戏进程 ID
// returns: 追踪会话 ID 和可能的错误
func (s *ActiveTimeTracker) StartTracking(sessionID string, gameID string, processID uint32) (string, error) {
	return s.StartTrackingWithActiveTrack(sessionID, gameID, processID, ActiveTrack{})
}

func (s *ActiveTimeTracker) StartTrackingWithActiveTrack(sessionID string, gameID string, processID uint32, activeTrack ActiveTrack) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有追踪中的会话
	if _, exists := s.sessions[gameID]; exists {
		applog.LogInfof(s.ctx, "[ActiveTimeTracker] Game %s is already being tracked", gameID)
		log.Printf("[ActiveTimeTracker] Game %s is already being tracked", gameID)
		return gameID, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	session := &TrackingSession{
		SessionID:   sessionID,
		GameID:      gameID,
		ProcessID:   processID,
		ActiveTrack: activeTrack,
		StartTime:   time.Now(),
		cancel:      cancel,
	}
	s.sessions[gameID] = session

	// 启动追踪 goroutine
	go s.trackActiveTime(ctx, session)

	applog.LogInfof(s.ctx, "[ActiveTimeTracker] Started tracking for game %s (PID: %d)", gameID, processID)
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

	applog.LogInfof(s.ctx, "[ActiveTimeTracker] Stopped tracking for game %s, accumulated %d seconds", gameID, accumulated)
	log.Printf("[ActiveTimeTracker] Stopped tracking for game %s, accumulated %d seconds", gameID, accumulated)
	return accumulated
}

// trackActiveTime 追踪活跃时间（核心逻辑）
func (s *ActiveTimeTracker) trackActiveTime(ctx context.Context, session *TrackingSession) {
	if session.ActiveTrack.Kind != ActiveTrackDefault {
		s.trackActiveTimeByPolling(ctx, session, true)
		return
	}

	// 创建焦点追踪器
	tracker := focusing.NewFocusTracker(session.ProcessID)
	focusChan, err := tracker.Start()
	if err != nil {
		applog.LogErrorf(s.ctx, "[ActiveTimeTracker] Failed to start focus tracker for game %s: %v", session.GameID, err)
		// 降级到轮询模式
		s.trackActiveTimeByPolling(ctx, session, false)
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
				s.logFocusChanged(session.GameID, isFocused)
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
				applog.LogInfof(s.ctx, "[ActiveTimeTracker] Game %s focus state corrected to %v", session.GameID, isFocused)
				log.Printf("[ActiveTimeTracker] Game %s focus state corrected to %v", session.GameID, isFocused)
			}
		}
	}
}

func (s *ActiveTimeTracker) trackActiveTimeByPolling(ctx context.Context, session *TrackingSession, strategyPolling bool) {
	if strategyPolling {
		applog.LogInfof(s.ctx, "[ActiveTimeTracker] Using active-track foreground polling mode for game %s (%s)", session.GameID, session.ActiveTrack.Kind)
		log.Printf("[ActiveTimeTracker] Using active-track foreground polling mode for game %s (%s)", session.GameID, session.ActiveTrack.Kind)
	} else {
		applog.LogInfof(s.ctx, "[ActiveTimeTracker] Falling back to polling mode for game %s", session.GameID)
		log.Printf("[ActiveTimeTracker] Falling back to polling mode for game %s", session.GameID)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	isFocused := s.isSessionFocused(session)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentFocus := s.isSessionFocused(session)
			if currentFocus != isFocused {
				isFocused = currentFocus
				s.logFocusChanged(session.GameID, isFocused)
			}
			if isFocused {
				s.incrementPlayTime(session.GameID, 1)
			}
		}
	}
}

func (s *ActiveTimeTracker) logFocusChanged(gameID string, isFocused bool) {
	if isFocused {
		applog.LogInfof(s.ctx, "[ActiveTimeTracker] Game %s gained focus", gameID)
		log.Printf("[ActiveTimeTracker] Game %s gained focus", gameID)
		return
	}
	applog.LogInfof(s.ctx, "[ActiveTimeTracker] Game %s lost focus", gameID)
	log.Printf("[ActiveTimeTracker] Game %s lost focus", gameID)
}

func (s *ActiveTimeTracker) isSessionFocused(session *TrackingSession) bool {
	switch session.ActiveTrack.Kind {
	case ActiveTrackBundlePath:
		return isBundlePathFocused(session.ActiveTrack.BundlePath)
	case ActiveTrackWineRootPID:
		foregroundPID, ok := getForegroundProcessID()
		if !ok {
			return false
		}
		rootPID := session.ActiveTrack.RootPID
		if rootPID == 0 {
			rootPID = session.ProcessID
		}
		if foregroundPID == rootPID {
			return true
		}
		descendants, err := getDescendantProcesses(rootPID)
		if err != nil {
			return false
		}
		for _, proc := range descendants {
			if proc.PID == foregroundPID {
				return true
			}
		}
		return false
	case ActiveTrackLauncherPID:
		launcherPID := session.ActiveTrack.LauncherPID
		if launcherPID == 0 {
			launcherPID = session.ProcessID
		}
		foregroundPID, ok := getForegroundProcessID()
		return ok && foregroundPID == launcherPID
	default:
		return isProcessFocused(session.ProcessID)
	}
}

// incrementPlayTime 增加游玩时间（秒）- 仅在内存中累加
func (s *ActiveTimeTracker) incrementPlayTime(gameID string, seconds int) {
	// 获取当前会话
	s.mu.RLock()
	session, exists := s.sessions[gameID]
	s.mu.RUnlock()

	if !exists {
		applog.LogInfof(s.ctx, "[ActiveTimeTracker] No active session found for game %s", gameID)
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

// StopAllTracking 停止所有正在追踪的会话（用于程序关闭时）
// 返回所有会话的 gameID 和累加秒数的映射
func (s *ActiveTimeTracker) StopAllTracking() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]int)

	for gameID, session := range s.sessions {
		session.cancel()

		session.mu.Lock()
		accumulated := session.accumulatedSeconds
		session.mu.Unlock()

		result[gameID] = accumulated

		applog.LogInfof(s.ctx, "[ActiveTimeTracker] Stopped tracking for game %s, accumulated %d seconds", gameID, accumulated)
		log.Printf("[ActiveTimeTracker] Stopped tracking for game %s, accumulated %d seconds", gameID, accumulated)
	}

	// 清空所有会话
	s.sessions = make(map[string]*TrackingSession)

	applog.LogInfof(s.ctx, "[ActiveTimeTracker] Stopped all tracking, cleaned up %d sessions", len(result))
	log.Printf("[ActiveTimeTracker] Stopped all tracking, cleaned up %d sessions", len(result))

	return result
}
