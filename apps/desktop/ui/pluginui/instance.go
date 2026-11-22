// Package pluginui 在界面线程上管理插件插槽实例、描述树缓存、输入框状态、白名单组件渲染、
// 全局装饰层与插件独立悬浮窗。
// 遵循线程规则：界面线程从不同步等待 JS，只读缓存树、只投递事件消息。
package pluginui

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"

	"lunabox/backend/plugin"
	"lunabox/backend/utils/httputils"
	"lunabox/backend/version"
	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/env"
)

const inputDebounce = 150 * time.Millisecond

// InputState 为每个 (实例, key) 保存一份 Go 侧持有的 *string，避免输入法候选与光标受插件往返影响。
type InputState struct {
	Value        string
	LastSent     string
	Dirty        bool
	DirtyAt      time.Time
	PendingEvent string
}

// Instance 表示一个正在挂载的插件插槽或独立窗口实例，只在界面线程上读写。
type Instance struct {
	ID       string
	PluginID string
	SlotName string
	RegID    string
	PropsKey string

	Tree    *plugin.UINode
	Version uint64
	Error   string

	Inputs       map[string]*InputState
	Scrolls      map[string]*components.ScrollBar
	Collapsibles map[string]*bool
	PreviewSrc   string
	PreviewOpen  bool
}

// HostCallbacks 由 ui.App 注入，处理插件发起的全局 UI 动作（抽屉、导入弹窗、导航等）。
type HostCallbacks struct {
	OnOpenDrawer      func(pluginID, drawerID string, props json.RawMessage)
	OnCloseDrawer     func()
	OnOpenImportModal func(props json.RawMessage)
	OnMainWindowCmd   func(action string)
}

// Manager 持有所有插件 UI 实例、独立子窗口与图片缓存，随 ui.App 存活。
type Manager struct {
	env       *env.Env
	callbacks HostCallbacks

	instances map[string]*Instance // instanceID -> *Instance
	windows   map[string]*pluginWindow

	unsubTree   func()
	unsubUI     func()
	unsubState  func()
	unsubAction func()
	unsubWinCmd func()

	imgMu      sync.RWMutex
	imgCache   map[string]*ui.Bitmap
	svgCache   map[string]*ui.SVG
	imgLoading map[string]bool

	// 开发模式判断（决定未知组件与错误边界是否展示详细错误信息）
	DevMode bool
}

// New 创建插件界面管理器。
func New(e *env.Env, cb HostCallbacks) *Manager {
	return &Manager{
		env:        e,
		callbacks:  cb,
		instances:  make(map[string]*Instance),
		windows:    make(map[string]*pluginWindow),
		imgCache:   make(map[string]*ui.Bitmap),
		svgCache:   make(map[string]*ui.SVG),
		imgLoading: make(map[string]bool),
		DevMode:    mygo.IsDev(),
	}
}

// Attach 订阅后端插件服务推送的描述树、UI 注册变更、动作与窗口指令，全部通过 Win.Update 切回界面线程。
func (m *Manager) Attach(win *mygo.Window) {
	if m.env == nil || m.env.Svc == nil || m.env.Svc.Plugin == nil {
		return
	}
	svc := m.env.Svc.Plugin
	runOnUI := func(fn func()) {
		if win == nil {
			fn()
			return
		}
		win.Update(fn)
	}

	m.unsubTree = svc.SubscribeTreeUpdates(func(u plugin.UITreeUpdate) {
		runOnUI(func() {
			m.applyTreeUpdate(u)
		})
	})
	m.unsubUI = svc.SubscribeUIChanges(func() {
		runOnUI(func() {
			m.syncFonts()
			if win != nil {
				win.Invalidate()
			}
		})
	})
	m.unsubState = svc.SubscribeState(func() {
		runOnUI(func() {
			m.syncFonts()
			if win != nil {
				win.Invalidate()
			}
		})
	})
	m.unsubAction = svc.SubscribeActions(func(a plugin.UIAction) {
		runOnUI(func() {
			m.handleAction(a)
		})
	})
	m.unsubWinCmd = svc.SubscribeWindowCommands(func(cmd plugin.WindowCommand) {
		runOnUI(func() {
			m.handleWindowCommand(cmd)
		})
	})
	m.syncFonts()
}

