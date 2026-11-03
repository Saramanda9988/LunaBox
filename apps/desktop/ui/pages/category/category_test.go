package category

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/service/gamehelper"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/theme"
)

type testPage struct {
	*Page
	svc *host.Backend
	tt  *ui.Tester
	id  string
}

// newTestPage 打开“视觉小说”收藏夹：库里三个游戏，收藏夹里是前两个。
func newTestPage(t *testing.T) *testPage {
	t.Helper()
	svc := hosttest.NewServices(t)
	b := envtest.NewBackend(3)
	for _, g := range b.Games {
		hosttest.AddGame(t, svc, g)
	}
	if err := svc.Category.AddCategory("视觉小说", ""); err != nil {
		t.Fatal(err)
	}
	cats, err := svc.Category.GetCategories()
	if err != nil {
		t.Fatal(err)
	}
	tp := &testPage{svc: svc}
	for _, c := range cats {
		if !c.IsSystem {
			tp.id = c.ID
		}
	}
	for _, g := range []string{"0", "1"} {
		if err := svc.Category.AddGameToCategory(g, tp.id); err != nil {
			t.Fatal(err)
		}
	}
	e := &env.Env{Backend: b, Svc: svc.API(), Router: ui.NewRouter(env.CategoryPath(tp.id)), Covers: covers.New(), Theme: theme.New(true, 1.5), MainWidth: 1024}
	tp.Page = New(e)
	tp.Open(tp.id, time.Now())
	tp.tt = ui.NewTester(tp.View, 1280, 800)
	tp.tt.SetScale(1.5)
	tp.tt.SetDark(true)
	tp.frames(3)
	return tp
}

func (tp *testPage) frames(n int) {
	for range n {
		tp.tt.Frame()
	}
}

func (tp *testPage) expectTexts(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !tp.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tp.tt.Texts())
		}
	}
}

func TestShowsCategoryGames(t *testing.T) {
	tp := newTestPage(t)
	tp.expectTexts(t, "返回", "视觉小说", "共 2 个游戏", "橘光柠水随想曲", "Summer Pockets")
	if tp.tt.HasText("星空鉄道とシロの旅") {
		t.Error("game outside the category is listed")
	}

	// 状态筛选写进 GetCategoryGames 的请求，并改变计数文案。
	tp.filters.Status = enums.StatusCompleted
	tp.reload()
	tp.frames(2)
	tp.expectTexts(t, "共 1 个 已通关 游戏", "Summer Pockets")
}

func TestRemovesGame(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("从收藏夹中移除"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	tp.expectTexts(t, "共 1 个游戏")
	if n := tp.category.GameCount; n != 1 {
		t.Errorf("category game count = %d", n)
	}
}

func TestBatchRemoves(t *testing.T) {
	tp := newTestPage(t)
	for _, label := range []string{"批量选择", "全选", "批量移除"} {
		if err := tp.tt.Click(label); err != nil {
			t.Fatalf("click %q: %v", label, err)
		}
		tp.frames(2)
	}
	tp.expectTexts(t, "这个收藏夹还没有游戏")
	if tp.filters.Batch {
		t.Error("still in batch mode after removing")
	}
}

func TestAddsGameFromModal(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("添加游戏"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "添加游戏到收藏夹")
	if got := len(tp.addGames.games); got != 1 || tp.addGames.games[0].ID != "2" {
		t.Fatalf("candidates = %+v", tp.addGames.games)
	}
	if err := tp.tt.Click("星空鉄道とシロの旅"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	tp.expectTexts(t, "共 3 个游戏", "没有可添加的游戏")
}

func TestRenamesAndDeletes(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("操作"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if err := tp.tt.Click("重命名"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "编辑收藏夹", "4/60")
	tp.renameValue = "galgame"
	tp.frames(1)
	if err := tp.tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.expectTexts(t, "galgame")
	if cat, _ := tp.svc.Category.GetCategoryByID(tp.id); cat.Name != "galgame" {
		t.Errorf("saved name = %q", cat.Name)
	}

	if err := tp.tt.Click("操作"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if err := tp.tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	tp.expectTexts(t, `确定要删除收藏夹 "galgame" 吗？此操作无法撤销。`)
	if err := tp.tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if got := tp.env.Router.Path(); got != env.PathCategories {
		t.Fatalf("path after delete = %q", got)
	}
	if _, err := tp.svc.Category.GetCategoryByID(tp.id); err == nil {
		t.Error("category still exists")
	}
}

func TestSystemCategory(t *testing.T) {
	tp := newTestPage(t)
	tp.Open(gamehelper.SystemFavoritesCategoryID, tp.tt.Now())
	tp.frames(3)
	tp.expectTexts(t, "系统", "这个收藏夹还没有游戏")
	if tp.tt.HasText("操作") {
		t.Error("system category has the rename / delete menu")
	}
}

func TestNotFound(t *testing.T) {
	tp := newTestPage(t)
	tp.Open("missing", tp.tt.Now())
	tp.frames(3)
	tp.expectTexts(t, "未找到该分类")
	if err := tp.tt.Click("返回列表"); err != nil {
		t.Fatal(err)
	}
	if got := tp.env.Router.Path(); got != env.PathCategories {
		t.Fatalf("path = %q", got)
	}
}

// 批量模式下在卡片上按右键切换它的选中状态（拖动选择的起点，useDragSelection）。
func TestBatchRightPressSelects(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("批量选择"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	if err := tp.tt.RightClick("橘光柠水随想曲"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if !tp.selected["0"] || len(tp.selected) != 1 || tp.tt.Menu() != nil {
		t.Fatalf("selected %v, menu %v", tp.selected, tp.tt.Menu())
	}
}
