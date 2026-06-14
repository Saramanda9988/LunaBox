//go:build !darwin

package apptray

import (
	goruntime "runtime"

	"github.com/energye/systray"
)

func Start(options Options) {
	setCallbacks(options.Callbacks)

	go func() {
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		systray.Run(func() {
			onSystrayReady(options.Icon)
		}, onSystrayExit)
	}()
}

func Stop() {
	systray.Quit()
}

func onSystrayReady(icon []byte) {
	systray.SetIcon(icon)
	systray.SetTitle("LunaBox")
	systray.SetTooltip("LunaBox")

	systray.SetOnClick(func(menu systray.IMenu) {
		showMainWindow()
	})

	systray.SetOnDClick(func(menu systray.IMenu) {
		showMainWindow()
	})

	mShow := systray.AddMenuItem("显示主窗口", "显示 LunaBox 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 LunaBox")

	mShow.Click(func() {
		showMainWindow()
	})

	mQuit.Click(func() {
		requestQuit()
	})

	notifyReady()
}

func onSystrayExit() {
	notifyExit()
}
