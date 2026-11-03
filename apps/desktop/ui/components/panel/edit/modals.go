package edit

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/importer"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// deleteModal 是页面的删除确认（ConfirmModal type="danger"）。
func (e *editor) deleteModal(c *ui.Context) {
	components.ConfirmModal(c, e.t, "delete-game", components.ConfirmProps{
		Open:        &e.s.deleteOpen,
		Title:       locales.GameDeleteGame(),
		Message:     locales.GameDeleteConfirmMsg(e.g.Name),
		ConfirmText: locales.GameConfirmDelete(),
		Danger:      true,
		TopOffset:   panel.TopOffset,
		OnConfirm:   func() { e.deleteGame(c) },
	})
}

// fieldSelectModal 对应 MetadataFieldSelectModal（单个游戏）：标题“选择要更新的字段”，确认“开始更新”，
// 按勾选的字段从默认来源更新（handleUpdateFromRemote）。
func (e *editor) fieldSelectModal(c *ui.Context) {
	s := e.s
	better.MetadataFieldSelectModal(c, e.t, "metadata-fields", better.MetadataFieldSelectProps{
		Open:        s.fieldsOpen,
		Title:       locales.MetadataUpdateFieldsModalSingleTitle(),
		Description: locales.MetadataUpdateFieldsModalSingleDescription(),
		ConfirmText: locales.MetadataUpdateFieldsModalUpdate(),
		Fields:      &s.fields,
		Submitting:  s.updating,
		TopOffset:   panel.TopOffset,
		OnClose:     func() { s.fieldsOpen = false },
		OnConfirm:   func(fields []enums.MetadataUpdateField) { e.updateFromRemote(c, fields) },
	})
}

// searchModal 对应 MetadataSourceSearchModal：max-w-2xl rounded-xl bg-white p-6 shadow-2xl dark:bg-brand-800，
// text-4xl 的标题与关闭按钮，下面是 MetadataSearchResultsStep。应用结果时（isApplying）按钮都禁用。
func (e *editor) searchModal(c *ui.Context) {
	t, s := e.t, e.s
	closeModal := func() { s.searchOpen = false; s.resultsUI.Reset() }
	components.Modal(c, t, "metadata-search", components.ModalProps{Open: s.searchOpen, TopOffset: panel.TopOffset, MaxWidth: 672},
		func(pnl ui.Element) {
			pnl.Radius(12).Padding(24).Background(t.PanelBg).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
				// mb-2 flex items-center justify-between gap-4
				ui.Row(c).Margin(0, 0, 8, 0).Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					components.Text(c, locales.GameEditMetadataSearchTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
					// i-mdi-close rounded-lg p-1 text-2xl：按钮本身就是图标，悬停只变色。
					btn := ui.ButtonBase(c.Key("close")).Size(32, 32).Center().Shrink(0).FocusRing(false).Label(locales.CommonCancel())
					color := t.Pick(theme.Brand500, theme.Brand400)
					if s.applying {
						components.Disable(btn, 0.6)
					} else if btn.Hovered() {
						color = t.Pick(theme.Brand700, theme.Brand200)
					}
					btn.OnClick(closeModal).Children(func() { components.Icon(c, "close", 32, color) })
				})
				e.searchResultsStep(c, closeModal)
			})
		})
}

// searchResultsStep 是 MetadataSearchResultsStep（importer.SearchResults）：没有结果时 min-h-64 的空状态，
// 底部 border-t pt-4 的“← 返回”。
func (e *editor) searchResultsStep(c *ui.Context, closeModal func()) {
	t, s := e.t, e.s
	s.resultsUI.View(c, t, importer.SearchResultsProps{
		Results:  s.results,
		Covers:   e.p.Env.Covers,
		Disabled: s.applying,
		OnSelect: func(item vo.GameMetadataFromWebVO) { e.selectSearchResult(c, item) },
		OnRemove: func(i int) { s.results = slices.Delete(slices.Clone(s.results), i, i+1) },
		Empty: func() {
			// flex min-h-64 flex-col items-center justify-center gap-3 text-brand-500 dark:text-brand-400
			color := t.Pick(theme.Brand500, theme.Brand400)
			ui.Column(c.Key("empty")).MinHeight(256).Center().Gap(12).Children(func() {
				components.Icon(c, "database-search-outline", 36, color)
				components.Text(c, locales.GameEditMetadataSearchEmpty(), theme.TextSM, theme.WeightNormal, color)
			})
		},
		Footer: func() {
			// flex items-center justify-between border-t border-brand-200 pt-4 dark:border-brand-700
			ui.Row(c.Key("footer")).Padding(16, 0, 0, 0).BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Divider).Children(func() {
				back := ui.ButtonBase(c.Key("back")).FocusRing(false).Label(locales.AddGameModalGoBack())
				color := t.Pick(theme.Brand500, theme.Brand400)
				if s.applying {
					components.Disable(back, 0.6)
				} else if back.Hovered() {
					color = t.Pick(theme.Brand700, theme.Brand200)
				}
				back.OnClick(closeModal).Children(func() {
					components.Text(c, "←"+locales.AddGameModalGoBack(), theme.TextSM, theme.WeightNormal, color).NoWrap()
				})
			})
		},
	})
}
