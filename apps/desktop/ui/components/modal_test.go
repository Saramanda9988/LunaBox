package components

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

func TestConfirmModal(t *testing.T) {
	open, confirmed := true, 0
	th := theme.New(true, 1)
	tt := ui.NewTester(func(c *ui.Context) {
		ConfirmModal(c, th, "confirm", ConfirmProps{
			Open: &open, Title: "删除游玩记录", Message: "确定要删除这条游玩记录吗？", Danger: true,
			OnConfirm: func() { confirmed++ },
		})
	}, 800, 600)
	tt.Frame()
	for _, s := range []string{"删除游玩记录", "确定要删除这条游玩记录吗？", "取消", "确定"} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if confirmed != 1 || open || tt.HasText("取消") {
		t.Fatalf("confirmed %d, open %v, texts %q", confirmed, open, tt.Texts())
	}
	open = true
	tt.Frame()
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if confirmed != 1 || open {
		t.Fatalf("cancel: confirmed %d, open %v", confirmed, open)
	}
}

func TestModalEscape(t *testing.T) {
	open := true
	th := theme.New(true, 1)
	tt := ui.NewTester(func(c *ui.Context) {
		Modal(c, th, "m", ModalProps{Open: open, OnClose: func() { open = false }}, func(panel ui.Element) {
			ModalPanel(th, panel).Children(func() { Text(c, "内容", theme.TextSM, theme.WeightNormal, th.Text) })
		})
	}, 800, 600)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if open {
		t.Fatal("Esc did not close the modal")
	}
	open = true
	tt.Frame()
	tt.ClickAt(10, 10)
	if open {
		t.Fatal("clicking the backdrop did not close the modal")
	}
}
