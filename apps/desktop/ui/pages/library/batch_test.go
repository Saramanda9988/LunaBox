package library

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop/api"
	"lunabox/desktop/ui/env/envtest"
	"lunabox/desktop/ui/locales"
)

// fakeBatch 实现批量操作用到的服务方法（其余方法来自内嵌的 nil 接口，调用会 panic）。
type fakeBatch struct {
	api.GameService
	api.CategoryService
	api.IntegrationService

	b          *envtest.Backend
	statusIDs  []string
	status     string
	deleted    []string
	addedGames []string
	addedCats  []string
	steam      service.SteamLaunchStatus
	steamItems map[string]service.SteamBatchImportItemResult
	steamCalls [][]string
	missing    vo.MissingPathScanResult
	err        error
}

func (f *fakeBatch) FindGamesWithMissingPath() (vo.MissingPathScanResult, error) {
	return f.missing, f.err
}

func (f *fakeBatch) BatchUpdateStatus(ids []string, status string) error {
	f.statusIDs, f.status = ids, status
	return f.err
}

func (f *fakeBatch) DeleteGames(ids []string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = ids
	f.b.Games = slices.DeleteFunc(f.b.Games, func(g models.Game) bool { return slices.Contains(ids, g.ID) })
	return nil
}

func (f *fakeBatch) GetCategories() ([]vo.CategoryVO, error) {
	return []vo.CategoryVO{{ID: "c1", Name: "Galgame"}, {ID: "c2", Name: "通关"}}, nil
}

func (f *fakeBatch) AddGamesToCategories(gameIDs, categoryIDs []string) error {
	f.addedGames, f.addedCats = gameIDs, categoryIDs
	return f.err
}

func (f *fakeBatch) GetGameSteamStatus(string) (service.SteamLaunchStatus, error) {
	return f.steam, nil
}

func (f *fakeBatch) BatchImportGamesToSteam(ids []string) (service.SteamBatchImportResult, error) {
	f.steamCalls = append(f.steamCalls, ids)
	var r service.SteamBatchImportResult
	for _, id := range ids {
		it, ok := f.steamItems[id]
		if !ok {
			it = service.SteamBatchImportItemResult{GameID: id, Imported: true, Status: service.SteamLaunchStatus{Ready: true}}
		}
		r.Items = append(r.Items, it)
		switch {
		case it.Error != "" || !it.Status.Ready:
			r.FailedCount++
		case it.Imported:
			r.ImportedCount++
		default:
			r.ExistingCount++
		}
	}
	return r, nil
}

