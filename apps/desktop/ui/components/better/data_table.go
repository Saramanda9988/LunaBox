package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// DataTableColumn 是 BetterDataTableColumn。
type DataTableColumn[T any] struct {
	Key    string
	Header string
	// Width 是 className 里的列宽（w-[30%] 写成 0.3）。表格是 table-fixed，各列按 Width 的比例分宽度；
	// 为 0 的列与其他为 0 的列平分（日历的 w-[14.285%] 七列等宽，可以都写 0）。
	Width float32
	// FixedWidth 是 w-16、w-32 这类固定宽度（px）；不为 0 时 Width 不起作用，其余列分剩下的宽度。
	FixedWidth float32
	// Center 对应 cellClassName 的 text-center。内容比格子宽时（ContentWidth 大于格子的内容宽度），
	// 与 CSS 一样从左边开始、向右溢出。表头的 headerClassName="text-center" 在原版里被基础样式里
	// 更靠后的 text-left 盖掉，所以表头总是左对齐。
	Center       bool
	ContentWidth float32
	// Cell 是 render：在 td（px-3 py-2 align-middle 的横向行）里构建内容。块级内容（原版的 div）要自己 Grow(1).MinWidth(0)。
	Cell func(row T, index int)
	// HeaderCell 是 header 不是文字时（如全选的复选框）的内容；为 nil 时显示 Header。
	HeaderCell func()
	// Filter 是列筛选（FilterHeader）：表头右侧的漏斗按钮与下拉菜单。
	Filter *ColumnFilter
}

// DataTableProps 是 BetterDataTable 的属性。
type DataTableProps[T any] struct {
	Rows    []T
	Columns []DataTableColumn[T]
	RowKey  func(row T, index int) string
	Empty   func() // empty：没有行时 p-8 text-center 的内容
	// MaxHeight 是 maxHeightClassName 的 max-h-*（原版默认 max-h-[400px]）；0 为 max-h-none，不滚动。
	MaxHeight float32
	// RowOpacity 是 rowClassName 里的 opacity-*（返回 0 或 1 时不透明）。
	RowOpacity func(row T, index int) float32
	Selection  *TableSelection[T]
}

