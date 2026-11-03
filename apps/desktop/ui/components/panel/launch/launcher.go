package launch

import (
	"log"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	launcherpkg "lunabox/backend/service/launcher"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// runtimeEvent 是 StartService 发出的运行状态事件（useAppRuntimeEffects 订阅的 game-runtime:changed）。
const runtimeEvent = "game-runtime:changed"

// runtimeInfo 对应 store 的 GameRuntimeInfo（没有 isFocused：界面没有用到）。
type runtimeInfo struct {
	game          *models.Game
	sessionID     string
	startTime     time.Time
	state         service.GameRuntimeState
	timingMode    service.GameRuntimeTimingMode
	activeSeconds *int
	// optimistic 是 startGame 乐观写入时的序号（原版用乐观的 startTime 认出自己）；后端事件带来真实的
	// start_time 后清零，启动失败时只回滚仍是自己写入的那一条。
	optimistic int
}

// steamAction 对应 SteamPendingAction：Steam 状态就绪后要继续做的事。
type steamAction string

const (
	steamNone        steamAction = ""
	steamSaveDefault steamAction = "save-default"
	steamLaunch      steamAction = "launch"
)

// steamFlow 是 Steam 导入弹窗与 pendingSteamAction（routes/game.tsx 的 isSteamModalOpen、isCheckingSteam、
// isImportingSteam、steamStatus、steamImportCompatibility）。
type steamFlow struct {
	open      bool
	checking  bool
	importing bool
	status    *service.SteamLaunchStatus
	action    steamAction
	gameID    string

	// 弹窗里的 Steam 启动参数与 Proton（SteamImportModal 的 launchOptions、compatibilityTool）：打开时取游戏的
	// steam_launch_options 与 compat 的 current_tool。compat 只在 Linux 上读取。
	launchOptions string
	tool          string
	compat        *service.SteamCompatibilityInfo
}

// Launcher 启动游戏并跟踪运行状态（store.gameRuntimes），整个应用一个（For），详情页与首页共用。
// Steam 导入流程（resolveSteamGame 与 SteamImportModal）也在这里：页头的启动按钮与“启动配置”里
// 选择 Steam 默认启动方式共用它。
type Launcher struct {
	env      *env.Env
	goos     string // store 的 platformGOOS（GetGOOS）
	runtimes map[string]*runtimeInfo
	activeID string // activeGameRuntimeId：灵动岛显示的游戏
	seq      int
	steam    steamFlow
	// latest 是最近一帧页面传来的内容（Modals 与 View 每帧更新）。后台结果回到主线程时，
	// 在它的游戏上合并改动，而不是在发起操作时的旧副本上。
	latest panel.Game
}

// launchers 让各页面与“启动配置”标签页（View 只拿到 Env）找到同一个 Launcher。
var launchers sync.Map // *env.Env → *Launcher

// NewLauncher 创建启动器；订阅服务的运行状态事件（事件在主线程上分发）。
func NewLauncher(e *env.Env) *Launcher {
	l := &Launcher{env: e, goos: runtime.GOOS, runtimes: map[string]*runtimeInfo{}}
	if e != nil && e.Svc != nil && e.Svc.Version != nil {
		l.goos = e.Svc.Version.GetGOOS()
	}
	if e != nil && e.Svc != nil && e.Svc.Events != nil {
		e.Svc.Events.On(runtimeEvent, l.applyEvent)
	}
	launchers.Store(e, l)
	return l
}

// For 返回 e 的 Launcher，没有时创建一个：详情页、首页与“启动配置”标签页共用同一份运行状态。
func For(e *env.Env) *Launcher {
	if v, ok := launchers.Load(e); ok {
		return v.(*Launcher)
	}
	return NewLauncher(e)
}

// GOOS 是 store 的 platformGOOS。
func (l *Launcher) GOOS() string { return l.goos }

// SupportsAdmin 对应 supportsAdminLaunch：只有 Windows 能以管理员身份启动（也只有 Windows 有启动增强工具）。
func (l *Launcher) SupportsAdmin() bool { return l.goos == "windows" }

// SupportsCompatibility 报告是否有兼容层启动（LaunchModeCompatibility）：只有 macOS。
func (l *Launcher) SupportsCompatibility() bool { return l.goos == "darwin" }

// SupportsWine 对应 supportsWineLaunch：macOS 与 Linux 用 Wine / Proton 运行 Windows 游戏。
func (l *Launcher) SupportsWine() bool { return l.goos == "darwin" || l.goos == "linux" }

// SupportsSteam 对应 supportsSteamLaunch：Windows、Linux 都能通过 Steam 启动，macOS 只有关联到 Steam 原生游戏的可以。
func (l *Launcher) SupportsSteam(g *models.Game) bool {
	switch l.goos {
	case "windows", "linux":
		return true
	case "darwin":
		return g != nil && g.SteamLaunchKind == "native" && g.SteamLaunchID != ""
	}
	return false
}

// EffectiveMode 对应详情页的 selectedLaunchMode 与面板里的 launchMode：当前平台不支持的启动方式按普通启动。
func (l *Launcher) EffectiveMode(m enums.LaunchMode, g *models.Game) enums.LaunchMode {
	switch {
	case m == enums.LaunchModeSteam && !l.SupportsSteam(g),
		m == enums.LaunchModeCompatibility && !l.SupportsCompatibility(),
		m == enums.LaunchModeAdmin && !l.SupportsAdmin(),
		m == "":
		return enums.LaunchModeNormal
	}
	return m
}

// Status 是游戏的运行状态：running 对应 gameRuntime 存在，ending 对应 state === "ending"。
func (l *Launcher) Status(gameID string) (running, ending bool) {
	r := l.runtimes[gameID]
	if r == nil {
		return false, false
	}
	return true, r.state == service.GameRuntimeStateEnding
}

// Active 对应 hasVisibleGameRuntime：有游戏正在启动、运行或结束中（idle 的运行状态已经删除）。
func (l *Launcher) Active() bool { return len(l.runtimes) > 0 }

// Runtime 是灵动岛显示的运行状态（store 的 gameRuntimes[activeGameRuntimeId]）。
type Runtime struct {
	Game      models.Game
	SessionID string
	StartTime time.Time
	State     service.GameRuntimeState
	// ActiveSeconds 是 timingMode === "active" 时后端报告的活跃时长；为 nil 时按 StartTime 计墙钟时长。
	ActiveSeconds *int
}

// Current 返回灵动岛显示的游戏与正在运行的游戏数（runtimeCount）；没有正在运行的游戏，或它还没有游戏数据时 ok 为 false。
func (l *Launcher) Current() (rt Runtime, count int, ok bool) {
	r := l.runtimes[l.activeID]
	if r == nil || r.game == nil {
		return Runtime{}, len(l.runtimes), false
	}
	rt = Runtime{Game: *r.game, SessionID: r.sessionID, StartTime: r.startTime, State: r.state}
	if r.timingMode == service.GameRuntimeTimingModeActive {
		secs := 0
		if r.activeSeconds != nil {
			secs = *r.activeSeconds
		}
		rt.ActiveSeconds = &secs
	}
	return rt, len(l.runtimes), true
}

// sortedIDs 是正在运行的游戏，按启动时间排列（Go 的 map 没有顺序，原版按插入顺序）。
func (l *Launcher) sortedIDs() []string {
	ids := make([]string, 0, len(l.runtimes))
	for id := range l.runtimes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := l.runtimes[ids[i]], l.runtimes[ids[j]]
		if !a.startTime.Equal(b.startTime) {
			return a.startTime.Before(b.startTime)
		}
		return ids[i] < ids[j]
	})
	return ids
}

