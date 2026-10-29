// Package homerail 对应 components/panel/HomeGameRailPanel.tsx：首页底部可展开的面板，有“最近游玩”的封面选择、
// 游戏库统计与年度热力图三个标签页。
package homerail

import (
	"log"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/chart"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// 标签页（HomeGameRailPanelTab）。
const (
	TabRecent  = "recent"
	TabStats   = "stats"
	TabHeatmap = "heatmap"
)

// 面板的尺寸：pt-5、h-48 的内容区、mt-3 与 SlideButton（p-1 + py-1.5 + 20 的行高 + py-1.5 + p-1）、pb-4。
const (
	contentH = 192
	Height   = 20 + contentH + 12 + 40 + 16
	cardW    = 144 // w-36
)

// spinStops 是选中封面的旋转描边：conic-gradient(from 0deg, #ef4444, #a855f7 90deg, #dc2626 180deg, #7e22ce 270deg, #ef4444)
// 与 opacity-95。
var spinStops = []ui.GradientStop{
	{Color: ui.Hex("#ef4444").Alpha(0.95), At: 0},
	{Color: ui.Hex("#a855f7").Alpha(0.95), At: 0.25},
	{Color: ui.Hex("#dc2626").Alpha(0.95), At: 0.5},
	{Color: ui.Hex("#7e22ce").Alpha(0.95), At: 0.75},
	{Color: ui.Hex("#ef4444").Alpha(0.95), At: 1},
}

// Rail 是面板自己的状态（activeTab、heatmapStats 等），首页持有，进入首页时 Reset。
type Rail struct {
	env *env.Env

	tab            string
	heatmap        *vo.PeriodStats
	heatmapLoading bool
	heatmapFailed  bool
	wantHeatmap    bool // 上一次构建时是否该加载热力图（原版 effect 的依赖），变为 true 时加载
	gen            int  // 每次 Reset 加一，丢弃之前还没返回的结果
	scroll         ui.ScrollState
}

// New 创建面板状态。
func New(e *env.Env) *Rail { return &Rail{env: e, tab: TabRecent} }

// Reset 对应组件重新挂载：回到“最近游玩”，丢掉热力图数据与滚动位置。
func (r *Rail) Reset() {
	r.gen++
	r.tab, r.heatmap, r.heatmapLoading, r.heatmapFailed, r.wantHeatmap = TabRecent, nil, false, false, false
	r.scroll = ui.ScrollState{}
}

// Tab 是当前的标签页（测试用）。
func (r *Rail) Tab() string { return r.tab }

// Props 是 HomeGameRailPanel 的属性。
type Props struct {
	Games            []models.Game
	Expanded         bool
	LibraryStats     *vo.PeriodStats
	SelectedID       string
	BlurNSFW         bool
	Covers           func(g *models.Game) (bmp *ui.Bitmap, missing bool)
	OnExpandedChange func(expanded bool)
	OnSelect         func(gameID string)
}

// loadHeatmap 对应 loadHeatmapStats：本年的全局统计（dimension year）。
func (r *Rail) loadHeatmap() {
	r.heatmapLoading, r.heatmapFailed = true, false
	gen := r.gen
	r.env.Async(func() func() {
		data, err := r.env.Svc.Stats.GetGlobalPeriodStats(vo.PeriodStatsRequest{Dimension: enums.Year})
		return func() {
			if gen != r.gen {
				return
			}
			r.heatmapLoading = false
			if err != nil {
				log.Printf("Failed to fetch home heatmap stats: %v", err)
				r.heatmapFailed = true
				return
			}
			r.heatmap = &data
		}
	})
}

// View 构建面板：absolute inset-x-0 bottom-0 z-20，收起时 translate-y-full（transition-transform duration-300 ease-out），
// 放在首页 relative 容器的最后。原版悬停时暂停轮播（onPauseChange），但面板展开时轮播本来就停了，所以这里不需要。
func (r *Rail) View(c *ui.Context, t *theme.Theme, p Props) {
	want := p.Expanded && r.tab == TabHeatmap && r.heatmap == nil
	if want && !r.wantHeatmap {
		r.loadHeatmap()
	}
	r.wantHeatmap = want

	outer := ui.Box(c.Key("home-rail")).Absolute().Left(0).Right(0).Height(Height)
	k := outer.AnimateWith("expand", theme.Bool(p.Expanded), 300*time.Millisecond, theme.EaseOut)
	// translate-y-full 用 bottom 偏移代替 Translate：变换下的裁剪（面板圆角、滚动区、每张封面的圆角）每帧都要在 CPU 上
	// 重新光栅化裁剪蒙版，展开、收起时一帧要几十毫秒；按布局移动的裁剪由着色器直接裁，与静止时一样快。
	outer.Bottom(-(1 - k) * Height).Children(func() {
		// pointer-events-none absolute inset-x-0 bottom-0 h-72 bg-gradient-to-t from-black/25 via-black/8 to-transparent
		// dark:from-black/40 dark:via-black/10。原版画在面板之下（面板是 relative，按文档顺序在它之后）；这里面板也绝对定位，按构建顺序画。
		from, via := t.Pick(theme.Black.Alpha(0.25), theme.Black.Alpha(0.4)), t.Pick(theme.Black.Alpha(0.08), theme.Black.Alpha(0.1))
		ui.Box(c.Key("shade")).Absolute().Left(0).Right(0).Bottom(0).Height(288).PassThrough().Children(func() {
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(144).
				LinearGradient(ui.LinearGradient{From: theme.Black.Alpha(0), To: via, Angle: 180, Oklab: true})
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(144).
				LinearGradient(ui.LinearGradient{From: via, To: from, Angle: 180, Oklab: true})
		})
		r.panel(c, t, p)
		r.toggle(c, t, p)
	})
}

// toggle 是展开/收起按钮：BetterEdgeIconButton placement="bottom"，absolute left-1/2 top-0 -translate-x-1/2 -translate-y-full，
// border-b-0 border-white/15 dark:border-white/8，bg-white/10 hover:bg-white/18 dark:bg-black/12 dark:hover:bg-black/20，backdrop-blur-[2px]。
func (r *Rail) toggle(c *ui.Context, t *theme.Theme, p Props) {
	icon, label := "chevron-up", locales.HomeExpandCoverPicker()
	if p.Expanded {
		icon, label = "chevron-down", locales.HomeCollapseCoverPicker()
	}
	ui.Row(c.Key("toggle")).Absolute().Top(-32).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
		btn := better.EdgeIconButton(c, t, "btn", better.EdgeIconButtonProps{
			Placement:    better.EdgeBottom,
			Icon:         icon,
			Label:        label,
			Blur:         toggleBlur(t),
			Surface:      t.Pick(theme.White.Alpha(0.1), theme.Black.Alpha(0.12)),
			SurfaceHover: t.Pick(theme.White.Alpha(0.18), theme.Black.Alpha(0.2)),
			OnClick: func() {
				if p.OnExpandedChange != nil {
					p.OnExpandedChange(!p.Expanded)
				}
			},
		})
		btn.BorderWidth(t.HL, t.HL, 0, t.HL).BorderColor(t.Pick(theme.White.Alpha(0.15), theme.White.Alpha(0.08)))
	})
}

