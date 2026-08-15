//go:build darwin

package focusing

/*
#cgo darwin LDFLAGS: -framework AppKit
#include <stdint.h>
#include <stdlib.h>

uint32_t lunabox_frontmost_process_id(void);
char *lunabox_frontmost_bundle_path(void);
*/
import "C"

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
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
	pid := uint32(C.lunabox_frontmost_process_id())
	if pid == 0 {
		return 0, false
	}
	return pid, true
}

func GetForegroundBundlePath() (string, bool) {
	rawPath := C.lunabox_frontmost_bundle_path()
	if rawPath == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(rawPath))

	raw := strings.TrimSpace(C.GoString(rawPath))
	if raw == "" {
		return "", false
	}
	return strings.TrimSuffix(raw, "/"), true
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
