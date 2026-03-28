package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/archiveutils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/vo"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	grab "github.com/cavaliergopher/grab/v3"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zeebo/blake3"
)

// DownloadStatus 下载状态
type DownloadStatus string

const (
	DownloadStatusPending     DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusExtracting  DownloadStatus = "extracting"
	DownloadStatusPaused      DownloadStatus = "paused"
	DownloadStatusDone        DownloadStatus = "done"
	DownloadStatusError       DownloadStatus = "error"
	DownloadStatusCancelled   DownloadStatus = "cancelled"
	DownloadManualExtractFlag                = "manual_extract_required"
	downloadUserAgent                        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 LunaBox/1.0"
)

// DownloadTask 单个下载任务
type DownloadTask struct {
	ID         string            `json:"id"`
	Request    vo.InstallRequest `json:"request"`
	Status     DownloadStatus    `json:"status"`
	Progress   float64           `json:"progress"`   // 0~100
	Downloaded int64             `json:"downloaded"` // bytes downloaded
	Total      int64             `json:"total"`      // bytes total (0 = unknown)
	Error      string            `json:"error,omitempty"`
	FilePath   string            `json:"file_path,omitempty"` // 下载完成后的本地路径
	cancel     context.CancelFunc
	pauseReq   bool
	cancelReq  bool
}

// DownloadProgressEvent 通过 Wails event 推送的进度事件
type DownloadProgressEvent struct {
	ID         string            `json:"id"`
	Request    vo.InstallRequest `json:"request"`
	Status     DownloadStatus    `json:"status"`
	Progress   float64           `json:"progress"`
	Downloaded int64             `json:"downloaded"`
	Total      int64             `json:"total"`
	Error      string            `json:"error,omitempty"`
	FilePath   string            `json:"file_path,omitempty"`
}

// DownloadService 管理所有下载任务
type DownloadService struct {
	ctx            context.Context
	db             *sql.DB
	config         *appconf.AppConfig
	gameService    *GameService
	mu             sync.RWMutex
	tasks          map[string]*DownloadTask
	pendingInstall *vo.InstallRequest // 从 lunabox:// URI 传入的待安装请求，在 GUI 就绪前暂存
}

func NewDownloadService() *DownloadService {
	return &DownloadService{
		tasks: make(map[string]*DownloadTask),
	}
}

func (s *DownloadService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	if err := s.loadTasksFromDB(); err != nil {
		applog.LogErrorf(s.ctx, "failed to load download tasks from db: %v", err)
	}
}

// SetGameService 注入游戏服务（用于下载完成后预抓取元数据）
func (s *DownloadService) SetGameService(gameService *GameService) {
	s.gameService = gameService
}

// SetPendingInstall 在 Wails 启动前由 main.go 调用，暂存待安装请求
func (s *DownloadService) SetPendingInstall(req *vo.InstallRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingInstall = req
}

// GetPendingInstall 前端初始化完成后调用，获取并清除待安装请求
func (s *DownloadService) GetPendingInstall() *vo.InstallRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	req := s.pendingInstall
	s.pendingInstall = nil
	return req
}

// StartDownload 开始一个下载任务，返回任务 ID
func (s *DownloadService) StartDownload(req vo.InstallRequest) (string, error) {
	if err := validateInstallRequest(req); err != nil {
		return "", err
	}

	taskID := uuid.New().String()

	ctx, cancel := context.WithCancel(s.ctx)
	task := &DownloadTask{
		ID:        taskID,
		Request:   req,
		Status:    DownloadStatusPending,
		Total:     req.Size,
		cancel:    cancel,
		cancelReq: false,
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()

	if err := s.upsertTask(task); err != nil {
		applog.LogErrorf(s.ctx, "failed to persist download task %s: %v", task.ID, err)
	}

	go s.runDownload(ctx, task)
	return taskID, nil
}

// CancelDownload 取消指定任务
func (s *DownloadService) CancelDownload(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}

	if task.Status == DownloadStatusDone || task.Status == DownloadStatusError || task.Status == DownloadStatusCancelled {
		s.mu.Unlock()
		return nil
	}

	task.pauseReq = false
	task.cancelReq = true
	status := task.Status
	cancel := task.cancel
	s.mu.Unlock()

	if status == DownloadStatusPaused {
		destPath := ""
		if path, err := s.getTaskDestPath(task.Request); err == nil {
			destPath = path
		}
		extractPath := downloadutils.BuildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
		s.cancelTaskAndCleanup(task, destPath, extractPath)
		return nil
	}

	if status == DownloadStatusExtracting {
		return nil
	}

	if cancel != nil {
		cancel()
	}
	return nil
}

