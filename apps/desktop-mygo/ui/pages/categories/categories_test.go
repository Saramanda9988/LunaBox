package categories

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
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
}

// newTestPage 打开收藏夹列表：系统收藏夹（含一个游戏）与“视觉小说”“待补”两个收藏夹。
func newTestPage(t *testing.T) *testPage {
	t.Helper()
	svc := hosttest.NewServices(t)
	b := envtest.NewBackend(3)
	for _, g := range b.Games {
		hosttest.AddGame(t, svc, g)
	}
	for _, name := range []string{"视觉小说", "待补"} {
		if err := svc.Category.AddCategory(name, ""); err != nil {
			t.Fatal(err)
		}
	}
	tp := &testPage{svc: svc}
	if err := svc.Category.AddGameToCategory("0", tp.categoryID(t, "视觉小说")); err != nil {
		t.Fatal(err)
	}
	e := &env.Env{Backend: b, Svc: svc.API(), Router: ui.NewRouter(env.PathCategories), Covers: covers.New(), Theme: theme.New(true, 1.5), MainWidth: 1024}
	tp.Page = New(e)
	tp.Open(time.Now())
	tp.tt = ui.NewTester(tp.View, 1280, 800)
	tp.tt.SetScale(1.5)
	tp.tt.SetDark(true)
	tp.frames(3)
	return tp
}

func (tp *testPage) categoryID(t *testing.T, name string) string {
	t.Helper()
	cats, err := tp.svc.Category.GetCategories()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cats {
		if c.Name == name {
			return c.ID
		}
	}
	t.Fatalf("no category %q", name)
	return ""
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

func TestListsAndSearches(t *testing.T) {
	tp := newTestPage(t)
	tp.expectTexts(t, "收藏", "最喜欢的游戏", "视觉小说", "待补")

	tp.filters.Search = "视觉"
	tp.frames(1)
	tp.expectTexts(t, "视觉小说")
	if tp.tt.HasText("待补") {
		t.Errorf("search did not filter: %q", tp.tt.Texts())
	}
}

func TestSortsByGameCount(t *testing.T) {
	tp := newTestPage(t)
	tp.filters.SortBy, tp.filters.SortOrder = sortByGameCount, "desc"
	if got := tp.visible()[0].Name; got != "视觉小说" {
		t.Fatalf("first by game count = %q", got)
	}
}

func TestOpensCategory(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("视觉小说"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if got, want := tp.env.Router.Path(), env.CategoryPath(tp.categoryID(t, "视觉小说")); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestCreatesCategory(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("新建收藏夹"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	tp.tt.Type("新的收藏")
	tp.frames(1)
	tp.expectTexts(t, "4/60")
	if err := tp.tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	tp.expectTexts(t, "新的收藏")
	if tp.addOpen {
		t.Error("modal still open after creating")
	}
}

// 批量模式下在卡片上按右键切换它的选中状态（拖动选择的起点，useDragSelection）；系统收藏夹不是拖动选择的目标。
func TestBatchRightPressSelects(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("批量选择"); err != nil {
		t.Fatal(err)
	}
	tp.frames(2)
	id := tp.categoryID(t, "视觉小说")
	if err := tp.tt.RightClick("视觉小说"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if !tp.selected[id] || tp.selectedCount() != 1 || tp.tt.Menu() != nil {
		t.Fatalf("selected %v, menu %v", tp.selected, tp.tt.Menu())
	}
	if err := tp.tt.RightClick("视觉小说"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if err := tp.tt.RightClick("最喜欢的游戏"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if tp.selectedCount() != 0 {
		t.Fatalf("selected %v after toggling off and pressing the system collection", tp.selected)
	}
}

func TestBatchDeletesSkippingSystem(t *testing.T) {
	tp := newTestPage(t)
	if err := tp.tt.Click("批量选择"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if err := tp.tt.Click("全选"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if got := tp.selectedCount(); got != 2 {
		t.Fatalf("selected %d categories, want 2 (system excluded)", got)
	}
	if err := tp.tt.Click("批量删除收藏夹"); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	tp.expectTexts(t, "确定要删除选中的 2 个收藏夹吗？此操作无法撤销。")
	if err := tp.tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	tp.frames(3)
	cats, err := tp.svc.Category.GetCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 1 || !cats[0].IsSystem {
		t.Fatalf("categories after batch delete: %v", names(cats))
	}
	if tp.filters.Batch {
		t.Error("still in batch mode after deleting")
	}
}

func names(cats []vo.CategoryVO) []string {
	var out []string
	for _, c := range cats {
		out = append(out, c.Name)
	}
	return out
}

// 名称按拼音排序，与原版的 localeCompare 一致：数字、拉丁字母在汉字之前，汉字按读音（阿 a、测 ce、最 zui），
// 不按码位（最 U+6700 < 测 U+6D4B < 阿 U+963F）。
func TestSortsNamesByPinyin(t *testing.T) {
	names := []string{"最喜欢的游戏", "阿", "测试", "bangumi", "2", "Steam"}
	slices.SortFunc(names, func(a, b string) int { return localeCompare("zh-CN", a, b) })
	want := []string{"2", "bangumi", "Steam", "阿", "测试", "最喜欢的游戏"}
	if !slices.Equal(names, want) {
		t.Fatalf("order = %v, want %v", names, want)
	}
}
