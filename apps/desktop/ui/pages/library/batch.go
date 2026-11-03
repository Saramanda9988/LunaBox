package library

import (
	"log"
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/bar"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/modal"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// batchState 是批量操作的弹窗：加入收藏夹（AddToCategoryModal）、删除确认（ConfirmModal）与导入到 Steam
// （SteamBatchImportModal）。
type batchState struct {
	category      modal.AddToCategory
	confirmDelete bool

	steamOpen      bool                       // isBatchSteamModalOpen
	steamStatus    *service.SteamLaunchStatus // batchSteamStatus
	steamChecking  bool                       // isCheckingBatchSteam
	steamImporting bool                       // isBatchImportingToSteam
}

// selectedIDs 是选中的游戏（selectedGameIds），按在列表里的顺序。
func (l *Page) selectedIDs() []string {
	ids := make([]string, 0, len(l.selected))
	seen := map[string]bool{}
	for _, g := range l.pages.Games {
		if g != nil && l.selected[g.ID] && !seen[g.ID] {
			ids = append(ids, g.ID)
			seen[g.ID] = true
		}
	}
	var rest []string
	for id := range l.selected {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	slices.Sort(rest)
	return append(ids, rest...)
}

// endBatch 是批量操作成功后的 setSelectedGameIds([]) 与 setBatchMode(false)。
func (l *Page) endBatch() {
	clear(l.selected)
	l.filters.Batch = false
}

// batchSetStatus 对应 handleBatchStatusUpdate：BatchUpdateStatus 后刷新游戏库。
func (l *Page) batchSetStatus(c *ui.Context, status enums.GameStatus) {
	ids := l.selectedIDs()
	if len(ids) == 0 || l.env.Svc == nil {
		return
	}
	svc := l.env.Svc.Game
	l.env.Async(func() func() {
		err := svc.BatchUpdateStatus(ids, string(status))
		return func() {
			if err != nil {
				log.Printf("library: batch update status: %v", err)
				l.env.Toast(c, env.Error(locales.LibraryToastBatchStatusFailed()))
				return
			}
			l.Reload()
			l.env.Toast(c, env.Success(locales.LibraryToastBatchStatusUpdated(strconv.Itoa(len(ids)), bar.StatusLabel(status))))
		}
	})
}

// openBatchCategories 对应 openBatchAddModal：读取全部收藏夹后打开 AddToCategoryModal（不预先勾选）。
func (l *Page) openBatchCategories(c *ui.Context) {
	if len(l.selected) == 0 || l.env.Svc == nil {
		return
	}
	svc := l.env.Svc.Category
	l.env.Async(func() func() {
		all, err := svc.GetCategories()
		return func() {
			if err != nil {
				log.Printf("library: load categories: %v", err)
				l.env.Toast(c, env.Error(locales.LibraryToastLoadFavFailed()))
				return
			}
			l.batch.category.Show(all, nil)
		}
	})
}

// batchAddToCategories 对应 handleBatchAddToCategory：AddGamesToCategories 成功后退出批量模式。
func (l *Page) batchAddToCategories(c *ui.Context, categoryIDs []string) {
	ids := l.selectedIDs()
	if len(ids) == 0 || len(categoryIDs) == 0 {
		return
	}
	svc := l.env.Svc.Category
	l.env.Async(func() func() {
		err := svc.AddGamesToCategories(ids, categoryIDs)
		return func() {
			if err != nil {
				log.Printf("library: batch add to categories: %v", err)
				l.env.Toast(c, env.Error(locales.LibraryToastBatchAddFavFailed()))
				return
			}
			l.env.Toast(c, env.Success(locales.LibraryToastBatchAddFavSuccess(strconv.Itoa(len(ids)))))
			l.endBatch()
		}
	})
}

// batchDelete 对应 handleBatchDelete 确认后的 DeleteGames：删除后刷新游戏库并退出批量模式（首页在下次进入时重新读取）。
func (l *Page) batchDelete(c *ui.Context) {
	ids := l.selectedIDs()
	if len(ids) == 0 || l.env.Svc == nil {
		return
	}
	svc := l.env.Svc.Game
	l.env.Async(func() func() {
		err := svc.DeleteGames(ids)
		return func() {
			if err != nil {
				log.Printf("library: batch delete: %v", err)
				l.env.Toast(c, env.Error(locales.LibraryToastBatchDeleteFailed()))
				return
			}
			l.endBatch()
			l.Reload()
			l.env.Toast(c, env.Success(locales.LibraryToastBatchDeleteSuccess()))
		}
	})
}

// checkBatchSteam 对应 handleBatchImportToSteam：打开弹窗，用第一个选中的游戏检查 Steam 的状态（是否安装、是否在运行）。
func (l *Page) checkBatchSteam(c *ui.Context) {
	ids := l.selectedIDs()
	b := &l.batch
	if len(ids) == 0 || b.steamImporting || l.env.Svc == nil {
		return
	}
	b.steamOpen, b.steamStatus, b.steamChecking = true, nil, true
	svc := l.env.Svc.Integration
	l.env.Async(func() func() {
		status, err := svc.GetGameSteamStatus(ids[0])
		return func() {
			b.steamChecking = false
			if err != nil {
				log.Printf("library: check steam status: %v", err)
				b.steamOpen = false
				l.env.Toast(c, env.Error(locales.SteamImportCheckFailed(err.Error())))
				return
			}
			b.steamStatus = &status
		}
	})
}

// importBatchSteam 对应 performBatchImportToSteam：全部成功时退出批量模式；有失败时只保留失败的游戏，
// 全部因为 Steam 正在运行而失败时留在弹窗里等用户重试。
func (l *Page) importBatchSteam(c *ui.Context) {
	ids := l.selectedIDs()
	b := &l.batch
	if len(ids) == 0 || b.steamImporting {
		return
	}
	b.steamImporting = true
	svc := l.env.Svc.Integration
	l.env.Async(func() func() {
		result, err := svc.BatchImportGamesToSteam(ids)
		return func() {
			b.steamImporting = false
			if err != nil {
				log.Printf("library: batch import to steam: %v", err)
				b.steamOpen = false
				l.env.Toast(c, env.Error(locales.LibraryToastBatchSteamImportFailed()))
				return
			}
			var failed []service.SteamBatchImportItemResult
			states := map[string]bool{}
			for _, it := range result.Items {
				if it.Error != "" || !it.Status.Ready {
					failed = append(failed, it)
					if it.Error != "" {
						states["error"] = true
					} else {
						states[it.Status.State] = true
					}
				}
			}
			done := result.ImportedCount + result.ExistingCount
			if done > 0 {
				l.Reload()
			}
			summary := locales.LibraryToastBatchSteamImportSummary(strconv.Itoa(result.ImportedCount),
				strconv.Itoa(result.FailedCount), strconv.Itoa(result.ExistingCount))
			if result.FailedCount == 0 {
				l.endBatch()
				b.steamOpen = false
				l.env.Toast(c, env.Success(summary))
				return
			}
			clear(l.selected)
			for _, it := range failed {
				l.selected[it.GameID] = true
			}
			if done == 0 && len(states) == 1 && states["steam_running"] {
				status := failed[0].Status
				b.steamStatus = &status
				return
			}
			b.steamOpen = false
			if done > 0 {
				l.env.Toast(c, env.Success(summary))
			} else {
				l.env.Toast(c, env.Error(summary))
			}
		}
	})
}

// batchModals 构建批量操作的弹窗，在页面最后调用。
func (l *Page) batchModals(c *ui.Context, t *theme.Theme) {
	b := &l.batch
	b.category.View(c, t, modal.AddToCategoryProps{
		Env: l.env, Title: locales.LibraryBatchAddToFilter(), ConfirmText: locales.CommonAdd(),
		OnSave: func(ids []string) { l.batchAddToCategories(c, ids) },
	})
	components.ConfirmModal(c, t, "batch-delete", components.ConfirmProps{
		Open:      &b.confirmDelete,
		Title:     locales.LibraryToastBatchDeleteTitle(),
		Message:   locales.LibraryToastBatchDeleteConfirmMsg(strconv.Itoa(len(l.selected))),
		Danger:    true,
		TopOffset: env.TopbarHeight,
		OnConfirm: func() { l.batchDelete(c) },
	})
	l.steamBatchModal(c, t)
}

// steamBatchModal 对应 SteamBatchImportModal：glass-card w-full max-w-lg rounded-2xl border p-6 shadow-2xl，
// 左侧 h-11 w-11 rounded-xl 的图标块，标题 text-xl font-bold，说明 mt-2 text-sm leading-relaxed，
// 按钮 mt-7 flex-wrap justify-end gap-3。遮罩不响应点击与 Esc（原版没有）。
func (l *Page) steamBatchModal(c *ui.Context, t *theme.Theme) {
	b := &l.batch
	state := "confirm"
	switch {
	case b.steamChecking || b.steamStatus == nil:
		state = "checking"
	case !b.steamStatus.SteamInstalled:
		state = "steam_not_installed"
	case b.steamStatus.SteamRunning:
		state = "steam_running"
	}
	icon, title, message := "steam", locales.LibraryBatchImportToSteam(), locales.LibraryToastBatchSteamImportConfirmMsg(strconv.Itoa(len(l.selected)))
	switch state {
	case "checking":
		icon, title, message = "loading", locales.SteamImportCheckingTitle(), locales.SteamImportCheckingMessage()
	case "steam_running":
		title, message = locales.SteamImportSteamRunningTitle(), locales.SteamImportSteamRunning()
	case "steam_not_installed":
		icon, title, message = "alert-circle-outline", locales.SteamImportSteamMissingTitle(), locales.SteamImportSteamMissing()
	}
	busy := b.steamChecking || b.steamImporting
	components.Modal(c, t, "steam-batch-import", components.ModalProps{Open: b.steamOpen, TopOffset: env.TopbarHeight, MaxWidth: 512}, func(p ui.Element) {
		p.Radius(16).Padding(24).Background(t.Pick(theme.White, theme.Brand800)).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
		p.Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				fg := t.Pick(theme.Neutral700, theme.Neutral300)
				ui.Box(c).Size(44, 44).Shrink(0).Center().Radius(12).Background(t.Pick(theme.Neutral100, theme.Neutral900.Alpha(0.3))).Children(func() {
					if icon == "loading" {
						components.Spinner(c, 24, fg)
					} else {
						components.Icon(c, icon, 24, fg)
					}
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, title, theme.TextXL, theme.WeightBold, t.TextStrong)
					components.TextMargin(c, message, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400), 8, 0, 0, 0)
				})
			})
			ui.Row(c).Margin(28, 0, 0, 0).Wrap().GapX(12).GapY(12).Justify(ui.End).Children(func() {
				better.Button(c, t, "steam-cancel", better.ButtonProps{Label: locales.CommonCancel(), Disabled: busy}).
					OnClick(func() { b.steamOpen = false })
				switch state {
				case "confirm":
					better.Button(c, t, "steam-import", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "plus", Label: locales.SteamImportImport(), Loading: b.steamImporting,
					}).OnClick(func() { l.importBatchSteam(c) })
				case "steam_running":
					better.Button(c, t, "steam-retry", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "refresh", Label: locales.SteamImportRetry(), Loading: b.steamChecking,
					}).OnClick(func() { l.checkBatchSteam(c) })
				}
			})
		})
	})
}
