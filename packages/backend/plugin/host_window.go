package plugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MoeForge/moejs"
)

func windowInstanceID(pluginID, windowID string) string {
	return fmt.Sprintf("window:%s/%s", pluginID, windowID)
}

func (p *Plugin) buildWindowAPI() map[string]any {
	return map[string]any{
		"open": p.safeNative("window.open", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			windowID := strings.TrimSpace(argString(args, 0))
			optsVal := moejs.Arg(args, 1)
			if windowID == "" || !optsVal.IsObject() {
				return moejs.Undefined(), throwJSError(r, "window.open 需要 windowId 和配置对象")
			}
			raw, err := p.rt.AppendJSON(nil, optsVal)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			var opts struct {
				Title       string `json:"title"`
				Width       int    `json:"width"`
				Height      int    `json:"height"`
				X           *int   `json:"x"`
				Y           *int   `json:"y"`
				Frameless   bool   `json:"frameless"`
				Transparent bool   `json:"transparent"`
				AlwaysOnTop bool   `json:"alwaysOnTop"`
				SkipTaskbar bool   `json:"skipTaskbar"`
				Resizable   *bool  `json:"resizable"`
			}
			if err := json.Unmarshal(raw, &opts); err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			if opts.Width <= 0 {
				opts.Width = 320
			}
			if opts.Height <= 0 {
				opts.Height = 240
			}
			resizable := true
			if opts.Resizable != nil {
				resizable = *opts.Resizable
			}
			renderFn, err := p.rt.Get(optsVal, "render")
			if err != nil || !renderFn.IsObject() {
				return moejs.Undefined(), throwJSError(r, "window.open 需要 render 组件函数")
			}

			instID := windowInstanceID(p.ID, windowID)
			key := slotCallbackKey("window", windowID)
			cbs := &windowCallbacks{}
			p.mu.Lock()
			p.renderFns[key] = renderFn
			p.winHandlers[windowID] = cbs
			p.mu.Unlock()

			spec := PluginWindowSpec{
				PluginID:    p.ID,
				WindowID:    windowID,
				InstanceID:  instID,
				Title:       opts.Title,
				Width:       opts.Width,
				Height:      opts.Height,
				X:           opts.X,
				Y:           opts.Y,
				Frameless:   opts.Frameless,
				Transparent: opts.Transparent,
				AlwaysOnTop: opts.AlwaysOnTop,
				SkipTaskbar: opts.SkipTaskbar,
				Resizable:   resizable,
			}

			unreg := p.manager.ui.RegisterWindow(spec)
			// 立即挂载该独立窗口对应的 UI 实例
			p.mountInternal(instID, renderFn, "{}")

			dispose := p.scope.Effect(func() {
				p.unmountInternal(instID)
				unreg()
				p.mu.Lock()
				delete(p.renderFns, key)
				delete(p.winHandlers, windowID)
				p.mu.Unlock()
			})

			handle := map[string]any{
				"id": windowID,
				"setSize": p.safeNative("window.setSize", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					p.manager.ui.EmitWindowCommand(WindowCommand{
						Action:   "setSize",
						PluginID: p.ID,
						WindowID: windowID,
						Width:    int(argFloat(args, 0)),
						Height:   int(argFloat(args, 1)),
					})
					return moejs.Undefined(), nil
				}),
				"setPosition": p.safeNative("window.setPosition", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					p.manager.ui.EmitWindowCommand(WindowCommand{
						Action:   "setPosition",
						PluginID: p.ID,
						WindowID: windowID,
						X:        int(argFloat(args, 0)),
						Y:        int(argFloat(args, 1)),
					})
					return moejs.Undefined(), nil
				}),
				"setAlwaysOnTop": p.safeNative("window.setAlwaysOnTop", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					p.manager.ui.EmitWindowCommand(WindowCommand{
						Action:      "setAlwaysOnTop",
						PluginID:    p.ID,
						WindowID:    windowID,
						AlwaysOnTop: argBool(args, 0),
					})
					return moejs.Undefined(), nil
				}),
				"show": p.safeNative("window.show", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
					p.manager.ui.EmitWindowCommand(WindowCommand{Action: "show", PluginID: p.ID, WindowID: windowID})
					return moejs.Undefined(), nil
				}),
				"hide": p.safeNative("window.hide", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
					p.manager.ui.EmitWindowCommand(WindowCommand{Action: "hide", PluginID: p.ID, WindowID: windowID})
					return moejs.Undefined(), nil
				}),
				"close": p.safeNative("window.close", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
					dispose()
					return moejs.Undefined(), nil
				}),
				"onMove": p.safeNative("window.onMove", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					fn := moejs.Arg(args, 0)
					if fn.IsObject() {
						p.mu.Lock()
						cbs.onMove = append(cbs.onMove, fn)
						p.mu.Unlock()
					}
					return moejs.Undefined(), nil
				}),
				"onFocus": p.safeNative("window.onFocus", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					fn := moejs.Arg(args, 0)
					if fn.IsObject() {
						p.mu.Lock()
						cbs.onFocus = append(cbs.onFocus, fn)
						p.mu.Unlock()
					}
					return moejs.Undefined(), nil
				}),
				"onClose": p.safeNative("window.onClose", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
					fn := moejs.Arg(args, 0)
					if fn.IsObject() {
						p.mu.Lock()
						cbs.onClose = append(cbs.onClose, fn)
						p.mu.Unlock()
					}
					return moejs.Undefined(), nil
				}),
			}
			return p.rt.FromGo(handle)
		}),
		"main": map[string]any{
			"show": p.safeNative("window.main.show", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				p.manager.ui.EmitWindowCommand(WindowCommand{Action: "main.show", PluginID: p.ID})
				return moejs.Undefined(), nil
			}),
			"focus": p.safeNative("window.main.focus", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				p.manager.ui.EmitWindowCommand(WindowCommand{Action: "main.focus", PluginID: p.ID})
				return moejs.Undefined(), nil
			}),
			"minimize": p.safeNative("window.main.minimize", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				p.manager.ui.EmitWindowCommand(WindowCommand{Action: "main.minimize", PluginID: p.ID})
				return moejs.Undefined(), nil
			}),
			"isVisible": p.safeNative("window.main.isVisible", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				if p.manager.mainWindowVisible != nil {
					return p.rt.FromGo(p.manager.mainWindowVisible())
				}
				return p.rt.FromGo(true)
			}),
		},
	}
}

// NotifyWindowEvent 由桌面端独立子窗口把 move/focus/close 事件传回插件线程。
func (p *Plugin) NotifyWindowEvent(windowID, event string, payload map[string]any) {
	p.mu.RLock()
	cbs := p.winHandlers[windowID]
	if cbs == nil {
		p.mu.RUnlock()
		return
	}
	var fns []moejs.Value
	switch event {
	case "move":
		fns = append([]moejs.Value(nil), cbs.onMove...)
	case "focus":
		fns = append([]moejs.Value(nil), cbs.onFocus...)
	case "close":
		fns = append([]moejs.Value(nil), cbs.onClose...)
	}
	p.mu.RUnlock()
	if len(fns) == 0 {
		return
	}
	raw, _ := json.Marshal(payload)
	p.Post(func(rt *moejs.Runtime) {
		var arg moejs.Value = moejs.Undefined()
		if len(raw) > 0 && string(raw) != "null" {
			if v, err := rt.ParseJSON(raw); err == nil {
				arg = v
			}
		}
		for _, fn := range fns {
			if _, err := rt.CallFunction(fn, moejs.Undefined(), arg); err != nil {
				p.recordError(err)
			}
		}
	})
}
