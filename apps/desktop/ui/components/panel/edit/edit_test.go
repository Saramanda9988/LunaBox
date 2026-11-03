package edit

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

// harness 模拟详情页：OnChange 替换游戏，OnSaved 从数据库重新读取，OnDeleted 记一次。
type harness struct {
	svc     *host.Backend
	env     *env.Env
	game    models.Game
	state   State
	changes int
	saved   int
	deleted int
	tt      *ui.Tester
}

const (
	winW = 1280
	winH = 1900 // 面板很高，窗口要放得下才能点到下面的按钮
)

func testGame() models.Game {
	return models.Game{
		ID: "g1", Name: "千恋＊万花", Company: "ゆずソフト", Summary: "穗织的神社里，\n刺着一把神刀。",
		Rating: 8.5, ReleaseDate: "2016-07-15", CoverURL: "https://example.com/cover.jpg",
		Path: `D:\Games\Senren\SenrenBanka.exe`, GameDirectory: `D:\Games\Senren`,
		SourceType: enums.Bangumi, SourceID: "123",
		MetadataSources: []models.GameMetadataSource{
			{SourceType: enums.Bangumi, SourceID: "123"},
			{SourceType: enums.VNDB, SourceID: "v19073"},
		},
	}
}

func newHarness(t *testing.T, g models.Game) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t)}
	hosttest.AddGame(t, h.svc, g)
	h.reload(t)
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: winW - 256}
	h.tt = ui.NewTester(h.view, winW, winH)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(2)
	return h
}

func (h *harness) reload(t testing.TB) {
	g, err := h.svc.Game.GetGameByID("g1")
	if err != nil {
		t.Fatal(err)
	}
	h.game = g
}

// view 是详情页里面板所在的位置：侧边栏右边、p-8 的主内容区。
func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Padding(32, 32, 32, 256+32).Background(t.AppBg).Children(func() {
		View(c, t, &h.state, panel.Game{
			Env:  h.env,
			Game: &h.game,
			OnChange: func(g models.Game) {
				h.game = g
				h.changes++
			},
			OnSaved: func() {
				h.saved++
				if g, err := h.svc.Game.GetGameByID("g1"); err == nil {
					h.game = g
				}
			},
			OnDeleted: func() { h.deleted++ },
		})
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
		t.Fatalf("%v in %q", err, h.tt.Texts())
	}
	h.frames(2)
}

// clickBelow 点击标签下方的输入框（标签与输入框的无障碍名称相同，Tester 先找到标签）。
func (h *harness) clickBelow(t *testing.T, label string, dx float32) {
	h.clickBelowBy(t, label, dx, 24)
}

// clickBelowBy 点击标签下方 dy 处。
func (h *harness) clickBelowBy(t *testing.T, label string, dx, dy float32) {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok {
		t.Fatalf("no %q in %q", label, h.tt.Texts())
	}
	h.tt.ClickAt(r.X+dx, r.Y+r.H+dy)
	h.frames(2)
}

