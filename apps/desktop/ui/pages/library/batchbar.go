package library

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/bar"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/panel/launch"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// batchActions 是 library.tsx 传给 FilterBar 的 batchActions：导入到 Steam（只在 Windows 上）、设为状态、加入收藏夹、删除；
// 前面加了原版没有的“检查失效路径”。它们放在批量操作浮动岛（bar.BatchIsland）里，删除与其他操作用竖线隔开。
func (l *Page) batchActions(c *ui.Context, t *theme.Theme) {
	none := len(l.selected) == 0
	scanIcon := "folder-alert-outline"
	if l.missing.scanning {
		scanIcon = "loading"
	}
	bar.BatchIslandButton(c, t, "missing-paths", bar.BatchButton{Icon: scanIcon, Label: locales.NativeMissingPathScan(),
		Disabled: l.missing.scanning || l.env.Svc == nil, OnClick: func() { l.scanMissingPaths(c) }})
	if launch.For(l.env).GOOS() == "windows" {
		icon := "steam"
		if l.batch.steamImporting {
			icon = "loading"
		}
		bar.BatchIslandButton(c, t, "steam", bar.BatchButton{Icon: icon, Label: locales.LibraryBatchImportToSteam(),
			Disabled: none || l.batch.steamImporting, OnClick: func() { l.checkBatchSteam(c) }})
	}
	setStatus := bar.BatchIslandButton(c, t, "set-status", bar.BatchButton{Icon: "tag-edit-outline", Label: locales.LibrarySetStatus(), Disabled: none})
	l.statusMenu(c, t, setStatus, none)
	bar.BatchIslandButton(c, t, "add-category", bar.BatchButton{Icon: "folder-plus-outline", Label: locales.LibraryBatchAddToFilter(),
		Disabled: none, OnClick: func() { l.openBatchCategories(c) }})
	bar.BatchIslandDivider(c, t)
	bar.BatchIslandButton(c, t, "delete", bar.BatchButton{Icon: "delete", Label: locales.LibraryToastBatchDeleteTitle(),
		Danger: true, Disabled: none, OnClick: func() { l.batch.confirmDelete = true }})
}

// statusMenu 是“设为状态”的 BetterDropdownMenu（pill 样式，min-w-[130px]）。
func (l *Page) statusMenu(c *ui.Context, t *theme.Theme, trigger ui.Element, disabled bool) {
	var items []better.MenuItem
	for _, o := range bar.StatusOptions[1:] {
		icon, bg, fg := components.GameStatusStyle(t, o.Value)
		items = append(items, better.MenuItem{
			Key: string(o.Value), Label: o.Label(), Icon: icon, Pill: true, PillBg: bg, PillText: fg,
			OnClick: func() { l.batchSetStatus(c, o.Value) },
		})
	}
	better.DropdownMenu(c, t, trigger, better.MenuProps{Title: locales.LibrarySetStatus(), MinWidth: 130, Items: items, Disabled: disabled})
}
