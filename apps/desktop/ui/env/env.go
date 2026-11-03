// Package env 是页面与布局共享的运行环境：后端、窗口、路由、封面缓存和当前帧的主题。
package env

import (
	"net/url"
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/theme"
)

// Backend 是游戏库列表用到的查询，由真实服务实现（见 app 包的 list.go）；单独成接口是为了
// -repeat 压测时把列表放大，以及测试时用内存实现。其余页面用 Env.Svc。
type Backend interface {
	GetGames(req vo.GameListRequest) (vo.GameListResponse, error)
	SearchTagsInLibrary(query string) ([]string, error)
	FilterExistingTagNames(names []string) ([]string, error)
	ListGameFilterPresets() ([]models.GameFilterPreset, error)
	Config() *appconf.AppConfig
	DataDir() string
}

// 路由路径，与原版 frontend/src/routes 一致。
const (
	PathHome       = "/"
	PathLibrary    = "/library"
	PathStats      = "/stats"
	PathCategories = "/categories"
	PathCategory   = "/categories/{id}" // 收藏详情，Route.Param("id") 是收藏夹 ID
	PathDownloads  = "/downloads"
	PathSettings   = "/settings"
	PathGame       = "/game/{id}" // 游戏详情，Route.Param("id") 是游戏 ID
)

// GamePath 返回游戏详情页的路径。
func GamePath(id string) string { return "/game/" + url.PathEscape(id) }

// CategoryPath 返回收藏详情页的路径。
func CategoryPath(id string) string { return "/categories/" + url.PathEscape(id) }

// TopbarHeight 对应 TopBar.tsx 的 TOPBAR_HEIGHT。
const TopbarHeight = 28

// Env 由 ui.App 创建，各页面持有同一个指针。
type Env struct {
	Backend Backend
	Svc     *api.Services // 后端服务（运行时是 apps/desktop 的真实服务）
	Win     *mygo.Window  // 测试时为 nil
	Router  *ui.Router
	Covers  *covers.Cache

	// SidebarOpen 报告侧边栏是否展开（store 的 isSidebarOpen）：保存配置时 sidebar_open 取它（withSidebarState）。
	// 测试里为 nil，保存时保留配置里的值。
	SidebarOpen func() bool
	// ConfigSaved 在设置页保存配置之后（主线程）调用，App 用它让语言与主题立即生效；测试里为 nil。
	ConfigSaved func(prev, next appconf.AppConfig)

	// 以下由 App.View 在每帧开始时更新。
	Theme     *theme.Theme
	MainWidth float32 // 主内容区（窗口减去侧边栏）的宽度，随侧边栏动画逐帧变化

	toasts []ui.Toast // 帧外调用 Toast 时排队，下一帧显示
}

// Config 返回应用配置，可能为 nil。
func (e *Env) Config() *appconf.AppConfig {
	if e.Svc != nil {
		return e.Svc.Config
	}
	return e.Backend.Config()
}

// PatchConfig 对应 store 的 patchLiveConfig（设置页以外的调用方）：在当前配置上改几项并立即保存，
// sidebar_open 取侧边栏当前的状态。在主线程调用。
func (e *Env) PatchConfig(fn func(cfg *appconf.AppConfig)) error {
	cur := e.Config()
	if cur == nil || e.Svc == nil {
		return nil
	}
	prev, next := *cur, *cur
	fn(&next)
	if e.SidebarOpen != nil {
		next.SidebarOpen = e.SidebarOpen()
	}
	if err := e.Svc.AppConfig.UpdateAppConfig(next); err != nil {
		return err
	}
	if e.ConfigSaved != nil {
		e.ConfigSaved(prev, *e.Config())
	}
	return nil
}

// Async 在后台执行 work，再把它返回的函数放回主线程执行；没有窗口（测试）时同步执行。
func (e *Env) Async(work func() func()) {
	if e.Win == nil {
		work()()
		return
	}
	go func() {
		apply := work()
		e.Win.Update(apply)
	}()
}

// Notice 是一条提示，对应 react-hot-toast 的 toast()、toast.success()、toast.error()、toast.loading()：
// Type 决定 components.Toaster 里的图标与配色。
type Notice struct {
	Type string // ""（普通）、"success"、"error"、"loading"
	Msg  string
	// ID 是 toast 的 id 选项：相同 ID 的提示互相替换；为空时每次都是新的一条（与 react-hot-toast 一样会层叠）。
	ID string
	// Duration 是 duration 选项：0 是默认时长，负数一直显示（Infinity）。
	Duration time.Duration
	// Action 是提示里的按钮文字，点击时调用 OnAction 并关闭提示（原版用 toast.custom 画的带按钮的提示）。
	Action   string
	OnAction func()
}

// Info 对应 toast(msg)。
func Info(msg string) Notice { return Notice{Msg: msg} }

// Success 对应 toast.success(msg)。
func Success(msg string) Notice { return Notice{Type: "success", Msg: msg} }

// Error 对应 toast.error(msg)。
func Error(msg string) Notice { return Notice{Type: "error", Msg: msg} }

// Loading 对应 toast.loading(msg)。
func Loading(msg string) Notice { return Notice{Type: "loading", Msg: msg} }

// Toast 显示提示，样式由 components.Toaster 按原版 AppToaster 绘制。设了 ID 且相同 ID 的提示正在显示时替换它。
func (e *Env) Toast(c *ui.Context, n Notice) {
	t := n.toast()
	if !c.Valid() {
		// 帧外（如 Async 放回主线程的回调里）AddToast 不生效：留到下一帧由 FlushToasts 显示。
		e.toasts = append(e.toasts, t)
		c.Invalidate()
		return
	}
	c.AddToast(t)
}

// Notify 在下一帧显示提示，给拿不到 Context 的地方用（如离开页面时才开始的保存，结果回来时页面已经不再构建）。
// 在主线程调用。
func (e *Env) Notify(n Notice) {
	e.toasts = append(e.toasts, n.toast())
	if e.Win != nil {
		e.Win.Invalidate()
	}
}

func (n Notice) toast() ui.Toast {
	timeout := n.Duration
	if n.Type == "loading" && timeout == 0 {
		timeout = -1 // react-hot-toast 的 loading 默认一直显示
	}
	return ui.Toast{ID: n.ID, Title: n.Msg, Type: n.Type, Timeout: timeout, Action: n.Action, OnAction: n.OnAction}
}

// FlushToasts 显示帧外调用 Toast 时排队的提示，由 App.View 每帧调用。
func (e *Env) FlushToasts(c *ui.Context) {
	for _, t := range e.toasts {
		c.AddToast(t)
	}
	e.toasts = nil
}

// ApplyTheme 对应 hooks/useAppTheme.ts 按 config.theme 切换 dark 类：light、dark 强制浅色或深色，其余跟随系统。
func ApplyTheme(theme string) {
	switch theme {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// ApplyLanguage 对应 App.tsx 里的 i18n.changeLanguage(config.language)：显示配置里的语言（zh-CN、zh-TW、
// en-US、ja-JP）。为空或不是支持的语言时跟随系统的语言（原版的 LanguageDetector），都不匹配时是 zh-CN。
// 运行时设置 MYGO_LOCALE（-lang）时它优先，启动时这里的设置不生效。
func ApplyLanguage(lang string) {
	if !slices.Contains(mygo.App.Languages(), lang) {
		lang = ""
	}
	mygo.App.SetLocale(lang)
}
