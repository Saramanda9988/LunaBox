package importer

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// skippedModal 对应 SkippedImportCandidatesModal：max-h-[82vh] w-full max-w-3xl rounded-xl 的面板，页头 border-b p-5
// （i-mdi-folder-alert-outline、text-lg font-bold 的标题与 mt-1 text-sm 的说明），中间 p-5 space-y-2 的列表，
// 页脚 border-t p-4 的确定按钮。点遮罩关闭。
func skippedModal(c *ui.Context, t *theme.Theme, cands []Candidate, open *bool) {
	closeModal := func() { *open = false }
	_, winH := c.Size()
	components.Modal(c, t, "skipped-candidates", components.ModalProps{Open: *open, TopOffset: env.TopbarHeight, MaxWidth: 768, OnClose: closeModal, Label: locales.CommonClose()},
		func(pnl ui.Element) {
			maxH := winH * 0.82
			pnl.MaxHeight(maxH).Radius(12).Clip().Background(t.PanelBg).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
			pnl.Children(func() {
				ui.Row(c.Key("header")).Padding(20).AlignItems(ui.Center).Justify(ui.SpaceBetween).
					BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
					ui.Column(c).MinWidth(0).Grow(1).Basis(0).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							components.Icon(c, "folder-alert-outline", 24, t.Pick(theme.Yellow500, theme.Yellow300))
							components.Text(c, locales.BatchImportModalSkippedExistingTitle(), theme.TextLG, theme.WeightBold, t.TextStrong).SingleLine()
						})
						components.TextMargin(c, locales.BatchImportModalSkippedExistingHint(), theme.TextSM, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
					})
					closeIconButton(c, t, closeModal)
				})
				// 页头（p-5 + 两行）与页脚（p-4 + py-2）之外是列表
				listH := maxH - (40 + 28 + 24) - (32 + 36) - 2*t.HL
				ui.Scroll(c.Key("list")).MaxHeight(max(0, listH)).Children(func() {
					ui.Column(c).Padding(20).Gap(8).Children(func() {
						for i, cand := range cands {
							skippedRow(c.Key(fmt.Sprint(i)), t, cand)
						}
					})
				})
				ui.Row(c.Key("footer")).Padding(16).Justify(ui.End).BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
					// rounded-lg bg-brand-700 px-5 py-2 text-sm font-medium text-white hover:bg-brand-800
					// dark:bg-brand-600 dark:hover:bg-brand-500
					solidButton(c, "close", locales.CommonConfirm(), t.Pick(theme.Brand700, theme.Brand600), t.Pick(theme.Brand800, theme.Brand500), false, false, 20, 8).
						OnClick(closeModal)
				})
			})
		})
}

// skippedRow：rounded-lg border border-brand-200 bg-brand-50/80 p-3，名称 truncate text-sm font-medium，路径 mt-1 text-xs，
// 原因 mt-2 text-xs text-yellow-700。
func skippedRow(c *ui.Context, t *theme.Theme, cand Candidate) {
	ui.Column(c).Padding(12).Radius(8).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
		Background(t.Pick(theme.Brand50.Alpha(0.8), theme.Brand900.Alpha(0.3))).Children(func() {
		name := cand.SearchName
		if name == "" {
			name = cand.FolderName
		}
		components.Text(c, name, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine()
		components.TextMargin(c, locales.BatchImportModalSkippedExistingPath()+": "+cand.FolderPath, theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0).SingleLine()
		if cand.SkipReason != "" {
			components.TextMargin(c, locales.BatchImportModalSkippedExistingReason()+": "+cand.SkipReason, theme.TextXS, theme.WeightNormal,
				t.Pick(theme.Yellow700, theme.Yellow300), 8, 0, 0, 0)
		}
	})
}

// closeIconButton 是 i-mdi-close rounded-lg p-1 text-2xl 的关闭按钮（悬停时图标变成 brand-100，见 ImportModalContainer）。
func closeIconButton(c *ui.Context, t *theme.Theme, onClose func()) {
	btn := ui.ButtonBase(c.Key("close")).Size(24, 24).Center().Shrink(0).FocusRing(false).Label(locales.CommonClose())
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand100, theme.Brand700)
	}
	btn.OnClick(onClose).Children(func() { components.Icon(c, "close", 24, color) })
}

