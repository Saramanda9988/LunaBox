// Package grid 对应原版 frontend/src/components/grid/VirtualGameGrid.tsx。
package grid

import (
	"math"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components/card"
)

// 与 VirtualGameGrid.tsx 相同的网格参数（rem 按 16px）。
const (
	portraitMinWidth  = 140 // 8.75rem
	landscapeMinWidth = 256 // 16rem
	Gap               = 12  // 0.75rem
	cardMetaHeight    = 56  // 3.5rem，行高估算用
	overscan          = 2
)

// Geometry 是网格在给定宽度下的列数、卡片宽度与行高。
type Geometry struct {
	Width     float32
	Cols      int
	CardW     float32
	RowH      float32
	Landscape bool
}

// NewGeometry 按 grid-cols-[repeat(auto-fill,minmax(min,1fr))] 计算列数。
func NewGeometry(width float32, landscape bool) Geometry {
	minW := float32(portraitMinWidth)
	if landscape {
		minW = landscapeMinWidth
	}
	cols := max(1, int(math.Floor(float64((width+Gap)/(minW+Gap)))))
	cardW := (width - Gap*float32(cols-1)) / float32(cols)
	return Geometry{
		Width:     width,
		Cols:      cols,
		CardW:     cardW,
		RowH:      float32(math.Ceil(float64(cardW*card.CoverRatio(landscape) + cardMetaHeight))),
		Landscape: landscape,
	}
}

// Props 是 VirtualGrid 的输入。
type Props struct {
	Geometry Geometry
	Total    int
	// ViewTop、ViewBottom 是可见区域在窗口中的纵坐标（DIP），用于决定构建哪些行。
	ViewTop, ViewBottom float32
	// OnRange 在构建前报告可见下标范围 [start, end]，页面据此请求缺失的分页。
	OnRange func(start, end int)
	// Item 构建第 i 个格子；用游戏 ID 等稳定的值作 c.Key，滚动时卡片才能保留悬停等状态。
	Item func(c *ui.Context, i int)
}

// VirtualGrid 只构建可见区域（加上下 overscan 两行）的行，每行绝对定位在 index*rowH，
// 容器高度为全部行高之和，滚动条因此与完整列表一致。
func VirtualGrid(c *ui.Context, p Props) {
	g := p.Geometry
	rows := (p.Total + g.Cols - 1) / g.Cols
	box := ui.Box(c.Key("vgrid")).FillWidth().Height(float32(rows) * g.RowH)
	top := box.Bounds().Y // 上一帧在窗口中的位置，已含滚动偏移
	if box.Bounds().H == 0 {
		top = p.ViewTop + 250
	}
	first := max(0, int(math.Floor(float64((p.ViewTop-top)/g.RowH)))-overscan)
	last := min(rows-1, int(math.Floor(float64((p.ViewBottom-top)/g.RowH)))+overscan)
	if last < first {
		return
	}
	if p.OnRange != nil {
		p.OnRange(first*g.Cols, min(p.Total-1, (last+1)*g.Cols-1))
	}
	box.Children(func() {
		for r := first; r <= last; r++ {
			ui.Row(c.Key(r)).Absolute().Top(float32(r) * g.RowH).Left(0).Right(0).Gap(Gap).AlignItems(ui.Start).Children(func() {
				for col := range g.Cols {
					i := r*g.Cols + col
					if i >= p.Total {
						break
					}
					p.Item(c, i)
				}
			})
		}
	})
}