// selectRuntime 对应 runtimeSelectionPatch / pickGameRuntime：优先 preferred，否则最晚启动的游戏。
func (l *Launcher) selectRuntime(preferred string) {
	if preferred != "" && l.runtimes[preferred] != nil {
		l.activeID = preferred
		return
	}
	l.activeID = ""
	var latest time.Time
	for id, r := range l.runtimes {
		if l.activeID == "" || r.startTime.After(latest) || (r.startTime.Equal(latest) && id < l.activeID) {
			l.activeID, latest = id, r.startTime
		}
	}
}

// SelectNext 对应 selectNextGameRuntime：灵动岛换到下一个正在运行的游戏。
func (l *Launcher) SelectNext() {
	ids := l.sortedIDs()
	if len(ids) <= 1 {
		return
	}
	cur := -1
	for i, id := range ids {
		if id == l.activeID {
			cur = i
		}
	}
	l.selectRuntime(ids[(cur+1)%len(ids)])
}

// EndSession 对应灵动岛的 EndCurrentPlaySession：结束游戏的当前会话（后端随后发出 ending、idle 事件）。
// done 在主线程上收到错误。
func (l *Launcher) EndSession(gameID string, done func(err error)) {
	e := l.env
	e.Async(func() func() {
		err := e.Svc.Start.EndCurrentPlaySession(gameID)
		return func() { done(err) }
	})
}

