package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/gamehelper"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/archiveutils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/imageutils"
	metadatautils "lunabox/internal/utils/metadata"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"lunabox/internal/wailsruntime"
)

const (
	imageDownloadSource           = "cover-image-batch"
	downloadGameImportedEvent     = "download:game-imported"
	downloadGameImportFailedEvent = "download:game-import-failed"
	downloadMetadataFailedEvent   = "download:metadata-failed"
)

type downloadMetadataResult struct {
	metadata *vo.GameMetadataFromWebVO
	err      error
}

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
)

// DownloadTask 单个下载任务
type DownloadTask struct {
	ID         string            `json:"id"`
	Request    vo.InstallRequest `json:"request"`
	Status     DownloadStatus    `json:"status"`
	CreatedAt  *time.Time        `json:"created_at,omitempty"`
	Progress   float64           `json:"progress"`   // 0~100
	Downloaded int64             `json:"downloaded"` // bytes downloaded
	Total      int64             `json:"total"`      // bytes total (0 = unknown)
	Error      string            `json:"error,omitempty"`
	FilePath   string            `json:"file_path,omitempty"` // 下载完成后的本地路径
	cancel     context.CancelFunc
	pauseReq   bool
	cancelReq  bool
	// downloadCompleted 表示本次运行中下载已校验通过并重命名到最终路径，
	// 此后 destPath 上的文件属于本任务产物，清理时才可以删除它
	downloadCompleted bool
	// coverItems 仅用于封面批量任务：在内存中保留"上一轮仍需处理的项"，
	// 首跑时是全量列表，失败重试时是上一次的失败项。重启进程后该字段为空，
	// 此时图片任务不再支持按粒度重试（与现有持久化策略一致）。
	coverItems []CoverImageDownloadItem
}

// DownloadProgressEvent 通过 Wails event 推送的进度事件
type DownloadProgressEvent struct {
	ID         string            `json:"id"`
	Request    vo.InstallRequest `json:"request"`
	Status     DownloadStatus    `json:"status"`
	CreatedAt  *time.Time        `json:"created_at,omitempty"`
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
	runtime        wailsruntime.Runtime
	emitEvent      func(string, ...interface{})
	mu             sync.RWMutex
	tasks          map[string]*DownloadTask
	pendingInstall *vo.InstallRequest // 从 lunabox:// URI 传入的待安装请求，在 GUI 就绪前暂存
}

func NewDownloadService() *DownloadService {
	runtime := wailsruntime.Unavailable()
	return &DownloadService{
		tasks:     make(map[string]*DownloadTask),
		runtime:   runtime,
		emitEvent: func(name string, data ...interface{}) { runtime.Emit(name, data...) },
	}
}

//wails:ignore
func (s *DownloadService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	if err := s.loadTasksFromDB(); err != nil {
		applog.LogErrorf(s.ctx, "failed to load download tasks from db: %v", err)
	}
}

//wails:ignore
func (s *DownloadService) SetRuntime(runtime wailsruntime.Runtime) {
	if runtime == nil {
		return
	}
	s.runtime = runtime
	s.emitEvent = func(name string, data ...interface{}) {
		runtime.Emit(name, data...)
	}
}

// SetGameService 注入游戏服务（用于下载完成后预抓取元数据）
//
//wails:ignore
func (s *DownloadService) SetGameService(gameService *GameService) {
	s.gameService = gameService
}

