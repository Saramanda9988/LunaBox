// Package proxy 对应原版 components/panel/ProxySettingsPanel.tsx（设置页的“网络代理”分区）。
package proxy

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// State 是面板跨帧保留的状态，由设置页持有。面板没有自己的状态。
type State struct{}

// modeOptions 对应 modeOptions：跟随系统、手动代理、直连。
func modeOptions() []better.SelectOption {
	return []better.SelectOption{
		{Value: "system", Label: locales.SettingsProxyModeSystem()},
		{Value: "manual", Label: locales.SettingsProxyModeManual()},
		{Value: "direct", Label: locales.SettingsProxyModeDirect()},
	}
}

// View 对应 ProxySettingsPanel：根元素 space-y-4，依次是代理模式（label、BetterSelect、提示）与手动代理 URL
// （label、提示、BetterInput，不是手动模式时再加一行 text-xs text-brand-400 dark:text-brand-500 的说明）。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	_ = s
	cfg := p.Config
	mode := cfg.NetworkProxyMode
	if mode == "" {
		mode = "system"
	}
	ui.Column(c.Key("proxy-panel")).FillWidth().Gap(16).Children(func() {
		panel.Field(c, t, "mode", locales.SettingsProxyGlobalMode(), locales.SettingsProxyGlobalModeHint(), func() {
			better.Select(c, t, "select", better.SelectProps{
				Value:   mode,
				Options: modeOptions(),
				OnChange: func(v string) {
					p.Change(func(cfg *appconf.AppConfig) { cfg.NetworkProxyMode = v })
				},
			})
		})

		// space-y-2：label、提示在输入框之前。
		ui.Column(c.Key("url")).FillWidth().Gap(8).Children(func() {
			label := locales.SettingsProxyManualProxyURL()
			panel.FieldLabel(c, t, label)
			panel.FieldHint(c, t, locales.SettingsProxyManualProxyURLHint())
			v := cfg.NetworkProxyURL
			// 原版的输入框没有 aria-label，无障碍名称来自占位文字。
			if better.Input(c, t, "input", &v, better.InputProps{Placeholder: locales.SettingsProxyManualProxyURLPlaceholder()}).Changed() {
				p.Change(func(cfg *appconf.AppConfig) { cfg.NetworkProxyURL = v })
			}
			if mode != "manual" {
				components.Text(c, locales.SettingsProxyManualProxyURLIdle(), theme.TextXS, theme.WeightNormal,
					t.Pick(theme.Brand400, theme.Brand500))
			}
		})
	})
}
