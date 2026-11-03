// Package game 是游戏详情页，对应原版 routes/game.tsx：页头、“游戏统计”标签页（GameStatsPanel）与页头的弹窗。
// 其余标签页在 ui/components/panel 的子包里。
package game

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/modal"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/backup"
	"lunabox/desktop-mygo/ui/components/panel/edit"
	"lunabox/desktop-mygo/ui/components/panel/launch"
	"lunabox/desktop-mygo/ui/components/panel/progress"
	"lunabox/desktop-mygo/ui/components/panel/review"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// Links 是详情页与游戏库之间的联系：点击开发商按名称搜索，点击标签按标签筛选；游戏被修改或删除后
// LibraryChanged 让游戏库重新加载（对应原版的 invalidateAllGameLists）。
type Links struct {
	SearchLibrary  func(query string)
	FilterByTag    func(tag string)
	LibraryChanged func()
}

// 标签页，顺序与原版一致。
var tabs = []struct {
	key   string
	label func() string
}{
	{"stats", locales.GameTabsStats},
	{"edit", locales.CommonEdit},
	{"launch", locales.GameTabsLaunch},
	{"backup", locales.GameTabsBackup},
	{"progress", locales.GameTabsProgress},
	{"review", locales.GameTabsReview},
}

const (
	skeletonDelay = 300 * time.Millisecond // 原版显示骨架屏前的等待，数据很快返回时不闪一下骨架
	autoSaveDelay = 500 * time.Millisecond // 原版自动保存的防抖
)

// Page 是详情页的状态。每次进入页面（Open）都重新加载，与原版路由重新挂载组件一致。
type Page struct {
	env      *env.Env
	links    Links
	launcher *launch.Launcher // 运行状态是整个应用的，跨游戏保留

	id       string
	gen      int
	game     *models.Game
	tags     []models.GameTag
	loading  bool
	openedAt time.Time
	dirty    bool      // 编辑后尚未自动保存
	dirtyAt  time.Time // 最后一次编辑的时间（View 用 c.Now() 记下，与防抖用同一个时钟）

	tab             string
	launchMode      enums.LaunchMode
	statusOpen      bool       // 窄屏时状态下拉菜单
	viewerOpen      bool       // 封面大图（BetterImageViewer）
	fullCover       *ui.Bitmap // 图片查看器的大图
	tagEditor       components.TagEditor
	category        modal.AddToCategory
	categoryInitial []string // 打开收藏弹窗时游戏所在的收藏夹
	guide           guideModal
	addSession      addSessionModal
	scroll          components.ScrollBar

	failed   func() string // 待显示的错误提示（显示时按当前语言取）
	debugTab string        // -state tab:<key>
	nextTab  string        // 下一次 Open 显示的标签页（ShowTab）

	stats       statsPanel
	editPanel   edit.State
	launchPanel launch.State
	backupPanel backup.State
	progress    progress.State
	review      review.State
}

// New 创建页面。数据在 Open 时加载。
func New(e *env.Env, links Links) *Page {
	p := &Page{env: e, links: links, tab: "stats", launcher: launch.For(e)}
	if e.Svc != nil {
		e.Svc.Events.On("cover-image:download", p.coverDownloaded)
	}
	return p
}

// coverDownloaded 对应原版对 cover-image:download 事件的处理：远程封面下载到本地后，换上新封面。
func (p *Page) coverDownloaded(data any) {
	ev, _ := data.(map[string]string)
	if p.game == nil || ev["game_id"] != p.game.ID || ev["status"] != "done" {
		return
	}
	id, gen := p.game.ID, p.gen
	p.env.Async(func() func() {
		saved, err := p.env.Svc.Game.GetGameByID(id)
		return func() {
			if err != nil || gen != p.gen || p.game == nil {
				return
			}
			p.env.Covers.Forget(p.game)
			p.game.CoverURL, p.game.CoverSourceURL, p.game.UpdatedAt = saved.CoverURL, saved.CoverSourceURL, saved.UpdatedAt
			p.env.Covers.Forget(p.game)
			p.fullCover = nil
		}
	})
}

// Open 进入某个游戏的详情页：补存上一个游戏未保存的修改，清空状态并加载数据。
func (p *Page) Open(id string, now time.Time) {
	p.Leave()
	p.id = id
	p.gen++
	p.game, p.tags = nil, nil
	p.loading, p.openedAt = true, now
	p.tab = "stats"
	if p.debugTab != "" {
		p.tab = p.debugTab
	}
	if p.nextTab != "" {
		p.tab, p.nextTab = p.nextTab, ""
	}
	p.statusOpen, p.viewerOpen, p.fullCover, p.tagEditor = false, false, nil, components.TagEditor{}
	p.category, p.categoryInitial, p.guide, p.addSession = modal.AddToCategory{}, nil, guideModal{}, addSessionModal{}
	p.scroll = components.ScrollBar{}
	p.stats = statsPanel{}
	p.editPanel, p.launchPanel, p.backupPanel, p.progress, p.review = edit.State{}, launch.State{}, backup.State{}, progress.State{}, review.State{}

	gen := p.gen
	p.env.Async(func() func() {
		g, err := p.env.Svc.Game.GetGameByID(id)
		tags, _ := p.env.Svc.Tag.GetTagsByGame(id)
		return func() {
			if gen != p.gen {
				return
			}
			p.loading = false
			if err != nil {
				p.failed = locales.GameToastLoadDataFailed
				return
			}
			p.game, p.tags = &g, tags
			p.launchMode = defaultLaunchMode(g)
		}
	})
	p.stats.load(p.env, id)
}