// panel：relative overflow-hidden rounded-t-2xl bg-white/10 px-1 pb-4 pt-5 backdrop-blur-[2px] dark:bg-black/12，
// 收起时 opacity-0 pointer-events-none（transition-opacity duration-200）。里面 h-48 的标签页内容与 mt-3 居中的 SlideButton。
func (r *Rail) panel(c *ui.Context, t *theme.Theme, p Props) {
	pnl := ui.Column(c.Key("panel")).Absolute().Left(0).Right(0).Top(0).Bottom(0).Padding(20, 4, 16, 4).
		Radius(16, 16, 0, 0).Clip().Background(t.Pick(theme.White.Alpha(0.1), theme.Black.Alpha(0.12)))
	if !t.Glass {
		pnl.Material(glass.Blur{Radius: 2}) // 玻璃模式下 data-glass:backdrop-filter-off
	}
	k := pnl.AnimateWith("opacity", theme.Bool(p.Expanded), 200*time.Millisecond, theme.EaseTailwind)
	pnl.Opacity(k)
	if !p.Expanded && k == 0 {
		return // 完全收起后不构建内容（原版 aria-hidden、tabIndex=-1，内容不可交互）
	}
	pnl.Children(func() {
		ui.Box(c.Key("content")).Height(contentH).Shrink(0).Children(func() {
			switch r.tab {
			case TabStats:
				r.stats(c, t, p.LibraryStats)
			case TabHeatmap:
				r.heatmapTab(c, t)
			default:
				r.recent(c, t, p)
			}
		})
		ui.Row(c.Key("tabs")).Margin(12, 0, 0, 0).Justify(ui.Center).Children(func() {
			components.SlideButton(c, t, "rail-tabs", components.SlideButtonProps{
				Options: []components.SlideButtonOption{
					{Label: locales.HomeRecentPlayed(), Value: TabRecent, Icon: "history"},
					{Label: locales.StatsLibrarySectionTitle(), Value: TabStats, Icon: "library-shelves"},
					{Label: locales.StatsHeatmapTitle(), Value: TabHeatmap, Icon: "calendar-month-outline"},
				},
				Value:      r.tab,
				Disabled:   !p.Expanded,
				OnChange:   func(v string) { r.tab = v },
				Background: t.Pick(theme.White.Alpha(0.35), theme.Black.Alpha(0.3)),
			})
		})
	})
}