// SyncFromHome 对应 store 的 setGameRuntimeFromHome：以首页数据里正在游玩的游戏为准重建运行状态，
// 保留已有的会话与 launching、ending 状态。
func (l *Launcher) SyncFromHome(recent []vo.LastPlayedGame) {
	next := map[string]*runtimeInfo{}
	for _, item := range recent {
		if !item.IsPlaying || item.Game.ID == "" {
			continue
		}
		g := item.Game
		r := &runtimeInfo{game: &g, startTime: item.LastPlayedAt, state: service.GameRuntimeStatePlaying}
		if cur := l.runtimes[g.ID]; cur != nil {
			r.sessionID, r.timingMode, r.activeSeconds = cur.sessionID, cur.timingMode, cur.activeSeconds
			if cur.state == service.GameRuntimeStateLaunching || cur.state == service.GameRuntimeStateEnding {
				r.state = cur.state
			}
		}
		next[g.ID] = r
	}
	l.runtimes = next
	l.selectRuntime(l.activeID)
}

// applyEvent 对应 store 的 applyGameRuntimeEvent：idle 删除，其余按会话合并；同一会话的状态不回退。
func (l *Launcher) applyEvent(data any) {
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
	state := ev.State
	if state == "" {
		state = service.GameRuntimeStateIdle
	}
	id := ev.GameID
	if id == "" && ev.Game != nil {
		id = ev.Game.ID
	}
	if id == "" {
		if state == service.GameRuntimeStateIdle {
			clear(l.runtimes)
			l.activeID = ""
		}
		return
	}
	cur := l.runtimes[id]
	if state == service.GameRuntimeStateIdle {
		if ev.SessionID != "" && cur != nil && cur.sessionID != "" && cur.sessionID != ev.SessionID {
			return
		}
		delete(l.runtimes, id)
		preferred := l.activeID
		if preferred == id {
			preferred = ""
		}
		l.selectRuntime(preferred)
		return
	}
	if stateRegression(cur, ev.SessionID, state) {
		return
	}
	next := &runtimeInfo{}
	if cur != nil {
		*next = *cur
	}
	next.state = state
	if ev.Game != nil {
		next.game = ev.Game
	}
	if ev.SessionID != "" {
		next.sessionID = ev.SessionID
	}
	if !ev.StartTime.IsZero() {
		next.startTime, next.optimistic = ev.StartTime, 0
	}
	if ev.TimingMode != "" {
		next.timingMode = ev.TimingMode
	}
	if ev.ActiveSeconds != nil {
		secs := *ev.ActiveSeconds
		next.activeSeconds = &secs
	}
	preferred := l.activeID
	if state == service.GameRuntimeStateLaunching || l.runtimes[preferred] == nil {
		preferred = id
	}
	l.runtimes[id] = next
	l.selectRuntime(preferred)
}

// stateRegression 对应 isSameSessionStateRegression：同一会话里 playing/ending 之后的 launching、ending 之后的 playing 是迟到的事件。
func stateRegression(cur *runtimeInfo, sessionID string, next service.GameRuntimeState) bool {
	if cur == nil || cur.sessionID == "" || sessionID == "" || cur.sessionID != sessionID {
		return false
	}
	switch next {
	case service.GameRuntimeStateLaunching:
		return cur.state == service.GameRuntimeStatePlaying || cur.state == service.GameRuntimeStateEnding
	case service.GameRuntimeStatePlaying:
		return cur.state == service.GameRuntimeStateEnding
	}
	return false
}

// Start 对应详情页的 handleStartGame：正在运行时什么也不做；Steam 启动先确认 Steam 关联（resolveSteamGame）。
func (l *Launcher) Start(c *ui.Context, p panel.Game, mode enums.LaunchMode) {
	if p.Game == nil || p.Game.ID == "" || l.runtimes[p.Game.ID] != nil {
		return
	}
	l.latest = p
	if mode == enums.LaunchModeSteam {
		l.resolveSteam(c, steamLaunch, *p.Game, func(models.Game) {
			l.steam.action = steamNone
			l.performStart(c, l.gameOr(p.Game), enums.LaunchModeSteam)
		})
		return
	}
	l.performStart(c, *p.Game, mode)
}

