package review

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
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

// newHarness 在真实服务（临时 DuckDB）上打开面板，外面套一层详情页的 p-8 与背景。自动保存不等待。
func newHarness(t *testing.T, g models.Game) *harness {
	t.Helper()
	old := autoSaveDelay
	autoSaveDelay = 0
	t.Cleanup(func() { autoSaveDelay = old })
	h := &harness{svc: hosttest.NewServices(t), game: g}
	hosttest.AddGame(t, h.svc, g)
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
	h.frames(3)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

// clickStar 点击第 value 分（第 (value-1)/2 颗星的左半或右半）。
func (h *harness) clickStar(t *testing.T, value int) {
	t.Helper()
	r, ok := h.tt.Find("个人评分")
	if !ok {
		t.Fatal("stars not found")
	}
	i := (value - 1) / 2
	x := r.X + float32(i)*40 + 8
	if value%2 == 0 {
		x += 16
	}
	h.tt.ClickAt(x, r.Y+16)
	h.frames(3)
}

func (h *harness) saved(t *testing.T) *models.GameReview {
	t.Helper()
	rv, err := h.svc.GameReview.GetGameReview(h.game.ID)
	if err != nil {
		t.Fatal(err)
	}
	return rv
}

func TestEmptyReview(t *testing.T) {
	h := newHarness(t, models.Game{ID: "review-empty", Name: "橘光柠水随想曲", SourceType: enums.Bangumi, SourceID: "1"})
	h.expect(t, "请先留步...", "橘光柠水随想曲", "请发表你对这部作品的评分", "尚未评分", "0/300",
		"包含剧透", "此标记会在发送到 Hikarinagi 时一并提交。", "修改后会自动保存", "同步到 Bangumi", "同步到 Hikarinagi")
	if h.tt.HasText("清除评分") {
		t.Error("clear button without rating")
	}
	if h.saved(t) != nil {
		t.Error("opening the panel saved a review")
	}
	// 未授权：同步按钮不可用，点击不发生任何事。
	h.click(t, "同步到 Bangumi")
	if h.state.syncing != "" || h.saved(t) != nil {
		t.Errorf("sync without auth: %+v", h.state)
	}
}

func TestEditAndAutoSave(t *testing.T) {
	h := newHarness(t, models.Game{ID: "review-edit", Name: "评价测试"})

	h.clickStar(t, 7)
	h.expect(t, "7 / 10", "清除评分", "已自动保存")
	if rv := h.saved(t); rv == nil || rv.Rating == nil || *rv.Rating != 7 {
		t.Fatalf("saved = %+v", rv)
	}
	h.clickStar(t, 10)
	if rv := h.saved(t); rv.Rating == nil || *rv.Rating != 10 {
		t.Fatalf("rating = %v", rv.Rating)
	}

	h.click(t, "评价内容")
	h.tt.Type("共通线很好，个人线")
	h.frames(3)
	h.expect(t, "9/300", "已自动保存")
	if rv := h.saved(t); rv.Content != "共通线很好，个人线" {
		t.Fatalf("content = %q", rv.Content)
	}

	h.click(t, "包含剧透")
	if rv := h.saved(t); !rv.IsSpoiler {
		t.Error("spoiler not saved")
	}

	h.click(t, "清除评分")
	h.expect(t, "尚未评分")
	if rv := h.saved(t); rv.Rating != nil || rv.Content != "共通线很好，个人线" || !rv.IsSpoiler {
		t.Fatalf("after clear = %+v", rv)
	}

	// 重新进入：读到保存的评价，状态是已自动保存。
	h.state = State{}
	h.frames(3)
	if h.state.content != "共通线很好，个人线" || !h.state.spoiler || h.state.rating != 0 || h.state.status != statusSaved {
		t.Errorf("reloaded = %+v", h.state)
	}
	h.expect(t, "已自动保存", "尚未评分", "9/300")
}

func TestDebounce(t *testing.T) {
	h := newHarness(t, models.Game{ID: "review-debounce", Name: "防抖"})
	autoSaveDelay = time.Hour
	h.clickStar(t, 4)
	h.expect(t, "等待自动保存", "4 / 10")
	if h.saved(t) != nil {
		t.Fatal("saved before the delay")
	}
	autoSaveDelay = 0
	h.frames(2)
	if rv := h.saved(t); rv == nil || *rv.Rating != 4 {
		t.Fatalf("saved = %+v", rv)
	}
}

func TestMaxLength(t *testing.T) {
	h := newHarness(t, models.Game{ID: "review-max", Name: "字数"})
	h.click(t, "评价内容")
	h.tt.Type(strings.Repeat("あ", 310))
	h.frames(3)
	if n := utf16Len(h.state.content); n != contentMaxLength {
		t.Fatalf("content length = %d", n)
	}
	h.expect(t, "300/300")
}

func TestClampEdit(t *testing.T) {
	for _, tc := range []struct {
		old, cur string
		max      int
		want     string
	}{
		{"abc", "abcd", 5, "abcd"},
		{"abcd", "abXYZcd", 5, "abXcd"},       // 中间插入只截掉插入的部分
		{"abcde", "abcdeX", 5, "abcde"},       // 已满时不再接受输入
		{"abcdefg", "abcdefgX", 5, "abcdefg"}, // 原本超长：不截断原文
		{"ab", "ab😀😀", 5, "ab😀"},              // 按 UTF-16 码元计
	} {
		if got := clampEdit(tc.old, tc.cur, tc.max); got != tc.want {
			t.Errorf("clampEdit(%q, %q, %d) = %q, want %q", tc.old, tc.cur, tc.max, got, tc.want)
		}
	}
}

func TestGameHasProvider(t *testing.T) {
	g := &models.Game{SourceType: enums.VNDB, SourceID: "v1", MetadataSources: []models.GameMetadataSource{
		{SourceType: enums.Bangumi, SourceID: "123"}, {SourceType: enums.Hikarinagi, SourceID: " "},
	}}
	if !gameHasProvider(g, enums.Bangumi) || gameHasProvider(g, enums.Hikarinagi) || !gameHasProvider(g, enums.VNDB) {
		t.Errorf("gameHasProvider wrong for %+v", g)
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把评价面板（填写后、骨架屏、悬停预览）存成图片，用于目测。
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
	h := newHarness(t, models.Game{ID: "review-shot", Name: "Summer Pockets", SourceType: enums.Bangumi, SourceID: "1"})
	save(h, "review-empty.png")
	h.clickStar(t, 7)
	h.click(t, "评价内容")
	h.tt.Type("夏日的岛上，与少女们相遇。共通线节奏舒缓，个人线各有亮点。")
	h.frames(3)
	h.click(t, "包含剧透")
	r, _ := h.tt.Find("个人评分")
	h.tt.Move(r.X+3*40+24, r.Y+16) // 悬停预览 8 分
	h.frames(2)
	h.tt.Advance(300 * time.Millisecond) // 等 150ms 的宽度过渡
	h.frames(2)
	save(h, "review-filled.png")

	h.state = State{}
	h.state.gameID = h.game.ID
	h.state.loading, h.state.loadStart = true, time.Now().Add(-time.Second)
	h.frames(2)
	save(h, "review-skeleton.png")
}
