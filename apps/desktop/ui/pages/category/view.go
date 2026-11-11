package category

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/bar"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/card"
	"lunabox/desktop/ui/components/grid"
	"lunabox/desktop/ui/components/panel/launch"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

const (
	pagePadding  = 32  // p-8
	scrollTopMin = 225 // usePageScrollControls：工具栏滚出视口后显示“回到顶部”
)

var textLG = theme.FontStep{Size: 18, Line: 28} // text-lg

func itoa(n int) string { return strconv.Itoa(n) }

// View 构建页面：h-full w-full overflow-y-auto scrollbar-stable p-8，返回、页头、FilterBar 与游戏网格；
// 弹窗与筛选抽屉在最后构建。
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
	switch {
	case p.loading && p.category == nil:
		if wait := skeletonDelay - c.Now().Sub(p.openedAt); wait > 0 {
			c.After(wait)
			return
		}
		p.skeleton(c, t)
		return
	case p.category == nil:
		p.notFound(c, t)
		return
	}
	p.filters.Debounce(c, p.reload)
	p.addGames.debounce(c, p)
	fp := p.filterProps(c, t)

	_, winH := c.Size()
	gutter := t.Gutter()
	contentW := p.env.MainWidth - 2*pagePadding - gutter
	p.geometry = grid.NewGeometry(contentW, p.landscape)
	components.ScrollArea(c, t, "category-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding+bar.BatchIslandInset(p.filters.Batch), pagePadding)
		sc.Children(func() {
			ui.Column(c).Width(contentW).Children(func() {
				p.backButton(c, t)
				// flex flex-col gap-6：页头与 FilterBar
				ui.Column(c).Gap(24).Children(func() {
					p.header(c, t)
					bar.FilterBar(c, t, fp)
				})
				ui.Column(c).Margin(24, 0, 0, 0).Children(func() { p.content(c, t, winH) })
			})
		})
	})

	components.ScrollToTopButton(c, t, locales.CommonBackToTop(), p.scroll.State.Y > scrollTopMin, func() {
		p.scroll.State.ScrollTo(0, 0, ui.Animation{Duration: 400 * time.Millisecond, Ease: theme.EaseCSS})
	})
	bar.BatchIsland(c, t, fp)
	bar.FilterDrawer(c, t, fp)
	p.addGamesView(c, t)
	components.CategoryModal(c, t, "rename-category", components.CategoryModalProps{
		Open:      p.renameOpen,
		Value:     &p.renameValue,
		Edit:      true,
		TopOffset: env.TopbarHeight,
		OnClose:   func() { p.renameOpen, p.renameValue = false, "" },
		OnSubmit:  p.rename,
	})
	components.ConfirmModal(c, t, "delete-category", components.ConfirmProps{
		Open:      &p.deleteOpen,
		Title:     locales.CategoriesToastDeleteTitle(),
		Message:   locales.CategoriesToastDeleteMsg(p.category.Name),
		Danger:    true,
		TopOffset: env.TopbarHeight,
		OnConfirm: p.deleteCategory,
	})
}

// filterProps 是 category.tsx 传给 FilterBar 的属性：与游戏库相同的筛选与排序，批量移除与“添加游戏”。
func (p *Page) filterProps(c *ui.Context, t *theme.Theme) bar.FilterBarProps {
	f := &p.filters
	return bar.FilterBarProps{
		Env:               p.env,
		Filters:           f,
		SearchPlaceholder: locales.LibrarySearchPlaceholder(),
		SortOptions:       bar.GameSortOptions(),
		DefaultSortBy:     string(enums.GameListSortByCreatedAt),
		DefaultSortOrder:  enums.SortOrderDesc,
		SecondarySort:     true,
		GameFilters:       true,
		OnChange:          p.reload,
		Random: &bar.RandomGame{
			Disabled: p.pages.Loading || f.Settling() || p.pages.LoadedKey != p.pages.Key || p.pages.Total == 0,
			Loading:  p.randomLoading,
			OnClick:  p.openRandomGame,
		},
		OnBatchChange: func(on bool) {
			f.Batch = on
			if !on {
				clear(p.selected)
			}
		},
		SelectedCount: len(p.selected),
		OnSelectAll: func() {
			for _, g := range p.pages.Games {
				p.selected[g.ID] = true
			}
		},
		OnClearSelection: func() { clear(p.selected) },
		BatchActions:     func() { p.batchRemoveButton(c, t) },
		ActionButton:     func() { p.addGameButton(c, t) },
	}
}

