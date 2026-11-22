package pluginui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/theme"
)

func syncThemeFonts(fonts []plugin.FontRegistration) {
	theme.SyncPluginFonts(fonts)
}

// RenderDecorations 在主窗口的 background 或 foreground 层绘制插件注册的装饰（如右下角角色立绘水印）。
// pointerEvents == "none" 时使用 PassThrough()，确保点击与滚轮完全穿透不遮挡底部卡片与按钮。
func (m *Manager) RenderDecorations(c *ui.Context, t *theme.Theme, layer string, currentRoute string) {
	if m == nil || m.env == nil || m.env.Svc == nil || m.env.Svc.Plugin == nil {
		return
	}
	decs := m.env.Svc.Plugin.Decorations()
	if len(decs) == 0 {
		return
	}
	for _, dec := range decs {
		if dec.Layer != layer {
			continue
		}
		if len(dec.Routes) > 0 && !matchDecorationRoute(currentRoute, dec.Routes) {
			continue
		}

		box := ui.Box(c.Key("plugin-dec-"+dec.PluginID+"-"+dec.ID)).Absolute()
		applyDecorationAnchor(box, dec)
		if dec.Width > 0 {
			box.Width(dec.Width)
		}
		if dec.Height > 0 {
			box.Height(dec.Height)
		}
		if dec.Opacity >= 0 && dec.Opacity < 1 {
			box.Opacity(dec.Opacity)
		}
		if dec.PointerEvents != "auto" {
			box.PassThrough()
		}

		d := dec
		box.Children(func() {
			if d.HasRender {
				inst := m.EnsureMounted(plugin.SlotAppDecoration, d.PluginID, d.ID, "global", map[string]any{
					"route":  currentRoute,
					"dark":   t.Dark,
					"anchor": d.Anchor,
				})
				m.RenderInstance(c, t, inst)
				return
			}
			if d.ImagePath != "" {
				if bmp := m.loadBitmap(d.ImagePath); bmp != nil {
					img := ui.Image(c, bmp).Fill().Fit(ui.Contain)
					if d.PointerEvents != "auto" {
						img.PassThrough()
					}
				}
			}
		})
	}
}

func applyDecorationAnchor(box ui.Element, dec plugin.DecorationRegistration) {
	switch dec.Anchor {
	case "top-left":
		box.Top(dec.OffsetY).Left(dec.OffsetX)
	case "top-right":
		box.Top(dec.OffsetY).Right(dec.OffsetX)
	case "bottom-left":
		box.Bottom(dec.OffsetY).Left(dec.OffsetX)
	case "center":
		box.Attach(ui.AnchorCenter, ui.AnchorCenter).Translate(dec.OffsetX, dec.OffsetY)
	default: // bottom-right
		box.Bottom(dec.OffsetY).Right(dec.OffsetX)
	}
}

func matchDecorationRoute(current string, routes []string) bool {
	if idx := strings.IndexByte(current, '?'); idx >= 0 {
		current = current[:idx]
	}
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" || r == "*" {
			return true
		}
		if r == current {
			return true
		}
		if strings.HasSuffix(r, "*") && strings.HasPrefix(current, strings.TrimSuffix(r, "*")) {
			return true
		}
	}
	return false
}
