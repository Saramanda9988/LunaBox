package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/timerutils"
	"lunabox/internal/utils/timerutils/focusing"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type protocolLaunchErrorEvent struct {
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	GameID  string `json:"game_id,omitempty"`
}

const homeRefreshRequestedEvent = "home:refresh-requested"

// LaunchOptions 定义游戏启动选项
type LaunchOptions struct {
	UseLocaleEmulator *bool // 是否使用 Locale Emulator，nil 表示使用游戏配置
	UseMagpie         *bool // 是否使用 Magpie，nil 表示使用游戏配置
}

type StartService struct {
	ctx               context.Context
	config            *appconf.AppConfig
	backupService     *BackupService
	gameService       *GameService
	sessionService    *SessionService
	activeTimeTracker *timerutils.ActiveTimeTracker

	// 进程选择相关
	pendingProcessSelect   map[string]chan string // gameID -> channel，用于接收用户选择的进程名
	pendingProcessSelectMu sync.RWMutex
}

type launchedProcess struct {
	PID    uint32
	Name   string
	Handle uintptr
}

func NewStartService() *StartService {
	return &StartService{
		pendingProcessSelect: make(map[string]chan string),
		// activeTimeTracker 将在 Init 时创建
	}
}

func (s *StartService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	// db 不再使用，但保留参数以保持与其他服务的接口一致性
	s.config = config
	// 初始化内部服务
	s.activeTimeTracker = timerutils.NewActiveTimeTracker(ctx, db)
	// 确保 map 已初始化
	if s.pendingProcessSelect == nil {
		s.pendingProcessSelect = make(map[string]chan string)
	}
}

// SetBackupService 设置备份服务（用于自动备份）
func (s *StartService) SetBackupService(backupService *BackupService) {
	s.backupService = backupService
}

// SetGameService 设置游戏服务（用于获取游戏信息）
func (s *StartService) SetGameService(gameService *GameService) {
	s.gameService = gameService
}

// SetSessionService 设置会话服务（用于管理游玩记录）
func (s *StartService) SetSessionService(sessionService *SessionService) {
	s.sessionService = sessionService
}

// StartGameWithTracking 启动游戏并自动追踪游玩时长
// 当游戏进程退出时，自动保存游玩记录到数据库
func (s *StartService) StartGameWithTracking(gameID string) (bool, error) {
	return s.startGame(gameID, LaunchOptions{})
}

// StartGameWithOptions 使用指定选项启动游戏
// 供 CLI 调用，支持覆盖 LE 和 Magpie 设置
func (s *StartService) StartGameWithOptions(gameID string, options LaunchOptions) (bool, error) {
	return s.startGame(gameID, options)
}

// HandleProtocolLaunch validates and dispatches a protocol-triggered game launch.
func (s *StartService) HandleProtocolLaunch(req vo.ProtocolLaunchRequest) error {
	gameID := strings.TrimSpace(req.GameID)
	if gameID == "" {
		err := fmt.Errorf("missing required parameter: game_id")
		s.emitProtocolLaunchError("快捷启动失败", err.Error(), "")
		return err
	}

	if s.gameService == nil {
		err := fmt.Errorf("game service is not initialized")
		s.emitProtocolLaunchError("快捷启动失败", err.Error(), gameID)
		return err
	}

	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to resolve target game: %w", err)
		s.emitProtocolLaunchError("未找到该游戏快捷方式对应的游戏记录", err.Error(), gameID)
		return wrappedErr
	}

	started, err := s.StartGameWithTracking(gameID)
	if err != nil {
		wrappedErr := fmt.Errorf("start game via protocol: %w", err)
		s.emitProtocolLaunchError(fmt.Sprintf("启动《%s》失败", game.Name), err.Error(), gameID)
		return wrappedErr
	}
	if !started {
		err := fmt.Errorf("game failed to start")
		s.emitProtocolLaunchError(fmt.Sprintf("启动《%s》失败", game.Name), err.Error(), gameID)
		return err
	}

	return nil
}

