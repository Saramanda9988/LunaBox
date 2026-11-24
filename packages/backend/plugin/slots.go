package plugin

import (
	"encoding/json"
	"slices"
	"sync"
)

// 插槽名常量（见 docs/plugin-system.md §10）。
const (
	SlotSidebarPage      = "sidebar.page"
	SlotWorkspaceDrawer  = "workspace.drawer"
	SlotTopbarItem       = "topbar.item"
	SlotLibraryAddMenu   = "library.add_menu"
	SlotLibraryToolbar   = "library.toolbar"
	SlotGameDetailTab    = "game.detail.tab"
	SlotGameDetailHero   = "game.detail.hero"
	SlotGameDetailHeader = "game.detail.header"
	SlotGameDetailAside  = "game.detail.aside"
	SlotGameActions      = "game.actions"
	SlotDownloadsHeader  = "downloads.header"
	SlotAppDecoration    = "app.decoration"
	SlotSettingsSection  = "settings.section"
)

// SlotRegistration 描述一个由插件注册到指定插槽的条目。
type SlotRegistration struct {
	SlotName     string `json:"slot_name"`
	PluginID     string `json:"plugin_id"`
	PluginName   string `json:"plugin_name"`
	ID           string `json:"id"`
	Title        string `json:"title,omitempty"`
	Icon         string `json:"icon,omitempty"`
	Group        string `json:"group,omitempty"`         // sidebar.page: "main" | "bottom"
	Badge        string `json:"badge,omitempty"`
	Mode         string   `json:"mode,omitempty"`          // library.toolbar: "normal" | "batch" | "both"
	DefaultWidth int      `json:"default_width,omitempty"` // workspace.drawer
	Order        int      `json:"order,omitempty"`         // game.detail.tab 排序权重（>0 时靠前）
	HideTabs     []string `json:"hide_tabs,omitempty"`     // game.detail.* 隐藏的内置标签页 key（如 "progress"）
	HasRender    bool     `json:"has_render"`
	HasOnPress   bool   `json:"has_on_press"`
	HasRun       bool   `json:"has_run"`
	HasWhen      bool   `json:"has_when"`
	// 自动生成的设置表单 schema（由 ctx.config.define 注册）
	ConfigSchema *ConfigSchema `json:"config_schema,omitempty"`
}

// GlassOverride 是主题插件可选的毛玻璃参数覆盖。
type GlassOverride struct {
	Enabled         *bool    `json:"enabled,omitempty"`
	Opacity         *float64 `json:"opacity,omitempty"`
	Blur            *int     `json:"blur,omitempty"`
	BackgroundImage string   `json:"backgroundImage,omitempty"`
}

// ShapeOverride 描述主题插件对宿主控件圆角的覆盖（单位 DIP，未设置时保留组件自身的圆角）。
type ShapeOverride struct {
	Control *float64 `json:"control,omitempty"` // 详情页工具栏图标按钮、状态按钮、启动按钮等控件
	Panel   *float64 `json:"panel,omitempty"`   // 下拉菜单等浮层面板
}

// ThemeOverride 描述插件通过 ctx.theme.extend 注册的色板覆盖。
type ThemeOverride struct {
	PluginID string            `json:"plugin_id"`
	ID       string            `json:"id"`
	Light    map[string]string `json:"light,omitempty"`
	Dark     map[string]string `json:"dark,omitempty"`
	Glass    *GlassOverride    `json:"glass,omitempty"`
	Shape    *ShapeOverride    `json:"shape,omitempty"`
}

// FontRegistration 描述插件通过 ctx.theme.registerFont 注册的自定义字体。
type FontRegistration struct {
	PluginID string `json:"plugin_id"`
	Family   string `json:"family"`
	FilePath string `json:"file_path"` // 已解析为磁盘绝对路径
	Target   string `json:"target"`    // "ui" | "mono" | ""
}

// DecorationRegistration 描述插件通过 ctx.theme.registerDecoration 或 app.decoration 注册的装饰层。
type DecorationRegistration struct {
	PluginID      string   `json:"plugin_id"`
	ID            string   `json:"id"`
	Layer         string   `json:"layer"`          // "background" | "foreground"
	Anchor        string   `json:"anchor"`         // "bottom-right" | "bottom-left" | "top-right" | "top-left" | "center"
	OffsetX       float32  `json:"offset_x"`
	OffsetY       float32  `json:"offset_y"`
	Width         float32  `json:"width"`
	Height        float32  `json:"height"`
	Opacity       float32  `json:"opacity"`
	PointerEvents string   `json:"pointer_events"` // "none" | "auto"
	Routes        []string `json:"routes,omitempty"`
	ImagePath     string   `json:"image_path,omitempty"` // 绝对路径或 http URL
	HasRender     bool     `json:"has_render"`
}

