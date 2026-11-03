package account

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// progressModal 对应 RemoteStatusSyncProgressModal.tsx：ModalPortal 里 w-full max-w-lg rounded-xl border border-brand-200
// bg-white p-6 shadow-2xl dark:border-brand-700 dark:bg-brand-800 的面板，py-5 text-center 的图标、标题、进度、
// 当前游戏、进度条、成功与失败数、最后的错误；同步结束后才有“完成”按钮（遮罩与 Esc 都不关闭）。
func progressModal(c *ui.Context, t *theme.Theme, key string, cs *cardState, provider string) {
	if !cs.progressOpen {
		return
	}
	pr := cs.progress
	syncing := cs.syncing
	isFailed := pr.Status == "failed"
	isDone := pr.Status == "done"
	warn := isFailed || pr.FailedGames > 0 // isFailed || hasPartialFailure
	percent := float32(0)
	switch {
	case pr.Total > 0:
		percent = min(1, max(0, float32(pr.Current)/float32(pr.Total)))
	case isDone:
		percent = 1
	}

	components.Modal(c, t, key, components.ModalProps{Open: true, TopOffset: panel.TopOffset, MaxWidth: 512}, func(pnl ui.Element) {
		pnl.Radius(12).Padding(24).Background(t.PanelBg).Border(t.HL, t.PanelBorder).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
		pnl.Children(func() {
			ui.Column(c).FillWidth().Padding(20, 0).AlignItems(ui.Center).Children(func() {
				// mx-auto mb-4 text-5xl：i-mdi-cloud-sync-outline animate-pulse text-info-500 /
				// i-mdi-alert-circle-outline text-warning-500 / i-mdi-check-circle-outline text-success-500
				var icon ui.Element
				switch {
				case syncing:
					icon = components.Icon(c, "cloud-sync-outline", 48, theme.Info500)
					icon.Opacity(components.Pulse(icon))
				case warn:
					icon = components.Icon(c, "alert-circle-outline", 48, theme.Warning500)
				default:
					icon = components.Icon(c, "check-circle-outline", 48, theme.Success500)
				}
				icon.Margin(0, 0, 16, 0)

				title := locales.SettingsBasicRemoteStatusSyncDoneTitle(provider)
				switch {
				case syncing:
					title = locales.SettingsBasicRemoteStatusSyncRunningTitle(provider)
				case warn:
					title = locales.SettingsBasicRemoteStatusSyncFailedTitle(provider)
				}
				// text-lg font-semibold text-brand-900 dark:text-brand-100
				components.Text(c, title, theme.TextLG, theme.WeightSemibold, t.Pick(theme.Brand900, theme.Brand100)).
					FillWidth().TextAlign(ui.Center)
				muted := t.Pick(theme.Brand500, theme.Brand400)
				// mt-2 text-sm text-brand-500 dark:text-brand-400：{current}{" "}/{total}（JSX 去掉了“/”之后的换行）
				components.TextMargin(c, fmt.Sprintf("%d /%d", pr.Current, pr.Total), theme.TextSM, theme.WeightNormal, muted, 8, 0, 0, 0)
				// mt-2 min-h-5 truncate text-sm text-brand-500 dark:text-brand-400
				name := pr.GameName
				if name == "" {
					if pr.Total == 0 && isDone {
						name = locales.SettingsBasicRemoteStatusSyncEmpty(provider)
					} else {
						name = locales.SettingsBasicRemoteStatusSyncPreparing()
					}
				}
				ui.Row(c.Key("game")).FillWidth().MinHeight(20).Margin(8, 0, 0, 0).Justify(ui.Center).Children(func() {
					components.Text(c, name, theme.TextSM, theme.WeightNormal, muted).SingleLine().MinWidth(0)
				})

				// mx-auto mt-4 h-2 w-full max-w-md overflow-hidden rounded-full bg-brand-200 dark:bg-brand-700；里面 h-full
				// rounded-full transition-[width] duration-300，bg-warning-500 或 bg-info-500。
				bar := theme.Info500
				if warn {
					bar = theme.Warning500
				}
				track := ui.Box(c.Key("bar")).FillWidth().MaxWidth(448).Height(8).Margin(16, 0, 0, 0).Radius(999).Clip().
					Background(t.Pick(theme.Brand200, theme.Brand700))
				w := track.AnimateWith("width", percent, 300*time.Millisecond, theme.EaseTailwind)
				track.Children(func() {
					ui.Box(c).WidthPercent(w * 100).Height(8).Radius(999).Background(bar)
				})

				// mx-auto mt-5 grid max-w-sm grid-cols-2 gap-3
				ui.Grid(c.Key("stats")).Columns(2).GapX(12).GapY(12).FillWidth().MaxWidth(384).Margin(20, 0, 0, 0).Children(func() {
					statBox(c, "succeeded", pr.SucceededGames, locales.SettingsBasicRemoteStatusSyncSucceeded(),
						t.Pick(theme.Success50, theme.Success900.Alpha(0.2)),
						t.Pick(theme.Success700, theme.Success300), t.Pick(theme.Success700.Alpha(0.8), theme.Success300.Alpha(0.8)))
					statBox(c, "failed", pr.FailedGames, locales.SettingsBasicRemoteStatusSyncFailed(),
						t.Pick(theme.Warning50, theme.Warning900.Alpha(0.2)),
						t.Pick(theme.Warning700, theme.Warning300), t.Pick(theme.Warning700.Alpha(0.8), theme.Warning300.Alpha(0.8)))
				})

				if pr.LastError != "" {
					// mx-auto mt-4 max-w-md break-words text-xs text-warning-700 dark:text-warning-300
					components.TextMargin(c, pr.LastError, theme.TextXS, theme.WeightNormal, t.Pick(theme.Warning700, theme.Warning300),
						16, 0, 0, 0).MaxWidth(448).TextAlign(ui.Center)
				}

				if !syncing {
					// mt-6 flex justify-center：BetterButton variant="secondary"
					ui.Row(c.Key("buttons")).Margin(24, 0, 0, 0).Justify(ui.Center).Children(func() {
						better.Button(c, t, "complete", better.ButtonProps{Label: locales.CommonComplete()}).OnClick(cs.closeProgress)
					})
				}
			})
		})
	})
}

// statBox：rounded-lg px-3 py-2，text-base font-semibold 的数字与 text-xs 的说明（继承 text-center）。
func statBox(c *ui.Context, key string, n int, label string, bg, number, caption ui.Color) {
	ui.Column(c.Key(key)).Padding(8, 12).Radius(8).AlignItems(ui.Center).Background(bg).Children(func() {
		components.Text(c, strconv.Itoa(n), theme.TextBase, theme.WeightSemibold, number)
		components.Text(c, label, theme.TextXS, theme.WeightNormal, caption).TextAlign(ui.Center)
	})
}
