package better

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// SettingSwitchRowProps 是 SettingSwitchRow 的属性。
type SettingSwitchRowProps struct {
	Label    string
	Hint     string // 为空时不显示提示（原版的 SettingSwitchRow 总有提示，各面板手写的同样结构有的没有）
	Checked  bool
	Disabled bool
	OnChange func(checked bool)
}

// SettingSwitchRow 对应 components/ui/SettingSwitchRow.tsx：flex items-center justify-between gap-4，左侧 flex-1 space-y-2 的
// 标签（text-sm font-medium text-brand-700 dark:text-brand-300，禁用时 text-brand-400 dark:text-brand-500）与提示
// （text-xs text-brand-500 dark:text-brand-400），右侧 BetterSwitch。点击标签切换开关（label htmlFor）。
func SettingSwitchRow(c *ui.Context, t *theme.Theme, key string, p SettingSwitchRowProps) {
	labelColor := t.SectionLabel
	if p.Disabled {
		labelColor = t.Pick(theme.Brand400, theme.Brand500)
	}
	change := func(v bool) {
		if !p.Disabled && p.OnChange != nil {
			p.OnChange(v)
		}
	}
	ui.Row(c.Key(key)).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			label := ui.Box(c.Key("label")).AlignSelf(ui.Stretch)
			if !p.Disabled {
				label.OnClick(func() { change(!p.Checked) })
			}
			label.Children(func() { components.Text(c, p.Label, theme.TextSM, theme.WeightMedium, labelColor) })
			if p.Hint != "" {
				components.Text(c, p.Hint, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
			}
		})
		Switch(c, t, "switch", p.Checked, p.Disabled, change)
	})
}
