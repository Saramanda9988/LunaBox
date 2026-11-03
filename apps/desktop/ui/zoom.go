package ui

import (
	"log"
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// zoom 对应 hooks/useAppZoom.ts：按 window_zoom_factor 缩放整个窗口，返回缩放比例。原版改根元素的字号，
// 这里用 Context.SetZoom 像浏览器缩放页面一样缩放，文字、图标与图片按缩放后的尺寸清晰绘制。
// Ctrl + / Ctrl - / Ctrl 0（macOS 上是 Cmd）逐档调整，立即保存并提示。原版在输入框里不响应快捷键；
// MyGo 没有查询焦点是否在输入框的接口，输入框也不处理这几个组合键，这里不区分。
// 每帧在构建任何内容之前调用：缩放改变时这一帧按新的尺寸重新构建。
func (a *App) zoom(c *ui.Context) float64 {
	cfg := a.env.Config()
	if cfg == nil {
		c.SetZoom(1)
		return 1
	}
	cur := env.NormalizeZoom(cfg.WindowZoomFactor)
	next := cur
	switch {
	case c.Shortcut(ui.Cmd, ui.Key0):
		next = 1
	case c.Shortcut(ui.Cmd, ui.KeyEqual) || c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEqual):
		next = env.NextZoom(cur, 1)
	case c.Shortcut(ui.Cmd, ui.KeyMinus) || c.Shortcut(ui.Cmd|ui.Shift, ui.KeyMinus):
		next = env.NextZoom(cur, -1)
	}
	if next != cur {
		// patchLiveConfig({ window_zoom_factor })：设置页的草稿也跟着改，免得之后保存草稿时把缩放覆盖回旧值。
		if err := a.env.PatchConfig(func(cfg *appconf.AppConfig) { cfg.WindowZoomFactor = next }); err != nil {
			log.Printf("zoom: failed to save config: %v", err)
		} else {
			cur = next
			a.settings.ApplyZoom(next)
			value := strconv.Itoa(int(math.Round(next*100))) + "%"
			a.env.Toast(c, env.Notice{Type: "success", Msg: locales.SettingsBasicZoomToast(value), ID: "app-zoom-changed"})
		}
	}
	c.SetZoom(float32(cur))
	return cur
}