// backButton：flex rounded-md items-center text-brand-600 hover:text-brand-900 mb-6（dark：brand-400 / brand-200），
// i-mdi-arrow-left text-2xl mr-1。回到收藏夹列表。
func (p *Page) backButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("back")).AlignSelf(ui.Start).AlignItems(ui.Center).Radius(6).Margin(0, 0, 24, 0).FocusRing(false)
	color := t.Pick(theme.Brand600, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand900, theme.Brand200)
	}
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	btn.OnClick(func() { p.env.Router.Push(env.PathCategories) }).Children(func() {
		components.Icon(c, "arrow-left", 24, color).Margin(0, 4, 0, 0)
		components.Text(c, locales.CategoryBack(), theme.TextBase, theme.WeightNormal, color).NoWrap()
	})
}

// header：flex items-start justify-between gap-6。左侧图标（系统收藏夹是表情，其余是 EmojiPickerPopover）、
// text-4xl font-bold 的名称与“系统”标签，下面 mt-2 的游戏数量；右侧是重命名 / 删除的菜单。
func (p *Page) header(c *ui.Context, t *theme.Theme) {
	cat := p.category
	ui.Row(c.Key("header")).Gap(24).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).MinWidth(0).Children(func() {
				if cat.IsSystem {
					if e := strings.TrimSpace(cat.Emoji); e != "" {
						components.EmojiText(c, e, 30).Shrink(0) // text-3xl leading-none
					}
				} else {
					components.EmojiPicker(c, t, "emoji", components.EmojiPickerProps{
						Value: cat.Emoji, CanEdit: true, Compact: true, FallbackIcon: "folder",
						OnChange: p.setEmoji,
					})
				}
				// h1 flex min-w-0 flex-wrap items-center gap-3
				ui.Row(c).Wrap().GapX(12).GapY(12).AlignItems(ui.Center).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, cat.Name, theme.Text4XL, theme.WeightBold, t.TextStrong).MinWidth(0)
					if cat.IsSystem {
						// shrink-0 rounded-md bg-neutral-100 px-2 py-1 text-sm text-neutral-800（dark：bg-neutral-900 text-neutral-300）
						ui.Box(c).Padding(4, 8).Radius(6).Shrink(0).Background(t.Pick(theme.Neutral100, ui.Hex("#0f172a"))).Children(func() {
							components.Text(c, locales.CategorySystemTag(), theme.TextSM, theme.WeightNormal, t.Pick(ui.Hex("#1e293b"), theme.Neutral300)).NoWrap()
						})
					}
				})
			})
			components.TextMargin(c, p.filters.CountText(p.pages.Total), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 8, 0, 0, 0)
		})
		if !cat.IsSystem {
			p.actionsMenu(c, t)
		}
	})
}

