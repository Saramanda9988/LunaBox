package pluginui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/theme"
)

// Markdown 原语走 MyGo 的 ui/markdown：行内的加粗、代码与链接按样式显示，不留下 Markdown 标记；
// streaming 时未闭合的语法也按最终样子显示。
func TestRenderMarkdownInlineAndStreaming(t *testing.T) {
	mgr := New(&env.Env{MainWidth: 600}, HostCallbacks{})
	for _, streaming := range []bool{false, true} {
		inst := newInstance("md", "demo", plugin.SlotSettingsSection, "sec", "")
		inst.Version = 1
		content := "这是 **Stella of the End**，开发商是 `Key`。\n\n1. 第一项\n2. 第二项\n\n| 字段 | 值 |\n| --- | --- |\n| 发售 | 2024 |"
		if streaming {
			content += "\n\n还在写 **加粗"
		}
		inst.Tree = &plugin.UINode{Type: "Markdown", Key: "m", Props: map[string]any{"content": content, "streaming": streaming}}
		tt := ui.NewTester(func(c *ui.Context) {
			mgr.RenderInstance(c, theme.New(false, 1), inst)
		}, 600, 600)
		for i := 0; i < 40; i++ {
			tt.Advance(50 * time.Millisecond)
		}
		all := strings.Join(tt.Texts(), "\n")
		if strings.Contains(all, "**") || strings.Contains(all, "`") || strings.Contains(all, "| --- |") {
			t.Errorf("streaming=%v: Markdown markers should not show, got %q", streaming, all)
		}
		for _, want := range []string{"Stella of the End", "第二项", "发售"} {
			if !strings.Contains(all, want) {
				t.Errorf("streaming=%v: missing %q in %q", streaming, want, all)
			}
		}
		if len(inst.Markdowns) != 1 {
			t.Errorf("streaming=%v: markdown options should be cached per key, got %d", streaming, len(inst.Markdowns))
		}
	}
}