// startGame 内部启动方法，支持通过 options 覆盖配置
func (s *StartService) startGame(gameID string, options LaunchOptions) (bool, error) {
	// 获取游戏路径和进程配置
	path, processName, err := s.getGamePathAndProcess(gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("failed to get game path: %w", err)
	}

	// 如果未配置路径或配置的是文件夹，则在首次启动时要求用户选择可执行文件并写回游戏路径
	resolvedPath, resolvedProcessName, cancelled, err := s.resolveExecutablePath(gameID, path, processName)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to resolve executable path: %v", err)
		return false, fmt.Errorf("failed to resolve executable path: %w", err)
	}
	if cancelled {
		applog.LogInfof(s.ctx, "user cancelled executable selection for game: %s", gameID)
		return false, nil
	}
	path = resolvedPath
	if strings.TrimSpace(resolvedProcessName) != "" {
		processName = resolvedProcessName
	}
	if strings.TrimSpace(processName) == "" {
		processName = filepath.Base(path)
	}

	// 获取启动exe的名称
	launcherExeName := filepath.Base(path)

	// 获取游戏的启动配置
	defaultUseLE, defaultUseMagpie, err := s.getGameLaunchConfig(gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get launch config: %v", err)
		return false, fmt.Errorf("failed to get launch config: %w", err)
	}

	// 确定最终配置：优先使用 options 中的设置（如果非 nil），否则使用游戏默认配置
	useLE := defaultUseLE
	if options.UseLocaleEmulator != nil {
		useLE = *options.UseLocaleEmulator
	}

	useMagpie := defaultUseMagpie
	if options.UseMagpie != nil {
		useMagpie = *options.UseMagpie
	}

	var launchFile string
	var launchArgs []string
	var launchDir string

	// 如果启用了 Locale Emulator
	if useLE && s.config.LocaleEmulatorPath != "" {
		applog.LogInfof(s.ctx, "Starting game with Locale Emulator: %s", gameID)
		launchFile = s.config.LocaleEmulatorPath
		launchArgs = []string{path}
		launchDir = filepath.Dir(path)
	} else {
		// 普通启动
		launchFile = path
		launchDir = filepath.Dir(path)
	}
	startedProcess, err := processutils.StartProcess(launchFile, launchArgs, launchDir)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to start game: %v", err)
		return false, fmt.Errorf("failed to start game: %w", err)
	}

	// 如果启用了 Magpie，在游戏启动后启动 Magpie
	if useMagpie && s.config.MagpiePath != "" {
		go s.startMagpie()
	}

	launcher := launchedProcess{
		PID:    startedProcess.PID,
		Name:   filepath.Base(launchFile),
		Handle: startedProcess.Handle,
	}

	startTime := time.Now()
	sessionID, err := s.sessionService.CreatePendingSession(gameID, startTime)
	if err != nil {
		processutils.CloseProcessHandle(startedProcess.Handle)
		return false, fmt.Errorf("failed to create play session: %w", err)
	}

	// 启动进程检测和监控 goroutine
	go s.detectAndMonitorProcess(sessionID, gameID, startTime, launcher, launcherExeName, filepath.Dir(path), processName, useLE)

	// pending session 已创建，Home 数据已发生变化，立即通知前端刷新
	s.requestHomeRefresh()

	// 启动成功，返回 true 给前端
	return true, nil
}

// detectAndMonitorProcess 检测实际游戏进程并开始监控。
// 采用分阶段检测策略，利用60秒会话记录阈值提供的余裕时间。
// usedLE: 是否使用了 Locale Emulator 启动，影响进程检测策略。
func (s *StartService) detectAndMonitorProcess(sessionID string, gameID string, startTime time.Time, launcher launchedProcess, launcherExeName string, launchDir string, savedProcessName string, usedLE bool) {
	var actualProcessID uint32
	var actualProcessName string
	var needExternalMonitor bool // 是否需要外部进程监控（非cmd子进程）
	useLauncherHandle := false

	// 情况1: 已保存了process_name，且与启动器exe名称不同
	// 说明之前用户已选择过实际的游戏进程
	if hasReliableSavedProcessName(savedProcessName, launcherExeName) {
		applog.LogInfof(s.ctx, "Game %s has saved process_name: %s, will search for it after initial delay", gameID, savedProcessName)

		// 等待5秒，给启动器时间启动实际游戏
		time.Sleep(5 * time.Second)

		// 在系统进程中搜索保存的进程名
		pid, err := processutils.GetProcessPIDByName(savedProcessName)
		if err != nil {
			applog.LogWarningf(s.ctx, "Failed to find saved process %s: %v, falling back to launcher monitoring", savedProcessName, err)
			if !s.isLauncherRunning(launcher) {
				s.closeLauncherHandle(launcher)
				if s.monitorDetectedGameProcess(sessionID, gameID, startTime, launcher, launcherExeName, launchDir) {
					return
				}
				s.promptUserToSelectProcess(sessionID, gameID, startTime, launcherExeName)
				return
			}
			// 如果找不到保存的进程且启动器仍在运行，使用启动器进程监控
			actualProcessID = launcher.PID
			actualProcessName = launcher.Name
			needExternalMonitor = false
			useLauncherHandle = true
		} else {
			actualProcessID = pid
			actualProcessName = savedProcessName
			needExternalMonitor = true // 需要外部监控，因为这不是cmd的子进程
			applog.LogInfof(s.ctx, "Found saved process %s with PID %d", savedProcessName, pid)
		}
	} else {
		// 情况2: 没有保存的process_name，或process_name与启动器相同

		// 检查是否启用自动进程检测
		if !s.config.AutoDetectGameProcess {
			// 用户禁用了自动检测，直接使用启动器进程
			applog.LogInfof(s.ctx, "Auto-detect disabled for game %s, using launcher process: %s (PID %d)", gameID, launcher.Name, launcher.PID)
			actualProcessID = launcher.PID
			actualProcessName = launcher.Name
			needExternalMonitor = false
			useLauncherHandle = true

			// 保存进程名
			if shouldPersistLauncherProcessName(savedProcessName) {
				s.updateGameProcessName(gameID, launcherExeName)
			}
		} else {
			// 启用了自动检测，使用分阶段检测策略来准确判断启动器类型
			applog.LogInfof(s.ctx, "Starting staged detection for game %s, launcher: %s (PID %d)", gameID, launcher.Name, launcher.PID)

			// 阶段1: 初始等待5秒，让启动器有时间启动实际游戏
			time.Sleep(5 * time.Second)

			if s.monitorVisibleGameProcess(sessionID, gameID, startTime, launcher, launcherExeName, launchDir) {
				return
			}

			// 阶段2: 第一次检测
			launcherStillRunning := s.isLauncherRunning(launcher)

			if !launcherStillRunning {
				// 启动器在5秒内就退出了，说明是快速启动器（如Steam）
				applog.LogInfof(s.ctx, "Launcher %s exited quickly (within 5s), resolving actual game process", launcherExeName)
				s.closeLauncherHandle(launcher)
				if s.monitorDetectedGameProcess(sessionID, gameID, startTime, launcher, launcherExeName, launchDir) {
					return
				}
				s.promptUserToSelectProcess(sessionID, gameID, startTime, launcherExeName)
				return
			}

			// 阶段3: 启动器还在运行，进入观察期（15秒）
			// 每2秒检查一次，看启动器是否会退出
			applog.LogInfof(s.ctx, "Launcher %s still running, entering observation period (15s)", launcherExeName)

			observationPeriod := 15 * time.Second
			checkInterval := 2 * time.Second
			observationStart := time.Now()

			for time.Since(observationStart) < observationPeriod {
				time.Sleep(checkInterval)

				if s.monitorVisibleGameProcess(sessionID, gameID, startTime, launcher, launcherExeName, launchDir) {
					return
				}

				if !s.isLauncherRunning(launcher) {
					// 启动器在观察期内退出了，说明它只是个启动器
					applog.LogInfof(s.ctx, "Launcher %s exited during observation period, resolving actual game process", launcherExeName)
					s.closeLauncherHandle(launcher)
					if s.monitorDetectedGameProcess(sessionID, gameID, startTime, launcher, launcherExeName, launchDir) {
						return
					}
					s.promptUserToSelectProcess(sessionID, gameID, startTime, launcherExeName)
					return
				}
			}

			// 阶段4: 观察期结束，启动器仍在运行
			// 说明启动器本身就是游戏进程（如普通单exe游戏）
			applog.LogInfof(s.ctx, "Launcher %s still running after 20s total, treating it as the game process", launcherExeName)
			actualProcessID = launcher.PID
			actualProcessName = launcher.Name
			needExternalMonitor = false
			useLauncherHandle = true

			// 保存进程名
			if shouldPersistLauncherProcessName(savedProcessName) {
				s.updateGameProcessName(gameID, launcherExeName)
			}
		}
	}

	// 启动活跃时间追踪（如果启用）
	s.startActiveTimeTracking(sessionID, gameID, actualProcessID)

	// 根据情况选择监控方式
	if needExternalMonitor {
		// 需要外部监控：实际游戏进程不是cmd的子进程
		s.closeLauncherHandle(launcher)
		s.monitorProcessByPID(sessionID, gameID, startTime, actualProcessID, actualProcessName)
	} else if useLauncherHandle && launcher.Handle != 0 {
		s.monitorProcessByHandle(sessionID, gameID, startTime, actualProcessID, actualProcessName, launcher.Handle)
	} else {
		s.monitorProcessByPID(sessionID, gameID, startTime, actualProcessID, actualProcessName)
	}
}

