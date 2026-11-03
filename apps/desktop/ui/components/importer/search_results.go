package importer

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// SearchResults 是 MetadataSearchResultsStep 的状态：选择说明是否打开、候选卡片的横向滚动。
type SearchResults struct {
	helpOpen bool
	scroll   ui.ScrollState
}

// Reset 关闭说明并滚回开头（换了一组结果时调用）。
func (s *SearchResults) Reset() { s.helpOpen, s.scroll.X = false, 0 }

// SearchResultsProps 是 MetadataSearchResultsStep 的属性。
type SearchResultsProps struct {
	Results  []vo.GameMetadataFromWebVO
	Covers   *covers.Cache // 候选封面；为 nil 时只显示占位
	Disabled bool          // disabled：应用结果时按钮都禁用
	OnSelect func(result vo.GameMetadataFromWebVO)
	OnRemove func(index int)
	Empty    func() // 没有结果时的内容；为 nil 时不显示
	Footer   func()
}

// View 对应 MetadataSearchResultsStep（space-y-6）：提示与选择说明，横向滚动的候选卡片（两侧翻页按钮），
// 没有结果时的 Empty，最后是 Footer。
func (s *SearchResults) View(c *ui.Context, t *theme.Theme, p SearchResultsProps) {
	ui.Column(c.Key("results-step")).Gap(24).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			components.Text(c, locales.AddGameModalWhichResult(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).NoWrap()
			s.selectionHelp(c, t)
		})
		switch {
		case len(p.Results) > 0:
			s.cards(c, t, p)
		case p.Empty != nil:
			p.Empty()
		}
		if p.Footer != nil {
			p.Footer()
		}
	})
}

