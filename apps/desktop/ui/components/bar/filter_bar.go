package bar

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/tagname"
	"lunabox/desktop-mygo/ui/theme"
)

// SearchDebounce 是搜索框的防抖（useDebouncedValue(searchQuery, 250)）。
const SearchDebounce = 250 * time.Millisecond

// Filters 是 FilterBar 的状态：搜索、排序、游戏筛选（标签、状态、元数据来源）、筛选抽屉与批量模式。
// 页面持有它，路由切走再回来时保持不变。
type Filters struct {
	Search         string
	Debounced      string // 防抖后的搜索词，查询用它
	SearchEditedAt time.Time

	SortBy             string
	SortOrder          enums.SortOrder
	SecondarySortBy    string
	SecondarySortOrder enums.SortOrder

	Status         enums.GameStatus
	StatusInverted bool
	Source         enums.SourceType
	SourceInverted bool
	Tags           []string
	TagsInverted   bool
	TagInput       string
	TagSuggestions []string
	ShowSortField  bool // 封面展示排序字段

	Presets    []models.GameFilterPreset
	presetUI   presetMenu
	DrawerOpen bool
	Batch      bool
}

// Debounce 在搜索词停止变化 SearchDebounce 后提交它并调用 onChange；在页面构建开始时调用。
func (f *Filters) Debounce(c *ui.Context, onChange func()) {
	if f.Search == f.Debounced {
		return
	}
	if wait := SearchDebounce - c.Now().Sub(f.SearchEditedAt); wait > 0 {
		c.After(wait)
		return
	}
	f.Debounced = f.Search
	onChange()
}

// Settling 报告搜索词还在防抖中（isSearchSettling）。
func (f *Filters) Settling() bool { return f.Search != f.Debounced }

// GameListRequest 组装 GetGames / GetCategoryGames 的查询条件（不含分页）。
func (f *Filters) GameListRequest() vo.GameListRequest {
	req := vo.GameListRequest{
		SearchQuery: strings.TrimSpace(f.Debounced),
		Tags:        f.Tags,
		ExcludeTags: f.TagsInverted && len(f.Tags) > 0,
		SortBy:      enums.GameListSortBy(f.SortBy),
		SortOrder:   f.SortOrder,
	}
	if f.Status != "" {
		s := f.Status
		req.Status, req.ExcludeStatus = &s, f.StatusInverted
	}
	if f.Source != "" {
		s := f.Source
		req.MetadataSource, req.ExcludeMetadataSource = &s, f.SourceInverted
	}
	if f.SecondarySortBy != "" {
		req.SecondarySortBy, req.SecondarySortOrder = enums.GameListSortBy(f.SecondarySortBy), f.SecondarySortOrder
	}
	return req
}