func (s *StartService) monitorDetectedGameProcess(sessionID string, gameID string, startTime time.Time, launcher launchedProcess, launcherExeName string, launchDir string) bool {
	actualProcessID, actualProcessName, ok := s.detectLaunchedDescendantProcess(gameID, launcher.PID, launcherExeName)
	if !ok {
		actualProcessID, actualProcessName, ok = s.detectProcessInLaunchDir(gameID, launchDir, launcher.PID)
	}
	if !ok {
		return false
	}

	s.persistDetectedProcessName(gameID, launcherExeName, actualProcessName)

	s.startActiveTimeTracking(sessionID, gameID, actualProcessID)
	s.monitorProcessByPID(sessionID, gameID, startTime, actualProcessID, actualProcessName)
	return true
}

func (s *StartService) monitorVisibleGameProcess(sessionID string, gameID string, startTime time.Time, launcher launchedProcess, launcherExeName string, launchDir string) bool {
	actualProcessID, actualProcessName, ok := s.detectVisibleLaunchedDescendantProcess(gameID, launcher.PID, launcherExeName)
	if !ok {
		actualProcessID, actualProcessName, ok = s.detectVisibleProcessInLaunchDir(gameID, launchDir, launcher.PID)
	}
	if !ok || actualProcessID == launcher.PID {
		return false
	}

	s.persistDetectedProcessName(gameID, launcherExeName, actualProcessName)

	applog.LogInfof(s.ctx, "Detected visible game window process for game %s: %s (PID %d), launcher: %s (PID %d)", gameID, actualProcessName, actualProcessID, launcherExeName, launcher.PID)
	s.closeLauncherHandle(launcher)
	s.startActiveTimeTracking(sessionID, gameID, actualProcessID)
	s.monitorProcessByPID(sessionID, gameID, startTime, actualProcessID, actualProcessName)
	return true
}

