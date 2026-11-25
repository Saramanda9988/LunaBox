package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
)

func slotCallbackKey(slotName, id string) string {
	return slotName + "::" + id
}

func (p *Plugin) buildUIAPI() map[string]any {
	api := map[string]any{
		"slot": p.safeNative("ui.slot", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			slotName := strings.TrimSpace(argString(args, 0))
			def := moejs.Arg(args, 1)
			if slotName == "" || !def.IsObject() {
				return moejs.Undefined(), throwJSError(r, "ui.slot 需要插槽名称和注册定义对象")
			}
			// app.decoration 等价于 ctx.theme.registerDecoration
			if slotName == SlotAppDecoration {
				return p.registerDecorationFromValue(r, def)
			}

			idVal, _ := p.rt.Get(def, "id")
			id := strings.TrimSpace(idVal.String())
			if id == "" {
				return moejs.Undefined(), throwJSError(r, "ui.slot 注册定义缺少 id")
			}
			getStr := func(k string) string {
				if v, err := p.rt.Get(def, k); err == nil && v.IsString() {
					return strings.TrimSpace(v.String())
				}
				return ""
			}
			defaultWidth := 0
			if v, err := p.rt.Get(def, "defaultWidth"); err == nil && v.IsNumber() {
				defaultWidth = int(v.AsNumber())
			}
			order := 0
			if v, err := p.rt.Get(def, "order"); err == nil && v.IsNumber() {
				order = int(v.AsNumber())
			}
			var hideTabs []string
			if v, err := p.rt.Get(def, "hideTabs"); err == nil && v.IsObject() {
				if raw, merr := p.rt.AppendJSON(nil, v); merr == nil {
					_ = json.Unmarshal(raw, &hideTabs)
				}
			}

			renderFn, _ := p.rt.Get(def, "render")
			onPressFn, _ := p.rt.Get(def, "onPress")
			runFn, _ := p.rt.Get(def, "run")
			whenFn, _ := p.rt.Get(def, "when")

			key := slotCallbackKey(slotName, id)
			p.mu.Lock()
			if renderFn.IsObject() {
				p.renderFns[key] = renderFn
			}
			if onPressFn.IsObject() {
				p.actionFns[key] = onPressFn
			} else if runFn.IsObject() {
				p.actionFns[key] = runFn
			}
			if whenFn.IsObject() {
				p.whenFns[key] = whenFn
			}
			p.mu.Unlock()

			unreg := p.manager.ui.RegisterSlot(SlotRegistration{
				SlotName:     slotName,
				PluginID:     p.ID,
				PluginName:   p.Manifest.DisplayName(p.manager.currentLanguage()),
				ID:           id,
				Title:        getStr("title"),
				Icon:         getStr("icon"),
				Group:        getStr("group"),
				Badge:        getStr("badge"),
				Mode:         getStr("mode"),
				DefaultWidth: defaultWidth,
				Order:        order,
				HideTabs:     hideTabs,
				HasRender:    renderFn.IsObject(),
				HasOnPress:   onPressFn.IsObject(),
				HasRun:       runFn.IsObject(),
				HasWhen:      whenFn.IsObject(),
			})
			dispose := p.scope.Effect(func() {
				unreg()
				p.mu.Lock()
				delete(p.renderFns, key)
				delete(p.actionFns, key)
				delete(p.whenFns, key)
				p.mu.Unlock()
			})
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
		"toast": p.safeNative("ui.toast", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			msg := argString(args, 0)
			level := strings.ToLower(strings.TrimSpace(argString(args, 1)))
			if level == "" {
				level = "info"
			}
			p.manager.ui.EmitAction(UIAction{
				Type:     "toast",
				PluginID: p.ID,
				Message:  msg,
				Level:    level,
			})
			return moejs.Undefined(), nil
		}),
		"navigate": p.safeNative("ui.navigate", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			path := strings.TrimSpace(argString(args, 0))
			var params map[string]any
			if len(args) > 1 && args[1].IsObject() {
				if raw, err := p.rt.AppendJSON(nil, args[1]); err == nil {
					_ = json.Unmarshal(raw, &params)
				}
			}
			p.manager.ui.EmitAction(UIAction{
				Type:     "navigate",
				PluginID: p.ID,
				Path:     path,
				Params:   params,
			})
			return moejs.Undefined(), nil
		}),
		"openDrawer": p.safeNative("ui.openDrawer", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			drawerID := strings.TrimSpace(argString(args, 0))
			var props json.RawMessage
			if len(args) > 1 && !args[1].IsUndefined() {
				if raw, err := p.rt.AppendJSON(nil, args[1]); err == nil {
					props = raw
				}
			}
			p.manager.ui.EmitAction(UIAction{
				Type:     "openDrawer",
				PluginID: p.ID,
				DrawerID: drawerID,
				Props:    props,
			})
			return moejs.Undefined(), nil
		}),
		"closeDrawer": p.safeNative("ui.closeDrawer", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			p.manager.ui.EmitAction(UIAction{
				Type:     "closeDrawer",
				PluginID: p.ID,
			})
			return moejs.Undefined(), nil
		}),
	}
	for k, v := range p.buildDialogAPI() {
		api[k] = v
	}
	return api
}

