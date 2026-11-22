package plugin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MoeForge/moejs"
)

func (p *Plugin) resolvePluginAssetPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(p.Dir, filepath.FromSlash(raw))
}

func (p *Plugin) buildThemeAPI() map[string]any {
	seq := 0
	return map[string]any{
		"extend": p.safeNative("theme.extend", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			arg := moejs.Arg(args, 0)
			if !arg.IsObject() {
				return moejs.Undefined(), throwJSError(r, "theme.extend 需要令牌对象")
			}
			raw, err := p.rt.AppendJSON(nil, arg)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			var input struct {
				ID    string            `json:"id"`
				Light map[string]string `json:"light"`
				Dark  map[string]string `json:"dark"`
				Glass *GlassOverride    `json:"glass"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			if input.Glass != nil && input.Glass.BackgroundImage != "" {
				input.Glass.BackgroundImage = p.resolvePluginAssetPath(input.Glass.BackgroundImage)
			}
			seq++
			id := strings.TrimSpace(input.ID)
			if id == "" {
				id = fmt.Sprintf("theme-%d", seq)
			}
			dispose := p.scope.Effect(p.manager.ui.RegisterTheme(ThemeOverride{
				PluginID: p.ID,
				ID:       id,
				Light:    input.Light,
				Dark:     input.Dark,
				Glass:    input.Glass,
			}))
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
		"registerFont": p.safeNative("theme.registerFont", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			arg := moejs.Arg(args, 0)
			if !arg.IsObject() {
				return moejs.Undefined(), throwJSError(r, "theme.registerFont 需要字体配置对象")
			}
			raw, err := p.rt.AppendJSON(nil, arg)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			var input struct {
				Family string `json:"family"`
				File   string `json:"file"`
				Target string `json:"target"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			if strings.TrimSpace(input.Family) == "" || strings.TrimSpace(input.File) == "" {
				return moejs.Undefined(), throwJSError(r, "theme.registerFont 需要 family 和 file")
			}
			dispose := p.scope.Effect(p.manager.ui.RegisterFont(FontRegistration{
				PluginID: p.ID,
				Family:   strings.TrimSpace(input.Family),
				FilePath: p.resolvePluginAssetPath(input.File),
				Target:   strings.ToLower(strings.TrimSpace(input.Target)),
			}))
			return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
		"registerDecoration": p.safeNative("theme.registerDecoration", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			arg := moejs.Arg(args, 0)
			if !arg.IsObject() {
				return moejs.Undefined(), throwJSError(r, "theme.registerDecoration 需要装饰配置对象")
			}
			return p.registerDecorationFromValue(r, arg)
		}),
	}
}

func (p *Plugin) registerDecorationFromValue(r *moejs.Realm, arg moejs.Value) (moejs.Value, error) {
	raw, err := p.rt.AppendJSON(nil, arg)
	if err != nil {
		return moejs.Undefined(), throwJSError(r, err.Error())
	}
	var input struct {
		ID            string   `json:"id"`
		Layer         string   `json:"layer"`
		Anchor        string   `json:"anchor"`
		Offset        struct {
			X float32 `json:"x"`
			Y float32 `json:"y"`
		} `json:"offset"`
		Size struct {
			Width  float32 `json:"width"`
			Height float32 `json:"height"`
		} `json:"size"`
		Opacity       *float32 `json:"opacity"`
		PointerEvents string   `json:"pointerEvents"`
		Routes        []string `json:"routes"`
		Image         string   `json:"image"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return moejs.Undefined(), throwJSError(r, err.Error())
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return moejs.Undefined(), throwJSError(r, "decoration 缺少 id")
	}
	layer := strings.ToLower(strings.TrimSpace(input.Layer))
	if layer != "background" && layer != "foreground" {
		layer = "foreground"
	}
	anchor := strings.ToLower(strings.TrimSpace(input.Anchor))
	if anchor == "" {
		anchor = "bottom-right"
	}
	opacity := float32(1.0)
	if input.Opacity != nil {
		opacity = *input.Opacity
		if opacity < 0 {
			opacity = 0
		}
		if opacity > 1 {
			opacity = 1
		}
	}
	pe := strings.ToLower(strings.TrimSpace(input.PointerEvents))
	if pe == "" {
		pe = "none"
	}

	renderFn, _ := p.rt.Get(arg, "render")
	hasRender := renderFn.IsObject()
	slotKey := slotCallbackKey(SlotAppDecoration, id)
	if hasRender {
		p.mu.Lock()
		p.renderFns[slotKey] = renderFn
		p.mu.Unlock()
	}

	unreg := p.manager.ui.RegisterDecoration(DecorationRegistration{
		PluginID:      p.ID,
		ID:            id,
		Layer:         layer,
		Anchor:        anchor,
		OffsetX:       input.Offset.X,
		OffsetY:       input.Offset.Y,
		Width:         input.Size.Width,
		Height:        input.Size.Height,
		Opacity:       opacity,
		PointerEvents: pe,
		Routes:        input.Routes,
		ImagePath:     p.resolvePluginAssetPath(input.Image),
		HasRender:     hasRender,
	})
	dispose := p.scope.Effect(func() {
		unreg()
		if hasRender {
			p.mu.Lock()
			delete(p.renderFns, slotKey)
			p.mu.Unlock()
		}
	})
	return p.rt.Function("dispose", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
		dispose()
		return moejs.Undefined(), nil
	}), nil
}