// Key 是查询条件的标识（queryKey），条件不变时请求结果可以复用。
func (f *Filters) Key() string {
	r := f.GameListRequest()
	return fmt.Sprintf("%s|%v|%v|%s|%s|%s|%s|%v|%v|%v|%v", r.SearchQuery, r.Tags, r.ExcludeTags, r.SortBy, r.SortOrder,
		r.SecondarySortBy, r.SecondarySortOrder, deref(r.Status), r.ExcludeStatus, deref(r.MetadataSource), r.ExcludeMetadataSource)
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// HasActiveFilters 对应 hasActiveGameFilters：有搜索词或任一筛选时，空列表显示“没有匹配的游戏”。
func (f *Filters) HasActiveFilters() bool {
	return strings.TrimSpace(f.Debounced) != "" || len(f.Tags) > 0 || f.Status != "" || f.Source != ""
}

// ActiveFilterCount 是“筛选”按钮上的数量（activeFilterCount）。
func (f *Filters) ActiveFilterCount() int {
	n := 0
	if f.Status != "" {
		n++
	}
	if f.Source != "" {
		n++
	}
	if len(f.Tags) > 0 || f.TagInput != "" {
		n++
	}
	return n
}

// CountText 是标题下的游戏数量：按状态筛选时带上状态（category.filteredGameCount 等）。
func (f *Filters) CountText(total int) string {
	count := strconv.Itoa(total)
	switch {
	case f.Status != "" && f.StatusInverted:
		return locales.CategoryExcludedStatusGameCount(count, StatusLabel(f.Status))
	case f.Status != "":
		return locales.CategoryFilteredGameCount(count, StatusLabel(f.Status))
	}
	return locales.CategoryGameCount(count)
}

// LoadPresets 读取已保存的筛选预设（GameFilterPresetMenu 打开时的 ListGameFilterPresets）。
func (f *Filters) LoadPresets(e *env.Env) {
	e.Async(func() func() {
		presets, err := e.Backend.ListGameFilterPresets()
		return func() {
			f.presetUI.loaded = true
			if err != nil {
				log.Printf("filter presets: list: %v", err) // 原版提示 filterPresets.loadFailed；这里在 Reload 里读，没有 ui.Context
				return
			}
			f.Presets = presets
		}
	})
}

// ApplyPreset 对应 applyFilterPreset：标签、状态、来源与排序换成预设里的。
func (f *Filters) ApplyPreset(preset models.GameFilterPreset) {
	f.Tags, f.TagsInverted = preset.Tags, len(preset.Tags) > 0 && preset.ExcludeTags
	f.Status, f.StatusInverted = preset.Status, preset.Status != "" && preset.ExcludeStatus
	f.Source, f.SourceInverted = preset.MetadataSource, false
	f.TagInput, f.TagSuggestions = "", nil
	if preset.SortBy != "" {
		f.SortBy, f.SortOrder = string(preset.SortBy), preset.SortOrder
	}
	f.SecondarySortBy, f.SecondarySortOrder = string(preset.SecondarySortBy), preset.SecondarySortOrder
	if f.SecondarySortOrder == "" {
		f.SecondarySortOrder = enums.SortOrderAsc
	}
}

// SearchFor 直接按名称搜索（以 search.searchQuery 进入页面），不等防抖。
func (f *Filters) SearchFor(query string) { f.Search, f.Debounced = query, query }

// FilterByTag 只按这个标签筛选（以 search.tagFilter 进入页面）。
func (f *Filters) FilterByTag(tag string) {
	f.Tags, f.TagsInverted, f.TagInput, f.TagSuggestions = []string{tag}, false, "", nil
}

func (f *Filters) clear(p FilterBarProps) {
	f.Search, f.Debounced = "", ""
	f.Status, f.StatusInverted = "", false
	f.Source, f.SourceInverted = "", false
	f.Tags, f.TagsInverted, f.TagInput, f.TagSuggestions = nil, false, "", nil
	f.SortBy, f.SortOrder = p.DefaultSortBy, p.DefaultSortOrder
	f.SecondarySortBy, f.SecondarySortOrder = "", enums.SortOrderAsc
}

func (f *Filters) canClear(p FilterBarProps) bool {
	return f.Search != "" || f.ActiveFilterCount() > 0 || f.SortBy != p.DefaultSortBy || f.SortOrder != p.DefaultSortOrder ||
		p.SecondarySort && f.SecondarySortBy != ""
}

func (f *Filters) addTag(tag string) {
	if !contains(f.Tags, tag) {
		f.Tags = append(f.Tags, tag)
	}
	f.TagInput, f.TagSuggestions = "", nil
}

func (f *Filters) removeTag(tag string) {
	out := f.Tags[:0:0]
	for _, t := range f.Tags {
		if t != tag {
			out = append(out, t)
		}
	}
	f.Tags = out
	if len(out) == 0 {
		f.TagsInverted = false
	}
}

// searchTags 对应 useTagGameFilter 的标签建议：库里名称匹配的标签，加上译名匹配、库里也有的标签
// （enable_tag_translation 时），按原名或显示名过滤，去掉已选的，最多 50 个。出错时没有建议。
func (f *Filters) searchTags(e *env.Env, query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		f.TagSuggestions = nil
		return
	}
	translate := tagname.Enabled(e.Config())
	var translated []string
	if translate {
		translated = tagname.FindRaw(q) // 按界面语言，在主线程取
	}
	e.Async(func() func() {
		names, err := e.Backend.SearchTagsInLibrary(q)
		var existing []string
		if err == nil && len(translated) > 0 {
			existing, err = e.Backend.FilterExistingTagNames(translated)
		}
		return func() {
			if query != f.TagInput {
				return
			}
			if err != nil {
				f.TagSuggestions = nil
				return
			}
			var merged []string
			for _, n := range append(names, existing...) {
				if !contains(merged, n) {
					merged = append(merged, n)
				}
			}
			var out []string
			for _, n := range tagname.FilterByDisplay(merged, q, translate) {
				if !contains(f.Tags, n) {
					out = append(out, n)
				}
			}
			f.TagSuggestions = out[:min(len(out), 50)]
		}
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// RandomGame 是“随机游戏”按钮（onRandomGame）。
type RandomGame struct {
	Disabled, Loading bool
	OnClick           func()
}

// FilterBarProps 对应 FilterBar.tsx 的属性。
type FilterBarProps struct {
	Env               *env.Env
	Filters           *Filters
	SearchPlaceholder string
	SortOptions       []better.SelectOption
	DefaultSortBy     string
	DefaultSortOrder  enums.SortOrder
	SecondarySort     bool // supportsSecondarySort：次排序字段与方向
	GameFilters       bool // 游戏列表：标签、状态、元数据来源、筛选预设与“封面展示排序字段”
	OnChange          func()

	Random           *RandomGame // nil 时不显示
	OnBatchChange    func(on bool)
	SelectedCount    int
	OnSelectAll      func()
	OnClearSelection func()
	BatchActions     func() // 批量模式第二行右侧的按钮
	ActionButton     func() // 右侧的主按钮（actionButton）
}

func (p FilterBarProps) changed() {
	if p.OnChange != nil {
		p.OnChange()
	}
}

// FilterBar 对应 components/bar/FilterBar.tsx（flex flex-wrap items-center justify-between gap-4 my-4）：
// 搜索框，右侧随机游戏、批量模式、筛选按钮与 actionButton；批量模式下第二行是批量操作。
// 筛选抽屉由 FilterDrawer 在页面最后构建。
func FilterBar(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	ui.Column(c.Key("filterbar")).Margin(16, 0).Gap(16).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(16).Children(func() {
			searchInput(c, t, p)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
				if p.Random != nil {
					randomButton(c, t, *p.Random)
				}
				if p.OnBatchChange != nil {
					batchToggle(c, t, f.Batch, p.OnBatchChange)
				}
				filterButton(c, t, f)
				if p.ActionButton != nil {
					p.ActionButton()
				}
			})
		})
		if f.Batch {
			batchBar(c, t, p)
		}
	})
}