// PauseDownload 暂停下载任务（保留已下载部分，可恢复）
func (s *DownloadService) PauseDownload(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status == DownloadStatusPaused {
		s.mu.Unlock()
		return nil
	}
	if task.Status != DownloadStatusDownloading && task.Status != DownloadStatusPending {
		s.mu.Unlock()
		return fmt.Errorf("task %s is not active", taskID)
	}
	task.pauseReq = true
	cancel := task.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// ResumeDownload 恢复已暂停任务
func (s *DownloadService) ResumeDownload(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != DownloadStatusPaused {
		s.mu.Unlock()
		return fmt.Errorf("task %s is not paused", taskID)
	}
	task.pauseReq = false
	task.cancelReq = false
	ctx, cancel := context.WithCancel(s.ctx)
	task.cancel = cancel
	task.Status = DownloadStatusPending
	task.Error = ""
	s.mu.Unlock()
	s.emitProgress(task)
	go s.runDownload(ctx, task)
	return nil
}

// GetDownloadTasks 返回所有任务快照
func (s *DownloadService) GetDownloadTasks() []DownloadTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]DownloadTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		cp := *t
		cp.cancel = nil
		result = append(result, cp)
	}
	return result
}

// DeleteDownloadTask 删除已结束的下载任务记录
func (s *DownloadService) DeleteDownloadTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if ok && (task.Status == DownloadStatusPending || task.Status == DownloadStatusDownloading) {
		return fmt.Errorf("cannot delete active task %s", taskID)
	}
	if ok && task.Status == DownloadStatusExtracting {
		return fmt.Errorf("cannot delete active task %s", taskID)
	}

	delete(s.tasks, taskID)
	if s.db == nil {
		return nil
	}

	if _, err := s.db.Exec(`DELETE FROM download_tasks WHERE id = ?`, taskID); err != nil {
		return fmt.Errorf("failed to delete download task %s: %w", taskID, err)
	}

	return nil
}

// OpenDownloadTaskLocation 打开下载任务对应文件所在位置
func (s *DownloadService) OpenDownloadTaskLocation(taskID string) error {
	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.FilePath == "" {
		return fmt.Errorf("task %s has no file path", taskID)
	}
	if err := apputils.OpenFileOrFolder(task.FilePath); err != nil {
		return fmt.Errorf("open download task location failed: %w", err)
	}
	return nil
}

// ImportDownloadTaskAsGame 将下载任务导入到游戏库（含元数据与可执行文件选择）
func (s *DownloadService) ImportDownloadTaskAsGame(taskID string) error {
	if s.gameService == nil {
		return fmt.Errorf("game service not initialized")
	}

	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != DownloadStatusDone {
		return fmt.Errorf("task %s is not completed", taskID)
	}
	if strings.TrimSpace(task.FilePath) == "" {
		return fmt.Errorf("task %s has no file path", taskID)
	}

	importPath, resolvedByStartupPath, err := resolveExecutablePathFromRequest(task.FilePath, task.Request.StartupPath)
	if err != nil {
		return fmt.Errorf("resolve startup_path: %w", err)
	}
	if !resolvedByStartupPath {
		importPath, err = s.gameService.ResolveExecutablePathForImport(task.FilePath)
		if err != nil {
			applog.LogErrorf(s.ctx, "resolve executable path for task %s failed: %v", task.ID, err)
			return fmt.Errorf("resolve executable path: %w", err)
		}
		importPath = strings.TrimSpace(importPath)
		if importPath == "" {
			return fmt.Errorf("select executable cancelled")
		}
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)
	metadata := s.fetchMetadataForTask(task)

	if sourceOk && metaID != "" {
		if existingID, exists := s.gameService.findGameIDBySource(metaSource, metaID); exists {
			s.updateExistingGame(existingID, importPath, metaSource, metaID, metadata)
			applog.LogInfof(s.ctx, "import task %s as game: updated existing game by source", task.ID)
			return nil
		}
	}

	if existingID, exists := s.gameService.findGameIDByPath(importPath); exists {
		s.updateExistingGame(existingID, importPath, metaSource, metaID, metadata)
		applog.LogInfof(s.ctx, "import task %s as game: updated existing game by path", task.ID)
		return nil
	}

	game := models.Game{
		Name:       strings.TrimSpace(task.Request.Title),
		Path:       importPath,
		SourceType: enums.Local,
		SourceID:   "",
		Status:     enums.StatusNotStarted,
	}

	if sourceOk {
		game.SourceType = metaSource
		game.SourceID = metaID
	}

	if metadata != nil {
		mergeMetadataIntoGame(&game, metadata.Game)
		game.Path = importPath
	}

	if sourceOk && game.SourceType == enums.Local {
		game.SourceType = metaSource
	}
	if game.SourceID == "" {
		game.SourceID = metaID
	}
	if strings.TrimSpace(game.Name) == "" {
		game.Name = "未知标题"
	}

	var addErr error
	if metadata != nil {
		metaToSave := *metadata
		metaToSave.Game = game
		addErr = s.gameService.AddGameFromWebMetadata(metaToSave)
	} else {
		addErr = s.gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
		})
	}
	if addErr != nil {
		applog.LogErrorf(s.ctx, "import task %s as game failed: %v", task.ID, addErr)
		return fmt.Errorf("add game: %w", addErr)
	}

	applog.LogInfof(s.ctx, "import task %s as game success: %s", task.ID, game.Name)
	return nil
}

