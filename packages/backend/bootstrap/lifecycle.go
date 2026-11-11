package bootstrap

import "lunabox/backend/appconf"

// 发给界面的应用级事件。
const (
	EventMainWindowShown   = "app:main-window-shown"
	EventQuitSyncRequested = "app:quit-sync-requested"
	EventInstallPending    = "install:pending"
)

// Window 是桌面端的主窗口。方法可以在任意 goroutine 里调用。
type Window interface {
	Show()
	Hide()
	Focus()
	UnMinimise()
	IsMinimised() bool
	IsMaximised() bool
	Size() (width, height int)
}

func (a *App) attached() (Window, func()) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.window, a.quit
}

// IsTrayAvailable 报告托盘是否可用（关闭到托盘需要它）。
func (a *App) IsTrayAvailable() bool {
	return a.trayAvailable.Load() && !a.shuttingDown.Load()
}

// SetTrayAvailable 在桌面端创建托盘后调用。
func (a *App) SetTrayAvailable() {
	a.trayAvailable.Store(true)
}

// ShouldForceQuit 报告是否正在退出：主窗口的关闭钩子此时直接放行。
func (a *App) ShouldForceQuit() bool {
	return a.forceQuit.Load() || a.shuttingDown.Load()
}

// ForceQuit 在不经过 QuitApplication 的退出开始时调用（启动错误窗关闭、框架自己开始退出等），
// 之后主窗口的关闭钩子直接放行。
func (a *App) ForceQuit() {
	a.CaptureWindowState()
	a.forceQuit.Store(true)
}

// HasPendingQuitRequest 报告是否已请求退出前同步、正在等界面完成。
func (a *App) HasPendingQuitRequest() bool {
	return a.quitRequestPending.Load()
}

// ShowMainWindow 还原、显示并聚焦主窗口（托盘、第二次启动、协议请求）。
func (a *App) ShowMainWindow() {
	if a.shuttingDown.Load() {
		return
	}
	window, quit := a.attached()
	if quit == nil || window == nil {
		return
	}
	window.UnMinimise()
	window.Show()
	window.Focus()
	a.emit(EventMainWindowShown)
}

// QuitApplication 不再询问，直接退出。
func (a *App) QuitApplication() {
	if a.shuttingDown.Load() {
		return
	}
	_, quit := a.attached()
	if quit == nil {
		return
	}
	a.CaptureWindowState()
	a.forceQuit.Store(true)
	a.shuttingDown.Store(true)
	quit()
}

// RequestQuit 是用户发起的退出（托盘菜单、应用内更新等）：开启了自动备份数据库时先让界面做退出前同步。
func (a *App) RequestQuit(reason string) {
	if shouldRunFrontendQuitSync(a.Config) && a.RequestFrontendQuitSync(reason) {
		return
	}
	a.QuitApplication()
}

// MarkSystemSessionEnding 记下系统正在结束会话（注销、重启、关机），退出时跳过退出前同步与自动备份。
// 框架自己报告会话结束（mygo.QuitEvent.SessionEnding）时调用，退出由框架进行。
func (a *App) MarkSystemSessionEnding() {
	a.systemSessionEnding.Store(true)
}

// QuitForSystemSessionEnd 在系统注销、关机时立即退出，不做退出前同步与退出时的自动备份。
func (a *App) QuitForSystemSessionEnd() {
	a.CaptureWindowState()
	a.MarkSystemSessionEnding()
	a.forceQuit.Store(true)

	_, quit := a.attached()
	if quit == nil || a.shuttingDown.Load() {
		return
	}
	quit()
}

// CaptureWindowState 把主窗口的最大化状态与（未最大化时的）尺寸写进配置，退出时保存。
func (a *App) CaptureWindowState() {
	if a.Config == nil || a.o.KeepWindowState {
		return
	}
	window, _ := a.attached()
	if window == nil {
		return
	}
	a.Config.WindowMaximised = window.IsMaximised()
	if !a.Config.WindowMaximised && !window.IsMinimised() {
		a.Config.WindowWidth, a.Config.WindowHeight = window.Size()
	}
}

// ShouldQuitApplication 处理系统发起的应用退出（例如 macOS 的 Cmd+Q），报告是否继续退出。
func (a *App) ShouldQuitApplication() bool {
	if a.ShouldForceQuit() {
		return true
	}
	if a.HasPendingQuitRequest() {
		return false
	}
	if shouldRunFrontendQuitSync(a.Config) && a.RequestFrontendQuitSync("application-quit") {
		return false
	}

	// 应用级退出必须绕过窗口的关闭钩子，否则可能被当成关闭到托盘。
	a.CaptureWindowState()
	a.forceQuit.Store(true)
	return true
}

// MainWindowClosing 处理主窗口的关闭，报告是否放行：退出中直接关闭；开启“关闭到托盘”时隐藏；
// 需要退出前同步时交给界面；否则退出应用。
func (a *App) MainWindowClosing() bool {
	a.CaptureWindowState()
	if a.ShouldForceQuit() {
		return true
	}
	if a.HasPendingQuitRequest() {
		return false
	}
	if a.Config.CloseToTray && a.IsTrayAvailable() {
		if window, _ := a.attached(); window != nil {
			window.Hide()
		}
		return false
	}
	if shouldRunFrontendQuitSync(a.Config) && a.RequestFrontendQuitSync("window-close") {
		return false
	}
	go a.QuitApplication()
	return false
}

// RequestFrontendQuitSync 显示主窗口并让界面开始退出前同步，报告是否已交给界面（之前已在同步也算）。
// 界面创建并上传数据库备份后调用 ConfigService.SafeQuit 真正退出。
func (a *App) RequestFrontendQuitSync(reason string) bool {
	if a.shuttingDown.Load() {
		return false
	}
	window, quit := a.attached()
	if quit == nil || window == nil {
		return false
	}
	if !a.quitRequestPending.CompareAndSwap(false, true) {
		return true
	}

	a.frontendQuitSyncPlanned.Store(true)
	a.frontendQuitSyncBacked.Store(false)
	window.UnMinimise()
	window.Show()
	a.emit(EventQuitSyncRequested, map[string]string{
		"reason": reason,
	})
	return true
}

// shouldRunFrontendQuitSync 报告退出前是否要让界面先备份数据库（开启了自动备份数据库）。
func shouldRunFrontendQuitSync(config *appconf.AppConfig) bool {
	return config != nil && config.AutoBackupDB
}