func (h *harness) expectTexts(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

// waitAnimation 等抽屉、弹窗的进出动画走完。先构建一帧，让测试从外部改的状态（drawerOpen）开始动画。
func (h *harness) waitAnimation() {
	h.tt.Frame()
	h.tt.Advance(350 * time.Millisecond)
	h.frames(3)
}

func TestShowsFields(t *testing.T) {
	h := newHarness(t, testGame())
	h.expectTexts(t,
		"游戏名称", "游戏别名", "别名会参与游戏库搜索；支持设置多个别名。",
		"封面图片", "支持远端 URL、本地图片选取；复制图片后可在输入框粘贴导入",
		"远端封面源", "本地封面丢失或加载失败时，将回退到此远端地址。", "开发商",
		"评分", "/ 10", "按 10 分制保存，留空表示暂无评分。", "发售日期", "2016/07/15",
		"游戏目录", "可执行文件", "游戏的安装根目录，可与启动文件所在目录不同。",
		"存档路径", "您可以指定游戏的存档目录或文件，用于云端备份和多端同步", "简介",
		"元数据来源", "Bangumi", "ID", "123", "等 2 个", "编辑",
		"锁定元数据", "NSFW 游戏", "标记后，可根据全局游戏设置自动模糊此游戏的封面。",
		"从远端更新信息", "删除",
	)
	if h.changes != 0 {
		t.Errorf("building changed the game %d times", h.changes)
	}
}

func TestEditTextFields(t *testing.T) {
	h := newHarness(t, testGame())
	h.clickBelow(t, "游戏名称", 400)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type("Senren Banka")
	h.frames(2)
	if h.game.Name != "Senren Banka" || h.changes == 0 {
		t.Fatalf("name = %q after %d changes", h.game.Name, h.changes)
	}
	// 可执行文件显示游戏目录下的相对路径；输入相对路径会拼回游戏目录。
	h.clickBelow(t, "可执行文件", 100)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type(`bin\game.exe`)
	h.frames(2)
	if h.game.Path != `D:\Games\Senren\bin\game.exe` {
		t.Errorf("path = %q", h.game.Path)
	}
}

func TestRating(t *testing.T) {
	h := newHarness(t, testGame())
	h.clickBelow(t, "评分", 100)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type("12")
	h.frames(2)
	if h.game.Rating != 10 || h.state.rating != "10" {
		t.Fatalf("rating = %v, text %q", h.game.Rating, h.state.rating)
	}
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Key(0, ui.KeyBackspace)
	h.frames(2)
	if h.game.Rating != 0 || h.state.rating != "" {
		t.Fatalf("cleared rating = %v, text %q", h.game.Rating, h.state.rating)
	}
	h.tt.Type("7.5")
	h.frames(2)
	if h.game.Rating != 7.5 {
		t.Fatalf("rating = %v", h.game.Rating)
	}
}

func TestAliases(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "添加别名")
	h.tt.Type("Senren")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if !slices.Equal(h.game.Aliases, []string{"Senren"}) || h.state.aliasAdding {
		t.Fatalf("aliases %q, adding %v", h.game.Aliases, h.state.aliasAdding)
	}
	// 重复（忽略大小写）的不加。
	h.click(t, "添加别名")
	h.tt.Type("senren")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if len(h.game.Aliases) != 1 {
		t.Fatalf("duplicate added: %q", h.game.Aliases)
	}
	// Esc 放弃草稿。
	h.click(t, "添加别名")
	h.tt.Type("放弃")
	h.tt.Key(0, ui.KeyEscape)
	h.frames(2)
	if h.state.aliasAdding || len(h.game.Aliases) != 1 {
		t.Fatalf("escape: adding %v, aliases %q", h.state.aliasAdding, h.game.Aliases)
	}
	// 草稿为空时退格删除最后一个别名。
	h.click(t, "添加别名")
	h.tt.Key(0, ui.KeyBackspace)
	h.frames(2)
	if len(h.game.Aliases) != 0 {
		t.Fatalf("backspace: aliases %q", h.game.Aliases)
	}
	// 失去焦点时添加。
	h.tt.Type("ばんか")
	h.frames(1)
	h.clickBelow(t, "开发商", 400)
	if !slices.Equal(h.game.Aliases, []string{"ばんか"}) {
		t.Fatalf("blur: aliases %q", h.game.Aliases)
	}
	h.click(t, "删除别名 ばんか")
	if len(h.game.Aliases) != 0 {
		t.Fatalf("remove: aliases %q", h.game.Aliases)
	}
}

// 输入法组字时按下的 Enter 只选定候选，松开 Enter 时添加（原版 submitAliasAfterComposition）；
// 用空格选词后松开空格不添加。
func TestAliasAfterComposition(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "添加别名")
	h.tt.Compose("qianlian", 8)
	h.frames(1)
	h.tt.Type("千恋") // 按下 Enter：输入法提交候选，这个键不交给应用
	h.frames(2)
	if len(h.game.Aliases) != 0 || !h.state.aliasAdding {
		t.Fatalf("committing the candidate added %q", h.game.Aliases)
	}
	h.tt.KeyUp(ui.KeyEnter)
	h.frames(2)
	if !slices.Equal(h.game.Aliases, []string{"千恋"}) || h.state.aliasAdding {
		t.Fatalf("enter after composition: aliases %q, adding %v", h.game.Aliases, h.state.aliasAdding)
	}

	h.click(t, "添加别名")
	h.tt.Compose("wanhua", 6)
	h.frames(1)
	h.tt.Type("万花")
	h.frames(2)
	h.tt.KeyUp(ui.KeySpace)
	h.frames(2)
	if len(h.game.Aliases) != 1 || h.state.aliasDraft != "万花" {
		t.Fatalf("space after composition: aliases %q, draft %q", h.game.Aliases, h.state.aliasDraft)
	}
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if !slices.Equal(h.game.Aliases, []string{"千恋", "万花"}) {
		t.Fatalf("enter: aliases %q", h.game.Aliases)
	}
}

