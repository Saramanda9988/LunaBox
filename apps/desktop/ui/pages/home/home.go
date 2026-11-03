// Package home 是首页，对应原版 routes/index.tsx：最近游玩的游戏轮播（模糊的封面背景、封面大图、上次游玩时间与
// 总时长）、继续游戏，以及底部可展开的最近游玩、游戏库统计与热力图面板（HomeGameRailPanel）。
package home

import (
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components/panel/homerail"
	"lunabox/desktop-mygo/ui/components/panel/launch"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// 让首页刷新的服务事件（hooks/useAppRuntimeEffects.ts）。
const (
	refreshEvent     = "home:refresh-requested"
	runtimeEvent     = "game-runtime:changed"
	windowShownEvent = "app:main-window-shown" // bootstrap.EventMainWindowShown
)

// routes/index.tsx 的常量。
const (
	backgroundCrossfade = 1200 * time.Millisecond // BACKGROUND_CROSSFADE_MS
	heroFadeOut         = 280 * time.Millisecond  // HERO_FADE_OUT_MS
	heroFadeInDelay     = 90 * time.Millisecond   // HERO_FADE_IN_DELAY_MS
	backgroundBlur      = 16                      // HOME_BACKGROUND_BLUR_PX
)

// heroSnap 对应 HeroSnapshot：页面左下角显示的游戏。
type heroSnap struct {
	game       models.Game
	playing    bool
	lastPlayed time.Time
	total      int // 总游玩时长（秒）
}

// heroState 对应 useSnapshotVisibilityTransition：换游戏时旧内容先淡出 heroFadeOut，换上新内容，
// 再等 heroFadeInDelay 后淡入。
type heroState struct {
	target  string    // 上一次构建时的 currentSnapshotKey（effect 的依赖）
	shown   *heroSnap // displayedSnapshot
	key     string    // displayedSnapshotKeyRef
	visible bool
	next    *heroSnap // 淡出结束后换上的内容（effect 运行时的 pendingSnapshotRef）
	swapAt  time.Time // 换上 next 的时间
	showAt  time.Time // 晚于它的下一帧变为可见
}

// update 在每次构建时推进状态；返回下一个要重绘的时间间隔（没有等待中的计时器时为 0）。
func (h *heroState) update(cur *heroSnap, now time.Time) time.Duration {
	curKey := ""
	if cur != nil {
		curKey = cur.game.ID
	}
	if curKey != h.target {
		h.target = curKey
		h.swapAt, h.showAt, h.next = time.Time{}, time.Time{}, nil
		switch {
		case curKey == "":
			h.shown, h.key, h.visible = nil, "", false
		case h.key == "" || h.key == curKey:
			h.shown, h.key, h.visible = cur, curKey, false
			h.showAt = now
		default:
			h.visible = false
			h.next, h.swapAt = cur, now.Add(heroFadeOut)
		}
	}
	if !h.swapAt.IsZero() && !now.Before(h.swapAt) {
		h.shown, h.key, h.next = h.next, h.target, nil
		h.swapAt, h.showAt = time.Time{}, now.Add(heroFadeInDelay)
	}
	if !h.showAt.IsZero() && now.After(h.showAt) {
		h.visible, h.showAt = true, time.Time{}
	}
	if cur != nil && h.key == curKey {
		h.shown = cur
	}
	switch {
	case !h.swapAt.IsZero():
		return h.swapAt.Sub(now)
	case !h.showAt.IsZero():
		return max(h.showAt.Sub(now), time.Millisecond)
	}
	return 0
}

// crossfade 对应 useCrossfadeBackground：背景换图时旧图盖在上面，在 backgroundCrossfade 内淡出。
type crossfade struct {
	game    models.Game // 当前背景的游戏（封面缓存按游戏取图）
	url     string      // backgroundUrlRef
	prev    models.Game // 正在淡出的背景的游戏
	prevURL string      // previousBackgroundUrl
	since   time.Time   // 开始淡出的帧时间
}

// update 记录背景地址的变化；返回旧图还要显示多久（不在淡出时为 0）。
func (f *crossfade) update(g models.Game, url string, now time.Time) time.Duration {
	switch {
	case url == "":
		f.prevURL = ""
	case f.url != "" && url != f.url:
		f.prev, f.prevURL, f.since = f.game, f.url, now
	}
	f.game, f.url = g, url
	if f.prevURL == "" {
		return 0
	}
	left := backgroundCrossfade - now.Sub(f.since)
	if left <= 0 {
		f.prevURL = ""
		return 0
	}
	return left
}

// Page 是首页的状态。每次进入页面（Open）都重新加载首页数据与统计，与原版路由重新挂载组件一致。
type Page struct {
	env      *env.Env
	launcher *launch.Launcher // 运行状态是整个应用的，与详情页共用
	rail     *homerail.Rail

	data         *vo.HomePageData // homeData；没有加载过或加载失败时为 nil
	version      int              // 每次拿到新的首页数据加一（原版 carouselGames 随之变化，轮播重新计时）
	loading      bool             // isLoading
	seq          int              // 每次获取首页数据加一，只采用最新的结果
	libraryStats *vo.PeriodStats
	gen          int  // 每次进入页面加一，丢弃上一次进入时还没返回的统计
	skipLaunch   bool // skipNextLaunchHomeRefreshRef：刚启动游戏时跳过下一次 home:refresh-requested

	activeID string // activeGameId
	expanded bool   // isPickerExpanded
	tickFrom time.Time
	tickDeps string // 轮播计时器的依赖，变化时重新计时（原版 effect 重新 setInterval）

	debugPicker bool // 截图调试：进入页面时展开封面选择面板，见 SetDebugState

	hero   heroState
	bg     crossfade
	accent ui.Color            // heroAccentRgb：换游戏后在新封面的主色算出之前保持原来的颜色
	toasts []func() env.Notice // 待显示的提示（显示时按当前语言取）
}

// New 创建页面并订阅让首页刷新的事件；数据在 Open 时加载。
func New(e *env.Env) *Page {
	p := &Page{env: e, launcher: launch.For(e), rail: homerail.New(e), accent: covers.DefaultAccent}
	if e.Svc != nil && e.Svc.Events != nil {
		e.Svc.Events.On(refreshEvent, func(any) {
			if p.skipLaunch {
				p.skipLaunch = false
				return
			}
			p.fetch(false, false)
		})
		e.Svc.Events.On(runtimeEvent, p.runtimeChanged)
		e.Svc.Events.On(windowShownEvent, func(any) { p.fetch(true, true) })
	}
	return p
}

// Open 进入页面：重置页面状态，加载首页数据与游戏库统计（useEffect 里的 fetchHomeData 与 loadLibraryPreviewStats）。
func (p *Page) Open() {
	p.gen++
	p.activeID, p.expanded, p.libraryStats = "", p.debugPicker, nil
	p.tickFrom, p.tickDeps = time.Time{}, ""
	p.hero, p.bg, p.toasts = heroState{}, crossfade{}, nil
	p.accent = covers.DefaultAccent
	p.rail.Reset()
	p.fetch(true, true)
	p.loadLibraryStats()
}

// Refresh 对应根布局 handleImportComplete 里的 fetchHomeData({ showLoading: false, syncRuntime: false })。
func (p *Page) Refresh() { p.fetch(false, false) }

// SetDebugState 让截图直接进入某个状态：picker（展开封面选择面板）。
func (p *Page) SetDebugState(state string) { p.debugPicker = state == "picker" }

// Loading 报告首页数据是否在加载中（截图前等待）。
func (p *Page) Loading() bool { return p.loading }

// fetch 对应 store 的 fetchHomeData：showLoading 时加载期间页面为空；syncRuntime 时按首页数据同步正在运行的游戏
// （setGameRuntimeFromHome）。失败时保留原来的数据。
func (p *Page) fetch(showLoading, syncRuntime bool) {
	if p.env.Svc == nil {
		return
	}
	if showLoading {
		p.loading = true
	}
	p.seq++
	seq := p.seq
	p.env.Async(func() func() {
		data, err := p.env.Svc.Home.GetHomePageData()
		return func() {
			if showLoading {
				p.loading = false
			}
			if err != nil {
				log.Printf("Failed to fetch home data: %v", err)
				return
			}
			if seq != p.seq {
				return
			}
			p.data = &data
			p.version++
			if syncRuntime {
				p.launcher.SyncFromHome(data.RecentPlayed)
			}
		}
	})
}

// loadLibraryStats 对应 loadLibraryPreviewStats：全部时间的全局统计。
func (p *Page) loadLibraryStats() {
	if p.env.Svc == nil {
		return
	}
	gen := p.gen
	p.env.Async(func() func() {
		data, err := p.env.Svc.Stats.GetGlobalPeriodStats(vo.PeriodStatsRequest{Dimension: enums.All})
		return func() {
			if err != nil {
				log.Printf("Failed to fetch library preview stats: %v", err)
				return
			}
			if gen == p.gen {
				p.libraryStats = &data
			}
		}
	})
}

// runtimeChanged 对应 useAppRuntimeEffects 里的 game-runtime:changed：刚启动（launched）时跳过随后的刷新请求，
// playing、ending 不刷新，其余（结束、启动失败）刷新首页数据。运行状态本身由 Launcher 合并。
func (p *Page) runtimeChanged(data any) {
	var ev service.GameRuntimeChangedEvent
	switch v := data.(type) {
	case service.GameRuntimeChangedEvent:
		ev = v
	case *service.GameRuntimeChangedEvent:
		if v == nil {
			return
		}
		ev = *v
	default:
		return
	}
	switch {
	case ev.State == service.GameRuntimeStateLaunching && ev.Reason == "launched":
		p.skipLaunch = true
	case ev.State == service.GameRuntimeStatePlaying || ev.State == service.GameRuntimeStateEnding:
	default:
		p.fetch(false, false)
	}
}

// items 对应 carouselItems：最近游玩的游戏；上次游玩的游戏不在其中时放在最前面。
func (p *Page) items() []vo.LastPlayedGame {
	if p.data == nil {
		return nil
	}
	items := p.data.RecentPlayed
	if lp := p.data.LastPlayed; lp != nil && lp.Game.ID != "" &&
		!slices.ContainsFunc(items, func(it vo.LastPlayedGame) bool { return it.Game.ID == lp.Game.ID }) {
		items = append([]vo.LastPlayedGame{*lp}, items...)
	}
	return items
}

// selected 对应 selectedCarouselItem：activeGameId 对应的游戏，找不到时是第一个；同时把 activeGameId 校正过去
// （原版依赖 carouselItems 的 effect）。
func (p *Page) selected(items []vo.LastPlayedGame) *vo.LastPlayedGame {
	if len(items) == 0 {
		p.activeID = ""
		return nil
	}
	i := slices.IndexFunc(items, func(it vo.LastPlayedGame) bool { return it.Game.ID == p.activeID })
	if i < 0 {
		i = 0
	}
	p.activeID = items[i].Game.ID
	return &items[i]
}

// carouselInterval 对应 homeGameCarouselIntervalMs：max(4, home_game_carousel_interval_sec || 6) 秒。
func carouselInterval(cfg *appconf.AppConfig) time.Duration {
	sec := appconf.DefaultHomeGameCarouselIntervalSec
	if cfg != nil && cfg.HomeGameCarouselIntervalSec != 0 {
		sec = cfg.HomeGameCarouselIntervalSec
	}
	return time.Duration(max(appconf.MinHomeGameCarouselIntervalSec, sec)) * time.Second
}

// tick 对应轮播的 setInterval：多于一个游戏、开启了自动轮播、面板收起、没有游戏在运行时，每隔一段时间切换到下一个游戏。
// 返回距下一次切换的时间（计时器没在运行时为 0）。
func (p *Page) tick(items []vo.LastPlayedGame, now time.Time) time.Duration {
	cfg := p.env.Config()
	enabled := cfg == nil || cfg.HomeGameCarouselEnabled
	if len(items) <= 1 || !enabled || p.expanded || p.launcher.Active() {
		p.tickFrom, p.tickDeps = time.Time{}, ""
		return 0
	}
	interval := carouselInterval(cfg)
	deps := strconv.Itoa(p.version) + "/" + interval.String()
	if p.tickFrom.IsZero() || deps != p.tickDeps {
		p.tickFrom, p.tickDeps = now, deps
	}
	if now.Sub(p.tickFrom) >= interval {
		i := slices.IndexFunc(items, func(it vo.LastPlayedGame) bool { return it.Game.ID == p.activeID })
		p.activeID = items[(i+1)%len(items)].Game.ID // 找不到时 i 为 -1，回到第一个
		p.tickFrom = p.tickFrom.Add(interval)
		if now.Sub(p.tickFrom) >= interval { // 窗口隐藏期间没有帧：不补切，从现在重新计时
			p.tickFrom = now
		}
	}
	return interval - now.Sub(p.tickFrom)
}

// openDetail 对应 openGameDetail。
func (p *Page) openDetail(id string) {
	if id != "" {
		p.env.Router.Push(env.GamePath(id))
	}
}

// continueGame 对应 handleContinuePlay：不带选项启动游戏（store.startGame）；成功时选中它，出错时提示。
func (p *Page) continueGame(g models.Game) {
	if g.ID == "" {
		return
	}
	id := g.ID
	p.launcher.StartGame(g, func(started bool, err error) {
		if err != nil {
			log.Printf("Failed to launch game: %v", err)
			p.toasts = append(p.toasts, func() env.Notice { return env.Error(locales.HomeToastLaunchFailed()) })
			return
		}
		if started {
			p.activeID = id
		}
	})
}
