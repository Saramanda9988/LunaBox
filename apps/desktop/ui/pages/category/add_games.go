package category

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// addGamesModal 是 AddGameToCategoryModal 的状态：不在收藏夹里的游戏，按名称搜索，分页“加载更多”。
type addGamesModal struct {
	open      bool
	query     string
	debounced string
	editedAt  time.Time
	games     []models.Game
	hasMore   bool
	loading   bool
	req       int // candidateRequestIdRef：只采用最后一次请求的结果
	scroll    components.ScrollBar
}

// openAddGames 对应 openAddGameModal：清空候选列表后打开，并加载第一页。
func (p *Page) openAddGames() {
	p.addGames = addGamesModal{open: true}
	p.loadCandidates(0, false)
}

// debounce 在搜索词停止变化 250ms 后重新加载候选（debouncedCandidateSearchQuery）。
func (m *addGamesModal) debounce(c *ui.Context, p *Page) {
	if !m.open || m.query == m.debounced {
		return
	}
	if wait := bar.SearchDebounce - c.Now().Sub(m.editedAt); wait > 0 {
		c.After(wait)
		return
	}
	m.debounced = m.query
	p.loadCandidates(0, false)
}

// loadCandidates 对应 loadCandidates：SearchCategoryGameCandidates，append 时接在已有结果后面。
func (p *Page) loadCandidates(offset int, appendMore bool) {
	if p.category == nil {
		return
	}
	m := &p.addGames
	m.req++
	m.loading = true
	id, gen := m.req, p.gen
	req := vo.CategoryGameCandidateRequest{
		CategoryID:  p.category.ID,
		Limit:       candidatePageSize,
		Offset:      offset,
		SearchQuery: strings.TrimSpace(m.debounced),
	}
	p.env.Async(func() func() {
		resp, err := p.env.Svc.Category.SearchCategoryGameCandidates(req)
		return func() {
			if gen != p.gen || id != m.req {
				return
			}
			m.loading = false
			if err != nil {
				p.failed = locales.CategoryToastLoadAllGamesFailed
				return
			}
			m.hasMore = resp.HasMore
			if appendMore {
				m.games = append(m.games, resp.Games...)
			} else {
				m.games = resp.Games
			}
		}
	})
}

// addGame 对应 handleAddGameToCategory：加入后从候选里去掉它，刷新收藏夹。
func (p *Page) addGame(gameID string) {
	if p.category == nil {
		return
	}
	id := p.category.ID
	p.env.Async(func() func() {
		err := p.env.Svc.Category.AddGameToCategory(gameID, id)
		return func() {
			if err != nil {
				p.failed = locales.CategoryToastAddGameFailed
				return
			}
			m := &p.addGames
			m.games = slices.DeleteFunc(m.games, func(g models.Game) bool { return g.ID == gameID })
			p.changed()
		}
	})
}

// addGamesView 对应 AddGameToCategoryModal：w-full max-w-2xl h-[80vh] rounded-xl bg-white flex-col shadow-xl
// （dark:bg-brand-800）。标题栏 p-6 border-b，搜索框 px-6 pt-4，下面 flex-1 overflow-y-auto p-6 的游戏网格。
func (p *Page) addGamesView(c *ui.Context, t *theme.Theme) {
	m := &p.addGames
	winW, winH := c.Size()
	closeModal := func() { m.open = false }
	components.Modal(c, t, "add-games", components.ModalProps{Open: m.open, TopOffset: env.TopbarHeight, MaxWidth: 672},
		func(panel ui.Element) {
			panel.Height((winH-env.TopbarHeight)*0.8).Radius(12).Clip().Background(t.PanelBg).
				Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
			panel.Children(func() {
				// p-6 border-b flex justify-between items-center
				ui.Row(c).Padding(24).AlignItems(ui.Center).Justify(ui.SpaceBetween).
					BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Children(func() {
					components.Text(c, locales.CategoryAddGameModalTitle(), theme.TextXL, theme.WeightBold, t.TextStrong)
					btn := ui.ButtonBase(c.Key("close")).FocusRing(false).Label(locales.CommonCancel())
					color := t.Pick(theme.Brand500, theme.Brand400)
					if btn.Hovered() {
						color = t.Pick(theme.Brand700, theme.White)
					}
					btn.OnClick(closeModal).Children(func() { components.Icon(c, "close", 20, color) })
				})
				p.candidateSearch(c, t)
				gutter := float32(0)
				if m.scroll.Overflowing() {
					gutter = t.Gutter()
				}
				components.ScrollArea(c, t, "candidates", &m.scroll, func(w ui.Element) { w.Grow(1).Basis(0).MinHeight(0) }, func(sc ui.Element) {
					sc.Padding(24, 24+gutter, 24, 24).Children(func() {
						p.candidateGrid(c, t, winW)
					})
				})
			})
		})
}