// actionsMenu：h-11 w-11 rounded-full border bg-white 的 i-mdi-dots-horizontal 触发按钮（BetterDropdownMenu align="end"
// menuWidth="min-w-[140px]"），菜单是重命名与删除。玻璃模式下 data-glass:border-transparent bg-white/10
// dark:bg-black/12 text-brand-900 dark:text-white backdrop-blur-12（悬停不变；backdrop-saturate-180 没有还原）。
func (p *Page) actionsMenu(c *ui.Context, t *theme.Theme) {
	trigger := ui.ButtonBase(c.Key("actions")).Size(44, 44).Radius(999).Center().Shrink(0).FocusRing(false).Label(locales.CommonAction())
	if t.Glass {
		fg := t.Pick(theme.Brand900, theme.White)
		trigger.Material(glass.Blur{Radius: 12}).Children(func() {
			ui.Box(c).Fill().Radius(999).Center().Background(t.Pick(theme.White.Alpha(0.1), theme.Black.Alpha(0.12))).Children(func() {
				components.Icon(c, "dots-horizontal", 24, fg)
			})
		})
	} else {
		bg, fg := t.Pick(theme.White, theme.Brand800), t.Pick(theme.Brand500, theme.Brand400)
		if trigger.Hovered() {
			bg, fg = t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand900, theme.White)
		}
		trigger.Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(bg).
			Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
			components.Icon(c, "dots-horizontal", 24, fg)
		})
	}
	better.DropdownMenu(c, t, trigger, better.MenuProps{
		MinWidth: 140,
		Items: []better.MenuItem{
			{Key: "rename", Label: locales.CategoriesRename(), Icon: "pencil", OnClick: func() {
				name := []rune(p.category.Name)
				p.renameValue = string(name[:min(len(name), components.CategoryNameMaxLength)])
				p.renameOpen = true
			}},
			{Key: "delete", Label: locales.CommonDelete(), Icon: "delete", IconColor: t.Pick(theme.Error500, theme.Error400), DividerBefore: true,
				OnClick: func() { p.deleteOpen = true }},
		},
	})
}

// batchRemoveButton 是 batchActions：批量操作浮动岛里的“移除选中”（原版是 glass-panel 的红色按钮），没有选中时禁用。
func (p *Page) batchRemoveButton(c *ui.Context, t *theme.Theme) {
	bar.BatchIslandButton(c, t, "batch-remove", bar.BatchButton{Icon: "delete", Label: locales.CategoryBatchRemoveBtn(),
		ShowText: true, Danger: true, Disabled: len(p.selected) == 0, OnClick: p.batchRemove})
}

// addGameButton 是 actionButton：glass-btn-neutral rounded-lg bg-neutral-600 px-4 py-2 text-sm font-medium text-white。
func (p *Page) addGameButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("add-game")).Padding(8, 16).Radius(8).AlignItems(ui.Center).FocusRing(false).Label(locales.CategoryAddGameBtn())
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: t.PrimaryBtn, Hover: t.PrimaryBtnHover, Text: theme.White})
	bg := g.Bg
	if btn.Hovered() {
		bg = g.Hover
	}
	if t.Glass {
		btn.Border(t.HL, g.Border)
	}
	btn.Background(bg).OnClick(p.openAddGames).Children(func() {
		components.Icon(c, "plus", 18, g.Text).Margin(0, 8, 0, 0)
		components.Text(c, locales.CategoryAddGameBtn(), theme.TextSM, theme.WeightMedium, g.Text).NoWrap()
	})
}