// Close 取消全部订阅并关闭所有插件子窗口。
func (m *Manager) Close() {
	if m.unsubTree != nil {
		m.unsubTree()
	}
	if m.unsubUI != nil {
		m.unsubUI()
	}
	if m.unsubState != nil {
		m.unsubState()
	}
	if m.unsubAction != nil {
		m.unsubAction()
	}
	if m.unsubWinCmd != nil {
		m.unsubWinCmd()
	}
	for k, w := range m.windows {
		w.close()
		delete(m.windows, k)
	}
}

func (m *Manager) applyTreeUpdate(u plugin.UITreeUpdate) {
	inst, ok := m.instances[u.InstanceID]
	if !ok {
		// 独立窗口可能在 open 之前就产生了第一棵树，提前建实例缓存
		inst = newInstance(u.InstanceID, u.PluginID, "", "", "")
		m.instances[u.InstanceID] = inst
	}
	inst.Tree = u.Tree
	inst.Version = u.Version
	inst.Error = u.Error

	// 如果该实例属于独立子窗口，同时刷新那个子窗口
	for _, w := range m.windows {
		if w.spec.InstanceID == u.InstanceID && w.win != nil {
			w.win.Invalidate()
		}
	}
}

func (m *Manager) handleAction(a plugin.UIAction) {
	switch a.Type {
	case "toast":
		var n env.Notice
		switch a.Level {
		case "success":
			n = env.Success(a.Message)
		case "error":
			n = env.Error(a.Message)
		case "loading":
			n = env.Loading(a.Message)
		default:
			n = env.Info(a.Message)
		}
		m.env.Notify(n)
	case "navigate":
		if m.env.Router != nil && strings.TrimSpace(a.Path) != "" {
			m.env.Router.Push(a.Path)
		}
	case "openDrawer":
		if m.callbacks.OnOpenDrawer != nil {
			m.callbacks.OnOpenDrawer(a.PluginID, a.DrawerID, a.Props)
		}
	case "closeDrawer":
		if m.callbacks.OnCloseDrawer != nil {
			m.callbacks.OnCloseDrawer()
		}
	case "openImportModal":
		if m.callbacks.OnOpenImportModal != nil {
			m.callbacks.OnOpenImportModal(a.Props)
		}
	}
}

func newInstance(id, pluginID, slotName, regID, propsKey string) *Instance {
	return &Instance{
		ID:           id,
		PluginID:     pluginID,
		SlotName:     slotName,
		RegID:        regID,
		PropsKey:     propsKey,
		Inputs:       make(map[string]*InputState),
		Scrolls:      make(map[string]*components.ScrollBar),
		Collapsibles: make(map[string]*bool),
	}
}

// InstanceID 为一个插槽挂载点生成稳定唯一的实例 ID。
func InstanceID(slotName, pluginID, regID, scopeKey string) string {
	return slotName + "::" + pluginID + "::" + regID + "::" + scopeKey
}

func propsHash(props any) string {
	if props == nil {
		return ""
	}
	b, err := json.Marshal(props)
	if err != nil {
		return ""
	}
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:8])
}

// EnsureMounted 确保插槽实例已挂载；若 props 发生变化则先卸载再挂载（见 §9.4）。
func (m *Manager) EnsureMounted(slotName, pluginID, regID, scopeKey string, props any) *Instance {
	id := InstanceID(slotName, pluginID, regID, scopeKey)
	pk := propsHash(props)
	if inst, ok := m.instances[id]; ok {
		if inst.PropsKey == pk {
			return inst
		}
		m.Unmount(id)
	}
	inst := newInstance(id, pluginID, slotName, regID, pk)
	m.instances[id] = inst
	if m.env != nil && m.env.Svc != nil && m.env.Svc.Plugin != nil {
		m.env.Svc.Plugin.MountSlot(id, pluginID, slotName, regID, props)
	}
	return inst
}

// Unmount 卸载单个插槽实例，丢弃树缓存与输入框状态。
func (m *Manager) Unmount(instanceID string) {
	inst, ok := m.instances[instanceID]
	if !ok {
		return
	}
	delete(m.instances, instanceID)
	if m.env != nil && m.env.Svc != nil && m.env.Svc.Plugin != nil {
		m.env.Svc.Plugin.UnmountSlot(instanceID, inst.PluginID)
	}
}

// UnmountBySlot 卸载指定插槽名（与可选前缀）下的全部实例，供页面 Leave 时调用。
func (m *Manager) UnmountBySlot(slotName string) {
	prefix := slotName + "::"
	for id := range m.instances {
		if strings.HasPrefix(id, prefix) {
			m.Unmount(id)
		}
	}
}

