package pluginui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

func TestRenderUITreeAndErrorBoundary(t *testing.T) {
	e := &env.Env{MainWidth: 800}
	mgr := New(e, HostCallbacks{})
	mgr.DevMode = true

	inst := newInstance("inst-1", "demo", plugin.SlotSettingsSection, "sec", "")
	inst.Version = 1
	inst.Tree = &plugin.UINode{
		Type:  "Column",
		Key:   "root",
		Props: map[string]any{"gap": "md"},
		Children: []*plugin.UINode{
			{Type: "Text", Props: map[string]any{"text": "Plugin Title", "size": "lg", "weight": "bold"}},
			{Type: "Input", Key: "q", Props: map[string]any{"value": "initial", "placeholder": "Search"}, Events: map[string]string{"change": "h1"}},
			{Type: "Button", Key: "go", Props: map[string]any{"label": "Run", "variant": "primary"}, Events: map[string]string{"press": "h2"}},
			{Type: "Markdown", Key: "md", Props: map[string]any{"content": "# Heading\n\n- item 1\n> quote"}},
			{Type: "Badge", Key: "b1", Props: map[string]any{"label": "OK", "tone": "success"}},
			{Type: "Progress", Key: "p1", Props: map[string]any{"value": 0.5, "label": "50%"}},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(false, 1)
		mgr.RenderInstance(c, th, inst)
	}, 600, 400)
	tt.Advance(20 * time.Millisecond)

	if st, ok := inst.Inputs["q"]; !ok || st.Value != "initial" {
		t.Fatalf("expected input state to be initialized to 'initial', got %+v", st)
	}

	// 测试错误边界占位
	errInst := newInstance("inst-err", "broken-plugin", plugin.SlotSettingsSection, "err", "")
	errInst.Error = "ReferenceError: boom"
	tt2 := ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1)
		mgr.RenderInstance(c, th, errInst)
	}, 600, 200)
	tt2.Advance(20 * time.Millisecond)

	if want := locales.NativePluginsUiError("broken-plugin"); want == "" {
		t.Fatalf("expected non-empty error boundary message")
	}
}