func (s *StartService) detectVisibleLaunchedDescendantProcess(gameID string, launcherPID uint32, launcherExeName string) (uint32, string, bool) {
	descendants, err := processutils.GetDescendantProcesses(launcherPID)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to enumerate visible descendant processes for launcher %s (PID %d): %v", launcherExeName, launcherPID, err)
		return 0, "", false
	}
	if len(descendants) == 0 {
		return 0, "", false
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if isLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}
	if len(candidates) == 0 {
		return 0, "", false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return 0, "", false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID {
				applog.LogInfof(s.ctx, "Auto-detected foreground visible-window descendant process for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
				return proc.PID, proc.Name, true
			}
		}
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		applog.LogInfof(s.ctx, "Auto-detected visible-window descendant process for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
		return proc.PID, proc.Name, true
	}

	applog.LogInfof(s.ctx, "Multiple visible-window descendant candidates found for game %s, requiring more evidence: %s", gameID, formatProcessCandidates(windowCandidates))
	return 0, "", false
}

func (s *StartService) detectLaunchedDescendantProcess(gameID string, launcherPID uint32, launcherExeName string) (uint32, string, bool) {
	descendants, err := processutils.GetDescendantProcesses(launcherPID)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to enumerate descendant processes for launcher %s (PID %d): %v", launcherExeName, launcherPID, err)
		return 0, "", false
	}
	if len(descendants) == 0 {
		applog.LogInfof(s.ctx, "No descendant process found for launcher %s (PID %d)", launcherExeName, launcherPID)
		return 0, "", false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range descendants {
			if proc.PID == foregroundPID && !isLikelyHelperProcess(proc.Name) {
				applog.LogInfof(s.ctx, "Auto-detected foreground descendant process for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
				return proc.PID, proc.Name, true
			}
		}
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if isLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		applog.LogInfof(s.ctx, "Auto-detected visible-window descendant process for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
		return proc.PID, proc.Name, true
	}

	if len(candidates) == 1 {
		proc := candidates[0]
		if isPersistableProcessName(proc.Name) {
			applog.LogInfof(s.ctx, "Auto-detected stable descendant process for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
			return proc.PID, proc.Name, true
		}
		applog.LogInfof(s.ctx, "Single non-exe descendant process found for game %s without visible window, requiring manual selection: %s", gameID, formatProcessCandidates(candidates))
		return 0, "", false
	}

	if len(candidates) > 1 {
		applog.LogInfof(s.ctx, "Multiple descendant process candidates found for game %s, requiring manual selection: %s", gameID, formatProcessCandidates(candidates))
	} else {
		applog.LogInfof(s.ctx, "Only helper descendant processes found for game %s, requiring manual selection: %s", gameID, formatProcessCandidates(descendants))
	}
	return 0, "", false
}

func (s *StartService) detectVisibleProcessInLaunchDir(gameID string, launchDir string, launcherPID uint32) (uint32, string, bool) {
	candidates, err := s.launchDirProcessCandidates(gameID, launchDir)
	if err != nil || len(candidates) == 0 {
		return 0, "", false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return 0, "", false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID && proc.PID != launcherPID {
				applog.LogInfof(s.ctx, "Auto-detected foreground visible-window process in launch dir for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
				return proc.PID, proc.Name, true
			}
		}
	}

	nonLauncherWindowCandidates := make([]processutils.ProcessInfo, 0, len(windowCandidates))
	for _, proc := range windowCandidates {
		if proc.PID == launcherPID {
			continue
		}
		nonLauncherWindowCandidates = append(nonLauncherWindowCandidates, proc)
	}

	if len(nonLauncherWindowCandidates) == 1 {
		proc := nonLauncherWindowCandidates[0]
		applog.LogInfof(s.ctx, "Auto-detected visible-window process in launch dir for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
		return proc.PID, proc.Name, true
	}

	if len(nonLauncherWindowCandidates) > 1 {
		applog.LogInfof(s.ctx, "Multiple visible-window launch dir candidates found for game %s, requiring more evidence: %s", gameID, formatProcessCandidates(nonLauncherWindowCandidates))
		return 0, "", false
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		applog.LogInfof(s.ctx, "Only launcher has a visible window for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
		return proc.PID, proc.Name, true
	}

	return 0, "", false
}

func (s *StartService) detectProcessInLaunchDir(gameID string, launchDir string, launcherPID uint32) (uint32, string, bool) {
	candidates, err := s.launchDirProcessCandidates(gameID, launchDir)
	if err != nil || len(candidates) == 0 {
		return 0, "", false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range candidates {
			if proc.PID == foregroundPID && !isLikelyHelperProcess(proc.Name) {
				applog.LogInfof(s.ctx, "Auto-detected foreground process in launch dir for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
				return proc.PID, proc.Name, true
			}
		}
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		applog.LogInfof(s.ctx, "Auto-detected visible-window process in launch dir for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
		return proc.PID, proc.Name, true
	}

	nonLauncherWindowCandidates := make([]processutils.ProcessInfo, 0, len(windowCandidates))
	for _, proc := range windowCandidates {
		if proc.PID == launcherPID {
			continue
		}
		nonLauncherWindowCandidates = append(nonLauncherWindowCandidates, proc)
	}
	if len(nonLauncherWindowCandidates) == 1 {
		proc := nonLauncherWindowCandidates[0]
		applog.LogInfof(s.ctx, "Auto-detected non-launcher visible-window process in launch dir for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
		return proc.PID, proc.Name, true
	}

	if len(candidates) == 1 {
		proc := candidates[0]
		if isPersistableProcessName(proc.Name) {
			applog.LogInfof(s.ctx, "Auto-detected stable process in launch dir for game %s: %s (PID %d)", gameID, proc.Name, proc.PID)
			return proc.PID, proc.Name, true
		}
		applog.LogInfof(s.ctx, "Single non-exe launch dir process found for game %s without visible window, requiring manual selection: %s", gameID, formatProcessCandidates(candidates))
		return 0, "", false
	}

	if len(candidates) > 1 {
		applog.LogInfof(s.ctx, "Multiple launch dir process candidates found for game %s, requiring manual selection: %s", gameID, formatProcessCandidates(candidates))
	}
	return 0, "", false
}

func (s *StartService) launchDirProcessCandidates(gameID string, launchDir string) ([]processutils.ProcessInfo, error) {
	candidates, err := processutils.GetProcessesByExecutableDir(launchDir)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to enumerate processes in launch dir %s for game %s: %v", launchDir, gameID, err)
		return nil, err
	}
	if len(candidates) == 0 {
		applog.LogInfof(s.ctx, "No running process found in launch dir %s for game %s", launchDir, gameID)
		return nil, nil
	}

	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, proc := range candidates {
		if isLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}

	if len(filtered) == 0 {
		applog.LogInfof(s.ctx, "Only helper processes found in launch dir for game %s, requiring manual selection: %s", gameID, formatProcessCandidates(candidates))
	}
	return filtered, nil
}

func (s *StartService) isLauncherRunning(launcher launchedProcess) bool {
	return processutils.IsProcessPresentByPID(launcher.PID)
}

func (s *StartService) closeLauncherHandle(launcher launchedProcess) {
	if launcher.Handle == 0 {
		return
	}
	if err := processutils.CloseProcessHandle(launcher.Handle); err != nil {
		applog.LogWarningf(s.ctx, "Failed to close launcher process handle for %s (PID %d): %v", launcher.Name, launcher.PID, err)
	}
}

func (s *StartService) startActiveTimeTracking(sessionID string, gameID string, processID uint32) {
	if !s.config.RecordActiveTimeOnly {
		return
	}

	_, err := s.activeTimeTracker.StartTracking(sessionID, gameID, processID)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to start active time tracking: %v", err)
	}
}

func (s *StartService) persistDetectedProcessName(gameID string, launcherExeName string, detectedProcessName string) {
	processName := processNameForPersistence(launcherExeName, detectedProcessName)
	if processName == "" {
		return
	}
	if !strings.EqualFold(processName, strings.TrimSpace(detectedProcessName)) {
		applog.LogInfof(s.ctx, "Detected transient wrapper process for game %s: %s, persisting stable launcher name: %s", gameID, detectedProcessName, processName)
	}
	if err := s.updateGameProcessName(gameID, processName); err != nil {
		applog.LogWarningf(s.ctx, "Failed to update auto-detected process name for game %s: %v", gameID, err)
	}
}

func (s *StartService) persistSelectedProcessName(gameID string, selectedProcessName string) {
	if !isPersistableProcessName(selectedProcessName) {
		applog.LogInfof(s.ctx, "Selected non-exe process for game %s will not be persisted as process_name: %s", gameID, selectedProcessName)
		return
	}
	if err := s.updateGameProcessName(gameID, selectedProcessName); err != nil {
		applog.LogWarningf(s.ctx, "Failed to update selected process name for game %s: %v", gameID, err)
	}
}

func hasReliableSavedProcessName(savedProcessName string, launcherExeName string) bool {
	saved := strings.TrimSpace(savedProcessName)
	if saved == "" || strings.EqualFold(saved, strings.TrimSpace(launcherExeName)) {
		return false
	}
	return isPersistableProcessName(saved)
}

func shouldPersistLauncherProcessName(savedProcessName string) bool {
	saved := strings.TrimSpace(savedProcessName)
	return saved == "" || !isPersistableProcessName(saved)
}

func processNameForPersistence(launcherExeName string, detectedProcessName string) string {
	detected := strings.TrimSpace(detectedProcessName)
	if isPersistableProcessName(detected) {
		return detected
	}
	launcher := strings.TrimSpace(launcherExeName)
	if isPersistableProcessName(launcher) {
		return launcher
	}
	return ""
}

func isPersistableProcessName(processName string) bool {
	name := strings.ToLower(strings.TrimSpace(processName))
	return strings.HasSuffix(name, ".exe")
}

func isLikelyHelperProcess(processName string) bool {
	name := strings.ToLower(strings.TrimSpace(processName))
	if name == "" {
		return true
	}
	switch name {
	case "conhost.exe",
		"crashpad_handler.exe",
		"crashreporter.exe",
		"cef_server.exe",
		"cefsharp.browsersubprocess.exe",
		"werfault.exe":
		return true
	default:
		return false
	}
}

func formatProcessCandidates(processes []processutils.ProcessInfo) string {
	if len(processes) == 0 {
		return "(none)"
	}

	parts := make([]string, 0, len(processes))
	for _, proc := range processes {
		parts = append(parts, fmt.Sprintf("%s(PID %d)", proc.Name, proc.PID))
	}
	return strings.Join(parts, ", ")
}

func (s *StartService) emitProtocolLaunchError(message string, detail string, gameID string) {
	if s.ctx == nil {
		return
	}
	runtime.EventsEmit(s.ctx, "protocol-launch:error", protocolLaunchErrorEvent{
		Message: strings.TrimSpace(message),
		Detail:  strings.TrimSpace(detail),
		GameID:  strings.TrimSpace(gameID),
	})
}

// promptUserToSelectProcess 提示用户选择实际的游戏进程
func (s *StartService) promptUserToSelectProcess(sessionID string, gameID string, startTime time.Time, launcherExeName string) {
	// 创建等待用户选择的 channel
	selectChan := make(chan string, 1)
	s.pendingProcessSelectMu.Lock()
	s.pendingProcessSelect[gameID] = selectChan
	s.pendingProcessSelectMu.Unlock()

	// 发送事件通知前端弹出进程选择窗口
	runtime.EventsEmit(s.ctx, "process-select-required", map[string]interface{}{
		"gameID":          gameID,
		"sessionID":       sessionID,
		"launcherExeName": launcherExeName,
	})

	// 等待用户选择（最多等待5分钟）
	var selectedProcess string
	var ok bool
	select {
	case selectedProcess, ok = <-selectChan:
		if !ok {
			// channel 被关闭，说明用户取消了选择
			applog.LogInfof(s.ctx, "User cancelled process selection for game %s", gameID)
			s.sessionService.DeletePlaySession(sessionID)
			return
		}
		// 用户已选择进程
		applog.LogInfof(s.ctx, "User selected process: %s for game %s", selectedProcess, gameID)
	case <-time.After(5 * time.Minute):
		// 超时未选择
		applog.LogWarningf(s.ctx, "Process selection timeout for game %s, cleaning up session", gameID)
		s.pendingProcessSelectMu.Lock()
		delete(s.pendingProcessSelect, gameID)
		s.pendingProcessSelectMu.Unlock()
		s.sessionService.DeletePlaySession(sessionID)
		return
	}

	// 清理 channel
	s.pendingProcessSelectMu.Lock()
	delete(s.pendingProcessSelect, gameID)
	s.pendingProcessSelectMu.Unlock()

	// 获取选中进程的PID
	pid, err := processutils.GetProcessPIDByName(selectedProcess)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to find selected process %s: %v", selectedProcess, err)
		s.sessionService.DeletePlaySession(sessionID)
		return
	}

	// 保存用户选择的进程名
	s.persistSelectedProcessName(gameID, selectedProcess)

	// 启动活跃时间追踪（如果启用）
	s.startActiveTimeTracking(sessionID, gameID, pid)

	// 监控选中的进程
	s.monitorProcessByPID(sessionID, gameID, startTime, pid, selectedProcess)
}

// monitorProcessByPID 通过PID监控外部进程直到退出
// 优先使用 WaitForSingleObject；权限不足时退回进程快照轮询。
func (s *StartService) monitorProcessByPID(sessionID string, gameID string, startTime time.Time, processID uint32, processName string) {
	applog.LogInfof(s.ctx, "Starting to monitor external process %s (PID %d) using WaitForSingleObject", processName, processID)

	// 创建进程监控器
	pm, exitChan, err := processutils.WaitForProcessExitAsync(processID)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to open process monitor for %s (PID %d), falling back to process snapshot polling: %v", processName, processID, err)
		snapshotMonitor, snapshotExitChan := processutils.WaitForProcessExitBySnapshotAsync(processID)
		defer snapshotMonitor.Stop()
		s.waitForProcessExit(sessionID, gameID, startTime, processName, processID, snapshotExitChan)
		return
	}
	defer pm.Stop()

	s.waitForProcessExit(sessionID, gameID, startTime, processName, processID, exitChan)
}

func (s *StartService) monitorProcessByHandle(sessionID string, gameID string, startTime time.Time, processID uint32, processName string, processHandle uintptr) {
	applog.LogInfof(s.ctx, "Starting to monitor launched process %s (PID %d) using ShellExecuteEx handle", processName, processID)

	pm, exitChan, err := processutils.WaitForProcessHandleExitAsync(processID, processHandle)
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to monitor process handle for %s (PID %d), falling back to PID monitor: %v", processName, processID, err)
		s.monitorProcessByPID(sessionID, gameID, startTime, processID, processName)
		return
	}
	defer pm.Stop()

	s.waitForProcessExit(sessionID, gameID, startTime, processName, processID, exitChan)
}

func (s *StartService) waitForProcessExit(sessionID string, gameID string, startTime time.Time, processName string, processID uint32, exitChan <-chan struct{}) {
	// 等待进程退出或超时（24小时）
	select {
	case <-exitChan:
		applog.LogInfof(s.ctx, "Game process %s (PID %d) has exited", processName, processID)
	case <-time.After(24 * time.Hour):
		applog.LogWarningf(s.ctx, "Game %s exceeded maximum runtime (24h), forcing cleanup", gameID)
	}

	// 执行统一的会话清理逻辑
	s.finalizePlaySession(sessionID, gameID, startTime)
}

// finalizePlaySession 完成游玩会话的最终处理
// 包括停止追踪、计算时长、更新数据库、自动备份等
func (s *StartService) finalizePlaySession(sessionID string, gameID string, startTime time.Time) {
	// 确保停止追踪（无论如何都要执行）
	activeSeconds := s.activeTimeTracker.StopTracking(gameID)

	endTime := time.Now()

	// 如果启用活跃时间追踪，使用累加的活跃时长
	// 否则使用整个运行时长
	var duration int
	if s.config.RecordActiveTimeOnly {
		duration = activeSeconds
		applog.LogInfof(s.ctx, "Game %s active play time: %d seconds", gameID, duration)
	} else {
		duration = int(endTime.Sub(startTime).Seconds())
		applog.LogInfof(s.ctx, "Game %s total runtime: %d seconds", gameID, duration)
	}

	// 如果游玩时长小于1分钟，删除临时会话记录
	if duration < 60 {
		err := s.sessionService.DeletePlaySession(sessionID)
		if err != nil {
			applog.LogErrorf(s.ctx, "Failed to delete short play session %s: %v", sessionID, err)
		} else {
			s.requestHomeRefresh()
		}
		return
	}

	// 更新会话记录
	session := models.PlaySession{
		ID:        sessionID,
		GameID:    gameID,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	}
	err := s.sessionService.UpdatePlaySession(session)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to update play session %s: %v", sessionID, err)
		return
	}

	s.requestHomeRefresh()

	// 自动备份游戏存档
	if s.config.AutoBackupGameSave && s.backupService != nil {
		s.autoBackupGameSave(gameID)
	}
}

func (s *StartService) requestHomeRefresh() {
	if s.ctx == nil {
		return
	}

	runtime.EventsEmit(s.ctx, homeRefreshRequestedEvent)
}

// updateGameProcessName 更新游戏的进程名
func (s *StartService) updateGameProcessName(gameID string, processName string) error {
	return s.gameService.UpdateGameProcessName(gameID, processName)
}

// NotifyProcessSelected 用户选择了进程后调用此方法通知后端
// 这会唤醒等待的 goroutine，并在选择稳定 exe 进程时更新数据库
func (s *StartService) NotifyProcessSelected(gameID string, processName string) error {
	// 先更新数据库。非 exe 包装进程通常是随机/伪装名称，只用于本次监控。
	if isPersistableProcessName(processName) {
		if err := s.updateGameProcessName(gameID, processName); err != nil {
			return err
		}
	} else {
		applog.LogInfof(s.ctx, "Selected non-exe process for game %s will not be persisted as process_name: %s", gameID, processName)
	}

	// 通过 channel 通知等待的 goroutine
	s.pendingProcessSelectMu.RLock()
	selectChan, exists := s.pendingProcessSelect[gameID]
	s.pendingProcessSelectMu.RUnlock()

	if exists {
		// 非阻塞发送（如果 channel 已满或已关闭则跳过）
		select {
		case selectChan <- processName:
			applog.LogInfof(s.ctx, "Notified process selection for game %s: %s", gameID, processName)
		default:
			applog.LogWarningf(s.ctx, "Failed to notify process selection for game %s (channel full or closed)", gameID)
		}
	} else {
		applog.LogWarningf(s.ctx, "No pending process selection for game %s", gameID)
	}

	return nil
}

// CancelProcessSelection 用户取消了进程选择
// 关闭等待的 channel 并清理临时会话
func (s *StartService) CancelProcessSelection(gameID string) error {
	s.pendingProcessSelectMu.Lock()
	selectChan, exists := s.pendingProcessSelect[gameID]
	if exists {
		// 关闭 channel（让等待的 goroutine 知道用户取消了）
		close(selectChan)
		delete(s.pendingProcessSelect, gameID)
	}
	s.pendingProcessSelectMu.Unlock()

	if exists {
		applog.LogInfof(s.ctx, "User cancelled process selection for game %s", gameID)
	} else {
		applog.LogWarningf(s.ctx, "No pending process selection to cancel for game %s", gameID)
	}

	return nil
}

// CleanupPendingSessions 清理所有待定的进程选择会话
// 用于程序关闭时的清理，包括：
// 1. 关闭所有等待的进程选择 channels
// 2. 停止所有活跃时间追踪
// 3. 清理数据库中未完成的会话记录
func (s *StartService) CleanupPendingSessions() {
	// 1. 清理进程选择 channels
	s.pendingProcessSelectMu.Lock()
	if len(s.pendingProcessSelect) > 0 {
		applog.LogInfof(s.ctx, "Cleaning up %d pending process selections", len(s.pendingProcessSelect))
		// 关闭所有等待的 channels
		for gameID, selectChan := range s.pendingProcessSelect {
			close(selectChan)
			applog.LogInfof(s.ctx, "Cancelled pending process selection for game %s", gameID)
		}
		// 清空 map
		s.pendingProcessSelect = make(map[string]chan string)
	}
	s.pendingProcessSelectMu.Unlock()

	// 2. 停止所有活跃时间追踪
	if s.activeTimeTracker != nil {
		s.activeTimeTracker.StopAllTracking()
		applog.LogInfof(s.ctx, "Stopped all active time tracking")
	}

	// 3. 清理数据库中未完成的会话
	if s.sessionService != nil {
		err := s.sessionService.CleanupUnfinishedSessions()
		if err != nil {
			applog.LogErrorf(s.ctx, "Failed to cleanup unfinished sessions: %v", err)
		} else {
			applog.LogInfof(s.ctx, "Successfully cleaned up unfinished sessions")
		}
	}
}

// autoBackupGameSave 自动备份游戏存档
func (s *StartService) autoBackupGameSave(gameID string) {
	// 检查是否设置了存档目录
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil || game.SavePath == "" {
		applog.LogDebugf(s.ctx, "Game %s has no save path configured, skipping auto backup", gameID)
		return
	}

	// 执行备份
	applog.LogInfof(s.ctx, "Auto backing up game save for: %s", gameID)
	backup, err := s.backupService.CreateBackup(gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to auto backup game save: %v", err)
		return
	}

	// 如果启用了游戏存档自动上传到云端
	if s.config.AutoUploadSaveToCloud && s.config.CloudBackupEnabled && s.config.BackupUserID != "" {
		applog.LogInfof(s.ctx, "Auto uploading backup to cloud: %s", backup.Path)
		err = s.backupService.UploadGameBackupToCloud(gameID, backup.Path)
		if err != nil {
			applog.LogErrorf(s.ctx, "Failed to auto upload backup to cloud: %v", err)
		} else {
			applog.LogInfof(s.ctx, "Successfully uploaded backup to cloud: %s", backup.Path)
		}
	}
	applog.LogInfof(s.ctx, "Auto backup completed for game: %s", gameID)
}

// getGamePathAndProcess 获取游戏路径和已保存的进程名
func (s *StartService) getGamePathAndProcess(gameID string) (path string, processName string, err error) {
	if s.gameService == nil {
		return "", "", fmt.Errorf("game service is not initialized")
	}
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return "", "", err
	}
	return game.Path, game.ProcessName, nil
}