func (m *Manager) dispatch(inst *Instance, handlerID string, payload any) {
	if inst == nil || handlerID == "" || m.env == nil || m.env.Svc == nil || m.env.Svc.Plugin == nil {
		return
	}
	m.env.Svc.Plugin.DispatchEvent(plugin.UIEvent{
		InstanceID: inst.ID,
		PluginID:   inst.PluginID,
		HandlerID:  handlerID,
		Version:    inst.Version,
		Payload:    payload,
	})
}

func (m *Manager) pluginName(pluginID string) string {
	if m.env != nil && m.env.Svc != nil && m.env.Svc.Plugin != nil {
		for _, info := range m.env.Svc.Plugin.List() {
			if info.ID == pluginID {
				return info.Name
			}
		}
	}
	return pluginID
}

// LoadImageSource 异步加载本地图标/图片（支持 SVG、PNG、JPEG、GIF、WebP）并缓存为 ui.ImageSource，绝不阻塞界面线程。
func (m *Manager) LoadImageSource(src string) ui.ImageSource {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil
	}
	if strings.HasSuffix(strings.ToLower(src), ".svg") {
		m.imgMu.RLock()
		if svg, ok := m.svgCache[src]; ok {
			m.imgMu.RUnlock()
			return svg
		}
		if m.imgLoading[src] {
			m.imgMu.RUnlock()
			return nil
		}
		m.imgMu.RUnlock()

		m.imgMu.Lock()
		if m.imgLoading[src] {
			m.imgMu.Unlock()
			return nil
		}
		m.imgLoading[src] = true
		m.imgMu.Unlock()

		go func() {
			data, err := os.ReadFile(src)
			var svg *ui.SVG
			if err == nil && len(data) > 0 {
				if parsed, perr := ui.ParseSVG(data); perr == nil {
					svg = parsed
				}
			}
			m.imgMu.Lock()
			delete(m.imgLoading, src)
			if svg != nil {
				m.svgCache[src] = svg
			}
			m.imgMu.Unlock()
			if svg != nil && m.env != nil && m.env.Win != nil {
				m.env.Win.Invalidate()
			}
		}()
		return nil
	}
	return m.loadBitmap(src)
}

// loadBitmap 异步加载本地路径或 http(s) URL 图片并缓存为 *ui.Bitmap，绝不阻塞界面线程。
func (m *Manager) loadBitmap(src string) *ui.Bitmap {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil
	}
	m.imgMu.RLock()
	if bmp, ok := m.imgCache[src]; ok {
		m.imgMu.RUnlock()
		return bmp
	}
	if m.imgLoading[src] {
		m.imgMu.RUnlock()
		return nil
	}
	m.imgMu.RUnlock()

	m.imgMu.Lock()
	if m.imgLoading[src] {
		m.imgMu.Unlock()
		return nil
	}
	m.imgLoading[src] = true
	m.imgMu.Unlock()

	go func() {
		var img image.Image
		var err error
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			client, _, cerr := httputils.NewClient(httputils.ClientOptions{
				Timeout:     15 * time.Second,
				ProxyConfig: m.env.Config(),
			})
			if cerr != nil || client == nil {
				client = &http.Client{Timeout: 15 * time.Second}
			}
			req, rerr := http.NewRequest(http.MethodGet, src, nil)
			if rerr == nil {
				req.Header.Set("User-Agent", version.UserAgent())
				resp, derr := client.Do(req)
				if derr == nil {
					img, _, err = image.Decode(resp.Body)
					resp.Body.Close()
				} else {
					err = derr
				}
			} else {
				err = rerr
			}
		} else {
			f, ferr := os.Open(src)
			if ferr == nil {
				img, _, err = image.Decode(f)
				f.Close()
			} else {
				err = ferr
			}
		}

		var bmp *ui.Bitmap
		if err == nil && img != nil {
			bmp = ui.NewBitmap(assets.Downscale(img, 1024))
		}
		m.imgMu.Lock()
		delete(m.imgLoading, src)
		if bmp != nil {
			m.imgCache[src] = bmp
		}
		m.imgMu.Unlock()

		if bmp != nil && m.env != nil && m.env.Win != nil {
			m.env.Win.Invalidate()
		}
	}()
	return nil
}

func (m *Manager) syncFonts() {
	if m.env == nil || m.env.Svc == nil || m.env.Svc.Plugin == nil {
		return
	}
	// 由 ui/theme 包统一注册并记录全局字体覆盖
	syncThemeFonts(m.env.Svc.Plugin.Fonts())
}
