// Package settings 是设置页，对应原版 routes/settings.tsx 以及 store.ts 里设置页用到的 fetchConfig、setDraftConfig、
// saveDraftConfig、patchLiveConfig：各分区的面板改草稿，停止修改 250ms 后整份保存（UpdateAppConfig）。
// 面板在 components/panel 下每个一个子包，页面只负责布局、草稿与保存。
package settings

import (
	"log"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/ai"
	"lunabox/desktop-mygo/ui/components/panel/appdata"
	"lunabox/desktop-mygo/ui/components/panel/autobackup"
	"lunabox/desktop-mygo/ui/components/panel/background"
	"lunabox/desktop-mygo/ui/components/panel/basic"
	"lunabox/desktop-mygo/ui/components/panel/cloudbackup"
	"lunabox/desktop-mygo/ui/components/panel/dbbackup"
	"lunabox/desktop-mygo/ui/components/panel/fullbackup"
	"lunabox/desktop-mygo/ui/components/panel/gamesettings"
	"lunabox/desktop-mygo/ui/components/panel/metadata"
	"lunabox/desktop-mygo/ui/components/panel/portable"
	"lunabox/desktop-mygo/ui/components/panel/proxy"
	"lunabox/desktop-mygo/ui/components/panel/update"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// saveDelay 是草稿停止修改后自动保存的延迟（routes/settings.tsx 的 setTimeout 250ms）。
const saveDelay = 250 * time.Millisecond

// sectionID 是分区，顺序同原版。
type sectionID int

const (
	sectionBasic sectionID = iota
	sectionMetadata
	sectionAppearance
	sectionGame
	sectionProxy
	sectionCloudBackup
	sectionAutoBackup
	sectionAI
	sectionDBBackup
	sectionFullDataBackup
	sectionUpdate
	sectionAppData
	sectionPortableSetup
	sectionCount
)

// panels 是各面板跨帧保留的状态。原版每次进入设置页都重新挂载面板；这里在页面的整个生命周期里保留
// （面板订阅的服务事件没有退订），进入页面时只重置分区的展开状态、账号卡片的展开与草稿。
type panels struct {
	basic       basic.State
	metadata    metadata.State
	background  background.State
	game        gamesettings.State
	proxy       proxy.State
	cloudBackup cloudbackup.State
	autoBackup  autobackup.State
	ai          ai.State
	dbBackup    dbbackup.State
	fullBackup  fullbackup.State
	update      update.State
	appData     appdata.State
	portable    portable.State
}

// Page 是设置页的状态。
type Page struct {
	env *env.Env

	draft     appconf.AppConfig // draftConfig
	loaded    bool              // 读到了配置（原版 config 与 draftConfig 都不为空）
	dirty     bool              // 草稿与生效的配置不同，等待保存
	changedAt time.Time         // 最后一次修改草稿的时间；为零时下一次构建取 c.Now()
	failed    func() string     // 待显示的提示（显示时按当前语言取）

	version        map[string]string // GetVersionInfo
	versionLoading bool
	gen            int

	open    [sectionCount]bool // 各分区是否展开
	handles [sectionCount]ui.Handle
	scroll  components.ScrollBar
	panels  panels

	debug       string    // 截图调试：collapsed 或分区的 key，见 SetDebugState
	debugScroll bool      // 还要把 debug 指定的分区滚到顶部
	debugUntil  time.Time // 对齐到这个时间为止
}

// New 创建页面；配置在 Open 时读取。
func New(e *env.Env) *Page { return &Page{env: e} }

// Open 进入页面：与原版重新挂载 SettingsPage 一致，重新读取配置（fetchConfig）与版本信息，分区回到默认的展开状态
// （只有基础设置展开），账号卡片回到等宽，滚动回到顶部。
func (p *Page) Open() {
	p.open = [sectionCount]bool{sectionBasic: true}
	p.panels.basic.ResetAccounts()
	p.scroll = components.ScrollBar{}
	if p.debug != "" {
		p.open = [sectionCount]bool{}
		for _, sec := range p.sectionList(true) { // 版本信息还没读到，不按构建模式过滤
			if sec.key == p.debug {
				p.open[sec.id], p.debugScroll = true, true
			}
		}
	}
	p.fetchConfig()
	p.loadVersion()
}

// SetDebugState 让截图直接进入某个状态：collapsed（全部收起），或分区的 key（basic、metadata、appearance、game、
// proxy、cloud-backup、auto-backup、ai、db-backup、full-data-backup、update、app-data、portable-setup）——只展开它，
// 并把它滚到主内容区顶部下方 24px 处（tools/parity 的 settings-* 场景在原版上做同样的滚动）。
func (p *Page) SetDebugState(state string) { p.debug = state }

// Flush 在离开页面前补存还没保存的草稿（原版卸载时取消了计时器，草稿留在 store 里，下次进入设置页时被
// resetDraftConfig 丢掉；这里直接保存，不丢修改），并取消进行中的 Umbra 授权（云配置面板卸载时的 effect）。
func (p *Page) Flush() {
	if p.dirty {
		p.save()
	}
	cloudbackup.Leave(&p.panels.cloudBackup, p.props())
}

// Background 由 App 每一帧调用（不论当前在哪个页面）：数据库备份与全量备份面板在后台完成的提示、安排恢复后的退出。
func (p *Page) Background(c *ui.Context) {
	props := p.props()
	dbbackup.Background(c, &p.panels.dbBackup, props)
	fullbackup.Background(c, &p.panels.fullBackup, props)
}

// ApplyCloudSyncStatus 对应 store 的 applyCloudSyncStatus：云同步的结果由服务写进生效的配置，草稿里的同步结果
// 也跟着更新，免得之后保存草稿时把它覆盖回旧值。
func (p *Page) ApplyCloudSyncStatus(st vo.CloudSyncStatus) {
	if !p.loaded {
		return
	}
	p.draft.LastCloudSyncTime, p.draft.LastCloudSyncStatus, p.draft.LastCloudSyncError = st.LastSyncTime, st.LastSyncStatus, st.LastSyncError
}

// ApplyZoom 对应在设置页以外调用的 patchLiveConfig({ window_zoom_factor })（缩放快捷键）：生效的配置已经保存，
// 草稿里的缩放也跟着更新，免得之后保存草稿时把它覆盖回旧值。
func (p *Page) ApplyZoom(zoom float64) {
	if !p.loaded {
		return
	}
	p.draft.WindowZoomFactor = zoom
}

// Reload 对应 *:auth-status-changed 之后的 refreshConfig（store 的 fetchConfig）：账号服务把令牌直接写进生效的配置，
// 草稿重置为它，免得之后保存草稿时把令牌覆盖回旧值。还没进入过设置页时不用读，Open 会读。
func (p *Page) Reload() {
	if p.loaded {
		p.fetchConfig()
	}
}

// Loading 报告页面是否还在加载（截图用）。
func (p *Page) Loading() bool { return p.versionLoading }

// fetchConfig 对应 store 的 fetchConfig：读取配置，草稿重置为它。
func (p *Page) fetchConfig() {
	p.dirty = false
	if p.env.Svc == nil {
		cfg := p.env.Config()
		p.loaded = cfg != nil
		if cfg != nil {
			p.draft = *cfg
		}
		return
	}
	cfg, err := p.env.Svc.AppConfig.GetAppConfig()
	if err != nil {
		log.Printf("settings: failed to fetch config: %v", err)
		p.loaded = false
		return
	}
	p.draft, p.loaded = p.withSidebarState(cfg), true
}

// loadVersion 对应 GetVersionInfo。
func (p *Page) loadVersion() {
	if p.env.Svc == nil {
		return
	}
	p.gen++
	gen := p.gen
	p.versionLoading = true
	p.env.Async(func() func() {
		info := p.env.Svc.Version.GetVersionInfo()
		return func() {
			if gen == p.gen {
				p.version, p.versionLoading = info, false
			}
		}
	})
}

// withSidebarState 对应 store 的 withSidebarState：sidebar_open 取侧边栏当前的状态。
func (p *Page) withSidebarState(cfg appconf.AppConfig) appconf.AppConfig {
	if p.env.SidebarOpen != nil {
		cfg.SidebarOpen = p.env.SidebarOpen()
	}
	return cfg
}

// props 是传给各面板的 panel.Settings。
func (p *Page) props() panel.Settings {
	return panel.Settings{
		Env:       p.env,
		Config:    &p.draft,
		Change:    p.change,
		PatchLive: p.patchLive,
		Refresh:   p.fetchConfig,
	}
}

// change 对应 setDraftConfig：改草稿，250ms 后保存。
func (p *Page) change(fn func(cfg *appconf.AppConfig)) {
	fn(&p.draft)
	p.dirty = true
	p.changedAt = time.Time{}
}

// patchLive 对应 patchLiveConfig：草稿与生效的配置同时打上补丁并立即保存；失败时恢复两者。
// 草稿里其余还没保存的修改留在草稿里，照常在 250ms 后保存。
func (p *Page) patchLive(fn func(cfg *appconf.AppConfig)) {
	cur := p.env.Config()
	if cur == nil || p.env.Svc == nil {
		return
	}
	prevDraft := p.draft
	next := p.withSidebarState(*cur)
	fn(&next)
	fn(&p.draft)
	p.draft.SidebarOpen = next.SidebarOpen
	if !p.update(*cur, next) {
		p.draft = prevDraft
	}
}

// tick 在草稿停止修改 250ms 后保存（routes/settings.tsx 里依赖 draftConfig 的 effect）。
func (p *Page) tick(now time.Time) (wait time.Duration) {
	if !p.dirty {
		return 0
	}
	if p.changedAt.IsZero() {
		p.changedAt = now
	}
	if wait := saveDelay - now.Sub(p.changedAt); wait > 0 {
		return wait
	}
	p.save()
	return 0
}

// save 对应 saveDraftConfig：保存整份草稿。原版保存失败只打日志，草稿保持原样；这里还提示并把草稿恢复成生效的配置，
// 否则一项保存不了的修改（如原型不支持的开机自启）会让之后的每次保存都失败。
func (p *Page) save() {
	p.dirty = false
	cur := p.env.Config()
	if cur == nil || p.env.Svc == nil {
		return
	}
	next := p.withSidebarState(p.draft)
	if !p.update(*cur, next) {
		p.draft = p.withSidebarState(*cur)
		return
	}
	p.draft = next
}

// update 保存 next 并让它生效，报告是否成功。UpdateAppConfig 只是写一个 JSON 文件，并把结果写进各服务共用的
// *AppConfig；在主线程上调用，界面每帧读这份配置时不会与它竞争。
func (p *Page) update(prev, next appconf.AppConfig) bool {
	if err := p.env.Svc.AppConfig.UpdateAppConfig(next); err != nil {
		log.Printf("settings: failed to save config: %v", err)
		p.failed = locales.NativeSettingsSaveFailed
		return false
	}
	if p.env.ConfigSaved != nil {
		if cur := p.env.Config(); cur != nil {
			next = *cur // UpdateAppConfig 会规范化部分字段
		}
		p.env.ConfigSaved(prev, next)
	}
	return true
}
