package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
	"github.com/PuerkitoBio/goquery"
	"github.com/natefinch/atomic"

	"lunabox/backend/applog"
	"lunabox/backend/utils/apputils"
)

const configStorageReservedKey = "__lunabox_config__"

// ─────────────────────────── ctx.logger ───────────────────────────

func (p *Plugin) buildLoggerAPI() map[string]any {
	logWith := func(level string) moejs.NativeFunc {
		return p.safeNative("logger."+level, func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			parts := make([]string, 0, len(args))
			for _, a := range args {
				parts = append(parts, p.formatJSValue(p.rt, a))
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
			return moejs.Undefined(), nil
		})
	}
	return map[string]any{
		"info":  logWith("info"),
		"warn":  logWith("warn"),
		"error": logWith("error"),
	}
}

// ─────────────────────────── ctx.i18n ───────────────────────────

type pluginI18n struct {
	p         *Plugin
	mu        sync.RWMutex
	bundles   map[string]map[string]string
	listeners map[uint64]moejs.Value
	nextID    uint64
}

func newPluginI18n(p *Plugin) *pluginI18n {
	i := &pluginI18n{
		p:         p,
		bundles:   make(map[string]map[string]string),
		listeners: make(map[uint64]moejs.Value),
	}
	localesDir := filepath.Join(p.Dir, "locales")
	entries, err := os.ReadDir(localesDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
				continue
			}
			lang := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			data, rerr := os.ReadFile(filepath.Join(localesDir, entry.Name()))
			if rerr != nil {
				continue
			}
			var raw map[string]any
			if json.Unmarshal(data, &raw) == nil {
				flat := make(map[string]string)
				flattenLocaleMap("", raw, flat)
				i.bundles[lang] = flat
			}
		}
	}
	return i
}

func flattenLocaleMap(prefix string, src map[string]any, dst map[string]string) {
	for k, v := range src {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			dst[full] = val
		case map[string]any:
			flattenLocaleMap(full, val, dst)
		}
	}
}

func (i *pluginI18n) translate(key string, args map[string]any) string {
	lang := i.p.manager.currentLanguage()
	i.mu.RLock()
	tpl := ""
	if b, ok := i.bundles[lang]; ok {
		tpl = b[key]
	}
	if tpl == "" {
		if idx := strings.IndexAny(lang, "-_"); idx > 0 {
			prefix := strings.ToLower(lang[:idx])
			for k, b := range i.bundles {
				if strings.HasPrefix(strings.ToLower(k), prefix) {
					if v := b[key]; v != "" {
						tpl = v
						break
					}
				}
			}
		}
	}
	if tpl == "" {
		if b, ok := i.bundles["en-US"]; ok {
			tpl = b[key]
		}
	}
	if tpl == "" {
		if b, ok := i.bundles["en"]; ok {
			tpl = b[key]
		}
	}
	i.mu.RUnlock()
	if tpl == "" {
		return key
	}
	for k, v := range args {
		tpl = strings.ReplaceAll(tpl, "{"+k+"}", fmt.Sprint(v))
	}
	return tpl
}

func (i *pluginI18n) notifyLanguageChange(lang string) {
	i.mu.RLock()
	fns := make([]moejs.Value, 0, len(i.listeners))
	for _, fn := range i.listeners {
		fns = append(fns, fn)
	}
	i.mu.RUnlock()
	if len(fns) == 0 {
		return
	}
	i.p.Post(func(rt *moejs.Runtime) {
		v := moejs.String(lang)
		for _, fn := range fns {
			if _, err := rt.CallFunction(fn, moejs.Undefined(), v); err != nil {
				i.p.recordError(err)
			}
		}
	})
}

func (p *Plugin) buildI18nAPI() map[string]any {
	return map[string]any{
		"language": p.manager.currentLanguage(),
		"getLanguage": p.safeNative("i18n.getLanguage", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			return moejs.String(p.manager.currentLanguage()), nil
		}),
		"t": p.safeNative("i18n.t", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			key := argString(args, 0)
			var params map[string]any
			if len(args) > 1 && args[1].IsObject() {
				if raw, err := p.rt.AppendJSON(nil, args[1]); err == nil {
					_ = json.Unmarshal(raw, &params)
				}
			}
			return moejs.String(p.i18n.translate(key, params)), nil
		}),
		"onChange": p.safeNative("i18n.onChange", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			fn := moejs.Arg(args, 0)
			if !fn.IsObject() {
				return moejs.Undefined(), throwJSError(r, "i18n.onChange 需要函数参数")
			}
			p.i18n.mu.Lock()
			p.i18n.nextID++
			id := p.i18n.nextID
			p.i18n.listeners[id] = fn
			p.i18n.mu.Unlock()
			dispose := p.scope.Effect(func() {
				p.i18n.mu.Lock()
				delete(p.i18n.listeners, id)
				p.i18n.mu.Unlock()
			})
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
	}
}

