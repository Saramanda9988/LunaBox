//go:build linux

package focusing

import (
	"context"
	"lunabox/internal/utils/processutils"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var kdeActiveWindowPIDOutput = func() ([]byte, error) {
	if _, err := exec.LookPath("kdotool"); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	return exec.CommandContext(ctx, "kdotool", "getactivewindow", "getwindowpid").Output()
}

// WindowFocusInfo 窗口焦点信息。
type WindowFocusInfo struct {
	HWnd      uintptr
	ProcessID uint32
	IsFocused bool
}

// FocusTracker provides a conservative Linux fallback. Without a reliable
// desktop-agnostic foreground-window API, LunaBox treats the tracked process as
// active while it is alive.
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
	ticker := time.NewTicker(1 * time.Second)
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
	foregroundPID, ok := GetForegroundProcessID()
	if ok {
		return foregroundPID == ft.targetPID
	}
	return processutils.IsProcessPresentByPID(ft.targetPID)
}

func GetForegroundProcessID() (uint32, bool) {
	return getKDEForegroundProcessID()
}

func getKDEForegroundProcessID() (uint32, bool) {
	out, err := kdeActiveWindowPIDOutput()
	if err != nil {
		return 0, false
	}
	return parseKDEForegroundProcessID(string(out))
}

func parseKDEForegroundProcessID(output string) (uint32, bool) {
	for _, field := range strings.Fields(strings.TrimSpace(output)) {
		pid64, err := strconv.ParseUint(field, 10, 32)
		if err == nil && pid64 != 0 {
			return uint32(pid64), true
		}
	}
	return 0, false
}

func GetForegroundBundlePath() (string, bool) {
	return "", false
}

func IsBundlePathFocused(bundlePath string) bool {
	return false
}

func IsProcessFocused(processID uint32) bool {
	foregroundPID, ok := GetForegroundProcessID()
	if ok {
		return foregroundPID == processID
	}
	return processutils.IsProcessPresentByPID(processID)
}
