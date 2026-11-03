package game

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components/modal"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// openCategories 对应 openCategoryModal：读取全部收藏夹与游戏所在的收藏夹后打开。
func (p *Page) openCategories(c *ui.Context) {
	id, gen := p.id, p.gen
	p.env.Async(func() func() {
		all, err := p.env.Svc.Category.GetCategories()
		var mine []vo.CategoryVO
		if err == nil {
			mine, err = p.env.Svc.Category.GetCategoriesByGame(id)
		}
		return func() {
			if gen != p.gen {
				return
			}
			if err != nil {
				p.env.Toast(c, env.Error(locales.GameToastLoadFavFailed()))
				return
			}
			ids := make([]string, 0, len(mine))
			for _, cat := range mine {
				ids = append(ids, cat.ID)
			}
			p.category.Show(all, ids)
			p.categoryInitial = ids
		}
	})
}

// saveCategories 对应 handleSaveCategories：逐个加入、移出，再刷新数量。
func (p *Page) saveCategories(c *ui.Context, selected []string) {
	id, initial := p.id, p.categoryInitial
	var add, remove []string
	for _, s := range selected {
		if !slices.Contains(initial, s) {
			add = append(add, s)
		}
	}
	for _, s := range initial {
		if !slices.Contains(selected, s) {
			remove = append(remove, s)
		}
	}
	p.env.Async(func() func() {
		var err error
		for _, cat := range add {
			if err = p.env.Svc.Category.AddGameToCategory(id, cat); err != nil {
				break
			}
		}
		for _, cat := range remove {
			if err != nil {
				break
			}
			err = p.env.Svc.Category.RemoveGameFromCategory(id, cat)
		}
		return func() {
			if err != nil {
				p.env.Toast(c, env.Error(locales.GameToastUpdateFavFailed()))
				return
			}
			if len(add)+len(remove) > 0 {
				p.env.Toast(c, env.Success(locales.GameToastFavUpdated()))
			}
		}
	})
}

// categoryView 是 AddToCategoryModal（components/modal）。
func (p *Page) categoryView(c *ui.Context, t *theme.Theme) {
	p.category.View(c, t, modal.AddToCategoryProps{Env: p.env, OnSave: func(selected []string) { p.saveCategories(c, selected) }})
}