// ─────────────────────────── ctx.storage ───────────────────────────

type pluginStorage struct {
	p     *Plugin
	path  string
	mu    sync.Mutex
	data  map[string]json.RawMessage
	timer *time.Timer
	dirty bool
}

func newPluginStorage(p *Plugin) *pluginStorage {
	s := &pluginStorage{
		p:    p,
		path: filepath.Join(p.DataDir, "storage.json"),
		data: make(map[string]json.RawMessage),
	}
	if raw, err := os.ReadFile(s.path); err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &s.data)
	}
	if s.data == nil {
		s.data = make(map[string]json.RawMessage)
	}
	return s
}

func (s *pluginStorage) Get(key string) (json.RawMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return nil, false
	}
	cp := make(json.RawMessage, len(v))
	copy(cp, v)
	return cp, true
}

func (s *pluginStorage) Set(key string, val json.RawMessage) {
	s.mu.Lock()
	cp := make(json.RawMessage, len(val))
	copy(cp, val)
	s.data[key] = cp
	s.dirty = true
	if s.timer != nil {
		s.timer.Reset(300 * time.Millisecond)
	} else {
		s.timer = time.AfterFunc(300*time.Millisecond, s.FlushSync)
	}
	s.mu.Unlock()
}

func (s *pluginStorage) Delete(key string) {
	s.mu.Lock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.dirty = true
		if s.timer != nil {
			s.timer.Reset(300 * time.Millisecond)
		} else {
			s.timer = time.AfterFunc(300*time.Millisecond, s.FlushSync)
		}
	}
	s.mu.Unlock()
}

func (s *pluginStorage) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		if k == configStorageReservedKey {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (s *pluginStorage) FlushSync() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if !s.dirty {
		s.mu.Unlock()
		return
	}
	s.dirty = false
	data, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	_ = atomic.WriteFile(s.path, bytes.NewReader(data))
}

func (p *Plugin) buildStorageAPI() map[string]any {
	return map[string]any{
		"get": p.safeNative("storage.get", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			key := argString(args, 0)
			raw, ok := p.storage.Get(key)
			if !ok {
				return moejs.Undefined(), nil
			}
			return p.rt.ParseJSON(raw)
		}),
		"set": p.safeNative("storage.set", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			key := argString(args, 0)
			if key == "" {
				return moejs.Undefined(), throwJSError(r, "storage.set 需要非空 key")
			}
			v := moejs.Arg(args, 1)
			if v.IsUndefined() {
				p.storage.Delete(key)
				return moejs.Undefined(), nil
			}
			raw, err := p.rt.AppendJSON(nil, v)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			p.storage.Set(key, raw)
			return moejs.Undefined(), nil
		}),
		"delete": p.safeNative("storage.delete", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			p.storage.Delete(argString(args, 0))
			return moejs.Undefined(), nil
		}),
		"keys": p.safeNative("storage.keys", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			return toJSValue(p.rt, p.storage.Keys())
		}),
	}
}

// ─────────────────────────── ctx.config ───────────────────────────

type pluginConfig struct {
	p         *Plugin
	mu        sync.RWMutex
	schema    *ConfigSchema
	values    map[string]any
	listeners map[uint64]moejs.Value
	nextID    uint64
}

func newPluginConfig(p *Plugin) *pluginConfig {
	c := &pluginConfig{
		p:         p,
		values:    make(map[string]any),
		listeners: make(map[uint64]moejs.Value),
	}
	if raw, ok := p.storage.Get(configStorageReservedKey); ok {
		_ = json.Unmarshal(raw, &c.values)
	}
	if c.values == nil {
		c.values = make(map[string]any)
	}
	return c
}

func (c *pluginConfig) GetValues() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]any, len(c.values))
	if c.schema != nil {
		for _, f := range c.schema.Fields {
			if f.Default != nil {
				out[f.Key] = f.Default
			}
		}
	}
	for k, v := range c.values {
		out[k] = v
	}
	return out
}

