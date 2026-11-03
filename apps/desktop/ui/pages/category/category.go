// Package category 是收藏详情页，对应原版 routes/category.tsx（路由 /categories/{id}）：收藏夹里的游戏，
// 与游戏库相同的 FilterBar 和虚拟网格；重命名、更换图标、删除收藏夹，添加与移除游戏。
package category

import (
	"math/rand/v2"
	"strings"
	"time"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/bar"
	"lunabox/desktop/ui/components/grid"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
)

const (
	skeletonDelay     = 300 * time.Millisecond // 延迟显示骨架屏
	candidatePageSize = 80                     // CANDIDATE_PAGE_SIZE
)

// Page 是收藏详情页的状态。筛选与排序跨收藏夹保留（原版存进 localStorage 的 category_*），
// 其余每次进入页面（Open）时重置，与原版路由重新挂载组件一致。
type Page struct {
	env *env.Env

	id        string
	gen       int
	category  *vo.CategoryVO
	loading   bool // 收藏夹本身还在加载
	openedAt  time.Time
	filters   bar.Filters
	pages     *grid.Pages
	blurNSFW  bool
	landscape bool

	selected      map[string]bool
	drag          grid.DragSelection // 批量模式下按住右键拖过卡片选择（useDragSelection）
	randomLoading bool
	renameOpen    bool
	renameValue   string
	deleteOpen    bool
	addGames      addGamesModal
	scroll        components.ScrollBar
	geometry      grid.Geometry

	failed    func() string // 待显示的提示（显示时按当前语言取）
	succeeded func() string
}

// New 创建页面；数据在 Open 时加载。
func New(e *env.Env) *Page {
	p := &Page{
		env: e,
		filters: bar.Filters{
			SortBy:             string(enums.GameListSortByCreatedAt),
			SortOrder:          enums.SortOrderDesc,
			SecondarySortOrder: enums.SortOrderAsc,
		},
		pages:    grid.NewPages(e),
		selected: map[string]bool{},
	}
	if cfg := e.Config(); cfg != nil {
		p.filters.ShowSortField = cfg.ShowSortFieldOnCover
		p.blurNSFW = cfg.BlurNSFWGameCovers
		p.landscape = cfg.GameCardLayout == "landscape"
	}
	return p
}

// Open 进入某个收藏夹：清空状态，加载收藏夹与第一页游戏。
func (p *Page) Open(id string, now time.Time) {
	p.id = id
	p.gen++
	p.category, p.loading, p.openedAt = nil, true, now
	p.filters.Batch, p.filters.DrawerOpen = false, false
	clear(p.selected)
	p.randomLoading, p.renameOpen, p.renameValue, p.deleteOpen = false, false, "", false
	p.addGames = addGamesModal{}
	p.scroll = components.ScrollBar{}
	p.pages = grid.NewPages(p.env)
	p.pages.OnError = func() { p.failed = locales.CategoryToastLoadGamesFailed }
	p.loadCategory()
	p.reload()
}

// Title 是页面标题（窗口历史里显示）。
func (p *Page) Title() string {
	if p.category != nil {
		return p.category.Name
	}
	return locales.CategoriesTitle()
}

// Loading 报告收藏夹或第一页游戏是否还在加载（截图用）。
func (p *Page) Loading() bool { return p.id != "" && (p.loading || p.pages.Loading) }

// loadCategory 对应 loadCategory：GetCategoryByID（游戏数量随之更新）。
func (p *Page) loadCategory() {
	id, gen := p.id, p.gen
	p.env.Async(func() func() {
		cat, err := p.env.Svc.Category.GetCategoryByID(id)
		return func() {
			if gen != p.gen {
				return
			}
			p.loading = false
			if err != nil {
				p.failed = locales.CategoryToastLoadCategoryFailed
				return
			}
			p.category = &cat
		}
	})
}

// request 是 GetCategoryGames 的查询条件（不含分页）。
func (p *Page) request() vo.CategoryGameListRequest {
	return vo.CategoryGameListRequest{CategoryID: p.id, GameListRequest: p.filters.GameListRequest()}
}

// reload 在查询条件变化（或收藏夹里的游戏变化）后重新请求第一页。
func (p *Page) reload() {
	req := p.request()
	p.pages.Reload(p.id+"|"+p.filters.Key(), func(offset, limit int) ([]models.Game, int, error) {
		r := req
		r.Offset, r.Limit = offset, limit
		resp, err := p.env.Svc.Category.GetCategoryGames(r)
		return resp.Games, resp.Total, err
	})
	p.filters.LoadPresets(p.env)
}