// SetPendingInstall 在 Wails 启动前由 main.go 调用，暂存待安装请求
//
//wails:ignore
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
	createdAt := time.Now()

	ctx, cancel := context.WithCancel(s.ctx)
	task := &DownloadTask{
		ID:        taskID,
		Request:   req,
		Status:    DownloadStatusPending,
		CreatedAt: &createdAt,
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

// StartCoverImageDownloadTask creates a lightweight download-management task for batch cover caching.
func (s *DownloadService) StartCoverImageDownloadTask(items []CoverImageDownloadItem) string {
	normalized := normalizeCoverImageDownloadItems(items)
	if len(normalized) == 0 {
		return ""
	}

	taskID := uuid.New().String()
	createdAt := time.Now()
	ctx, cancel := context.WithCancel(s.ctx)
	task := &DownloadTask{
		ID: taskID,
		Request: vo.InstallRequest{
			Title:          "批量下载游戏图片",
			DownloadSource: imageDownloadSource,
			FileName:       "cover-images",
			ArchiveFormat:  "none",
			Size:           int64(len(normalized)),
			ExpiresAt:      time.Now().Add(24 * time.Hour).Unix(),
		},
		Status:     DownloadStatusPending,
		CreatedAt:  &createdAt,
		Total:      int64(len(normalized)),
		cancel:     cancel,
		cancelReq:  false,
		coverItems: normalized,
	}

	s.mu.Lock()
	s.tasks[taskID] = task
	s.mu.Unlock()
	s.emitProgress(task)

	go s.runCoverImageDownloadTask(ctx, task, normalized)
	return taskID
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
		extractPath := ""
		if path, err := s.getTaskDestPath(task.Request); err == nil {
			destPath = path
			extractPath, _ = s.getTaskExtractPath(task.Request, destPath)
		}
		extractStagingPath := downloadTaskExtractStagingPath(extractPath, task.ID)
		// 暂停中的任务从未重命名到最终路径，destPath 不属于它
		s.cancelTaskAndCleanup(task, downloadTaskPartialCleanupPaths(destPath, extractStagingPath)...)
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
	if task.Request.DownloadSource == imageDownloadSource {
		s.mu.Unlock()
		return fmt.Errorf("image download task cannot be resumed")
	}
	ctx := s.requeueTaskLocked(task)
	s.mu.Unlock()
	s.emitProgress(task)
	go s.runDownload(ctx, task)
	return nil
}

// RetryDownload 重新尝试一个失败的下载任务
func (s *DownloadService) RetryDownload(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != DownloadStatusError {
		s.mu.Unlock()
		return fmt.Errorf("task %s is not retryable", taskID)
	}
	if task.Request.DownloadSource == imageDownloadSource {
		// 图片批量任务按粒度重试：只跑上一次失败的封面项
		pending := task.coverItems
		if len(pending) == 0 {
			s.mu.Unlock()
			return fmt.Errorf("no failed cover items to retry")
		}
		ctx := s.requeueTaskLocked(task)
		task.Total = int64(len(pending))
		task.Downloaded = 0
		task.Progress = 0
		s.mu.Unlock()
		s.emitProgress(task)
		go s.runCoverImageDownloadTask(ctx, task, pending)
		return nil
	}
	ctx := s.requeueTaskLocked(task)
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

func (s *DownloadService) libraryChangeBlockingTaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, task := range s.tasks {
		if task.Request.DownloadSource == imageDownloadSource {
			continue
		}
		switch task.Status {
		case DownloadStatusPending, DownloadStatusDownloading, DownloadStatusExtracting, DownloadStatusPaused, DownloadStatusError:
			count++
		}
	}
	return count
}

func (s *DownloadService) rebaseCompletedTaskPaths(oldRoot string, newRoot string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := 0
	for _, task := range s.tasks {
		if task.Status != DownloadStatusDone || strings.TrimSpace(task.FilePath) == "" {
			continue
		}
		newPath, matches := rebaseLibraryPath(task.FilePath, oldRoot, newRoot)
		if !matches {
			continue
		}
		task.FilePath = newPath
		updated++
	}
	return updated
}

func (s *DownloadService) CheckDownloadImportStates(requests []vo.DownloadImportStateRequest) ([]vo.DownloadImportState, error) {
	states := make([]vo.DownloadImportState, 0, len(requests))
	if len(requests) == 0 {
		return states, nil
	}

	for _, req := range requests {
		imported, err := s.isDownloadImportRequestImported(req)
		if err != nil {
			return nil, err
		}
		states = append(states, vo.DownloadImportState{
			TaskID:   req.TaskID,
			Imported: imported,
		})
	}
	return states, nil
}

func (s *DownloadService) isDownloadImportRequestImported(req vo.DownloadImportStateRequest) (bool, error) {
	filePath := strings.TrimSpace(req.FilePath)
	metaSource := strings.TrimSpace(req.MetaSource)
	metaID := strings.TrimSpace(req.MetaID)
	if filePath == "" && (metaSource == "" || metaID == "") {
		return false, nil
	}

	whereParts := make([]string, 0, 2)
	args := make([]interface{}, 0, 4)
	if filePath != "" {
		whereParts = append(whereParts, "path = ?")
		args = append(args, filePath)
	}
	if metaSource != "" && metaID != "" {
		whereParts = append(whereParts, "(LOWER(COALESCE(source_type, '')) = ? AND COALESCE(source_id, '') = ?)")
		args = append(args, strings.ToLower(metaSource), metaID)
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM games WHERE %s", strings.Join(whereParts, " OR "))
	var count int
	if err := s.db.QueryRowContext(s.ctx, query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("check download import state: %w", err)
	}
	return count > 0, nil
}

// DeleteDownloadTask 删除已结束的下载任务记录；未完成任务的残留文件一并清理
func (s *DownloadService) DeleteDownloadTask(taskID string) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if ok && (task.Status == DownloadStatusPending || task.Status == DownloadStatusDownloading || task.Status == DownloadStatusExtracting) {
		s.mu.Unlock()
		return fmt.Errorf("cannot delete active task %s", taskID)
	}

	delete(s.tasks, taskID)
	var status DownloadStatus
	var request vo.InstallRequest
	downloadCompleted := false
	if ok {
		status = task.Status
		request = task.Request
		downloadCompleted = task.downloadCompleted
	}
	s.mu.Unlock()

	// 未完成的任务（暂停/失败/取消）删除记录时同步清理磁盘上的半成品：
	// 临时下载文件、分段临时目录、可能存在的部分解压目录。
	// 只有本次运行中下载已落盘到最终路径（解压阶段失败）的任务才连 destPath 一起删；
	// 否则 destPath 上的文件可能是用户已有文件或之前完成的下载，必须保留。
	// 已完成任务的文件是用户的游戏数据，保留。
	if ok && status != DownloadStatusDone && request.DownloadSource != imageDownloadSource {
		if destPath, err := s.getTaskDestPath(request); err == nil {
			extractPath, _ := s.getTaskExtractPath(request, destPath)
			extractStagingPath := downloadTaskExtractStagingPath(extractPath, taskID)
			if downloadCompleted {
				s.cleanupDownloadArtifacts(downloadTaskCleanupPaths(destPath, extractStagingPath)...)
			} else {
				s.cleanupDownloadArtifacts(downloadTaskPartialCleanupPaths(destPath, extractStagingPath)...)
			}
		}
	}

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
	if task.Request.DownloadSource == imageDownloadSource {
		return fmt.Errorf("image download task cannot be imported as game")
	}
	if strings.TrimSpace(task.FilePath) == "" {
		return fmt.Errorf("task %s has no file path", taskID)
	}
	metadataResults := s.prefetchMetadataForTask(task)
	gameDirectory := gamehelper.DefaultGameDirectory(task.FilePath)

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

	return s.importWithPrefetchedMetadata(task, metadataResults, func(metadata *vo.GameMetadataFromWebVO) error {
		return s.importDownloadedGame(task, importPath, gameDirectory, metadata)
	})
}

// =================== 内部下载逻辑 ===================

func (s *DownloadService) emitProgress(task *DownloadTask) {
	if err := s.upsertTask(task); err != nil {
		applog.LogErrorf(s.ctx, "failed to persist download task progress %s: %v", task.ID, err)
	}

	if s.ctx == nil {
		return
	}
	s.runtime.Emit("download:progress", DownloadProgressEvent{
		ID:         task.ID,
		Request:    task.Request,
		Status:     task.Status,
		CreatedAt:  task.CreatedAt,
		Progress:   task.Progress,
		Downloaded: task.Downloaded,
		Total:      task.Total,
		Error:      task.Error,
		FilePath:   task.FilePath,
	})
}

func (s *DownloadService) emitGameImported(taskID string) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(downloadGameImportedEvent, map[string]string{
		"task_id": taskID,
	})
}

