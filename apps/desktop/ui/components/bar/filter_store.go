package bar

import (
	"slices"

	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/env"
)

// savedFilters 是跨重启保留的筛选，对应原版 FilterBar 与页面存进 localStorage 的 <storageKey>_*：
// 搜索词、主次排序、状态与元数据来源筛选（含反选）。标签筛选原版也不保存。
type savedFilters struct {
	Search             string           `json:"searchQuery,omitempty"`
	SortBy             string           `json:"sortBy,omitempty"`
	SortOrder          enums.SortOrder  `json:"sortOrder,omitempty"`
	SecondarySortBy    string           `json:"secondarySortBy,omitempty"`
	SecondarySortOrder enums.SortOrder  `json:"secondarySortOrder,omitempty"`
	Status             enums.GameStatus `json:"statusFilter,omitempty"`
	StatusInverted     bool             `json:"statusFilterInverted,omitempty"`
	Source             enums.SourceType `json:"metadataSourceFilter,omitempty"`
	SourceInverted     bool             `json:"metadataSourceFilterInverted,omitempty"`
}

// filterStore 记录 Filters 保存在哪里，以及上次保存的值（变化时才写）。
type filterStore struct {
	storage *env.Storage
	key     string
	saved   savedFilters
}

// Restore 对应原版初始化时从 localStorage 恢复：读取 key（library、category、categories）下上次保存的筛选，
// 排序字段不在 sortOptions 里、状态或来源不是可选值时保留当前（默认）值。之后 FilterBar 在筛选变化时保存。
// 在页面创建时、第一次查询之前调用。
func (f *Filters) Restore(s *env.Storage, key string, sortOptions []better.SelectOption) {
	f.store = filterStore{storage: s, key: key}
	var v savedFilters
	if s.Get(key, &v) {
		sortBy := make([]string, len(sortOptions))
		for i, o := range sortOptions {
			sortBy[i] = o.Value
		}
		f.apply(v, sortBy)
	}
	f.store.saved = f.snapshot()
}

func (f *Filters) apply(v savedFilters, sortBy []string) {
	f.Search, f.Debounced = v.Search, v.Search
	if slices.Contains(sortBy, v.SortBy) {
		f.SortBy = v.SortBy
	}
	if validOrder(v.SortOrder) {
		f.SortOrder = v.SortOrder
	}
	if v.SecondarySortBy != f.SortBy && slices.Contains(sortBy, v.SecondarySortBy) {
		f.SecondarySortBy = v.SecondarySortBy
	}
	if validOrder(v.SecondarySortOrder) {
		f.SecondarySortOrder = v.SecondarySortOrder
	}
	if v.Status != "" && validStatus(v.Status) {
		f.Status, f.StatusInverted = v.Status, v.StatusInverted
	}
	if v.Source != "" && validSource(v.Source) {
		f.Source, f.SourceInverted = v.Source, v.SourceInverted
	}
}

func validStatus(s enums.GameStatus) bool {
	for _, o := range StatusOptions {
		if o.Value == s {
			return true
		}
	}
	return false
}

func validSource(s enums.SourceType) bool {
	for _, o := range SourceOptions {
		if o.Value == s {
			return true
		}
	}
	return false
}

func validOrder(o enums.SortOrder) bool { return o == enums.SortOrderAsc || o == enums.SortOrderDesc }

// snapshot 是当前要保存的筛选。
func (f *Filters) snapshot() savedFilters {
	v := savedFilters{
		Search:    f.Search,
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
		Status:    f.Status,
		Source:    f.Source,
	}
	if f.SecondarySortBy != "" {
		v.SecondarySortBy, v.SecondarySortOrder = f.SecondarySortBy, f.SecondarySortOrder
	}
	v.StatusInverted = f.Status != "" && f.StatusInverted
	v.SourceInverted = f.Source != "" && f.SourceInverted
	return v
}

// persist 在筛选与上次保存的不同时保存它们；没有调用过 Restore 时什么也不做。FilterBar 每次构建时调用。
func (f *Filters) persist() {
	if f.store.key == "" {
		return
	}
	if v := f.snapshot(); v != f.store.saved {
		f.store.saved = v
		f.store.storage.Set(f.store.key, v)
	}
}