// changed 对应 invalidateCategoryGameLists 加 loadCategory：游戏列表与数量都重新读取。
func (p *Page) changed() {
	p.reload()
	p.loadCategory()
}

// openRandomGame 对应 handleOpenRandomGame：在当前条件下随机取一个游戏并打开它的详情页。
func (p *Page) openRandomGame() {
	if p.randomLoading || p.pages.Total <= 0 {
		return
	}
	p.randomLoading = true
	req := p.request()
	req.Offset, req.Limit = rand.IntN(p.pages.Total), 1
	p.env.Async(func() func() {
		resp, err := p.env.Svc.Category.GetCategoryGames(req)
		return func() {
			p.randomLoading = false
			if err != nil || len(resp.Games) == 0 || resp.Games[0].ID == "" {
				p.failed = locales.LibraryToastRandomGameFailed
				return
			}
			p.env.Router.Push(env.GamePath(resp.Games[0].ID))
		}
	})
}

// rename 对应 handleUpdateCategory。
func (p *Page) rename() {
	if p.category == nil || strings.TrimSpace(p.renameValue) == "" {
		return
	}
	id, name, emoji, gen := p.category.ID, p.renameValue, p.category.Emoji, p.gen
	p.env.Async(func() func() {
		err := p.env.Svc.Category.UpdateCategory(id, name, emoji)
		return func() {
			if err != nil {
				p.failed = locales.CategoriesToastUpdateFailed
				return
			}
			if gen == p.gen && p.category != nil {
				p.category.Name = name
			}
			p.renameOpen, p.renameValue = false, ""
			p.succeeded = locales.CategoriesToastUpdateSuccess
		}
	})
}

// setEmoji 对应 handleUpdateCategoryEmoji（系统收藏夹不能改）。
func (p *Page) setEmoji(emoji string) {
	if p.category == nil || p.category.IsSystem {
		return
	}
	id, name, gen := p.category.ID, p.category.Name, p.gen
	p.env.Async(func() func() {
		err := p.env.Svc.Category.UpdateCategory(id, name, emoji)
		return func() {
			if err != nil {
				p.failed = locales.CategoriesToastIconUpdateFailed
				return
			}
			if gen == p.gen && p.category != nil {
				p.category.Emoji = emoji
			}
			p.succeeded = locales.CategoriesToastIconUpdated
		}
	})
}

// deleteCategory 对应 handleDeleteCategory：删除后回到收藏夹列表。
func (p *Page) deleteCategory() {
	if p.category == nil {
		return
	}
	id := p.category.ID
	p.env.Async(func() func() {
		err := p.env.Svc.Category.DeleteCategory(id)
		return func() {
			if err != nil {
				p.failed = locales.CategoriesToastDeleteFailed
				return
			}
			p.succeeded = locales.CategoriesToastDeleteSuccess
			p.env.Router.Push(env.PathCategories)
		}
	})
}

// removeGame 对应 handleRemoveGame（卡片右上角的移除按钮）。
func (p *Page) removeGame(gameID string) {
	if p.category == nil {
		return
	}
	id := p.category.ID
	p.env.Async(func() func() {
		err := p.env.Svc.Category.RemoveGameFromCategory(gameID, id)
		return func() {
			if err != nil {
				p.failed = locales.CategoryToastRemoveGameFailed
				return
			}
			p.changed()
		}
	})
}

// batchRemove 对应 handleBatchRemove。
func (p *Page) batchRemove() {
	if p.category == nil || len(p.selected) == 0 {
		return
	}
	id := p.category.ID
	ids := make([]string, 0, len(p.selected))
	for gid := range p.selected {
		ids = append(ids, gid)
	}
	p.env.Async(func() func() {
		err := p.env.Svc.Category.RemoveGamesFromCategory(ids, id)
		return func() {
			if err != nil {
				p.failed = locales.CategoryToastBatchRemoveFailed
				return
			}
			n := len(ids)
			p.succeeded = func() string { return locales.CategoryToastBatchRemoveSuccess(itoa(n)) }
			clear(p.selected)
			p.filters.Batch = false
			p.changed()
		}
	})
}

func (p *Page) toggleSelected(id string) {
	if p.selected[id] {
		delete(p.selected, id)
	} else {
		p.selected[id] = true
	}
}

// setSelected 对应 setGameSelection：拖动选择把经过的游戏设成同一个状态。
func (p *Page) setSelected(id string, selected bool) {
	if selected {
		p.selected[id] = true
	} else {
		delete(p.selected, id)
	}
}