func (s *DownloadService) emitGameImportFailed(task *DownloadTask, err error) {
	s.emitDownloadTaskError(downloadGameImportFailedEvent, task, err)
}

func (s *DownloadService) emitMetadataFailed(task *DownloadTask, err error) {
	s.emitDownloadTaskError(downloadMetadataFailedEvent, task, err)
}

func (s *DownloadService) emitDownloadTaskError(eventName string, task *DownloadTask, err error) {
	if s.ctx == nil || s.emitEvent == nil || task == nil || err == nil {
		return
	}
	s.emitEvent(eventName, map[string]string{
		"task_id":     task.ID,
		"title":       strings.TrimSpace(task.Request.Title),
		"meta_source": strings.TrimSpace(task.Request.MetaSource),
		"meta_id":     strings.TrimSpace(task.Request.MetaID),
		"error":       err.Error(),
	})
}

func (s *DownloadService) runDownload(ctx context.Context, task *DownloadTask) {
	applog.LogInfof(s.ctx, "Download started: %s  url=%s", task.ID, task.Request.URL)

	if err := validateInstallRequest(task.Request); err != nil {
		s.failTask(task, fmt.Sprintf("invalid install request: %v", err))
		return
	}
	metadataResults := s.prefetchMetadataForTask(task)

	destPath, extractPath, downloader, err := s.prepareDownloadExecution(task)
	if err != nil {
		s.failTask(task, err.Error())
		return
	}

	err = downloader.Download(ctx, downloadutils.TransferRequest{
		URL:             task.Request.URL,
		DestinationPath: destPath,
		ExpectedSize:    task.Request.Size,
		ChecksumAlgo:    task.Request.ChecksumAlgo,
		Checksum:        task.Request.Checksum,
		Progress: func(progress downloadutils.Progress) {
			s.updateTaskProgress(task, progress.Downloaded, progress.Total)
			s.emitProgress(task)
		},
	})
	if err != nil {
		// 下载尚未完成，destPath 不属于本任务，只清理中间产物
		extractStagingPath := downloadTaskExtractStagingPath(extractPath, task.ID)
		if s.handleGrabDownloadInterruption(task, err, downloadTaskPartialCleanupPaths(destPath, extractStagingPath)...) {
			return
		}
		s.failTask(task, downloadutils.FormatDownloadError(task.Request.Size, err))
		return
	}

	s.mu.Lock()
	task.downloadCompleted = true
	s.mu.Unlock()

	finalPath, manualExtractRequired, handled, err := s.postProcessDownloadedTask(task, destPath, extractPath)
	if handled {
		return
	}
	if err != nil {
		s.failTask(task, err.Error())
		return
	}

	s.completeDownloadTask(task, finalPath, manualExtractRequired)

	if err := s.importWithPrefetchedMetadata(task, metadataResults, func(metadata *vo.GameMetadataFromWebVO) error {
		return s.autoCreateOrUpdateGame(task, finalPath, metadata)
	}); err != nil {
		applog.LogWarningf(s.ctx, "auto import game failed for task %s: %v", task.ID, err)
		s.emitGameImportFailed(task, err)
	}
}

func (s *DownloadService) runCoverImageDownloadTask(ctx context.Context, task *DownloadTask, items []CoverImageDownloadItem) {
	applog.LogInfof(s.ctx, "Cover image batch download started: %s count=%d", task.ID, len(items))
	s.mu.Lock()
	task.Status = DownloadStatusDownloading
	task.Progress = 0
	task.Downloaded = 0
	task.Total = int64(len(items))
	task.Error = ""
	s.mu.Unlock()
	s.emitProgress(task)

	success := 0
	failedItems := make([]CoverImageDownloadItem, 0)
	for index, item := range items {
		select {
		case <-ctx.Done():
			if s.isTaskPauseRequested(task) {
				s.markTaskPaused(task)
				return
			}
			s.cancelTaskAndCleanup(task)
			return
		default:
		}

		if ok := s.downloadAndUpdateCoverImage(ctx, item); ok {
			success++
		} else {
			failedItems = append(failedItems, item)
		}

		downloaded := int64(index + 1)
		progress := float64(downloaded) / float64(len(items)) * 100
		s.mu.Lock()
		task.Downloaded = downloaded
		task.Progress = progress
		if len(failedItems) > 0 {
			task.Error = fmt.Sprintf("success=%d failed=%d", success, len(failedItems))
		}
		s.mu.Unlock()
		s.emitProgress(task)
	}

	s.mu.Lock()
	task.Progress = 100
	task.Downloaded = int64(len(items))
	task.Total = int64(len(items))
	task.coverItems = failedItems
	if len(failedItems) > 0 {
		// 留在 error 状态，让用户可以在下载页对该任务再次点重试，只重跑失败项
		task.Status = DownloadStatusError
		task.Error = fmt.Sprintf("success=%d failed=%d", success, len(failedItems))
	} else {
		task.Status = DownloadStatusDone
		task.Error = ""
	}
	s.mu.Unlock()
	s.emitProgress(task)
	applog.LogInfof(s.ctx, "Cover image batch download complete: %s success=%d failed=%d", task.ID, success, len(failedItems))
}

