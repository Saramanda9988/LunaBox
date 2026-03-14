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
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"lunabox/internal/vo"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
		extractPath := buildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
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
	if err := utils.OpenFileOrFolder(task.FilePath); err != nil {
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
		mergeMetadataIntoGame(&game, *metadata)
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

	if err := s.gameService.AddGame(game); err != nil {
		applog.LogErrorf(s.ctx, "import task %s as game failed: %v", task.ID, err)
		return fmt.Errorf("add game: %w", err)
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

	// 确定下载目标路径
	destDir, err := s.getDownloadDir()
	if err != nil {
		s.failTask(task, fmt.Sprintf("failed to get download dir: %v", err))
		return
	}
	fileName := sanitizeDownloadedFileName(task.Request.FileName)
	if fileName == "" {
		s.failTask(task, "invalid file_name")
		return
	}
	destPath := filepath.Join(destDir, fileName)

	resumeOffset := int64(0)
	if fileInfo, statErr := os.Stat(destPath); statErr == nil && !fileInfo.IsDir() {
		resumeOffset = fileInfo.Size()
	}
	if resumeOffset > task.Request.Size {
		applog.LogWarningf(s.ctx, "partial file exceeds declared size for task %s, reset partial file", task.ID)
		_ = os.Remove(destPath)
		resumeOffset = 0
	}

	s.mu.Lock()
	if resumeOffset > 0 {
		task.Downloaded = resumeOffset
	}
	task.Total = task.Request.Size
	task.pauseReq = false
	task.cancelReq = false
	s.mu.Unlock()

	// 创建 HTTP 请求（支持取消）
	client, proxyDesc, err := newSecureDownloadClient(s.config) // 大文件不设超时，靠 context cancel
	if err != nil {
		s.failTask(task, fmt.Sprintf("create download client: %v", err))
		return
	}
	applog.LogInfof(s.ctx, "Download proxy for task %s: %s", task.ID, proxyDesc)
	buildRequest := func(offset int64) (*http.Response, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, task.Request.URL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 LunaBox/1.0")
		return client.Do(req)
	}

	resp, err := buildRequest(resumeOffset)
	if err != nil {
		if errors.Is(err, context.Canceled) && s.isTaskPauseRequested(task) {
			s.markTaskPaused(task)
			return
		}
		if errors.Is(err, context.Canceled) && s.isTaskCancelled(task) {
			extractPath := buildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
			s.cancelTaskAndCleanup(task, destPath, extractPath)
			return
		}
		s.failTask(task, fmt.Sprintf("http request: %v", err))
		return
	}
	if resumeOffset > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		_ = resp.Body.Close()
		applog.LogWarningf(s.ctx, "range request got 416 for task %s, reset partial file and retry full download", task.ID)
		_ = os.Remove(destPath)
		resumeOffset = 0
		s.mu.Lock()
		task.Downloaded = 0
		task.Progress = 0
		s.mu.Unlock()
		resp, err = buildRequest(0)
		if err != nil {
			if errors.Is(err, context.Canceled) && s.isTaskPauseRequested(task) {
				s.markTaskPaused(task)
				return
			}
			if errors.Is(err, context.Canceled) && s.isTaskCancelled(task) {
				extractPath := buildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
				s.cancelTaskAndCleanup(task, destPath, extractPath)
				return
			}
			s.failTask(task, fmt.Sprintf("http request after range reset: %v", err))
			return
		}
	}
	defer resp.Body.Close()

	if resumeOffset > 0 && resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		s.failTask(task, fmt.Sprintf("server returned %d for range request", resp.StatusCode))
		return
	}
	if resumeOffset == 0 && resp.StatusCode != http.StatusOK {
		s.failTask(task, fmt.Sprintf("server returned %d", resp.StatusCode))
		return
	}

	if resumeOffset > 0 && resp.StatusCode == http.StatusOK {
		resumeOffset = 0
		s.mu.Lock()
		task.Downloaded = 0
		task.Progress = 0
		s.mu.Unlock()
	}
	if task.Request.Size > 0 && resp.ContentLength > 0 && task.Request.Size != resp.ContentLength+resumeOffset {
		s.failTask(task, fmt.Sprintf("size mismatch before download: expected=%d got=%d", task.Request.Size, resp.ContentLength))
		return
	}

	// 如果请求中没有 size，用 Content-Length 补充
	s.mu.Lock()
	task.Status = DownloadStatusDownloading
	s.mu.Unlock()
	s.emitProgress(task)

	// 创建/追加目标文件
	fileFlags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if resumeOffset > 0 {
		fileFlags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(destPath, fileFlags, 0644)
	if err != nil {
		s.failTask(task, fmt.Sprintf("create file: %v", err))
		return
	}
	fileClosed := false
	closeFile := func() error {
		if fileClosed {
			return nil
		}
		fileClosed = true
		return f.Close()
	}

	checksumState, err := newChecksumState(task.Request.ChecksumAlgo)
	if err != nil {
		_ = closeFile()
		s.failTask(task, fmt.Sprintf("invalid checksum algo: %v", err))
		return
	}
	if checksumState != nil && resumeOffset > 0 {
		if err := hashExistingFilePortion(destPath, resumeOffset, checksumState); err != nil {
			_ = closeFile()
			s.failTask(task, fmt.Sprintf("prepare checksum state: %v", err))
			return
		}
	}

	// 流式写入 + 进度上报（每 500ms 或每 5MB 上报一次）
	buf := make([]byte, 32*1024)
	lastEmit := time.Now()
	lastEmitBytes := int64(0)

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			pauseRequested := task.pauseReq
			task.pauseReq = false
			s.mu.Unlock()
			_ = closeFile()
			if pauseRequested {
				s.markTaskPaused(task)
				applog.LogInfof(s.ctx, "Download paused: %s", task.ID)
			} else {
				extractPath := buildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
				s.cancelTaskAndCleanup(task, destPath, extractPath)
				applog.LogInfof(s.ctx, "Download cancelled: %s", task.ID)
			}
			return
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			s.mu.Lock()
			remaining := task.Total - task.Downloaded
			s.mu.Unlock()
			if remaining < int64(n) {
				_ = closeFile()
				_ = os.Remove(destPath)
				s.failTask(task, fmt.Sprintf("response exceeded declared size: expected=%d remaining=%d chunk=%d", task.Total, remaining, n))
				return
			}
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				_ = closeFile()
				s.failTask(task, fmt.Sprintf("write file: %v", writeErr))
				return
			}
			if checksumState != nil {
				if _, hashErr := checksumState.Write(buf[:n]); hashErr != nil {
					_ = closeFile()
					s.failTask(task, fmt.Sprintf("hash file: %v", hashErr))
					return
				}
			}
			s.mu.Lock()
			task.Downloaded += int64(n)
			task.Progress = float64(task.Downloaded) / float64(task.Total) * 100
			currentDownloaded := task.Downloaded
			s.mu.Unlock()

			now := time.Now()
			bytesSinceLastEmit := currentDownloaded - lastEmitBytes
			if now.Sub(lastEmit) >= 500*time.Millisecond || bytesSinceLastEmit >= 5*1024*1024 {
				s.emitProgress(task)
				lastEmit = now
				lastEmitBytes = currentDownloaded
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if errors.Is(readErr, context.Canceled) {
				_ = closeFile()
				s.mu.Lock()
				pauseRequested := task.pauseReq
				task.pauseReq = false
				if pauseRequested {
					s.mu.Unlock()
					s.markTaskPaused(task)
					applog.LogInfof(s.ctx, "Download paused (read canceled): %s", task.ID)
				} else {
					s.mu.Unlock()
					extractPath := buildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
					s.cancelTaskAndCleanup(task, destPath, extractPath)
					applog.LogInfof(s.ctx, "Download cancelled (read canceled): %s", task.ID)
				}
				return
			}
			_ = closeFile()
			s.failTask(task, fmt.Sprintf("read response: %v", readErr))
			return
		}
	}

	s.mu.RLock()
	downloadedBytes := task.Downloaded
	s.mu.RUnlock()

	if task.Request.Size > 0 && downloadedBytes != task.Request.Size {
		_ = closeFile()
		s.failTask(task, fmt.Sprintf("size mismatch after download: expected=%d got=%d", task.Request.Size, downloadedBytes))
		return
	}

	if err := verifyChecksum(task.Request.ChecksumAlgo, task.Request.Checksum, checksumState); err != nil {
		_ = closeFile()
		s.failTask(task, fmt.Sprintf("checksum verify failed: %v", err))
		return
	}

	if err := closeFile(); err != nil {
		s.failTask(task, fmt.Sprintf("close file before post process: %v", err))
		return
	}

	extractPath := buildExpectedExtractDir(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
	s.mu.Lock()
	task.Status = DownloadStatusExtracting
	s.mu.Unlock()
	s.emitProgress(task)

	if s.isTaskCancelled(task) {
		s.cancelTaskAndCleanup(task, destPath, extractPath)
		return
	}

	// 下载完成后处理（压缩包解压/路径归一）
	finalPath, manualExtractRequired, err := s.handleDownloadedFile(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
	if err != nil {
		if s.isTaskCancelled(task) {
			s.cancelTaskAndCleanup(task, destPath, extractPath)
			return
		}
		s.failTask(task, fmt.Sprintf("post process download file: %v", err))
		return
	}
	finalPath, err = normalizeGamePath(finalPath)
	if err != nil {
		if s.isTaskCancelled(task) {
			s.cancelTaskAndCleanup(task, destPath, extractPath, finalPath)
			return
		}
		s.failTask(task, fmt.Sprintf("normalize game path: %v", err))
		return
	}
	if s.isTaskCancelled(task) {
		s.cancelTaskAndCleanup(task, destPath, extractPath, finalPath)
		return
	}

	// 下载完成
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
	name := sanitizeDownloadedFileName(req.FileName)
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

func (s *DownloadService) fetchMetadataForTask(task *DownloadTask) *models.Game {
	if s.gameService == nil {
		return nil
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)
	if !sourceOk || metaID == "" {
		return nil
	}

	game, err := s.gameService.FetchMetadata(vo.MetadataRequest{Source: metaSource, ID: metaID})
	if err != nil {
		applog.LogWarningf(s.ctx, "fetch metadata failed for download task %s (source=%s id=%s): %v", task.ID, metaSource, metaID, err)
		return nil
	}

	applog.LogInfof(s.ctx, "fetch metadata success for download task %s: %s", task.ID, game.Name)
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "download:metadata-prefetched", map[string]interface{}{
			"task_id":     task.ID,
			"meta_source": string(metaSource),
			"meta_id":     metaID,
			"game":        game,
		})
	}
	return &game
}