// DataTable 对应 BetterDataTable：overflow-auto rounded-lg border border-brand-200 bg-white/80 dark:border-brand-700
// dark:bg-brand-900/50；表头 sticky bg-brand-50/95 dark:bg-brand-800/95，th 是 border-b px-3 py-2 text-xs font-semibold
// text-brand-600 dark:text-brand-300；td 是 px-3 py-2 text-sm text-brand-700 dark:text-brand-300，行 hover:bg-brand-50/80
// dark:hover:bg-brand-800/70。表格是 border-separate，tbody 的 divide-y 画在 tr 上，不显示。
//
// 有 MaxHeight 时表头固定在滚动区外（sticky），滚动条（components.ScrollArea）只在表体旁边；原版的滚动条从表头开始。
func DataTable[T any](c *ui.Context, t *theme.Theme, key string, p DataTableProps[T]) ui.Element {
	// 玻璃模式：data-glass:bg-white/10 data-glass:dark:bg-black/10
	box := ui.Column(c.Key(key)).FillWidth().Radius(8).Clip().Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
		Background(t.IfGlass(t.Pick(theme.White.Alpha(0.8), theme.Brand900.Alpha(0.5)), theme.White.Alpha(0.1), theme.Black.Alpha(0.1)))
	sb := ui.Local(box, "scroll", func() components.ScrollBar { return components.ScrollBar{} })
	// cell 按列宽设置格子：固定宽度，或按 Width 的比例分宽度。
	cell := func(e ui.Element, col DataTableColumn[T]) ui.Element {
		if col.FixedWidth > 0 {
			return e.Width(col.FixedWidth).Shrink(0)
		}
		g := col.Width
		if g <= 0 {
			g = 1
		}
		return e.Grow(g).Basis(0).MinWidth(0)
	}
	// 滚动条是常显的 9px 槽（::-webkit-scrollbar），超出时占用表格的宽度。
	gutter := float32(0)
	if p.MaxHeight > 0 && sb.Overflowing() {
		gutter = t.Gutter()
	}
	box.Children(func() {
		if len(p.Rows) == 0 {
			// p-8 text-center text-sm text-brand-400 dark:text-brand-500
			ui.Column(c).Padding(32).AlignItems(ui.Center).Children(func() {
				if p.Empty != nil {
					p.Empty()
				}
			})
			return
		}
		sel := p.Selection
		selectable := func(row T, i int) bool { return sel.Selectable == nil || sel.Selectable(row, i) }
		body := func() {
			for i, row := range p.Rows {
				tr := ui.Row(c.Key(p.RowKey(row, i))).AlignItems(ui.Stretch)
				if tr.Hovered() {
					tr.Background(t.Pick(theme.Brand50.Alpha(0.8), theme.Brand800.Alpha(0.7)))
				}
				if p.RowOpacity != nil {
					if o := p.RowOpacity(row, i); o > 0 && o < 1 {
						tr.Opacity(o)
					}
				}
				tr.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
					if sel != nil {
						// w-12 px-3 py-2 text-center align-middle
						ui.Row(c.Key("__select__")).Width(48).Shrink(0).Padding(8, 12).Center().Children(func() {
							label := "Select row"
							if sel.RowLabel != nil {
								label = sel.RowLabel(row, i)
							}
							ok := selectable(row, i)
							components.Checkbox(c, "check", components.CheckboxProps{
								Checked: ok && sel.Selected(row, i), Disabled: !ok, Label: label,
								OnChange: func(on bool) { sel.OnToggleRow(row, i, on) },
							})
						})
					}
					for _, col := range p.Columns {
						td := cell(ui.Row(c.Key(col.Key)), col).Padding(8, 12).AlignItems(ui.Center)
						if col.Center && (col.ContentWidth == 0 || td.Bounds().W-24 >= col.ContentWidth) {
							td.Justify(ui.Center)
						}
						td.Children(func() { col.Cell(row, i) })
					}
				})
			}
		}
		// 表头：bg-brand-50/95 dark:bg-brand-800/95，data-glass:bg-white/20 data-glass:dark:bg-black/30
		thead := t.IfGlass(t.Pick(theme.Brand50.Alpha(0.95), theme.Brand800.Alpha(0.95)), theme.White.Alpha(0.2), theme.Black.Alpha(0.3))
		headColor, headBorder := t.Pick(theme.Brand600, theme.Brand300), t.Pick(theme.Brand200, theme.Brand700)
		// th 都与最高的一格同高（表格行），内容垂直居中
		ui.Row(c.Key("thead")).Padding(0, gutter, 0, 0).AlignItems(ui.Stretch).Background(thead).Children(func() {
			if sel != nil {
				total, selected := 0, 0
				for i, row := range p.Rows {
					if selectable(row, i) {
						total++
						if sel.Selected(row, i) {
							selected++
						}
					}
				}
				label := sel.AllLabel
				if label == "" {
					label = "Select all"
				}
				ui.Row(c.Key("__select__")).Width(48).Shrink(0).Padding(8, 12).Center().
					BorderWidth(0, 0, t.HL, 0).BorderColor(headBorder).Children(func() {
					components.Checkbox(c, "check-all", components.CheckboxProps{
						Checked: total > 0 && selected == total, Indeterminate: selected > 0 && selected < total,
						Disabled: total == 0, Label: label,
						OnChange: func(on bool) {
							if sel.OnToggleAll != nil {
								sel.OnToggleAll(on)
							}
						},
					})
				})
			}
			for _, col := range p.Columns {
				th := cell(ui.Row(c.Key(col.Key)), col).Padding(8, 12).AlignItems(ui.Center).
					BorderWidth(0, 0, t.HL, 0).BorderColor(headBorder)
				th.Children(func() {
					header := func() {
						if col.HeaderCell != nil {
							col.HeaderCell()
							return
						}
						components.Text(c, col.Header, theme.TextXS, theme.WeightSemibold, headColor).SingleLine()
					}
					if col.Filter == nil {
						header()
						return
					}
					// FilterHeader：flex w-full items-center gap-1.5，标题 min-w-0 flex-1 truncate
					ui.Row(c).Grow(1).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Row(c).Grow(1).Basis(0).MinWidth(0).Children(header)
						filterButton(c, t, col.Filter)
					})
				})
			}
		})
		if p.MaxHeight <= 0 {
			body()
			return
		}
		// 表头 sticky：放在滚动区外面（py-2 加 text-xs 的行高是 32，再加 border-b 与外框的上下边框）。
		maxH := max(0, p.MaxHeight-32-3*t.HL)
		components.ScrollArea(c, t, "scroll", sb, func(w ui.Element) { w.FillWidth().MaxHeight(maxH) }, func(sc ui.Element) {
			sc.Children(func() {
				ui.Column(c).FillWidth().Padding(0, gutter, 0, 0).Children(body)
			})
		})
	})
	return box
}
