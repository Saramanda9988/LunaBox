package importer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop/api"
	"lunabox/desktop/host"
	"lunabox/desktop/host/hosttest"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/env/envtest"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// harness 在 Tester 里构建一个弹窗，env 用真实服务（或替换掉其中的导入服务）。
type harness struct {
	svc       *host.Backend
	env       *env.Env
	tt        *ui.Tester
	completed int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	return &harness{svc: svc, env: &env.Env{Svc: svc.API()}}
}

// run 开始构建 view（NewTester 会立即构建第一帧，所以在弹窗创建好之后调用）。
func (h *harness) run(view func(h *harness, c *ui.Context, th *theme.Theme)) {
	h.tt = ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1.5)
		h.env.FlushToasts(c)
		view(h, c, th)
	}, 1280, 800)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("click %q: %v in %q", s, err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Fatalf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) gameNames(t *testing.T) []string {
	t.Helper()
	rows, err := h.svc.DB.Query(`SELECT name FROM games ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	return names
}

// gameLibrary 在临时目录里放两个游戏文件夹，各有一个可执行文件。
func gameLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, g := range []string{"Sakura Game", "Hoshi Game"} {
		if err := os.MkdirAll(filepath.Join(dir, g), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, g, strings.ReplaceAll(g, " ", "")+".exe"), []byte("MZ"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// 批量导入：扫描配置的游戏库目录，不匹配元数据直接导入，导入的游戏从列表里移除。
func TestBatchImportScansAndImports(t *testing.T) {
	var b *BatchImport
	h := newHarness(t)
	view := func(h *harness, c *ui.Context, th *theme.Theme) {
		b.View(c, th, func() { h.completed++ })
	}
	b = NewBatchImport(h.env)
	h.run(view)
	h.svc.Config.GameLibraryPath = gameLibrary(t)
	b.Open()
	h.frames(2)
	h.expect(t, locales.BatchImportModalTitle(), locales.BatchImportModalSelectDir())

	h.click(t, locales.BatchImportModalBtnScanGameLibrary())
	h.expect(t, "Sakura Game", "Hoshi Game", locales.BatchImportModalDetected(), locales.BatchImportModalStartMatching(),
		locales.BatchImportModalImportCount("2"))

	// 取消选中一个，只导入另一个
	if err := h.tt.Click("Hoshi Game"); err != nil { // 复选框的无障碍名称是搜索名
		t.Fatal(err)
	}
	h.frames(1)
	h.expect(t, locales.BatchImportModalImportCount("1"))
	h.click(t, locales.BatchImportModalImportCount("1"))
	h.frames(2)
	if got := h.gameNames(t); !slices.Equal(got, []string{"Sakura Game"}) {
		t.Fatalf("imported %q", got)
	}
	if h.completed != 1 || !slices.Contains(h.tt.Announcements(), locales.BatchImportModalToastImportSuccess("1")) {
		t.Fatalf("completed %d, toasts %q", h.completed, h.tt.Announcements())
	}
	// 回到预览，只剩下没有导入的那个
	if len(b.flow.Candidates) != 1 || b.flow.Candidates[0].SearchName != "Hoshi Game" || b.step != "preview" {
		t.Fatalf("step %q, candidates %+v", b.step, b.flow.Candidates)
	}

	// 再扫描一次：已经导入的游戏归入“跳过”
	b.step = "select"
	h.frames(1)
	h.click(t, locales.BatchImportModalBtnScanGameLibrary())
	h.expect(t, locales.BatchImportModalSkippedExistingSummary("1"))
	h.click(t, locales.BatchImportModalSkippedExistingView())
	h.expect(t, locales.BatchImportModalSkippedExistingTitle())
}

// 拖放导入：处理拖进来的目录后进入预览；没有可导入的内容时提示并关闭。
func TestDragDropImport(t *testing.T) {
	var d *DragDrop
	var pending []string
	h := newHarness(t)
	view := func(h *harness, c *ui.Context, th *theme.Theme) {
		if pending != nil {
			d.Drop(c, pending)
			pending = nil
		}
		d.View(c, th, func() { h.completed++ })
	}
	d = NewDragDrop(h.env)
	h.run(view)

	pending = []string{t.TempDir()}
	h.frames(3)
	if d.Active() || !slices.Contains(h.tt.Announcements(), locales.DragDropImportModalToastNoValidGames()) {
		t.Fatalf("empty drop: active %v, toasts %q", d.Active(), h.tt.Announcements())
	}

	lib := gameLibrary(t)
	pending = []string{filepath.Join(lib, "Sakura Game")}
	h.frames(3)
	h.expect(t, "Sakura Game", locales.BatchImportModalImportCount("1"))
	h.click(t, locales.CommonCancel())
	if d.Active() {
		t.Fatal("cancel should close the modal")
	}
}

// 拖放提示整个都不接收指针：指针（与拖进来的文件，两者共用命中测试）落到下面的拖放目标上。
// 提示里的卡片或色块挡住目标时，拖着文件经过窗口会让目标与提示每帧交替出现（闪烁），放下也导入不了。
func TestDropOverlayPassesPointer(t *testing.T) {
	th := theme.New(true, 1)
	var hovered bool
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c.Key("target")).Fill().Children(func() {
			hovered = ui.Box(c.Key("under")).Grow(1).Hovered()
		})
		DropOverlay(c, th, 0)
	}, 800, 600)
	for _, p := range [][2]float32{{400, 300}, {20, 20}} { // 中间的卡片与四周的色块
		tt.Move(p[0], p[1])
		tt.Frame()
		if !hovered {
			t.Fatalf("overlay takes the pointer at %v", p)
		}
	}
}

// fakeImport 替换导入服务里会联网的方法。
type fakeImport struct {
	api.ImportService
	match      func(name string) (vo.BatchImportMetadataMatchResult, error)
	matchCtx   context.Context // 最近一次匹配请求的 ctx
	duplicates []vo.ImportMetadataDuplicateResult
	imported   []vo.BatchImportCandidate
	result     service.ImportResult

	preview    []service.PreviewGame
	selections []vo.ImportSelection
	skip       bool
	samePath   string
}

func (f *fakeImport) FetchMetadataForCandidateWithPreferenceContext(ctx context.Context, name string, _ enums.SourceType) (vo.BatchImportMetadataMatchResult, error) {
	f.matchCtx = ctx
	return f.match(name)
}

func (f *fakeImport) CheckImportMetadataDuplicates([]vo.ImportMetadataDuplicateRequest) ([]vo.ImportMetadataDuplicateResult, error) {
	return f.duplicates, nil
}

func (f *fakeImport) BatchImportGames(c []vo.BatchImportCandidate) (service.ImportResult, error) {
	f.imported = c
	return f.result, nil
}

func (f *fakeImport) SelectJSONFile() (string, error) { return "export.json", nil }

func (f *fakeImport) PreviewPlayniteImport(string) ([]service.PreviewGame, error) {
	return f.preview, nil
}

func (f *fakeImport) ImportFromPlayniteWithSelection(_ string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
	f.skip, f.samePath, f.selections = skip, same, sel
	return service.ImportResult{Success: len(sel)}, nil
}

func flowWith(fake *fakeImport, names ...string) *Flow {
	f := newFlow(&env.Env{Svc: &api.Services{Import: fake}})
	f.Delay = -1
	for _, n := range names {
		f.Candidates = append(f.Candidates, Candidate{SearchName: n, Selected: true, MatchStatus: MatchPending})
	}
	return f
}

func runMatch(t *testing.T, f *Flow, s matchSettings) bool {
	t.Helper()
	done := false
	tt := ui.NewTester(func(c *ui.Context) {
		if s.all == nil {
			return
		}
		f.StartMatch(s, func() { done = true })
		s.all = nil
	}, 100, 100)
	tt.Frame()
	return done
}

func meta(source enums.SourceType, name, id string) vo.GameMetadataFromWebVO {
	return vo.GameMetadataFromWebVO{Source: source, Game: models.Game{Name: name, SourceID: id, SourceType: source}}
}

// 没有优先来源时按默认顺序取结果（Bangumi 在 VNDB 之前），之后检查元数据是否已在库里。
func TestMatchPicksBestSourceAndMarksDuplicates(t *testing.T) {
	fake := &fakeImport{
		match: func(name string) (vo.BatchImportMetadataMatchResult, error) {
			if name == "none" {
				return vo.BatchImportMetadataMatchResult{}, nil
			}
			return vo.BatchImportMetadataMatchResult{Matches: []vo.GameMetadataFromWebVO{meta(enums.VNDB, "v-"+name, "v1"), meta(enums.Bangumi, "b-"+name, "B9")}}, nil
		},
		duplicates: []vo.ImportMetadataDuplicateResult{{Source: enums.Bangumi, SourceID: "b9", Exists: true, ExistingID: "g1", ExistingName: "已有的游戏"}},
	}
	f := flowWith(fake, "sakura", "none")
	if !runMatch(t, f, matchSettings{all: []enums.SourceType{enums.Bangumi, enums.VNDB}}) {
		t.Fatal("onDone not called")
	}
	c0, c1 := f.Candidates[0], f.Candidates[1]
	if c0.MatchStatus != MatchMatched || c0.MatchSource != enums.Bangumi || c0.MatchedGame.Name != "b-sakura" || c0.DuplicateName != "已有的游戏" {
		t.Fatalf("candidate 0 = %+v", c0)
	}
	if c1.MatchStatus != MatchNotFound || c1.MatchError != locales.BatchImportModalNoMatchResult() {
		t.Fatalf("candidate 1 = %+v", c1)
	}
	if f.Progress.Current != 2 || f.Progress.Total != 2 {
		t.Fatalf("progress %+v", f.Progress)
	}
}

// 优先来源连续失败 3 次后暂停，剩下的保持待匹配。
func TestMatchPausesAfterPreferredSourceFailures(t *testing.T) {
	fake := &fakeImport{match: func(string) (vo.BatchImportMetadataMatchResult, error) {
		return vo.BatchImportMetadataMatchResult{PreferredError: "timeout"}, nil
	}}
	f := flowWith(fake, "a", "b", "c", "d")
	done := runMatch(t, f, matchSettings{preferred: enums.VNDB, preferredLabel: "VNDB", all: []enums.SourceType{enums.VNDB}})
	want := locales.BatchImportModalPreferredSourceConsecutiveFailurePause("VNDB", "3", "timeout")
	if !done || f.Pause != want {
		t.Fatalf("done %v, pause %q", done, f.Pause)
	}
	for i, st := range []MatchStatus{MatchError, MatchError, MatchError, MatchPending} {
		if f.Candidates[i].MatchStatus != st {
			t.Fatalf("candidate %d status %q", i, f.Candidates[i].MatchStatus)
		}
	}
	if f.matchable() != 4 {
		t.Fatalf("matchable %d", f.matchable())
	}
}

// 限流时立即暂停。
func TestMatchPausesWhenRateLimited(t *testing.T) {
	fake := &fakeImport{match: func(string) (vo.BatchImportMetadataMatchResult, error) {
		return vo.BatchImportMetadataMatchResult{PreferredRateLimited: true, PreferredError: "429"}, nil
	}}
	f := flowWith(fake, "a", "b")
	runMatch(t, f, matchSettings{preferred: enums.Bangumi, preferredLabel: "Bangumi", all: []enums.SourceType{enums.Bangumi}})
	if f.Pause != locales.BatchImportModalPreferredSourceRateLimitedPause("Bangumi", "429") || f.Candidates[1].MatchStatus != MatchPending {
		t.Fatalf("pause %q, candidates %+v", f.Pause, f.Candidates)
	}
}

// 停止匹配时取消进行中的那一次请求：它的结果被丢弃，候选项保持待匹配，之后的不再请求。
func TestStopCancelsInFlightMatch(t *testing.T) {
	var f *Flow
	calls := 0
	fake := &fakeImport{}
	fake.match = func(name string) (vo.BatchImportMetadataMatchResult, error) {
		calls++
		f.Stop() // 请求还没返回时用户点了“停止匹配”
		return vo.BatchImportMetadataMatchResult{Matches: []vo.GameMetadataFromWebVO{meta(enums.Bangumi, "b-"+name, "1")}}, nil
	}
	f = flowWith(fake, "a", "b")
	if runMatch(t, f, matchSettings{all: []enums.SourceType{enums.Bangumi}}) {
		t.Fatal("onDone should not be called after Stop")
	}
	if fake.matchCtx == nil || fake.matchCtx.Err() == nil {
		t.Fatal("Stop should cancel the in-flight request")
	}
	if calls != 1 || f.Candidates[0].MatchStatus != MatchPending || f.Candidates[1].MatchStatus != MatchPending {
		t.Fatalf("calls %d, candidates %+v", calls, f.Candidates)
	}
}

// 导入结果：跳过与失败的留下并取消选中，其余按成功数量移除；只导入已匹配的时不提交其他候选项。
func TestImportAppliesResult(t *testing.T) {
	g := models.Game{Name: "Matched"}
	fake := &fakeImport{result: service.ImportResult{Success: 1, Skipped: 1, Failed: 1, SkippedNames: []string{"skip (重复)"}, FailedNames: []string{"Matched"}}}
	f := flowWith(fake, "ok", "skip", "fail", "unmatched")
	f.Candidates[2].MatchedGame, f.Candidates[2].MatchStatus = &g, MatchManual
	f.Candidates[3].Selected = false

	tt := ui.NewTester(func(c *ui.Context) {
		if fake.imported == nil {
			f.Import(c, false, func() {}, func() {}, nil)
		}
	}, 100, 100)
	tt.Frame()
	if len(fake.imported) != 3 {
		t.Fatalf("submitted %d candidates", len(fake.imported))
	}
	var names []string
	for _, c := range f.Candidates {
		names = append(names, c.SearchName)
	}
	if !slices.Equal(names, []string{"skip", "fail", "unmatched"}) {
		t.Fatalf("remaining %q", names)
	}
	if f.Candidates[0].Selected || f.Candidates[0].MatchError != "skip (重复)" ||
		f.Candidates[1].MatchStatus != MatchError || f.Candidates[1].MatchError != "Matched" {
		t.Fatalf("candidates %+v", f.Candidates)
	}

	// matchedOnly：只有已匹配的被提交
	if req := importRequest([]Candidate{{SearchName: "a", Selected: true}, {SearchName: "b", Selected: true, MatchedGame: &g, MatchStatus: MatchMatched}}, true); len(req) != 1 || req[0].SearchName != "b" {
		t.Fatalf("matched-only request %+v", req)
	}
}

// 从其他启动器导入：预览里已存在的不能选，没有路径的默认跳过；同路径的选择合并后可以导入。
func TestGameImportPreviewAndImport(t *testing.T) {
	var g *GameImport
	h := newHarness(t)
	view := func(h *harness, c *ui.Context, th *theme.Theme) {
		g.View(c, th, func() { h.completed++ })
	}
	fake := &fakeImport{preview: []service.PreviewGame{
		{Name: "New", Path: `C:\g\new.exe`, HasPath: true},
		{Name: "Exists", Exists: true, HasPath: true},
		{Name: "NoPath"},
		{Name: "Same", Exists: true, HasPath: true, ConflictType: "same_path"},
	}}
	svc := *h.env.Svc
	svc.Import = fake
	h.env.Svc = &svc
	g = NewGameImport(h.env)
	h.run(view)
	g.Open(SourcePlaynite)
	h.frames(2)
	h.expect(t, locales.GameImportModalPlayniteTitle(), locales.GameImportModalPlayniteBtn())
	h.click(t, locales.GameImportModalPlayniteBtn())
	h.frames(2)
	h.expect(t, "New", "NoPath", locales.GameImportModalSamePathActionTitle("1"), locales.GameImportModalSkipNoPath(),
		locales.GameImportModalImportCount("1"))

	h.click(t, locales.GameImportModalSamePathMerge())
	h.click(t, locales.GameImportModalSkipNoPath()) // 不再跳过没有路径的
	h.expect(t, locales.GameImportModalStatusUpdate())
	// 合并与不跳过后它们变得可以导入，但还没有选中
	h.click(t, "Same")
	h.click(t, "NoPath")
	h.expect(t, locales.GameImportModalImportCount("3"))
	h.click(t, locales.GameImportModalImportCount("3"))
	h.frames(2)
	var names []string
	for _, s := range fake.selections {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"New", "NoPath", "Same"}) || fake.skip || fake.samePath != "merge" {
		t.Fatalf("selections %q, skip %v, samePath %q", names, fake.skip, fake.samePath)
	}
	h.expect(t, locales.GameImportModalResultSuccess())
	if h.completed != 1 {
		t.Fatalf("completed %d", h.completed)
	}
	h.click(t, locales.CommonComplete())
	if g.IsOpen() {
		t.Fatal("complete should close")
	}
}

// 手动添加远程游戏：没有路径，状态是“想玩”。
func TestAddGameManualRemote(t *testing.T) {
	var a *AddGame
	h := newHarness(t)
	view := func(h *harness, c *ui.Context, th *theme.Theme) {
		a.View(c, th, func() { h.completed++ })
	}
	a = NewAddGame(h.env)
	h.run(view)
	a.Open()
	h.frames(2)
	h.expect(t, locales.AddGameModalChooseImportType())
	h.click(t, locales.AddGameModalRemoteGame())
	h.click(t, locales.CommonManualAdd())
	h.expect(t, locales.AddGameModalRemoteManualFillInfo())
	a.name, a.form.company = "星之梦", "ゆずソフト"
	h.frames(1)
	h.click(t, locales.CommonSave())
	h.frames(2)
	if h.completed != 1 || a.IsOpen() {
		t.Fatalf("completed %d, open %v", h.completed, a.IsOpen())
	}
	var status, path, company string
	if err := h.svc.DB.QueryRow(`SELECT status, path, company FROM games WHERE name = ?`, "星之梦").Scan(&status, &path, &company); err != nil {
		t.Fatal(err)
	}
	if status != string(enums.StatusWantToPlay) || path != "" || company != "ゆずソフト" {
		t.Fatalf("saved status %q, path %q, company %q", status, path, company)
	}
}

// 本地游戏手动添加：没有选择可执行文件时“手动添加”不可用；名称从可执行文件所在的文件夹推断。
func TestAddGameLocalNeedsExecutable(t *testing.T) {
	if got := inferName(`D:\Games\Sakura Game\bin\game.exe`); got != "bin" {
		t.Fatalf("inferName = %q", got)
	}
	if got := inferName(`/Applications/Foo.app`); got != "Foo" {
		t.Fatalf("inferName(.app) = %q", got)
	}
	var a *AddGame
	h := newHarness(t)
	view := func(h *harness, c *ui.Context, th *theme.Theme) { a.View(c, th, nil) }
	a = NewAddGame(h.env)
	h.run(view)
	a.Open()
	h.frames(2)
	h.click(t, locales.AddGameModalLocalGame())
	_ = h.tt.Click(locales.CommonManualAdd())
	h.frames(1)
	if a.step != "local" {
		t.Fatalf("manual add without an executable: step %q", a.step)
	}
	a.exe, a.name = `D:\Games\Sakura\sakura.exe`, "Sakura"
	h.frames(1)
	h.click(t, locales.CommonManualAdd())
	h.click(t, locales.CommonSave())
	h.frames(2)
	var path, status string
	if err := h.svc.DB.QueryRow(`SELECT path, status FROM games WHERE name = 'Sakura'`).Scan(&path, &status); err != nil {
		t.Fatal(err)
	}
	if path != `D:\Games\Sakura\sakura.exe` || status != string(enums.StatusNotStarted) {
		t.Fatalf("path %q, status %q", path, status)
	}
}

// fakeMetadataGame 替换真实服务的按名称获取元数据：during 在请求返回之前调用（模拟用户在等待时点了取消）。
type fakeMetadataGame struct {
	api.GameService
	during  func()
	ctx     context.Context
	results []vo.GameMetadataFromWebVO
}

func (f *fakeMetadataGame) FetchMetadataByNameContext(ctx context.Context, _ string) ([]vo.GameMetadataFromWebVO, error) {
	f.ctx = ctx
	if f.during != nil {
		f.during()
	}
	return f.results, nil
}

// 按名称搜索时可以取消：取消按钮复位搜索状态并取消请求；请求返回前取消的，结果被丢弃，不进入结果步骤。
func TestAddGameCancelSearch(t *testing.T) {
	var a *AddGame
	h := newHarness(t)
	fake := &fakeMetadataGame{GameService: h.env.Svc.Game, results: []vo.GameMetadataFromWebVO{meta(enums.Bangumi, "Sakura", "1")}}
	h.env.Svc.Game = fake
	view := func(h *harness, c *ui.Context, th *theme.Theme) { a.View(c, th, nil) }
	a = NewAddGame(h.env)
	h.run(view)
	a.Open()
	h.frames(2)
	h.click(t, locales.AddGameModalRemoteGame())
	a.name = "Sakura"
	h.frames(1)

	fake.during = func() { a.cancelSearch() }
	h.click(t, locales.AddGameModalSearchMeta())
	if fake.ctx == nil || fake.ctx.Err() == nil {
		t.Fatal("cancel should cancel the request")
	}
	if a.step != "remote" || a.loading || a.search.Active() || len(a.results) != 0 {
		t.Fatalf("after cancel: step %q, loading %v, results %d", a.step, a.loading, len(a.results))
	}

	// 等待中搜索按钮显示“点击取消”，点它取消搜索，弹窗与步骤不变。
	a.loading, a.searchBy = true, "name"
	ctx := a.search.Start()
	h.frames(1)
	h.click(t, locales.NativeClickToCancel())
	if ctx.Err() == nil || a.loading || !a.IsOpen() || a.step != "remote" || h.tt.HasText(locales.NativeClickToCancel()) {
		t.Fatalf("click to cancel: ctx %v, loading %v, open %v, step %q", ctx.Err(), a.loading, a.IsOpen(), a.step)
	}

	// 不取消时照常进入结果步骤。
	fake.during = nil
	h.click(t, locales.AddGameModalSearchMeta())
	if a.step != "results" || len(a.results) != 1 {
		t.Fatalf("search: step %q, results %d", a.step, len(a.results))
	}
}

// 手动填写与详情页的编辑字段对齐：别名、评分、发售日期、标签（从库里的标签候选中选，或输入新的）、
// 元数据来源（远程游戏上一步填的 ID 带过来，可以再添加、切换默认），保存后都写进数据库。
func TestAddGameManualFullForm(t *testing.T) {
	var a *AddGame
	h := newHarness(t)
	h.env.Backend = &envtest.Backend{Tags: []string{"纯爱", "乐队"}}
	view := func(h *harness, c *ui.Context, th *theme.Theme) { a.View(c, th, func() { h.completed++ }) }
	a = NewAddGame(h.env)
	h.run(view)
	h.tt.SetSize(1280, 1600) // 表单不用滚动就能点到底部的按钮
	a.Open()
	h.frames(2)
	h.click(t, locales.AddGameModalRemoteGame())
	a.manualID = "160559"
	h.frames(1)
	h.click(t, locales.CommonManualAdd())
	h.expect(t, locales.NativeAddGameCoverEmpty(), locales.GameEditAliases(), locales.GameEditRating(), locales.GameEditReleaseDate(),
		locales.NativeTags(), locales.GameEditMetadataSources())
	f := &a.form
	if len(f.sources) != 1 || f.sources[0].SourceID != "160559" {
		t.Fatalf("prefilled sources %+v", f.sources)
	}
	first := f.sources[0].SourceType
	a.name = "千恋＊万花"
	f.ratingValue, f.rating, f.releaseDate = 8.5, "8.5", "2016-07-15"
	h.frames(1)

	h.click(t, locales.GameEditAddAlias())
	h.tt.Type("Senren")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if !slices.Equal(f.aliases, []string{"Senren"}) {
		t.Fatalf("aliases %q", f.aliases)
	}

	h.click(t, locales.TagsInputPlaceholder())
	h.tt.Type("纯")
	h.frames(2)
	h.click(t, "纯爱") // 库里的候选
	h.tt.Type("新标签")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if !slices.Equal(f.tags, []string{"纯爱", "新标签"}) {
		t.Fatalf("tags %q", f.tags)
	}

	second := enums.VNDB
	if first == enums.VNDB {
		second = enums.Bangumi
	}
	f.sourceDraftType, f.sourceDraftID = second, "v19073"
	if second == enums.Bangumi {
		f.sourceDraftID = "160559"
	}
	h.frames(1)
	h.click(t, locales.GameEditAddSource())
	if len(f.sources) != 2 {
		t.Fatalf("sources %+v", f.sources)
	}
	h.click(t, locales.GameEditSetDefaultSource()) // 第二个设为默认
	if def, _ := f.defaultSourceOf(f.sources); def.SourceType != second {
		t.Fatalf("default %q", def.SourceType)
	}
	h.click(t, locales.GameEditSetDefaultSource()) // 再换回第一个（不会按第二个来源联网补标签）
	if def, _ := f.defaultSourceOf(f.sources); def.SourceType != first {
		t.Fatalf("default %q", def.SourceType)
	}

	h.click(t, locales.CommonSave())
	h.frames(2)
	if h.completed != 1 || a.IsOpen() {
		t.Fatalf("completed %d, open %v", h.completed, a.IsOpen())
	}
	var id string
	if err := h.svc.DB.QueryRow(`SELECT id FROM games WHERE name = ?`, "千恋＊万花").Scan(&id); err != nil {
		t.Fatal(err)
	}
	g, err := h.svc.Game.GetGameByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if g.Rating != 8.5 || g.ReleaseDate != "2016-07-15" || !slices.Equal(g.Aliases, []string{"Senren"}) ||
		g.SourceType != first || len(g.MetadataSources) != 2 || g.Status != enums.StatusWantToPlay {
		t.Fatalf("saved game %+v", g)
	}
	tags, err := h.svc.Tag.GetTagsByGame(id)
	if err != nil {
		t.Fatal(err)
	}
	var user []string
	for _, tag := range tags {
		if tag.Source == "user" {
			user = append(user, tag.Name)
		}
	}
	slices.Sort(user)
	if !slices.Equal(user, []string{"新标签", "纯爱"}) {
		t.Fatalf("user tags %q", user)
	}
}
