package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/appruntime"
	"lunabox/backend/common/hooks"
	"lunabox/backend/service"
	"lunabox/backend/utils/apputils"
	"lunabox/backend/version"
)

// Dependencies 是插件管理器依赖的宿主服务集合。
type Dependencies struct {
	Config        *appconf.AppConfig
	ConfigService *service.ConfigService
	Game          *service.GameService
	Tag           *service.TagService
	Category      *service.CategoryService
	Start         *service.StartService
	Download      *service.DownloadService
	Import        *service.ImportService
	AI            *service.AiService
	Hikarinagi    *service.HikarinagiService
	Session       *service.SessionService
	Progress      *service.GameProgressService
	Review        *service.GameReviewService
	Runtime       appruntime.Runtime
	// DataDir 为空时默认使用 apputils.GetDataDir()。
	DataDir string
	// HostVersion 为空时默认使用 version.GetVersion()。
	HostVersion string
}

type managedPlugin struct {
	id        string
	dir       string
	dataDir   string
	isDev     bool
	manifest  Manifest
	status    PluginStatus
	lastError string
	enabled   bool
	runtime   *Plugin
}

// Manager 负责插件的发现、生命周期（加载/卸载/热重载）、状态查询、排序、安装与卸载，
// 并持有 HookBus 与 UIRegistry。
type Manager struct {
	ctx         context.Context
	deps        Dependencies
	dataDir     string
	hostVersion string

	hooks *HookBus
	ui    *UIRegistry

	mu      sync.RWMutex
	plugins map[string]*managedPlugin
	order   []string

	mainWindowVisible func() bool
	audioPlayer       AudioPlayer

	started atomic.Bool // StartAsync 已加载完启动时启用的插件

	subMu      sync.RWMutex
	nextSubID  uint64
	stateSubs  map[uint64]func()
	devDirSubs map[uint64]func([]string)
}

// NewManager 构造插件管理器，必须在 bootstrap 中调用 Init 注入服务。
func NewManager() *Manager {
	m := &Manager{
		ctx:        context.Background(),
		plugins:    make(map[string]*managedPlugin),
		stateSubs:  make(map[uint64]func()),
		devDirSubs: make(map[uint64]func([]string)),
	}
	m.hooks = newHookBus(m)
	m.ui = newUIRegistry()
	return m
}

// Init 注入宿主依赖与运行上下文。
func (m *Manager) Init(ctx context.Context, deps Dependencies) {
	if ctx != nil {
		m.ctx = ctx
	}
	if deps.Runtime == nil {
		deps.Runtime = appruntime.Unavailable()
	}
	m.deps = deps
	if strings.TrimSpace(deps.DataDir) != "" {
		m.dataDir = deps.DataDir
	} else if dir, err := apputils.GetDataDir(); err == nil {
		m.dataDir = dir
	}
	if strings.TrimSpace(deps.HostVersion) != "" {
		m.hostVersion = deps.HostVersion
	} else {
		m.hostVersion = version.GetVersion()
	}
}

// SetRuntime 更新桌面运行时接口（在 bootstrap.Attach 时调用）。
func (m *Manager) SetRuntime(rt appruntime.Runtime) {
	if rt != nil {
		m.deps.Runtime = rt
	}
}

// SetMainWindowVisibleProvider 注入主窗口可见性查询（供 ctx.window.main.isVisible 使用）。
func (m *Manager) SetMainWindowVisibleProvider(fn func() bool) {
	m.mainWindowVisible = fn
}

// SetAudioPlayer 注入桌面宿主音频播放器实现（由 apps/desktop/host 基于 mygo/audio 提供）。
func (m *Manager) SetAudioPlayer(player AudioPlayer) {
	m.mu.Lock()
	m.audioPlayer = player
	m.mu.Unlock()
}

// HookBus 返回供业务 service 注入的 hooks.Bus 实例。
func (m *Manager) HookBus() hooks.Bus { return m.hooks }

// UI 返回插槽/主题/装饰/子窗口注册表。
func (m *Manager) UI() *UIRegistry { return m.ui }

func (m *Manager) PluginsDir() string {
	return filepath.Join(m.dataDir, "plugins")
}

func (m *Manager) PluginsDataDir() string {
	return filepath.Join(m.dataDir, "plugins-data")
}

