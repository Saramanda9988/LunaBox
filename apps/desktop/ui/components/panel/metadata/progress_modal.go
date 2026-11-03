package metadata

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// refreshProgress 对应 MetadataRefreshProgressModal.tsx 的 MetadataRefreshProgress（metadata:refresh-progress 事件的内容）。
type refreshProgress struct {
	status          string
	current, total  int
	gameName        string
	updated         int
	skipped         int
	failed          int
	locked          int
	failedGameIDs   []string
	failedGameNames []string
}

// newRefreshProgress 对应 createMetadataRefreshProgress。
func newRefreshProgress(status string) refreshProgress { return refreshProgress{status: status} }

// progressFromResult 对应 metadataResultToProgress。
func progressFromResult(r vo.MetadataRefreshResult, status string) refreshProgress {
	return refreshProgress{
		status: status, current: r.TotalGames, total: r.TotalGames,
		updated: r.UpdatedGames, skipped: r.SkippedGames, failed: r.FailedGames, locked: r.LockedGames,
		failedGameIDs: r.FailedGameIDs, failedGameNames: r.FailedGameNames,
	}
}

// progressFromEvent 读取 GameService.emitMetadataRefreshProgress 发出的 map；字段缺失时为零值（原版的 failed_game_ids || []）。
func progressFromEvent(data any) (refreshProgress, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return refreshProgress{}, false
	}
	str := func(k string) string { s, _ := m[k].(string); return s }
	num := func(k string) int {
		switch v := m[k].(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
		return 0
	}
	list := func(k string) []string {
		switch v := m[k].(type) {
		case []string:
			return v
		case []any:
			out := make([]string, 0, len(v))
			for _, x := range v {
				if s, ok := x.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
		return nil
	}
	return refreshProgress{
		status: str("status"), current: num("current"), total: num("total"), gameName: str("game_name"),
		updated: num("updated_games"), skipped: num("skipped_games"), failed: num("failed_games"), locked: num("locked_games"),
		failedGameIDs: list("failed_game_ids"), failedGameNames: list("failed_game_names"),
	}, true
}

// progressModalProps 是 MetadataRefreshProgressModal 的属性。
type progressModalProps struct {
	open          bool
	progress      refreshProgress
	refreshing    bool
	onRetryFailed func()
	onClose       func()
}

// progressModal 对应 MetadataRefreshProgressModal.tsx：遮罩 p-4，面板 w-full max-w-xl rounded-xl border border-brand-200 bg-white p-6
// shadow-2xl dark:border-brand-700 dark:bg-brand-800；里面 py-8 text-center 的状态图标、标题、进度、当前游戏、进度条、
// 四格统计，最后是提示（更新中）或“完成”与“重试失败项”。
func progressModal(c *ui.Context, t *theme.Theme, p progressModalProps) {
	pr := p.progress
	winW, _ := c.Size()
	sm := winW >= 640
	width := float32(0)
	if pr.total > 0 {
		width = min(1, max(0, float32(pr.current)/float32(pr.total)))
	}
	failedCount := pr.failed
	if failedCount == 0 {
		failedCount = len(pr.failedGameIDs)
	}
	canRetry := !p.refreshing && len(pr.failedGameIDs) > 0 && p.onRetryFailed != nil

	components.Modal(c, t, "metadata-refresh-progress", components.ModalProps{Open: p.open, TopOffset: panel.TopOffset, MaxWidth: 576},
		func(pnl ui.Element) {
			pnl.Radius(12).Padding(24).Background(t.PanelBg).Border(t.HL, t.PanelBorder).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
			pnl.Children(func() {
				ui.Column(c).FillWidth().Padding(32, 0).AlignItems(ui.Center).Children(func() {
					// mx-auto mb-4 text-5xl：i-mdi-loading animate-spin text-info-500 / alert-circle-outline text-warning-500 /
					// check-circle-outline text-success-500
					var icon ui.Element
					switch {
					case p.refreshing:
						icon = components.Spinner(c, 48, theme.Info500)
					case failedCount > 0:
						icon = components.Icon(c, "alert-circle-outline", 48, theme.Warning500)
					default:
						icon = components.Icon(c, "check-circle-outline", 48, theme.Success500)
					}
					icon.Margin(0, 0, 16, 0)
					title := locales.SettingsMetadataProgressDoneTitle()
					switch {
					case p.refreshing:
						title = locales.SettingsMetadataProgressTitle()
					case failedCount > 0:
						title = locales.SettingsMetadataProgressFailedTitle()
					}
					// text-lg text-brand-600 dark:text-brand-300
					components.Text(c, title, theme.TextLG, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).TextAlign(ui.Center)
					// mt-2 text-sm text-brand-400 dark:text-brand-500：{current}{" "}/{total}
					components.TextMargin(c, fmt.Sprintf("%d /%d", pr.current, pr.total), theme.TextSM, theme.WeightNormal,
						t.Pick(theme.Brand400, theme.Brand500), 8, 0, 0, 0)
					// mt-2 min-h-5 truncate text-sm text-neutral-500 dark:text-neutral-400
					name := pr.gameName
					if name == "" {
						name = locales.SettingsMetadataProgressPreparing()
					}
					ui.Row(c.Key("game")).FillWidth().MinHeight(20).Margin(8, 0, 0, 0).Justify(ui.Center).Children(func() {
						components.Text(c, name, theme.TextSM, theme.WeightNormal, t.Pick(theme.Neutral500, theme.Neutral400)).SingleLine().MinWidth(0)
					})
					// mx-auto mt-4 h-2 w-full max-w-md rounded-full bg-brand-200 dark:bg-brand-700；里面 h-2 rounded-full
					// transition-all duration-300 bg-info-500（完成且有失败时 bg-warning-500），宽度是百分比。
					bar := theme.Info500
					if failedCount > 0 && !p.refreshing {
						bar = theme.Warning500
					}
					track := ui.Box(c.Key("bar")).FillWidth().MaxWidth(448).Height(8).Margin(16, 0, 0, 0).Radius(999).
						Background(t.Pick(theme.Brand200, theme.Brand700))
					w := track.AnimateWith("width", width, 300*time.Millisecond, theme.EaseTailwind)
					track.Children(func() {
						ui.Box(c).WidthPercent(w * 100).Height(8).Radius(999).Background(bar).Transition(theme.ColorsTransition(300 * time.Millisecond))
					})
					// mx-auto mt-5 grid max-w-md grid-cols-2 gap-2 sm:grid-cols-4
					cols := 2
					if sm {
						cols = 4
					}
					ui.Grid(c.Key("stats")).Columns(cols).GapX(8).GapY(8).FillWidth().MaxWidth(448).Margin(20, 0, 0, 0).Children(func() {
						green := [2]ui.Color{t.Pick(theme.Success700, theme.Success300), t.Pick(theme.Success700.Alpha(0.8), theme.Success300.Alpha(0.8))}
						amber := [2]ui.Color{t.Pick(theme.Warning700, theme.Warning300), t.Pick(theme.Warning700.Alpha(0.8), theme.Warning300.Alpha(0.8))}
						gray := [2]ui.Color{t.Pick(theme.Brand700, theme.Brand300), t.Pick(theme.Brand500, theme.Brand400)}
						statBox(c, t, "updated", pr.updated, locales.SettingsMetadataProgressUpdated(), t.Pick(theme.Success50, theme.Success900.Alpha(0.2)), green)
						statBox(c, t, "failed", failedCount, locales.SettingsMetadataProgressFailed(), t.Pick(theme.Warning50, theme.Warning900.Alpha(0.2)), amber)
						statBox(c, t, "skipped", pr.skipped, locales.SettingsMetadataProgressSkipped(), t.Pick(theme.Brand50, theme.Brand900.Alpha(0.3)), gray)
						statBox(c, t, "locked", pr.locked, locales.SettingsMetadataProgressLocked(), t.Pick(theme.Brand50, theme.Brand900.Alpha(0.3)), gray)
					})
					if p.refreshing {
						// mt-4 text-xs text-brand-400
						components.TextMargin(c, locales.SettingsMetadataProgressHint(), theme.TextXS, theme.WeightNormal, theme.Brand400, 16, 0, 0, 0).
							TextAlign(ui.Center)
						return
					}
					// mt-6 flex flex-col-reverse justify-center gap-3 sm:flex-row
					buttons := ui.Row(c.Key("buttons")).Margin(24, 0, 0, 0).Gap(12).Justify(ui.Center)
					if !sm {
						buttons.Column().Reverse().AlignSelf(ui.Stretch)
					}
					buttons.Children(func() {
						if p.onClose != nil {
							better.Button(c, t, "complete", better.ButtonProps{Label: locales.CommonComplete()}).OnClick(p.onClose)
						}
						if canRetry {
							better.Button(c, t, "retry", better.ButtonProps{
								Variant: better.ButtonPrimary, Icon: "refresh", Label: locales.SettingsMetadataProgressRetryFailed(strconv.Itoa(len(pr.failedGameIDs))),
							}).OnClick(p.onRetryFailed)
						}
					})
				})
			})
		})
}

// statBox 是四格统计之一：rounded-lg px-3 py-2，text-base font-semibold 的数字与 text-xs 的说明（居中）。
func statBox(c *ui.Context, t *theme.Theme, key string, n int, label string, bg ui.Color, fg [2]ui.Color) {
	ui.Column(c.Key(key)).Padding(8, 12).Radius(8).AlignItems(ui.Center).Background(bg).Children(func() {
		components.Text(c, strconv.Itoa(n), theme.TextBase, theme.WeightSemibold, fg[0])
		components.Text(c, label, theme.TextXS, theme.WeightNormal, fg[1]).TextAlign(ui.Center)
	})
}
