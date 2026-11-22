package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
	"github.com/MoeForge/moejs/eventloop"
	"github.com/MoeForge/moejs/node"
	"github.com/MoeForge/moejs/webapi"

	"lunabox/backend/applog"
	"lunabox/backend/utils/httputils"
	"lunabox/backend/version"
)

const (
	pluginMemoryLimit = 256 << 20 // 256 MiB
	applyTimeout      = 10 * time.Second
)

// PluginStatus 表示插件的生命周期状态。
type PluginStatus string

const (
	StatusDiscovered PluginStatus = "discovered"
	StatusLoading    PluginStatus = "loading"
	StatusActive     PluginStatus = "active"
	StatusDisposing  PluginStatus = "disposing"
	StatusDisabled   PluginStatus = "disabled"
	StatusError      PluginStatus = "error"
)

// PluginInfo 是向界面暴露的插件状态快照。
type PluginInfo struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	APIVersion  int               `json:"api_version"`
	Engines     string            `json:"engines,omitempty"`
	Name        string            `json:"name"`
	Names       map[string]string `json:"names,omitempty"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	License     string            `json:"license,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	IconBg      string            `json:"icon_bg,omitempty"`
	IconPath    string            `json:"icon_path,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Dir         string            `json:"dir"`
	DataDir     string            `json:"data_dir"`
	IsDev       bool              `json:"is_dev"`
	Enabled     bool              `json:"enabled"`
	Status      PluginStatus      `json:"status"`
	LastError   string            `json:"last_error,omitempty"`
}

// Plugin 表示一个已加载运行的插件实例，独占一个 moejs.Runtime 与 eventloop goroutine。
type Plugin struct {
	ID       string
	Manifest Manifest
	Dir      string
	DataDir  string
	IsDev    bool

	manager *Manager
	scope   *Scope

	rt      *moejs.Runtime
	loop    *eventloop.Loop
	webAPI  *webapi.API
	nodeAPI *node.API

	storage   *pluginStorage
	i18n      *pluginI18n
	configMgr *pluginConfig

	mu          sync.RWMutex
	lastError   string
	renderFns   map[string]moejs.Value // slot/window key -> JS render function
	actionFns   map[string]moejs.Value // slot key -> JS onPress/run function
	whenFns     map[string]moejs.Value // slot key -> JS when(game) function
	winHandlers map[string]*windowCallbacks
	instScopes  map[string]*Scope

	// 待发送描述树合并槽（同一实例在发送前仅保留最新一棵树）
	pendingMu    sync.Mutex
	pendingTrees map[string]UITreeUpdate

	closed atomic.Bool
}

type windowCallbacks struct {
	onMove  []moejs.Value
	onFocus []moejs.Value
	onClose []moejs.Value
}

// defaultUARoundTripper 为插件 fetch 请求设置默认 User-Agent（AGENTS.md MUST），
// 仅当插件未显式设置 User-Agent 时写入 version.UserAgent()。
type defaultUARoundTripper struct {
	base http.RoundTripper
}

func (d *defaultUARoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if clone.Header.Get("User-Agent") == "" {
		clone.Header.Set("User-Agent", version.UserAgent())
	}
	return d.base.RoundTrip(clone)
}

func (m *Manager) newPluginHTTPClient(timeout time.Duration) *http.Client {
	opts := httputils.ClientOptions{
		Timeout: timeout,
	}
	if m.deps.Config != nil {
		opts.ProxyConfig = m.deps.Config
	}
	client, _, err := httputils.NewClient(opts)
	if err != nil || client == nil {
		client = &http.Client{Timeout: timeout}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = &defaultUARoundTripper{base: base}
	return client
}

func startPluginRuntime(m *Manager, manifest Manifest, dir, dataDir string, isDev bool) (*Plugin, error) {
	p := &Plugin{
		ID:           manifest.ID,
		Manifest:     manifest,
		Dir:          dir,
		DataDir:      dataDir,
		IsDev:        isDev,
		manager:      m,
		scope:        NewScope(m.ctx),
		renderFns:    make(map[string]moejs.Value),
		actionFns:    make(map[string]moejs.Value),
		whenFns:      make(map[string]moejs.Value),
		winHandlers:  make(map[string]*windowCallbacks),
		instScopes:   make(map[string]*Scope),
		pendingTrees: make(map[string]UITreeUpdate),
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		p.scope.Dispose()
		return nil, fmt.Errorf("创建插件数据目录失败: %w", err)
	}
	p.storage = newPluginStorage(p)
	p.i18n = newPluginI18n(p)
	p.configMgr = newPluginConfig(p)

	sdkMod, err := getLunaboxSDKModule()
	if err != nil {
		p.scope.Dispose()
		return nil, fmt.Errorf("编译 lunabox SDK 模块失败: %w", err)
	}

	resolveFn := func(_ moejs.Referrer, specifier string) (*moejs.Module, error) {
		switch specifier {
		case "lunabox", "lunabox/jsx-runtime", "lunabox/jsx-dev-runtime":
			return sdkMod, nil
		default:
			return nil, fmt.Errorf("插件只允许导入虚拟模块 lunabox 或 node:buffer，禁止导入 %q", specifier)
		}
	}
	baseImporter := &moejs.Importer{Resolve: resolveFn}
	imp := node.Importer(baseImporter)

	locale := m.currentLanguage()
	rt := moejs.NewRuntime(moejs.Options{
		Locale:      locale,
		Importer:    imp,
		MemoryLimit: pluginMemoryLimit,
	})
	p.rt = rt

	loop, err := eventloop.New(rt, eventloop.Options{
		OnError: func(err error) {
			p.recordError(err)
		},
	})
	if err != nil {
		p.scope.Dispose()
		return nil, fmt.Errorf("创建插件事件循环失败: %w", err)
	}
	p.loop = loop

	httpClient := m.newPluginHTTPClient(30 * time.Second)
	webAPI, err := webapi.Install(loop, webapi.Options{
		Client:  httpClient,
		Allow:   nil, // 完全信任，不限制目标
		Context: p.scope.Context(),
	})
	if err != nil {
		p.scope.Dispose()
		return nil, fmt.Errorf("安装 webapi 失败: %w", err)
	}
	p.webAPI = webAPI

	nodeAPI, err := node.InstallLoop(loop, node.Options{
		Buffer: true,
	})
	if err != nil {
		webAPI.Close()
		p.scope.Dispose()
		return nil, fmt.Errorf("安装 node:buffer 失败: %w", err)
	}
	p.nodeAPI = nodeAPI

	// 入口文件编译
	entryPath := filepath.Join(dir, filepath.FromSlash(manifest.Main))
	srcBytes, err := os.ReadFile(entryPath)
	if err != nil {
		p.shutdownRuntime()
		return nil, fmt.Errorf("读取入口文件 %s 失败: %w", manifest.Main, err)
	}
	var entryMod *moejs.Module
	ext := strings.ToLower(filepath.Ext(entryPath))
	if ext == ".ts" || ext == ".tsx" {
		if !isDev {
			p.shutdownRuntime()
			return nil, fmt.Errorf("正式插件入口必须为已打包的 .js 文件，不允许加载 %s", manifest.Main)
		}
		entryMod, err = moejs.CompileTS(manifest.Main, string(srcBytes))
	} else {
		entryMod, err = moejs.Compile(manifest.Main, string(srcBytes))
	}
	if err != nil {
		p.shutdownRuntime()
		return nil, fmt.Errorf("编译插件入口失败: %w", err)
	}
	linkedMod, err := moejs.Link(entryMod, node.Resolver(resolveFn))
	if err != nil {
		p.shutdownRuntime()
		return nil, fmt.Errorf("链接插件模块失败: %w", err)
	}

	if err := loop.Start(); err != nil {
		p.shutdownRuntime()
		return nil, fmt.Errorf("启动插件事件循环失败: %w", err)
	}

	// 在插件线程安装全局对象、加载模块并调用 apply(ctx)（限时 10s）
	initCtx, cancel := context.WithTimeout(p.scope.Context(), applyTimeout)
	defer cancel()

	if err := p.runSync(initCtx, func(rt *moejs.Runtime) error {
		if err := p.installGlobals(rt); err != nil {
			return err
		}
		if err := rt.Load(linkedMod); err != nil {
			return err
		}
		applyFn, ok := rt.Export("apply")
		if !ok || applyFn.IsUndefined() {
			if def, okDef := rt.Export("default"); okDef && def.IsObject() {
				if v, gerr := rt.Get(def, "apply"); gerr == nil && !v.IsUndefined() {
					applyFn = v
					ok = true
				}
			}
		}
		if !ok || applyFn.IsUndefined() {
			return fmt.Errorf("插件未导出 apply(ctx) 函数")
		}
		ctxObj, err := p.buildContextObject(rt)
		if err != nil {
			return fmt.Errorf("构建插件 ctx 失败: %w", err)
		}
		res, err := rt.CallFunction(applyFn, moejs.Undefined(), ctxObj)
		if err != nil {
			return err
		}
		// 若 apply 返回 Promise，则在其 settle 后返回
		if state, pRes, isPromise := moejs.PromiseResult(res); isPromise {
			switch state {
			case moejs.PromiseFulfilled:
				return nil
			case moejs.PromiseRejected:
				return fmt.Errorf("apply Promise rejected: %s", p.formatJSValue(rt, pRes))
			}
			// Pending Promise：使用异步等待
			return &pendingPromiseErr{promise: res}
		}
		return nil
	}); err != nil {
		var pending *pendingPromiseErr
		if errors.As(err, &pending) {
			if waitErr := p.awaitPromise(initCtx, pending.promise); waitErr != nil {
				p.recordError(waitErr)
				p.Dispose()
				return nil, waitErr
			}
		} else {
			p.recordError(err)
			p.Dispose()
			return nil, err
		}
	}

	return p, nil
}

type pendingPromiseErr struct {
	promise moejs.Value
}

func (e *pendingPromiseErr) Error() string { return "pending promise" }

// awaitPromise 等待插件线程上一个 pending Promise settle，超时或 ctx 取消时 Interrupt。
func (p *Plugin) awaitPromise(ctx context.Context, promise moejs.Value) error {
	_, err := p.awaitPromiseValue(ctx, promise)
	return err
}

func (p *Plugin) awaitPromiseValue(ctx context.Context, promise moejs.Value) (json.RawMessage, error) {
	type settleResult struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan settleResult, 1)

	ok := p.loop.RunOnLoop(func(rt *moejs.Runtime) {
		thenFn, err := rt.Get(promise, "then")
		if err != nil || thenFn.IsUndefined() {
			raw, _ := rt.AppendJSON(nil, promise)
			ch <- settleResult{raw: raw}
			return
		}
		onFulfilled := rt.Function("onFulfilled", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			v := moejs.Arg(args, 0)
			if v.IsUndefined() {
				ch <- settleResult{}
				return moejs.Undefined(), nil
			}
			raw, err := rt.AppendJSON(nil, v)
			if err != nil {
				ch <- settleResult{err: err}
				return moejs.Undefined(), nil
			}
			ch <- settleResult{raw: raw}
			return moejs.Undefined(), nil
		})
		onRejected := rt.Function("onRejected", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			v := moejs.Arg(args, 0)
			ch <- settleResult{err: fmt.Errorf("%s", p.formatJSValue(rt, v))}
			return moejs.Undefined(), nil
		})
		if _, err := rt.CallFunction(thenFn, promise, onFulfilled, onRejected); err != nil {
			ch <- settleResult{err: err}
		}
	})
	if !ok {
		return nil, fmt.Errorf("插件运行时已停止")
	}

	select {
	case res := <-ch:
		return res.raw, res.err
	case <-ctx.Done():
		p.loop.Interrupt(ctx.Err())
		return nil, ctx.Err()
	}
}

// runSync 在插件线程上同步执行一个函数，ctx 取消时 Interrupt。
func (p *Plugin) runSync(ctx context.Context, fn func(rt *moejs.Runtime) error) error {
	if p.closed.Load() {
		return fmt.Errorf("插件已卸载")
	}
	done := make(chan error, 1)
	ok := p.loop.RunOnLoop(func(rt *moejs.Runtime) {
		defer func() {
			if rec := recover(); rec != nil {
				done <- fmt.Errorf("plugin runtime panic: %v", rec)
			}
		}()
		rt.ClearInterrupt()
		done <- fn(rt)
	})
	if !ok {
		return fmt.Errorf("插件运行时已停止")
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		p.loop.Interrupt(ctx.Err())
		return ctx.Err()
	}
}

// Call 在插件线程上调用 JS 函数并等待结果（自动 await Promise），ctx 取消/超时时 Interrupt。
func (p *Plugin) Call(ctx context.Context, fn moejs.Value, args ...any) (json.RawMessage, error) {
	if p.closed.Load() {
		return nil, fmt.Errorf("插件已卸载")
	}
	var rawOut json.RawMessage
	var pending moejs.Value
	hasPending := false

	err := p.runSync(ctx, func(rt *moejs.Runtime) error {
		jsArgs := make([]moejs.Value, len(args))
		for i, a := range args {
			v, err := toJSValue(rt, a)
			if err != nil {
				return err
			}
			jsArgs[i] = v
		}
		res, err := rt.CallFunction(fn, moejs.Undefined(), jsArgs...)
		if err != nil {
			return err
		}
		if state, pRes, isPromise := moejs.PromiseResult(res); isPromise {
			switch state {
			case moejs.PromiseFulfilled:
				if !pRes.IsUndefined() {
					rawOut, err = rt.AppendJSON(nil, pRes)
					return err
				}
				return nil
			case moejs.PromiseRejected:
				return fmt.Errorf("%s", p.formatJSValue(rt, pRes))
			default:
				pending = res
				hasPending = true
				return nil
			}
		}
		if !res.IsUndefined() {
			rawOut, err = rt.AppendJSON(nil, res)
			return err
		}
		return nil
	})
	if err != nil {
		p.recordError(err)
		return nil, err
	}
	if hasPending {
		raw, err := p.awaitPromiseValue(ctx, pending)
		if err != nil {
			p.recordError(err)
			return nil, err
		}
		return raw, nil
	}
	return rawOut, nil
}

// Post 向插件线程投递一个非阻塞任务后立即返回（供界面事件与回调使用）。
func (p *Plugin) Post(task func(rt *moejs.Runtime)) {
	if p == nil || p.closed.Load() || p.loop == nil {
		return
	}
	p.loop.RunOnLoop(func(rt *moejs.Runtime) {
		defer func() {
			if rec := recover(); rec != nil {
				p.recordError(fmt.Errorf("plugin task panic: %v", rec))
			}
		}()
		task(rt)
	})
}

// Dispose 卸载插件：取消作用域、按 LIFO 执行所有撤销函数、停止事件循环并释放运行时。
func (p *Plugin) Dispose() {
	if p == nil || !p.closed.CompareAndSwap(false, true) {
		return
	}
	// 先刷新存储中尚未落盘的数据
	if p.storage != nil {
		p.storage.FlushSync()
	}
	p.scope.Dispose()
	p.shutdownRuntime()
}

func (p *Plugin) shutdownRuntime() {
	if p.webAPI != nil {
		p.webAPI.Close()
		p.webAPI = nil
	}
	if p.nodeAPI != nil {
		p.nodeAPI.Close()
		p.nodeAPI = nil
	}
	if p.loop != nil {
		p.loop.Terminate()
		_ = p.loop.Stop()
		p.loop = nil
	}
}

func (p *Plugin) LastError() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError
}

func (p *Plugin) recordError(err error) {
	if err == nil {
		return
	}
	msg := p.formatError(err)
	p.mu.Lock()
	p.lastError = msg
	p.mu.Unlock()
	applog.LogErrorf(p.manager.ctx, "[plugin:%s] %s", p.ID, msg)
}

func (p *Plugin) formatError(err error) string {
	var exc *moejs.Exception
	if errors.As(err, &exc) && p.rt != nil {
		if stack := p.rt.StackTrace(exc); strings.TrimSpace(stack) != "" {
			return stack
		}
	}
	return err.Error()
}

func (p *Plugin) formatJSValue(rt *moejs.Runtime, v moejs.Value) string {
	if v.IsUndefined() {
		return "undefined"
	}
	if v.IsNull() {
		return "null"
	}
	if v.IsObject() {
		if msg, err := rt.Get(v, "stack"); err == nil && msg.IsString() {
			return msg.String()
		}
		if msg, err := rt.Get(v, "message"); err == nil && msg.IsString() {
			return msg.String()
		}
		if raw, err := rt.AppendJSON(nil, v); err == nil {
			return string(raw)
		}
	}
	return v.String()
}

// installGlobals 安装 console.* 与内部 __lunabox_commitTree 桥接。
func (p *Plugin) installGlobals(rt *moejs.Runtime) error {
	logArgs := func(level string, args []moejs.Value) {
		parts := make([]string, 0, len(args))
		for _, a := range args {
			parts = append(parts, p.formatJSValue(rt, a))
		}
		msg := strings.Join(parts, " ")
		switch level {
		case "warn":
			applog.LogWarningf(p.manager.ctx, "[plugin:%s] %s", p.ID, msg)
		case "error":
			applog.LogErrorf(p.manager.ctx, "[plugin:%s] %s", p.ID, msg)
		default:
			applog.LogInfof(p.manager.ctx, "[plugin:%s] %s", p.ID, msg)
		}
	}

	consoleObj := map[string]any{
		"log":   p.safeNative("log", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) { logArgs("info", args); return moejs.Undefined(), nil }),
		"info":  p.safeNative("info", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) { logArgs("info", args); return moejs.Undefined(), nil }),
		"debug": p.safeNative("debug", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) { logArgs("info", args); return moejs.Undefined(), nil }),
		"warn":  p.safeNative("warn", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) { logArgs("warn", args); return moejs.Undefined(), nil }),
		"error": p.safeNative("error", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) { logArgs("error", args); return moejs.Undefined(), nil }),
	}
	if err := rt.SetGlobal("console", consoleObj); err != nil {
		return err
	}

	// __lunabox_commitTree(instanceId, version, treeJson, errMsg)
	commitFn := p.safeNative("__lunabox_commitTree", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		instID := argString(args, 0)
		ver := uint64(argFloat(args, 1))
		treeJSON := argString(args, 2)
		errMsg := argString(args, 3)

		update := UITreeUpdate{
			InstanceID: instID,
			PluginID:   p.ID,
			Version:    ver,
			Error:      errMsg,
		}
		if errMsg == "" {
			tree, err := DecodeAndValidateUITree([]byte(treeJSON))
			if err != nil {
				update.Error = err.Error()
				p.recordError(err)
			} else {
				update.Tree = tree
			}
		} else {
			p.recordError(errors.New(errMsg))
		}
		p.enqueueTreeUpdate(update)
		return moejs.Undefined(), nil
	})
	return rt.SetGlobal("__lunabox_commitTree", commitFn)
}

// enqueueTreeUpdate 合并同一实例在投递前的多棵描述树，只保留最新一棵。
func (p *Plugin) enqueueTreeUpdate(u UITreeUpdate) {
	p.pendingMu.Lock()
	_, alreadyQueued := p.pendingTrees[u.InstanceID]
	p.pendingTrees[u.InstanceID] = u
	p.pendingMu.Unlock()

	if alreadyQueued {
		return
	}
	go func(instID string) {
		p.pendingMu.Lock()
		latest, ok := p.pendingTrees[instID]
		delete(p.pendingTrees, instID)
		p.pendingMu.Unlock()
		if ok && !p.closed.Load() {
			p.manager.ui.EmitTreeUpdate(latest)
		}
	}(u.InstanceID)
}

// safeNative 包装 Go 宿主函数，捕获所有 Go panic 并转成 JS 异常，确保运行时继续可用。
func (p *Plugin) safeNative(_ string, fn moejs.NativeFunc) moejs.NativeFunc {
	return func(r *moejs.Realm, this moejs.Value, args []moejs.Value) (ret moejs.Value, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				err = throwJSError(r, fmt.Sprintf("host panic: %v", rec))
			}
		}()
		return fn(r, this, args)
	}
}

// asyncNative 在后台 goroutine 执行可能阻塞的 IO/数据库操作，完成后通过 loop.NewPromise() settle。
func (p *Plugin) asyncNative(name string, work func(ctx context.Context, argsJSON []json.RawMessage) (any, error)) moejs.NativeFunc {
	return p.safeNative(name, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		rawArgs := make([]json.RawMessage, len(args))
		for i, a := range args {
			if a.IsUndefined() {
				continue
			}
			b, err := p.rt.AppendJSON(nil, a)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, fmt.Sprintf("参数 %d 无法序列化为 JSON: %v", i, err))
			}
			rawArgs[i] = b
		}

		promise, settle := p.loop.NewPromise()
		go func() {
			res, err := work(p.scope.Context(), rawArgs)
			settle(func(rt *moejs.Runtime) (moejs.Value, error) {
				if err != nil {
					return moejs.Undefined(), throwJSError(rt.Realm(), err.Error())
				}
				return toJSValue(rt, res)
			})
		}()
		return promise, nil
	})
}

func throwJSError(r *moejs.Realm, msg string) error {
	if r == nil {
		return errors.New(msg)
	}
	return r.Throw(r.NewErrorValue(engine.KindError, r.InternGoString(msg)))
}

func toJSValue(rt *moejs.Runtime, v any) (moejs.Value, error) {
	if v == nil {
		return moejs.Undefined(), nil
	}
	switch val := v.(type) {
	case moejs.Value:
		return val, nil
	case json.RawMessage:
		if len(val) == 0 {
			return moejs.Undefined(), nil
		}
		return rt.ParseJSON(val)
	case string, bool, int, int32, int64, uint, uint32, uint64, float32, float64:
		return rt.FromGo(val)
	default:
		// 结构体与复杂 map 统一走 JSON 序列化，确保使用 snake_case 的 json tag
		data, err := json.Marshal(v)
		if err != nil {
			return moejs.Undefined(), err
		}
		return rt.ParseJSON(data)
	}
}

func argString(args []moejs.Value, idx int) string {
	v := moejs.Arg(args, idx)
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	return v.String()
}

func argFloat(args []moejs.Value, idx int) float64 {
	v := moejs.Arg(args, idx)
	if v.IsNumber() {
		return v.AsNumber()
	}
	return 0
}

func argBool(args []moejs.Value, idx int) bool {
	v := moejs.Arg(args, idx)
	if v.IsBool() {
		return v.AsBool()
	}
	return false
}

// buildContextObject 组装传给 apply(ctx) 的完整 ctx 宿主 API 对象。
func (p *Plugin) buildContextObject(rt *moejs.Runtime) (moejs.Value, error) {
	ctxMap := map[string]any{
		"logger":     p.buildLoggerAPI(),
		"i18n":       p.buildI18nAPI(),
		"storage":    p.buildStorageAPI(),
		"config":     p.buildConfigAPI(),
		"games":      p.buildGamesAPI(),
		"categories": p.buildCategoriesAPI(),
		"metadata":   p.buildMetadataAPI(),
		"downloads":  p.buildDownloadsAPI(),
		"import":     p.buildImportAPI(),
		"theme":      p.buildThemeAPI(),
		"window":     p.buildWindowAPI(),
		"ai":         p.buildAIAPI(),
		"http":       p.buildHTTPAPI(),
		"on":         p.buildOnFunc(),
		"ui":         p.buildUIAPI(),
		"html":       p.buildHTMLAPI(),
		"shell":      p.buildShellAPI(),
		"audio":      p.buildAudioAPI(),
		"fs":         p.buildFSAPI(),
		"paths": map[string]any{
			"plugin": p.Dir,
			"data":   p.DataDir,
		},
		"effect": p.buildEffectFunc(),
	}
	return rt.FromGo(ctxMap)
}