func TestReleaseDatePicker(t *testing.T) {
	h := newHarness(t, testGame())
	h.clickBelow(t, "发售日期", 100)
	if !h.state.dateOpen {
		t.Fatal("calendar not open")
	}
	h.expectTexts(t, "2016/07", "Su", "Sa")
	h.click(t, "Next month")
	h.expectTexts(t, "2016/08")
	h.click(t, "2016-08-20")
	if h.game.ReleaseDate != "2016-08-20" || h.state.dateOpen {
		t.Fatalf("release = %q, open %v", h.game.ReleaseDate, h.state.dateOpen)
	}
	h.expectTexts(t, "2016/08/20")
	h.click(t, "清除发售日期")
	if h.game.ReleaseDate != "" {
		t.Fatalf("release = %q", h.game.ReleaseDate)
	}
}

func TestUnsupportedReleaseDate(t *testing.T) {
	g := testGame()
	g.ReleaseDate = "2016年7月"
	h := newHarness(t, g)
	h.expectTexts(t, "当前值：2016年7月。选择日期后会按 YYYY-MM-DD 保存。", "清除发售日期")
}

func TestSwitches(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "锁定元数据")
	if !h.game.MetadataLocked {
		t.Fatal("not locked")
	}
	// 锁定后“从远端更新信息”禁用。
	h.click(t, "从远端更新信息")
	if h.state.fieldsOpen {
		t.Fatal("field modal opened while locked")
	}
	h.click(t, "NSFW 游戏")
	if !h.game.IsNSFW {
		t.Fatal("not nsfw")
	}
}

func TestDeleteGame(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "删除")
	h.expectTexts(t, "删除游戏", `确定要删除游戏 "千恋＊万花" 吗？此操作将从库中移除该游戏，但不会删除本地游戏文件。`, "确认删除")
	h.click(t, "取消")
	if h.state.deleteOpen || h.deleted != 0 {
		t.Fatalf("cancel: open %v, deleted %d", h.state.deleteOpen, h.deleted)
	}
	h.click(t, "删除")
	h.click(t, "确认删除")
	if h.deleted != 1 {
		t.Fatalf("deleted %d", h.deleted)
	}
	if _, err := h.svc.Game.GetGameByID("g1"); err == nil {
		t.Fatal("game still in the database")
	}
}

func TestFieldSelectModal(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "从远端更新信息")
	h.expectTexts(t, "选择要更新的字段", "只会从远端覆盖你勾选的字段，未勾选的本地内容会保留。", "更新全部字段",
		"已选择 8 / 8 个字段", "游戏名称", "使用远端条目标题覆盖本地名称", "自动标签", "开始更新")
	h.click(t, "自动标签")
	h.expectTexts(t, "已选择 7 / 8 个字段")
	if slices.Contains(h.state.fields, enums.MetadataUpdateFieldTags) {
		t.Fatalf("fields %q", h.state.fields)
	}
	h.click(t, "关闭")
	if h.state.fieldsOpen {
		t.Fatal("modal still open")
	}
}

func TestMetadataDrawer(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "编辑")
	h.waitAnimation()
	h.expectTexts(t, "元数据来源", "Bangumi", "VNDB", "添加来源", "或",
		"根据当前游戏名称重新搜索（仅更新来源关联）", "根据当前游戏名称重新搜索（更新元数据）")
	if !h.tt.HasText("默认") {
		t.Errorf("no default star in %q", h.tt.Texts())
	}

	// 把 VNDB 设为默认。
	h.click(t, "设为默认")
	if h.game.SourceType != enums.VNDB || h.saved == 0 {
		t.Fatalf("default = %q, saved %d", h.game.SourceType, h.saved)
	}

	// 修改 Bangumi 的 ID（VNDB 成为默认后，“Bangumi”只出现在抽屉里），500ms 后自动保存。
	h.clickBelowBy(t, "Bangumi", 200, 40)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type("999")
	h.frames(1)
	if h.state.sourceEdits[enums.Bangumi] != "999" {
		t.Fatalf("edits %q", h.state.sourceEdits)
	}
	h.tt.Advance(sourceSaveDelay + 100*time.Millisecond)
	h.frames(2)
	if i := slices.IndexFunc(h.game.MetadataSources, func(s models.GameMetadataSource) bool { return s.SourceType == enums.Bangumi }); i < 0 || h.game.MetadataSources[i].SourceID != "999" {
		t.Fatalf("sources after auto-save: %+v", h.game.MetadataSources)
	}

	// 删除 Bangumi（第一行）。
	h.click(t, "删除元数据来源")
	if len(h.game.MetadataSources) != 1 || h.game.MetadataSources[0].SourceType != enums.VNDB {
		t.Fatalf("sources after delete: %+v", h.game.MetadataSources)
	}
}