// PluginWindowSpec 描述插件通过 ctx.window.open 打开的独立子窗口配置。
type PluginWindowSpec struct {
	PluginID    string `json:"plugin_id"`
	WindowID    string `json:"window_id"`
	InstanceID  string `json:"instance_id"`
	Title       string `json:"title"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	X           *int   `json:"x,omitempty"`
	Y           *int   `json:"y,omitempty"`
	Frameless   bool   `json:"frameless"`
	Transparent bool   `json:"transparent"`
	AlwaysOnTop bool   `json:"always_on_top"`
	SkipTaskbar bool   `json:"skip_taskbar"`
	Resizable   bool   `json:"resizable"`
}

// WindowCommand 是插件向已打开的独立窗口或主窗口发送的控制指令。
type WindowCommand struct {
	Action      string            `json:"action"` // open | close | show | hide | setSize | setPosition | setAlwaysOnTop | main.show | main.focus | main.minimize
	PluginID    string            `json:"plugin_id,omitempty"`
	WindowID    string            `json:"window_id,omitempty"`
	Spec        *PluginWindowSpec `json:"spec,omitempty"`
	Width       int               `json:"width,omitempty"`
	Height      int               `json:"height,omitempty"`
	X           int               `json:"x,omitempty"`
	Y           int               `json:"y,omitempty"`
	AlwaysOnTop bool              `json:"always_on_top,omitempty"`
}

// UIAction 是插件发起的宿主界面操作（toast / navigate / drawer / importModal）。
type UIAction struct {
	Type     string          `json:"type"` // toast | navigate | openDrawer | closeDrawer | openImportModal
	PluginID string          `json:"plugin_id"`
	Message  string          `json:"message,omitempty"`
	Level    string          `json:"level,omitempty"` // info | success | warning | error
	Path     string          `json:"path,omitempty"`
	Params   map[string]any  `json:"params,omitempty"`
	DrawerID string          `json:"drawer_id,omitempty"`
	Props    json.RawMessage `json:"props,omitempty"`
}

// UIRegistry 管理全部插件注册的插槽、主题、字体、装饰层与子窗口状态，供界面只读快照与变更订阅。
type UIRegistry struct {
	mu          sync.RWMutex
	slots       []SlotRegistration
	themes      []ThemeOverride
	fonts       []FontRegistration
	decorations []DecorationRegistration
	windows     map[string]PluginWindowSpec // key: pluginID + "/" + windowID

	nextSubID  uint64
	changeSubs map[uint64]func()
	treeSubs   map[uint64]func(UITreeUpdate)
	actionSubs map[uint64]func(UIAction)
	winCmdSubs map[uint64]func(WindowCommand)
}

func newUIRegistry() *UIRegistry {
	return &UIRegistry{
		windows:    make(map[string]PluginWindowSpec),
		changeSubs: make(map[uint64]func()),
		treeSubs:   make(map[uint64]func(UITreeUpdate)),
		actionSubs: make(map[uint64]func(UIAction)),
		winCmdSubs: make(map[uint64]func(WindowCommand)),
	}
}

func (r *UIRegistry) RegisterSlot(reg SlotRegistration) func() {
	r.mu.Lock()
	r.slots = slices.DeleteFunc(r.slots, func(s SlotRegistration) bool {
		return s.SlotName == reg.SlotName && s.PluginID == reg.PluginID && s.ID == reg.ID
	})
	r.slots = append(r.slots, reg)
	r.mu.Unlock()
	r.notifyChange()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			r.slots = slices.DeleteFunc(r.slots, func(s SlotRegistration) bool {
				return s.SlotName == reg.SlotName && s.PluginID == reg.PluginID && s.ID == reg.ID
			})
			r.mu.Unlock()
			r.notifyChange()
		})
	}
}

func (r *UIRegistry) Slots(slotName string) []SlotRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SlotRegistration, 0)
	for _, s := range r.slots {
		if slotName == "" || s.SlotName == slotName {
			out = append(out, s)
		}
	}
	return out
}

func (r *UIRegistry) RegisterTheme(override ThemeOverride) func() {
	r.mu.Lock()
	r.themes = append(r.themes, override)
	r.mu.Unlock()
	r.notifyChange()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			r.themes = slices.DeleteFunc(r.themes, func(t ThemeOverride) bool {
				return t.PluginID == override.PluginID && t.ID == override.ID
			})
			r.mu.Unlock()
			r.notifyChange()
		})
	}
}

func (r *UIRegistry) ThemeOverrides() []ThemeOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.themes)
}

func (r *UIRegistry) RegisterFont(reg FontRegistration) func() {
	r.mu.Lock()
	r.fonts = append(r.fonts, reg)
	r.mu.Unlock()
	r.notifyChange()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			r.fonts = slices.DeleteFunc(r.fonts, func(f FontRegistration) bool {
				return f.PluginID == reg.PluginID && f.Family == reg.Family && f.FilePath == reg.FilePath
			})
			r.mu.Unlock()
			r.notifyChange()
		})
	}
}

func (r *UIRegistry) Fonts() []FontRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.fonts)
}

func (r *UIRegistry) RegisterDecoration(reg DecorationRegistration) func() {
	r.mu.Lock()
	r.decorations = slices.DeleteFunc(r.decorations, func(d DecorationRegistration) bool {
		return d.PluginID == reg.PluginID && d.ID == reg.ID
	})
	r.decorations = append(r.decorations, reg)
	r.mu.Unlock()
	r.notifyChange()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			r.decorations = slices.DeleteFunc(r.decorations, func(d DecorationRegistration) bool {
				return d.PluginID == reg.PluginID && d.ID == reg.ID
			})
			r.mu.Unlock()
			r.notifyChange()
		})
	}
}

func (r *UIRegistry) Decorations() []DecorationRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.decorations)
}

func (r *UIRegistry) RegisterWindow(spec PluginWindowSpec) func() {
	key := spec.PluginID + "/" + spec.WindowID
	r.mu.Lock()
	r.windows[key] = spec
	r.mu.Unlock()
	r.EmitWindowCommand(WindowCommand{
		Action:   "open",
		PluginID: spec.PluginID,
		WindowID: spec.WindowID,
		Spec:     &spec,
	})

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.windows, key)
			r.mu.Unlock()
			r.EmitWindowCommand(WindowCommand{
				Action:   "close",
				PluginID: spec.PluginID,
				WindowID: spec.WindowID,
			})
		})
	}
}

func (r *UIRegistry) EmitTreeUpdate(u UITreeUpdate) {
	r.mu.RLock()
	cbs := make([]func(UITreeUpdate), 0, len(r.treeSubs))
	for _, fn := range r.treeSubs {
		cbs = append(cbs, fn)
	}
	r.mu.RUnlock()
	for _, fn := range cbs {
		fn(u)
	}
}

func (r *UIRegistry) EmitAction(a UIAction) {
	r.mu.RLock()
	cbs := make([]func(UIAction), 0, len(r.actionSubs))
	for _, fn := range r.actionSubs {
		cbs = append(cbs, fn)
	}
	r.mu.RUnlock()
	for _, fn := range cbs {
		fn(a)
	}
}

func (r *UIRegistry) EmitWindowCommand(cmd WindowCommand) {
	r.mu.RLock()
	cbs := make([]func(WindowCommand), 0, len(r.winCmdSubs))
	for _, fn := range r.winCmdSubs {
		cbs = append(cbs, fn)
	}
	r.mu.RUnlock()
	for _, fn := range cbs {
		fn(cmd)
	}
}

func (r *UIRegistry) SubscribeChanges(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	r.mu.Lock()
	r.nextSubID++
	id := r.nextSubID
	r.changeSubs[id] = fn
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.changeSubs, id)
		r.mu.Unlock()
	}
}

func (r *UIRegistry) SubscribeTreeUpdates(fn func(UITreeUpdate)) func() {
	if fn == nil {
		return func() {}
	}
	r.mu.Lock()
	r.nextSubID++
	id := r.nextSubID
	r.treeSubs[id] = fn
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.treeSubs, id)
		r.mu.Unlock()
	}
}

func (r *UIRegistry) SubscribeActions(fn func(UIAction)) func() {
	if fn == nil {
		return func() {}
	}
	r.mu.Lock()
	r.nextSubID++
	id := r.nextSubID
	r.actionSubs[id] = fn
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.actionSubs, id)
		r.mu.Unlock()
	}
}

func (r *UIRegistry) SubscribeWindowCommands(fn func(WindowCommand)) func() {
	if fn == nil {
		return func() {}
	}
	r.mu.Lock()
	r.nextSubID++
	id := r.nextSubID
	r.winCmdSubs[id] = fn
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.winCmdSubs, id)
		r.mu.Unlock()
	}
}

func (r *UIRegistry) notifyChange() {
	r.mu.RLock()
	cbs := make([]func(), 0, len(r.changeSubs))
	for _, fn := range r.changeSubs {
		cbs = append(cbs, fn)
	}
	r.mu.RUnlock()
	for _, fn := range cbs {
		fn()
	}
}
