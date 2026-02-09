package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/models"
	"lunabox/internal/service/timer"
	"lunabox/internal/utils"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	activeTimeTracker *timer.ActiveTimeTracker

	// 进程选择相关
	pendingProcessSelect   map[string]chan string // gameID -> channel，用于接收用户选择的进程名
	pendingProcessSelectMu sync.RWMutex
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
	s.activeTimeTracker = timer.NewActiveTimeTracker(ctx, db)
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

// startGame 内部启动方法，支持通过 options 覆盖配置
func (s *StartService) startGame(gameID string, options LaunchOptions) (bool, error) {
	// 获取游戏路径和进程配置
	path, processName, err := s.getGamePathAndProcess(gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("failed to get game path: %w", err)
	}

	if path == "" {
		applog.LogErrorf(s.ctx, "game path is empty for game: %s", gameID)
		return false, fmt.Errorf("game path is empty for game: %s", gameID)
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

	var cmd *exec.Cmd

	// 如果启用了 Locale Emulator
	if useLE && s.config.LocaleEmulatorPath != "" {
		applog.LogInfof(s.ctx, "Starting game with Locale Emulator: %s", gameID)
		cmd = exec.Command(s.config.LocaleEmulatorPath, path)
		cmd.Dir = filepath.Dir(path)
	} else {
		// 普通启动
		cmd = exec.Command(path)
		cmd.Dir = filepath.Dir(path)
	}

	if err := cmd.Start(); err != nil {
		applog.LogErrorf(s.ctx, "failed to start game: %v", err)
		return false, fmt.Errorf("failed to start game: %w", err)
	}

	// 如果启用了 Magpie，在游戏启动后启动 Magpie
	if useMagpie && s.config.MagpiePath != "" {
		go s.startMagpie()
	}

	// 获取启动器的进程 ID
	launcherPID := uint32(cmd.Process.Pid)

	startTime := time.Now()
	sessionID, err := s.sessionService.CreatePendingSession(gameID, startTime)
	if err != nil {
		return false, fmt.Errorf("failed to create play session: %w", err)
	}

	// 启动进程检测和监控 goroutine
	go s.detectAndMonitorProcess(cmd, sessionID, gameID, startTime, launcherPID, launcherExeName, processName)

	// 启动成功，返回 true 给前端
	return true, nil
}

// detectAndMonitorProcess 检测实际游戏进程并开始监控
// 采用分阶段检测策略，利用60秒会话记录阈值提供的余裕时间
func (s *StartService) detectAndMonitorProcess(cmd *exec.Cmd, sessionID string, gameID string, startTime time.Time, launcherPID uint32, launcherExeName string, savedProcessName string) {
	var actualProcessID uint32
	var actualProcessName string
	var needExternalMonitor bool // 是否需要外部进程监控（非cmd子进程）

	// 情况1: 已保存了process_name，且与启动器exe名称不同
	// 说明之前用户已选择过实际的游戏进程
	if savedProcessName != "" && savedProcessName != launcherExeName {
		applog.LogInfof(s.ctx, "Game %s has saved process_name: %s, will search for it after initial delay", gameID, savedProcessName)

		// 等待5秒，给启动器时间启动实际游戏
		time.Sleep(5 * time.Second)

		// 在系统进程中搜索保存的进程名
		pid, err := utils.GetProcessPIDByName(savedProcessName)
		if err != nil {
			applog.LogWarningf(s.ctx, "Failed to find saved process %s: %v, falling back to launcher monitoring", savedProcessName, err)
			// 如果找不到保存的进程，使用启动器进程监控
			actualProcessID = launcherPID
			actualProcessName = launcherExeName
			needExternalMonitor = false
		} else {
			actualProcessID = pid
			actualProcessName = savedProcessName
			needExternalMonitor = true // 需要外部监控，因为这不是cmd的子进程
			applog.LogInfof(s.ctx, "Found saved process %s with PID %d", savedProcessName, pid)
		}
	} else {
		// 情况2: 没有保存的process_name，或process_name与启动器相同
		// 使用分阶段检测策略来准确判断启动器类型

		applog.LogInfof(s.ctx, "Starting staged detection for game %s, launcher: %s (PID %d)", gameID, launcherExeName, launcherPID)

		// 阶段1: 初始等待5秒，让启动器有时间启动实际游戏
		time.Sleep(5 * time.Second)

		// 阶段2: 第一次检测
		launcherStillRunning := utils.IsProcessRunningByPID(launcherPID, s.ctx)

		if !launcherStillRunning {
			// 启动器在5秒内就退出了，说明是快速启动器（如Steam）
			applog.LogInfof(s.ctx, "Launcher %s exited quickly (within 5s), will prompt for actual game process", launcherExeName)
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

			if !utils.IsProcessRunningByPID(launcherPID, s.ctx) {
				// 启动器在观察期内退出了，说明它只是个启动器
				applog.LogInfof(s.ctx, "Launcher %s exited during observation period, will prompt for actual game process", launcherExeName)
				s.promptUserToSelectProcess(sessionID, gameID, startTime, launcherExeName)
				return
			}
		}

		// 阶段4: 观察期结束，启动器仍在运行
		// 说明启动器本身就是游戏进程（如普通单exe游戏）
		applog.LogInfof(s.ctx, "Launcher %s still running after 20s total, treating it as the game process", launcherExeName)
		actualProcessID = launcherPID
		actualProcessName = launcherExeName
		needExternalMonitor = false

		// 保存进程名
		if savedProcessName == "" {
			s.updateGameProcessName(gameID, launcherExeName)
		}
	}

	// 启动活跃时间追踪（如果启用）
	if s.config.RecordActiveTimeOnly {
		_, err := s.activeTimeTracker.StartTracking(sessionID, gameID, actualProcessID)
		if err != nil {
			applog.LogWarningf(s.ctx, "Failed to start active time tracking: %v", err)
		}
	}

	// 根据情况选择监控方式
	if needExternalMonitor {
		// 需要外部监控：实际游戏进程不是cmd的子进程
		s.monitorProcessByPID(sessionID, gameID, startTime, actualProcessID, actualProcessName)
	} else {
		// 使用原有的 waitForGameExit：可以利用 cmd.Wait() 事件驱动
		s.waitForGameExit(cmd, sessionID, gameID, startTime, actualProcessID)
	}
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
	pid, err := utils.GetProcessPIDByName(selectedProcess)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to find selected process %s: %v", selectedProcess, err)
		s.sessionService.DeletePlaySession(sessionID)
		return
	}

	// 保存用户选择的进程名
	s.updateGameProcessName(gameID, selectedProcess)

	// 启动活跃时间追踪（如果启用）
	if s.config.RecordActiveTimeOnly {
		_, err := s.activeTimeTracker.StartTracking(sessionID, gameID, pid)
		if err != nil {
			applog.LogWarningf(s.ctx, "Failed to start active time tracking: %v", err)
		}
	}

	// 监控选中的进程
	s.monitorProcessByPID(sessionID, gameID, startTime, pid, selectedProcess)
}