// candidateSearch：relative 的搜索框，i-mdi-magnify 在 left-3；input rounded-lg border border-brand-200 bg-white
// py-2 pl-10 pr-3 text-sm（dark：border-brand-700 bg-brand-900），focus:ring-2 focus:ring-primary-500。
func (p *Page) candidateSearch(c *ui.Context, t *theme.Theme) {
	m := &p.addGames
	ui.Box(c.Key("candidate-search")).Padding(16, 24, 0, 24).Children(func() {
		in := ui.TextInputBase(c.Key("input"), &m.query).FillWidth().Padding(8, 12, 8, 40).Radius(8).
			FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
			Background(t.Pick(theme.White, theme.Brand900)).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
			Placeholder(locales.LibrarySearchPlaceholder()).Label(locales.LibrarySearchPlaceholder())
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		if in.Changed() {
			m.editedAt = c.Now()
		}
		if in.Focused() {
			in.Shadow(0, 0, 0, 2, theme.Primary500)
		}
		ui.Box(c).Absolute().Left(24 + 12).Top(16).Bottom(0).Center().PassThrough().Children(func() {
			components.Icon(c, "magnify", 16, theme.Brand400)
		})
	})
}

// candidateGrid：grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4 的游戏，下面是“加载更多”；没有候选时居中提示。
func (p *Page) candidateGrid(c *ui.Context, t *theme.Theme, winW float32) {
	m := &p.addGames
	if len(m.games) == 0 {
		msg := locales.CategoryAddGameModalNoGamesAvailable()
		if m.loading {
			msg = locales.CommonLoading()
		}
		ui.Column(c).FillWidth().Padding(120, 0).Center().Children(func() {
			components.Text(c, msg, theme.TextBase, theme.WeightNormal, theme.Brand500)
		})
		return
	}
	cols := 2
	switch {
	case winW >= 768:
		cols = 4
	case winW >= 640:
		cols = 3
	}
	// 面板 max-w-2xl（遮罩 p-4），p-6，有滚动条时再减去滚动条槽
	inner := min(672, winW-32) - 48
	if m.scroll.Overflowing() {
		inner -= t.Gutter()
	}
	coverH := ((inner-16*float32(cols-1))/float32(cols) - 16) * 4 / 3
	blur := p.blurNSFW
	ui.Grid(c.Key("candidate-grid")).Columns(cols).GapX(16).GapY(16).Children(func() {
		for i := range m.games {
			g := &m.games[i]
			candidate(c.Key(g.ID), t, p.env, g, coverH, blur, func() { p.addGame(g.ID) })
		}
	})
	if m.hasMore {
		// mt-4 flex justify-center；glass-btn-neutral rounded-lg px-4 py-2 text-sm，disabled:opacity-50
		ui.Row(c).Margin(16, 0, 0, 0).Justify(ui.Center).Children(func() {
			label := locales.CommonLoadMore()
			if m.loading {
				label = locales.CommonLoading()
			}
			btn := ui.ButtonBase(c.Key("load-more")).Padding(8, 16).Radius(8).FocusRing(false).Label(label)
			if m.loading {
				components.Disable(btn, 0.5)
			} else {
				btn.OnClick(func() { p.loadCandidates(len(m.games), true) })
			}
			btn.Children(func() {
				components.Text(c, label, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand200)).NoWrap()
			})
		})
	}
}

// candidate：flex flex-col items-center p-2 rounded-lg hover:bg-brand-100 的按钮，aspect-[3/4] rounded-lg 的封面
// （悬停时 bg-black/40 加 i-mdi-plus），下面 text-sm font-medium line-clamp-2 的名称。
func candidate(c *ui.Context, t *theme.Theme, e *env.Env, g *models.Game, coverH float32, blur bool, onAdd func()) {
	btn := ui.ButtonBase(c).Column().Padding(8).Radius(8).AlignItems(ui.Stretch).FocusRing(false).Label(g.Name)
	hovered := btn.Hovered()
	if hovered {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
	}
	overlay := btn.AnimateWith("overlay", theme.Bool(hovered), 150*time.Millisecond, theme.EaseTailwind)
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onAdd).Children(func() {
		cover := ui.Box(c.Key("cover")).FillWidth().Height(coverH).Radius(8).Clip().Background(t.Pick(theme.Brand200, theme.Brand700)).Margin(0, 0, 8, 0)
		cover.Children(func() {
			if bmp := e.Covers.Get(g); bmp != nil {
				img := ui.Image(c, bmp).Fill().Fit(ui.Cover)
				if g.IsNSFW && blur {
					components.CoverBlur(c, btn, img, false)
				}
			} else if g.CoverURL == "" && g.CoverSourceURL == "" || e.Covers.Missing(g) {
				ui.Box(c).Fill().Center().Children(func() { components.Icon(c, "image-off", 24, theme.Brand400) })
			}
			ov := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Background(ui.RGBA(0, 0, 0, 0.4)).Opacity(overlay).PassThrough()
			ov.Children(func() { components.Icon(c, "plus", 30, theme.White) })
		})
		components.Text(c, g.Name, theme.TextSM, theme.WeightMedium, t.TextStrong).MaxLines(2)
	})
}
