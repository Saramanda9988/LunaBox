package ui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
)

// TestZoomShortcuts：Ctrl + / Ctrl - / Ctrl 0 逐档调整 window_zoom_factor 并立即保存，窗口按它缩放，
// 主题的缩放比例是显示器缩放乘以界面缩放。
func TestZoomShortcuts(t *testing.T) {
	a, tt := newTestApp(t, 3)
	cfg := a.env.Svc.Config
	steps := []struct {
		mods ui.Modifiers
		key  ui.Key
		want float64
	}{
		{ui.Cmd, ui.KeyEqual, 1.1},
		{ui.Cmd | ui.Shift, ui.KeyEqual, 1.25}, // Ctrl + 是 Shift 加 =
		{ui.Cmd, ui.KeyMinus, 1.1},
		{ui.Cmd, ui.Key0, 1},
		{ui.Cmd, ui.KeyMinus, 0.9},
		{ui.Cmd, ui.KeyMinus, 0.8},
		{ui.Cmd, ui.KeyMinus, 0.8}, // 最小一档，不变
	}
	for i, s := range steps {
		tt.Key(s.mods, s.key)
		tt.Frame()
		if cfg.WindowZoomFactor != s.want {
			t.Fatalf("step %d: zoom %v, want %v", i, cfg.WindowZoomFactor, s.want)
		}
		if got := a.env.Theme.Scale; math.Abs(got-1.5*s.want) > 1e-9 {
			t.Fatalf("step %d: theme scale %v, want %v", i, got, 1.5*s.want)
		}
	}
	if !hasToast(tt, locales.SettingsBasicZoomToast("80%")) {
		t.Errorf("missing zoom toast in %q", tt.Texts())
	}
}

// TestZoomFromConfig：启动时按配置里的缩放显示，不在档位上的值取最近的一档。
func TestZoomFromConfig(t *testing.T) {
	a, tt := newTestApp(t, 3)
	a.env.Svc.Config.WindowZoomFactor = 1.3
	tt.Frame()
	if got := a.env.Theme.Scale; math.Abs(got-1.5*1.25) > 1e-9 {
		t.Errorf("theme scale %v, want %v", got, 1.5*1.25)
	}
}
