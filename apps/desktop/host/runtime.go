package host

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"

	"lunabox/backend/applog"
	"lunabox/backend/appruntime"
	"lunabox/backend/autostart"
)

// Events 把服务发出的事件（game-runtime:changed、cover-image:download 等）转给界面。
// 服务可能在任意 goroutine 里发事件，订阅者总在 dispatch 指定的线程（窗口的主线程）上被调用。
type Events struct {
	mu       sync.Mutex
	subs     map[string][]func(data any)
	dispatch func(func())
}

// NewEvents 创建事件总线；在 SetDispatcher 之前，事件在发出它的 goroutine 上直接分发。
func NewEvents() *Events { return &Events{subs: map[string][]func(any){}} }

// SetDispatcher 设置订阅者运行的地方，通常是 Window.Update。
func (e *Events) SetDispatcher(dispatch func(func())) {
	e.mu.Lock()
	e.dispatch = dispatch
	e.mu.Unlock()
}

// On 订阅事件，data 是服务发出的第一个参数（没有时为 nil）。
func (e *Events) On(name string, fn func(data any)) {
	e.mu.Lock()
	e.subs[name] = append(e.subs[name], fn)
	e.mu.Unlock()
}

// Emit 分发事件，报告是否有订阅者。
func (e *Events) Emit(name string, data ...any) bool {
	e.mu.Lock()
	subs := append([]func(any){}, e.subs[name]...)
	dispatch := e.dispatch
	e.mu.Unlock()
	if len(subs) == 0 {
		return false
	}
	var payload any
	if len(data) > 0 {
		payload = data[0]
	}
	run := func() {
		for _, fn := range subs {
			fn(payload)
		}
	}
	if dispatch != nil {
		dispatch(run)
	} else {
		run()
	}
	return true
}

// Window 用 MyGo 的窗口实现 bootstrap.Window。
type Window struct{ *mygo.Window }

func (w Window) UnMinimise() {
	if w.IsMinimized() {
		w.Restore()
	}
}

func (w Window) IsMinimised() bool { return w.IsMinimized() }
func (w Window) IsMaximised() bool { return w.IsMaximized() }

// Runtime 用 MyGo 实现 appruntime.Runtime：对话框是系统原生对话框，事件交给 Events。
// 对话框会阻塞到用户选择，所以服务里弹对话框的方法要在后台 goroutine 里调用（env.Async）。
type Runtime struct {
	Events *Events
	Win    *mygo.Window
	// Autostart 为 false 时 SetAutostart 只记日志、不改系统设置：开发运行不能把开发版注册成开机启动。
	Autostart bool
}

var _ appruntime.Runtime = (*Runtime)(nil)

func (r *Runtime) Emit(name string, data ...any) bool { return r.Events.Emit(name, data...) }

func (r *Runtime) OpenURL(url string) error { return mygo.Shell.OpenExternal(url) }

// SetAutostart 与 Wails 版写同一个开机启动项（autostart.Set），两个版本互相替换而不会各留一个。
func (r *Runtime) SetAutostart(enabled bool) error {
	if !r.Autostart {
		applog.LogInfof(context.Background(), "launch at login (%v) not applied: development run", enabled)
		return nil
	}
	return autostart.Set(enabled)
}

func (r *Runtime) RestoreWindow() {
	if r.Win != nil {
		r.Win.Restore() // Window 的方法自己切到主线程
	}
}

func (r *Runtime) ShowWindow() {
	if r.Win != nil {
		r.Win.Show()
	}
}

func (r *Runtime) OpenFile(o appruntime.OpenDialogOptions) (string, error) {
	return r.open(o, false)
}

func (r *Runtime) OpenDirectory(o appruntime.OpenDialogOptions) (string, error) {
	return r.open(o, true)
}

func (r *Runtime) open(o appruntime.OpenDialogOptions, dir bool) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: r.Win, Title: o.Title, DefaultPath: o.Directory, Filters: filters(o.Filters),
		Directory: dir, CreateDirectories: dir, ShowHiddenFiles: o.ShowHiddenFiles,
		TreatPackagesAsDirectories: o.TreatsFilePackagesAsDirectories,
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func (r *Runtime) SaveFile(o appruntime.SaveDialogOptions) (string, error) {
	def := o.Filename
	if o.Directory != "" {
		def = filepath.Join(o.Directory, o.Filename)
	}
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: r.Win, Title: o.Title, DefaultPath: def, Filters: filters(o.Filters),
		ShowHiddenFiles: o.ShowHiddenFiles, CreateDirectories: true,
		TreatPackagesAsDirectories: o.TreatsFilePackagesAsDirectories,
	})
}

// filters 把 Wails 的 "*.exe;*.lnk" 写法转成 MyGo 的扩展名列表。
func filters(fs []appruntime.FileFilter) []mygo.FileFilter {
	out := make([]mygo.FileFilter, 0, len(fs))
	for _, f := range fs {
		var exts []string
		for _, p := range strings.Split(f.Pattern, ";") {
			p = strings.TrimSpace(p)
			p = strings.TrimPrefix(strings.TrimPrefix(p, "*"), ".")
			if p != "" {
				exts = append(exts, p)
			}
		}
		out = append(out, mygo.FileFilter{Name: f.DisplayName, Extensions: exts})
	}
	return out
}
