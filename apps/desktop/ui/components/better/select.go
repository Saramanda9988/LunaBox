package better

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// SelectOption 是 BetterSelectOption。
type SelectOption struct {
	Label string
	Value string
}

// SelectProps 是 BetterSelect 的属性。
type SelectProps struct {
	Value       string
	Options     []SelectOption
	Placeholder string // 默认“请选择”（原版硬编码的中文，这里是 native.selectPlaceholder）
	Disabled    bool
	OnChange    func(value string)
	// Font 是按钮文字的字号（className 的 text-*），默认 text-base。
	Font theme.FontStep
	// Style 在默认样式之后调整按钮（buttonClassName，如 h-11 rounded-lg py-0）。
	Style func(btn ui.Element)
}

// Select 对应 BetterSelect：按钮 px-3 py-2 pr-10 rounded-md，弹出列表 mt-1 py-1 rounded-md max-h-60，
// 选项 py-2 pl-10 pr-4，选中项左侧 check。宽度填满父元素。
func Select(c *ui.Context, t *theme.Theme, key string, p SelectProps) {
	label := p.Placeholder
	if label == "" {
		label = locales.NativeSelectPlaceholder()
	}
	// 没有选中项时显示占位文字：text-brand-400 dark:text-brand-500。
	labelColor := t.Pick(theme.Brand400, theme.Brand500)
	for _, o := range p.Options {
		if o.Value == p.Value {
			label, labelColor = o.Label, t.SelectText
		}
	}
	btn := ui.ButtonBase(c.Key(key)).FillWidth().Padding(8, 40, 8, 12).Radius(6).
		Justify(ui.Start).FocusRing(false)
	bg, border := t.GlassInput(t.SelectBg, t.SelectBorder) // glass-input
	btn.Background(bg).Border(t.HL, border)
	if p.Style != nil {
		p.Style(btn)
	}
	font := p.Font
	if font.Size == 0 {
		font = theme.TextBase
	}
	open := ui.Local(btn, "open", func() bool { return false })
	if p.Disabled {
		components.Disable(btn, 0.5)
		*open = false
	}
	btn.OnClick(func() { *open = !*open }).Children(func() {
		components.Text(c, label, font, theme.WeightNormal, labelColor).SingleLine().Grow(1)
		ui.Box(c).Absolute().Top(0).Bottom(0).Right(12).Center().Children(func() {
			components.Icon(c, "chevron-down", 20, t.TextFaint)
		})
	})
	width := btn.Bounds().W
	sb := ui.Local(btn, "scroll", func() components.ScrollBar { return components.ScrollBar{} })
	ui.PopoverBase(c, btn, open, func(panel ui.Element) {
		panel.Width(width).Margin(4, 0, 0, 0).Radius(6).Background(t.PanelBg).Border(t.HL, t.SelectBorder)
		panel.Children(func() {
			// max-h-60 overflow-auto：选项多时在列表里滚动，滚动条出现时占用右侧宽度（overflow-auto 不留 gutter）。
			components.ScrollArea(c, t, "scroll", sb, func(w ui.Element) {
				w.FillWidth().MaxHeight(240 - 2*t.HL).Radius(6 - t.HL)
			}, func(sc ui.Element) {
				gutter := float32(0)
				if sb.Overflowing() {
					gutter = t.Gutter()
				}
				sc.Children(func() {
					ui.Column(c).FillWidth().Padding(4, gutter, 4, 0).Children(func() { selectOptions(c, t, p, open) })
				})
			})
		})
	})
}

// selectOptions 是弹出列表里的选项：py-2 pl-10 pr-4，选中项左侧 check。
func selectOptions(c *ui.Context, t *theme.Theme, p SelectProps, open *bool) {
	check := t.Pick(theme.Neutral600, theme.Neutral400) // text-neutral-600 dark:text-neutral-400
	for _, o := range p.Options {
		item := ui.ButtonBase(c.Key("opt-"+o.Value)).FillWidth().Padding(8, 16, 8, 40).Justify(ui.Start).FocusRing(false)
		selected := o.Value == p.Value
		switch {
		case item.Hovered():
			item.Background(t.ListFocus)
		case selected:
			item.Background(t.ListSelected)
		}
		weight := theme.WeightNormal
		if selected {
			weight = theme.WeightMedium
		}
		item.OnClick(func() {
			*open = false
			if p.OnChange != nil {
				p.OnChange(o.Value)
			}
		}).Children(func() {
			if selected {
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(12).Center().Children(func() {
					components.Icon(c, "check", 20, check)
				})
			}
			components.Text(c, o.Label, theme.TextBase, weight, t.SelectText).SingleLine()
		})
	}
}