// 抽屉打开时离开页面：还没到期的来源 ID 由详情页调用 Flush 立即保存（原版卸载时补存）。
func TestFlushSourceSaves(t *testing.T) {
	h := newHarness(t, testGame())
	h.click(t, "编辑")
	h.waitAnimation()
	h.click(t, "设为默认") // VNDB 成为默认后，“Bangumi”只出现在抽屉里
	h.clickBelowBy(t, "Bangumi", 200, 40)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type("777")
	h.frames(1)
	if len(h.state.sourceSaves) != 1 {
		t.Fatalf("pending saves %+v", h.state.sourceSaves)
	}
	saved := h.saved
	Flush(&h.state, panel.Game{Env: h.env, Game: &h.game, OnSaved: func() { h.saved++ }})
	if len(h.state.sourceSaves) != 0 || h.saved != saved+1 {
		t.Fatalf("pending %+v, saved %d", h.state.sourceSaves, h.saved-saved)
	}
	h.reload(t)
	if i := slices.IndexFunc(h.game.MetadataSources, func(s models.GameMetadataSource) bool { return s.SourceType == enums.Bangumi }); i < 0 || h.game.MetadataSources[i].SourceID != "777" {
		t.Fatalf("sources after flush: %+v", h.game.MetadataSources)
	}
}

func TestAddSource(t *testing.T) {
	g := testGame()
	h := newHarness(t, g)
	h.state.drawerOpen = true
	h.waitAnimation()
	h.state.sourceDraftType = enums.Steam
	h.state.sourceDraftID = "1144400"
	h.frames(2)
	r, ok := h.tt.Find("添加来源")
	if !ok {
		t.Fatal("no add form")
	}
	h.tt.ClickAt(float32(winW)-16-50, r.Y+r.H+8+20)
	h.frames(2)
	if !configured(&h.game, enums.Steam) {
		t.Fatalf("sources %+v", h.game.MetadataSources)
	}
	if h.state.sourceDraftID != "" || h.state.sourceDraftType != enums.Ymgal {
		t.Errorf("draft %q %q", h.state.sourceDraftType, h.state.sourceDraftID)
	}
}

func searchResults() []vo.GameMetadataFromWebVO {
	return []vo.GameMetadataFromWebVO{
		{Source: enums.Bangumi, Game: models.Game{Name: "千恋＊万花", SourceID: "160559"}},
		{Source: enums.VNDB, Game: models.Game{Name: "Senren * Banka", SourceID: "v19073"}},
		{Source: enums.Ymgal, Game: models.Game{Name: "千恋万花", SourceID: "15832"}},
	}
}

func TestSearchModal(t *testing.T) {
	h := newHarness(t, testGame())
	h.state.results, h.state.searchOpen = searchResults(), true
	h.frames(2)
	h.expectTexts(t, "重新选择元数据来源", "哪些结果是您期望的？", "Senren * Banka", "来自 VNDB", "来自 月幕gal", "←返回")
	h.click(t, "查看候选结果选择说明")
	h.expectTexts(t, "如何选择候选结果", "每个数据源仅保留一个正确结果。")
	h.tt.Key(0, ui.KeyEscape)
	h.frames(2)

	h.click(t, "移除 月幕gal 的这个结果")
	if len(h.state.results) != 2 {
		t.Fatalf("results %d", len(h.state.results))
	}
	// 选择 VNDB：两个来源都关联，VNDB 设为默认。
	h.click(t, "Senren * Banka")
	if h.state.searchOpen {
		t.Fatal("modal still open")
	}
	if h.game.SourceType != enums.VNDB || h.game.SourceID != "v19073" || !configured(&h.game, enums.Bangumi) {
		t.Fatalf("game %q %q %+v", h.game.SourceType, h.game.SourceID, h.game.MetadataSources)
	}
	for _, s := range h.game.MetadataSources {
		if s.SourceType == enums.Bangumi && s.SourceID != "160559" {
			t.Errorf("bangumi id = %q", s.SourceID)
		}
	}
}