// waitForGameExit 等待游戏进程退出并更新游玩记录
func (s *StartService) waitForGameExit(cmd *exec.Cmd, sessionID string, gameID string, startTime time.Time, processID uint32) {
	// 使用独立 goroutine 等待进程，避免永久阻塞
	exitChan := make(chan error, 1)
	go func() {
		exitChan <- cmd.Wait()
	}()

	// 等待进程退出，最长等待24小时（防止永久阻塞）
	var exitErr error
	select {
	case exitErr = <-exitChan:
		// 游戏正常退出
		if exitErr != nil {
			applog.LogDebugf(s.ctx, "Game %s exited with error: %v", gameID, exitErr)
		}
	case <-time.After(24 * time.Hour):
		// 超时保护（24小时后强制清理）
		applog.LogWarningf(s.ctx, "Game %s exceeded maximum runtime (24h), forcing cleanup", gameID)
	}

	// 执行统一的会话清理逻辑
	s.finalizePlaySession(sessionID, gameID, startTime)
}

// monitorProcessByPID 通过PID监控外部进程直到退出
// 使用 WaitForSingleObject 事件驱动，避免轮询
func (s *StartService) monitorProcessByPID(sessionID string, gameID string, startTime time.Time, processID uint32, processName string) {
	applog.LogInfof(s.ctx, "Starting to monitor external process %s (PID %d) using WaitForSingleObject", processName, processID)

	// 创建进程监控器
	pm, exitChan, err := utils.WaitForProcessExitAsync(processID)
	if err != nil {
		applog.LogErrorf(s.ctx, "Failed to start process monitor for %s (PID %d): %v", processName, processID, err)
		// 监控失败，直接清理
		s.finalizePlaySession(sessionID, gameID, startTime)
		return
	}
	defer pm.Stop()

	// 等待进程退出或超时（24小时）
	select {
	case <-exitChan:
		applog.LogInfof(s.ctx, "External process %s (PID %d) has exited", processName, processID)
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

	// 自动备份游戏存档
	if s.config.AutoBackupGameSave && s.backupService != nil {
		s.autoBackupGameSave(gameID)
	}
}

// updateGameProcessName 更新游戏的进程名
func (s *StartService) updateGameProcessName(gameID string, processName string) error {
	return s.gameService.UpdateGameProcessName(gameID, processName)
}

// NotifyProcessSelected 用户选择了进程后调用此方法通知后端
// 这会唤醒等待的 goroutine 并更新数据库
func (s *StartService) NotifyProcessSelected(gameID string, processName string) error {
	// 先更新数据库
	if err := s.updateGameProcessName(gameID, processName); err != nil {
		return err
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
// 用于程序关闭时的清理
func (s *StartService) CleanupPendingSessions() {
	s.pendingProcessSelectMu.Lock()
	defer s.pendingProcessSelectMu.Unlock()

	if len(s.pendingProcessSelect) == 0 {
		return
	}

	applog.LogInfof(s.ctx, "Cleaning up %d pending process selections", len(s.pendingProcessSelect))

	// 关闭所有等待的 channels
	for gameID, selectChan := range s.pendingProcessSelect {
		close(selectChan)
		applog.LogInfof(s.ctx, "Cancelled pending process selection for game %s", gameID)
	}

	// 清空 map
	s.pendingProcessSelect = make(map[string]chan string)
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
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return "", "", err
	}
	return game.Path, game.ProcessName, nil
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
	isRunning, err := utils.CheckIfProcessRunning("Magpie.exe")
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
