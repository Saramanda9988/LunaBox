package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/service/timer"
	"lunabox/internal/utils"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	// 获取游戏路径和进程配置
	path, processName, err := s.getGamePathAndProcess(gameID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("failed to get game path: %w", err)
	}

	if path == "" {
		runtime.LogErrorf(s.ctx, "game path is empty for game: %s", gameID)
		return false, fmt.Errorf("game path is empty for game: %s", gameID)
	}

	// 获取启动exe的名称
	launcherExeName := filepath.Base(path)

	// 获取游戏的启动配置
	useLE, useMagpie, err := s.getGameLaunchConfig(gameID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to get launch config: %v", err)
		return false, fmt.Errorf("failed to get launch config: %w", err)
	}

	var cmd *exec.Cmd

	// 如果启用了 Locale Emulator
	if useLE && s.config.LocaleEmulatorPath != "" {
		runtime.LogInfof(s.ctx, "Starting game with Locale Emulator: %s", gameID)
		cmd = exec.Command(s.config.LocaleEmulatorPath, path)
		cmd.Dir = filepath.Dir(path)
	} else {
		// 普通启动
		cmd = exec.Command(path)
		cmd.Dir = filepath.Dir(path)
	}

	if err := cmd.Start(); err != nil {
		runtime.LogErrorf(s.ctx, "failed to start game: %v", err)
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
		runtime.LogInfof(s.ctx, "Game %s active play time: %d seconds", gameID, duration)
	} else {
		duration = int(endTime.Sub(startTime).Seconds())
		runtime.LogInfof(s.ctx, "Game %s total runtime: %d seconds", gameID, duration)
	}

	// If play time is less than 1 minute, remove the temporary session record
	if duration < 60 {
		err := s.sessionService.DeletePlaySession(sessionID)
		if err != nil {
			runtime.LogErrorf(s.ctx, "Failed to delete short play session %s: %v", sessionID, err)
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
		runtime.LogErrorf(s.ctx, "Failed to update play session %s: %v", sessionID, err)
		return
	}

	// 自动备份游戏存档
	if s.config.AutoBackupGameSave && s.backupService != nil {
		s.autoBackupGameSave(gameID)
	}
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
			runtime.LogDebugf(s.ctx, "Game %s exited with error: %v", gameID, exitErr)
		}
	case <-time.After(24 * time.Hour):
		// 超时保护（24小时后强制清理）
		runtime.LogWarningf(s.ctx, "Game %s exceeded maximum runtime (24h), forcing cleanup", gameID)
	}

	// 执行统一的会话清理逻辑
	s.finalizePlaySession(sessionID, gameID, startTime)
}

