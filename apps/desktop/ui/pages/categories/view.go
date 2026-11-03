package categories

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/bar"
	"lunabox/desktop/ui/components/card"
	"lunabox/desktop/ui/components/grid"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

const pagePadding = 32 // p-8

// View 构建页面：h-full w-full overflow-y-auto scrollbar-stable p-8，标题、FilterBar 与收藏夹网格。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	if p.failed != nil {
		p.env.Toast(c, env.Error(p.failed()))
		p.failed = nil
	}
	if p.succeeded != nil {
		p.env.Toast(c, env.Success(p.succeeded()))
		p.succeeded = nil
	}
	if p.loading && len(p.categories) == 0 {
		if wait := skeletonDelay - c.Now().Sub(p.openedAt); wait > 0 {
			c.After(wait)
			return
		}
		p.skeleton(c, t)
		return
	}

	list := p.visible()
	fp := p.filterProps(c, t, list)
	gutter := t.Gutter()
	contentW := p.env.MainWidth - 2*pagePadding - gutter
	components.ScrollArea(c, t, "categories-scroll", &p.scroll, func(w ui.Element) {
		w.Fill()
		if p.loading { // opacity-50 pointer-events-none
			w.Opacity(0.5).PassThrough()
		}
	}, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			ui.Column(c).Width(contentW).Children(func() {
				components.Text(c, locales.CategoriesTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
				bar.FilterBar(c, t, fp)
				p.grid(c, t, list, contentW)
			})
		})
	})
	bar.FilterDrawer(c, t, fp)
	components.CategoryModal(c, t, "add-category", components.CategoryModalProps{
		Open:      p.addOpen,
		Value:     &p.newName,
		TopOffset: env.TopbarHeight,
		OnClose:   func() { p.addOpen, p.newName = false, "" },
		OnSubmit:  p.add,
	})
	components.ConfirmModal(c, t, "batch-delete", components.ConfirmProps{
		Open:      &p.confirmOpen,
		Title:     locales.CategoriesToastBatchDeleteTitle(),
		Message:   batchDeleteMessage(p.selectedCount()),
		Danger:    true,
		TopOffset: env.TopbarHeight,
		OnConfirm: p.batchDelete,
	})
}

// filterProps 是 categories.tsx 传给 FilterBar 的属性：搜索、排序（没有次排序与游戏筛选）、批量删除与“新建收藏夹”。
func (p *Page) filterProps(c *ui.Context, t *theme.Theme, list []vo.CategoryVO) bar.FilterBarProps {
	f := &p.filters
	return bar.FilterBarProps{
		Env:               p.env,
		Filters:           f,
		SearchPlaceholder: locales.CategoriesSearchPlaceholder(),
		SortOptions:       sortOptions(),
		DefaultSortBy:     sortByName,
		DefaultSortOrder:  enums.SortOrderAsc,
		OnBatchChange: func(on bool) {
			f.Batch = on
			if !on {
				clear(p.selected)
			}
		},
		SelectedCount: p.selectedCount(),
		OnSelectAll: func() {
			for _, cat := range list {
				p.setSelected(cat, true)
			}
		},
		OnClearSelection: func() { clear(p.selected) },
		BatchActions:     func() { p.deleteButton(c, t) },
		ActionButton:     func() { p.newButton(c, t) },
	}
}

// gridColumns 对应 grid-cols-[repeat(auto-fill,minmax(min(11rem,100%),1fr))] gap-4：列数与列宽。
func gridColumns(width float32) (cols int, colW float32) {
	minW := min(card.CategoryMinWidth, width)
	cols = max(1, int((width+card.CategoryGap)/(minW+card.CategoryGap)))
	return cols, (width - card.CategoryGap*float32(cols-1)) / float32(cols)
}

// grid：pt-2 的收藏夹网格，卡片 w-full max-w-48 在格子里居中（justify-items-center）。批量模式下网格接收拖动选择
// （useDragSelection），可选的卡片（非系统收藏夹，原版的 data-drag-selection-id）登记为目标。
func (p *Page) grid(c *ui.Context, t *theme.Theme, list []vo.CategoryVO, width float32) {
	cols, colW := gridColumns(width)
	cardW := min(card.CategoryMaxWidth, colW)
	blur := true
	if cfg := p.env.Config(); cfg != nil {
		blur = cfg.BlurNSFWGameCovers
	}
	byID := make(map[string]vo.CategoryVO, len(list))
	for _, cat := range list {
		byID[cat.ID] = cat
	}
	g := ui.Grid(c.Key("category-grid")).Columns(cols).GapX(card.CategoryGap).GapY(card.CategoryGap).Padding(8, 0, 0, 0)
	p.drag.Surface(c, g, grid.DragProps{
		Enabled:  p.filters.Batch,
		Selected: func(id string) bool { return p.selected[id] },
		OnSelectChange: func(id string, on bool) {
			if cat, ok := byID[id]; ok {
				p.setSelected(cat, on)
			}
		},
		Scroll: &p.scroll.State,
	})
	g.Children(func() {
		for _, cat := range list {
			ui.Box(c.Key(cat.ID)).AlignItems(ui.Center).Children(func() {
				build := func(c *ui.Context) {
					card.CategoryCard(c, t, card.CategoryCardProps{
						Category:          cat,
						Width:             cardW,
						Covers:            func(g *models.Game) *ui.Bitmap { return p.env.Covers.Get(g) },
						BlurNSFW:          blur,
						SelectionMode:     p.filters.Batch,
						Selected:          p.selected[cat.ID],
						SelectionDisabled: cat.IsSystem,
						OnSelectChange:    func(on bool) { p.setSelected(cat, on) },
						OnOpen:            func() { p.env.Router.Push(env.CategoryPath(cat.ID)) },
					})
				}
				if cat.IsSystem {
					build(c)
				} else {
					p.drag.Target(c, cat.ID, build)
				}
			})
		}
	})
}