func (m *Manager) currentLanguage() string {
	if m.deps.Config != nil && strings.TrimSpace(m.deps.Config.Language) != "" {
		return m.deps.Config.Language
	}
	return "zh-CN"
}

// StartAsync 在后台发现并加载所有已启用的插件（不阻塞首帧），加载完毕后触发 app/ready 钩子。
func (m *Manager) StartAsync() {
	go func() {
		m.Discover()
		m.loadEnabledPlugins()
		m.started.Store(true)
		m.ui.notifyChange() // 让界面在插件加载完后重新读取主题覆盖与 Started
		m.hooks.Emit(hooks.EventAppReady, nil)
		m.notifyDevDirs()
	}()
}

// Started 报告启动时启用的插件是否已全部加载：之前读到的主题覆盖等注册还不完整。
func (m *Manager) Started() bool { return m.started.Load() }

// Discover 扫描 <DataDir>/plugins/* 与 AppConfig.PluginDevPaths 中的插件。
func (m *Manager) Discover() {
	type found struct {
		dir   string
		isDev bool
	}
	var candidates []found

	if pluginsDir := m.PluginsDir(); pluginsDir != "" {
		if entries, err := os.ReadDir(pluginsDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					candidates = append(candidates, found{dir: filepath.Join(pluginsDir, e.Name()), isDev: false})
				}
			}
		}
	}
	if m.deps.Config != nil {
		for _, devPath := range m.deps.Config.PluginDevPaths {
			p := strings.TrimSpace(devPath)
			if p != "" {
				candidates = append(candidates, found{dir: p, isDev: true})
			}
		}
	}

	cfgEntries := map[string]bool{}
	cfgOrder := map[string]int{}
	if m.deps.Config != nil {
		for i, entry := range m.deps.Config.Plugins {
			id := strings.ToLower(strings.TrimSpace(entry.ID))
			cfgEntries[id] = entry.Enabled
			cfgOrder[id] = i
		}
	}

	m.mu.Lock()
	discoveredIDs := make(map[string]struct{})
	for _, cand := range candidates {
		manifest, err := LoadManifest(cand.dir)
		if err != nil {
			// 目录无效但仍是开发目录时保留错误占位
			if cand.isDev {
				fallbackID := strings.ToLower(filepath.Base(cand.dir))
				if fallbackID == "" {
					continue
				}
				discoveredIDs[fallbackID] = struct{}{}
				m.plugins[fallbackID] = &managedPlugin{
					id:        fallbackID,
					dir:       cand.dir,
					dataDir:   filepath.Join(m.PluginsDataDir(), fallbackID),
					isDev:     true,
					manifest:  Manifest{ID: fallbackID, Name: map[string]string{"en-US": fallbackID}},
					status:    StatusError,
					lastError: err.Error(),
				}
			}
			continue
		}
		id := manifest.ID
		discoveredIDs[id] = struct{}{}
		enabled, hasCfg := cfgEntries[id]
		if !hasCfg && cand.isDev {
			// 开发目录新增插件默认启用，方便开发体验
			enabled = true
		}

		if existing, ok := m.plugins[id]; ok {
			existing.dir = cand.dir
			existing.isDev = cand.isDev
			existing.manifest = manifest
			existing.enabled = enabled
		} else {
			mp := &managedPlugin{
				id:       id,
				dir:      cand.dir,
				dataDir:  filepath.Join(m.PluginsDataDir(), id),
				isDev:    cand.isDev,
				manifest: manifest,
				status:   StatusDiscovered,
				enabled:  enabled,
			}
			if verr := manifest.Validate(m.hostVersion); verr != nil {
				mp.status = StatusError
				mp.lastError = verr.Error()
			} else if !enabled {
				mp.status = StatusDisabled
			}
			m.plugins[id] = mp
		}
	}

	// 按配置顺序重排 m.order
	order := make([]string, 0, len(m.plugins))
	for id := range m.plugins {
		order = append(order, id)
	}
	slices.SortFunc(order, func(a, b string) int {
		oa, oka := cfgOrder[a]
		ob, okb := cfgOrder[b]
		if oka && okb {
			if oa < ob {
				return -1
			}
			if oa > ob {
				return 1
			}
		}
		if oka != okb {
			if oka {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	})
	m.order = order
	m.mu.Unlock()
	m.notifyState()
}

func (m *Manager) loadEnabledPlugins() {
	m.mu.RLock()
	var ids []string
	for _, id := range m.order {
		if mp := m.plugins[id]; mp != nil && mp.enabled && mp.runtime == nil {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range ids {
		if err := m.loadPlugin(id); err != nil {
			applog.LogErrorf(m.ctx, "[plugin:%s] failed to load: %v", id, err)
		}
	}
}

func (m *Manager) loadPlugin(id string) error {
	m.mu.Lock()
	mp, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("未找到插件 %s", id)
	}
	if mp.runtime != nil {
		m.mu.Unlock()
		return nil
	}
	manifest, err := LoadManifest(mp.dir)
	if err != nil {
		mp.status = StatusError
		mp.lastError = err.Error()
		m.mu.Unlock()
		m.notifyState()
		return err
	}
	mp.manifest = manifest
	if err := manifest.Validate(m.hostVersion); err != nil {
		mp.status = StatusError
		mp.lastError = err.Error()
		m.mu.Unlock()
		m.notifyState()
		return err
	}
	mp.status = StatusLoading
	mp.lastError = ""
	dir, dataDir, isDev := mp.dir, mp.dataDir, mp.isDev
	m.mu.Unlock()
	m.notifyState()

	rtPlugin, err := startPluginRuntime(m, manifest, dir, dataDir, isDev)
	m.mu.Lock()
	if cur, exists := m.plugins[id]; exists {
		if err != nil {
			cur.status = StatusError
			cur.lastError = err.Error()
			cur.runtime = nil
		} else {
			cur.status = StatusActive
			cur.lastError = ""
			cur.runtime = rtPlugin
		}
	} else if rtPlugin != nil {
		rtPlugin.Dispose()
	}
	m.mu.Unlock()
	m.notifyState()
	return err
}

func (m *Manager) unloadPlugin(id string) {
	m.mu.Lock()
	mp, ok := m.plugins[id]
	if !ok || mp.runtime == nil {
		if ok && mp.status != StatusError {
			mp.status = StatusDisabled
		}
		m.mu.Unlock()
		m.notifyState()
		return
	}
	rt := mp.runtime
	mp.runtime = nil
	mp.status = StatusDisposing
	m.mu.Unlock()
	m.notifyState()

	rt.Dispose()

	m.mu.Lock()
	if cur, exists := m.plugins[id]; exists {
		cur.status = StatusDisabled
	}
	m.mu.Unlock()
	m.notifyState()
}

// Enable 启用并加载指定插件。
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	mp, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("未找到插件 %s", id)
	}
	mp.enabled = true
	m.mu.Unlock()
	m.persistPluginConfig()
	return m.loadPlugin(id)
}