func (s *DownloadService) downloadAndUpdateCoverImage(ctx context.Context, item CoverImageDownloadItem) bool {
	if strings.TrimSpace(item.GameID) == "" || strings.TrimSpace(item.CoverURL) == "" {
		return false
	}
	localPath, err := imageutils.DownloadAndSaveCoverImageWithProxyConfigContext(ctx, item.CoverURL, item.GameID, s.config)
	if err != nil {
		applog.LogWarningf(s.ctx, "cover image batch download failed for %s: %v", item.GameName, err)
		return false
	}
	if s.gameService != nil {
		if err := s.gameService.updateDownloadedCoverURL(item.GameID, localPath, item.CoverURL); err != nil {
			applog.LogWarningf(s.ctx, "cover image batch update failed for %s: %v", item.GameName, err)
			return false
		}
	}
	return true
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
		SELECT id, request_json, status, progress, downloaded, total, error, file_path,
		       created_at
		FROM download_tasks
	`)
	if err != nil {
		return fmt.Errorf("query download_tasks: %w", err)
	}
	defer rows.Close()

	loaded := make(map[string]*DownloadTask)
	tasksToNormalize := make([]*DownloadTask, 0)
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
			createdAt   sql.NullTime
		)

		if err := rows.Scan(&id, &requestJSON, &status, &progress, &downloaded, &total, &errorMsg, &filePath, &createdAt); err != nil {
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
		needsNormalization := false
		if taskStatus == DownloadStatusPending || taskStatus == DownloadStatusDownloading || taskStatus == DownloadStatusExtracting {
			taskStatus = DownloadStatusError
			needsNormalization = true
			if taskError == "" {
				taskError = "download interrupted by app restart"
			}
		}
		var normalizedCreatedAt *time.Time
		if createdAt.Valid {
			value := createdAt.Time
			normalizedCreatedAt = &value
		}

		task := &DownloadTask{
			ID:         id,
			Request:    request,
			Status:     taskStatus,
			CreatedAt:  normalizedCreatedAt,
			Progress:   progress,
			Downloaded: downloaded,
			Total:      total,
			Error:      taskError,
			FilePath:   filePath.String,
		}
		loaded[id] = task
		if needsNormalization {
			tasksToNormalize = append(tasksToNormalize, task)
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

	for _, task := range tasksToNormalize {
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
			id, request_json, status, progress, downloaded, total, error, file_path, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			request_json = excluded.request_json,
			status = excluded.status,
			progress = excluded.progress,
			downloaded = excluded.downloaded,
			total = excluded.total,
			error = excluded.error,
			file_path = excluded.file_path,
			updated_at = now()
	`, task.ID, string(requestJSON), string(task.Status), task.Progress, task.Downloaded, task.Total, task.Error, task.FilePath, task.CreatedAt, task.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert download task: %w", err)
	}

	return nil
}

func (s *DownloadService) prefetchMetadataForTask(task *DownloadTask) <-chan downloadMetadataResult {
	metaSource := strings.TrimSpace(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)
	if metaSource == "" && metaID == "" {
		return nil
	}

	results := make(chan downloadMetadataResult, 1)
	go func() {
		metadata, err := s.fetchMetadataForTask(task)
		if err != nil {
			applog.LogWarningf(s.ctx, "fetch metadata failed for download task %s (source=%s id=%s): %v", task.ID, metaSource, metaID, err)
		}
		results <- downloadMetadataResult{metadata: metadata, err: err}
		close(results)
	}()
	return results
}

func (s *DownloadService) importWithPrefetchedMetadata(
	task *DownloadTask,
	results <-chan downloadMetadataResult,
	importGame func(*vo.GameMetadataFromWebVO) error,
) error {
	if results == nil {
		return importGame(nil)
	}

	select {
	case result := <-results:
		if result.err == nil {
			return importGame(result.metadata)
		}
		if err := importGame(nil); err != nil {
			return err
		}
		s.emitMetadataFailed(task, result.err)
		return nil
	default:
		if err := importGame(nil); err != nil {
			return err
		}
		go s.applyPrefetchedMetadata(task, results, importGame)
		return nil
	}
}

func (s *DownloadService) applyPrefetchedMetadata(
	task *DownloadTask,
	results <-chan downloadMetadataResult,
	importGame func(*vo.GameMetadataFromWebVO) error,
) {
	result := <-results
	if result.err != nil {
		s.emitMetadataFailed(task, result.err)
		return
	}
	if err := importGame(result.metadata); err != nil {
		applog.LogWarningf(s.ctx, "apply prefetched metadata failed for download task %s: %v", task.ID, err)
		s.emitMetadataFailed(task, fmt.Errorf("apply fetched metadata: %w", err))
	}
}