// MountSlotInstance 在插件线程挂载一个插槽实例，新建子作用域并触发首次 render。
func (p *Plugin) MountSlotInstance(instanceID, slotName, regID string, props any) {
	key := slotCallbackKey(slotName, regID)
	p.mu.RLock()
	renderFn, ok := p.renderFns[key]
	p.mu.RUnlock()
	if !ok || !renderFn.IsObject() {
		return
	}
	p.mountInternal(instanceID, renderFn, slotPropsJSON(props))
}

// UpdateSlotInstance 用新的 props 重新渲染已挂载的插槽实例，保留 useState / useEffect 等 hook 状态
// （等同 React 父组件带着新 props 重渲染）；实例尚未挂载时退化为首次挂载。
func (p *Plugin) UpdateSlotInstance(instanceID, slotName, regID string, props any) {
	key := slotCallbackKey(slotName, regID)
	p.mu.RLock()
	renderFn, ok := p.renderFns[key]
	_, mounted := p.instScopes[instanceID]
	p.mu.RUnlock()
	if !ok || !renderFn.IsObject() {
		return
	}
	propsJSON := slotPropsJSON(props)
	if !mounted {
		p.mountInternal(instanceID, renderFn, propsJSON)
		return
	}
	p.Post(func(rt *moejs.Runtime) {
		fn, err := rt.Get(engine.ObjectValue(rt.Realm().Global), "__lunabox_ui_update")
		if err != nil || !fn.IsObject() {
			p.recordError(fmt.Errorf("__lunabox_ui_update 不可用: %v", err))
			return
		}
		if _, err := rt.CallFunction(fn, moejs.Undefined(), moejs.String(instanceID), renderFn, moejs.String(propsJSON)); err != nil {
			p.recordError(err)
		}
	})
}

func slotPropsJSON(props any) string {
	if props != nil {
		if b, err := json.Marshal(props); err == nil {
			return string(b)
		}
	}
	return "{}"
}

func (p *Plugin) mountInternal(instanceID string, renderFn moejs.Value, propsJSON string) {
	child := p.scope.Fork()
	p.mu.Lock()
	if prev, exists := p.instScopes[instanceID]; exists {
		prev.Dispose()
	}
	p.instScopes[instanceID] = child
	p.mu.Unlock()

	child.Effect(func() {
		p.Post(func(rt *moejs.Runtime) {
			if fn, err := rt.Get(engine.ObjectValue(rt.Realm().Global), "__lunabox_ui_unmount"); err == nil && fn.IsObject() {
				_, _ = rt.CallFunction(fn, moejs.Undefined(), moejs.String(instanceID))
			}
		})
	})

	p.Post(func(rt *moejs.Runtime) {
		fn, err := rt.Get(engine.ObjectValue(rt.Realm().Global), "__lunabox_ui_mount")
		if err != nil || !fn.IsObject() {
			p.recordError(fmt.Errorf("__lunabox_ui_mount 不可用: %v", err))
			return
		}
		if _, err := rt.CallFunction(fn, moejs.Undefined(), moejs.String(instanceID), renderFn, moejs.String(propsJSON)); err != nil {
			p.recordError(err)
		}
	})
}

