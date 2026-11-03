// Package basic 对应原版 components/panel/BasicSettingsPanel.tsx（设置页的“基础设置”分区）：账号授权卡片、
// VNDB Access Token、主题、语言、界面缩放、时区、游戏库目录（含 GameLibraryPathChangeModal）、关闭到托盘与开机自启。
package basic

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/account"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// vndbTokenLabel 是 VNDB Access Token 一项的 label（原版写死的英文）。
const vndbTokenLabel = "VNDB Access Token" // i18n:keep：原版写死的英文专有名词

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	// Accounts 是三张账号卡片的状态。
	Accounts account.State

	// 账号卡片的展开（expandedAccount、isAccountContentVisible 与 handleAccountExpand 的计时器）。
	expanded      account.Provider // 为空时三张卡片等宽
	contentHidden bool             // !isAccountContentVisible
	phase         accountPhase
	next          account.Provider // 淡出结束后展开的卡片
	phaseAt       time.Time        // 当前阶段开始的帧时间；为零时下一次构建取 c.Now()
	gridAnim      bool             // hasGridAnimation：点击时窗口宽度 ≥ 640（网格列宽有过渡）

	// 游戏库目录（pendingGameLibraryInput、libraryChangePreview、isLibraryPreviewLoading、isLibraryChangeApplying）。
	libInput       string
	libPending     bool // pendingGameLibraryInput 不为 null
	libPreview     *service.GameLibraryPathChangePreview
	libChanges     []gamePathChange     // groupChangesByGame(preview.changes)
	libScroll      components.ScrollBar // 弹窗内容（max-h-[58dvh] overflow-y-auto）
	libOuter       components.ScrollBar // 弹窗比窗口还高时整个遮罩滚动（overflow-y-auto）
	previewLoading bool
	applying       bool
	libGen         int // 最近一次预览请求的编号，丢弃过期结果

	toasts []env.Notice // 后台结果里待显示的提示（回调拿不到 Context），下一次构建时显示
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// View 构建面板。原版的根是片段，各项直接放进分区内容的 space-y-4 列；GameLibraryPathChangeModal 在 ModalPortal 里，
// 不占位置。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil
	cfg := p.Config

	s.accountSection(c, t, p)

	panel.Field(c, t, "vndb-token", vndbTokenLabel, "", func() {
		v := cfg.VNDBAccessToken
		if better.Input(c, t, "input", &v, better.InputProps{Label: vndbTokenLabel}).Changed() {
			p.Change(func(cfg *appconf.AppConfig) { cfg.VNDBAccessToken = v })
		}
	})

	panel.Field(c, t, "theme", locales.SettingsBasicThemeLabel(), "", func() {
		better.Select(c, t, "select", better.SelectProps{
			Value: cfg.Theme,
			Options: []better.SelectOption{
				{Value: "light", Label: locales.SettingsBasicThemeLight()},
				{Value: "dark", Label: locales.SettingsBasicThemeDark()},
				{Value: "system", Label: locales.SettingsBasicThemeSystem()},
			},
			OnChange: func(v string) { p.Change(func(cfg *appconf.AppConfig) { cfg.Theme = v }) },
		})
	})

	panel.Field(c, t, "language", locales.SettingsBasicLanguageLabel(), "", func() {
		better.Select(c, t, "select", better.SelectProps{
			Value:    cfg.Language,
			Options:  languageOptions(),
			OnChange: func(v string) { p.Change(func(cfg *appconf.AppConfig) { cfg.Language = v }) },
		})
	})

	// 界面缩放：onZoomChange → patchLiveConfig({ window_zoom_factor })，立即保存。
	panel.Field(c, t, "zoom", locales.SettingsBasicZoomLabel(), locales.SettingsBasicZoomHint(), func() {
		zoom := cfg.WindowZoomFactor
		if zoom == 0 {
			zoom = 1
		}
		better.Select(c, t, "select", better.SelectProps{
			Value:   zoomValue(zoom),
			Options: appZoomOptions(),
			OnChange: func(v string) {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return
				}
				p.PatchLive(func(cfg *appconf.AppConfig) { cfg.WindowZoomFactor = f })
			},
		})
	})

	panel.Field(c, t, "timezone", locales.SettingsBasicTimezoneLabel(), locales.SettingsBasicTimezoneHint(), func() {
		tz := cfg.TimeZone
		if tz == "" {
			tz = "Asia/Shanghai"
		}
		better.Select(c, t, "select", better.SelectProps{
			Value:       tz,
			Options:     commonTimezones,
			Placeholder: locales.SettingsBasicTimezonePlaceholder(),
			OnChange:    func(v string) { p.Change(func(cfg *appconf.AppConfig) { cfg.TimeZone = v }) },
		})
	})

	panel.Field(c, t, "game-library", locales.SettingsBasicGameLibraryPath(), locales.SettingsBasicGameLibraryPathHint(), func() {
		s.libraryInput(c, t, p)
	})

	s.libraryModal(c, t, p)

	// 关闭到托盘：label 没有 htmlFor，点文字不切换。
	ui.Column(c.Key("close-to-tray")).FillWidth().Gap(8).Children(func() {
		switchRow(c, t, "row", locales.SettingsBasicCloseToTray(), false, cfg.CloseToTray, func(v bool) {
			p.Change(func(cfg *appconf.AppConfig) { cfg.CloseToTray = v })
		})
	})

	// 开机自启：label htmlFor="launch_at_login" cursor-pointer，点文字也切换。原型不支持设置开机自启
	// （SetAutostart 返回 ErrUnavailable），保存失败时由设置页提示并恢复草稿。
	ui.Column(c.Key("launch-at-login")).FillWidth().Gap(8).Children(func() {
		switchRow(c, t, "row", locales.SettingsBasicLaunchAtLogin(), true, cfg.LaunchAtLogin, func(v bool) {
			p.Change(func(cfg *appconf.AppConfig) { cfg.LaunchAtLogin = v })
		})
	})
}

// switchRow 对应 flex items-center justify-between gap-4：左侧 label（block text-sm font-medium text-brand-700
// dark:text-brand-300），右侧 BetterSwitch。clickable 对应 label 的 htmlFor。
func switchRow(c *ui.Context, t *theme.Theme, key, label string, clickable, checked bool, onChange func(bool)) {
	ui.Row(c.Key(key)).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		box := ui.Box(c.Key("label")).MinWidth(0) // flex 子项：宽度是文字的宽度，只有文字可以点
		if clickable {
			box.OnClick(func() { onChange(!checked) })
		}
		box.Children(func() { panel.FieldLabel(c, t, label) })
		better.Switch(c, t, "switch", checked, false, onChange)
	})
}