func (s *DownloadService) fetchMetadataForTask(task *DownloadTask) (*vo.GameMetadataFromWebVO, error) {
	if s.gameService == nil {
		return nil, fmt.Errorf("game service not initialized")
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)
	if !sourceOk {
		return nil, fmt.Errorf("unsupported metadata source: %s", strings.TrimSpace(task.Request.MetaSource))
	}
	if metaID == "" {
		return nil, fmt.Errorf("metadata id is empty")
	}

	metaResult, err := s.gameService.FetchMetadataFromWeb(vo.MetadataRequest{Source: metaSource, ID: metaID})
	if err != nil {
		return nil, err
	}

	applog.LogInfof(s.ctx, "fetch metadata success for download task %s: %s", task.ID, metaResult.Game.Name)
	if s.ctx != nil && s.emitEvent != nil {
		s.emitEvent("download:metadata-prefetched", map[string]interface{}{
			"task_id":     task.ID,
			"meta_source": string(metaSource),
			"meta_id":     metaID,
			"game":        metaResult.Game,
		})
	}
	return &metaResult, nil
}

func (s *DownloadService) handleDownloadedFile(downloadedPath string, extractDir string, fileName string, archiveFormat string, title string, taskID string, stripTopLevel bool) (string, bool, error) {
	format := downloadutils.NormalizeArchiveFormat(archiveFormat)
	if format == "none" {
		return downloadedPath, false, nil
	}
	if !downloadutils.IsSupportedArchiveFormat(format) {
		return "", false, fmt.Errorf("unsupported archive_format: %s", archiveFormat)
	}

	if strings.TrimSpace(extractDir) == "" {
		extractDir = downloadutils.BuildExpectedExtractDir(downloadedPath, fileName, format, title)
	}
	if strings.TrimSpace(extractDir) == "" {
		return "", false, fmt.Errorf("resolve extract dir")
	}

	extractStagingDir := downloadTaskExtractStagingPath(extractDir, taskID)
	if err := os.RemoveAll(extractStagingDir); err != nil {
		return "", false, fmt.Errorf("reset extract staging dir: %w", err)
	}
	if err := os.MkdirAll(extractStagingDir, 0755); err != nil {
		return "", false, fmt.Errorf("create extract dir: %w", err)
	}

	extracted, extractErr := archiveutils.ExtractArchive(downloadedPath, extractStagingDir)
	if extractErr != nil {
		if !extracted {
			applog.LogErrorf(s.ctx, "extract archive failed, fallback to manual extract mode: %v", extractErr)
			finalExtractDir, finalizeErr := finalizeDownloadExtractDir(extractStagingDir, extractDir)
			if finalizeErr != nil {
				return "", false, fmt.Errorf("prepare manual extract dir: %w", finalizeErr)
			}
			applog.LogWarningf(s.ctx, "archive kept at %s, created empty dir %s for manual extraction", downloadedPath, finalExtractDir)
			return finalExtractDir, true, nil
		}
		return "", false, fmt.Errorf("extract archive: %w", extractErr)
	}

	finalExtractDir, err := finalizeDownloadExtractDir(extractStagingDir, extractDir)
	if err != nil {
		return "", false, fmt.Errorf("finalize extract dir: %w", err)
	}

	if err := os.Remove(downloadedPath); err != nil {
		applog.LogWarningf(s.ctx, "failed to delete source archive after unzip: %v", err)
	}

	if stripTopLevel {
		collapsed, ok := collapseSingleRootDirectory(finalExtractDir)
		if !ok {
			return finalExtractDir, false, nil
		}
		finalExtractDir = collapsed
	}

	return finalExtractDir, false, nil
}

func (s *DownloadService) autoCreateOrUpdateGame(task *DownloadTask, gamePath string, metadata *vo.GameMetadataFromWebVO) error {
	if s.gameService == nil {
		return fmt.Errorf("game service not initialized")
	}

	importPath := gamePath
	gameDirectory := gamehelper.DefaultGameDirectory(gamePath)
	if resolvedPath, ok, err := resolveExecutablePathFromRequest(gamePath, task.Request.StartupPath); err != nil {
		return fmt.Errorf("resolve startup_path: %w", err)
	} else if ok {
		importPath = resolvedPath
	}
	return s.importDownloadedGame(task, importPath, gameDirectory, metadata)
}

