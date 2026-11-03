package metadata

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

type harness struct {
	svc   *host.Backend
	env   *env.Env
	cfg   appconf.AppConfig
	state State
	tt    *ui.Tester
}

// newHarness 在真实服务（临时 DuckDB 与数据目录）上打开面板，外面套一层设置页分区内容的 p-5 space-y-4 列；
// Change 与 PatchLive 直接改本地的配置。
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t)}
	h.cfg = *h.svc.Config
	h.cfg.MetadataSources = nil // 未设置时显示默认来源
	h.env = &env.Env{Svc: h.svc.API(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1100, 1300)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	change := func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg) }
	ui.Column(c).Fill().Background(t.AppBg).Padding(32).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(16).Background(t.PanelBg).Children(func() {
			View(c, t, &h.state, panel.Settings{Env: h.env, Config: &h.cfg, Change: change, PatchLive: change, Refresh: func() {}})
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
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) find(t *testing.T, s string) ui.Rect {
	t.Helper()
	r, ok := h.tt.Find(s)
	if !ok {
		t.Fatalf("no %q in %q", s, h.tt.Texts())
	}
	return r
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

// expectNot 检查没有显示（完全相同的）文字。
func (h *harness) expectNot(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if _, ok := h.tt.Find(s); ok {
			t.Errorf("unexpected %q", s)
		}
	}
}

func TestRendersSourcesAndOptions(t *testing.T) {
	h := newHarness(t)
	h.expect(t, locales.SettingsMetadataSourceTitle(),
		"Bangumi", "VNDB", "Hikarinagi", locales.GameEditSourceKungal(), "Steam", "DLsite", "TouchGAL", "Ymgal", "ErogameScape",
		locales.SettingsMetadataSourceHintsBangumi(), locales.SettingsMetadataSourceHintsErogamescape(),
		locales.SettingsMetadataBangumiCoverSource(), locales.SettingsMetadataVndbCoverSource(), locales.SettingsMetadataCoverSourcesHikarinagi(),
		locales.SettingsMetadataErogameScapeBaseURL(), locales.SettingsMetadataErogameScapeBaseURLHint(),
		locales.SettingsMetadataSteamPortraitCover(), locales.SettingsMetadataAllowDuplicateMetadataImport(),
		locales.SettingsMetadataEnableTagTranslation(), locales.SettingsMetadataScrapedTagLimit(), locales.SettingsMetadataScrapedTagLimitUnlimited(),
		locales.SettingsMetadataRefreshTitle(), locales.SettingsMetadataRefreshHint(),
		locales.SettingsMetadataRefreshButton(), locales.SettingsMetadataDownloadCoverButton(), locales.SettingsMetadataIdEnrichmentButton())
	// 来源卡片按 sourceItems 的顺序两列排列：Bangumi 与 VNDB 同一行，Hikarinagi 在下一行。
	bgm, vndb, hk := h.find(t, "Bangumi"), h.find(t, "VNDB"), h.find(t, "Hikarinagi")
	if bgm.Y != vndb.Y || vndb.X <= bgm.X || hk.Y <= bgm.Y {
		t.Errorf("source grid: bangumi %v vndb %v hikarinagi %v", bgm, vndb, hk)
	}
	// “批量更新”在选项之后：mt-6 被父元素的 space-y-4 盖掉，加上 pt-6。
	if r := h.find(t, locales.SettingsMetadataRefreshTitle()); r.Y <= h.find(t, locales.SettingsMetadataScrapedTagLimit()).Y {
		t.Errorf("refresh title at %v", r)
	}
}

func TestToggleSource(t *testing.T) {
	h := newHarness(t)
	// 没有设置时是默认的四个来源；打开的加到末尾。
	h.click(t, "DLsite")
	if want := []string{"bangumi", "vndb", "ymgal", "steam", "dlsite"}; !slices.Equal(h.cfg.MetadataSources, want) {
		t.Fatalf("sources = %q, want %q", h.cfg.MetadataSources, want)
	}
	h.click(t, "Bangumi")
	if want := []string{"vndb", "ymgal", "steam", "dlsite"}; !slices.Equal(h.cfg.MetadataSources, want) {
		t.Fatalf("sources = %q, want %q", h.cfg.MetadataSources, want)
	}
	// 未知与重复的来源被忽略；最后一个来源不能关闭。
	h.cfg.MetadataSources = []string{" VNDB ", "vndb", "unknown"}
	h.frames(2)
	h.click(t, "VNDB")
	h.expect(t, locales.SettingsMetadataToastAtLeastOneSource())
	if want := []string{" VNDB ", "vndb", "unknown"}; !slices.Equal(h.cfg.MetadataSources, want) {
		t.Fatalf("last source removed: %q", h.cfg.MetadataSources)
	}
}

func TestOptions(t *testing.T) {
	h := newHarness(t)
	h.cfg.SteamCoverOrientation = enums.SteamCoverOrientationPortrait
	h.cfg.ScrapedTagLimit = 10
	h.frames(2)

	h.click(t, locales.SettingsMetadataSteamPortraitCover())
	if h.cfg.SteamCoverOrientation != enums.SteamCoverOrientationLandscape {
		t.Errorf("steam orientation = %q", h.cfg.SteamCoverOrientation)
	}
	before := h.cfg.AllowDuplicateMetadataImport
	h.click(t, locales.SettingsMetadataAllowDuplicateMetadataImport())
	if h.cfg.AllowDuplicateMetadataImport == before {
		t.Error("allow duplicate import not toggled")
	}
	before = h.cfg.EnableTagTranslation
	h.click(t, locales.SettingsMetadataEnableTagTranslation())
	if h.cfg.EnableTagTranslation == before {
		t.Error("tag translation not toggled")
	}

	// “不限”：-1，再关回默认的 10。
	h.click(t, locales.SettingsMetadataScrapedTagLimitUnlimited())
	if h.cfg.ScrapedTagLimit != -1 {
		t.Fatalf("tag limit = %d", h.cfg.ScrapedTagLimit)
	}
	h.click(t, locales.SettingsMetadataScrapedTagLimitUnlimited())
	if h.cfg.ScrapedTagLimit != defaultScrapedTagLimit {
		t.Fatalf("tag limit = %d", h.cfg.ScrapedTagLimit)
	}

	// Bangumi 封面来源：打开第一个下拉框，选“原图”。
	h.click(t, locales.SettingsMetadataCoverSourcesHikarinagi())
	h.click(t, locales.SettingsMetadataCoverSourcesOriginal())
	if h.cfg.BangumiCoverSource != enums.MetadataCoverSourceOriginal || h.cfg.VNDBCoverSource != enums.MetadataCoverSourceHikarinagi {
		t.Errorf("cover sources = %q, %q", h.cfg.BangumiCoverSource, h.cfg.VNDBCoverSource)
	}

	// ErogameScape 地址：每次输入都改草稿。
	h.cfg.ErogameScapeBaseURL = ""
	h.frames(2)
	r := h.find(t, locales.SettingsMetadataErogameScapeBaseURL())
	h.tt.ClickAt(r.X+20, r.Y+r.H+8+20)
	h.frames(2)
	h.tt.Type("https://example.org")
	h.frames(3)
	if h.cfg.ErogameScapeBaseURL != "https://example.org" {
		t.Errorf("erogamescape url = %q", h.cfg.ErogameScapeBaseURL)
	}
}

func TestFieldSelectModal(t *testing.T) {
	h := newHarness(t)
	h.click(t, locales.SettingsMetadataRefreshButton())
	if !h.state.fieldsOpen {
		t.Fatal("field modal not open")
	}
	h.expect(t, locales.MetadataUpdateFieldsModalFullTitle(), locales.MetadataUpdateFieldsModalFullDescription(),
		locales.MetadataUpdateFieldsSelectAll(), "已选择 8 / 8 个字段", locales.MetadataUpdateFieldsModalContinue())

	h.click(t, locales.Key("metadataUpdateFields.tags.label"))
	h.expect(t, "已选择 7 / 8 个字段")
	if slices.Contains(h.state.fieldDraft, enums.MetadataUpdateFieldTags) {
		t.Fatalf("fields %q", h.state.fieldDraft)
	}

	// “更新全部字段”的开关在 p-3 的行的右端，关闭按钮（p-6 的面板右边）的下方。
	allSwitch := func() {
		cl, all := h.find(t, locales.CommonClose()), h.find(t, locales.MetadataUpdateFieldsSelectAll())
		h.tt.ClickAt(cl.X+cl.W-12-22, all.Y+all.H+2)
		h.frames(2)
	}
	allSwitch()
	h.expect(t, "已选择 8 / 8 个字段")
	allSwitch()
	h.expect(t, "已选择 0 / 8 个字段", locales.MetadataUpdateFieldsEmptyWarning())
	h.click(t, locales.MetadataUpdateFieldsModalContinue()) // 禁用：没有字段时不能继续
	if !h.state.fieldsOpen || h.state.confirmOpen {
		t.Fatal("continued without fields")
	}
	h.click(t, locales.Key("metadataUpdateFields.name.label"))
	h.click(t, locales.Key("metadataUpdateFields.cover.label"))
	h.expect(t, "已选择 2 / 8 个字段")

	// 继续：关闭字段弹窗，记住字段，打开确认。
	h.click(t, locales.MetadataUpdateFieldsModalContinue())
	want := []enums.MetadataUpdateField{enums.MetadataUpdateFieldName, enums.MetadataUpdateFieldCover}
	if h.state.fieldsOpen || !h.state.confirmOpen || !slices.Equal(h.state.selectedFields, want) {
		t.Fatalf("after continue: fields open %v, confirm %v, selected %q", h.state.fieldsOpen, h.state.confirmOpen, h.state.selectedFields)
	}
	h.expect(t, locales.SettingsMetadataModalRefreshTitle(), locales.SettingsMetadataModalRefreshMessage())
	h.click(t, locales.CommonCancel())
	if h.state.confirmOpen || h.state.refreshing {
		t.Fatal("cancel did not close the confirm")
	}

	// 再次打开时勾选上次用过的字段；关闭按钮关闭。
	h.click(t, locales.SettingsMetadataRefreshButton())
	h.expect(t, "已选择 2 / 8 个字段")
	h.click(t, locales.CommonClose())
	if h.state.fieldsOpen {
		t.Fatal("close did not close")
	}
}

func TestRefreshAll(t *testing.T) {
	h := newHarness(t)
	// 本地游戏没有来源，跳过（不访问网络）。
	hosttest.AddGame(t, h.svc, models.Game{ID: "local-1", Name: "本地游戏"})
	h.click(t, locales.SettingsMetadataRefreshButton())
	h.click(t, locales.MetadataUpdateFieldsModalContinue())
	h.click(t, locales.CommonConfirm())
	if h.state.refreshing || h.state.refreshOpen {
		t.Fatalf("refresh still running: %+v", h.state.progress)
	}
	h.expect(t, locales.SettingsMetadataToastRefreshSuccess("0", "0", "1", "0"))
	if h.state.progress.total != 1 || h.state.progress.skipped != 1 || h.state.progress.status != "done" {
		t.Errorf("progress = %+v", h.state.progress)
	}
}

func TestProgressModal(t *testing.T) {
	h := newHarness(t)
	h.state.refreshing, h.state.refreshOpen = true, true
	h.state.progress = newRefreshProgress("started")
	h.frames(2)
	h.expect(t, locales.SettingsMetadataProgressTitle(), "0 /0", locales.SettingsMetadataProgressPreparing(), locales.SettingsMetadataProgressHint(),
		locales.SettingsMetadataProgressUpdated(), locales.SettingsMetadataProgressFailed(), locales.SettingsMetadataProgressSkipped(),
		locales.SettingsMetadataProgressLocked(), locales.SettingsMetadataRefreshing())

	// 服务发出的进度事件更新弹窗。
	h.svc.Events.Emit(progressEvent, map[string]any{
		"status": "running", "current": 3, "total": 10, "game_name": "千恋＊万花",
		"updated_games": 2, "skipped_games": 0, "failed_games": 1, "locked_games": 0,
		"failed_game_ids": []string{"missing-game"}, "failed_game_names": []string{"千恋＊万花"},
	})
	h.frames(2)
	h.expect(t, "3 /10", "千恋＊万花")
	if h.state.progress.current != 3 || h.state.progress.updated != 2 || !slices.Equal(h.state.progress.failedGameIDs, []string{"missing-game"}) {
		t.Fatalf("progress = %+v", h.state.progress)
	}
	h.expectNot(t, locales.CommonComplete())

	// 结束且有失败：标题、“完成”与“重试失败项”。
	h.state.refreshing = false
	h.svc.Events.Emit(progressEvent, map[string]any{
		"status": "done", "current": 10, "total": 10, "game_name": "",
		"updated_games": 9, "skipped_games": 0, "failed_games": 1, "locked_games": 0,
		"failed_game_ids": []string{"missing-game"}, "failed_game_names": []string{"千恋＊万花"},
	})
	h.frames(2)
	h.expect(t, locales.SettingsMetadataProgressFailedTitle(), "10 /10", locales.CommonComplete(), locales.SettingsMetadataProgressRetryFailed("1"))
	h.expectNot(t, locales.SettingsMetadataProgressHint())

	// 重试：只更新失败的游戏（库里没有这个游戏，什么也不做），没有失败时关闭弹窗。
	h.click(t, locales.SettingsMetadataProgressRetryFailed("1"))
	if h.state.refreshOpen || h.state.refreshing {
		t.Fatalf("retry: open %v refreshing %v", h.state.refreshOpen, h.state.refreshing)
	}
	h.expect(t, locales.SettingsMetadataToastRefreshSuccess("0", "0", "0", "0"))

	// 完成：关闭弹窗。
	h.state.refreshOpen = true
	h.state.progress = refreshProgress{status: "done", current: 1, total: 1, updated: 1}
	h.frames(2)
	h.expect(t, locales.SettingsMetadataProgressDoneTitle())
	h.expectNot(t, locales.SettingsMetadataProgressRetryFailed("0"))
	h.click(t, locales.CommonComplete())
	if h.state.refreshOpen {
		t.Fatal("complete did not close")
	}
}

func fakePreview() *service.GameIDEnrichmentPreview {
	return &service.GameIDEnrichmentPreview{
		ScannedGames: 3, EnrichableGames: 1, UnchangedGames: 2, AddedSources: 2,
		Items: []service.GameIDEnrichmentPreviewItem{
			{
				GameID: "g1", GameName: "千恋＊万花", DefaultSource: enums.Bangumi, DefaultSourceID: "160043",
				ExistingSources: []service.GameIDEnrichmentSource{{SourceType: enums.Bangumi, SourceID: "160043"}},
				AddedSources:    []service.GameIDEnrichmentSource{{SourceType: enums.VNDB, SourceID: "v19073"}, {SourceType: enums.Steam, SourceID: "1144400"}},
				CanEnrich:       true, Reason: "can_enrich",
			},
			{
				GameID: "g2", GameName: "サノバウィッチ", DefaultSource: enums.VNDB, DefaultSourceID: "v16044",
				ExistingSources: []service.GameIDEnrichmentSource{{SourceType: enums.VNDB, SourceID: "v16044"}},
				Reason:          "no_mapping",
			},
			{
				GameID: "g3", GameName: "RIDDLE JOKER", DefaultSource: enums.Steam, DefaultSourceID: "1277930",
				ExistingSources: []service.GameIDEnrichmentSource{{SourceType: enums.Steam, SourceID: "1277930"}},
				Reason:          "already_complete",
			},
		},
	}
}

func TestEnrichmentPreviewModal(t *testing.T) {
	h := newHarness(t)
	h.state.previewOpen, h.state.previewLoading = true, true
	h.frames(2)
	h.expect(t, locales.SettingsMetadataIdPreviewTitle(), locales.SettingsMetadataIdPreviewLoadingText(), locales.SettingsMetadataIdPreviewLoading())

	h.state.previewLoading, h.state.preview = false, fakePreview()
	h.frames(2)
	h.expect(t, locales.SettingsMetadataIdPreviewEnrichableGames(), locales.SettingsMetadataIdPreviewUnchangedGames(),
		locales.SettingsMetadataIdPreviewColumnsGame(), locales.SettingsMetadataIdPreviewColumnsExisting(), locales.SettingsMetadataIdPreviewColumnsResult(),
		"千恋＊万花", "Bangumi 160043", "v19073", "1144400", "サノバウィッチ", "VNDB v16044",
		locales.SettingsMetadataIdPreviewReasonsNoMapping(), locales.SettingsMetadataIdPreviewReasonsAlreadyComplete(),
		locales.SettingsMetadataIdPreviewScanned("3"), locales.SettingsMetadataIdPreviewConfirm("1"))
	// 三列按 30% / 34% / 36% 分宽度。
	g, e, r := h.find(t, locales.SettingsMetadataIdPreviewColumnsGame()), h.find(t, locales.SettingsMetadataIdPreviewColumnsExisting()),
		h.find(t, locales.SettingsMetadataIdPreviewColumnsResult())
	if a, b := e.X-g.X, r.X-e.X; a <= 0 || b <= a {
		t.Errorf("column widths %v, %v", a, b)
	}

	// 补全：库里没有候选游戏，没有新增来源；关闭弹窗并提示。
	h.click(t, locales.SettingsMetadataIdPreviewConfirm("1"))
	if h.state.previewOpen || h.state.preview != nil || h.state.enriching {
		t.Fatalf("after confirm: open %v enriching %v", h.state.previewOpen, h.state.enriching)
	}
	h.expect(t, locales.SettingsMetadataToastIdEnrichmentNoChanges("0", "0"))

	// 应用中不能关闭。
	h.state.previewOpen, h.state.preview, h.state.enriching = true, fakePreview(), true
	h.frames(2)
	h.expect(t, locales.SettingsMetadataIdEnriching())
	h.click(t, locales.CommonClose())
	if !h.state.previewOpen {
		t.Fatal("closed while applying")
	}
	h.state.enriching = false
	h.frames(2)
	h.click(t, locales.CommonClose())
	if h.state.previewOpen || h.state.preview != nil {
		t.Fatal("close did not close")
	}
}

func TestEnrichmentPreviewFromService(t *testing.T) {
	h := newHarness(t)
	// 一个 Bangumi 游戏：映射库里没有这个 ID（no_mapping），没有可补全的游戏，按钮禁用。
	hosttest.AddGame(t, h.svc, models.Game{ID: "bgm-1", Name: "测试游戏", SourceType: enums.Bangumi, SourceID: "999999999",
		MetadataSources: []models.GameMetadataSource{{SourceType: enums.Bangumi, SourceID: "999999999"}}})
	h.click(t, locales.SettingsMetadataIdEnrichmentButton())
	if !h.state.previewOpen || h.state.previewLoading || h.state.preview == nil {
		t.Fatalf("preview: open %v loading %v preview %v", h.state.previewOpen, h.state.previewLoading, h.state.preview)
	}
	h.expect(t, "测试游戏", "Bangumi 999999999", locales.SettingsMetadataIdPreviewScanned("1"), locales.SettingsMetadataIdPreviewConfirm("0"))
	h.click(t, locales.SettingsMetadataIdPreviewConfirm("0"))
	if h.state.enriching || !h.state.previewOpen {
		t.Fatal("confirm enabled without enrichable games")
	}
}

func TestDownloadRemoteCovers(t *testing.T) {
	h := newHarness(t)
	h.click(t, locales.SettingsMetadataDownloadCoverButton())
	if h.state.queueingCover {
		t.Fatal("still queueing")
	}
	h.expect(t, locales.SettingsMetadataToastNoRemoteCovers())
}

// DESKTOP_MYGO_PNG=<目录> 时把面板与弹窗存成图片（深色，150%），用于目测。
func TestScreenshotDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	h := newHarness(t)
	save := func(name string) {
		t.Helper()
		h.frames(2)
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	save("metadata-panel.png")
	h.state.fieldDraft, h.state.fieldsOpen = slices.Clone(enums.DefaultMetadataUpdateFields[:5]), true
	save("metadata-fields.png")
	h.state.fieldsOpen = false
	h.state.refreshOpen, h.state.refreshing = true, true
	h.state.progress = refreshProgress{status: "running", current: 3, total: 10, gameName: "千恋＊万花", updated: 2, failed: 1}
	save("metadata-progress.png")
	h.state.refreshing = false
	h.state.progress.failedGameIDs = []string{"g1"}
	save("metadata-progress-done.png")
	h.state.refreshOpen = false
	h.state.previewOpen, h.state.preview = true, fakePreview()
	save("metadata-preview.png")
}