// resolveExecutablePath 当路径为空或路径是目录时，引导用户选择可执行文件并保存到游戏配置
func (s *StartService) resolveExecutablePath(gameID string, path string, processName string) (string, string, bool, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		applog.LogInfof(s.ctx, "game path is empty for game %s, prompting executable selection", gameID)
		selection, err := s.gameService.SelectGameExecutable("")
		if err != nil {
			return "", "", false, fmt.Errorf("open executable dialog failed: %w", err)
		}
		return s.saveSelectedExecutablePath(gameID, selection, processName)
	}

	normalizedPath, err := filepath.Abs(filepath.Clean(trimmedPath))
	if err != nil {
		return "", "", false, fmt.Errorf("normalize game path failed: %w", err)
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return "", "", false, fmt.Errorf("stat game path: %w", err)
	}
	if !info.IsDir() {
		return normalizedPath, strings.TrimSpace(processName), false, nil
	}

	selection, err := s.gameService.ResolveExecutablePathForImport(normalizedPath)
	if err != nil {
		return "", "", false, fmt.Errorf("open executable dialog failed: %w", err)
	}
	if selection == "" {
		return "", "", true, nil
	}

	return s.saveSelectedExecutablePath(gameID, selection, processName)
}

func (s *StartService) saveSelectedExecutablePath(gameID string, selection string, processName string) (string, string, bool, error) {
	if s.gameService == nil {
		return "", "", false, fmt.Errorf("game service is not initialized")
	}

	selection = strings.TrimSpace(selection)
	if selection == "" {
		return "", "", true, nil
	}

	resolvedSelection, err := filepath.Abs(filepath.Clean(selection))
	if err != nil {
		return "", "", false, fmt.Errorf("normalize selected executable failed: %w", err)
	}
	selectionInfo, err := os.Stat(resolvedSelection)
	if err != nil {
		return "", "", false, fmt.Errorf("stat selected executable failed: %w", err)
	}
	if selectionInfo.IsDir() {
		return "", "", false, fmt.Errorf("selected path is a directory, not executable")
	}

	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to load game for path update: %w", err)
	}

	resolvedProcessName := strings.TrimSpace(processName)
	if resolvedProcessName == "" {
		resolvedProcessName = filepath.Base(resolvedSelection)
	}

	game.Path = resolvedSelection
	if strings.TrimSpace(game.ProcessName) == "" {
		game.ProcessName = resolvedProcessName
	}
	if err := s.gameService.UpdateGame(game); err != nil {
		return "", "", false, fmt.Errorf("failed to save selected executable: %w", err)
	}

	return resolvedSelection, resolvedProcessName, false, nil
}