// panel 是传给标签页的内容。
func (p *Page) panel() panel.Game {
	return panel.Game{
		Env:       p.env,
		Game:      p.game,
		OnChange:  p.updateGame,
		OnSaved:   p.reload,
		OnDeleted: p.deleted,
	}
}

// updateGame 对应 updateGameState：替换页面里的游戏，autoSaveDelay 后由 View 保存。
func (p *Page) updateGame(g models.Game) {
	if p.game == nil || g.ID != p.game.ID {
		return
	}
	if g.LaunchMode != p.game.LaunchMode {
		p.launchMode = defaultLaunchMode(g)
	}
	p.game = &g
	p.dirty, p.dirtyAt = true, time.Time{}
}

// autoSave 在编辑停下 autoSaveDelay 后保存（原版的自动保存 useEffect）。
func (p *Page) autoSave(now time.Time) (wait time.Duration) {
	if !p.dirty || p.game == nil {
		return 0
	}
	if p.dirtyAt.IsZero() {
		p.dirtyAt = now
	}
	if wait := autoSaveDelay - now.Sub(p.dirtyAt); wait > 0 {
		return wait
	}
	p.Flush()
	return 0
}

// Leave 在离开页面、进入另一个游戏前调用：补存未保存的修改，以及编辑面板里还没到期的来源 ID（抽屉打开时离开页面）。
func (p *Page) Leave() {
	p.Flush()
	edit.Flush(&p.editPanel, p.panel())
}

// Flush 立即保存未保存的修改（离开页面前，原版卸载时的补存）。
func (p *Page) Flush() {
	if !p.dirty || p.game == nil {
		return
	}
	p.dirty, p.dirtyAt = false, time.Time{}
	g := *p.game
	p.env.Async(func() func() {
		err := p.env.Svc.Game.UpdateGame(g)
		return func() {
			if err != nil {
				msg := err.Error()
				p.failed = func() string { return locales.GameToastSaveFailed(msg) }
			}
			if p.links.LibraryChanged != nil {
				p.links.LibraryChanged()
			}
		}
	})
}

// reload 重新读取游戏与标签（面板调用了会改游戏的服务之后），并刷新游戏库。
func (p *Page) reload() {
	id, gen := p.id, p.gen
	p.env.Async(func() func() {
		g, err := p.env.Svc.Game.GetGameByID(id)
		tags, _ := p.env.Svc.Tag.GetTagsByGame(id)
		return func() {
			if gen != p.gen || err != nil {
				return
			}
			if g.LaunchMode != p.game.LaunchMode { // 原版只在 launch_mode 变化时重设临时启动方式
				p.launchMode = defaultLaunchMode(g)
			}
			p.game, p.tags, p.dirty = &g, tags, false
			if p.links.LibraryChanged != nil {
				p.links.LibraryChanged()
			}
		}
	})
}

// deleted 在游戏被删除后回到游戏库。
func (p *Page) deleted() {
	p.dirty = false
	if p.links.LibraryChanged != nil {
		p.links.LibraryChanged()
	}
	p.env.Router.Push(env.PathLibrary)
}

// Title 是页面标题（窗口历史里显示）。
func (p *Page) Title() string {
	if p.game != nil {
		return p.game.Name
	}
	return locales.NativeGameDetail()
}

// Loading 报告游戏数据是否还在加载（截图用）。
func (p *Page) Loading() bool { return p.loading || p.stats.loading }

// SetTab 让之后进入的详情页停在某个标签页（截图调试用）。
func (p *Page) SetTab(tab string) { p.tab, p.debugTab = tab, tab }

// Tab 是当前的标签页（测试用）。
func (p *Page) Tab() string { return p.tab }

// ShowTab 对应 App.tsx 的 openGameLaunchSettings 设置 location.hash：显示游戏 id 的 tab 标签页。open 报告详情页
// 已经打开着这个游戏（直接切换）；否则留到接下来导航到它时的 Open。
func (p *Page) ShowTab(id, tab string, open bool) {
	if open && p.id == id {
		p.tab, p.nextTab = tab, ""
		return
	}
	p.nextTab = tab
}

// defaultLaunchMode 对应 defaultLaunchModeForGame。
func defaultLaunchMode(g models.Game) enums.LaunchMode {
	switch g.LaunchMode {
	case enums.LaunchModeSteam, enums.LaunchModeCompatibility, enums.LaunchModeAdmin:
		return g.LaunchMode
	}
	return enums.LaunchModeNormal
}