// gameOr 返回页面当前的游戏（与 fallback 同一个 ID 时），否则 fallback。
func (l *Launcher) gameOr(fallback *models.Game) models.Game {
	if g := l.latest.Game; g != nil && g.ID == fallback.ID {
		return *g
	}
	return *fallback
}

// launchOptions 对应 performStartGame 传给 startGame 的选项：不支持管理员启动的平台上，管理员启动按普通启动。
func (l *Launcher) launchOptions(mode enums.LaunchMode) launcherpkg.LaunchOptions {
	f, tr := false, true
	switch {
	case mode == enums.LaunchModeAdmin && l.SupportsAdmin():
		return launcherpkg.LaunchOptions{RunAsAdmin: &tr, UseSteam: &f, UseCompatibility: &f}
	case mode == enums.LaunchModeSteam:
		return launcherpkg.LaunchOptions{UseSteam: &tr}
	case mode == enums.LaunchModeCompatibility:
		return launcherpkg.LaunchOptions{RunAsAdmin: &f, UseSteam: &f, UseCompatibility: &tr}
	}
	return launcherpkg.LaunchOptions{RunAsAdmin: &f, UseSteam: &f, UseCompatibility: &f}
}

// performStart 对应 performStartGame 与 store.startGame：先乐观地记为 launching，StartGameWithOptions 失败或
// 未启动时回滚；启动后重新读取游戏（页面的 OnSaved）。
func (l *Launcher) performStart(c *ui.Context, g models.Game, mode enums.LaunchMode) {
	id, name := g.ID, g.Name
	if id == "" {
		return
	}
	opts := l.launchOptions(mode)
	l.startGame(g, func() (bool, error) { return l.env.Svc.Start.StartGameWithOptions(id, opts) }, func(started bool, err error) {
		l.afterStart(c, id, name, started, err)
	})
}

// StartGame 对应不带选项的 store.startGame（StartGameWithTracking，按游戏自己的启动配置）：首页的“继续游戏”用它。
// done 在主线程上收到是否启动与错误。
func (l *Launcher) StartGame(g models.Game, done func(started bool, err error)) {
	if g.ID == "" {
		return
	}
	id := g.ID
	l.startGame(g, func() (bool, error) { return l.env.Svc.Start.StartGameWithTracking(id) }, done)
}

// StartFromCard 对应 GameCard 的 handleStartGame（游戏库、收藏详情的卡片）：正在运行时什么也不做；
// 不带选项启动，未启动或出错时提示。
func (l *Launcher) StartFromCard(c *ui.Context, g models.Game) {
	if g.ID == "" || l.runtimes[g.ID] != nil {
		return
	}
	name := g.Name
	l.StartGame(g, func(started bool, err error) {
		switch {
		case err != nil:
			log.Printf("Failed to start game: %v", err)
			l.env.Toast(c, env.Error(locales.GameCardStartFailedLog(name)))
		case !started:
			l.env.Toast(c, env.Error(locales.GameCardStartFailedNotLaunched(name)))
		}
	})
}

// startGame 是 store.startGame：已经在运行时直接算启动成功；否则先乐观地记为 launching，run 失败或未启动时回滚。
func (l *Launcher) startGame(g models.Game, run func() (bool, error), done func(started bool, err error)) {
	id := g.ID
	if l.runtimes[id] != nil {
		l.selectRuntime(id)
		done(true, nil)
		return
	}
	l.seq++
	seq, prevActive := l.seq, l.activeID
	l.runtimes[id] = &runtimeInfo{game: &g, startTime: time.Now(), state: service.GameRuntimeStateLaunching, optimistic: seq}
	l.selectRuntime(id)
	l.env.Async(func() func() {
		started, err := run()
		return func() {
			if !started || err != nil {
				if r := l.runtimes[id]; r != nil && r.optimistic == seq && r.state == service.GameRuntimeStateLaunching {
					delete(l.runtimes, id)
					l.selectRuntime(prevActive)
				}
			}
			done(started, err)
		}
	})
}

func (l *Launcher) afterStart(c *ui.Context, id, name string, started bool, err error) {
	switch {
	case err != nil:
		l.env.Toast(c, env.Error(locales.GameCardStartFailedLog(name)))
	case !started:
		l.env.Toast(c, env.Error(locales.GameCardStartFailedNotLaunched(name)))
	default:
		if p := l.latest; p.Game != nil && p.Game.ID == id && p.OnSaved != nil {
			p.OnSaved()
		}
	}
}

