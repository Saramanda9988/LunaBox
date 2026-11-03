package metadata

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// Tailwind 默认的 gray 色阶（uno.config.ts 没有覆盖 gray）。
var (
	gray50  = ui.Hex("#f9fafb")
	gray100 = ui.Hex("#f3f4f6")
	gray300 = ui.Hex("#d1d5db")
	gray400 = ui.Hex("#9ca3af")
	gray600 = ui.Hex("#4b5563")
	gray700 = ui.Hex("#374151")
	gray900 = ui.Hex("#111827")
)

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// enrichmentReasonText 对应 enrichmentReasonTranslationKeys：未知的原因显示 no_mapping。
func enrichmentReasonText(reason string) string {
	switch reason {
	case "already_complete":
		return locales.SettingsMetadataIdPreviewReasonsAlreadyComplete()
	case "no_available_ids":
		return locales.SettingsMetadataIdPreviewReasonsNoAvailableIds()
	case "can_enrich":
		return locales.SettingsMetadataIdPreviewReasonsCanEnrich()
	}
	return locales.SettingsMetadataIdPreviewReasonsNoMapping()
}

// previewSourceLabel 对应 GameIDEnrichmentPreviewModal.tsx 的 sourceLabel。
func previewSourceLabel(s enums.SourceType) string {
	switch s {
	case enums.Bangumi:
		return "Bangumi"
	case enums.VNDB:
		return "VNDB"
	case enums.Steam:
		return "Steam"
	}
	return string(s)
}

// previewModalProps 是 GameIDEnrichmentPreviewModal 的属性。
type previewModalProps struct {
	open, loading, applying bool
	preview                 *service.GameIDEnrichmentPreview
	onClose, onConfirm      func()
}

// previewModal 对应 components/modal/GameIDEnrichmentPreviewModal.tsx：ImportModalContainer（i-mdi-database-search-outline
// text-3xl text-neutral-500）里，加载中是 min-h-80 的旋转图标与说明；有结果时是 space-y-4 的两格统计、BetterDataTable
// （max-h-[48vh]）与底部的扫描数和“补全”按钮。应用中（isApplying）不能关闭。
func previewModal(c *ui.Context, t *theme.Theme, p previewModalProps) {
	closeModal := func() {
		if !p.applying {
			p.onClose()
		}
	}
	components.ImportModalContainer(c, t, "metadata-id-preview", components.ImportModalProps{
		Open: p.open, Title: locales.SettingsMetadataIdPreviewTitle(), Icon: "database-search-outline", IconSize: 30,
		IconColor: theme.Neutral500, TopOffset: panel.TopOffset, OnClose: closeModal,
	}, func() {
		switch {
		case p.loading:
			// flex min-h-80 flex-col items-center justify-center gap-3 text-brand-500 dark:text-brand-400
			color := t.Pick(theme.Brand500, theme.Brand400)
			ui.Column(c.Key("loading")).FillWidth().MinHeight(320).Center().Gap(12).Children(func() {
				components.Spinner(c, 36, color)
				components.Text(c, locales.SettingsMetadataIdPreviewLoadingText(), theme.TextSM, theme.WeightNormal, color)
			})
		case p.preview != nil:
			previewContent(c, t, p)
		}
	})
}

func previewContent(c *ui.Context, t *theme.Theme, p previewModalProps) {
	pv := p.preview
	winW, winH := c.Size()
	ui.Column(c.Key("preview")).FillWidth().Gap(16).Children(func() {
		// flex gap-4：两格 flex-1 rounded-lg p-4 text-center，text-3xl font-bold 的数字与 text-sm 的说明。
		ui.Row(c.Key("stats")).FillWidth().Gap(16).Children(func() {
			summaryBox(c, "enrichable", pv.EnrichableGames, locales.SettingsMetadataIdPreviewEnrichableGames(),
				t.Pick(theme.Success50, theme.Success900.Alpha(0.2)), t.Pick(theme.Success600, theme.Success400), t.Pick(theme.Success700, theme.Success300))
			summaryBox(c, "unchanged", pv.UnchangedGames, locales.SettingsMetadataIdPreviewUnchangedGames(),
				t.Pick(gray50, gray900.Alpha(0.2)), t.Pick(gray600, gray400), t.Pick(gray700, gray300))
		})
		better.DataTable(c, t, "items", better.DataTableProps[service.GameIDEnrichmentPreviewItem]{
			Rows:      pv.Items,
			Columns:   previewColumns(c, t),
			RowKey:    func(item service.GameIDEnrichmentPreviewItem, _ int) string { return item.GameID },
			MaxHeight: winH * 0.48,
			Empty: func() {
				components.Text(c, locales.SettingsMetadataIdPreviewEmpty(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand400, theme.Brand500)).
					TextAlign(ui.Center)
			},
		})
		// flex flex-col justify-between gap-3 sm:flex-row sm:items-center
		footer := ui.Row(c.Key("footer")).FillWidth().Gap(12).Justify(ui.SpaceBetween).AlignItems(ui.Center)
		if winW < 640 {
			footer.Column().AlignItems(ui.Stretch)
		}
		footer.Children(func() {
			components.Text(c, locales.SettingsMetadataIdPreviewScanned(strconv.Itoa(pv.ScannedGames)), theme.TextSM, theme.WeightNormal,
				t.Pick(theme.Brand500, theme.Brand400))
			// flex items-center justify-end gap-3
			ui.Row(c).Gap(12).AlignItems(ui.Center).Justify(ui.End).Children(func() {
				better.Button(c, t, "confirm", better.ButtonProps{
					Variant: better.ButtonPrimary, Icon: "database-plus-outline", Loading: p.applying, Disabled: pv.EnrichableGames == 0,
					Label: locales.SettingsMetadataIdPreviewConfirm(strconv.Itoa(pv.EnrichableGames)),
				}).OnClick(func() {
					if !p.applying && pv.EnrichableGames > 0 {
						p.onConfirm()
					}
				})
			})
		})
	})
}

