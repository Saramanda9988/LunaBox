//go:build windows

package sessionend

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Options struct {
	Reason            string
	OnQueryEndSession func()
}

type Hook struct {
	options Options
	hwnd    windows.Handle

	done    chan struct{}
	ready   chan error
	stop    sync.Once
	started atomic.Bool
	blocked atomic.Bool
	queried atomic.Bool
}

const (
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wmClose           = 0x0010
	wmDestroy         = 0x0002

	errorClassAlreadyExists syscall.Errno = 1410
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procShutdownBlockReasonCreate  = user32.NewProc("ShutdownBlockReasonCreate")
	procShutdownBlockReasonDestroy = user32.NewProc("ShutdownBlockReasonDestroy")
	procGetModuleHandleW           = kernel32.NewProc("GetModuleHandleW")

	wndProcCallback = syscall.NewCallback(wndProc)

	hooksMu sync.Mutex
	hooks   = map[windows.Handle]*Hook{}
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type point struct {
	X int32
	Y int32
}

type msg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// Start 在单独线程的隐藏窗口上监听会话结束。WM_QUERYENDSESSION 时返回 FALSE 并设置 ShutdownBlockReason，
// 系统关机界面显示 Reason 并等 LunaBox 自己退出（关闭 DuckDB 需要几秒），同时调用 OnQueryEndSession 开始退出。
// MyGo 只在 WM_ENDSESSION 时同步运行退出监听（mygo.QuitEvent.SessionEnding），不阻止关机，
// 返回后进程随时会被结束，所以这里保留，MyGo 的处理作兜底（见 apps/desktop/app 的 OnBeforeQuit）。
func Start(options Options) (*Hook, error) {
	if options.Reason == "" {
		options.Reason = "LunaBox is saving application data"
	}

	hook := &Hook{
		options: options,
		done:    make(chan struct{}),
		ready:   make(chan error, 1),
	}

	go hook.run()

	if err := <-hook.ready; err != nil {
		return nil, err
	}
	return hook, nil
}

func (h *Hook) Stop() error {
	if h == nil {
		return nil
	}

	h.stop.Do(func() {
		if h.hwnd != 0 {
			procPostMessageW.Call(uintptr(h.hwnd), wmClose, 0, 0)
		}
	})

	select {
	case <-h.done:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out stopping Windows session-end hook")
	}
}

func (h *Hook) ReleaseShutdownBlockReason() {
	if h == nil || h.hwnd == 0 {
		return
	}
	if h.blocked.CompareAndSwap(true, false) {
		procShutdownBlockReasonDestroy.Call(uintptr(h.hwnd))
	}
}

func (h *Hook) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(h.done)

	hwnd, err := createWindow()
	if err != nil {
		h.ready <- err
		return
	}

	h.hwnd = hwnd
	hooksMu.Lock()
	hooks[hwnd] = h
	hooksMu.Unlock()
	h.started.Store(true)
	h.ready <- nil

	var message msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}

	h.ReleaseShutdownBlockReason()
	hooksMu.Lock()
	delete(hooks, hwnd)
	hooksMu.Unlock()
}

func createWindow() (windows.Handle, error) {
	className, err := windows.UTF16PtrFromString("LunaBoxSessionEndWindow")
	if err != nil {
		return 0, err
	}
	windowName, err := windows.UTF16PtrFromString("LunaBox Session End")
	if err != nil {
		return 0, err
	}

	instance, _, err := procGetModuleHandleW.Call(0)
	if instance == 0 {
		return 0, fmt.Errorf("GetModuleHandleW failed: %w", err)
	}

	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   wndProcCallback,
		Instance:  windows.Handle(instance),
		ClassName: className,
	}

	if ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 && err != errorClassAlreadyExists {
		return 0, fmt.Errorf("RegisterClassExW failed: %w", err)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		instance,
		0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW failed: %w", err)
	}

	return windows.Handle(hwnd), nil
}

func wndProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	hook := lookupHook(hwnd)

	switch message {
	case wmQueryEndSession:
		if hook != nil {
			hook.ensureShutdownBlockReason(hwnd)
			if hook.queried.CompareAndSwap(false, true) && hook.options.OnQueryEndSession != nil {
				go hook.options.OnQueryEndSession()
			}
		}
		return 0
	case wmEndSession:
		if hook != nil && wParam == 0 {
			hook.ReleaseShutdownBlockReason()
			hook.queried.Store(false)
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return ret
}

func lookupHook(hwnd windows.Handle) *Hook {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	return hooks[hwnd]
}

func (h *Hook) ensureShutdownBlockReason(hwnd windows.Handle) {
	if h == nil || h.blocked.Load() {
		return
	}
	reason, err := windows.UTF16PtrFromString(h.options.Reason)
	if err != nil {
		return
	}
	ret, _, _ := procShutdownBlockReasonCreate.Call(uintptr(hwnd), uintptr(unsafe.Pointer(reason)))
	if ret != 0 {
		h.blocked.Store(true)
	}
}
