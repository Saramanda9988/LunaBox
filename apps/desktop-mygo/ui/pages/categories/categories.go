// Package categories 是收藏夹列表页面，对应原版 routes/categories.tsx：搜索、排序、批量删除收藏夹与新建收藏夹。
package categories

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/grid"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// 排序字段（CategoriesSortBy）。
const (
	sortByName      = "name"
	sortByGameCount = "game_count"
	sortByCreatedAt = "created_at"
	sortByUpdatedAt = "updated_at"
)

const skeletonDelay = 300 * time.Millisecond // 延迟显示骨架屏

// Page 是收藏夹列表页面的状态。搜索与排序跨页面保留（原版存进 localStorage），其余每次进入页面时重置。
type Page struct {
	env *env.Env

	filters    bar.Filters
	categories []vo.CategoryVO
	loading    bool
	openedAt   time.Time
	gen        int
	selected   map[string]bool
	drag       grid.DragSelection // 批量模式下按住右键拖过卡片选择（useDragSelection）

	addOpen     bool
	newName     string
	confirmOpen bool
	scroll      components.ScrollBar
	failed      func() string // 待显示的提示（显示时按当前语言取）
	succeeded   func() string
}

// New 创建页面；数据在 Open 时加载。
func New(e *env.Env) *Page {
	return &Page{
		env:      e,
		filters:  bar.Filters{SortBy: sortByName, SortOrder: enums.SortOrderAsc},
		selected: map[string]bool{},
	}
}

// Open 进入页面：与原版路由重新挂载组件一致，清空列表、退出批量模式并重新加载。
func (p *Page) Open(now time.Time) {
	p.categories, p.loading, p.openedAt = nil, true, now
	p.filters.Batch, p.filters.DrawerOpen = false, false
	clear(p.selected)
	p.addOpen, p.newName, p.confirmOpen = false, "", false
	p.scroll = components.ScrollBar{}
	p.load()
}

// Loading 报告列表是否还在加载（截图用）。
func (p *Page) Loading() bool { return p.loading }

// load 对应 loadCategories。
func (p *Page) load() {
	p.gen++
	gen := p.gen
	p.env.Async(func() func() {
		list, err := p.env.Svc.Category.GetCategories()
		return func() {
			if gen != p.gen {
				return
			}
			p.loading = false
			if err != nil {
				p.failed = locales.CategoriesToastLoadFailed
				return
			}
			p.categories = list
		}
	})
}

// visible 对应 filteredCategories：按名称搜索（不区分大小写），再按排序字段排序。
func (p *Page) visible() []vo.CategoryVO {
	q := strings.ToLower(p.filters.Search)
	var out []vo.CategoryVO
	for _, cat := range p.categories {
		if q == "" || strings.Contains(strings.ToLower(cat.Name), q) {
			out = append(out, cat)
		}
	}
	// localeCompare(undefined)：WebView 的默认语言，这里用应用的语言
	locale := i18n.Locale()
	if locale == "" {
		locale = "zh-CN"
	}
	desc := p.filters.SortOrder == enums.SortOrderDesc
	slices.SortStableFunc(out, func(a, b vo.CategoryVO) int {
		var n int
		switch p.filters.SortBy {
		case sortByName:
			n = localeCompare(locale, a.Name, b.Name)
		case sortByGameCount:
			n = a.GameCount - b.GameCount
		case sortByCreatedAt:
			n = a.CreatedAt.Compare(b.CreatedAt)
		case sortByUpdatedAt:
			n = a.UpdatedAt.Compare(b.UpdatedAt)
		}
		if desc {
			return -n
		}
		return n
	})
	return out
}

// add 对应 handleAddCategory。
func (p *Page) add() {
	name := p.newName
	if strings.TrimSpace(name) == "" {
		return
	}
	p.env.Async(func() func() {
		err := p.env.Svc.Category.AddCategory(name, "")
		return func() {
			if err != nil {
				p.failed = locales.CategoriesToastCreateFailed
				return
			}
			p.newName, p.addOpen = "", false
			p.succeeded = locales.CategoriesToastCreateSuccess
			p.load()
		}
	})
}

// batchDelete 对应 handleBatchDelete 的确认回调。
func (p *Page) batchDelete() {
	var ids []string
	for _, cat := range p.categories {
		if p.selected[cat.ID] {
			ids = append(ids, cat.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	p.env.Async(func() func() {
		err := p.env.Svc.Category.DeleteCategories(ids)
		return func() {
			if err != nil {
				p.failed = locales.CategoriesToastBatchDeleteFailed
				return
			}
			clear(p.selected)
			p.filters.Batch = false
			p.succeeded = locales.CategoriesToastBatchDeleteSuccess
			p.load()
		}
	})
}

// setSelected 对应 setCategorySelection：系统收藏夹不能选。
func (p *Page) setSelected(cat vo.CategoryVO, on bool) {
	if cat.IsSystem {
		return
	}
	if on {
		p.selected[cat.ID] = true
	} else {
		delete(p.selected, cat.ID)
	}
}

func (p *Page) selectedCount() int {
	n := 0
	for _, cat := range p.categories {
		if p.selected[cat.ID] {
			n++
		}
	}
	return n
}

// sortOptions 对应传给 FilterBar 的 sortOptions。
func sortOptions() []better.SelectOption {
	return []better.SelectOption{
		{Label: locales.CategoriesSortOptionsName(), Value: sortByName},
		{Label: locales.CategoriesSortOptionsGameCount(), Value: sortByGameCount},
		{Label: locales.CategoriesSortOptionsCreatedAt(), Value: sortByCreatedAt},
		{Label: locales.CategoriesSortOptionsUpdatedAt(), Value: sortByUpdatedAt},
	}
}

func batchDeleteMessage(count int) string {
	return locales.CategoriesToastBatchDeleteMsg(strconv.Itoa(count))
}
