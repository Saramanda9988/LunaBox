package importer

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// loadingStep 对应 ImportTaskLoadingStep：py-12 text-center，text-5xl 的旋转图标（mb-4），text-lg 的标题，
// mt-2 text-sm 的副标题。
func loadingStep(c *ui.Context, t *theme.Theme, color ui.Color, title, subtitle string) {
	ui.Column(c.Key("loading-step")).Padding(48, 0).AlignItems(ui.Center).Children(func() {
		components.Spinner(c, 48, color).Margin(0, 0, 16, 0)
		components.Text(c, title, theme.TextLG, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
		if subtitle != "" {
			components.TextMargin(c, subtitle, theme.TextSM, theme.WeightNormal, t.TextFaint, 8, 0, 0, 0)
		}
	})
}

// progressStep 对应 ImportMatchProgressStep：loadingStep 的样式，加上进度 current / total、当前游戏名、
// mx-auto mt-4 h-2 w-full max-w-md rounded-full 的进度条、mt-4 text-xs 的提示与可选的停止按钮。
func progressStep(c *ui.Context, t *theme.Theme, color ui.Color, title, hint string, p Progress, stopLabel string, onStop func()) {
	ui.Column(c.Key("progress-step")).Padding(48, 0).AlignItems(ui.Center).Children(func() {
		components.Spinner(c, 48, color).Margin(0, 0, 16, 0)
		components.Text(c, title, theme.TextLG, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
		components.TextMargin(c, strconv.Itoa(p.Current)+" /"+strconv.Itoa(p.Total), theme.TextSM, theme.WeightNormal, t.TextFaint, 8, 0, 0, 0)
		components.TextMargin(c, p.Name, theme.TextSM, theme.WeightNormal, theme.Neutral500, 8, 0, 0, 0)
		frac := float32(0)
		if p.Total > 0 {
			frac = float32(p.Current) / float32(p.Total)
		}
		bar := ui.Box(c.Key("bar")).FillWidth().MaxWidth(448).Height(8).Margin(16, 0, 0, 0).Radius(999).Background(t.Pick(theme.Brand200, theme.Brand700))
		w := bar.AnimateWith("w", frac, 300*time.Millisecond, theme.EaseTailwind)
		bar.Children(func() {
			ui.Box(c).Height(8).Radius(999).Width(bar.Bounds().W * w).Background(color)
		})
		components.TextMargin(c, hint, theme.TextXS, theme.WeightNormal, theme.Brand400, 16, 0, 0, 0)
		if onStop != nil && stopLabel != "" {
			btn := ui.ButtonBase(c.Key("stop")).Margin(16, 0, 0, 0).FocusRing(false)
			fg := t.Pick(theme.Brand500, theme.Brand400)
			if btn.Hovered() {
				fg = t.Pick(theme.Brand700, theme.Brand400)
			}
			btn.OnClick(onStop).Children(func() { components.Text(c, stopLabel, theme.TextSM, theme.WeightNormal, fg) })
		}
	})
}

// statCard 是导入弹窗里 flex-1 rounded-lg p-4 text-center 的统计卡片：可选的 text-3xl 图标（mb-2），
// 数字（text-3xl 或 text-2xl font-bold）与 text-sm 的说明。
type statCard struct {
	value                int
	labelStr             string
	icon                 string
	bg, valueFg, labelFg ui.Color
	small                bool // 结果页的 text-2xl
}

func statCards(c *ui.Context, cards []statCard) {
	ui.Row(c.Key("stats")).Gap(16).FillWidth().Children(func() {
		for i, s := range cards {
			ui.Column(c.Key(i)).Grow(1).Basis(0).MinWidth(0).Padding(16).Radius(8).AlignItems(ui.Center).Background(s.bg).Children(func() {
				if s.icon != "" {
					components.Icon(c, s.icon, 30, s.valueFg).Margin(0, 0, 8, 0)
				}
				f := theme.Text3XL
				if s.small {
					f = theme.Text2XL
				}
				components.Text(c, strconv.Itoa(s.value), f, theme.WeightBold, s.valueFg)
				components.Text(c, s.labelStr, theme.TextSM, theme.WeightNormal, s.labelFg)
			})
		}
	})
}

// resultLabels 是 ImportResultStep 的 labels。
type resultLabels struct {
	success, skipped, failed, skippedGames, failedGames, complete string
}

// resultStep 对应 ImportResultStep（space-y-6）：成功、跳过、失败的统计卡片，跳过与失败的名称列表，
// 居中的完成按钮（rounded-lg px-8 py-2.5 text-sm font-medium text-white）。
func resultStep(c *ui.Context, t *theme.Theme, r service.ImportResult, l resultLabels, bg, hover ui.Color, onComplete func()) {
	ui.Column(c.Key("result-step")).Gap(24).Children(func() {
		cards := []statCard{{value: r.Success, labelStr: l.success, icon: "check-circle", small: true,
			bg: t.Pick(theme.Success50, theme.Success900.Alpha(0.2)), valueFg: t.Pick(theme.Success600, theme.Success400), labelFg: t.Pick(theme.Success700, theme.Success300)}}
		if r.Skipped > 0 {
			cards = append(cards, statCard{value: r.Skipped, labelStr: l.skipped, icon: "skip-next-circle", small: true,
				bg: t.Pick(theme.Yellow50, theme.Yellow900.Alpha(0.2)), valueFg: t.Pick(theme.Yellow600, theme.Yellow400), labelFg: t.Pick(theme.Yellow700, theme.Yellow300)})
		}
		if r.Failed > 0 {
			cards = append(cards, statCard{value: r.Failed, labelStr: l.failed, icon: "close-circle", small: true,
				bg: t.Pick(theme.Error50, theme.Error900.Alpha(0.2)), valueFg: t.Pick(theme.Error600, theme.Error400), labelFg: t.Pick(theme.Error700, theme.Error300)})
		}
		statCards(c, cards)
		if len(r.SkippedNames) > 0 {
			nameList(c, t, "skipped", l.skippedGames, r.SkippedNames, 150,
				t.Pick(theme.Yellow200, theme.Yellow800), t.Pick(theme.Yellow700, theme.Yellow400), t.Pick(theme.Yellow600, theme.Yellow300))
		}
		if len(r.FailedNames) > 0 {
			nameList(c, t, "failed", l.failedGames, r.FailedNames, 0,
				t.Pick(theme.Error200, theme.Error800), t.Pick(theme.Error700, theme.Error400), t.Pick(theme.Error600, theme.Error300))
		}
		ui.Row(c).Justify(ui.Center).Children(func() {
			solidButton(c, "complete", l.complete, bg, hover, false, false, 32, 10).OnClick(onComplete)
		})
	})
}

// nameList 是 rounded-lg border p-4 的名称列表：h4 mb-2 font-medium 的标题，space-y-1 text-sm 的“•名称”。
// maxH 不为 0 时列表可滚动（max-h-[150px] overflow-y-auto）。
func nameList(c *ui.Context, t *theme.Theme, key, title string, names []string, maxH float32, border, titleFg, fg ui.Color) {
	ui.Column(c.Key(key)).Padding(16).Radius(8).Border(t.HL, border).Children(func() {
		components.TextMargin(c, title, theme.TextBase, theme.WeightMedium, titleFg, 0, 0, 8, 0)
		list := func() {
			ui.Column(c).Gap(4).Children(func() {
				for i, n := range names {
					components.Text(c.Key(i), "•"+n, theme.TextSM, theme.WeightNormal, fg)
				}
			})
		}
		if maxH == 0 {
			list()
			return
		}
		ui.Scroll(c.Key("scroll")).MaxHeight(maxH).Children(list)
	})
}

// solidButton 是导入弹窗里 rounded-lg text-sm font-medium text-white 的实色按钮（disabled:opacity-50）。
func solidButton(c *ui.Context, key, label string, bg, hover ui.Color, disabled, fill bool, padX, padY float32) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(padY, padX).Radius(8).Center().FocusRing(false).Label(label)
	if fill {
		btn.FillWidth()
	}
	if btn.Hovered() && !disabled {
		bg = hover
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}

// outlineButton 是 rounded-lg border border-brand-300 text-sm font-medium text-brand-700 hover:bg-brand-100
// dark:border-brand-600 dark:text-brand-300 dark:hover:bg-brand-700 的描边按钮。
func outlineButton(c *ui.Context, t *theme.Theme, key, label string, disabled bool, padX, padY float32) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(padY, padX).Radius(8).Center().FocusRing(false).Label(label).
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	if btn.Hovered() && !disabled {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300)).NoWrap()
	})
	return btn
}
