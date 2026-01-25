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

type StartService struct {
	ctx               context.Context
	db                *sql.DB
	config            *appconf.AppConfig
	backupService     *BackupService
	activeTimeTracker *ActiveTimeTracker
}

func NewStartService() *StartService {
	return &StartService{
		activeTimeTracker: NewActiveTimeTracker(),
	}
}

func (s *StartService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.activeTimeTracker.Init(ctx, db)
}

// SetBackupService 设置备份服务（用于自动备份）
func (s *StartService) SetBackupService(backupService *BackupService) {
	s.backupService = backupService
}

// StartGameWithTracking 启动游戏并自动追踪游玩时长
// 当游戏进程退出时，自动保存游玩记录到数据库
func (s *StartService) StartGameWithTracking(gameID string) (bool, error) {
	// 获取游戏路径
	path, err := s.getGamePath(gameID)
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("failed to get game path: %w", err)
	}

	if path == "" {
		runtime.LogErrorf(s.ctx, "failed to get game path: %v", err)
		return false, fmt.Errorf("game path is empty for game: %s", gameID)
	}

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

	// 获取进程 ID
	processID := uint32(cmd.Process.Pid)

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

	// 启动游戏监控 goroutine
	go s.waitForGameExit(cmd, sessionID, gameID, startTime, processID)

	// 如果启用了仅记录活跃时长，延迟启动活跃时间追踪（在独立 goroutine 中，避免阻塞主线程）
	if s.config.RecordActiveTimeOnly {
		go func() {
			// 延迟一小段时间，确保游戏窗口已经创建
			time.Sleep(500 * time.Millisecond)
			_, err := s.activeTimeTracker.StartTracking(sessionID, gameID, processID)
			if err != nil {
				runtime.LogWarningf(s.ctx, "Failed to start active time tracking: %v", err)
			}
		}()
	}

	// 启动成功，返回 true 给前端
	return true, nil
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
	case <-time.After(24 * time.Hour):
		// 超时保护（24小时后强制清理）
		runtime.LogWarningf(s.ctx, "Game %s exceeded maximum runtime (24h), forcing cleanup", gameID)
	}

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

	if exitErr != nil {
		runtime.LogDebugf(s.ctx, "Game %s exited with error: %v", gameID, exitErr)
	}

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
func (s *StartService) autoBackupGameSave(gameID string) {
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

func (s *StartService) getGamePath(gameID string) (string, error) {
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

// getGameLaunchConfig 获取游戏的启动配置
func (s *StartService) getGameLaunchConfig(gameID string) (useLE bool, useMagpie bool, err error) {
	err = s.db.QueryRowContext(
		s.ctx,
		"SELECT COALESCE(use_locale_emulator, FALSE), COALESCE(use_magpie, FALSE) FROM games WHERE id = ?",
		gameID,
	).Scan(&useLE, &useMagpie)

	if errors.Is(err, sql.ErrNoRows) {
		return false, false, fmt.Errorf("game not found: %s", gameID)
	}
	if err != nil {
		return false, false, err
	}

	return useLE, useMagpie, nil
}

// startMagpie 启动 Magpie 程序
func (s *StartService) startMagpie() {
	// 延迟一小段时间，确保游戏窗口已经创建
	time.Sleep(1 * time.Second)

	// 检查 Magpie 是否已经在运行
	isRunning, err := s.checkIfProcessRunning("Magpie.exe")
	if err != nil {
		runtime.LogErrorf(s.ctx, "Failed to check Magpie process: %v", err)
		return
	}

	if isRunning {
		runtime.LogInfof(s.ctx, "Magpie is already running")
		return
	}

	// 启动 Magpie
	runtime.LogInfof(s.ctx, "Starting Magpie: %s", s.config.MagpiePath)
	cmd := exec.Command(s.config.MagpiePath)
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

// checkIfProcessRunning 检查指定进程是否正在运行
func (s *StartService) checkIfProcessRunning(processName string) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", processName), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	outputStr := string(output)
	// 检查输出中是否包含进程名
	return len(outputStr) > 0 && outputStr != "INFO: No tasks are running which match the specified criteria.\r\n", nil
}

// AddPlaySession 手动添加游玩记录
// startTime: 开始时间
// durationMinutes: 游玩时长（分钟）
func (s *StartService) AddPlaySession(gameID string, startTime time.Time, durationMinutes int) (models.PlaySession, error) {
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
func (s *StartService) GetPlaySessions(gameID string) ([]models.PlaySession, error) {
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
func (s *StartService) DeletePlaySession(sessionID string) error {
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
func (s *StartService) UpdatePlaySession(session models.PlaySession) error {
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
func (s *StartService) BatchAddPlaySessions(sessions []models.PlaySession) error {
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