// UnmountSlotInstance 卸载插槽实例并撤销其子作用域（包括 useEffect 清理函数）。
func (p *Plugin) UnmountSlotInstance(instanceID string) {
	p.unmountInternal(instanceID)
}

func (p *Plugin) unmountInternal(instanceID string) {
	p.mu.Lock()
	child, ok := p.instScopes[instanceID]
	delete(p.instScopes, instanceID)
	p.mu.Unlock()
	if ok && child != nil {
		child.Dispose()
	}
}

// DispatchUIEvent 把界面事件非阻塞投递到插件线程。
func (p *Plugin) DispatchUIEvent(ev UIEvent) {
	payloadJSON := ""
	if ev.Payload != nil {
		if b, err := json.Marshal(ev.Payload); err == nil {
			payloadJSON = string(b)
		}
	}
	p.Post(func(rt *moejs.Runtime) {
		fn, err := rt.Get(engine.ObjectValue(rt.Realm().Global), "__lunabox_ui_dispatch")
		if err != nil || !fn.IsObject() {
			return
		}
		verVal, _ := rt.FromGo(ev.Version)
		if _, err := rt.CallFunction(fn, moejs.Undefined(), moejs.String(ev.InstanceID), verVal, moejs.String(ev.HandlerID), moejs.String(payloadJSON)); err != nil {
			p.recordError(err)
		}
	})
}

// InvokeSlotAction 调用非描述树插槽的 onPress / run 回调（例如 topbar.item、library.add_menu、library.toolbar、game.actions）。
func (p *Plugin) InvokeSlotAction(slotName, regID string, payload any) {
	key := slotCallbackKey(slotName, regID)
	p.mu.RLock()
	fn, ok := p.actionFns[key]
	p.mu.RUnlock()
	if !ok || !fn.IsObject() {
		return
	}
	var raw json.RawMessage
	if payload != nil {
		raw, _ = json.Marshal(payload)
	}
	p.Post(func(rt *moejs.Runtime) {
		var args []moejs.Value
		if len(raw) > 0 && string(raw) != "null" {
			if v, err := rt.ParseJSON(raw); err == nil {
				args = []moejs.Value{v}
			}
		}
		res, err := rt.CallFunction(fn, moejs.Undefined(), args...)
		if err != nil {
			p.recordError(err)
			return
		}
		if _, pRes, isP := moejs.PromiseResult(res); isP && !pRes.IsUndefined() {
			// 挂个 catch 防止 unhandled rejection
			if catchFn, cerr := rt.Get(res, "catch"); cerr == nil && catchFn.IsObject() {
				onErr := rt.Function("onErr", 1, func(_ *moejs.Realm, _ moejs.Value, a []moejs.Value) (moejs.Value, error) {
					p.recordError(fmt.Errorf("%s", p.formatJSValue(rt, moejs.Arg(a, 0))))
					return moejs.Undefined(), nil
				})
				_, _ = rt.CallFunction(catchFn, res, onErr)
			}
		}
	})
}

// CheckSlotWhen 评估 game.actions 的 when(game) 条件函数（限时 500ms），无 when 函数时返回 true。
func (p *Plugin) CheckSlotWhen(slotName, regID string, payload any) bool {
	key := slotCallbackKey(slotName, regID)
	p.mu.RLock()
	fn, ok := p.whenFns[key]
	p.mu.RUnlock()
	if !ok || !fn.IsObject() {
		return true
	}
	ctx, cancel := context.WithTimeout(p.scope.Context(), 500*time.Millisecond)
	defer cancel()
	raw, err := p.Call(ctx, fn, payload)
	if err != nil || len(raw) == 0 {
		return true
	}
	var res bool
	if json.Unmarshal(raw, &res) == nil {
		return res
	}
	return true
}

// ─────────────────────────── ctx.ui.confirm / select ───────────────────────────