// change 在页面当前的游戏上做改动并交给页面（updateGameState）；游戏已换或没有变化时不提交。
func (l *Launcher) change(id string, edit func(g *models.Game)) {
	p := l.latest
	if p.Game == nil || p.Game.ID != id || p.OnChange == nil {
		return
	}
	g := *p.Game
	edit(&g)
	if reflect.DeepEqual(g, *p.Game) {
		return
	}
	p.OnChange(g)
	l.latest.Game = &g
}

// withSteamStatus 对应 gameWithSteamStatus：写入 Steam 启动身份，wine_prefix 为空时用 Proton prefix。
func withSteamStatus(g *models.Game, status service.SteamLaunchStatus) {
	g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID = status.LaunchID, status.LaunchKind, status.UserID
	if g.WinePrefix == "" {
		g.WinePrefix = status.ProtonPrefix
	}
}

// mergeSteamSettings 对应 mergeSteamSettings：只取重新读到的 Steam 相关字段，保留页面里其余未保存的修改。
func mergeSteamSettings(g *models.Game, refreshed models.Game) {
	g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID = refreshed.SteamLaunchID, refreshed.SteamLaunchKind, refreshed.SteamUserID
	g.SteamLaunchOptions = refreshed.SteamLaunchOptions
	g.UseLocaleEmulator = refreshed.UseLocaleEmulator
	g.WinePrefix = refreshed.WinePrefix
}

// resolveSteam 对应 resolveSteamGame：查询 Steam 关联；未就绪时打开导入弹窗，就绪时继续 then。
// GetGameSteamStatus 就绪时会把启动身份写进数据库，这里同步到页面里的游戏，免得之后的自动保存用旧值覆盖。
func (l *Launcher) resolveSteam(c *ui.Context, action steamAction, g models.Game, then func(models.Game)) {
	l.steam.action, l.steam.checking, l.steam.gameID = action, true, g.ID
	e := l.env
	e.Async(func() func() {
		status, err := e.Svc.Integration.GetGameSteamStatus(g.ID)
		return func() {
			l.steam.checking = false
			if err != nil {
				l.steam.action = steamNone
				e.Toast(c, env.Error(locales.SteamImportCheckFailed(err.Error())))
				return
			}
			l.steam.status = &status
			if !status.Ready {
				l.openSteamModal(g)
				return
			}
			l.change(g.ID, func(pg *models.Game) { withSteamStatus(pg, status) })
			withSteamStatus(&g, status)
			then(g)
		}
	})
}

// openSteamModal 打开导入弹窗：已经打开时（重新检查）保留弹窗里填的内容；对应 SteamImportModal 打开时取
// initialLaunchOptions，以及 resolveSteamGame 里的 loadSteamImportCompatibility（只在 Linux 上读取 Proton 列表）。
func (l *Launcher) openSteamModal(g models.Game) {
	f := &l.steam
	if f.open {
		return
	}
	f.open = true
	f.launchOptions, f.tool, f.compat = g.SteamLaunchOptions, "", nil
	if l.goos != "linux" {
		return
	}
	e, id := l.env, g.ID
	e.Async(func() func() {
		info, err := e.Svc.Integration.GetGameSteamCompatibility(id)
		return func() {
			if err != nil {
				log.Printf("Failed to load Steam compatibility tools: %v", err)
				return
			}
			if f.open && f.gameID == id {
				f.compat, f.tool = &info, info.CurrentTool
			}
		}
	})
}

// syncSteamSettings 对应 syncSteamSettingsFromDB：把后端写入的 Steam 字段（启动身份、启动参数、prefix）合并进页面里的游戏，
// 保留其余未保存的修改；返回合并后的游戏。
func (l *Launcher) syncSteamSettings(g models.Game, refreshed models.Game) models.Game {
	l.change(g.ID, func(pg *models.Game) { mergeSteamSettings(pg, refreshed) })
	merged := l.gameOr(&g)
	mergeSteamSettings(&merged, refreshed)
	return merged
}

// saveSteamAsDefault 对应 saveSteamAsDefault：默认启动方式改为 Steam。原版立即 UpdateGame；这里交给页面的自动保存，
// 与页面里尚未保存的修改一起写入，不与之竞争。
func (l *Launcher) saveSteamAsDefault(g models.Game) {
	l.change(g.ID, func(pg *models.Game) {
		pg.SteamLaunchID, pg.SteamLaunchKind, pg.SteamUserID = g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID
		if pg.WinePrefix == "" {
			pg.WinePrefix = g.WinePrefix
		}
		pg.LaunchMode = enums.LaunchModeSteam
	})
}

