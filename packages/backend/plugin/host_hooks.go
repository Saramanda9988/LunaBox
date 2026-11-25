package plugin

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/MoeForge/moejs"

	"lunabox/backend/common/hooks"
)

type hookListener struct {
	id       uint64
	plugin   *Plugin
	event    string
	fn       moejs.Value
	prepend  bool
	sequence uint64
	timeout  time.Duration // 仅拦截器使用；0 表示默认值
}

const (
	defaultInterceptTimeout = 3 * time.Second
	maxInterceptTimeout     = 10 * time.Minute
)

// HookBus 实现 hooks.Bus 接口，管理所有插件的事件监听与钩子执行顺序。
type HookBus struct {
	manager   *Manager
	mu        sync.RWMutex
	nextID    uint64
	listeners map[string][]*hookListener
}

func newHookBus(m *Manager) *HookBus {
	return &HookBus{
		manager:   m,
		listeners: make(map[string][]*hookListener),
	}
}

func (b *HookBus) add(p *Plugin, event string, fn moejs.Value, prepend bool) func() {
	return b.addWithTimeout(p, event, fn, prepend, 0)
}

func (b *HookBus) addWithTimeout(p *Plugin, event string, fn moejs.Value, prepend bool, timeout time.Duration) func() {
	b.mu.Lock()
	b.nextID++
	l := &hookListener{
		id:       b.nextID,
		plugin:   p,
		event:    event,
		fn:       fn,
		prepend:  prepend,
		sequence: b.nextID,
		timeout:  timeout,
	}
	b.listeners[event] = append(b.listeners[event], l)
	b.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			list := b.listeners[event]
			list = slices.DeleteFunc(list, func(item *hookListener) bool {
				return item.id == l.id
			})
			if len(list) == 0 {
				delete(b.listeners, event)
			} else {
				b.listeners[event] = list
			}
			b.mu.Unlock()
		})
	}
}

func (b *HookBus) orderedListeners(event string) []*hookListener {
	b.mu.RLock()
	raw := b.listeners[event]
	if len(raw) == 0 {
		b.mu.RUnlock()
		return nil
	}
	list := slices.Clone(raw)
	b.mu.RUnlock()

	orderMap := b.manager.pluginOrderMap()
	slices.SortStableFunc(list, func(a, c *hookListener) int {
		if a.prepend != c.prepend {
			if a.prepend {
				return -1
			}
			return 1
		}
		oa, oka := orderMap[a.plugin.ID]
		oc, okc := orderMap[c.plugin.ID]
		if oka && okc && oa != oc {
			if oa < oc {
				return -1
			}
			return 1
		}
		if a.sequence < c.sequence {
			return -1
		}
		if a.sequence > c.sequence {
			return 1
		}
		return 0
	})
	return list
}

func (b *HookBus) HasListeners(event string) bool {
	b.mu.RLock()
	n := len(b.listeners[event])
	b.mu.RUnlock()
	return n > 0
}

func (b *HookBus) Emit(event string, payload any) {
	list := b.orderedListeners(event)
	if len(list) == 0 {
		return
	}
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		raw = data
	}
	// emit 在后台 goroutine 投递，不阻塞调用方
	go func() {
		for _, l := range list {
			listener := l
			listener.plugin.Post(func(rt *moejs.Runtime) {
				var args []moejs.Value
				if len(raw) > 0 {
					v, err := rt.ParseJSON(raw)
					if err != nil {
						listener.plugin.recordError(err)
						return
					}
					args = []moejs.Value{v}
				}
				if _, err := rt.CallFunction(listener.fn, moejs.Undefined(), args...); err != nil {
					listener.plugin.recordError(err)
				}
			})
		}
	}()
}

func (b *HookBus) Waterfall(ctx context.Context, event string, initial any) (json.RawMessage, error) {
	list := b.orderedListeners(event)
	if len(list) == 0 {
		return nil, nil
	}
	cur, err := json.Marshal(initial)
	if err != nil {
		return nil, err
	}
	for _, l := range list {
		if ctx.Err() != nil {
			break
		}
		next, callErr := l.plugin.Call(ctx, l.fn, json.RawMessage(cur))
		if callErr != nil || len(next) == 0 || string(next) == "undefined" {
			// 出错、超时或返回 undefined：跳过该监听者，沿用上一个值
			continue
		}
		cur = next
	}
	return cur, nil
}

func (b *HookBus) Bail(ctx context.Context, event string, payload any) (json.RawMessage, bool, error) {
	list := b.orderedListeners(event)
	if len(list) == 0 {
		return nil, false, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	for _, l := range list {
		if ctx.Err() != nil {
			break
		}
		res, callErr := l.plugin.Call(ctx, l.fn, json.RawMessage(raw))
		if callErr != nil || len(res) == 0 || string(res) == "undefined" {
			continue
		}
		return res, true, nil
	}
	return nil, false, nil
}

func (b *HookBus) Intercept(ctx context.Context, event string, payload any) (json.RawMessage, error) {
	list := b.orderedListeners(event)
	if len(list) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return nil, err
	}
	for _, l := range list {
		if ctx.Err() != nil {
			break
		}
		timeout := l.timeout
		if timeout <= 0 {
			timeout = defaultInterceptTimeout
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		res, callErr := l.plugin.Call(callCtx, l.fn, cur)
		cancel()
		if callErr != nil || len(res) == 0 || string(res) == "undefined" {
			continue
		}
		var patch map[string]any
		if json.Unmarshal(res, &patch) != nil {
			continue
		}
		mergeJSONObject(cur, patch)
		if cancelled, _ := cur["cancel"].(bool); cancelled {
			break
		}
	}
	return json.Marshal(cur)
}

// mergeJSONObject 把 patch 深合并进 dst：两边都是对象时递归，其余（含数组与 null）整体替换。
func mergeJSONObject(dst, patch map[string]any) {
	for k, v := range patch {
		if pv, ok := v.(map[string]any); ok {
			if dv, ok := dst[k].(map[string]any); ok {
				mergeJSONObject(dv, pv)
				continue
			}
		}
		dst[k] = v
	}
}

var _ hooks.Bus = (*HookBus)(nil)

func (p *Plugin) buildOnFunc() moejs.NativeFunc {
	return p.safeNative("ctx.on", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		event := argString(args, 0)
		fn := moejs.Arg(args, 1)
		if event == "" || !fn.IsObject() {
			return moejs.Undefined(), throwJSError(r, "ctx.on 需要事件名和监听函数")
		}
		prepend := false
		var timeout time.Duration
		if len(args) > 2 && args[2].IsObject() {
			if v, err := p.rt.Get(args[2], "prepend"); err == nil && v.IsBool() {
				prepend = v.AsBool()
			}
			// timeout 只对拦截器（launch/*）生效：需要等用户操作的拦截器可以放宽到最多 10 分钟
			if v, err := p.rt.Get(args[2], "timeout"); err == nil && v.IsNumber() && v.AsNumber() > 0 {
				timeout = min(time.Duration(v.AsNumber())*time.Millisecond, maxInterceptTimeout)
			}
		}
		dispose := p.scope.Effect(p.manager.hooks.addWithTimeout(p, event, fn, prepend, timeout))
		return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			dispose()
			return moejs.Undefined(), nil
		}), nil
	})
}