// Disable 停用并卸载指定插件。
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	mp, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("未找到插件 %s", id)
	}
	mp.enabled = false
	m.mu.Unlock()
	m.unloadPlugin(id)
	m.persistPluginConfig()
	return nil
}

// Reload 热重载插件（卸载 + 重新加载）；失败时进入 error 状态，不回退旧版。
func (m *Manager) Reload(id string) error {
	m.unloadPlugin(id)
	m.mu.RLock()
	mp := m.plugins[id]
	enabled := mp != nil && mp.enabled
	m.mu.RUnlock()
	if !enabled {
		return nil
	}
	return m.loadPlugin(id)
}

// ReloadByDir 按开发目录路径触发热重载（供 host 文件监听防抖回调使用）。
func (m *Manager) ReloadByDir(dir string) {
	clean := filepath.Clean(dir)
	m.mu.RLock()
	var targetID string
	for id, mp := range m.plugins {
		if strings.EqualFold(filepath.Clean(mp.dir), clean) {
			targetID = id
			break
		}
	}
	m.mu.RUnlock()
	if targetID != "" {
		if err := m.Reload(targetID); err != nil {
			applog.LogErrorf(m.ctx, "[plugin:%s] hot reload failed: %v", targetID, err)
		}
	}
}

// Reorder 更新插件执行顺序并持久化到 AppConfig。
func (m *Manager) Reorder(ids []string) error {
	m.mu.Lock()
	seen := make(map[string]struct{}, len(ids))
	nextOrder := make([]string, 0, len(m.order))
	for _, id := range ids {
		if _, exists := m.plugins[id]; exists {
			if _, dup := seen[id]; !dup {
				seen[id] = struct{}{}
				nextOrder = append(nextOrder, id)
			}
		}
	}
	for _, id := range m.order {
		if _, dup := seen[id]; !dup {
			nextOrder = append(nextOrder, id)
		}
	}
	m.order = nextOrder
	m.mu.Unlock()
	m.persistPluginConfig()
	m.notifyState()
	return nil
}