// content 是 mt-6 的列表区：等待第一页、游戏网格、没有匹配与空收藏夹四种状态。
func (p *Page) content(c *ui.Context, t *theme.Theme, winH float32) {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	switch {
	case p.pages.Waiting(p.filters.Settling()):
		ui.Column(c.Key("waiting")).Height(256).Center().Children(func() {
			components.Spinner(c, 60, muted).Margin(0, 0, 16, 0)
			components.Text(c, locales.CommonLoading(), textLG, theme.WeightNormal, muted)
		})
	case p.pages.Total > 0:
		ui.Box(c).Children(func() {
			wrap := ui.Box(c.Key("grid-wrap"))
			if p.pages.Loading {
				wrap.Opacity(0.6).PassThrough()
			}
			p.drag.Surface(c, wrap, grid.DragProps{
				Enabled:        p.filters.Batch,
				Selected:       func(id string) bool { return p.selected[id] },
				OnSelectChange: p.setSelected,
				Scroll:         &p.scroll.State,
			})
			wrap.Children(func() { p.gameGrid(c, t, winH) })
			if p.pages.Loading {
				loadingPill(c, t)
			}
		})
	case p.filters.HasActiveFilters():
		ui.Column(c.Key("no-match")).Height(256).Center().Children(func() {
			components.Icon(c, "magnify", 60, muted).Margin(0, 0, 16, 0)
			components.Text(c, locales.CategoryNoMatchingGames(), textLG, theme.WeightNormal, muted)
		})
	default:
		ui.Column(c.Key("empty")).Height(256).Center().Children(func() {
			components.Icon(c, "gamepad-variant-outline", 60, muted).Margin(0, 0, 16, 0)
			components.Text(c, locales.CategoryEmptyCategory(), textLG, theme.WeightNormal, muted)
			// mt-4 text-neutral-600 hover:underline dark:text-neutral-400
			btn := ui.ButtonBase(c.Key("add-first")).Margin(16, 0, 0, 0).FocusRing(false)
			btn.OnClick(p.openAddGames).Children(func() {
				text := components.Text(c, locales.CategoryAddFirstGame(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Neutral600, theme.Neutral400))
				if btn.Hovered() {
					text.Underline()
				}
			})
		})
	}
}

func (p *Page) gameGrid(c *ui.Context, t *theme.Theme, winH float32) {
	sortField := enums.GameListSortBy("")
	if p.filters.ShowSortField {
		sortField = enums.GameListSortBy(p.filters.SortBy)
	}
	launcher := launch.For(p.env)
	grid.VirtualGrid(c, grid.Props{
		Geometry:   p.geometry,
		Total:      p.pages.Total,
		ViewTop:    env.TopbarHeight,
		ViewBottom: winH,
		OnRange:    p.pages.Ensure,
		Item: func(c *ui.Context, i int) {
			g := p.pages.Games[i]
			if g == nil {
				card.Placeholder(c.Key(fmt.Sprintf("placeholder-%d", i)), t, p.geometry.CardW, p.landscape)
				return
			}
			running, ending := launcher.Status(g.ID)
			props := card.GameCardProps{
				Game:           g,
				Width:          p.geometry.CardW,
				Landscape:      p.landscape,
				Cover:          p.env.Covers.Get(g),
				NoCover:        p.env.Covers.Missing(g),
				SortField:      sortField,
				BlurNSFW:       p.blurNSFW,
				Highlight:      p.filters.Debounced,
				Batch:          p.filters.Batch,
				Selected:       p.selected[g.ID],
				OnToggleSelect: func() { p.toggleSelected(g.ID) },
				Running:        running,
				Ending:         ending,
				OnPlay:         func() { launcher.StartFromCard(c, *g) },
				OnDetail:       func() { p.env.Router.Push(env.GamePath(g.ID)) },
			}
			if !p.filters.Batch {
				props.Overlay = func(hover float32) { removeButton(c, t, hover, func() { p.removeGame(g.ID) }) }
			}
			p.drag.Target(c.Key(g.ID), g.ID, func(c *ui.Context) { card.GameCard(c, t, props) })
		},
	})
}

// removeButton 是 renderOverlay：absolute top-2 right-2 p-1 bg-error-500 text-white rounded-full shadow-md，
// 悬停卡片时显示（opacity-0 group-hover:opacity-100），悬停按钮时 bg-error-600；i-mdi-close text-sm。
func removeButton(c *ui.Context, t *theme.Theme, hover float32, onClick func()) {
	btn := ui.ButtonBase(c.Key("remove")).Absolute().Top(8-t.HL).Right(8-t.HL).Padding(4).Radius(999).FocusRing(false).
		Label(locales.NativeRemoveFromCategory()).Opacity(hover).
		Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
	bg := theme.Error500
	if btn.Hovered() {
		bg = theme.Error600
	}
	btn.Background(bg).OnClick(onClick).Children(func() { components.Icon(c, "close", 14, theme.White) })
}

