package service

import (
	"context"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/vo"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	ID         string         `json:"id"`
	Status     DownloadStatus `json:"status"`
	Progress   float64        `json:"progress"`
	Downloaded int64          `json:"downloaded"`
	Total      int64          `json:"total"`
	Error      string         `json:"error,omitempty"`
	FilePath   string         `json:"file_path,omitempty"`
}

// DownloadService 管理所有下载任务
type DownloadService struct {
	ctx            context.Context
	config         *appconf.AppConfig
	mu             sync.RWMutex
	tasks          map[string]*DownloadTask
	pendingInstall *vo.InstallRequest // 从 lunabox:// URI 传入的待安装请求，在 GUI 就绪前暂存
}

func NewDownloadService() *DownloadService {
	return &DownloadService{
		tasks: make(map[string]*DownloadTask),
	}
}

func (s *DownloadService) Init(ctx context.Context, config *appconf.AppConfig) {
	s.ctx = ctx
	s.config = config
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

// =================== 内部下载逻辑 ===================

func (s *DownloadService) emitProgress(task *DownloadTask) {
	if s.ctx == nil {
		return
	}
	runtime.EventsEmit(s.ctx, "download:progress", DownloadProgressEvent{
		ID:         task.ID,
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

	// 确定下载目标路径
	destDir, err := s.getDownloadDir()
	if err != nil {
		s.failTask(task, fmt.Sprintf("failed to get download dir: %v", err))
		return
	}
	fileName := sanitizeFileName(task.Request.Title)
	if fileName == "" {
		fileName = task.ID
	}
	// 保留原始扩展名
	if ext := guessExt(task.Request.URL); ext != "" {
		fileName += ext
	}
	destPath := filepath.Join(destDir, fileName)

	// 创建 HTTP 请求（支持取消）
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.Request.URL, nil)
	if err != nil {
		s.failTask(task, fmt.Sprintf("build request: %v", err))
		return
	}
	req.Header.Set("User-Agent", "LunaBox/1.0")

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

	// 下载完成
	s.mu.Lock()
	task.Status = DownloadStatusDone
	task.Progress = 100
	task.FilePath = destPath
	s.mu.Unlock()
	s.emitProgress(task)
	applog.LogInfof(s.ctx, "Download complete: %s  path=%s", task.ID, destPath)
}

func (s *DownloadService) failTask(task *DownloadTask, msg string) {
	applog.LogErrorf(s.ctx, "Download error [%s]: %s", task.ID, msg)
	s.mu.Lock()
	task.Status = DownloadStatusError
	task.Error = msg
	s.mu.Unlock()
	s.emitProgress(task)
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

func guessExt(rawURL string) string {
	// 只取路径部分，忽略 query string
	for i, c := range rawURL {
		if c == '?' {
			rawURL = rawURL[:i]
			break
		}
	}
	ext := filepath.Ext(rawURL)
	switch ext {
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return ext
	}
	return ""
}