func (s *DownloadService) importDownloadedGame(task *DownloadTask, importPath string, gameDirectory string, metadata *vo.GameMetadataFromWebVO) error {
	if s.gameService == nil {
		return fmt.Errorf("game service not initialized")
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)

	if sourceOk && metaID != "" {
		if existingID, ok := s.gameService.findGameIDBySource(metaSource, metaID); ok {
			if err := s.updateExistingGame(existingID, importPath, gameDirectory, metaSource, metaID, metadata); err != nil {
				return fmt.Errorf("update existing game by source: %w", err)
			}
			s.emitGameImported(task.ID)
			applog.LogInfof(s.ctx, "import task %s as game: updated existing game by source", task.ID)
			return nil
		}
	}

	if existingID, ok := s.gameService.findGameIDByPath(importPath); ok {
		if err := s.updateExistingGame(existingID, importPath, gameDirectory, metaSource, metaID, metadata); err != nil {
			return fmt.Errorf("update existing game by path: %w", err)
		}
		s.emitGameImported(task.ID)
		applog.LogInfof(s.ctx, "import task %s as game: updated existing game by path", task.ID)
		return nil
	}

	game := models.Game{
		Name:          strings.TrimSpace(task.Request.Title),
		Path:          importPath,
		GameDirectory: gameDirectory,
		SourceType:    enums2.Local,
		SourceID:      "",
	}

	if sourceOk {
		game.SourceType = metaSource
		game.SourceID = metaID
	}

	if metadata != nil {
		mergeMetadataIntoGame(&game, metadata.Game)
		game.MetadataSources = append([]models.GameMetadataSource(nil), metadata.Game.MetadataSources...)
		game.Path = importPath
	}

	if sourceOk && game.SourceType == enums2.Local {
		game.SourceType = metaSource
	}
	if game.SourceID == "" {
		game.SourceID = metaID
	}

	if strings.TrimSpace(game.Name) == "" {
		game.Name = strings.TrimSuffix(filepath.Base(importPath), filepath.Ext(importPath))
	}
	if strings.TrimSpace(game.Name) == "" {
		game.Name = "未知标题"
	}

	var tags []metadatautils.TagItem
	if metadata != nil {
		tags = metadata.Tags
	}
	if err := s.gameService.addGameWithTags(game, tags, false); err != nil {
		return fmt.Errorf("add game: %w", err)
	}

	s.emitGameImported(task.ID)
	applog.LogInfof(s.ctx, "import task %s as game success: %s", task.ID, game.Name)
	return nil
}

