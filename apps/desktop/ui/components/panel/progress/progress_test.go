package progress

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

type harness struct {
	svc   *host.Backend
	env   *env.Env
	game  models.Game
	state State
	tt    *ui.Tester
}

// newHarness 在真实服务（临时 DuckDB）上打开面板，外面套一层详情页的 p-8 与背景。
func newHarness(t *testing.T, id string, records ...models.GameProgress) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t), game: models.Game{ID: id, Name: "游玩进度测试"}}
	hosttest.AddGame(t, h.svc, h.game)
	for _, gp := range records {
		gp.GameID = id
		if _, err := h.svc.GameProgress.UpsertGameProgress(gp); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // updated_at 排序
	}
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 900)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Padding(32).Background(t.AppBg).Children(func() {
		View(c, t, &h.state, panel.Game{Env: h.env, Game: &h.game, OnChange: func(models.Game) {}, OnSaved: func() {}, OnDeleted: func() {}})
	})
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) history(t *testing.T) []models.GameProgress {
	t.Helper()
	list, err := h.svc.GameProgress.ListGameProgresses(h.game.ID)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// clickSubmit 点击弹窗底部的“新增记录”（与面板按钮、弹窗标题同名）：它在剧透说明下方、面板右缘。
func (h *harness) clickSubmit(t *testing.T) {
	t.Helper()
	hint, ok := h.tt.Find(spoilerHint(h.state.modal.boundary))
	if !ok {
		t.Fatalf("hint not found: %q", h.tt.Texts())
	}
	// 说明（16）→ space-y-4（16）→ border-t + pt-6（24）→ 按钮（36）的中线。
	h.tt.ClickAt(hint.X+hint.W-40, hint.Y+hint.H+16+1+24+18)
	h.frames(2)
}

func TestEmpty(t *testing.T) {
	h := newHarness(t, "progress-empty")
	h.expect(t, "游玩进度", "记录当前进度，AI 总结将严格遵守您设置的剧透边界", "新增记录",
		"还没有游玩记录。新增第一条后，这里会开始形成完整的游玩时间线。")
}

func TestAddRecords(t *testing.T) {
	h := newHarness(t, "progress-add")

	// 第一条：在章节输入框里回车提交表单。
	h.click(t, "新增记录")
	if !h.state.modal.open || h.state.modal.loading {
		t.Fatalf("modal = %+v", h.state.modal)
	}
	h.expect(t, "当前章节", "当前角色线 / 路线", "进度备注", "AI 可讨论范围", "禁止剧情（最安全）",
		"AI 不得提及任何具体剧情、角色关系、结局或路线发展。")
	h.click(t, "如：第二章 / Chapter 2")
	h.tt.Type("第一章")
	h.click(t, "如：水瀬ルート / 真エンド前")
	h.tt.Type("共通线")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	list := h.history(t)
	if len(list) != 1 || list[0].Chapter != "第一章" || list[0].Route != "共通线" || list[0].SpoilerBoundary != "none" {
		t.Fatalf("history = %+v", list)
	}
	if h.state.modal.open {
		t.Error("modal still open after save")
	}
	h.expect(t, "已新增游玩记录", "最近记录时间：", "第一章", "共通线", "没有填写备注", "禁止剧情（最安全）")

	// 第二条：沿用上一条的章节、路线与剧透范围（备注清空），改动后点击提交。
	h.click(t, "新增记录")
	m := h.state.modal
	if m.chapter != "第一章" || m.route != "共通线" || m.note != "" || m.boundary != "none" {
		t.Fatalf("prefill = %+v", m)
	}
	h.click(t, "自由描述当前进度，如：刚看到BE结局，尚未解锁真路线")
	h.tt.Type("刚看到BE结局")
	// 打开下拉：列表里的徽标也叫“禁止剧情（最安全）”，按标签位置点击下拉按钮（标签下 space-y-1.5，按钮高 42）。
	label, _ := h.tt.Find("AI 可讨论范围")
	h.tt.ClickAt(label.X+100, label.Y+label.H+6+21)
	h.frames(2)
	h.click(t, "无限制")
	if h.state.modal.boundary != "full" {
		t.Fatalf("boundary = %q", h.state.modal.boundary)
	}
	h.expect(t, "AI 不受剧透限制，可自由讨论全结局与所有剧情。")
	h.clickSubmit(t)
	list = h.history(t)
	if len(list) != 2 || list[0].ProgressNote != "刚看到BE结局" || list[0].SpoilerBoundary != "full" || list[0].Chapter != "第一章" {
		t.Fatalf("history = %+v", list)
	}
	h.expect(t, "刚看到BE结局", "无限制")

	// 关闭按钮不保存。
	h.click(t, "新增记录")
	h.click(t, "关闭")
	if h.state.modal.open || len(h.history(t)) != 2 {
		t.Errorf("close: modal %+v, %d records", h.state.modal, len(h.history(t)))
	}
}

func TestLoadsHistory(t *testing.T) {
	h := newHarness(t, "progress-list",
		models.GameProgress{Chapter: "序章", SpoilerBoundary: "chapter_end"},
		models.GameProgress{Chapter: "第三章", Route: "真结局", ProgressNote: "第一行\n第二行", SpoilerBoundary: "route_end"},
	)
	h.expect(t, "序章", "第三章", "真结局", "第一行\n第二行", "当前章节结束前", "当前路线完整内容", "-")
	if len(h.state.history) != 2 || h.state.history[0].Chapter != "第三章" {
		t.Errorf("order: %+v", h.state.history)
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把游玩进度面板（列表、空状态、骨架屏、弹窗）存成图片，用于目测。
func TestScreenshotDump(t *testing.T) {
	out := os.Getenv("DESKTOP_MYGO_PNG")
	if out == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(h *harness, name string) {
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	h := newHarness(t, "progress-shot",
		models.GameProgress{Chapter: "序章", SpoilerBoundary: "none"},
		models.GameProgress{Chapter: "第三章 / Chapter 3", Route: "水瀬ルート", ProgressNote: "刚看到BE结局，尚未解锁真路线。\n下次从第三章开头的选项继续。", SpoilerBoundary: "route_end"},
	)
	save(h, "progress-list.png")
	h.click(t, "新增记录")
	h.frames(2)
	save(h, "progress-modal.png")

	empty := newHarness(t, "progress-shot-empty")
	save(empty, "progress-empty.png")
	empty.state.loading, empty.state.history, empty.state.loadStart = true, nil, time.Now().Add(-time.Second)
	empty.frames(2)
	save(empty, "progress-skeleton.png")
	empty.state.loading = false
	empty.state.modal = modalState{open: true, loading: true}
	empty.frames(2)
	save(empty, "progress-modal-loading.png")
}
