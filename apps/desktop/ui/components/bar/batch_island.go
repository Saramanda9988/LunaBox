package bar

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// 批量操作浮动岛（原版没有：原版的批量操作是 FilterBar 的第二行）。浮在页面底部居中，滚动时一直看得见，
// 不占文档流，进出批量模式时网格不会跳动。
const (
	batchIslandH      = 52
	batchIslandBottom = 24 // 与视口底边的距离
	batchIslandGap    = 16 // 网格最后一行与浮动岛之间留的空
	batchIslandSide   = 32 + 44 + 16
	batchIslandEnter  = 360 * time.Millisecond
	batchIslandExit   = 220 * time.Millisecond
)

// BatchIslandInset 是批量模式下页面滚动内容要多留的底部空白，让最后一行不被浮动岛挡住。
func BatchIslandInset(on bool) float32 {
	if !on {
		return 0
	}
	return batchIslandH + batchIslandBottom + batchIslandGap
}

// BatchIsland 在页面的滚动区域之后、弹窗之前构建：已选数量、全选 / 清空 / 帮助，页面的批量操作（p.BatchActions），
// 最右侧退出批量模式。批量模式下 Esc 退出、Ctrl+A（macOS 是 Command+A）全选；焦点在输入框里时按键先给输入框。
func BatchIsland(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	on := p.Filters.Batch
	exit := func() {
		if p.OnBatchChange != nil {
			p.OnBatchChange(false)
		}
	}
	if on {
		c.OnWindowShortcut(0, ui.KeyEscape, exit)
		if p.OnSelectAll != nil {
			c.OnWindowShortcut(ui.Cmd, ui.KeyA, p.OnSelectAll)
		}
	}

	// 在页面内容的宽度里居中：右侧让出滚动条的位置（scrollbar-gutter: stable），两侧让出“回到顶部”按钮。
	gutter := t.Gutter()
	frame := ui.Row(c.Key("batch-island")).Absolute().Left(batchIslandSide).Right(batchIslandSide + gutter).
		Bottom(batchIslandBottom).Justify(ui.Center).PassThrough()
	dur, ease := batchIslandEnter, easeIslandIn
	if !on {
		dur, ease = batchIslandExit, easeIslandOut
	}
	show := frame.AnimateWith("show", theme.Bool(on), dur, ease)
	if show == 0 && !on {
		frame.Invisible()
		return
	}
	frame.Opacity(show).Translate(0, 16*(1-show))

	frame.Children(func() {
		pill := ui.Row(c.Key("pill")).Height(batchIslandH).Padding(0, 8, 0, 16).Gap(4).AlignItems(ui.Center).
			Radius(999).Background(islandBg(t)).
			Shadow(0, 0, 0, t.HL, t.Pick(theme.Brand900.Alpha(0.1), theme.White.Alpha(0.12))). // ring-1，同灵动岛
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.25)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.2))
		if !on {
			pill.PassThrough() // 退出动画期间不再接收点击
		}
		pill.Children(func() {
			batchCount(c, t, p.SelectedCount)
			if p.OnSelectAll != nil {
				batchTextButton(c, t, "select-all", locales.CommonSelectAll(), p.OnSelectAll)
			}
			if p.OnClearSelection != nil {
				batchTextButton(c, t, "clear-selection", locales.CommonClearStore(), p.OnClearSelection)
			}
			help := ui.ButtonBase(c.Key("batch-help")).Size(28, 28).Radius(999).Center().FocusRing(false).Label(locales.CommonBatchSelectionHelp())
			help.Children(func() { components.Icon(c, "help-circle-outline", 18, t.TextMuted) })
			better.Tooltip(c, t, help, locales.CommonBatchSelectionHelp())

			if p.BatchActions != nil {
				BatchIslandDivider(c, t)
				ui.Row(c).Gap(2).AlignItems(ui.Center).Children(p.BatchActions)
			}
			BatchIslandDivider(c, t)
			BatchIslandButton(c, t, "exit-batch", BatchButton{Icon: "close", Label: locales.NativeExitBatchSelect(), OnClick: exit}).Radius(999)
		})
	})
}

// batchCount：“已选 N”，选中数量用强调色。
func batchCount(c *ui.Context, t *theme.Theme, n int) {
	ui.Row(c.Key("selected-count")).AlignItems(ui.Center).Gap(6).Padding(0, 8, 0, 0).Children(func() {
		components.Text(c, locales.CommonSelected(), theme.TextSM, theme.WeightMedium, t.TextMuted).NoWrap()
		color := t.TextStrong
		if n > 0 {
			color = t.Pick(theme.Brand600, theme.Brand300)
		}
		components.Text(c, strconv.Itoa(n), theme.TextBase, theme.WeightBold, color).NoWrap()
	})
}

func batchTextButton(c *ui.Context, t *theme.Theme, key, label string, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Height(32).Padding(0, 10).Radius(8).AlignItems(ui.Center).FocusRing(false).Label(label)
	color := t.NavText
	if btn.Hovered() {
		btn.Background(batchHoverBg(t))
		color = t.TextStrong
	}
	btn.OnClick(onClick).Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightMedium, color).NoWrap()
	})
}

// BatchButton 是浮动岛里的一个操作按钮。
type BatchButton struct {
	Icon     string // "loading" 时显示转圈
	Label    string // 无障碍标签与提示；ShowText 时也显示在图标右侧
	ShowText bool
	Danger   bool
	Disabled bool
	OnClick  func() // 为 nil 时由调用方处理（例如作为下拉菜单的触发按钮）
}

// BatchIslandButton 是浮动岛里的无边框按钮：36px 高，悬停时浅色底，危险操作用红色。
func BatchIslandButton(c *ui.Context, t *theme.Theme, key string, b BatchButton) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Height(36).MinWidth(36).Radius(10).Gap(6).AlignItems(ui.Center).Justify(ui.Center).
		FocusRing(false).Label(b.Label)
	if b.ShowText {
		btn.Padding(0, 12)
	}
	color := t.NavText
	if b.Danger {
		color = t.Pick(theme.Error600, theme.Error400)
	}
	if b.Disabled {
		components.Disable(btn, 0.4)
	} else {
		if btn.Hovered() {
			if b.Danger {
				btn.Background(t.Pick(theme.Error600.Alpha(0.1), theme.Error400.Alpha(0.15)))
			} else {
				btn.Background(batchHoverBg(t))
			}
		}
		if b.OnClick != nil {
			btn.OnClick(b.OnClick)
		}
	}
	btn.Children(func() {
		if b.Icon == "loading" {
			components.Spinner(c, 18, color)
		} else {
			components.Icon(c, b.Icon, 18, color)
		}
		if b.ShowText {
			components.Text(c, b.Label, theme.TextSM, theme.WeightMedium, color).NoWrap()
		}
	})
	if !b.ShowText {
		better.Tooltip(c, t, btn, b.Label)
	}
	return btn
}

// BatchIslandDivider 是浮动岛里分组之间的竖线。
func BatchIslandDivider(c *ui.Context, t *theme.Theme) {
	ui.Box(c).Width(t.HL).Height(20).Margin(0, 6).Shrink(0).Background(t.Pick(theme.Brand900.Alpha(0.12), theme.White.Alpha(0.14)))
}

func batchHoverBg(t *theme.Theme) ui.Color {
	return t.Pick(theme.Brand900.Alpha(0.06), theme.White.Alpha(0.1))
}