func (c *pluginConfig) SetValues(next map[string]any) error {
	c.mu.Lock()
	if c.values == nil {
		c.values = make(map[string]any)
	}
	for k, v := range next {
		c.values[k] = v
	}
	snapshot := make(map[string]any, len(c.values))
	for k, v := range c.values {
		snapshot[k] = v
	}
	if c.schema != nil {
		for _, f := range c.schema.Fields {
			if _, ok := snapshot[f.Key]; !ok && f.Default != nil {
				snapshot[f.Key] = f.Default
			}
		}
	}
	raw, err := json.Marshal(c.values)
	fns := make([]moejs.Value, 0, len(c.listeners))
	for _, fn := range c.listeners {
		fns = append(fns, fn)
	}
	c.mu.Unlock()
	if err != nil {
		return err
	}
	c.p.storage.Set(configStorageReservedKey, raw)
	c.p.storage.FlushSync()

	if len(fns) > 0 {
		c.p.Post(func(rt *moejs.Runtime) {
			val, err := toJSValue(rt, snapshot)
			if err != nil {
				return
			}
			for _, fn := range fns {
				if _, err := rt.CallFunction(fn, moejs.Undefined(), val); err != nil {
					c.p.recordError(err)
				}
			}
		})
	}
	return nil
}

func (p *Plugin) buildConfigAPI() map[string]any {
	return map[string]any{
		"define": p.safeNative("config.define", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			arg := moejs.Arg(args, 0)
			if !arg.IsObject() {
				return moejs.Undefined(), throwJSError(r, "config.define 需要 schema 对象")
			}
			raw, err := p.rt.AppendJSON(nil, arg)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			var schema ConfigSchema
			// 兼容两种写法：{ fields: [...] } 或直接 { key: { type, label, default } }
			if err := json.Unmarshal(raw, &schema); err != nil || len(schema.Fields) == 0 {
				var rawMap map[string]ConfigFieldSchema
				if merr := json.Unmarshal(raw, &rawMap); merr == nil && len(rawMap) > 0 {
					keys := make([]string, 0, len(rawMap))
					for k := range rawMap {
						keys = append(keys, k)
					}
					slices.Sort(keys)
					for _, k := range keys {
						f := rawMap[k]
						if f.Key == "" {
							f.Key = k
						}
						if f.Label == "" {
							f.Label = k
						}
						if f.Type == "" {
							f.Type = "string"
						}
						schema.Fields = append(schema.Fields, f)
					}
				}
			}
			if schema.Title == "" {
				schema.Title = p.Manifest.DisplayName(p.manager.currentLanguage())
			}
			if schema.Icon == "" {
				schema.Icon = "application-brackets-outline"
			}
			p.configMgr.mu.Lock()
			p.configMgr.schema = &schema
			p.configMgr.mu.Unlock()

			// 自动在设置页注册一个 settings.section
			dispose := p.scope.Effect(p.manager.ui.RegisterSlot(SlotRegistration{
				SlotName:     SlotSettingsSection,
				PluginID:     p.ID,
				PluginName:   p.Manifest.DisplayName(p.manager.currentLanguage()),
				ID:           "__config__",
				Title:        schema.Title,
				Icon:         schema.Icon,
				ConfigSchema: &schema,
			}))
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
		"get": p.safeNative("config.get", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			return toJSValue(p.rt, p.configMgr.GetValues())
		}),
		"onChange": p.safeNative("config.onChange", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			fn := moejs.Arg(args, 0)
			if !fn.IsObject() {
				return moejs.Undefined(), throwJSError(r, "config.onChange 需要函数参数")
			}
			p.configMgr.mu.Lock()
			p.configMgr.nextID++
			id := p.configMgr.nextID
			p.configMgr.listeners[id] = fn
			p.configMgr.mu.Unlock()
			dispose := p.scope.Effect(func() {
				p.configMgr.mu.Lock()
				delete(p.configMgr.listeners, id)
				p.configMgr.mu.Unlock()
			})
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
	}
}

// ─────────────────────────── ctx.html (goquery) ───────────────────────────

func (p *Plugin) buildHTMLAPI() map[string]any {
	var wrapSelection func(sel *goquery.Selection) map[string]any
	wrapSelection = func(sel *goquery.Selection) map[string]any {
		return map[string]any{
			"select": p.safeNative("html.select", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				return p.rt.FromGo(wrapSelection(sel.Find(argString(args, 0))))
			}),
			"first": p.safeNative("html.first", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				return p.rt.FromGo(wrapSelection(sel.First()))
			}),
			"eq": p.safeNative("html.eq", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				return p.rt.FromGo(wrapSelection(sel.Eq(int(argFloat(args, 0)))))
			}),
			"length": p.safeNative("html.length", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				return p.rt.FromGo(sel.Length())
			}),
			"text": p.safeNative("html.text", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				return moejs.String(sel.Text()), nil
			}),
			"attr": p.safeNative("html.attr", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				v, ok := sel.Attr(argString(args, 0))
				if !ok {
					return moejs.Undefined(), nil
				}
				return moejs.String(v), nil
			}),
			"html": p.safeNative("html.html", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				h, _ := sel.Html()
				return moejs.String(h), nil
			}),
			"map": p.safeNative("html.map", func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				fn := moejs.Arg(args, 0)
				out := make([]moejs.Value, 0, sel.Length())
				var callErr error
				sel.EachWithBreak(func(idx int, s *goquery.Selection) bool {
					el, err := p.rt.FromGo(wrapSelection(s))
					if err != nil {
						callErr = err
						return false
					}
					idxVal, _ := p.rt.FromGo(idx)
					res, err := p.rt.CallFunction(fn, moejs.Undefined(), el, idxVal)
					if err != nil {
						callErr = err
						return false
					}
					out = append(out, res)
					return true
				})
				if callErr != nil {
					return moejs.Undefined(), callErr
				}
				return engine.ObjectValue(p.rt.Realm().NewArray(out...)), nil
			}),
		}
	}

	return map[string]any{
		"parse": p.safeNative("html.parse", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(argString(args, 0)))
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			return p.rt.FromGo(wrapSelection(doc.Selection))
		}),
	}
}