// launchOptions 对应页头的 launchOptions：普通启动，macOS 加兼容层启动，支持 Steam 启动时加 Steam，Windows 加管理员启动。
// 文字按当前语言取，每帧重建。
func launchOptions(l *launch.Launcher, g *models.Game) []better.SplitOption {
	opts := []better.SplitOption{
		{Key: string(enums.LaunchModeNormal), Label: locales.GameCardStartGame(), Description: locales.GameCardNormalLaunchDesc(), Icon: "play"},
	}
	if l.SupportsCompatibility() {
		opts = append(opts, better.SplitOption{Key: string(enums.LaunchModeCompatibility), Label: locales.GameCardStartWithCompatibility(), Description: locales.GameCardCompatibilityLaunchDesc(), Icon: "application-brackets-outline"})
	}
	if l.SupportsSteam(g) {
		opts = append(opts, better.SplitOption{Key: string(enums.LaunchModeSteam), Label: locales.GameCardStartWithSteam(), Description: locales.GameCardSteamLaunchDesc(), Icon: "steam"})
	}
	if l.SupportsAdmin() {
		opts = append(opts, better.SplitOption{Key: string(enums.LaunchModeAdmin), Label: locales.GameCardStartAsAdmin(), Description: locales.GameCardAdminLaunchDesc(), Icon: "shield-account"})
	}
	return opts
}

func launchOption(options []better.SplitOption, mode enums.LaunchMode) better.SplitOption {
	for _, o := range options {
		if o.Key == string(mode) {
			return o
		}
	}
	return options[0]
}

// currentStatus 是游戏的状态，未知状态按“未开始”。
func currentStatus(g *models.Game) enums.GameStatus {
	for _, s := range components.GameStatuses {
		if g.Status == s {
			return s
		}
	}
	return enums.StatusNotStarted
}

// metadataSource 是一个有网页的元数据来源。
type metadataSource struct {
	source enums.SourceType
	id     string
	url    string
}

// metadataSources 对应 metadataSources：没有来源列表时退回默认来源。
func metadataSources(g *models.Game) []models.GameMetadataSource {
	if len(g.MetadataSources) > 0 {
		return g.MetadataSources
	}
	if g.SourceType != "" && g.SourceID != "" {
		return []models.GameMetadataSource{{SourceType: g.SourceType, SourceID: g.SourceID}}
	}
	return nil
}

// metadataSourceText 对应 metadataSourceText：“bangumi 等 2 个”（gameEdit.multipleSources）。
func metadataSourceText(g *models.Game) string {
	if g.SourceType == "" {
		return "-"
	}
	if n := len(metadataSources(g)); n > 1 {
		return string(g.SourceType) + " " + locales.GameEditMultipleSources(strconv.Itoa(n))
	}
	return string(g.SourceType)
}

// sourceLinks 对应 metadataSourceLinks：有网页的来源，默认来源排在最前。
func sourceLinks(g *models.Game, erogameScapeBase string) []metadataSource {
	var def, rest []metadataSource
	for _, s := range metadataSources(g) {
		id := strings.TrimSpace(s.SourceID)
		u := sourceURL(s.SourceType, id, erogameScapeBase)
		if u == "" {
			continue
		}
		link := metadataSource{source: s.SourceType, id: id, url: u}
		if s.SourceType == g.SourceType {
			def = append(def, link)
		} else {
			rest = append(rest, link)
		}
	}
	return append(def, rest...)
}

// sourceURL 对应 utils/metadataSources.ts 的 getMetadataSourceURL。
func sourceURL(source enums.SourceType, id, erogameScapeBase string) string {
	if source == "" || id == "" {
		return ""
	}
	id = strings.ReplaceAll(url.QueryEscape(id), "+", "%20") // encodeURIComponent
	switch source {
	case enums.VNDB:
		return "https://vndb.org/" + id
	case enums.Bangumi:
		return "https://bgm.tv/subject/" + id
	case enums.Ymgal:
		return "https://www.ymgal.games/ga/" + id
	case enums.Steam:
		return "https://store.steampowered.com/app/" + id
	case enums.DLsite:
		return "https://www.dlsite.com/maniax/work/=/product_id/" + id + ".html"
	case enums.TouchGal:
		return "https://www.touchgal.ink/" + id
	case enums.Hikarinagi:
		return "https://www.hikarinagi.org/galgames/" + id
	case enums.ErogameScape:
		base := strings.TrimRight(strings.TrimSpace(erogameScapeBase), "/")
		if base == "" {
			base = appconf.DefaultErogameScapeBaseURL
		}
		return base + "/game.php?game=" + id
	}
	return ""
}

// sourceLabel 对应 importFlow.ts 的 sourceLabel：Bangumi、VNDB、Steam 原版不翻译。
func sourceLabel(source enums.SourceType) string {
	switch source {
	case enums.Bangumi:
		return "Bangumi"
	case enums.VNDB:
		return "VNDB"
	case enums.Ymgal:
		return locales.GameEditSourceYmgal()
	case enums.DLsite:
		return locales.GameEditSourceDlsite()
	case enums.TouchGal:
		return locales.GameEditSourceTouchGal()
	case enums.Hikarinagi:
		return locales.GameEditSourceHikarinagi()
	case enums.ErogameScape:
		return locales.GameEditSourceErogameScape()
	}
	return "Steam"
}