// AddDevPath 添加开发插件目录并立即发现与加载。
func (m *Manager) AddDevPath(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("开发目录不能为空")
	}
	if _, err := LoadManifest(dir); err != nil {
		return err
	}
	if m.deps.Config != nil {
		if !slices.Contains(m.deps.Config.PluginDevPaths, dir) {
			m.deps.Config.PluginDevPaths = append(m.deps.Config.PluginDevPaths, dir)
		}
	}
	m.Discover()
	m.persistPluginConfig()
	m.loadEnabledPlugins()
	m.notifyDevDirs()
	return nil
}

// RemoveDevPath 移除开发插件目录并卸载对应插件。
func (m *Manager) RemoveDevPath(dir string) error {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if m.deps.Config != nil {
		m.deps.Config.PluginDevPaths = slices.DeleteFunc(m.deps.Config.PluginDevPaths, func(p string) bool {
			return strings.EqualFold(filepath.Clean(p), clean)
		})
	}
	var removeIDs []string
	m.mu.RLock()
	for id, mp := range m.plugins {
		if mp.isDev && strings.EqualFold(filepath.Clean(mp.dir), clean) {
			removeIDs = append(removeIDs, id)
		}
	}
	m.mu.RUnlock()
	for _, id := range removeIDs {
		m.unloadPlugin(id)
		m.mu.Lock()
		delete(m.plugins, id)
		m.order = slices.DeleteFunc(m.order, func(x string) bool { return x == id })
		m.mu.Unlock()
	}
	m.persistPluginConfig()
	m.notifyState()
	m.notifyDevDirs()
	return nil
}

// DevPaths 返回当前配置的所有开发插件目录。
func (m *Manager) DevPaths() []string {
	if m.deps.Config == nil {
		return nil
	}
	return slices.Clone(m.deps.Config.PluginDevPaths)
}

// List 返回全部插件的状态快照（按当前顺序）。
func (m *Manager) List() []PluginInfo {
	lang := m.currentLanguage()
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PluginInfo, 0, len(m.order))
	for _, id := range m.order {
		mp := m.plugins[id]
		if mp == nil {
			continue
		}
		lastErr := mp.lastError
		if mp.runtime != nil && lastErr == "" {
			lastErr = mp.runtime.LastError()
		}
		iconRaw := strings.TrimSpace(mp.manifest.Icon)
		iconPath := ""
		if iconRaw != "" && !strings.Contains(iconRaw, "..") && !filepath.IsAbs(iconRaw) {
			cand := filepath.Join(mp.dir, filepath.FromSlash(iconRaw))
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				iconPath = cand
			}
		}
		out = append(out, PluginInfo{
			ID:          mp.id,
			Version:     mp.manifest.Version,
			APIVersion:  mp.manifest.APIVersion,
			Engines:     strings.TrimSpace(mp.manifest.Engines.LunaBox),
			Name:        mp.manifest.DisplayName(lang),
			Names:       mp.manifest.Name,
			Description: mp.manifest.DisplayDescription(lang),
			Author:      strings.TrimSpace(mp.manifest.Author),
			Homepage:    strings.TrimSpace(mp.manifest.Homepage),
			Repository:  strings.TrimSpace(mp.manifest.Repository),
			License:     strings.TrimSpace(mp.manifest.License),
			Icon:        iconRaw,
			IconBg:      strings.TrimSpace(mp.manifest.IconBg),
			IconPath:    iconPath,
			Keywords:    slices.Clone(mp.manifest.Keywords),
			Dir:         mp.dir,
			DataDir:     mp.dataDir,
			IsDev:       mp.isDev,
			Enabled:     mp.enabled,
			Status:      mp.status,
			LastError:   lastErr,
		})
	}
	return out
}