func (s *DownloadService) handleDownloadedFile(downloadedPath string, fileName string, archiveFormat string, title string) (string, bool, error) {
	format := normalizeArchiveFormat(archiveFormat)
	if format == "none" {
		return downloadedPath, false, nil
	}
	if !isSupportedArchiveFormat(format) {
		return "", false, fmt.Errorf("unsupported archive_format: %s", archiveFormat)
	}

	baseName := trimArchiveSuffixByFormat(strings.TrimSpace(fileName), format)
	baseName = sanitizeFileName(baseName)
	if baseName == "" {
		baseName = sanitizeFileName(title)
	}
	if baseName == "" {
		baseName = "game"
	}

	extractDir := filepath.Join(filepath.Dir(downloadedPath), baseName)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", false, fmt.Errorf("create extract dir: %w", err)
	}

	extracted, extractErr := utils.ExtractArchive(downloadedPath, extractDir)
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

func (s *DownloadService) autoCreateOrUpdateGame(task *DownloadTask, gamePath string, metadata *models.Game) {
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
		mergeMetadataIntoGame(&game, *metadata)
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

	if err := s.gameService.AddGame(game); err != nil {
		applog.LogWarningf(s.ctx, "auto import game failed for task %s: %v", task.ID, err)
		return
	}

	applog.LogInfof(s.ctx, "auto import game success for task %s: %s", task.ID, game.Name)
}