// ─────────────────────────── ctx.shell ───────────────────────────

func (p *Plugin) buildShellAPI() map[string]any {
	return map[string]any{
		"openURL": p.asyncNative("shell.openURL", func(_ context.Context, args []json.RawMessage) (any, error) {
			var u string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &u)
			}
			if strings.TrimSpace(u) == "" {
				return nil, fmt.Errorf("URL 不能为空")
			}
			return nil, p.manager.deps.Runtime.OpenURL(u)
		}),
		"openPath": p.asyncNative("shell.openPath", func(_ context.Context, args []json.RawMessage) (any, error) {
			var target string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &target)
			}
			if strings.TrimSpace(target) == "" {
				return nil, fmt.Errorf("路径不能为空")
			}
			return nil, apputils.OpenFileOrFolder(target)
		}),
	}
}

// ─────────────────────────── ctx.effect ───────────────────────────

func (p *Plugin) buildEffectFunc() moejs.NativeFunc {
	return p.safeNative("ctx.effect", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		setup := moejs.Arg(args, 0)
		if !setup.IsObject() {
			return moejs.Undefined(), throwJSError(r, "ctx.effect 需要函数参数")
		}
		res, err := p.rt.CallFunction(setup, moejs.Undefined())
		if err != nil {
			return moejs.Undefined(), err
		}
		var dispose Disposer = func() {}
		if res.IsObject() {
			cleanup := res
			dispose = p.scope.Effect(func() {
				p.Post(func(rt *moejs.Runtime) {
					_, _ = rt.CallFunction(cleanup, moejs.Undefined())
				})
			})
		}
		return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			dispose()
			return moejs.Undefined(), nil
		}), nil
	})
}

// ─────────────────────────── ctx.fs（完全信任文件访问） ───────────────────────────

func (p *Plugin) buildFSAPI() map[string]any {
	return map[string]any{
		"readFile": p.asyncNative("fs.readFile", func(_ context.Context, args []json.RawMessage) (any, error) {
			var path string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			return string(data), nil
		}),
		"writeFile": p.asyncNative("fs.writeFile", func(_ context.Context, args []json.RawMessage) (any, error) {
			var path, content string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &path)
			}
			if len(args) > 1 {
				_ = json.Unmarshal(args[1], &content)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			return nil, os.WriteFile(path, []byte(content), 0o644)
		}),
		"exists": p.asyncNative("fs.exists", func(_ context.Context, args []json.RawMessage) (any, error) {
			var path string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &path)
			}
			_, err := os.Stat(path)
			return err == nil, nil
		}),
		"readDir": p.asyncNative("fs.readDir", func(_ context.Context, args []json.RawMessage) (any, error) {
			var path string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &path)
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, err
			}
			type dirItem struct {
				Name  string `json:"name"`
				IsDir bool   `json:"is_dir"`
			}
			out := make([]dirItem, 0, len(entries))
			for _, e := range entries {
				out = append(out, dirItem{Name: e.Name(), IsDir: e.IsDir()})
			}
			return out, nil
		}),
		"mkdir": p.asyncNative("fs.mkdir", func(_ context.Context, args []json.RawMessage) (any, error) {
			var path string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &path)
			}
			return nil, os.MkdirAll(path, 0o755)
		}),
		"remove": p.asyncNative("fs.remove", func(_ context.Context, args []json.RawMessage) (any, error) {
			var path string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &path)
			}
			return nil, os.RemoveAll(path)
		}),
	}
}