// =================== 内部下载逻辑 ===================

func (s *DownloadService) emitProgress(task *DownloadTask) {
	if err := s.upsertTask(task); err != nil {
		applog.LogErrorf(s.ctx, "failed to persist download task progress %s: %v", task.ID, err)
	}

	if s.ctx == nil {
		return
	}
	runtime.EventsEmit(s.ctx, "download:progress", DownloadProgressEvent{
		ID:         task.ID,
		Request:    task.Request,
		Status:     task.Status,
		Progress:   task.Progress,
		Downloaded: task.Downloaded,
		Total:      task.Total,
		Error:      task.Error,
		FilePath:   task.FilePath,
	})
}

func (s *DownloadService) runDownload(ctx context.Context, task *DownloadTask) {
	applog.LogInfof(s.ctx, "Download started: %s  url=%s", task.ID, task.Request.URL)

	if err := validateInstallRequest(task.Request); err != nil {
		s.failTask(task, fmt.Sprintf("invalid install request: %v", err))
		return
	}

	destPath, extractPath, client, err := s.prepareDownloadExecution(task)
	if err != nil {
		s.failTask(task, err.Error())
		return
	}

	resp, err := s.downloadWithGrab(ctx, task, client, destPath)
	if err != nil {
		if s.handleGrabDownloadInterruption(task, err, destPath, extractPath) {
			return
		}
		s.failTask(task, formatGrabDownloadError(task, err))
		return
	}
	if resp != nil {
		s.updateTaskProgressFromGrab(task, resp)
	}

	finalPath, manualExtractRequired, handled, err := s.postProcessDownloadedTask(task, destPath, extractPath)
	if handled {
		return
	}
	if err != nil {
		s.failTask(task, err.Error())
		return
	}

	s.completeDownloadTask(task, finalPath, manualExtractRequired)

	// 先抓取元数据，再把元数据用于自动创建/更新游戏记录
	metadata := s.fetchMetadataForTask(task)
	s.autoCreateOrUpdateGame(task, finalPath, metadata)
}

func (s *DownloadService) failTask(task *DownloadTask, msg string) {
	applog.LogErrorf(s.ctx, "Download error [%s]: %s", task.ID, msg)
	s.mu.Lock()
	task.Status = DownloadStatusError
	task.Error = msg
	s.mu.Unlock()
	s.emitProgress(task)
}

func (s *DownloadService) loadTasksFromDB() error {
	if s.db == nil {
		return nil
	}

	rows, err := s.db.Query(`
		SELECT id, request_json, status, progress, downloaded, total, error, file_path
		FROM download_tasks
	`)
	if err != nil {
		return fmt.Errorf("query download_tasks: %w", err)
	}
	defer rows.Close()

	loaded := make(map[string]*DownloadTask)
	for rows.Next() {
		var (
			id          string
			requestJSON string
			status      string
			progress    float64
			downloaded  int64
			total       int64
			errorMsg    sql.NullString
			filePath    sql.NullString
		)

		if err := rows.Scan(&id, &requestJSON, &status, &progress, &downloaded, &total, &errorMsg, &filePath); err != nil {
			return fmt.Errorf("scan download task: %w", err)
		}

		var request vo.InstallRequest
		if requestJSON != "" {
			if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
				applog.LogErrorf(s.ctx, "failed to unmarshal request_json for task %s: %v", id, err)
			}
		}

		taskStatus := DownloadStatus(status)
		taskError := errorMsg.String
		if taskStatus == DownloadStatusPending || taskStatus == DownloadStatusDownloading || taskStatus == DownloadStatusExtracting {
			taskStatus = DownloadStatusError
			if taskError == "" {
				taskError = "download interrupted by app restart"
			}
		}

		loaded[id] = &DownloadTask{
			ID:         id,
			Request:    request,
			Status:     taskStatus,
			Progress:   progress,
			Downloaded: downloaded,
			Total:      total,
			Error:      taskError,
			FilePath:   filePath.String,
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate download tasks: %w", err)
	}

	s.mu.Lock()
	for id, task := range loaded {
		s.tasks[id] = task
	}
	s.mu.Unlock()

	for _, task := range loaded {
		if err := s.upsertTask(task); err != nil {
			applog.LogErrorf(s.ctx, "failed to normalize loaded task %s: %v", task.ID, err)
		}
	}

	return nil
}

func (s *DownloadService) getTaskDestPath(req vo.InstallRequest) (string, error) {
	dir, err := s.getDownloadDir()
	if err != nil {
		return "", err
	}
	name := downloadutils.SanitizeDownloadedFileName(req.FileName)
	if name == "" {
		return "", fmt.Errorf("invalid file_name")
	}
	return filepath.Join(dir, name), nil
}