func (s *DownloadService) updateExistingGame(gameID string, gamePath string, metaSource enums.SourceType, metaID string, metadata *models.Game) {
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
		if mergeMetadataIntoGame(&game, *metadata) {
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

func buildExpectedExtractDir(downloadedPath string, fileName string, archiveFormat string, title string) string {
	if strings.TrimSpace(downloadedPath) == "" {
		return ""
	}

	format := normalizeArchiveFormat(archiveFormat)
	if format == "none" || !isSupportedArchiveFormat(format) {
		return ""
	}

	baseName := trimArchiveSuffixByFormat(strings.TrimSpace(fileName), format)
	baseName = sanitizeFileName(baseName)
	if baseName == "" {
		baseName = sanitizeFileName(title)
	}
	if baseName == "" {
		baseName = "game"
	}

	return filepath.Join(filepath.Dir(downloadedPath), baseName)
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

func sanitizeFileName(name string) string {
	invalid := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|', '\x00'}
	result := []rune(name)
	for i, c := range result {
		for _, inv := range invalid {
			if c == inv {
				result[i] = '_'
				break
			}
		}
	}
	s := string(result)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func sanitizeDownloadedFileName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	base := filepath.Base(trimmed)
	if base == "." || base == ".." {
		return ""
	}
	safe := strings.TrimSpace(sanitizeFileName(base))
	if safe == "" || safe == "." || safe == ".." {
		return ""
	}
	return safe
}

func normalizeArchiveFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

func validateDownloadURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	if isBlockedHostname(host) {
		return fmt.Errorf("url host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("url host resolves to a blocked address")
	}
	return nil
}

func validateChecksumFields(algo string, checksum string) error {
	trimmedAlgo := strings.ToLower(strings.TrimSpace(algo))
	trimmedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	if trimmedAlgo == "" || trimmedChecksum == "" {
		return fmt.Errorf("checksum_algo and checksum are required")
	}

	if _, err := hex.DecodeString(trimmedChecksum); err != nil {
		return fmt.Errorf("checksum must be lowercase hex")
	}

	switch trimmedAlgo {
	case "sha256", "blake3":
		if len(trimmedChecksum) != 64 {
			return fmt.Errorf("%s checksum must be 64 hex characters", trimmedAlgo)
		}
	default:
		return fmt.Errorf("unsupported checksum_algo: %s", algo)
	}

	return nil
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

	selection, proxyDesc, err := utils.ResolveDownloadProxy(mode, proxyURL)
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
			return validateDownloadURL(req.URL.String())
		},
	}, proxyDesc, nil
}

func isSupportedArchiveFormat(format string) bool {
	switch normalizeArchiveFormat(format) {
	case "none", "zip", "rar", "7z", "tar", "tar.gz", "tar.bz2", "tar.xz", "tar.zst", "tgz", "tbz2", "txz", "tzst":
		return true
	default:
		return false
	}
}

func trimArchiveSuffixByFormat(name string, format string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ""
	}

	lower := strings.ToLower(trimmedName)
	var suffixes []string
	switch format {
	case "zip":
		suffixes = []string{".zip"}
	case "rar":
		suffixes = []string{".rar"}
	case "7z":
		suffixes = []string{".7z"}
	case "tar":
		suffixes = []string{".tar"}
	case "tar.gz", "tgz":
		suffixes = []string{".tar.gz", ".tgz"}
	case "tar.bz2", "tbz2":
		suffixes = []string{".tar.bz2", ".tbz2"}
	case "tar.xz", "txz":
		suffixes = []string{".tar.xz", ".txz"}
	case "tar.zst", "tzst":
		suffixes = []string{".tar.zst", ".tzst"}
	default:
		suffixes = nil
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(trimmedName[:len(trimmedName)-len(suffix)])
		}
	}

	return strings.TrimSuffix(trimmedName, filepath.Ext(trimmedName))
}

