package game

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/host/hosttest"
)

// 切换状态写进数据库（BatchUpdateStatus）。
func TestStatusPersists(t *testing.T) {
	tp := newTestPage(t, 1280)
	if err := tp.tt.Click("想玩"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	g, err := tp.svc.Game.GetGameByID("0")
	if err != nil {
		t.Fatal(err)
	}
	if g.Status != enums.StatusWantToPlay {
		t.Fatalf("saved status = %q", g.Status)
	}
}

func TestTagsAddAndDelete(t *testing.T) {
	tp := newTestPage(t, 1280)
	if err := tp.tt.Click("添加标签"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.tt.Type("新标签")
	tp.tt.Key(0, ui.KeyEnter)
	tp.frames(2)
	tags, _ := tp.svc.Tag.GetTagsByGame("0")
	if !slices.ContainsFunc(tags, func(tag models.GameTag) bool { return tag.Name == "新标签" && tag.Source == "user" }) {
		t.Fatalf("tag not saved: %+v", tags)
	}
	if tp.tagEditor.Adding || !tp.tt.HasText("新标签") {
		t.Fatalf("editor %+v, texts %q", tp.tagEditor, tp.tt.Texts())
	}

	// 悬停胶囊时出现删除按钮。
	r, ok := tp.tt.Find("纯爱")
	if !ok {
		t.Fatal("纯爱 not shown")
	}
	tp.tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tp.tt.Advance(time.Second) // 等悬停后删除按钮淡入
	if err := tp.tt.Click("删除标签"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tags, _ = tp.svc.Tag.GetTagsByGame("0")
	if slices.ContainsFunc(tags, func(tag models.GameTag) bool { return tag.Name == "纯爱" }) {
		t.Fatalf("tag not deleted: %+v", tags)
	}
}

func TestCategoryModal(t *testing.T) {
	tp := newTestPage(t, 1280)
	if err := tp.svc.Category.AddCategory("Galgame", ""); err != nil {
		t.Fatal(err)
	}
	if err := tp.tt.Click("添加到收藏"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "已选择 0 个收藏夹", "Galgame", "0 个游戏")
	if err := tp.tt.Click("Galgame"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	tp.expectTexts(t, "已选择 1 个收藏夹")
	if err := tp.tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	cats, err := tp.svc.Category.GetCategoriesByGame("0")
	if err != nil || len(cats) != 1 || cats[0].Name != "Galgame" {
		t.Fatalf("categories %+v, %v", cats, err)
	}

	// 新建收藏夹：搜索词带进名称，创建后自动选中。
	if err := tp.tt.Click("添加到收藏"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.tt.Type("通关")
	tp.frames(1)
	if err := tp.tt.Click("新建收藏夹"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if tp.category.CreateName() != "通关" {
		t.Fatalf("create form name = %q", tp.category.CreateName())
	}
	if err := tp.tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "通关", "已选择 2 个收藏夹")
}

func TestGuideDocuments(t *testing.T) {
	tp := newTestPage(t, 1280)
	_ = tp.tt.Click("打开说明文档") // 没有游戏目录时按钮禁用，点击无效
	tp.frames(1)
	if tp.guide.loading || tp.guide.open {
		t.Fatal("guide button should be disabled without a game directory")
	}
	dir := t.TempDir()
	g := *tp.game
	g.GameDirectory = dir
	if err := tp.svc.Game.UpdateGame(g); err != nil {
		t.Fatal(err)
	}
	tp.game = &g
	tp.frames(1)
	if err := tp.tt.Click("打开说明文档"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if tp.guide.open || tp.guide.loading {
		t.Fatalf("empty directory: %+v", tp.guide)
	}
	for _, name := range []string{"readme.txt", "manual.pdf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := tp.tt.Click("打开说明文档"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if !tp.guide.open {
		docs, err := tp.svc.Game.FindGameGuideDocuments("0")
		t.Fatalf("modal not open; documents %+v %v", docs, err)
	}
	tp.expectTexts(t, "选择游戏说明文档", "找到 2 个可打开的文档")
}

func TestDeleteSession(t *testing.T) {
	tp := newTestPage(t, 1280)
	tp.tt.SetSize(1280, 1600)
	tp.frames(2)
	if err := tp.tt.Click("查看游玩记录"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if err := tp.tt.Click("删除游玩记录"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	tp.expectTexts(t, "确定要删除这条游玩记录吗？此操作不可撤销。")
	if err := tp.tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	sessions, _ := tp.svc.Session.GetPlaySessions("0")
	if len(sessions) != 2 || len(tp.stats.sessions) != 2 {
		t.Fatalf("sessions after delete: db %d, page %d", len(sessions), len(tp.stats.sessions))
	}
}

// 编辑停下 500ms 后自动保存；离开前补存。
func TestAutoSave(t *testing.T) {
	tp := newTestPage(t, 1280)
	g := *tp.game
	g.Name = "改过的名字"
	tp.updateGame(g)
	tp.frames(1)
	if saved, _ := tp.svc.Game.GetGameByID("0"); saved.Name == "改过的名字" {
		t.Fatal("saved before the debounce")
	}
	tp.Flush()
	if saved, _ := tp.svc.Game.GetGameByID("0"); saved.Name != "改过的名字" {
		t.Fatalf("not saved after flush: %q", saved.Name)
	}
}

func TestAddSession(t *testing.T) {
	tp := newTestPage(t, 1280)
	tp.tt.SetSize(1280, 1600)
	tp.frames(2)
	if err := tp.tt.Click("手动添加"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "添加游玩记录", "日期", "开始时间", "游玩时长", "预计结束", "1小时")
	if err := tp.tt.Click("添加"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	sessions, _ := tp.svc.Session.GetPlaySessions("0")
	if len(sessions) != 4 || tp.addSession.open {
		t.Fatalf("sessions %d, modal open %v", len(sessions), tp.addSession.open)
	}
}

// VNDB 标签显示界面语言的译名，点击时按原名筛选；关闭 enable_tag_translation 后显示原名。
func TestTagTranslation(t *testing.T) {
	tp := newTestPage(t, 1280)
	hosttest.AddTag(t, tp.svc, "0", "t3", "Romance", "vndb", false)
	tp.Open("0", time.Now())
	tp.frames(3)
	tp.expectTexts(t, "恋爱")
	if tp.tt.HasText("Romance") {
		t.Errorf("raw name shown: %q", tp.tt.Texts())
	}
	if err := tp.tt.Click("恋爱"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if !slices.Equal(tp.tagged, []string{"Romance"}) {
		t.Fatalf("filtered by %q", tp.tagged)
	}
	tp.svc.Config.EnableTagTranslation = false
	tp.frames(2)
	tp.expectTexts(t, "Romance")
}