// recent 是“最近游玩”：横向滚动的封面（overflow-x-auto px-0.5 py-2，隐藏滚动条），放不下时 w-max 靠左，否则居中；
// 能向前/向后滚动时两侧显示 BetterEdgeIconButton，每次滚动 max(宽度 - 112, 280)。
func (r *Rail) recent(c *ui.Context, t *theme.Theme, p Props) {
	st := &r.scroll
	wrap := ui.Box(c.Key("recent")).Fill()
	viewW := wrap.Bounds().W
	wrap.Children(func() {
		// 滚动容器比 h-48 高出上下各 py-2，原版在 items-center 的行里上下各溢出 8px。
		ui.ScrollHorizontal(c.Key("scroller")).Absolute().Left(0).Right(0).Top(-8).Height(contentH + 16).TrackScroll(st).Children(func() {
			ui.Row(c).Padding(8, 2).Gap(4).AlignItems(ui.Center).MinWidth(max(0, viewW)).Justify(ui.Center).Children(func() {
				for _, g := range p.Games {
					r.card(c, t, p, g)
				}
			})
		})
		step := max(viewW-112, 280)
		if st.X > 2 {
			edge(c, t, "prev", better.EdgeLeft, "chevron-left", locales.HomeScrollRecentPrev(), func() { st.ScrollTo(st.X-step, 0, ui.Smooth) })
		}
		if st.X < st.MaxX-2 {
			edge(c, t, "next", better.EdgeRight, "chevron-right", locales.HomeScrollRecentNext(), func() { st.ScrollTo(st.X+step, 0, ui.Smooth) })
		}
	})
}

// edge 是 absolute top-1/2 -translate-y-1/2 z-10 的 BetterEdgeIconButton，贴在左侧或右侧。
func edge(c *ui.Context, t *theme.Theme, key string, placement better.EdgePlacement, icon, label string, onClick func()) {
	holder := ui.Column(c.Key(key)).Absolute().Top(0).Bottom(0).Justify(ui.Center).PassThrough()
	if placement == better.EdgeLeft {
		holder.Left(0)
	} else {
		holder.Right(0)
	}
	holder.Children(func() {
		better.EdgeIconButton(c, t, "btn", better.EdgeIconButtonProps{Placement: placement, Icon: icon, Label: label, OnClick: onClick})
	})
}