// summaryBox 是顶部的一格统计。
func summaryBox(c *ui.Context, key string, n int, label string, bg, numColor, labelColor ui.Color) {
	ui.Column(c.Key(key)).Grow(1).Basis(0).MinWidth(0).Padding(16).Radius(8).AlignItems(ui.Center).Background(bg).Children(func() {
		components.Text(c, strconv.Itoa(n), theme.Text3XL, theme.WeightBold, numColor)
		components.Text(c, label, theme.TextSM, theme.WeightNormal, labelColor).TextAlign(ui.Center)
	})
}

// previewColumns 是表格的三列：游戏（w-[30%]）、已有来源（w-[34%]）与补全结果（w-[36%]）。
func previewColumns(c *ui.Context, t *theme.Theme) []better.DataTableColumn[service.GameIDEnrichmentPreviewItem] {
	type item = service.GameIDEnrichmentPreviewItem
	return []better.DataTableColumn[item]{
		{
			Key: "game", Header: locales.SettingsMetadataIdPreviewColumnsGame(), Width: 0.30,
			Cell: func(it item, _ int) {
				// min-w-0：truncate font-medium text-brand-900 dark:text-white 的名称，mt-0.5 truncate text-xs
				// text-brand-400 dark:text-brand-500 的默认来源与 ID。
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, it.GameName, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine()
					components.TextMargin(c, previewSourceLabel(it.DefaultSource)+" "+it.DefaultSourceID, theme.TextXS, theme.WeightNormal,
						t.Pick(theme.Brand400, theme.Brand500), 2, 0, 0, 0).SingleLine()
				})
			},
		},
		{
			Key: "existing", Header: locales.SettingsMetadataIdPreviewColumnsExisting(), Width: 0.34,
			Cell: func(it item, _ int) { sourceBadges(c, t, it.ExistingSources, false) },
		},
		{
			Key: "result", Header: locales.SettingsMetadataIdPreviewColumnsResult(), Width: 0.36,
			Cell: func(it item, _ int) {
				if it.CanEnrich {
					sourceBadges(c, t, it.AddedSources, true)
					return
				}
				// flex items-start gap-1.5 text-xs text-brand-500 dark:text-brand-400：i-mdi-information-outline mt-0.5 shrink-0
				// text-brand-400 与原因。
				ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(6).AlignItems(ui.Start).Children(func() {
					components.Icon(c, "information-outline", 12, theme.Brand400).Margin(2, 0, 0, 0)
					components.Text(c, enrichmentReasonText(it.Reason), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).
						Grow(1).Basis(0).MinWidth(0)
				})
			},
		},
	}
}

// sourceBadges：flex flex-wrap gap-1.5 的 SourceBadge。
func sourceBadges(c *ui.Context, t *theme.Theme, sources []service.GameIDEnrichmentSource, added bool) {
	ui.Row(c).Grow(1).Basis(0).MinWidth(0).Wrap().Gap(6).Children(func() {
		for _, s := range sources {
			sourceBadge(c.Key(string(s.SourceType)), t, s, added)
		}
	})
}

// sourceBadge 对应 SourceBadge：inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs；新增的是 bg-success-100
// text-success-700 dark:bg-success-900/30 dark:text-success-400，已有的是 bg-gray-100 text-gray-700 dark:bg-gray-900/30
// dark:text-gray-400。来源名与 font-mono 的 ID。
func sourceBadge(c *ui.Context, t *theme.Theme, s service.GameIDEnrichmentSource, added bool) {
	bg, fg := t.Pick(gray100, gray900.Alpha(0.3)), t.Pick(gray700, gray400)
	if added {
		bg, fg = t.Pick(theme.Success100, theme.Success900.Alpha(0.3)), t.Pick(theme.Success700, theme.Success400)
	}
	ui.Row(c).Gap(4).Padding(4, 8).Radius(999).AlignItems(ui.Center).Background(bg).MinWidth(0).Children(func() {
		components.Text(c, previewSourceLabel(s.SourceType), theme.TextXS, theme.WeightNormal, fg).NoWrap()
		components.Text(c, s.SourceID, theme.TextXS, theme.WeightNormal, fg).Font(monoFont).NoWrap()
	})
}