func TestSearchModalDuplicateSource(t *testing.T) {
	h := newHarness(t, testGame())
	h.state.results = append(searchResults(), vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: models.Game{Name: "Other", SourceID: "v1"}})
	h.state.searchOpen = true
	h.frames(2)
	h.click(t, "Senren * Banka")
	if !h.state.searchOpen || h.game.SourceType != enums.Bangumi {
		t.Fatalf("applied despite duplicate: open %v, default %q", h.state.searchOpen, h.game.SourceType)
	}
}

func TestSearchModalEmpty(t *testing.T) {
	h := newHarness(t, testGame())
	h.state.searchOpen = true
	h.frames(2)
	h.expectTexts(t, "没有找到匹配的元数据")
	h.click(t, "←返回")
	if h.state.searchOpen {
		t.Fatal("still open")
	}
}

func TestAsyncPatches(t *testing.T) {
	h := newHarness(t, testGame())
	h.state.patch("g1", func(g *models.Game) { g.SavePath = `D:\Saves` })
	h.state.patch("other", func(g *models.Game) { g.SavePath = "wrong" })
	h.frames(2)
	if h.game.SavePath != `D:\Saves` {
		t.Fatalf("save path = %q", h.game.SavePath)
	}
}

func TestPathHelpers(t *testing.T) {
	for _, c := range []struct{ path, dir, display string }{
		{`D:\Games\A\a.exe`, `D:\Games\A`, `a.exe`},
		{`D:\Games\A\a.exe`, `d:/games/a/`, `a.exe`},
		{`E:\a.exe`, `D:\Games\A`, `E:\a.exe`},
		{`a.exe`, ``, `a.exe`},
	} {
		if got := executableDisplayPath(c.path, c.dir); got != c.display {
			t.Errorf("display(%q, %q) = %q", c.path, c.dir, got)
		}
	}
	for _, c := range []struct{ in, dir, want string }{
		{`bin\a.exe`, `D:\Games\A\`, `D:\Games\A\bin\a.exe`},
		{`/a.exe`, `/games`, `/games/a.exe`},
		{`C:\x.exe`, `D:\Games`, `C:\x.exe`},
		{`\\nas\x.exe`, `D:\Games`, `\\nas\x.exe`},
	} {
		if got := resolveExecutablePath(c.in, c.dir); got != c.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", c.in, c.dir, got, c.want)
		}
	}
	if formatDateInputValue("2023-02-29") != "" || formatDateInputValue(" 2024-02-29 ") != "2024-02-29" {
		t.Error("formatDateInputValue")
	}
	rows := calendarRows(time.Date(2024, 5, 1, 0, 0, 0, 0, time.Local))
	if got := rows[0].dates[0].Format("2006-01-02"); got != "2024-04-28" {
		t.Errorf("first calendar day = %s", got)
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把面板与它的抽屉、弹窗存成图片（深色，150%），用于目测。
func TestScreenshotDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(h *harness, name string) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	g := testGame()
	g.MetadataSources = append(g.MetadataSources, models.GameMetadataSource{SourceType: enums.Ymgal, SourceID: "15832"},
		models.GameMetadataSource{SourceType: enums.Steam, SourceID: "1144400"},
		models.GameMetadataSource{SourceType: enums.ErogameScape, SourceID: "22414"})
	h := newHarness(t, g)
	h.game.Aliases = []string{"Senren Banka", "千恋万花"}
	h.frames(2)
	save(h, "edit-panel.png")

	h.click(t, "添加别名")
	h.tt.Type("サノバ")
	h.clickBelow(t, "发售日期", 100)
	save(h, "edit-date-picker.png")
	h.tt.Key(0, ui.KeyEscape)
	h.frames(2)

	h.state.drawerOpen = true
	h.waitAnimation()
	save(h, "edit-drawer.png")
	h.state.drawerOpen = false
	h.waitAnimation()

	h.state.fieldsOpen, h.state.fields = true, slices.Clone(enums.DefaultMetadataUpdateFields[:5])
	h.frames(2)
	save(h, "edit-fields.png")
	h.state.fieldsOpen = false

	h.state.results, h.state.searchOpen = append(searchResults(), searchResults()...), true
	h.frames(2)
	save(h, "edit-search.png")
	h.click(t, "查看候选结果选择说明")
	save(h, "edit-search-help.png")
	h.state.searchOpen = false
	h.state.resultsUI.Reset()

	h.state.deleteOpen = true
	h.frames(2)
	save(h, "edit-delete.png")
}
