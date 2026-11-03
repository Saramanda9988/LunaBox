package library

import (
	"math/rand/v2"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/importer"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// filterProps 是 library.tsx 传给 FilterBar 的属性：游戏筛选、次排序、随机游戏、批量操作与“添加游戏”。
func (l *Page) filterProps(c *ui.Context, t *theme.Theme) bar.FilterBarProps {
	f := &l.filters
	return bar.FilterBarProps{
		Env:               l.env,
		Filters:           f,
		SearchPlaceholder: locales.LibrarySearchPlaceholder(),
		SortOptions:       bar.GameSortOptions(),
		DefaultSortBy:     string(enums.GameListSortByCreatedAt),
		DefaultSortOrder:  enums.SortOrderDesc,
		SecondarySort:     true,
		GameFilters:       true,
		OnChange:          l.Reload,
		Random: &bar.RandomGame{
			Disabled: l.pages.Loading || f.Settling() || l.pages.Total == 0,
			Loading:  l.randomLoading,
			OnClick:  func() { l.openRandomGame(c) },
		},
		OnBatchChange: func(on bool) {
			f.Batch = on
			if !on {
				clear(l.selected)
			}
		},
		SelectedCount: len(l.selected),
		OnSelectAll: func() {
			for _, g := range l.pages.Games {
				l.selected[g.ID] = true
			}
		},
		OnClearSelection: func() { clear(l.selected) },
		BatchActions:     func() { l.batchActions(c, t) },
		ActionButton:     func() { l.addGameButton(c, t) },
	}
}

// openRandomGame 随机取一个游戏并打开它的详情页。
func (l *Page) openRandomGame(c *ui.Context) {
	if l.pages.Total == 0 || l.randomLoading {
		return
	}
	l.randomLoading = true
	req := l.filters.GameListRequest()
	req.Offset, req.Limit = rand.IntN(l.pages.Total), 1
	l.env.Async(func() func() {
		resp, err := l.env.Backend.GetGames(req)
		return func() {
			l.randomLoading = false
			if err != nil || len(resp.Games) == 0 {
				l.env.Toast(c, env.Error(locales.LibraryToastRandomGameFailed()))
				return
			}
			l.env.Router.Push(env.GamePath(resp.Games[0].ID))
		}
	})
}

// addGameButton 是 library.tsx 的 actionButton：glass-btn-neutral bg-neutral-600 px-4 py-2 text-sm font-medium 的触发按钮
// 和 min-w-[220px] 的 BetterDropdownMenu。
func (l *Page) addGameButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("add-game")).Padding(8, 16).Radius(8).AlignItems(ui.Center).FocusRing(false).Label(locales.LibraryAddGame())
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: t.PrimaryBtn, Hover: t.PrimaryBtnHover, Text: theme.White})
	bg := g.Bg
	if btn.Hovered() {
		bg = g.Hover
	}
	if t.Glass {
		btn.Border(t.HL, g.Border)
	}
	btn.Background(bg).Children(func() {
		components.Icon(c, "plus", 18, g.Text).Margin(0, 8, 0, 0)
		components.Text(c, locales.LibraryAddGame(), theme.TextSM, theme.WeightMedium, g.Text).NoWrap()
		components.Icon(c, "chevron-down", 18, g.Text).Margin(0, 0, 0, 8)
	})

	manual, batch := locales.CommonManualAdd(), locales.LibraryBatchImport()
	items := []better.MenuItem{
		{Key: "manual", Label: manual, Description: locales.LibraryAddGameDesc1(), Icon: "gamepad-variant", IconColor: theme.Neutral500, OnClick: l.addGame.Open},
		{Key: "batch", Label: batch, Description: locales.LibraryBatchImportDesc(), Icon: "folder-multiple", IconColor: theme.Success500, OnClick: l.batchImport.Open},
	}
	for i, imp := range importOptions {
		it := better.MenuItem{Key: imp.key, Label: imp.label(), Description: imp.description(), DividerBefore: i == 0, OnClick: func() { l.gameImport.Open(importer.Source(imp.key)) }}
		if imp.icon != "" {
			it.Icon, it.IconColor = imp.icon, theme.Slate500
		} else {
			it.Image = assets.Importer(imp.key)
		}
		items = append(items, it)
	}
	better.DropdownMenu(c, t, btn, better.MenuProps{Open: &l.addMenuOpen, MinWidth: 220, Items: items})
}
