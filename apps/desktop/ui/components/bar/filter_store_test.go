package bar

import (
	"testing"

	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/env"
)

func gameFilters() Filters {
	return Filters{
		SortBy:             string(enums.GameListSortByCreatedAt),
		SortOrder:          enums.SortOrderDesc,
		SecondarySortOrder: enums.SortOrderAsc,
	}
}

// 搜索、排序、状态与来源筛选在变化时保存，下次创建页面（重启）时恢复；标签不保存（同原版 localStorage）。
func TestFiltersRestorePersisted(t *testing.T) {
	s := env.NewStorage("")
	f := gameFilters()
	f.Restore(s, "library", GameSortOptions())
	f.Search = "fate"
	f.SortBy, f.SortOrder = string(enums.GameListSortByName), enums.SortOrderAsc
	f.SecondarySortBy, f.SecondarySortOrder = string(enums.GameListSortByRating), enums.SortOrderDesc
	f.Status, f.StatusInverted = enums.StatusPlaying, true
	f.Source = enums.VNDB
	f.Tags = []string{"ADV"}
	f.persist()

	g := gameFilters()
	g.Restore(s, "library", GameSortOptions())
	if g.Search != "fate" || g.Debounced != "fate" {
		t.Errorf("search = %q / %q", g.Search, g.Debounced)
	}
	if g.SortBy != string(enums.GameListSortByName) || g.SortOrder != enums.SortOrderAsc {
		t.Errorf("sort = %s %s", g.SortBy, g.SortOrder)
	}
	if g.SecondarySortBy != string(enums.GameListSortByRating) || g.SecondarySortOrder != enums.SortOrderDesc {
		t.Errorf("secondary sort = %s %s", g.SecondarySortBy, g.SecondarySortOrder)
	}
	if g.Status != enums.StatusPlaying || !g.StatusInverted || g.Source != enums.VNDB || g.SourceInverted {
		t.Errorf("status = %s %v, source = %s %v", g.Status, g.StatusInverted, g.Source, g.SourceInverted)
	}
	if len(g.Tags) != 0 {
		t.Errorf("tags restored: %q", g.Tags)
	}

	// 其他页面的键互不影响
	other := gameFilters()
	other.Restore(s, "category", GameSortOptions())
	if other.Search != "" || other.SortBy != string(enums.GameListSortByCreatedAt) {
		t.Errorf("category got library filters: %+v", other.snapshot())
	}
}

// 保存的值不是当前可选项时保留默认值。
func TestFiltersRestoreIgnoresInvalid(t *testing.T) {
	s := env.NewStorage("")
	s.Set("library", savedFilters{
		SortBy: "bogus", SortOrder: "sideways",
		SecondarySortBy: string(enums.GameListSortByCreatedAt), // 与主排序相同
		Status:          "nope", StatusInverted: true,
		Source: "nowhere",
	})
	f := gameFilters()
	f.Restore(s, "library", GameSortOptions())
	def := gameFilters()
	if want := def.snapshot(); f.snapshot() != want {
		t.Errorf("restored %+v, want defaults %+v", f.snapshot(), want)
	}
}