// getGameLaunchConfig 获取游戏的启动配置
func (s *StartService) getGameLaunchConfig(gameID string) (useLE bool, useMagpie bool, err error) {
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return false, false, err
	}
	return game.UseLocaleEmulator, game.UseMagpie, nil
}

// startMagpie 启动 Magpie 程序
func (s *StartService) startMagpie() {
	// 延迟一小段时间，确保游戏窗口已经创建
	time.Sleep(1 * time.Second)

	// 检查 Magpie 是否已经在运行
	isRunning, err := processutils.CheckIfProcessRunning("Magpie.exe")
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to check Magpie process: %v", err)
		return
	}

	if isRunning {
		applog.LogInfof(s.ctx, "Magpie is already running")
		return
	}

	// 启动 Magpie (tray 模式)
	applog.LogInfof(s.ctx, "Starting Magpie in tray mode: %s", s.config.MagpiePath)
	cmd := exec.Command(s.config.MagpiePath, "-t")
	cmd.Dir = filepath.Dir(s.config.MagpiePath)

	if err := cmd.Start(); err != nil {
		applog.LogErrorf(s.ctx, "Failed to start Magpie: %v", err)
		return
	}

	// 分离进程，避免阻塞
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	applog.LogInfof(s.ctx, "Magpie started successfully")
}
