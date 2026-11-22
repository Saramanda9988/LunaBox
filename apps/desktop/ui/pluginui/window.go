package pluginui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/theme"
)

type pluginWindow struct {
	mgr  *Manager
	spec plugin.PluginWindowSpec
	win  *mygo.Window
}

func (w *pluginWindow) close() {
	if w.win != nil {
		w.win.Close()
		w.win = nil
	}
}

func (m *Manager) handleWindowCommand(cmd plugin.WindowCommand) {
	switch cmd.Action {
	case "main.show", "main.focus", "main.minimize":
		if m.callbacks.OnMainWindowCmd != nil {
			m.callbacks.OnMainWindowCmd(cmd.Action)
			return
		}
		if m.env != nil && m.env.Win != nil {
			switch cmd.Action {
			case "main.show":
				m.env.Win.Show()
			case "main.focus":
				m.env.Win.Show()
				m.env.Win.Focus()
			case "main.minimize":
				m.env.Win.Minimize()
			}
		}
		return
	}

	key := cmd.PluginID + "/" + cmd.WindowID
	switch cmd.Action {
	case "open":
		if cmd.Spec == nil {
			return
		}
		if existing, ok := m.windows[key]; ok {
			existing.close()
			delete(m.windows, key)
		}
		m.openPluginWindow(key, *cmd.Spec)
	case "close":
		if w, ok := m.windows[key]; ok {
			delete(m.windows, key)
			w.close()
		}
		if cmd.Spec != nil {
			delete(m.instances, cmd.Spec.InstanceID)
		}
	case "show":
		if w, ok := m.windows[key]; ok && w.win != nil {
			w.win.Show()
		}
	case "hide":
		if w, ok := m.windows[key]; ok && w.win != nil {
			w.win.Hide()
		}
	case "setSize":
		if w, ok := m.windows[key]; ok && w.win != nil && cmd.Width > 0 && cmd.Height > 0 {
			w.win.SetSize(cmd.Width, cmd.Height)
		}
	case "setPosition":
		if w, ok := m.windows[key]; ok && w.win != nil {
			w.win.SetPosition(cmd.X, cmd.Y)
		}
	case "setAlwaysOnTop":
		if w, ok := m.windows[key]; ok && w.win != nil {
			w.win.SetAlwaysOnTop(cmd.AlwaysOnTop)
		}
	}
}

func (m *Manager) openPluginWindow(key string, spec plugin.PluginWindowSpec) {
	inst, ok := m.instances[spec.InstanceID]
	if !ok {
		inst = newInstance(spec.InstanceID, spec.PluginID, "window", spec.WindowID, "")
		m.instances[spec.InstanceID] = inst
	}

	pw := &pluginWindow{mgr: m, spec: spec}
	var win *mygo.Window
	opts := mygo.WindowOptions{
		Title:         spec.Title,
		Width:         spec.Width,
		Height:        spec.Height,
		Frameless:     spec.Frameless,
		Transparent:   spec.Transparent,
		AlwaysOnTop:   spec.AlwaysOnTop,
		SkipTaskbar:   spec.SkipTaskbar,
		Panel:         spec.SkipTaskbar && !spec.Transparent,
		DisableShadow: spec.Transparent,
		DisableResize: !spec.Resizable,
		Content: ui.View(func(c *ui.Context) {
			scale := 1.0
			if win != nil {
				if d := mygo.Screen.DisplayMatching(win.Bounds()); d.ScaleFactor > 0 {
					scale = d.ScaleFactor
				}
			}
			t := theme.New(c.Theme().Dark, scale)
			if m.env != nil && m.env.Svc != nil && m.env.Svc.Plugin != nil {
				t.ApplyPluginOverrides(m.env.Svc.Plugin.ThemeOverrides())
			}
			applyWindowTheme(c, t, spec.Transparent)
			root := c.Root().TextColor(t.Text)
			if spec.Transparent {
				// MyGo 透明窗口必须显式把根元素设为 ui.Transparent，否则会用 Theme.Background 填满矩形底色。
				root.Background(ui.Transparent)
			} else {
				root.Background(t.AppBg)
			}
			if curInst := m.instances[spec.InstanceID]; curInst != nil {
				m.RenderInstance(c, t, curInst)
			}
		}),
	}
	if spec.X != nil {
		opts.X = *spec.X
	}
	if spec.Y != nil {
		opts.Y = *spec.Y
	}
	win = mygo.NewWindow(opts)
	pw.win = win
	m.windows[key] = pw

	svc := m.env.Svc.Plugin
	win.OnMove(func() {
		b := win.Bounds()
		if svc != nil {
			svc.NotifyWindowEvent(spec.PluginID, spec.WindowID, "move", map[string]any{"x": b.X, "y": b.Y})
		}
	})
	win.OnFocus(func() {
		if svc != nil {
			svc.NotifyWindowEvent(spec.PluginID, spec.WindowID, "focus", nil)
		}
	})
	win.OnClose(func(_ *mygo.CloseEvent) {
		delete(m.windows, key)
		delete(m.instances, spec.InstanceID)
		if svc != nil {
			svc.NotifyWindowEvent(spec.PluginID, spec.WindowID, "close", nil)
		}
	})
}

func applyWindowTheme(c *ui.Context, t *theme.Theme, transparent bool) {
	mt := *c.Theme()
	mt.Accent = t.TextStrong
	mt.InactiveAccent = mt.Accent
	mt.Text = t.Text
	mt.TextMuted = t.Placeholder
	if transparent {
		mt.Background = ui.Transparent
	} else {
		mt.Background = t.AppBg
	}
	mt.ScrollbarStyle = t.ScrollbarStyle()
	mt.Scrollbar = mt.ScrollbarStyle.Thumb
	mt.ScrollbarWidth = mt.ScrollbarStyle.ThumbWidth
	mt.Inverse, mt.InverseText = t.TooltipBg, t.TooltipText
	mt.FontSize = theme.TextBase.Size
	if f := theme.ActiveUIFont(); f != "" {
		mt.Font = f
	}
	c.SetTheme(&mt)
}
