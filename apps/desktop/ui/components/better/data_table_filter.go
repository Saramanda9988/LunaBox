package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// ColumnFilter 是 BetterDataTableColumnFilter。
type ColumnFilter struct {
	Value    string
	Options  []ColumnFilterOption
	OnChange func(value string)
	Title    string
	AllLabel string // 默认 All
	Pill     bool
	AlignEnd bool // align="end"
}

// ColumnFilterOption 是 BetterDataTableColumnFilterOption。
type ColumnFilterOption struct {
	Value, Label     string
	Icon             string   // 默认 filter-variant
	IconColor        ui.Color // 默认 text-brand-400
	PillBg, PillText ui.Color
}

// TableSelection 是 BetterDataTable 的 selection：表格最前面 w-12 的复选框列。
type TableSelection[T any] struct {
	Selected    func(row T, index int) bool
	Selectable  func(row T, index int) bool // 为 nil 时都可选
	OnToggleRow func(row T, index int, checked bool)
	OnToggleAll func(checked bool)
	AllLabel    string                        // 默认 Select all
	RowLabel    func(row T, index int) string // 默认 Select row
}

// filterButton 是 FilterHeader 的触发按钮（h-6 w-6 rounded-md，筛选生效时 text-success-600 并在右上角显示绿点）
// 与 min-w-[200px] 的 BetterDropdownMenu：第一项“全部”，选中的一项显示对勾。
func filterButton(c *ui.Context, t *theme.Theme, f *ColumnFilter) {
	active := f.Value != ""
	title := f.Title
	if title == "" {
		title = "Filter"
	}
	btn := ui.ButtonBase(c.Key("filter")).Size(24, 24).Radius(6).Center().Shrink(0).FocusRing(false).Label(title)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100.Alpha(0.7), theme.Brand700.Alpha(0.5)))
	}
	color, icon := t.Pick(theme.Brand500, theme.Brand400), "filter-outline"
	if active {
		color, icon = t.Pick(theme.Success600, theme.Success400), "filter"
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Icon(c, icon, 14, color)
		if active {
			ui.Box(c).Absolute().Top(2).Right(2).Size(6, 6).Radius(999).Background(theme.Success500)
		}
	})
	itemIcon := func(selected bool, icon string, color ui.Color) (string, ui.Color) {
		if selected {
			return "check", theme.Success500
		}
		if icon == "" {
			icon = "filter-variant"
		}
		if color == (ui.Color{}) {
			color = theme.Brand400
		}
		return icon, color
	}
	all := f.AllLabel
	if all == "" {
		all = "All"
	}
	allIcon, allColor := itemIcon(!active, "", ui.Color{})
	items := []MenuItem{{Key: "__all__", Label: all, Icon: allIcon, IconColor: allColor, Pill: f.Pill, OnClick: func() { f.OnChange("") }}}
	for _, o := range f.Options {
		ic, col := itemIcon(f.Value == o.Value, o.Icon, o.IconColor)
		items = append(items, MenuItem{
			Key: o.Value, Label: o.Label, Icon: ic, IconColor: col, Pill: f.Pill, PillBg: o.PillBg, PillText: o.PillText,
			OnClick: func() { f.OnChange(o.Value) },
		})
	}
	DropdownMenu(c, t, btn, MenuProps{Title: f.Title, MinWidth: 200, Items: items, AlignStart: !f.AlignEnd})
}