func validateInstallRequest(req vo.InstallRequest) error {
	if strings.TrimSpace(req.URL) == "" {
		return fmt.Errorf("missing url")
	}
	if err := validateDownloadURL(req.URL); err != nil {
		return err
	}
	if sanitizeDownloadedFileName(req.FileName) == "" {
		return fmt.Errorf("missing or invalid file_name")
	}

	format := normalizeArchiveFormat(req.ArchiveFormat)
	if format == "" {
		return fmt.Errorf("missing archive_format")
	}
	if !isSupportedArchiveFormat(format) {
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
	if err := validateChecksumFields(algo, checksum); err != nil {
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

type checksumWriter struct {
	h hash.Hash
}

func newChecksumState(algo string) (*checksumWriter, error) {
	trimmedAlgo := strings.ToLower(strings.TrimSpace(algo))
	if trimmedAlgo == "" {
		return nil, nil
	}
	switch trimmedAlgo {
	case "sha256":
		return &checksumWriter{h: sha256.New()}, nil
	case "blake3":
		return &checksumWriter{h: blake3.New()}, nil
	default:
		return nil, fmt.Errorf("unsupported checksum algo: %s", algo)
	}
}

func (w *checksumWriter) Write(p []byte) (int, error) {
	if w == nil || w.h == nil {
		return len(p), nil
	}
	return w.h.Write(p)
}

func hashExistingFilePortion(path string, bytesToHash int64, writer *checksumWriter) error {
	if writer == nil || writer.h == nil || bytesToHash <= 0 {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open existing file: %w", err)
	}
	defer file.Close()

	limited := io.LimitReader(file, bytesToHash)
	if _, err := io.Copy(writer, limited); err != nil {
		return fmt.Errorf("hash existing file: %w", err)
	}

	return nil
}

func verifyChecksum(algo string, expected string, writer *checksumWriter) error {
	trimmedAlgo := strings.ToLower(strings.TrimSpace(algo))
	trimmedExpected := strings.ToLower(strings.TrimSpace(expected))
	if trimmedAlgo == "" && trimmedExpected == "" {
		return nil
	}
	if trimmedAlgo == "" || trimmedExpected == "" {
		return fmt.Errorf("checksum_algo/checksum not paired")
	}
	if writer == nil || writer.h == nil {
		return fmt.Errorf("checksum state missing")
	}
	got := hex.EncodeToString(writer.h.Sum(nil))
	if got != trimmedExpected {
		return fmt.Errorf("%s mismatch: expected=%s got=%s", trimmedAlgo, trimmedExpected, got)
	}
	return nil
}