// selectionHelp 是“查看候选结果选择说明”：h-7 w-7 rounded-full 的问号按钮，点开 w-80 rounded-xl border p-4 shadow-xl 的说明。
func (s *SearchResults) selectionHelp(c *ui.Context, t *theme.Theme) {
	winW, _ := c.Size()
	btn := ui.ButtonBase(c.Key("help")).Size(28, 28).Radius(999).Center().Shrink(0).FocusRing(false).Label(locales.AddGameModalSelectionHelp())
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() || s.helpOpen {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		color = t.Pick(theme.Brand700, theme.Brand200)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	btn.OnClick(func() { s.helpOpen = !s.helpOpen }).Children(func() {
		components.Icon(c, "help-circle-outline", 18, color)
	})
	steps := []string{
		locales.AddGameModalSelectionHelpRemove(),
		locales.AddGameModalSelectionHelpUnique(),
		locales.AddGameModalSelectionHelpDefault(),
	}
	ui.PopoverBase(c, btn, &s.helpOpen, func(pnl ui.Element) {
		pnl.AttachTo(btn, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(8).Width(min(320, winW-32)).Padding(16).Radius(12).
			Background(t.PanelBg).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		pnl.Children(func() {
			components.Text(c, locales.AddGameModalSelectionHelpTitle(), theme.TextSM, theme.WeightSemibold, t.TextStrong)
			// mt-3 space-y-2.5 text-sm leading-5
			ui.Column(c).Margin(12, 0, 0, 0).Gap(10).Children(func() {
				for i, step := range steps {
					ui.Row(c.Key(i)).Gap(10).AlignItems(ui.Start).Children(func() {
						ui.Box(c).Size(20, 20).Radius(999).Center().Shrink(0).Background(t.Pick(theme.Brand100, theme.Brand700)).Children(func() {
							components.Text(c, fmt.Sprint(i+1), theme.TextXS, theme.WeightSemibold, t.Pick(theme.Brand700, theme.Brand200))
						})
						components.Text(c, step, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).Grow(1).Basis(0).MinWidth(0)
					})
				}
			})
		})
	})
}

// cards：relative 容器里 flex snap-x gap-4 overflow-x-auto p-2 pb-6 pt-2 的卡片（隐藏滚动条），
// 能向前/向后滚动时两侧显示 BetterEdgeIconButton，每次滚动 max(宽度 - 96, 240)。
func (s *SearchResults) cards(c *ui.Context, t *theme.Theme, p SearchResultsProps) {
	winW, _ := c.Size()
	st := &s.scroll
	cardW := float32(144) // w-36 sm:w-40
	if winW >= 640 {
		cardW = 160
	}
	wrap := ui.Box(c.Key("results"))
	viewW := wrap.Bounds().W
	wrap.Children(func() {
		ui.ScrollHorizontal(c.Key("scroller")).FillWidth().TrackScroll(st).Children(func() {
			ui.Row(c).Gap(16).Padding(8, 8, 24, 8).AlignItems(ui.Start).Children(func() {
				for i, item := range p.Results {
					if item.Game.Name == "" && item.Game.SourceID == "" {
						continue // 原版跳过没有 Game 的结果
					}
					resultCard(c.Key(string(item.Source)+":"+item.Game.SourceID+fmt.Sprint(":", i)), t, p, item, i, cardW)
				}
			})
		})
		step := max(viewW-96, 240)
		if st.X > 2 {
			edgeButton(c, t, "prev", "chevron-left", locales.AddGameModalScrollResultsPrev(), true, func() { st.ScrollTo(st.X-step, 0, ui.Smooth) })
		}
		if st.X < st.MaxX-2 {
			edgeButton(c, t, "next", "chevron-right", locales.AddGameModalScrollResultsNext(), false, func() { st.ScrollTo(st.X+step, 0, ui.Smooth) })
		}
	})
}

// resultCard：relative w-36 shrink-0 rounded-xl border border-brand-200 bg-brand-50/50 shadow-sm hover:border-brand-400
// hover:shadow-md dark:border-brand-700 dark:bg-brand-800/50 dark:hover:border-brand-500。里面 p-3 的按钮：aspect-[3/4]
// rounded-md 的封面、mt-2 text-sm font-bold 的名称与 text-xs 的来源；右上角 -right-3 -top-3 的移除按钮。
func resultCard(c *ui.Context, t *theme.Theme, p SearchResultsProps, item vo.GameMetadataFromWebVO, index int, width float32) {
	card := ui.Column(c).Width(width).Shrink(0).Radius(12).Background(t.Pick(theme.Brand50.Alpha(0.5), theme.Brand800.Alpha(0.5)))
	hovered := card.Hovered()
	border := t.Pick(theme.Brand200, theme.Brand700)
	if hovered {
		border = t.Pick(theme.Brand400, theme.Brand500)
		card.Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
	} else {
		card.Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 1, 2, -1, ui.RGBA(0, 0, 0, 0.1))
	}
	card.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
	source := SourceLabel(item.Source)
	card.Children(func() {
		btn := ui.ButtonBase(c.Key("select")).Column().FillWidth().Padding(12).AlignItems(ui.Stretch).FocusRing(false).
			Cursor(ui.CursorPointer).Label(item.Game.Name)
		if p.Disabled {
			components.Disable(btn, 0.6)
		}
		btn.OnClick(func() {
			if p.OnSelect != nil {
				p.OnSelect(item)
			}
		}).Children(func() {
			ui.Box(c).FillWidth().AspectRatio(3.0 / 4).Radius(6).Clip().Center().Background(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
				coverImage(c, p.Covers, item.Game, 36)
			})
			components.TextMargin(c, item.Game.Name, theme.TextSM, theme.WeightBold, t.TextStrong, 8, 0, 0, 0).SingleLine()
			components.Text(c, locales.AddGameModalFromSource(source), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).SingleLine()
		})
		// group absolute -right-3 -top-3 z-10 flex h-11 w-11：里面 h-8 w-8 rounded-full border-2 border-white bg-brand-300/85
		// text-white shadow-md backdrop-blur-sm group-hover:bg-error-600 dark:border-brand-700 dark:bg-brand-900/90。
		rm := ui.ButtonBase(c.Key("remove")).Absolute().Top(-12).Right(-12).Size(44, 44).Center().FocusRing(false).
			Label(locales.AddGameModalRemoveResult(source))
		if p.Disabled {
			components.Disable(rm, 0.6)
		}
		bg := t.Pick(theme.Brand300.Alpha(0.85), theme.Brand900.Alpha(0.9))
		if rm.Hovered() && !p.Disabled {
			bg = theme.Error600
		}
		rm.OnClick(func() {
			if p.OnRemove != nil {
				p.OnRemove(index)
			}
		}).Children(func() {
			ui.Box(c).Size(32, 32).Radius(999).Center().Background(bg).Border(2, t.Pick(theme.White, theme.Brand700)).
				Material(glass.Blur{Radius: 4}).Transition(theme.ColorsTransition(150*time.Millisecond)).
				Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1)).Children(func() {
				components.Icon(c, "close", 18, theme.White)
			})
		})
	})
}

// edgeButton 是 absolute top-1/2 -translate-y-1/2 的 BetterEdgeIconButton placement="left|right"。
func edgeButton(c *ui.Context, t *theme.Theme, key, icon, label string, left bool, onClick func()) {
	holder := ui.Column(c.Key(key)).Absolute().Top(0).Bottom(0).Justify(ui.Center).PassThrough()
	placement := better.EdgeRight
	if left {
		holder.Left(0)
		placement = better.EdgeLeft
	} else {
		holder.Right(0)
	}
	holder.Children(func() {
		better.EdgeIconButton(c, t, "btn", better.EdgeIconButtonProps{Placement: placement, Icon: icon, Label: label, OnClick: onClick})
	})
}

// coverImage 是候选结果的封面：没有封面地址时 i-mdi-image-off，加载失败同样显示图标。
func coverImage(c *ui.Context, cache *covers.Cache, g models.Game, iconSize float32) {
	switch {
	case g.CoverURL == "" && g.CoverSourceURL == "":
		components.Icon(c, "image-off", iconSize, theme.Brand400)
	case cache == nil:
	default:
		if bmp := cache.Get(&g); bmp != nil {
			ui.Image(c, bmp).Fill().Fit(ui.Cover)
		} else if cache.Missing(&g) {
			components.Icon(c, "image-off", iconSize, theme.Brand400)
		}
	}
}