// finishSteam 在 Steam 关联就绪后关闭弹窗并继续挂起的操作（handleRetrySteamStatus 等的共同结尾）。
func (l *Launcher) finishSteam(c *ui.Context, action steamAction, g models.Game) {
	l.steam.open = false
	l.steam.action = steamNone
	if action == steamSaveDefault {
		l.saveSteamAsDefault(g)
	} else {
		l.performStart(c, l.gameOr(&g), enums.LaunchModeSteam)
	}
}

// retrySteam 对应 handleRetrySteamStatus。
func (l *Launcher) retrySteam(c *ui.Context, g models.Game) {
	action := l.steam.action
	if action == steamNone {
		return
	}
	l.resolveSteam(c, action, g, func(sg models.Game) { l.finishSteam(c, action, sg) })
}

// importSteam 对应 handleImportGameToSteam：带上弹窗里的启动参数与 Proton 写入 Steam 快捷方式（Linux 上 Steam 正在运行时
// 后端会重启 Steam）；就绪后重新读取后端保存的 Steam 字段。刚导入时 Steam 要重启，只提示不启动。
func (l *Launcher) importSteam(c *ui.Context, g models.Game) {
	action := l.steam.action
	if action == steamNone {
		return
	}
	l.steam.importing = true
	e, options, tool := l.env, l.steam.launchOptions, l.steam.tool
	e.Async(func() func() {
		result, err := e.Svc.Integration.ImportGameToSteamWithOptions(g.ID, options, tool)
		var refreshed models.Game
		if err == nil && result.Status.Ready {
			refreshed, err = e.Svc.Game.GetGameByID(g.ID)
		}
		return func() {
			l.steam.importing = false
			if err != nil {
				e.Toast(c, env.Error(locales.SteamImportImportFailed(err.Error())))
				return
			}
			l.steam.status = &result.Status
			if !result.Status.Ready {
				return
			}
			steamGame := l.syncSteamSettings(g, refreshed)
			withSteamStatus(&steamGame, result.Status)
			l.change(g.ID, func(pg *models.Game) { withSteamStatus(pg, result.Status) })
			l.steam.open = false
			l.steam.action = steamNone
			if action == steamSaveDefault {
				l.saveSteamAsDefault(steamGame)
			}
			// action 为 launch 时原版还会 setLaunchMode(Steam)：页头的临时启动方式由页面持有，而发起这次启动时它已经是 Steam。
			switch {
			case result.Imported:
				e.Toast(c, env.Notice{Type: "success", Msg: locales.SteamImportImportSuccess(), Duration: 6 * time.Second})
			case action == steamLaunch:
				l.performStart(c, steamGame, enums.LaunchModeSteam)
			}
		}
	})
}

// steamSelectExecutable 对应 handleSteamSelectExecutable：选择可执行文件，保存路径后重新检查 Steam 关联。
func (l *Launcher) steamSelectExecutable(c *ui.Context, g models.Game) {
	e := l.env
	e.Async(func() func() {
		path, err := e.Svc.Game.SelectGameExecutable(g.Path)
		return func() {
			if err != nil {
				e.Toast(c, env.Error(locales.GameToastSelectExecutableFailed()))
				return
			}
			if path == "" {
				return
			}
			updated := l.gameOr(&g)
			updated.Path = path
			e.Async(func() func() {
				err := e.Svc.Game.UpdateGame(updated)
				return func() {
					if err != nil {
						e.Toast(c, env.Error(locales.GameToastSelectExecutableFailed()))
						return
					}
					l.change(updated.ID, func(pg *models.Game) { pg.Path = path })
					action := l.steam.action
					if action == steamNone {
						return
					}
					l.resolveSteam(c, action, updated, func(sg models.Game) { l.finishSteam(c, action, sg) })
				}
			})
		}
	})
}

// closeSteam 对应 handleCloseSteamModal。
func (l *Launcher) closeSteam() {
	l.steam.open = false
	l.steam.action = steamNone
}

// Modals 构建启动相关的弹窗（SteamImportModal），详情页在最后调用。
func (l *Launcher) Modals(c *ui.Context, t *theme.Theme, p panel.Game) {
	l.latest = p
	if p.Game == nil {
		return
	}
	if l.steam.open && l.steam.gameID != p.Game.ID { // 已换到另一个游戏：原版的弹窗随页面卸载
		l.closeSteam()
	}
	l.steamModal(c, t, *p.Game)
}