func newBatchPage(t *testing.T, n int) (*Page, *fakeBatch, *ui.Tester) {
	t.Helper()
	l, b, tt := newTestPage(t, n)
	f := &fakeBatch{b: b}
	l.env.Svc = &api.Services{Config: &b.Cfg, Game: f, Category: f, Integration: f}
	if err := tt.Click(locales.NativeBatchSelect()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return l, f, tt
}

func TestBatchSetStatus(t *testing.T) {
	l, f, tt := newBatchPage(t, 3)
	l.selected["0"], l.selected["2"] = true, true
	tt.Frame()
	if err := tt.Click(locales.LibrarySetStatus()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(locales.CommonCompleted()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Equal(slices.Sorted(slices.Values(f.statusIDs)), []string{"0", "2"}) || f.status != string(enums.StatusCompleted) {
		t.Fatalf("BatchUpdateStatus(%v, %q)", f.statusIDs, f.status)
	}
	want := locales.LibraryToastBatchStatusUpdated("2", locales.CommonCompleted())
	if !slices.Contains(tt.Announcements(), want) {
		t.Fatalf("toast %q missing: %q", want, tt.Announcements())
	}
	if !l.filters.Batch || len(l.selected) != 2 {
		t.Fatal("setting the status should keep the batch selection")
	}
}

func TestBatchAddToCategories(t *testing.T) {
	l, f, tt := newBatchPage(t, 3)
	l.selected["1"] = true
	tt.Frame()
	if err := tt.Click(locales.LibraryBatchAddToFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Galgame") || !tt.HasText(locales.CommonAdd()) {
		t.Fatalf("category modal not shown: %q", tt.Texts())
	}
	_ = tt.Click("Galgame")
	tt.Frame()
	if err := tt.Click(locales.CommonAdd()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Equal(f.addedGames, []string{"1"}) || !slices.Equal(f.addedCats, []string{"c1"}) {
		t.Fatalf("AddGamesToCategories(%v, %v)", f.addedGames, f.addedCats)
	}
	if l.filters.Batch || len(l.selected) != 0 {
		t.Fatal("batch mode should end after adding")
	}
}

func TestBatchDelete(t *testing.T) {
	l, f, tt := newBatchPage(t, 3)
	l.selected["0"] = true
	tt.Frame()
	if err := tt.Click(locales.LibraryToastBatchDeleteTitle()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.LibraryToastBatchDeleteConfirmMsg("1")) {
		t.Fatalf("confirm not shown: %q", tt.Texts())
	}
	if err := tt.Click(locales.CommonConfirm()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if !slices.Equal(f.deleted, []string{"0"}) {
		t.Fatalf("DeleteGames(%v)", f.deleted)
	}
	if l.filters.Batch || l.pages.Total != 2 {
		t.Fatalf("batch %v, total %d after delete", l.filters.Batch, l.pages.Total)
	}
}

func TestBatchDeleteFailureKeepsSelection(t *testing.T) {
	l, f, tt := newBatchPage(t, 3)
	f.err = errors.New("db")
	l.selected["0"] = true
	l.batch.confirmDelete = true
	tt.Frame()
	_ = tt.Click(locales.CommonConfirm())
	tt.Frame()
	if !slices.Contains(tt.Announcements(), locales.LibraryToastBatchDeleteFailed()) || !l.filters.Batch || !l.selected["0"] {
		t.Fatalf("toasts %q, batch %v, selected %v", tt.Announcements(), l.filters.Batch, l.selected)
	}
}

func TestBatchImportToSteamKeepsFailures(t *testing.T) {
	l, f, tt := newBatchPage(t, 3)
	f.steam = service.SteamLaunchStatus{SteamInstalled: true}
	f.steamItems = map[string]service.SteamBatchImportItemResult{
		"2": {GameID: "2", Status: service.SteamLaunchStatus{State: "executable_required"}},
	}
	l.selected["0"], l.selected["2"] = true, true
	l.batch.steamOpen, l.batch.steamStatus = true, &f.steam // 导入按钮只在 Windows 上显示，这里直接打开弹窗
	tt.Frame()
	if !tt.HasText(locales.LibraryToastBatchSteamImportConfirmMsg("2")) {
		t.Fatalf("confirm step not shown: %q", tt.Texts())
	}
	if err := tt.Click(locales.SteamImportImport()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(f.steamCalls) != 1 || !slices.Equal(slices.Sorted(slices.Values(f.steamCalls[0])), []string{"0", "2"}) {
		t.Fatalf("BatchImportGamesToSteam calls %v", f.steamCalls)
	}
	// 一个成功一个失败：弹窗关闭，只留下失败的游戏
	if l.batch.steamOpen || len(l.selected) != 1 || !l.selected["2"] || !l.filters.Batch {
		t.Fatalf("open %v, selected %v", l.batch.steamOpen, l.selected)
	}
	want := locales.LibraryToastBatchSteamImportSummary("1", "1", "0")
	if !slices.Contains(tt.Announcements(), want) {
		t.Fatalf("summary %q missing: %q", want, tt.Announcements())
	}
}

func TestBatchImportToSteamWaitsWhileSteamRuns(t *testing.T) {
	l, f, tt := newBatchPage(t, 3)
	f.steam = service.SteamLaunchStatus{SteamInstalled: true, SteamRunning: true}
	l.selected["1"] = true
	l.batch.steamOpen, l.batch.steamStatus = true, &f.steam
	tt.Frame()
	if !tt.HasText(locales.SteamImportSteamRunningTitle()) || !tt.HasText(locales.SteamImportRetry()) {
		t.Fatalf("steam running step not shown: %q", tt.Texts())
	}
	f.steam.SteamRunning = false
	if err := tt.Click(locales.SteamImportRetry()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.SteamImportImport()) {
		t.Fatalf("retry did not re-check: %q", tt.Texts())
	}
}

// 批量模式下在卡片上按右键切换它的选中状态（拖动选择的起点，useDragSelection），不弹右键菜单；拖过其他卡片的
// 行为在 grid 包的 DragSelection 测试里。
func TestBatchRightPressSelects(t *testing.T) {
	l, _, tt := newBatchPage(t, 3)
	if err := tt.RightClick("Summer Pockets"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !l.selected["1"] || len(l.selected) != 1 || tt.Menu() != nil {
		t.Fatalf("selected %v, menu %v", l.selected, tt.Menu())
	}
	if err := tt.RightClick("Summer Pockets"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(l.selected) != 0 {
		t.Fatalf("selected %v after the second right press", l.selected)
	}
	// 左键点选不受影响。
	if err := tt.Click("Summer Pockets"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !l.selected["1"] || len(l.selected) != 1 {
		t.Fatalf("selected %v after a click", l.selected)
	}
}

// 检查失效路径：只显示并选中失效的游戏，清除搜索；删除后回到全部游戏。所在磁盘不可用的游戏只提示，不选中。
func TestMissingPathsSelectsAndFilters(t *testing.T) {
	l, f, tt := newBatchPage(t, 6)
	f.missing = vo.MissingPathScanResult{
		Checked:     5,
		Missing:     []vo.MissingPathGame{{GameID: "1"}, {GameID: "4"}},
		Unavailable: []vo.MissingPathGame{{GameID: "5"}},
	}
	l.filters.SearchFor("summer")
	l.Reload()
	tt.Frame()

	if err := tt.Click(locales.NativeMissingPathScan()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if l.filters.Debounced != "" || l.pages.Total != 2 || len(l.selected) != 2 || !l.selected["1"] || !l.selected["4"] {
		t.Fatalf("search %q, total %d, selected %v", l.filters.Debounced, l.pages.Total, l.selected)
	}
	if !tt.HasText(locales.NativeMissingPathFilter("2")) {
		t.Errorf("missing banner in %q", tt.Texts())
	}
	toasts := tt.Announcements()
	if !slices.Contains(toasts, locales.NativeMissingPathFound("2")) || !slices.Contains(toasts, locales.NativeMissingPathUnavailable("1")) {
		t.Errorf("toasts %q", toasts)
	}

	l.batch.confirmDelete = true
	tt.Frame()
	if err := tt.Click(locales.CommonConfirm()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if !slices.Equal(f.deleted, []string{"1", "4"}) || l.missing.active() || l.filters.Batch || l.pages.Total != 4 {
		t.Fatalf("deleted %v, missing %v, batch %v, total %d", f.deleted, l.missing.ids, l.filters.Batch, l.pages.Total)
	}
}

func TestMissingPathsNoneKeepsLibrary(t *testing.T) {
	l, _, tt := newBatchPage(t, 3)
	if err := tt.Click(locales.NativeMissingPathScan()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if l.missing.active() || l.pages.Total != 3 || len(l.selected) != 0 {
		t.Fatalf("missing %v, total %d, selected %v", l.missing.ids, l.pages.Total, l.selected)
	}
	if !slices.Contains(tt.Announcements(), locales.NativeMissingPathNone()) {
		t.Errorf("toasts %q", tt.Announcements())
	}
}

// 退出批量模式时一并回到全部游戏。
func TestMissingPathsExitWithBatch(t *testing.T) {
	l, f, tt := newBatchPage(t, 4)
	f.missing = vo.MissingPathScanResult{Missing: []vo.MissingPathGame{{GameID: "2"}}}
	if err := tt.Click(locales.NativeMissingPathScan()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if l.pages.Total != 1 {
		t.Fatalf("total %d", l.pages.Total)
	}
	if err := tt.Click(locales.NativeBatchSelect()); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if l.missing.active() || l.filters.Batch || l.pages.Total != 4 {
		t.Fatalf("missing %v, batch %v, total %d", l.missing.ids, l.filters.Batch, l.pages.Total)
	}
}
