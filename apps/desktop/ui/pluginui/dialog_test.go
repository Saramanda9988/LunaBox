package pluginui

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/api"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

type dialogStub struct {
	api.PluginService
	ids     []string
	results []any
}

func (s *dialogStub) List() []plugin.PluginInfo {
	return []plugin.PluginInfo{{ID: "steam-accounts", Name: "Steam 多账号"}}
}

func (s *dialogStub) ResolveDialog(_, dialogID string, result any) {
	s.ids = append(s.ids, dialogID)
	s.results = append(s.results, result)
}

// 插件对话框按请求顺序逐个显示：select 必须先选中才能确认，作答后显示下一个；confirm 取消回传 false。
func TestPluginDialogsAnswerInOrder(t *testing.T) {
	stub := &dialogStub{}
	m := New(&env.Env{Svc: &api.Services{Plugin: stub}}, HostCallbacks{})
	m.handleAction(plugin.UIAction{Type: "dialog", PluginID: "steam-accounts", DialogID: "d1", Dialog: &plugin.DialogSpec{
		Kind:  "select",
		Title: "选择 Steam 账号",
		Options: []plugin.DialogOption{
			{Value: "alice", Label: "Alice", Description: "alice_login"},
			{Value: "bob", Label: "Bob"},
		},
	}})
	m.handleAction(plugin.UIAction{Type: "dialog", PluginID: "steam-accounts", DialogID: "d2", Dialog: &plugin.DialogSpec{
		Kind: "confirm", Title: "删除这个游戏？", Danger: true,
	}})
	m.handleAction(plugin.UIAction{Type: "dialog", PluginID: "steam-accounts", DialogID: "d3", Dialog: &plugin.DialogSpec{
		Kind: "confirm", Title: "插件卸载时关闭",
	}})

	th := theme.New(false, 1)
	tt := ui.NewTester(func(c *ui.Context) { m.DialogView(c, th, 0) }, 900, 700)
	frames := func() { tt.Advance(20 * time.Millisecond); tt.Advance(20 * time.Millisecond) }
	frames()

	texts := tt.Texts()
	if !slices.Contains(texts, "选择 Steam 账号") || !slices.Contains(texts, "Steam 多账号") || slices.Contains(texts, "删除这个游戏？") {
		t.Fatalf("first dialog should show alone with the plugin name: %q", texts)
	}
	// 没有选中项时确认无效
	if err := tt.Click(locales.CommonConfirm()); err != nil {
		t.Fatal(err)
	}
	frames()
	if len(stub.ids) != 0 {
		t.Fatalf("confirm without a selection should do nothing, got %v", stub.results)
	}
	if err := tt.Click("Bob"); err != nil {
		t.Fatal(err)
	}
	frames()
	if err := tt.Click(locales.CommonConfirm()); err != nil {
		t.Fatal(err)
	}
	frames()
	if len(stub.ids) != 1 || stub.ids[0] != "d1" || stub.results[0] != "bob" {
		t.Fatalf("select answer: ids=%v results=%v", stub.ids, stub.results)
	}

	if texts := tt.Texts(); !slices.Contains(texts, "删除这个游戏？") {
		t.Fatalf("second dialog should show next: %q", texts)
	}
	if err := tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	frames()
	if len(stub.ids) != 2 || stub.ids[1] != "d2" || stub.results[1] != false {
		t.Fatalf("confirm cancel: ids=%v results=%v", stub.ids, stub.results)
	}

	// 插件卸载：宿主发 closeDialog，直接移除，不再回传
	m.handleAction(plugin.UIAction{Type: "closeDialog", PluginID: "steam-accounts", DialogID: "d3"})
	frames()
	if len(m.dialogs) != 0 || len(stub.ids) != 2 {
		t.Fatalf("closeDialog should drop the dialog silently: dialogs=%d ids=%v", len(m.dialogs), stub.ids)
	}
}