// detectAndMonitorProcess 检测实际游戏进程并开始监控
func (s *StartService) detectAndMonitorProcess(cmd *exec.Cmd, sessionID string, gameID string, startTime time.Time, launcherPID uint32, launcherExeName string, savedProcessName string) {
	// 等待1秒，让启动器有时间启动实际游戏进程
	time.Sleep(1 * time.Second)

	var actualProcessID uint32
	var actualProcessName string
	var needExternalMonitor bool // 是否需要外部进程监控（非cmd子进程）

	// 情况1: 已保存了process_name，且与启动器exe名称不同
	// 说明之前用户已选择过实际的游戏进程
	if savedProcessName != "" && savedProcessName != launcherExeName {
		runtime.LogInfof(s.ctx, "Game %s has saved process_name: %s, searching for it", gameID, savedProcessName)

		// 在系统进程中搜索保存的进程名
		pid, err := utils.GetProcessPIDByName(savedProcessName)
		if err != nil {
			runtime.LogWarningf(s.ctx, "Failed to find saved process %s: %v, falling back to launcher monitoring", savedProcessName, err)
			// 如果找不到保存的进程，使用启动器进程监控
			actualProcessID = launcherPID
			actualProcessName = launcherExeName
			needExternalMonitor = false
		} else {
			actualProcessID = pid
			actualProcessName = savedProcessName
			needExternalMonitor = true // 需要外部监控，因为这不是cmd的子进程
			runtime.LogInfof(s.ctx, "Found saved process %s with PID %d", savedProcessName, pid)
		}
	} else {
		// 情况2: 没有保存的process_name，或process_name与启动器相同
		// 检查启动器进程是否仍在运行
		launcherStillRunning := utils.IsProcessRunningByPID(launcherPID)

		if launcherStillRunning {
			// 启动器仍在运行，直接监控启动器进程
			actualProcessID = launcherPID
			actualProcessName = launcherExeName
			needExternalMonitor = false
			runtime.LogInfof(s.ctx, "Launcher %s (PID %d) is still running, monitoring it", launcherExeName, launcherPID)

			// 如果没有保存过process_name，保存当前的
			if savedProcessName == "" {
				s.updateGameProcessName(gameID, launcherExeName)
			}
		} else {
			// 启动器已退出，需要让用户选择实际的游戏进程
			runtime.LogInfof(s.ctx, "Launcher %s (PID %d) has exited, notifying frontend to select actual game process", launcherExeName, launcherPID)

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

			// 等待用户选择（最多等待5分钟）使用channel
			var selectedProcess string
			select {
			case selectedProcess = <-selectChan:
				// 用户已选择进程
			case <-time.After(5 * time.Minute):
				// 超时未选择
				runtime.LogWarningf(s.ctx, "Process selection timeout for game %s, cleaning up session", gameID)
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
				runtime.LogErrorf(s.ctx, "Failed to find selected process %s: %v", selectedProcess, err)
				s.sessionService.DeletePlaySession(sessionID)
				return
			}
			actualProcessID = pid
			actualProcessName = selectedProcess
			needExternalMonitor = true
			runtime.LogInfof(s.ctx, "User selected process %s (PID %d)", selectedProcess, pid)
		}
	}

	// 启动活跃时间追踪（如果启用）
	if s.config.RecordActiveTimeOnly {
		_, err := s.activeTimeTracker.StartTracking(sessionID, gameID, actualProcessID)
		if err != nil {
			runtime.LogWarningf(s.ctx, "Failed to start active time tracking: %v", err)
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

// monitorProcessByPID 通过PID监控外部进程直到退出
// 使用 WaitForSingleObject 事件驱动，避免轮询
func (s *StartService) monitorProcessByPID(sessionID string, gameID string, startTime time.Time, processID uint32, processName string) {
	runtime.LogInfof(s.ctx, "Starting to monitor external process %s (PID %d) using WaitForSingleObject", processName, processID)

	// 创建进程监控器
	pm, exitChan, err := utils.WaitForProcessExitAsync(processID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "Failed to start process monitor for %s (PID %d): %v", processName, processID, err)
		// 监控失败，直接清理
		s.finalizePlaySession(sessionID, gameID, startTime)
		return
	}
	defer pm.Stop()

	// 等待进程退出或超时（24小时）
	select {
	case <-exitChan:
		runtime.LogInfof(s.ctx, "External process %s (PID %d) has exited", processName, processID)
	case <-time.After(24 * time.Hour):
		runtime.LogWarningf(s.ctx, "Game %s exceeded maximum runtime (24h), forcing cleanup", gameID)
	}

	// 执行统一的会话清理逻辑
	s.finalizePlaySession(sessionID, gameID, startTime)
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
			runtime.LogInfof(s.ctx, "Notified process selection for game %s: %s", gameID, processName)
		default:
			runtime.LogWarningf(s.ctx, "Failed to notify process selection for game %s (channel full or closed)", gameID)
		}
	} else {
		runtime.LogWarningf(s.ctx, "No pending process selection for game %s", gameID)
	}

	return nil
}

// autoBackupGameSave 自动备份游戏存档
func (s *StartService) autoBackupGameSave(gameID string) {
	// 检查是否设置了存档目录
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil || game.SavePath == "" {
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

	// 如果启用了游戏存档自动上传到云端
	if s.config.AutoUploadSaveToCloud && s.config.CloudBackupEnabled && s.config.BackupUserID != "" {
		runtime.LogInfof(s.ctx, "Auto uploading backup to cloud: %s", backup.Path)
		err = s.backupService.UploadGameBackupToCloud(gameID, backup.Path)
		if err != nil {
			runtime.LogErrorf(s.ctx, "Failed to auto upload backup to cloud: %v", err)
		} else {
			runtime.LogInfof(s.ctx, "Successfully uploaded backup to cloud: %s", backup.Path)
		}
	}
	runtime.LogInfof(s.ctx, "Auto backup completed for game: %s", gameID)
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
		runtime.LogErrorf(s.ctx, "Failed to check Magpie process: %v", err)
		return
	}

	if isRunning {
		runtime.LogInfof(s.ctx, "Magpie is already running")
		return
	}

	// 启动 Magpie (tray 模式)
	runtime.LogInfof(s.ctx, "Starting Magpie in tray mode: %s", s.config.MagpiePath)
	cmd := exec.Command(s.config.MagpiePath, "-t")
	cmd.Dir = filepath.Dir(s.config.MagpiePath)

	if err := cmd.Start(); err != nil {
		runtime.LogErrorf(s.ctx, "Failed to start Magpie: %v", err)
		return
	}

	// 分离进程，避免阻塞
	if cmd.Process != nil {
		cmd.Process.Release()
	}

	runtime.LogInfof(s.ctx, "Magpie started successfully")
}
