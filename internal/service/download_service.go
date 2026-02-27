package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"lunabox/internal/vo"
	"net/http"
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
	DownloadStatusDone        DownloadStatus = "done"
	DownloadStatusError       DownloadStatus = "error"
	DownloadStatusCancelled   DownloadStatus = "cancelled"
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
		ID:      taskID,
		Request: req,
		Status:  DownloadStatusPending,
		Total:   req.Size,
		cancel:  cancel,
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
	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	task.cancel()
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

	// 创建 HTTP 请求（支持取消）
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.Request.URL, nil)
	if err != nil {
		s.failTask(task, fmt.Sprintf("build request: %v", err))
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 LunaBox/1.0")

	client := &http.Client{Timeout: 0} // 大文件不设超时，靠 context cancel
	resp, err := client.Do(req)
	if err != nil {
		s.failTask(task, fmt.Sprintf("http request: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.failTask(task, fmt.Sprintf("server returned %d", resp.StatusCode))
		return
	}
	if task.Request.Size > 0 && resp.ContentLength > 0 && task.Request.Size != resp.ContentLength {
		s.failTask(task, fmt.Sprintf("size mismatch before download: expected=%d got=%d", task.Request.Size, resp.ContentLength))
		return
	}

	// 如果请求中没有 size，用 Content-Length 补充
	s.mu.Lock()
	if task.Total == 0 && resp.ContentLength > 0 {
		task.Total = resp.ContentLength
	}
	task.Status = DownloadStatusDownloading
	s.mu.Unlock()
	s.emitProgress(task)

	// 创建目标文件
	f, err := os.Create(destPath)
	if err != nil {
		s.failTask(task, fmt.Sprintf("create file: %v", err))
		return
	}
	defer f.Close()

	checksumState, err := newChecksumState(task.Request.ChecksumAlgo)
	if err != nil {
		s.failTask(task, fmt.Sprintf("invalid checksum algo: %v", err))
		return
	}

	// 流式写入 + 进度上报（每 500ms 或每 5MB 上报一次）
	buf := make([]byte, 32*1024)
	lastEmit := time.Now()
	lastEmitBytes := int64(0)

	for {
		select {
		case <-ctx.Done():
			f.Close()
			os.Remove(destPath)
			s.mu.Lock()
			task.Status = DownloadStatusCancelled
			s.mu.Unlock()
			s.emitProgress(task)
			applog.LogInfof(s.ctx, "Download cancelled: %s", task.ID)
			return
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				s.failTask(task, fmt.Sprintf("write file: %v", writeErr))
				return
			}
			if checksumState != nil {
				if _, hashErr := checksumState.Write(buf[:n]); hashErr != nil {
					s.failTask(task, fmt.Sprintf("hash file: %v", hashErr))
					return
				}
			}
			s.mu.Lock()
			task.Downloaded += int64(n)
			if task.Total > 0 {
				task.Progress = float64(task.Downloaded) / float64(task.Total) * 100
			}
			s.mu.Unlock()

			now := time.Now()
			bytesSinceLastEmit := task.Downloaded - lastEmitBytes
			if now.Sub(lastEmit) >= 500*time.Millisecond || bytesSinceLastEmit >= 5*1024*1024 {
				s.emitProgress(task)
				lastEmit = now
				lastEmitBytes = task.Downloaded
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			s.failTask(task, fmt.Sprintf("read response: %v", readErr))
			return
		}
	}

	s.mu.RLock()
	downloadedBytes := task.Downloaded
	s.mu.RUnlock()

	if task.Request.Size > 0 && downloadedBytes != task.Request.Size {
		s.failTask(task, fmt.Sprintf("size mismatch after download: expected=%d got=%d", task.Request.Size, downloadedBytes))
		return
	}

	if err := verifyChecksum(task.Request.ChecksumAlgo, task.Request.Checksum, checksumState); err != nil {
		s.failTask(task, fmt.Sprintf("checksum verify failed: %v", err))
		return
	}

	// 下载完成后处理（压缩包解压/路径归一）
	finalPath, err := s.handleDownloadedFile(destPath, task.Request.FileName, task.Request.ArchiveFormat, task.Request.Title)
	if err != nil {
		s.failTask(task, fmt.Sprintf("post process download file: %v", err))
		return
	}
	finalPath, err = normalizeGamePath(finalPath)
	if err != nil {
		s.failTask(task, fmt.Sprintf("normalize game path: %v", err))
		return
	}

	// 下载完成
	s.mu.Lock()
	task.Status = DownloadStatusDone
	task.Progress = 100
	task.FilePath = finalPath
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
		if taskStatus == DownloadStatusPending || taskStatus == DownloadStatusDownloading {
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

func (s *DownloadService) handleDownloadedFile(downloadedPath string, fileName string, archiveFormat string, title string) (string, error) {
	format := normalizeArchiveFormat(archiveFormat)
	if format == "none" {
		return downloadedPath, nil
	}
	if !isSupportedArchiveFormat(format) {
		return "", fmt.Errorf("unsupported archive_format: %s", archiveFormat)
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
		return "", fmt.Errorf("create extract dir: %w", err)
	}

	if err := utils.ExtractArchive(downloadedPath, extractDir); err != nil {
		return "", fmt.Errorf("extract archive: %w", err)
	}

	if err := os.Remove(downloadedPath); err != nil {
		applog.LogWarningf(s.ctx, "failed to delete source archive after unzip: %v", err)
	}

	return extractDir, nil
}

func (s *DownloadService) autoCreateOrUpdateGame(task *DownloadTask, gamePath string, metadata *models.Game) {
	if s.gameService == nil {
		return
	}

	metaSource, sourceOk := parseMetaSource(task.Request.MetaSource)
	metaID := strings.TrimSpace(task.Request.MetaID)

	if sourceOk && metaID != "" {
		if existingID, ok := s.gameService.findGameIDBySource(metaSource, metaID); ok {
			s.updateExistingGame(existingID, gamePath, metaSource, metaID, metadata)
			return
		}
	}

	if existingID, ok := s.gameService.findGameIDByPath(gamePath); ok {
		s.updateExistingGame(existingID, gamePath, metaSource, metaID, metadata)
		return
	}

	game := models.Game{
		Name:       strings.TrimSpace(task.Request.Title),
		Path:       gamePath,
		SourceType: enums.Local,
		SourceID:   "",
	}

	if sourceOk {
		game.SourceType = metaSource
		game.SourceID = metaID
	}

	if metadata != nil {
		mergeMetadataIntoGame(&game, *metadata)
		game.Path = gamePath
	}

	if sourceOk && game.SourceType == enums.Local {
		game.SourceType = metaSource
	}
	if game.SourceID == "" {
		game.SourceID = metaID
	}

	if strings.TrimSpace(game.Name) == "" {
		game.Name = strings.TrimSuffix(filepath.Base(gamePath), filepath.Ext(gamePath))
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

	if req.Size < 0 {
		return fmt.Errorf("size must be >= 0")
	}
	if req.ExpiresAt > 0 && req.ExpiresAt <= time.Now().Unix() {
		return fmt.Errorf("install request expired")
	}

	algo := strings.ToLower(strings.TrimSpace(req.ChecksumAlgo))
	checksum := strings.ToLower(strings.TrimSpace(req.Checksum))
	if algo != "" || checksum != "" {
		if algo == "" || checksum == "" {
			return fmt.Errorf("checksum_algo and checksum must be provided together")
		}
		switch algo {
		case "sha256", "blake3":
		default:
			return fmt.Errorf("unsupported checksum_algo: %s", req.ChecksumAlgo)
		}
	}

	return nil
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
