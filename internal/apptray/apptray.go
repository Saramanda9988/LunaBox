package apptray

import "sync"

type Options struct {
	Icon       []byte
	AppIcon    []byte
	DarwinIcon []byte
	Callbacks  Callbacks
}

type Callbacks struct {
	OnReady                   func()
	OnExit                    func()
	ShowMainWindow            func()
	NativeMainWindowDidShow   func()
	RequestFrontendQuitSync   func(reason string) bool
	QuitApplication           func()
	ShouldRunFrontendQuitSync func() bool
}

var (
	callbacksMu sync.RWMutex
	callbacks   Callbacks
)

func setCallbacks(next Callbacks) {
	callbacksMu.Lock()
	defer callbacksMu.Unlock()
	callbacks = next
}

func currentCallbacks() Callbacks {
	callbacksMu.RLock()
	defer callbacksMu.RUnlock()
	return callbacks
}

func notifyReady() {
	if cb := currentCallbacks().OnReady; cb != nil {
		cb()
	}
}

func notifyExit() {
	if cb := currentCallbacks().OnExit; cb != nil {
		cb()
	}
}

func showMainWindow() {
	if cb := currentCallbacks().ShowMainWindow; cb != nil {
		cb()
	}
}

func notifyNativeMainWindowDidShow() {
	if cb := currentCallbacks().NativeMainWindowDidShow; cb != nil {
		cb()
	}
}

func requestQuit() {
	cb := currentCallbacks()
	if cb.ShouldRunFrontendQuitSync != nil && cb.ShouldRunFrontendQuitSync() {
		if cb.RequestFrontendQuitSync != nil {
			cb.RequestFrontendQuitSync("tray-menu")
		}
		return
	}

	if cb.QuitApplication != nil {
		cb.QuitApplication()
	}
}
