package app

import (
	_ "embed"
	"runtime"

	"github.com/egoist/mygo"

	"lunabox/desktop/ui/locales"
)

// trayIcon 是 Wails 版的 build/windows/tray.png（32x32），Windows 的托盘图标。
//
//go:embed tray.png
var trayIcon []byte

// trayTemplateIcon 是 Wails 版的 build/darwin/tray-template.png：黑色透明底的模板图标，macOS 按菜单栏深浅着色。
//
//go:embed tray-template.png
var trayTemplateIcon []byte

// setupTray 创建通知区域图标，对应 Wails 版的 configureTray：单击显示主窗口，右键菜单是“显示主窗口”“退出”，
// 开发运行时中间多三项“开发：预览启动错误窗”“开发：预览恢复进度窗”“开发：打开初始设置向导”。必须在 WhenReady 里调用。
func (a *app) setupTray() {
	show := &mygo.MenuItem{Label: locales.NativeTrayShowMainWindow(), Click: func(*mygo.MenuItem, *mygo.Window) { a.core.ShowMainWindow() }}
	quit := &mygo.MenuItem{Label: locales.NativeTrayQuit(), Click: func(*mygo.MenuItem, *mygo.Window) { a.core.RequestQuit("tray-menu") }}
	items := []*mygo.MenuItem{show}
	if a.o.Dev {
		// 只给开发者看，同 Wails 版不翻译。
		preview := &mygo.MenuItem{Label: "开发：预览启动错误窗", Click: func(*mygo.MenuItem, *mygo.Window) { a.showStartupErrorPreview() }}
		restore := &mygo.MenuItem{Label: "开发：预览恢复进度窗", Click: func(*mygo.MenuItem, *mygo.Window) { a.showRestorePreview() }}
		setup := &mygo.MenuItem{Label: "开发：打开初始设置向导", Click: func(*mygo.MenuItem, *mygo.Window) { a.showSetupWizard() }}
		items = append(items, mygo.Separator(), preview, restore, setup)
	}
	items = append(items, mygo.Separator(), quit)
	opts := mygo.TrayOptions{
		ID:      "main", // 跨启动保留用户的选择：Windows 上显示在任务栏还是溢出区，macOS 上菜单栏里的位置
		Icon:    trayIcon,
		ToolTip: "LunaBox",
		Menu:    mygo.NewMenu(items),
	}
	switch runtime.GOOS {
	case "darwin":
		opts.Icon, opts.IconIsTemplate = trayTemplateIcon, true
	case "linux":
		// 同 Wails 版用应用图标：面板按自己的尺寸缩放，高分屏上比 32x32 的 tray.png 清楚。
		opts.Icon = a.o.Icon
	}
	tray, err := mygo.NewTray(opts)
	if err != nil {
		a.logger.Error("failed to create tray icon: " + err.Error())
		return
	}
	tray.OnClick(a.core.ShowMainWindow)
	// 设置页切换语言后菜单随之更新（Wails 版的菜单文字是写死的中文）。
	a.relabelTray = func() {
		show.SetLabel(locales.NativeTrayShowMainWindow())
		quit.SetLabel(locales.NativeTrayQuit())
	}
	a.tray = tray
	a.core.SetTrayAvailable()
}