// searchInput：relative flex-1 max-w-md，input p-2 pl-10 pr-10 text-sm rounded-lg border。
func searchInput(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	placeholder := p.SearchPlaceholder
	if placeholder == "" {
		placeholder = locales.CommonSearch() + "..."
	}
	bg, border := t.GlassInput(t.InputBg, t.InputBorder) // glass-input
	box := ui.Row(c.Key("search")).Grow(1).MaxWidth(448).AlignItems(ui.Center).Radius(8).
		Background(bg).Border(t.HL, border)
	box.Children(func() {
		ui.Box(c).Absolute().Top(0).Bottom(0).Left(12).Center().PassThrough().Children(func() {
			components.Icon(c, "magnify", 18, t.InputIcon)
		})
		input := ui.TextInputBase(c.Key("search-input"), &f.Search).Grow(1).Padding(8, 40).
			FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
			Placeholder(placeholder).Label(placeholder)
		if input.Changed() {
			f.SearchEditedAt = c.Now()
		}
		if input.Focused() {
			box.BorderColor(t.InputFocus)
		}
		if f.Search != "" {
			clear := ui.ButtonBase(c.Key("clear")).Absolute().Top(0).Bottom(0).Right(6).Width(32).Center().
				Radius(6).FocusRing(false).Label(locales.FilterBarClearSearch())
			clear.OnClick(func() {
				f.Search, f.Debounced = "", ""
				p.changed()
			}).Children(func() {
				components.Icon(c, "close-circle-outline", 18, t.Pick(theme.Brand500, theme.Brand400))
			})
		}
	})
}

