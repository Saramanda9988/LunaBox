package components

import (
	"github.com/egoist/mygo/ui"
)

// Chromium 原生复选框（原版没有设置 color-scheme，深色主题下也是浅色的样式）的颜色。
var (
	checkboxBorder      = ui.Hex("#767676")
	checkboxBorderHover = ui.Hex("#4f4f4f")
	checkboxAccent      = ui.Hex("#0075ff")
	checkboxAccentHover = ui.Hex("#005cc8")
)

// CheckboxProps 是原生 <input type="checkbox"> 的属性。
type CheckboxProps struct {
	Checked       bool
	Indeterminate bool // element.indeterminate：显示横线
	Disabled      bool
	Label         string // aria-label
	OnChange      func(checked bool)
}

// Checkbox 对应原版里没有样式的 <input type="checkbox">：13px 见方、圆角 2px、外边距 3px 3px 3px 4px，
// 选中时是 Chromium 的强调色（#0075ff）与白色对勾。
func Checkbox(c *ui.Context, key string, p CheckboxProps) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(13, 13).Margin(3, 3, 3, 4).Radius(2).Center().Shrink(0).FocusRing(false).Label(p.Label)
	on := p.Checked || p.Indeterminate
	hovered := btn.Hovered() && !p.Disabled
	switch {
	case on && hovered:
		btn.Background(checkboxAccentHover)
	case on:
		btn.Background(checkboxAccent)
	case hovered:
		btn.Background(ui.Hex("#ffffff")).Border(1, checkboxBorderHover)
	default:
		btn.Background(ui.Hex("#ffffff")).Border(1, checkboxBorder)
	}
	if p.Disabled {
		btn.Disabled(true) // MyGo 把禁用元素画成半透明，近似 Chromium 的灰色禁用样式
	} else if p.OnChange != nil {
		btn.OnClick(func() { p.OnChange(!p.Checked) })
	}
	check := ui.Hex("#ffffff")
	btn.Children(func() {
		switch {
		case p.Indeterminate:
			ui.Box(c).Size(7, 2).Radius(1).Background(check)
		case p.Checked:
			Icon(c, "check-bold", 11, check)
		}
	})
	return btn
}