// GetConfigValues 获取指定插件的 ctx.config 当前配置值（供设置页自动表单渲染）。
func (m *Manager) GetConfigValues(pluginID string) map[string]any {
	p := m.activePlugin(pluginID)
	if p == nil || p.configMgr == nil {
		return map[string]any{}
	}
	return p.configMgr.GetValues()
}

// SetConfigValues 更新指定插件的 ctx.config 配置值并触发插件 onChange 回调。
func (m *Manager) SetConfigValues(pluginID string, values map[string]any) error {
	p := m.activePlugin(pluginID)
	if p == nil || p.configMgr == nil {
		return fmt.Errorf("插件 %s 未运行", pluginID)
	}
	return p.configMgr.SetValues(values)
}

// MountSlot 挂载一个插槽实例。
func (m *Manager) MountSlot(instanceID, pluginID, slotName, regID string, props any) {
	if p := m.activePlugin(pluginID); p != nil {
		p.MountSlotInstance(instanceID, slotName, regID, props)
	}
}

// UpdateSlot 以新的 props 重渲染已挂载的插槽实例（保留插件内的 hook 状态）。
func (m *Manager) UpdateSlot(instanceID, pluginID, slotName, regID string, props any) {
	if p := m.activePlugin(pluginID); p != nil {
		p.UpdateSlotInstance(instanceID, slotName, regID, props)
	}
}

// UnmountSlot 卸载一个插槽实例。
func (m *Manager) UnmountSlot(instanceID, pluginID string) {
	if p := m.activePlugin(pluginID); p != nil {
		p.UnmountSlotInstance(instanceID)
	}
}

// DispatchEvent 投递界面事件给对应插件。
func (m *Manager) DispatchEvent(ev UIEvent) {
	if p := m.activePlugin(ev.PluginID); p != nil {
		p.DispatchUIEvent(ev)
	}
}

// ResolveDialog 把用户对 ctx.ui.confirm / select 对话框的回答交还插件（result 为 JSON；取消时传 nil）。
func (m *Manager) ResolveDialog(pluginID, dialogID string, result json.RawMessage) {
	if p := m.activePlugin(pluginID); p != nil {
		p.resolveDialog(dialogID, result)
	}
}

// InvokeAction 触发非描述树插槽的点击回调（topbar.item、library.add_menu、library.toolbar、game.actions）。
func (m *Manager) InvokeAction(pluginID, slotName, regID string, payload any) {
	if p := m.activePlugin(pluginID); p != nil {
		p.InvokeSlotAction(slotName, regID, payload)
	}
}

// CheckActionWhen 检查 game.actions 的 when(game) 条件。
func (m *Manager) CheckActionWhen(pluginID, slotName, regID string, payload any) bool {
	if p := m.activePlugin(pluginID); p != nil {
		return p.CheckSlotWhen(slotName, regID, payload)
	}
	return false
}

// NotifyWindowEvent 把独立悬浮窗的原生事件转发回插件。
func (m *Manager) NotifyWindowEvent(pluginID, windowID, event string, payload map[string]any) {
	if p := m.activePlugin(pluginID); p != nil {
		p.NotifyWindowEvent(windowID, event, payload)
	}
}

// NotifyLanguageChanged 在宿主语言切换时通知所有运行中的插件。
func (m *Manager) NotifyLanguageChanged(lang string) {
	m.mu.RLock()
	actives := make([]*Plugin, 0, len(m.plugins))
	for _, mp := range m.plugins {
		if mp.runtime != nil {
			actives = append(actives, mp.runtime)
		}
	}
	m.mu.RUnlock()
	for _, p := range actives {
		if p.i18n != nil {
			p.i18n.notifyLanguageChange(lang)
		}
	}
}