func (s *DownloadService) updateExistingGame(gameID string, gamePath string, gameDirectory string, metaSource enums2.SourceType, metaID string, metadata *vo.GameMetadataFromWebVO) error {
	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		applog.LogWarningf(s.ctx, "failed to load existing game %s for path update: %v", gameID, err)
		return err
	}

	changed := false
	if game.Path != gamePath {
		game.Path = gamePath
		changed = true
	}
	if gameDirectory != "" && game.GameDirectory != gameDirectory {
		game.GameDirectory = gameDirectory
		changed = true
	}

	if metadata != nil {
		if mergeMetadataIntoGame(&game, metadata.Game) {
			changed = true
		}
	}

	if metaSource != enums2.Local && game.SourceType != metaSource {
		game.SourceType = metaSource
		changed = true
	}
	if metaID != "" && game.SourceID != metaID {
		game.SourceID = metaID
		changed = true
	}

	if changed {
		if err := s.gameService.UpdateGame(game); err != nil {
			applog.LogWarningf(s.ctx, "failed to update existing game %s: %v", gameID, err)
			return err
		}
	}
	if metaSource != enums2.Local && strings.TrimSpace(metaID) != "" {
		if err := s.gameService.UpsertGameMetadataSource(gameID, metaSource, metaID); err != nil {
			return err
		}
	}
	if metadata != nil {
		for _, source := range metadata.Game.MetadataSources {
			if err := s.gameService.UpsertGameMetadataSource(gameID, source.SourceType, source.SourceID); err != nil {
				return err
			}
		}
		if metadata.Game.SourceType != "" && metadata.Game.SourceType != enums2.Local && strings.TrimSpace(metadata.Game.SourceID) != "" {
			if err := s.gameService.UpsertGameMetadataSource(gameID, metadata.Game.SourceType, metadata.Game.SourceID); err != nil {
				return err
			}
		}
	}

	if metadata != nil && s.gameService.tagService != nil && len(metadata.Tags) > 0 {
		if err := s.gameService.tagService.upsertScrapedTags(gameID, metadata.Tags); err != nil {
			applog.LogWarningf(s.ctx, "failed to update scraped tags for existing game %s: %v", gameID, err)
		}
	}
	return nil
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
	coverSourceURL := strings.TrimSpace(metadata.CoverSourceURL)
	if coverSourceURL == "" && gamehelper.IsDownloadableCoverURL(metadata.CoverURL) {
		coverSourceURL = strings.TrimSpace(metadata.CoverURL)
	}
	if coverSourceURL != "" && target.CoverSourceURL != coverSourceURL {
		target.CoverSourceURL = coverSourceURL
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
	if (metadata.SourceType == enums2.Bangumi || metadata.SourceType == enums2.VNDB || metadata.SourceType == enums2.Hikarinagi) && target.IsNSFW != metadata.IsNSFW {
		target.IsNSFW = metadata.IsNSFW
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

func parseMetaSource(metaSource string) (enums2.SourceType, bool) {
	switch strings.ToLower(strings.TrimSpace(metaSource)) {
	case string(enums2.Bangumi):
		return enums2.Bangumi, true
	case string(enums2.VNDB):
		return enums2.VNDB, true
	case string(enums2.Ymgal):
		return enums2.Ymgal, true
	case string(enums2.Steam):
		return enums2.Steam, true
	case string(enums2.DLsite):
		return enums2.DLsite, true
	case string(enums2.TouchGal):
		return enums2.TouchGal, true
	case string(enums2.Hikarinagi):
		return enums2.Hikarinagi, true
	case string(enums2.ErogameScape):
		return enums2.ErogameScape, true
	default:
		return enums2.Local, false
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

// downloadTaskPartialCleanupPaths 清理下载过程中的中间产物：临时下载文件、
// 分段临时目录、当前任务专属的解压 staging 目录。不包含最终路径 destPath，
// 也不包含按文件名推导的正式解压目录；它们可能是用户已有文件或已导入游戏。
func downloadTaskPartialCleanupPaths(destPath string, extractStagingPath string, extra ...string) []string {
	paths := make([]string, 0, len(extra)+3)
	if strings.TrimSpace(destPath) != "" {
		paths = append(paths,
			downloadutils.TempDownloadPath(destPath),
			downloadutils.MultipartTempDir(destPath),
		)
	}
	if strings.TrimSpace(extractStagingPath) != "" {
		paths = append(paths, extractStagingPath)
	}
	return append(paths, extra...)
}

// downloadTaskCleanupPaths 在中间产物之外额外包含最终下载路径 destPath。
// 仅用于本次任务已完成下载并重命名到最终路径之后的清理（解压阶段的取消/失败），
// 此时 destPath 上的文件确定是本任务的产物。
func downloadTaskCleanupPaths(destPath string, extractStagingPath string, extra ...string) []string {
	paths := downloadTaskPartialCleanupPaths(destPath, extractStagingPath, extra...)
	if strings.TrimSpace(destPath) != "" {
		paths = append(paths, destPath)
	}
	return paths
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

func (s *DownloadService) prepareDownloadExecution(task *DownloadTask) (string, string, *downloadutils.Downloader, error) {
	destPath, err := s.getTaskDestPath(task.Request)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve download path: %w", err)
	}
	extractPath, err := s.getTaskExtractPath(task.Request, destPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve extract path: %w", err)
	}

	resumeOffset := s.inspectResumeOffset(task, destPath)
	config := downloadutils.TransferConfig{ProxyConfig: s.config}
	downloader, proxyDesc, err := downloadutils.NewDownloader(config)
	if err != nil {
		return "", "", nil, fmt.Errorf("create download client: %w", err)
	}

	applog.LogInfof(s.ctx, "Download proxy for task %s: %s", task.ID, proxyDesc)
	s.markTaskDownloading(task, resumeOffset)

	return destPath, extractPath, downloader, nil
}

func (s *DownloadService) inspectResumeOffset(task *DownloadTask, destPath string) int64 {
	return downloadutils.InspectResumeOffset(destPath, task.Request.Size)
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
	// 注意：不要在这里重置 pauseReq/cancelReq。
	// 用户可能在任务刚入队、下载尚未真正开始的窗口内点了暂停/取消，
	// 这里重置会抹掉用户意图（暂停被误判成取消，导致误删已下载数据）。
	// 这两个标志由 StartDownload/requeueTaskLocked/markTaskPaused/cancelTaskAndCleanup 管理。
	task.Error = ""
	task.FilePath = ""
	s.mu.Unlock()

	s.emitProgress(task)
}

func (s *DownloadService) handleGrabDownloadInterruption(task *DownloadTask, err error, cleanupPaths ...string) bool {
	// 优先按用户意图标志判断，而不是按错误类型判断：
	// context 取消引发的连接中断在底层可能表现为各种非 context.Canceled 的
	// 读写错误（如 "use of closed network connection"），只看错误类型会把
	// 用户的暂停/取消误判成普通下载失败，导致文件不清理或状态错误。
	if s.isTaskPauseRequested(task) {
		s.markTaskPaused(task)
		applog.LogInfof(s.ctx, "Download paused: %s", task.ID)
		return true
	}

	if s.isTaskCancelled(task) {
		s.cancelTaskAndCleanup(task, cleanupPaths...)
		applog.LogInfof(s.ctx, "Download cancelled: %s", task.ID)
		return true
	}

	if errors.Is(err, context.Canceled) {
		// 没有用户的暂停/取消请求但 context 被取消（例如应用退出）：
		// 保留已下载数据，标记为可重试错误，重启后可以断点续传
		s.failTask(task, "download interrupted")
		return true
	}

	return false
}

func (s *DownloadService) postProcessDownloadedTask(task *DownloadTask, destPath string, extractPath string) (string, bool, bool, error) {
	s.markTaskExtracting(task)
	extractStagingPath := downloadTaskExtractStagingPath(extractPath, task.ID)

	if s.isTaskCancelled(task) {
		s.cancelTaskAndCleanup(task, downloadTaskCleanupPaths(destPath, extractStagingPath)...)
		return "", false, true, nil
	}

	finalPath, manualExtractRequired, err := s.handleDownloadedFile(destPath, extractPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title, task.ID, task.Request.StripTopLevel)
	if err != nil {
		if s.isTaskCancelled(task) {
			s.cancelTaskAndCleanup(task, downloadTaskCleanupPaths(destPath, extractStagingPath)...)
			return "", false, true, nil
		}
		return "", false, false, fmt.Errorf("post process download file: %w", err)
	}

	finalPath, err = normalizeGamePath(finalPath)
	if err != nil {
		if s.isTaskCancelled(task) {
			s.cancelTaskAndCleanup(task, downloadTaskCleanupPaths(destPath, extractStagingPath, finalPath)...)
			return "", false, true, nil
		}
		return "", false, false, fmt.Errorf("normalize game path: %w", err)
	}

	if s.isTaskCancelled(task) {
		// finalPath 由本任务的专属 staging 目录刚刚落位，归属明确，可以安全清理。
		s.cancelTaskAndCleanup(task, downloadTaskCleanupPaths(destPath, extractStagingPath, finalPath)...)
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

func (s *DownloadService) updateTaskProgress(task *DownloadTask, downloaded int64, total int64) {
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

func (s *DownloadService) requeueTaskLocked(task *DownloadTask) context.Context {
	ctx, cancel := context.WithCancel(s.ctx)
	task.cancel = cancel
	task.Status = DownloadStatusPending
	task.Error = ""
	task.FilePath = ""
	task.pauseReq = false
	task.cancelReq = false
	task.downloadCompleted = false
	return ctx
}

func normalizeCoverImageDownloadItems(items []CoverImageDownloadItem) []CoverImageDownloadItem {
	normalized := make([]CoverImageDownloadItem, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		gameID := strings.TrimSpace(item.GameID)
		coverURL := strings.TrimSpace(item.CoverURL)
		if gameID == "" || !strings.HasPrefix(coverURL, "http") || strings.Contains(coverURL, "wails.localhost") {
			continue
		}
		key := gameID + "\x00" + coverURL
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, CoverImageDownloadItem{
			GameID:   gameID,
			GameName: strings.TrimSpace(item.GameName),
			CoverURL: coverURL,
		})
	}
	return normalized
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

	nestedRoot := filepath.Join(dir, only.Name())
	if gamehelper.IsMacAppBundlePath(nestedRoot) {
		return nestedRoot, true
	}

	nestedEntries, err := os.ReadDir(nestedRoot)
	if err != nil {
		return "", false
	}
	for _, nestedEntry := range nestedEntries {
		src := filepath.Join(nestedRoot, nestedEntry.Name())
		dst := filepath.Join(dir, nestedEntry.Name())
		if err := os.Rename(src, dst); err != nil {
			return "", false
		}
	}
	if err := os.Remove(nestedRoot); err != nil {
		return "", false
	}
	return dir, true
}

func downloadTaskExtractStagingPath(extractPath string, taskID string) string {
	if strings.TrimSpace(extractPath) == "" {
		return ""
	}
	safeTaskID := downloadutils.SanitizeFileName(strings.TrimSpace(taskID))
	if safeTaskID == "" {
		safeTaskID = "unknown"
	}
	return extractPath + ".lunabox.extracting." + safeTaskID
}

// finalizeDownloadExtractDir 将任务专属的解压 staging 目录落位到正式目录。
// 如果同名目录已经存在，则保留原目录并选择新名字，避免覆盖或混入已有游戏。
func finalizeDownloadExtractDir(stagingPath string, preferredPath string) (string, error) {
	for suffix := 1; ; suffix++ {
		candidate := preferredPath
		if suffix > 1 {
			candidate = fmt.Sprintf("%s (%d)", preferredPath, suffix)
		}

		if _, err := os.Stat(candidate); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect extract destination %s: %w", candidate, err)
		}

		if err := os.Rename(stagingPath, candidate); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", fmt.Errorf("move extract staging dir to %s: %w", candidate, err)
		}
		return candidate, nil
	}
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

func (s *DownloadService) getTaskExtractPath(req vo.InstallRequest, destPath string) (string, error) {
	baseDir := filepath.Dir(destPath)
	if customPath, ok, err := resolveInstallSubdir(baseDir, req.InstallSubdir); err != nil {
		return "", err
	} else if ok {
		return customPath, nil
	}
	return downloadutils.BuildExpectedExtractDir(destPath, req.FileName, req.ArchiveFormat, req.Title), nil
}

func resolveInstallSubdir(baseDir string, installSubdir string) (string, bool, error) {
	trimmed := strings.TrimSpace(installSubdir)
	if trimmed == "" {
		return "", false, nil
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return "", false, fmt.Errorf("must not contain NUL bytes")
	}

	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	if strings.HasPrefix(normalized, "/") {
		return "", false, fmt.Errorf("must be relative path")
	}

	segments := strings.Split(normalized, "/")
	safeSegments := make([]string, 0, len(segments))
	for _, segment := range segments {
		trimmedSegment := strings.TrimSpace(segment)
		if trimmedSegment == "" || trimmedSegment == "." {
			continue
		}
		if trimmedSegment == ".." {
			return "", false, fmt.Errorf("must not escape game library")
		}
		safeSegment := strings.TrimSpace(downloadutils.SanitizeFileName(trimmedSegment))
		if safeSegment == "" || safeSegment == "." || safeSegment == ".." {
			return "", false, fmt.Errorf("contains invalid path segment")
		}
		safeSegments = append(safeSegments, safeSegment)
	}
	if len(safeSegments) == 0 {
		return "", false, fmt.Errorf("must not be empty")
	}

	cleanRelative := pathpkg.Clean(strings.Join(safeSegments, "/"))
	if cleanRelative == "." || strings.HasPrefix(cleanRelative, "../") || cleanRelative == ".." {
		return "", false, fmt.Errorf("must not escape game library")
	}

	cleanBase := strings.TrimSpace(baseDir)
	if cleanBase == "" {
		cleanBase = "."
	}
	baseAbs, err := filepath.Abs(cleanBase)
	if err != nil {
		return "", false, fmt.Errorf("normalize game library path: %w", err)
	}

	candidateAbs, err := filepath.Abs(filepath.Join(baseAbs, filepath.FromSlash(cleanRelative)))
	if err != nil {
		return "", false, fmt.Errorf("normalize install_subdir: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return "", false, fmt.Errorf("validate install_subdir: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false, fmt.Errorf("must not escape game library")
	}

	return candidateAbs, true, nil
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
	if _, _, err := resolveInstallSubdir(".", req.InstallSubdir); err != nil {
		return fmt.Errorf("invalid install_subdir: %w", err)
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