// card 是一张封面：h-48 w-36 shrink-0 rounded-xl border p-[2px]，hover:scale-[1.03]（transition 300ms）。选中时 border-transparent，
// 下面转动的彩色描边（animate-spin 3s）；其余 border-white/30 bg-white/30 opacity-75 dark:bg-black/20，悬停时上移 4px、
// 不透明、border-white/60。里面 rounded-[0.65rem] bg-brand-200 dark:bg-brand-800/70 的封面与 h-1/2 的底部渐变。
func (r *Rail) card(c *ui.Context, t *theme.Theme, p Props, g models.Game) {
	active := g.ID == p.SelectedID
	btn := ui.ButtonBase(c.Key(g.ID)).Size(cardW, contentH).Shrink(0).Radius(12).Padding(2).FocusRing(false).
		Label(locales.HomeSelectGame(g.Name))
	hovered := btn.Hovered()
	border, opacity, lift := theme.White.Alpha(0.3), float32(0.75), float32(0)
	switch {
	case active:
		border, opacity = theme.White.Alpha(0), 1
	case hovered:
		border, opacity, lift = theme.White.Alpha(0.6), 1, -4
		btn.Background(t.Pick(theme.White.Alpha(0.3), theme.Black.Alpha(0.2)))
	default:
		btn.Background(t.Pick(theme.White.Alpha(0.3), theme.Black.Alpha(0.2)))
	}
	ease := theme.EaseTailwind
	btn.Border(t.HL, btn.AnimateColor("border", border, theme.Tween(300*time.Millisecond)))
	btn.Opacity(btn.AnimateWith("opacity", opacity, 300*time.Millisecond, ease))
	sc := btn.AnimateWith("scale", 1+0.03*theme.Bool(hovered), 300*time.Millisecond, ease)
	btn.Scale(sc, sc).Translate(0, btn.AnimateWith("lift", lift, 300*time.Millisecond, ease))
	if active {
		// 原版是 22rem 见方、居中旋转的 conic-gradient（opacity-95 blur-[1px]），被按钮的圆角裁剪；
		// 转动整个方块等于转动渐变的起始角，所以这里直接在按钮的绘制层里画旋转的 conic 渐变（不模糊）。
		// 角度在绘制时按帧时间取，AnimationFrame 只重绘不重新构建界面。
		hl := t.HL
		btn.Draw(func(pt *ui.Painter, rc ui.Rect) {
			const period = 3 * time.Second // animate-spin，animationDuration: 3s
			angle := float32(pt.Now().UnixNano()%int64(period)) / float32(period) * 360
			in := ui.Rect{X: rc.X + hl, Y: rc.Y + hl, W: rc.W - 2*hl, H: rc.H - 2*hl}
			pt.FillConicGradient(in, ui.ConicGradient{Stops: spinStops, Angle: angle}, 12-hl)
			pt.AnimationFrame()
		})
	}
	if p.OnSelect != nil {
		id := g.ID
		btn.OnClick(func() { p.OnSelect(id) })
	}
	btn.Children(func() {
		const radius = 10.4 // rounded-[0.65rem]
		// overflow-hidden 不用 Clip，封面与底部渐变各自按圆角画：按钮悬停时缩放、上移，变换下的圆角裁剪要在 CPU 上
		// 光栅化蒙版，动画的每一帧都重建一张，不用的还会留在内存里（最多 32MB）。
		ui.Box(c).Fill().Radius(radius).Background(t.Pick(theme.Brand200, theme.Brand800.Alpha(0.7))).Children(func() {
			var bmp *ui.Bitmap
			missing := true
			if p.Covers != nil {
				bmp, missing = p.Covers(&g)
			}
			switch {
			case bmp != nil:
				ui.Image(c, bmp).Fill().Fit(ui.Cover).Radius(radius)
			case missing:
				ui.Box(c).Fill().Center().Children(func() {
					components.Icon(c, "image-off", 30, t.Pick(theme.Brand400, theme.White.Alpha(0.5)))
				})
			}
			// absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/60 to-transparent
			ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).HeightPercent(50).Radius(0, 0, radius, radius).PassThrough().
				LinearGradient(ui.LinearGradient{From: theme.Black.Alpha(0), To: theme.Black.Alpha(0.6), Angle: 180, Oklab: true})
		})
	})
}