func randomButton(c *ui.Context, t *theme.Theme, r RandomGame) {
	btn := better.ToolButton(c, t, "random", false).Label(locales.FilterBarRandomGame())
	if r.Disabled || r.Loading {
		components.Disable(btn, 0.5)
	}
	btn.OnClick(r.OnClick).Children(func() {
		color := better.ToolButtonColor(t, false)
		if r.Loading {
			components.Spinner(c, 18, color)
		} else {
			components.Icon(c, "dice-multiple-outline", 18, color)
		}
	})
}

func batchToggle(c *ui.Context, t *theme.Theme, on bool, onChange func(bool)) {
	btn := better.ToolButton(c, t, "batch", on).Label(locales.NativeBatchSelect())
	icon := "checkbox-multiple-marked-outline"
	if on {
		icon = "close-circle-outline"
	}
	btn.OnClick(func() { onChange(!on) }).Children(func() {
		components.Icon(c, icon, 18, better.ToolButtonColor(t, on))
	})
}

func filterButton(c *ui.Context, t *theme.Theme, f *Filters) {
	count := f.ActiveFilterCount()
	btn := better.ToolButton(c, t, "filters", false).Label(locales.FilterBarFilters() + " (" + strconv.Itoa(count) + ")")
	color := t.Pick(theme.Brand700, theme.Brand300)
	btn.OnClick(func() { f.DrawerOpen = true }).Children(func() {
		ui.Box(c).Size(20, 20).Center().Shrink(0).Children(func() {
			components.Icon(c, "filter-variant", 18, color)
			if count > 0 {
				ui.Box(c).Absolute().Top(0).Right(0).Size(8, 8).Radius(999).Background(theme.Primary500).
					Shadow(0, 0, 0, 2, t.Pick(theme.Brand150, theme.Brand700))
			}
		})
		components.Text(c, locales.FilterBarFilters(), theme.TextSM, theme.WeightNormal, color).NoWrap()
	})
}

// batchBar 是批量模式下的第二行：全选 / 清空 / 已选数量 / 帮助，右侧是页面的批量操作（batchActions）。
func batchBar(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	bar := ui.Row(c.Key("batch-bar")).Wrap().AlignItems(ui.Center).Gap(8).Padding(8, 12).Radius(8).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700.Alpha(0.5)))
	if t.Dark {
		bar.LinearGradient(ui.LinearGradient{From: theme.Brand800.Alpha(0.5), To: theme.Brand900.Alpha(0.3), Angle: 90})
	} else {
		bar.LinearGradient(ui.LinearGradient{From: theme.Brand50, To: theme.Brand50.Alpha(0.5), Angle: 90})
	}
	bar.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			if p.OnSelectAll != nil {
				batchPill(c, t, "select-all", locales.CommonSelectAll(), true).OnClick(p.OnSelectAll)
			}
			if p.OnClearSelection != nil {
				batchPill(c, t, "clear-selection", locales.CommonClearStore(), true).OnClick(p.OnClearSelection)
			}
		})
		batchPill(c, t, "selected-count", "", false).Children(func() {
			components.Text(c, locales.CommonSelected(), theme.TextXS, theme.WeightMedium, t.Text)
			components.TextMargin(c, fmt.Sprint(p.SelectedCount), theme.TextXS, theme.WeightSemibold, t.Text, 0, 0, 0, 4)
		})
		help := ui.ButtonBase(c.Key("batch-help")).Size(28, 28).Radius(999).Center().FocusRing(false).Label(locales.CommonBatchSelectionHelp())
		help.Children(func() { components.Icon(c, "help-circle-outline", 18, t.TextMuted) })
		better.Tooltip(c, t, help, locales.CommonBatchSelectionHelp())

		if p.BatchActions != nil {
			ui.Row(c).Gap(6).Margin(0, 0, 0, ui.Auto).Children(p.BatchActions)
		}
	})
}

func batchPill(c *ui.Context, t *theme.Theme, key, label string, clickable bool) ui.Element {
	pill := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(6).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand600)).Background(t.Pick(theme.White, theme.Brand700.Alpha(0.6))).
		AlignItems(ui.Center).FocusRing(false)
	if clickable && pill.Hovered() {
		pill.Background(t.Pick(theme.Brand50, theme.Brand700))
	}
	if label != "" {
		pill.Children(func() { components.Text(c, label, theme.TextXS, theme.WeightMedium, t.NavText) })
	}
	return pill
}