// OpenPluginDir 在系统文件管理器中打开插件目录（空 ID 时打开 plugins 根目录）。
func (m *Manager) OpenPluginDir(pluginID string) error {
	target := m.PluginsDir()
	if strings.TrimSpace(pluginID) != "" {
		m.mu.RLock()
		if mp, ok := m.plugins[pluginID]; ok && mp.dir != "" {
			target = mp.dir
		}
		m.mu.RUnlock()
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return apputils.OpenDirectory(target)
}

// SelectPluginZip 打开系统文件选择器选择 .zip 插件包。
func (m *Manager) SelectPluginZip() (string, error) {
	return m.deps.Runtime.OpenFile(appruntime.OpenDialogOptions{
		Title: "选择插件安装包 (.zip)",
		Filters: []appruntime.FileFilter{
			{DisplayName: "LunaBox 插件包 (*.zip)", Pattern: "*.zip"},
		},
	})
}

// SelectDevDir 打开系统目录选择器添加开发插件目录。
func (m *Manager) SelectDevDir() (string, error) {
	return m.deps.Runtime.OpenDirectory(appruntime.OpenDialogOptions{
		Title: "选择插件开发目录",
	})
}

// Shutdown 在应用退出时卸载所有插件，整体限时 2s（见 docs/plugin-system.md §4）。
func (m *Manager) Shutdown() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.mu.RLock()
		ids := slices.Clone(m.order)
		m.mu.RUnlock()
		for i := len(ids) - 1; i >= 0; i-- {
			m.unloadPlugin(ids[i])
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		applog.LogWarningf(m.ctx, "plugin manager shutdown timed out after 2s; abandoning remaining runtimes")
	}
}

// SubscribeState 订阅插件列表与状态变化，返回取消订阅函数。
func (m *Manager) SubscribeState(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	m.subMu.Lock()
	m.nextSubID++
	id := m.nextSubID
	m.stateSubs[id] = fn
	m.subMu.Unlock()
	return func() {
		m.subMu.Lock()
		delete(m.stateSubs, id)
		m.subMu.Unlock()
	}
}

// SubscribeDevDirs 订阅开发插件目录变更（供 host 维护 mygo.WatchFiles）。
func (m *Manager) SubscribeDevDirs(fn func([]string)) func() {
	if fn == nil {
		return func() {}
	}
	m.subMu.Lock()
	m.nextSubID++
	id := m.nextSubID
	m.devDirSubs[id] = fn
	m.subMu.Unlock()
	return func() {
		m.subMu.Lock()
		delete(m.devDirSubs, id)
		m.subMu.Unlock()
	}
}

func (m *Manager) notifyState() {
	m.subMu.RLock()
	cbs := make([]func(), 0, len(m.stateSubs))
	for _, fn := range m.stateSubs {
		cbs = append(cbs, fn)
	}
	m.subMu.RUnlock()
	for _, fn := range cbs {
		fn()
	}
}

func (m *Manager) notifyDevDirs() {
	dirs := m.DevPaths()
	m.subMu.RLock()
	cbs := make([]func([]string), 0, len(m.devDirSubs))
	for _, fn := range m.devDirSubs {
		cbs = append(cbs, fn)
	}
	m.subMu.RUnlock()
	for _, fn := range cbs {
		fn(dirs)
	}
}

func (m *Manager) activePlugin(id string) *Plugin {
	m.mu.RLock()
	mp := m.plugins[id]
	m.mu.RUnlock()
	if mp == nil {
		return nil
	}
	return mp.runtime
}

func (m *Manager) pluginOrderMap() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]int, len(m.order))
	for i, id := range m.order {
		out[id] = i
	}
	return out
}

func (m *Manager) persistPluginConfig() {
	if m.deps.ConfigService == nil {
		return
	}
	cfg, err := m.deps.ConfigService.GetAppConfig()
	if err != nil {
		return
	}
	m.mu.RLock()
	entries := make([]appconf.PluginEntry, 0, len(m.order))
	for _, id := range m.order {
		if mp := m.plugins[id]; mp != nil {
			entries = append(entries, appconf.PluginEntry{ID: mp.id, Enabled: mp.enabled})
		}
	}
	m.mu.RUnlock()
	cfg.Plugins = entries
	if m.deps.Config != nil {
		cfg.PluginDevPaths = slices.Clone(m.deps.Config.PluginDevPaths)
	}
	if err := m.deps.ConfigService.UpdateAppConfig(cfg); err != nil {
		applog.LogWarningf(m.ctx, "failed to persist plugin config: %v", err)
	}
}

// MarshalJSONProps 辅助函数：把任意 Go 结构转成 json.RawMessage。
func MarshalJSONProps(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}
