// Package appdata 对应原版 components/panel/AppDataSettingsPanel.tsx（设置页“应用数据”分区）：导出日志 ZIP、打开数据目录。
package appdata

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// configAPI 是面板用到的 ConfigService 方法（测试里替换打开资源管理器）。
type configAPI interface {
	ExportLogsZip() (string, error)
	OpenDataDirectory() (string, error)
}

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	exporting bool // isExportingLogs
	opening   bool // isOpeningDataDir

	toasts []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）

	api configAPI // 为空时用 Env.Svc.AppConfig
}

func (s *State) config(p panel.Settings) configAPI {
	if s.api != nil {
		return s.api
	}
	return p.Env.Svc.AppConfig
}

// View 构建面板：space-y-5 的两行，每行 flex items-center justify-between gap-4，左侧 flex-1 space-y-2 的标题与说明，
// 右侧 BetterButton。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil

	ui.Column(c.Key("app-data")).FillWidth().Gap(20).Children(func() {
		label := locales.SettingsAppDataExportLogs()
		if s.exporting {
			label = locales.SettingsAppDataExportingLogs()
		}
		row(c, t, "logs", locales.SettingsAppDataLogsTitle(), locales.SettingsAppDataLogsHint(), func() {
			better.Button(c, t, "export-logs", better.ButtonProps{
				Variant: better.ButtonPrimary, Icon: "folder-zip-outline", Loading: s.exporting, Label: label,
			}).OnClick(func() { s.exportLogs(p) })
		})
		row(c, t, "data-dir", locales.SettingsAppDataDataDirTitle(), locales.SettingsAppDataDataDirHint(), func() {
			better.Button(c, t, "open-data-dir", better.ButtonProps{
				Variant: better.ButtonSecondary, Icon: "folder-open-outline", Loading: s.opening, Label: locales.SettingsAppDataOpenDataDir(),
			}).OnClick(func() { s.openDataDir(p) })
		})
	})
}

func row(c *ui.Context, t *theme.Theme, key, title, hint string, button func()) {
	ui.Row(c.Key(key)).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			panel.FieldLabel(c, t, title)
			panel.FieldHint(c, t, hint)
		})
		button()
	})
}

// exportLogs 对应 handleExportLogs：ExportLogsZip 先弹保存对话框再打包，在后台执行；取消时返回空路径，不提示。
func (s *State) exportLogs(p panel.Settings) {
	if s.exporting {
		return
	}
	s.exporting = true
	api := s.config(p)
	p.Env.Async(func() func() {
		path, err := api.ExportLogsZip()
		return func() {
			s.exporting = false
			switch {
			case err != nil:
				s.toasts = append(s.toasts, env.Error(locales.SettingsAppDataToastExportLogsFailed(err.Error())))
			case path != "":
				s.toasts = append(s.toasts, env.Success(locales.SettingsAppDataToastExportLogsSuccess(path)))
			}
		}
	})
}

// openDataDir 对应 handleOpenDataDirectory。
func (s *State) openDataDir(p panel.Settings) {
	if s.opening {
		return
	}
	s.opening = true
	api := s.config(p)
	p.Env.Async(func() func() {
		_, err := api.OpenDataDirectory()
		return func() {
			s.opening = false
			if err != nil {
				s.toasts = append(s.toasts, env.Error(locales.SettingsAppDataToastOpenDataDirFailed(err.Error())))
			}
		}
	})
}