// loadingPill：加载中提示 absolute inset-x-0 top-0 flex justify-center py-3；pill 是 glass-panel。
func loadingPill(c *ui.Context, t *theme.Theme) {
	ui.Row(c).Absolute().Top(12).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
		bg, border := t.GlassPanel(t.Pick(theme.White.Alpha(0.85), theme.Brand900.Alpha(0.75)), t.Pick(theme.Brand200.Alpha(0.7), theme.Brand700.Alpha(0.7)))
		pill := ui.Row(c).Padding(6, 12).Radius(999).Gap(8).AlignItems(ui.Center).Background(bg).Border(t.HL, border)
		if !t.Glass {
			pill.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
		}
		pill.Children(func() {
			color := t.Pick(theme.Brand600, theme.Brand300)
			components.Spinner(c, 14, color)
			components.Text(c, locales.CommonLoading(), theme.TextSM, theme.WeightNormal, color)
		})
	})
}

// notFound：flex-col items-center justify-center space-y-4 text-brand-500，i-mdi-alert-circle-outline text-6xl，
// text-xl 的“未找到”与 text-neutral-600 hover:underline 的“返回列表”。
func (p *Page) notFound(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("not-found")).Fill().Center().Gap(16).Children(func() {
		components.Icon(c, "alert-circle-outline", 60, theme.Brand500)
		components.Text(c, locales.CategoryNotFound(), theme.TextXL, theme.WeightNormal, theme.Brand500)
		btn := ui.ButtonBase(c.Key("back-to-list")).FocusRing(false)
		btn.OnClick(func() { p.env.Router.Push(env.PathCategories) }).Children(func() {
			text := components.Text(c, locales.CategoryBackToList(), theme.TextBase, theme.WeightNormal, theme.Neutral600)
			if btn.Hovered() {
				text.Underline()
			}
		})
	})
}

// skeleton 对应 CategorySkeleton（animate-pulse），玻璃模式下各块是 data-glass:bg-white/2 dark:bg-black/2。
func (p *Page) skeleton(c *ui.Context, t *theme.Theme) {
	bg := t.IfGlass(t.Pick(theme.Brand200, theme.Brand800), theme.White.Alpha(0.02), theme.Black.Alpha(0.02))
	winW, _ := c.Size()
	gutter := t.Gutter()
	width := p.env.MainWidth - 2*pagePadding - gutter
	// mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8
	cols := 2
	for _, bp := range []struct {
		w    float32
		cols int
	}{{640, 3}, {768, 4}, {1024, 5}, {1280, 6}, {1536, 8}} {
		if winW >= bp.w {
			cols = bp.cols
		}
	}
	cellW := (width - 16*float32(cols-1)) / float32(cols)
	wrap := ui.Column(c.Key("skeleton")).Fill().Clip().Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding).PassThrough()
	wrap.Opacity(components.Pulse(wrap)).Children(func() {
		ui.Box(c).Size(96, 32).Radius(6).Background(bg).Margin(0, 0, 24, 0)
		ui.Column(c).Gap(24).Children(func() {
			ui.Column(c).Children(func() {
				ui.Box(c).Size(256, 48).Radius(8).Background(bg).Margin(0, 0, 8, 0)
				ui.Box(c).Size(128, 24).Radius(6).Background(bg)
			})
			ui.Box(c).Height(48).FillWidth().Radius(8).Background(bg)
		})
		ui.Grid(c).Columns(cols).GapX(16).GapY(16).Margin(24, 0, 0, 0).Children(func() {
			for range 12 {
				ui.Box(c).Height(cellW * 4 / 3).Radius(8).Background(bg)
			}
		})
	})
}
