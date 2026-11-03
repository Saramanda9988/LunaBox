package importer

import (
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
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
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

// fakeImport 替换导入服务里会联网的方法。
type fakeImport struct {
	api.ImportService
	match      func(name string) (vo.BatchImportMetadataMatchResult, error)
	duplicates []vo.ImportMetadataDuplicateResult
	imported   []vo.BatchImportCandidate
	result     service.ImportResult

	preview    []service.PreviewGame
	selections []vo.ImportSelection
	skip       bool
	samePath   string
}

func (f *fakeImport) FetchMetadataForCandidateWithPreference(name string, _ enums.SourceType) (vo.BatchImportMetadataMatchResult, error) {
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
	a.name, a.company = "星之梦", "ゆずソフト"
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