// stats 是“游戏库统计”：grid h-full grid-cols-4 items-center gap-4 px-8 py-2，每项 flex-col justify-center gap-3 px-4：
// text-3xl 的图标（text-brand-500/80 dark:text-brand-300/80）、text-2xl font-bold leading-tight 的数值与 mt-1 text-sm 的说明。
func (r *Rail) stats(c *ui.Context, t *theme.Theme, s *vo.PeriodStats) {
	value := func(f func(s *vo.PeriodStats) string) string {
		if s == nil {
			return "--"
		}
		return f(s)
	}
	items := []struct{ value, label, icon string }{
		{value(func(s *vo.PeriodStats) string { return strconv.Itoa(s.AllSessionsCount) }), locales.StatsLibraryTotalSessions(), "counter"},
		{value(func(s *vo.PeriodStats) string { return timefmt.Duration(s.AllSessionsDuration) }), locales.StatsLibraryTotalDuration(), "timer-outline"},
		{value(func(s *vo.PeriodStats) string { return strconv.Itoa(s.LibraryGamesCount) }), locales.StatsLibraryTotalGames(), "gamepad-square-outline"},
		{value(func(s *vo.PeriodStats) string { return strconv.Itoa(s.AllCompletedGamesCount) }), locales.StatsLibraryCompletedGames(), "trophy-outline"},
	}
	ui.Row(c.Key("stats")).Fill().Padding(8, 32).Gap(16).AlignItems(ui.Center).Children(func() {
		for _, it := range items {
			ui.Column(c.Key(it.icon)).Grow(1).Basis(0).MinWidth(0).FillHeight().Justify(ui.Center).Gap(12).Padding(0, 16).Children(func() {
				components.Icon(c, it.icon, 30, t.Pick(theme.Brand500.Alpha(0.8), theme.Brand300.Alpha(0.8)))
				ui.Column(c).Children(func() {
					components.Text(c, it.value, theme.FontStep{Size: 24, Line: 30}, theme.WeightBold, t.TextStrong)
					components.TextMargin(c, it.label, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.White.Alpha(0.7)), 4, 0, 0, 0)
				})
			})
		}
	})
}

// heatmapTab 是“热力图”：flex h-full items-center justify-center px-8 py-1；加载失败时是重试按钮，加载中是旋转图标与“加载中”，
// 之后是 PlayHeatmap。
func (r *Rail) heatmapTab(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("heatmap")).Fill().Padding(4, 32).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		switch {
		case r.heatmapFailed:
			r.retryButton(c, t)
		case r.heatmapLoading || r.heatmap == nil:
			color := t.Pick(theme.Brand600, theme.White.Alpha(0.7))
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Spinner(c, 18, color)
				components.Text(c, locales.CommonLoading(), theme.TextSM, theme.WeightNormal, color)
			})
		default:
			chart.PlayHeatmap(c, t, chart.HeatmapProps{Cells: r.heatmap.Heatmap}).FillWidth()
		}
	})
}

// retryButton：mx-auto flex items-center gap-2 rounded-lg border border-brand-300/70 bg-white/20 px-4 py-2 text-sm font-medium
// text-brand-700 hover:bg-white/35 dark:border-white/15 dark:bg-black/20 dark:text-white/80 dark:hover:bg-black/30，i-mdi-refresh。
func (r *Rail) retryButton(c *ui.Context, t *theme.Theme) {
	label := locales.StatsToastLoadStatsFailed()
	btn := ui.ButtonBase(c.Key("retry")).Row().Gap(8).AlignItems(ui.Center).Padding(8, 16).Radius(8).FocusRing(false).Label(label).
		Border(t.HL, t.Pick(theme.Brand300.Alpha(0.7), theme.White.Alpha(0.15)))
	// data-glass:bg-white/10 data-glass:hover:bg-white/18（选择器比 dark: 更具体，暗色也用它）
	bg := t.IfGlass(t.Pick(theme.White.Alpha(0.2), theme.Black.Alpha(0.2)), theme.White.Alpha(0.1), theme.White.Alpha(0.1))
	if btn.Hovered() {
		bg = t.IfGlass(t.Pick(theme.White.Alpha(0.35), theme.Black.Alpha(0.3)), theme.White.Alpha(0.18), theme.White.Alpha(0.18))
	}
	color := t.Pick(theme.Brand700, theme.White.Alpha(0.8))
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(r.loadHeatmap).Children(func() {
		components.Icon(c, "refresh", 16, color)
		components.Text(c, label, theme.TextSM, theme.WeightMedium, color).NoWrap()
	})
}

// toggleBlur 是展开按钮的 backdrop-blur-[2px]；玻璃模式下 data-glass:backdrop-filter-off。
func toggleBlur(t *theme.Theme) float32 {
	if t.Glass {
		return -1
	}
	return 2
}