func (s *DownloadService) upsertTask(task *DownloadTask) error {
	if s.db == nil {
		return nil
	}

	requestJSON, err := json.Marshal(task.Request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO download_tasks (
			id, request_json, status, progress, downloaded, total, error, file_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			request_json = excluded.request_json,
			status = excluded.status,
			progress = excluded.progress,
			downloaded = excluded.downloaded,
			total = excluded.total,
			error = excluded.error,
			file_path = excluded.file_path,
			updated_at = now()
	`, task.ID, string(requestJSON), string(task.Status), task.Progress, task.Downloaded, task.Total, task.Error, task.FilePath)
	if err != nil {
		return fmt.Errorf("upsert download task: %w", err)
	}

	return nil
}

func (s *DownloadService) fetchMetadataForTask(task *DownloadTask) *vo.GameMetadataFromWebVO {
	if s.gameService == nil {
		return nil
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)
	if !sourceOk || metaID == "" {
		return nil
	}

	metaResult, err := s.gameService.FetchMetadataFromWeb(vo.MetadataRequest{Source: metaSource, ID: metaID})
	if err != nil {
		applog.LogWarningf(s.ctx, "fetch metadata failed for download task %s (source=%s id=%s): %v", task.ID, metaSource, metaID, err)
		return nil
	}

	applog.LogInfof(s.ctx, "fetch metadata success for download task %s: %s", task.ID, metaResult.Game.Name)
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "download:metadata-prefetched", map[string]interface{}{
			"task_id":     task.ID,
			"meta_source": string(metaSource),
			"meta_id":     metaID,
			"game":        metaResult.Game,
		})
	}
	return &metaResult
}

func (s *DownloadService) handleDownloadedFile(downloadedPath string, fileName string, archiveFormat string, title string) (string, bool, error) {
	format := downloadutils.NormalizeArchiveFormat(archiveFormat)
	if format == "none" {
		return downloadedPath, false, nil
	}
	if !downloadutils.IsSupportedArchiveFormat(format) {
		return "", false, fmt.Errorf("unsupported archive_format: %s", archiveFormat)
	}

	baseName := downloadutils.TrimArchiveSuffixByFormat(strings.TrimSpace(fileName), format)
	baseName = downloadutils.SanitizeFileName(baseName)
	if baseName == "" {
		baseName = downloadutils.SanitizeFileName(title)
	}
	if baseName == "" {
		baseName = "game"
	}

	extractDir := filepath.Join(filepath.Dir(downloadedPath), baseName)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", false, fmt.Errorf("create extract dir: %w", err)
	}

	extracted, extractErr := archiveutils.ExtractArchive(downloadedPath, extractDir)
	if extractErr != nil {
		if !extracted {
			applog.LogErrorf(s.ctx, "extract archive failed, fallback to manual extract mode: %v", extractErr)
			applog.LogWarningf(s.ctx, "archive kept at %s, created/kept empty dir %s for manual extraction", downloadedPath, extractDir)
			return extractDir, true, nil
		}
		return "", false, fmt.Errorf("extract archive: %w", extractErr)
	}

	if err := os.Remove(downloadedPath); err != nil {
		applog.LogWarningf(s.ctx, "failed to delete source archive after unzip: %v", err)
	}

	finalExtractDir := extractDir
	if collapsed, ok := collapseSingleRootDirectory(extractDir); ok {
		finalExtractDir = collapsed
	}

	return finalExtractDir, false, nil
}

func (s *DownloadService) autoCreateOrUpdateGame(task *DownloadTask, gamePath string, metadata *vo.GameMetadataFromWebVO) {
	if s.gameService == nil {
		return
	}

	importPath := gamePath
	if resolvedPath, ok, err := resolveExecutablePathFromRequest(gamePath, task.Request.StartupPath); err != nil {
		applog.LogWarningf(s.ctx, "invalid startup_path for task %s: %v", task.ID, err)
	} else if ok {
		importPath = resolvedPath
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)

	if sourceOk && metaID != "" {
		if existingID, ok := s.gameService.findGameIDBySource(metaSource, metaID); ok {
			s.updateExistingGame(existingID, importPath, metaSource, metaID, metadata)
			return
		}
	}

	if existingID, ok := s.gameService.findGameIDByPath(importPath); ok {
		s.updateExistingGame(existingID, importPath, metaSource, metaID, metadata)
		return
	}

	game := models.Game{
		Name:       strings.TrimSpace(task.Request.Title),
		Path:       importPath,
		SourceType: enums.Local,
		SourceID:   "",
	}

	if sourceOk {
		game.SourceType = metaSource
		game.SourceID = metaID
	}

	if metadata != nil {
		mergeMetadataIntoGame(&game, metadata.Game)
		game.Path = importPath
	}

	if sourceOk && game.SourceType == enums.Local {
		game.SourceType = metaSource
	}
	if game.SourceID == "" {
		game.SourceID = metaID
	}

	if strings.TrimSpace(game.Name) == "" {
		game.Name = strings.TrimSuffix(filepath.Base(importPath), filepath.Ext(importPath))
	}

	var addErr error
	if metadata != nil {
		metaToSave := *metadata
		metaToSave.Game = game
		addErr = s.gameService.AddGameFromWebMetadata(metaToSave)
	} else {
		addErr = s.gameService.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
		})
	}
	if addErr != nil {
		applog.LogWarningf(s.ctx, "auto import game failed for task %s: %v", task.ID, addErr)
		return
	}

	applog.LogInfof(s.ctx, "auto import game success for task %s: %s", task.ID, game.Name)
}

func (s *DownloadService) updateExistingGame(gameID string, gamePath string, metaSource enums.SourceType, metaID string, metadata *vo.GameMetadataFromWebVO) {
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		applog.LogWarningf(s.ctx, "failed to load existing game %s for path update: %v", gameID, err)
		return
	}

	changed := false
	if game.Path != gamePath {
		game.Path = gamePath
		changed = true
	}

	if metadata != nil {
		if mergeMetadataIntoGame(&game, metadata.Game) {
			changed = true
		}
	}

	if metaSource != enums.Local && game.SourceType != metaSource {
		game.SourceType = metaSource
		changed = true
	}
	if metaID != "" && game.SourceID != metaID {
		game.SourceID = metaID
		changed = true
	}

	if !changed {
		return
	}

	if err := s.gameService.UpdateGame(game); err != nil {
		applog.LogWarningf(s.ctx, "failed to update existing game %s: %v", gameID, err)
	}
}

func mergeMetadataIntoGame(target *models.Game, metadata models.Game) bool {
	changed := false

	if name := strings.TrimSpace(metadata.Name); name != "" && target.Name != name {
		target.Name = name
		changed = true
	}
	if coverURL := strings.TrimSpace(metadata.CoverURL); coverURL != "" && target.CoverURL != coverURL {
		target.CoverURL = coverURL
		changed = true
	}
	if company := strings.TrimSpace(metadata.Company); company != "" && target.Company != company {
		target.Company = company
		changed = true
	}
	if summary := strings.TrimSpace(metadata.Summary); summary != "" && target.Summary != summary {
		target.Summary = summary
		changed = true
	}
	if metadata.Rating > 0 && target.Rating != metadata.Rating {
		target.Rating = metadata.Rating
		changed = true
	}
	if releaseDate := strings.TrimSpace(metadata.ReleaseDate); releaseDate != "" && target.ReleaseDate != releaseDate {
		target.ReleaseDate = releaseDate
		changed = true
	}
	if metadata.SourceType != "" && target.SourceType != metadata.SourceType {
		target.SourceType = metadata.SourceType
		changed = true
	}
	if sourceID := strings.TrimSpace(metadata.SourceID); sourceID != "" && target.SourceID != sourceID {
		target.SourceID = sourceID
		changed = true
	}
	if !metadata.CachedAt.IsZero() && !target.CachedAt.Equal(metadata.CachedAt) {
		target.CachedAt = metadata.CachedAt
		changed = true
	}

	return changed
}

func parseMetaSource(metaSource string) (enums.SourceType, bool) {
	switch strings.ToLower(strings.TrimSpace(metaSource)) {
	case string(enums.Bangumi):
		return enums.Bangumi, true
	case string(enums.VNDB):
		return enums.VNDB, true
	case string(enums.Ymgal):
		return enums.Ymgal, true
	case string(enums.Steam):
		return enums.Steam, true
	default:
		return enums.Local, false
	}
}

func normalizeGamePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned := filepath.Clean(trimmed)
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

func (s *DownloadService) isTaskCancelled(task *DownloadTask) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return task.cancelReq
}

func (s *DownloadService) isTaskPauseRequested(task *DownloadTask) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return task.pauseReq
}

func (s *DownloadService) markTaskPaused(task *DownloadTask) {
	s.mu.Lock()
	task.Status = DownloadStatusPaused
	task.Error = ""
	task.pauseReq = false
	task.cancelReq = false
	s.mu.Unlock()
	s.emitProgress(task)
}

func (s *DownloadService) cancelTaskAndCleanup(task *DownloadTask, paths ...string) {
	s.cleanupDownloadArtifacts(paths...)

	s.mu.Lock()
	task.Status = DownloadStatusCancelled
	task.Error = ""
	task.Progress = 0
	task.Downloaded = 0
	task.Total = task.Request.Size
	task.FilePath = ""
	task.pauseReq = false
	task.cancelReq = false
	s.mu.Unlock()

	s.emitProgress(task)
}

func (s *DownloadService) cleanupDownloadArtifacts(paths ...string) {
	seen := make(map[string]struct{})
	for _, rawPath := range paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}

		info, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				applog.LogWarningf(s.ctx, "failed to stat path while cleanup: %s err=%v", path, err)
			}
			continue
		}

		if info.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				applog.LogWarningf(s.ctx, "failed to remove dir while cleanup: %s err=%v", path, err)
			}
			continue
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			applog.LogWarningf(s.ctx, "failed to remove file while cleanup: %s err=%v", path, err)
		}
	}
}

func (s *DownloadService) prepareDownloadExecution(task *DownloadTask) (string, string, *grab.Client, error) {
	destPath, err := s.getTaskDestPath(task.Request)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve download path: %w", err)
	}
	extractPath := downloadutils.BuildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)

	resumeOffset := s.inspectResumeOffset(task, destPath)
	client, proxyDesc, err := newGrabDownloadClient(s.config)
	if err != nil {
		return "", "", nil, fmt.Errorf("create download client: %w", err)
	}

	applog.LogInfof(s.ctx, "Download proxy for task %s: %s", task.ID, proxyDesc)
	s.markTaskDownloading(task, resumeOffset)

	return destPath, extractPath, client, nil
}

func (s *DownloadService) inspectResumeOffset(task *DownloadTask, destPath string) int64 {
	resumeOffset := int64(0)
	if fileInfo, err := os.Stat(destPath); err == nil && !fileInfo.IsDir() {
		resumeOffset = fileInfo.Size()
	}
	if resumeOffset > task.Request.Size {
		applog.LogWarningf(s.ctx, "partial file exceeds declared size for task %s, reset partial file", task.ID)
		_ = os.Remove(destPath)
		return 0
	}
	return resumeOffset
}

func (s *DownloadService) markTaskDownloading(task *DownloadTask, resumeOffset int64) {
	progress := 0.0
	if resumeOffset > 0 && task.Request.Size > 0 {
		progress = float64(resumeOffset) / float64(task.Request.Size) * 100
	}

	s.mu.Lock()
	task.Status = DownloadStatusDownloading
	task.Progress = progress
	task.Downloaded = resumeOffset
	task.Total = task.Request.Size
	task.pauseReq = false
	task.cancelReq = false
	task.Error = ""
	task.FilePath = ""
	s.mu.Unlock()

	s.emitProgress(task)
}

func (s *DownloadService) handleGrabDownloadInterruption(task *DownloadTask, err error, destPath string, extractPath string) bool {
	if !errors.Is(err, context.Canceled) {
		return false
	}

	if s.isTaskPauseRequested(task) {
		s.markTaskPaused(task)
		applog.LogInfof(s.ctx, "Download paused: %s", task.ID)
		return true
	}

	s.cancelTaskAndCleanup(task, destPath, extractPath)
	applog.LogInfof(s.ctx, "Download cancelled: %s", task.ID)
	return true
}

func (s *DownloadService) postProcessDownloadedTask(task *DownloadTask, destPath string, extractPath string) (string, bool, bool, error) {
	s.markTaskExtracting(task)

	if s.isTaskCancelled(task) {
		s.cancelTaskAndCleanup(task, destPath, extractPath)
		return "", false, true, nil
	}

	finalPath, manualExtractRequired, err := s.handleDownloadedFile(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
	if err != nil {
		if s.isTaskCancelled(task) {
			s.cancelTaskAndCleanup(task, destPath, extractPath)
			return "", false, true, nil
		}
		return "", false, false, fmt.Errorf("post process download file: %w", err)
	}

	finalPath, err = normalizeGamePath(finalPath)
	if err != nil {
		if s.isTaskCancelled(task) {
			s.cancelTaskAndCleanup(task, destPath, extractPath, finalPath)
			return "", false, true, nil
		}
		return "", false, false, fmt.Errorf("normalize game path: %w", err)
	}

	if s.isTaskCancelled(task) {
		s.cancelTaskAndCleanup(task, destPath, extractPath, finalPath)
		return "", false, true, nil
	}

	return finalPath, manualExtractRequired, false, nil
}

func (s *DownloadService) markTaskExtracting(task *DownloadTask) {
	s.mu.Lock()
	task.Status = DownloadStatusExtracting
	s.mu.Unlock()
	s.emitProgress(task)
}

func (s *DownloadService) completeDownloadTask(task *DownloadTask, finalPath string, manualExtractRequired bool) {
	s.mu.Lock()
	task.Status = DownloadStatusDone
	task.Progress = 100
	task.FilePath = finalPath
	if manualExtractRequired {
		task.Error = DownloadManualExtractFlag
	} else {
		task.Error = ""
	}
	s.mu.Unlock()

	s.emitProgress(task)
	applog.LogInfof(s.ctx, "Download complete: %s  path=%s", task.ID, finalPath)
}

func (s *DownloadService) downloadWithGrab(ctx context.Context, task *DownloadTask, client *grab.Client, destPath string) (*grab.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.runGrabAttempt(ctx, task, client, destPath)
		if err != nil && attempt == 0 && shouldRetryGrabFromScratch(err, destPath) {
			applog.LogWarningf(s.ctx, "grab resume failed for task %s, reset partial file and retry full download: %v", task.ID, err)
			if removeErr := os.Remove(destPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return resp, fmt.Errorf("reset partial download: %w", removeErr)
			}
			s.mu.Lock()
			task.Downloaded = 0
			task.Progress = 0
			s.mu.Unlock()
			s.emitProgress(task)
			continue
		}
		return resp, err
	}

	return nil, fmt.Errorf("download failed after retry")
}

func (s *DownloadService) runGrabAttempt(ctx context.Context, task *DownloadTask, client *grab.Client, destPath string) (*grab.Response, error) {
	req, err := newGrabDownloadRequest(ctx, task, destPath)
	if err != nil {
		return nil, err
	}

	resp := client.Do(req)
	s.updateTaskProgressFromGrab(task, resp)
	s.emitProgress(task)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateTaskProgressFromGrab(task, resp)
			s.emitProgress(task)
		case <-resp.Done:
			s.updateTaskProgressFromGrab(task, resp)
			s.emitProgress(task)
			return resp, resp.Err()
		}
	}
}

func (s *DownloadService) updateTaskProgressFromGrab(task *DownloadTask, resp *grab.Response) {
	if resp == nil {
		return
	}

	downloaded := resp.BytesComplete()
	total := task.Request.Size
	if size := resp.Size(); size > 0 {
		total = size
	}

	progress := 0.0
	if total > 0 {
		progress = float64(downloaded) / float64(total) * 100
		if progress > 100 {
			progress = 100
		}
	}

	s.mu.Lock()
	task.Downloaded = downloaded
	task.Total = total
	task.Progress = progress
	s.mu.Unlock()
}

func newGrabDownloadRequest(ctx context.Context, task *DownloadTask, destPath string) (*grab.Request, error) {
	req, err := grab.NewRequest(destPath, task.Request.URL)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req = req.WithContext(ctx)
	req.Size = task.Request.Size
	req.IgnoreRemoteTime = true
	req.HTTPRequest.Header.Set("User-Agent", downloadUserAgent)

	checksumHash, checksumBytes, err := newGrabChecksumState(task.Request.ChecksumAlgo, task.Request.Checksum)
	if err != nil {
		return nil, fmt.Errorf("configure checksum: %w", err)
	}
	req.SetChecksum(checksumHash, checksumBytes, false)

	return req, nil
}

func newGrabDownloadClient(config *appconf.AppConfig) (*grab.Client, string, error) {
	httpClient, proxyDesc, err := newSecureDownloadClient(config)
	if err != nil {
		return nil, "", err
	}

	client := grab.NewClient()
	client.HTTPClient = httpClient
	client.UserAgent = downloadUserAgent

	return client, proxyDesc, nil
}

func shouldRetryGrabFromScratch(err error, destPath string) bool {
	if strings.TrimSpace(destPath) == "" {
		return false
	}

	if info, statErr := os.Stat(destPath); statErr != nil || info.IsDir() || info.Size() <= 0 {
		return false
	}

	var statusErr grab.StatusCodeError
	return errors.As(err, &statusErr) && int(statusErr) == http.StatusRequestedRangeNotSatisfiable
}

func formatGrabDownloadError(task *DownloadTask, err error) string {
	switch {
	case errors.Is(err, grab.ErrBadLength):
		return fmt.Sprintf("size mismatch during download: expected=%d", task.Request.Size)
	case errors.Is(err, grab.ErrBadChecksum):
		return "checksum verify failed: checksum mismatch"
	default:
		return fmt.Sprintf("download failed: %v", err)
	}
}

func collapseSingleRootDirectory(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	if len(entries) != 1 {
		return "", false
	}

	only := entries[0]
	if !only.IsDir() {
		return "", false
	}

	return filepath.Join(dir, only.Name()), true
}

// =================== 辅助函数 ===================

func (s *DownloadService) getDownloadDir() (string, error) {
	if s.config != nil && s.config.GameLibraryPath != "" {
		return s.config.GameLibraryPath, os.MkdirAll(s.config.GameLibraryPath, 0755)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Games")
	return dir, os.MkdirAll(dir, 0755)
}

func newGrabChecksumState(algo string, checksum string) (hash.Hash, []byte, error) {
	trimmedAlgo := strings.ToLower(strings.TrimSpace(algo))
	trimmedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	if trimmedAlgo == "" {
		return nil, nil, nil
	}

	sum, err := hex.DecodeString(trimmedChecksum)
	if err != nil {
		return nil, nil, fmt.Errorf("decode checksum: %w", err)
	}

	switch trimmedAlgo {
	case "sha256":
		return sha256.New(), sum, nil
	case "blake3":
		return blake3.New(), sum, nil
	default:
		return nil, nil, fmt.Errorf("unsupported checksum algo: %s", algo)
	}
}

func isBlockedHostname(host string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(host))
	return trimmed == "localhost" || strings.HasSuffix(trimmed, ".localhost")
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified()
}

func resolveAllowedAddress(ctx context.Context, address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if isBlockedHostname(host) {
		return "", fmt.Errorf("blocked host: %s", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return "", fmt.Errorf("blocked ip: %s", host)
		}
		return net.JoinHostPort(ip.String(), port), nil
	}

	resolver := net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			continue
		}
		return net.JoinHostPort(ip.IP.String(), port), nil
	}

	return "", fmt.Errorf("host %s resolved only to blocked addresses", host)
}

func newSecureDownloadClient(config *appconf.AppConfig) (*http.Client, string, error) {
	var mode string
	var proxyURL string
	if config != nil {
		mode = config.DownloadProxyMode
		proxyURL = config.DownloadProxyURL
	}

	selection, proxyDesc, err := proxyutils.ResolveDownloadProxy(mode, proxyURL)
	if err != nil {
		return nil, "", fmt.Errorf("resolve download proxy: %w", err)
	}

	allowedProxyTargets := map[string]struct{}{}
	if selection != nil {
		allowedProxyTargets = selection.AllowedDialTargets()
	}

	dialer := &net.Dialer{}
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if selection == nil {
				return nil, nil
			}
			return selection.Proxy(req)
		},
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			if _, ok := allowedProxyTargets[address]; ok {
				return dialer.DialContext(ctx, network, address)
			}

			resolvedAddress, err := resolveAllowedAddress(ctx, address)
			if err != nil {
				return nil, err
			}
			return dialer.DialContext(ctx, network, resolvedAddress)
		},
	}

	return &http.Client{
		Timeout:   0,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return downloadutils.ValidateDownloadURL(req.URL.String())
		},
	}, proxyDesc, nil
}

func validateInstallRequest(req vo.InstallRequest) error {
	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("missing url")
	}
	if err := downloadutils.ValidateDownloadURL(req.URL); err != nil {
		return err
	}
	if downloadutils.SanitizeDownloadedFileName(req.FileName) == "" {
		return fmt.Errorf("missing or invalid file_name")
	}

	format := downloadutils.NormalizeArchiveFormat(req.ArchiveFormat)
	if format == "" {
		return fmt.Errorf("missing archive_format")
	}
	if !downloadutils.IsSupportedArchiveFormat(format) {
		return fmt.Errorf("unsupported archive_format: %s", req.ArchiveFormat)
	}

	if _, _, err := resolveExecutablePathFromRequest("", req.StartupPath); err != nil {
		return fmt.Errorf("invalid startup_path: %w", err)
	}

	if req.Size <= 0 {
		return fmt.Errorf("size is required and must be > 0")
	}
	if req.ExpiresAt <= 0 {
		return fmt.Errorf("expires_at is required")
	}
	if req.ExpiresAt <= time.Now().Unix() {
		return fmt.Errorf("install request expired")
	}

	algo := strings.ToLower(strings.TrimSpace(req.ChecksumAlgo))
	checksum := strings.ToLower(strings.TrimSpace(req.Checksum))
	if err := downloadutils.ValidateChecksumFields(algo, checksum); err != nil {
		return err
	}

	return nil
}

func resolveExecutablePathFromRequest(downloadPath string, startupPath string) (string, bool, error) {
	trimmedStartup := strings.TrimSpace(startupPath)
	if trimmedStartup == "" {
		return "", false, nil
	}

	normalized := strings.ReplaceAll(trimmedStartup, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", false, fmt.Errorf("must be relative path")
	}

	cleanRelative := filepath.Clean(strings.ReplaceAll(normalized, "/", string(filepath.Separator)))
	if cleanRelative == "." || cleanRelative == "" {
		return "", false, fmt.Errorf("must not be empty")
	}
	if filepath.IsAbs(cleanRelative) {
		return "", false, fmt.Errorf("must be relative path")
	}
	if strings.HasPrefix(cleanRelative, "..") {
		return "", false, fmt.Errorf("must not escape download directory")
	}

	if strings.TrimSpace(downloadPath) == "" {
		return "", false, nil
	}

	basePath := downloadPath
	if info, err := os.Stat(downloadPath); err == nil {
		if !info.IsDir() {
			basePath = filepath.Dir(downloadPath)
		}
	}

	cleanRelative = optimizeStartupRelativePath(basePath, cleanRelative)

	joined := filepath.Join(basePath, cleanRelative)
	absJoined, err := filepath.Abs(filepath.Clean(joined))
	if err != nil {
		return "", false, fmt.Errorf("normalize startup executable path: %w", err)
	}

	return absJoined, true, nil
}

func optimizeStartupRelativePath(basePath string, relativePath string) string {
	current := filepath.Clean(strings.TrimSpace(relativePath))
	if current == "" || current == "." {
		return relativePath
	}

	baseName := filepath.Base(filepath.Clean(basePath))
	if baseName == "" || baseName == "." {
		return current
	}

	for {
		first, rest, ok := splitFirstRelativeSegment(current)
		if !ok || rest == "" || rest == "." {
			break
		}
		if !pathSegmentEquals(first, baseName) {
			break
		}

		fullCurrent := filepath.Join(basePath, current)
		fullRest := filepath.Join(basePath, rest)
		currentExists := pathExists(fullCurrent)
		restExists := pathExists(fullRest)

		if restExists && !currentExists {
			current = rest
			continue
		}

		if !currentExists && !restExists {
			current = rest
			continue
		}

		break
	}

	return current
}

func splitFirstRelativeSegment(path string) (string, string, bool) {
	normalized := strings.Trim(filepath.ToSlash(path), "/")
	if normalized == "" {
		return "", "", false
	}
	parts := strings.Split(normalized, "/")
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], filepath.FromSlash(strings.Join(parts[1:], "/")), true
}

func pathSegmentEquals(a string, b string) bool {
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