// manualModal 对应 ImportManualSelectModal：absolute inset-0 z-60 bg-black/50（没有模糊），max-h-[80vh] max-w-2xl rounded-xl
// 的面板；页头 border-b p-4 是“手动选择：名称”与关闭按钮，内容 p-4 space-y-4：候选卡片（w-36 rounded-lg border p-2，
// aspect-[3/4] 的封面）、没有结果时的提示、按 ID 搜索（BetterSelect w-32、输入框与搜索按钮）、不使用元数据导入。
func (f *Flow) manualModal(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	m := &f.manual
	if !m.open || m.index >= len(f.Candidates) {
		return
	}
	name := f.Candidates[m.index].SearchName
	closeModal := func() { m.open = false }
	_, winH := c.Size()
	components.Overlay(c, t, func() {
		host := ui.Box(c.Key("manual-select")).Absolute().Top(env.TopbarHeight).Left(0).Right(0).Bottom(0).Modal().
			Background(ui.RGBA(0, 0, 0, 0.5))
		host.OnShortcut(0, ui.KeyEscape, closeModal)
		host.Children(func() {
			ui.Column(c).Fill().Center().Children(func() {
				maxH := winH * 0.8
				pnl := ui.Column(c.Key("panel")).FillWidth().MaxWidth(672).MaxHeight(maxH).Radius(12).Clip().
					Background(t.PanelBg).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
				pnl.Children(func() {
					ui.Row(c.Key("header")).Padding(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).
						BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
						components.Text(c, locales.BatchImportModalManualSelect()+":"+name, theme.TextLG, theme.WeightBold, t.TextStrong).SingleLine().Shrink(1)
						btn := ui.ButtonBase(c.Key("close")).Size(20, 20).Center().Shrink(0).FocusRing(false).Label(locales.CommonClose())
						color := theme.Brand500
						if btn.Hovered() {
							color = theme.Brand700
						}
						btn.OnClick(closeModal).Children(func() { components.Icon(c, "close", 20, color) })
					})
					ui.Scroll(c.Key("body")).MaxHeight(max(0, maxH-60-t.HL)).Children(func() {
						ui.Column(c).Padding(16).Gap(16).Children(func() {
							if m.searching {
								loadingStep(c, t, theme.Neutral500, locales.CommonSearching(), "")
								return
							}
							f.manualMatches(c, t)
							f.manualByID(c, t, enabled)
							skip := ui.ButtonBase(c.Key("skip")).FillWidth().Padding(8, 0).Center().FocusRing(false)
							fg := theme.Brand400
							if skip.Hovered() {
								fg = theme.Brand600
							}
							skip.OnClick(f.skipMetadata).Children(func() {
								components.Text(c, locales.BatchImportModalImportWithoutMeta(), theme.TextSM, theme.WeightNormal, fg)
							})
						})
					})
				})
			})
		})
	})
}

// manualMatches：flex flex-wrap gap-3 的候选卡片；没有结果时 py-4 text-center 的提示。
func (f *Flow) manualMatches(c *ui.Context, t *theme.Theme) {
	m := &f.manual
	ui.Row(c.Key("matches")).Wrap().GapX(12).GapY(12).Children(func() {
		for i, match := range m.matches {
			if match.Game.Name == "" {
				continue
			}
			card := ui.ButtonBase(c.Key(fmt.Sprintf("%s-%s-%d", match.Source, match.Game.SourceID, i))).Column().Width(144).Padding(8).Radius(8).
				AlignItems(ui.Stretch).FocusRing(false).Cursor(ui.CursorPointer).Label(match.Game.Name)
			border := t.Pick(theme.Brand200, theme.Brand700)
			if card.Hovered() {
				border = theme.Neutral500
				card.Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
			}
			card.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { f.selectManual(match) }).Children(func() {
				ui.Box(c).FillWidth().AspectRatio(3.0 / 4).Radius(6).Clip().Center().Background(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
					coverImage(c, f.env.Covers, match.Game, 30)
				})
				components.TextMargin(c, match.Game.Name, theme.TextXS, theme.WeightBold, t.TextStrong, 4, 0, 0, 0).SingleLine()
				components.Text(c, string(match.Source), theme.TextXS, theme.WeightNormal, theme.Brand400)
			})
		}
	})
	if len(m.matches) == 0 {
		ui.Row(c.Key("no-match")).Padding(16, 0).Justify(ui.Center).Children(func() {
			components.Text(c, locales.BatchImportModalNoMatchResult(), theme.TextBase, theme.WeightNormal, theme.Brand400)
		})
	}
}

// manualByID：mt-4 border-t pt-4，mb-3 text-sm 的说明，flex gap-2 的来源、ID 与搜索按钮。
func (f *Flow) manualByID(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	m := &f.manual
	ui.Column(c.Key("by-id")).Margin(16, 0, 0, 0).Padding(16, 0, 0, 0).BorderWidth(t.HL, 0, 0, 0).
		BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		components.TextMargin(c, locales.BatchImportModalSearchById(), theme.TextSM, theme.WeightNormal, theme.Brand500, 0, 0, 12, 0)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			opts := make([]better.SelectOption, len(enabled))
			for i, s := range enabled {
				opts[i] = better.SelectOption{Label: SourceLabel(s), Value: string(s)}
			}
			ui.Box(c).Width(128).Shrink(0).Children(func() {
				better.Select(c, t, "source", better.SelectProps{Value: string(m.source), Options: opts, OnChange: func(v string) { m.source = enums.SourceType(v) }})
			})
			// flex-1 rounded border border-brand-300 bg-brand-50 px-3 py-1.5 text-sm dark:border-brand-600 dark:bg-brand-700
			in := ui.TextInputBase(c.Key("id"), &m.id).Grow(1).Basis(0).MinWidth(0).Padding(6, 12).Radius(4).
				FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
				Background(t.Pick(theme.Brand50, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).
				Placeholder(locales.BatchImportModalInputId()).Label(locales.BatchImportModalInputId())
			if theme.UIFont != "" {
				in.Font(theme.UIFont)
			}
			disabled := m.id == "" || m.searching
			btn := ui.ButtonBase(c.Key("search")).Padding(6, 16).Radius(4).Center().FocusRing(false)
			bg := theme.Neutral500
			if btn.Hovered() && !disabled {
				bg = theme.Neutral600
			}
			btn.Background(bg)
			if disabled {
				components.Disable(btn, 0.5)
			} else {
				btn.OnClick(func() { f.searchByID(c) })
			}
			btn.Children(func() {
				components.Text(c, locales.CommonSearch(), theme.TextSM, theme.WeightNormal, theme.White).NoWrap()
			})
			if in.Submitted() && !disabled {
				f.searchByID(c)
			}
		})
	})
}
