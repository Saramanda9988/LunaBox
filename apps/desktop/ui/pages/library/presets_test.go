package library

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/ui/locales"
)

// fakePresets 记录新建与删除的预设。
type fakePresets struct {
	created []vo.SaveGameFilterPresetRequest
	deleted []string
	err     error
}

func (f *fakePresets) CreateGameFilterPreset(req vo.SaveGameFilterPresetRequest) (models.GameFilterPreset, error) {
	if f.err != nil {
		return models.GameFilterPreset{}, f.err
	}
	f.created = append(f.created, req)
	return models.GameFilterPreset{
		ID: "new", Name: req.Name, Tags: req.Tags, Status: req.Status, ExcludeStatus: req.ExcludeStatus,
		SortBy: req.SortBy, SortOrder: req.SortOrder,
	}, nil
}

func (f *fakePresets) DeleteGameFilterPreset(id string) error {
	f.deleted = append(f.deleted, id)
	return f.err
}

func openDrawer(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("筛选 (0)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
}

func TestSaveFilterPreset(t *testing.T) {
	l, b, tt := newTestPage(t, 3)
	fp := &fakePresets{}
	l.env.Svc = &api.Services{Config: &b.Cfg, Presets: fp}
	l.filters.Status, l.filters.StatusInverted = enums.StatusCompleted, true
	openDrawer(t, tt)

	if !tt.HasText(locales.FilterPresetsEmpty()) {
		t.Fatalf("empty preset hint missing: %q", tt.Texts())
	}
	if err := tt.Click(locales.FilterPresetsSaveCurrent()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	summary := locales.FilterPresetsExcludeStatusSummary(locales.CommonCompleted()) + " · " +
		locales.FilterPresetsSortSummary(locales.CommonCreatedAt(), locales.FilterBarSortDesc())
	if !tt.HasText(summary) {
		t.Fatalf("form summary %q missing in %q", summary, tt.Texts())
	}

	// 没有名称时不保存（名称框自动聚焦，Enter 与“创建”按钮一样提交）
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(fp.created) != 0 || !slices.Contains(tt.Announcements(), locales.FilterPresetsNameRequired()) {
		t.Fatalf("saved without a name: %v, %q", fp.created, tt.Announcements())
	}

	tt.Type("通关以外")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(fp.created) != 1 {
		t.Fatalf("created %v", fp.created)
	}
	got := fp.created[0]
	if got.Name != "通关以外" || got.Status != enums.StatusCompleted || !got.ExcludeStatus ||
		got.SortBy != enums.GameListSortByCreatedAt || got.SortOrder != enums.SortOrderDesc || got.SecondarySortOrder != "" {
		t.Fatalf("request = %+v", got)
	}
	// 表单关闭：摘要只剩列表里的一份
	if n := countText(tt, summary); !tt.HasText("通关以外") || n != 1 {
		t.Fatalf("form not closed (%d summaries) or preset not listed: %q", n, tt.Texts())
	}
}

func TestApplyAndDeleteFilterPreset(t *testing.T) {
	b0 := []models.GameFilterPreset{{ID: "p1", Name: "想玩的", Status: enums.StatusWantToPlay}}
	l, b, tt := newTestPage(t, 3)
	b.Presets = b0
	l.Reload()
	fp := &fakePresets{}
	l.env.Svc = &api.Services{Config: &b.Cfg, Presets: fp}
	openDrawer(t, tt)

	if !tt.HasText(locales.FilterPresetsIncludeStatusSummary(locales.CommonWantToPlay())) {
		t.Fatalf("preset summary missing: %q", tt.Texts())
	}
	if err := tt.Click("想玩的"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if l.filters.Status != enums.StatusWantToPlay {
		t.Fatalf("preset not applied: status %q", l.filters.Status)
	}
	if req := b.Reqs[len(b.Reqs)-1]; req.Status == nil || *req.Status != enums.StatusWantToPlay {
		t.Fatalf("library not reloaded with the preset: %+v", req)
	}

	if err := tt.Click(locales.FilterPresetsDelete("想玩的")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !slices.Equal(fp.deleted, []string{"p1"}) || tt.HasText("想玩的") {
		t.Fatalf("deleted %v, texts %q", fp.deleted, tt.Texts())
	}
}

func TestSaveFilterPresetFailure(t *testing.T) {
	l, b, tt := newTestPage(t, 3)
	fp := &fakePresets{err: errors.New("db")}
	l.env.Svc = &api.Services{Config: &b.Cfg, Presets: fp}
	openDrawer(t, tt)
	_ = tt.Click(locales.FilterPresetsSaveCurrent())
	tt.Frame()
	tt.Type("x")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !slices.Contains(tt.Announcements(), locales.FilterPresetsCreateFailed()) || !tt.HasText(locales.FilterPresetsCreate()) {
		t.Fatalf("failure not reported or form closed: %q / %q", tt.Announcements(), tt.Texts())
	}
}

func countText(tt *ui.Tester, s string) int {
	n := 0
	for _, t := range tt.Texts() {
		if t == s {
			n++
		}
	}
	return n
}
