//go:build windows

package focusing

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// WindowFocusInfo 窗口焦点信息
type WindowFocusInfo struct {
	HWnd      uintptr
	ProcessID uint32
	IsFocused bool // 目标进程是否获得焦点
}

const (
	// EVENT_SYSTEM_FOREGROUND 前台窗口变化事件
	EVENT_SYSTEM_FOREGROUND = 3
	// WINEVENT_OUTOFCONTEXT 不需要 DLL 注入
	WINEVENT_OUTOFCONTEXT = 0
)

var (
	// user32 DLL 和函数
	user32                       = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

// FocusTracker 窗口焦点追踪器
// 使用轮询方式检测窗口焦点状态
type FocusTracker struct {
	mu           sync.Mutex
	targetPID    uint32
	isFocused    bool
	callbackChan chan WindowFocusInfo
	running      bool
	stopChan     chan struct{}
}

// NewFocusTracker 创建焦点追踪器
func NewFocusTracker(targetPID uint32) *FocusTracker {
	return &FocusTracker{
		targetPID:    targetPID,
		isFocused:    false,
		callbackChan: make(chan WindowFocusInfo, 10),
		stopChan:     make(chan struct{}),
	}
}

// Start 开始追踪窗口焦点
func (ft *FocusTracker) Start() (<-chan WindowFocusInfo, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if ft.running {
		return ft.callbackChan, nil
	}

	ft.running = true
	ft.isFocused = ft.isCurrentlyFocused()

	// 启动检查 goroutine
	go ft.checkLoop()

	return ft.callbackChan, nil
}

// checkLoop 检查循环
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

			// 焦点状态变化时发送通知
			if currentlyFocused != wasFocused {
				info := WindowFocusInfo{
					HWnd:      0, // 不需要窗口句柄
					ProcessID: ft.targetPID,
					IsFocused: currentlyFocused,
				}
				select {
				case ft.callbackChan <- info:
				default:
					// Channel full, skip
				}
			}

		case <-ft.stopChan:
			return
		}
	}
}

// Stop 停止追踪
func (ft *FocusTracker) Stop() {
	ft.mu.Lock()
	if !ft.running {
		ft.mu.Unlock()
		return
	}
	ft.running = false
	ft.mu.Unlock()

	// 安全关闭 stopChan
	select {
	case <-ft.stopChan:
		// 已经关闭
	default:
		close(ft.stopChan)
	}

	// 关闭 callback channel
	ft.mu.Lock()
	select {
	case <-ft.callbackChan:
		// 已经关闭
	default:
		close(ft.callbackChan)
	}
	ft.mu.Unlock()
}

// IsFocused 返回当前是否聚焦
func (ft *FocusTracker) IsFocused() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.isFocused
}

// isCurrentlyFocused 检查当前是否为前台窗口
func (ft *FocusTracker) isCurrentlyFocused() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}

	var processID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&processID)))

	return processID == ft.targetPID
}

// IsProcessFocused 检查指定进程的窗口是否为前台窗口
func IsProcessFocused(processID uint32) bool {
	foregroundHwnd, _, _ := procGetForegroundWindow.Call()
	if foregroundHwnd == 0 {
		return false
	}

	var foregroundPID uint32
	procGetWindowThreadProcessId.Call(foregroundHwnd, uintptr(unsafe.Pointer(&foregroundPID)))

	return foregroundPID == processID
}
