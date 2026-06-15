//go:build darwin

package focusing

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WindowFocusInfo 窗口焦点信息
type WindowFocusInfo struct {
	HWnd      uintptr
	ProcessID uint32
	IsFocused bool
}

// FocusTracker 窗口焦点追踪器
type FocusTracker struct {
	mu           sync.Mutex
	targetPID    uint32
	isFocused    bool
	callbackChan chan WindowFocusInfo
	running      bool
	stopChan     chan struct{}
}

func NewFocusTracker(targetPID uint32) *FocusTracker {
	return &FocusTracker{
		targetPID:    targetPID,
		callbackChan: make(chan WindowFocusInfo, 10),
		stopChan:     make(chan struct{}),
	}
}

func (ft *FocusTracker) Start() (<-chan WindowFocusInfo, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if ft.running {
		return ft.callbackChan, nil
	}

	ft.running = true
	ft.isFocused = ft.isCurrentlyFocused()
	go ft.checkLoop()

	return ft.callbackChan, nil
}

func (ft *FocusTracker) checkLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentlyFocused := ft.isCurrentlyFocused()

			ft.mu.Lock()
			wasFocused := ft.isFocused
			ft.isFocused = currentlyFocused
			ft.mu.Unlock()

			if currentlyFocused != wasFocused {
				info := WindowFocusInfo{
					ProcessID: ft.targetPID,
					IsFocused: currentlyFocused,
				}
				select {
				case ft.callbackChan <- info:
				default:
				}
			}
		case <-ft.stopChan:
			return
		}
	}
}

func (ft *FocusTracker) Stop() {
	ft.mu.Lock()
	if !ft.running {
		ft.mu.Unlock()
		return
	}
	ft.running = false
	stopChan := ft.stopChan
	callbackChan := ft.callbackChan
	ft.mu.Unlock()

	select {
	case <-stopChan:
	default:
		close(stopChan)
	}

	select {
	case <-callbackChan:
	default:
		close(callbackChan)
	}
}

func (ft *FocusTracker) IsFocused() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.isFocused
}

func (ft *FocusTracker) isCurrentlyFocused() bool {
	processID, ok := GetForegroundProcessID()
	return ok && processID == ft.targetPID
}

// GetForegroundProcessID 返回当前 macOS 前台应用的进程 ID。
func GetForegroundProcessID() (uint32, bool) {
	const script = `tell application "System Events" to get unix id of first application process whose frontmost is true`

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return 0, false
	}

	pid64, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 32)
	if err != nil || pid64 == 0 {
		return 0, false
	}
	return uint32(pid64), true
}

func GetForegroundBundlePath() (string, bool) {
	const script = `tell application "System Events" to get bundle url of first application process whose frontmost is true`

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", false
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", false
	}
	if strings.HasPrefix(raw, "file://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			raw = parsed.Path
		}
	}
	raw = strings.TrimSuffix(raw, "/")
	return raw, raw != ""
}

func IsBundlePathFocused(bundlePath string) bool {
	foregroundBundlePath, ok := GetForegroundBundlePath()
	return ok && sameBundlePath(foregroundBundlePath, bundlePath)
}

func IsProcessFocused(processID uint32) bool {
	foregroundPID, ok := GetForegroundProcessID()
	return ok && foregroundPID == processID
}

func sameBundlePath(a string, b string) bool {
	a = normalizeBundlePath(a)
	b = normalizeBundlePath(b)
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func normalizeBundlePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(strings.TrimSuffix(path, string(filepath.Separator)))
}