// openDialog 请求界面显示对话框，返回在用户作答（或插件卸载）时 settle 的 Promise。
// cancelValue 是取消或卸载时的结果；toResult 把界面回传的 JSON 转成 JS 返回值。
func (p *Plugin) openDialog(spec DialogSpec, cancelValue any, toResult func(raw json.RawMessage) any) moejs.Value {
	promise, settle := p.loop.NewPromise()
	p.mu.Lock()
	p.nextDialog++
	id := fmt.Sprintf("%s-dialog-%d", p.ID, p.nextDialog)
	p.mu.Unlock()

	var settled atomic.Bool
	// finish 只生效一次，返回是否由这次调用 settle
	finish := func(raw json.RawMessage) bool {
		if !settled.CompareAndSwap(false, true) {
			return false
		}
		p.mu.Lock()
		delete(p.dialogs, id)
		p.mu.Unlock()
		value := cancelValue
		if len(raw) > 0 && string(raw) != "null" {
			value = toResult(raw)
		}
		settle(func(rt *moejs.Runtime) (moejs.Value, error) {
			return toJSValue(rt, value)
		})
		return true
	}
	// 插件卸载时关闭界面上的对话框，并按取消处理
	dispose := p.scope.Effect(func() {
		if finish(nil) {
			p.manager.ui.EmitAction(UIAction{Type: "closeDialog", PluginID: p.ID, DialogID: id})
		}
	})
	p.mu.Lock()
	p.dialogs[id] = func(raw json.RawMessage) {
		if finish(raw) {
			dispose() // 已作答：撤销卸载清理（finish 再次调用为空操作）
		}
	}
	p.mu.Unlock()

	// 主窗口隐藏时（如从快捷方式启动游戏、启动拦截器弹出账号选择）先把它显示出来
	if visible := p.manager.mainWindowVisible; visible != nil && !visible() {
		if rt := p.manager.deps.Runtime; rt != nil {
			rt.ShowWindow()
		}
	}
	p.manager.ui.EmitAction(UIAction{Type: "dialog", PluginID: p.ID, DialogID: id, Dialog: &spec})
	return promise
}

func (p *Plugin) resolveDialog(id string, result json.RawMessage) {
	p.mu.RLock()
	finish := p.dialogs[id]
	p.mu.RUnlock()
	if finish != nil {
		finish(result)
	}
}

func (p *Plugin) parseDialogSpec(r *moejs.Realm, v moejs.Value, kind string) (DialogSpec, error) {
	var spec DialogSpec
	if !v.IsObject() {
		return spec, throwJSError(r, "ui."+kind+" 需要选项对象")
	}
	raw, err := p.rt.AppendJSON(nil, v)
	if err != nil {
		return spec, throwJSError(r, err.Error())
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, throwJSError(r, "ui."+kind+" 选项格式错误: "+err.Error())
	}
	spec.Kind = kind
	if strings.TrimSpace(spec.Title) == "" {
		spec.Title = p.Manifest.DisplayName(p.manager.currentLanguage())
	}
	return spec, nil
}

func (p *Plugin) buildDialogAPI() map[string]any {
	return map[string]any{
		"confirm": p.safeNative("ui.confirm", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			spec, err := p.parseDialogSpec(r, moejs.Arg(args, 0), "confirm")
			if err != nil {
				return moejs.Undefined(), err
			}
			return p.openDialog(spec, false, func(raw json.RawMessage) any {
				var ok bool
				_ = json.Unmarshal(raw, &ok)
				return ok
			}), nil
		}),
		"select": p.safeNative("ui.select", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			spec, err := p.parseDialogSpec(r, moejs.Arg(args, 0), "select")
			if err != nil {
				return moejs.Undefined(), err
			}
			if len(spec.Options) == 0 {
				return moejs.Undefined(), throwJSError(r, "ui.select 至少需要一个选项")
			}
			// 取消时返回 null（Go 的 nil 会变成 undefined）
			return p.openDialog(spec, json.RawMessage("null"), func(raw json.RawMessage) any {
				var value string
				if json.Unmarshal(raw, &value) != nil {
					return nil
				}
				return value
			}), nil
		}),
	}
}