// deleteButton 是 batchActions：glass-panel px-3 py-2 rounded-xl border bg-white text-error-600 的删除按钮
// （dark:bg-brand-800 dark:text-error-400），没有选中时 opacity-50。
func (p *Page) deleteButton(c *ui.Context, t *theme.Theme) {
	label := locales.CategoriesToastBatchDeleteTitle()
	bg, border := t.GlassPanel(t.PanelBg, t.PanelBorder)
	btn := ui.ButtonBase(c.Key("batch-delete")).Padding(8, 12).Radius(12).Border(t.HL, border).Background(bg).
		AlignItems(ui.Center).FocusRing(false).Label(label)
	if p.selectedCount() == 0 {
		components.Disable(btn, 0.5)
	} else {
		if btn.Hovered() && !t.Glass { // glass-panel 的 data-glass:bg 比 hover:bg 更具体
			btn.Background(t.MenuHover)
		}
		btn.OnClick(func() { p.confirmOpen = true })
	}
	btn.Children(func() { components.Icon(c, "delete", 18, t.Pick(theme.Error600, theme.Error400)) })
}

// newButton 是 actionButton：glass-btn-primary rounded-xl border border-neutral-500 bg-neutral-600 px-4 py-2
// text-sm font-medium text-white shadow-sm，悬停 -translate-y-0.5 bg-neutral-700 shadow-md（transition-all）。
func (p *Page) newButton(c *ui.Context, t *theme.Theme) {
	g := t.GlassButton(theme.GlassPrimary, theme.BtnColors{Bg: theme.Neutral600, Hover: theme.Neutral700, Border: theme.Neutral500, Text: theme.White})
	btn := ui.ButtonBase(c.Key("new-category")).Padding(8, 16).Radius(12).AlignItems(ui.Center).FocusRing(false).
		Border(t.HL, g.Border).Label(locales.CategoriesNewCategory())
	hovered := btn.Hovered()
	bg := g.Bg
	if hovered {
		bg = g.Hover
	}
	lift := btn.AnimateWith("lift", theme.Bool(hovered), 150*time.Millisecond, theme.EaseTailwind)
	btn.Background(bg).Transition(theme.ColorsTransition(150*time.Millisecond)).Translate(0, -2*lift)
	if hovered {
		btn.Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
	} else {
		btn.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	}
	btn.OnClick(func() { p.addOpen = true }).Children(func() {
		components.Icon(c, "plus", 18, g.Text).Margin(0, 8, 0, 0)
		components.Text(c, locales.CategoriesNewCategory(), theme.TextSM, theme.WeightMedium, g.Text).NoWrap()
	})
}

// skeleton 对应 CategoriesSkeleton（animate-pulse，px-6 py-7 lg:px-8 lg:py-9），玻璃模式下各块是
// data-glass:bg-white/8 dark:bg-black/12。
func (p *Page) skeleton(c *ui.Context, t *theme.Theme) {
	bg := t.IfGlass(t.Pick(theme.Brand200, theme.Brand800), theme.White.Alpha(0.08), theme.Black.Alpha(0.12))
	winW, _ := c.Size()
	padX, padY := float32(24), float32(28)
	if winW >= 1024 {
		padX, padY = 32, 36
	}
	gutter := t.Gutter()
	width := p.env.MainWidth - 2*padX - gutter
	cols, colW := gridColumns(width)
	cardW := min(card.CategoryMaxWidth, colW)
	wrap := ui.Column(c.Key("skeleton")).Fill().Clip().Padding(padY, padX+gutter, padY, padX).PassThrough()
	wrap.Opacity(components.Pulse(wrap)).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(0, 0, 16, 0).Children(func() {
			ui.Box(c).Size(44, 44).Radius(16).Background(bg)
			ui.Box(c).Size(112, 32).Radius(8).Background(bg)
		})
		ui.Box(c).Height(40).FillWidth().Radius(12).Background(bg).Margin(0, 0, 24, 0)
		ui.Grid(c).Columns(cols).GapX(card.CategoryGap).GapY(card.CategoryGap).Padding(8, 0, 0, 0).Children(func() {
			for range 8 {
				ui.Box(c).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Width(cardW).Children(func() {
						ui.Box(c).Height(cardW * 5 / 4).Radius(16).Background(bg)
						ui.Box(c).Height(20).Width(cardW*2/3).Margin(12, ui.Auto, 0, ui.Auto).Radius(6).Background(bg)
					})
				})
			}
		})
	})
}
