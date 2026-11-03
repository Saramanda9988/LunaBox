// Package startup 是启动失败时显示的窗口，对应原版 components/startup/StartupWindow.tsx：
// 左边是图标与标题图，右边是错误标题、可复制的错误详情与“退出”按钮。
package startup

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// 窗口尺寸：原版启动错误窗固定 760x360。
const (
	Width  = 760
	Height = 360
)

const copiedFor = 1600 * time.Millisecond // copyError 里 setCopied(false) 的延迟

var (
	textTitle  = theme.FontStep{Size: 23, Line: 32}    // text-[23px] leading-8
	textDetail = theme.FontStep{Size: 13, Line: 20}    // text-[13px] leading-5
	textError  = theme.FontStep{Size: 11, Line: 18.15} // text-[11px] leading-[1.65]
)

// Page 是启动错误窗的状态。
type Page struct {
	Message string  // 启动错误；为空时显示 startup.unknownError
	Scale   float64 // 窗口所在显示器的缩放比例，决定 1px 边框的宽度；为 0 时按 1
	OnExit  func()  // “退出 LunaBox”

	scroll   components.ScrollBar
	copiedAt time.Time
}

// View 构建窗口：startup-backdrop flex h-screen w-screen，左侧 w-[282px] 的 aside，右侧 flex-1 的 section。
func (p *Page) View(c *ui.Context) {
	scale := p.Scale
	if scale <= 0 {
		scale = 1
	}
	t := theme.New(c.Theme().Dark, scale)
	msg := p.Message
	if msg == "" {
		msg = locales.StartupUnknownError()
	}
	c.Root().Background(t.Pick(theme.White, theme.Brand800))
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		p.aside(c, t)
		// flex min-w-0 flex-1 flex-col bg-white px-9 pb-7 pt-8 dark:bg-brand-800
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Padding(32, 36, 28, 36).
			Background(t.Pick(theme.White, theme.Brand800)).Children(func() {
			header(c, t)
			p.errorBox(c, t, msg)
			// mt-5 flex justify-end
			ui.Row(c).FillWidth().Margin(20, 0, 0, 0).Justify(ui.End).Children(func() {
				p.exitButton(c, t)
			})
		})
	})
}

// aside：relative flex w-[282px] shrink-0 items-center justify-center overflow-hidden border-r
// border-brand-200 bg-brand-100 dark:border-white/8 dark:bg-brand-900，两个装饰圆环，中间是图标与标题图。
func (p *Page) aside(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Width(282).Shrink(0).Center().Clip().Background(t.Pick(theme.Brand100, theme.Brand900)).
		BorderWidth(0, t.HL, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.White.Alpha(0.08))).Children(func() {
		// -left-30 -top-34 h-72 w-72 rounded-full border border-primary-300/25 dark:border-primary-400/12
		ui.Box(c).Absolute().Left(-120).Top(-136).Size(288, 288).Radius(999).PassThrough().
			Border(t.HL, t.Pick(theme.Primary300.Alpha(0.25), theme.Primary400.Alpha(0.12)))
		// -bottom-38 -right-32 h-72 w-72 rounded-full border border-primary-300/18 dark:border-primary-400/10
		ui.Box(c).Absolute().Right(-128).Bottom(-152).Size(288, 288).Radius(999).PassThrough().
			Border(t.HL, t.Pick(theme.Primary300.Alpha(0.18), theme.Primary400.Alpha(0.10)))
		// relative flex -translate-y-1 flex-col items-center gap-5
		ui.Column(c).AlignItems(ui.Center).Gap(20).Translate(0, -4).Children(func() {
			ui.Image(c, assets.AppIcon(t.Dark)).Size(96, 96).Radius(23)
			title := assets.Title(t.Dark)
			tw, th := title.Size()
			ui.Image(c, title).Size(152, 152*float32(th)/float32(tw)) // w-38 h-auto
		})
	})
}

// header：flex items-start gap-3.5，i-mdi-alert-circle-outline 与标题、说明。
func header(c *ui.Context, t *theme.Theme) {
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(14).Children(func() {
		components.Icon(c, "alert-circle-outline", 28, t.Pick(theme.Error500, theme.Error400)).Margin(2, 0, 0, 0).Shrink(0)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, locales.StartupTitle(), textTitle, theme.WeightSemibold, t.Pick(theme.Brand900, theme.White))
			components.TextMargin(c, locales.StartupDetail(), textDetail, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
		})
	})
}

// errorBox：mt-6 min-h-0 flex-1 里的 rounded-xl border 卡片，h-11 的标题栏（错误报告与复制按钮）与可滚动的错误详情。
func (p *Page) errorBox(c *ui.Context, t *theme.Theme, msg string) {
	border := t.Pick(theme.Brand200, theme.Brand700)
	ui.Column(c).FillWidth().Grow(1).Basis(0).MinHeight(0).Margin(24, 0, 0, 0).Radius(12).Clip().
		Border(t.HL, border).Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.7))).Children(func() {
		ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 16).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			BorderWidth(0, 0, t.HL, 0).BorderColor(border).Children(func() {
			components.Text(c, locales.StartupErrorSubtitle(), theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand500, theme.Brand400))
			p.copyButton(c, t, msg)
		})
		// pre：scrollbar-stable min-h-0 flex-1 overflow-auto whitespace-pre-wrap break-words px-4 py-3.5 font-mono
		// text-[11px] leading-[1.65] text-error-700 select-text dark:text-error-300
		components.ScrollArea(c, t, "startup-error", &p.scroll, func(w ui.Element) { w.FillWidth().Grow(1).Basis(0).MinHeight(0) }, func(sc ui.Element) {
			sc.Padding(14, 16+t.Gutter(), 14, 16).Children(func() {
				components.Text(c, msg, textError, theme.WeightNormal, t.Pick(theme.Error700, theme.Error300)).
					FillWidth().Font("Consolas").Selectable()
			})
		})
	})
}

// copyButton：grid h-8 w-8 place-items-center rounded-lg，复制后 1.6 秒内显示 i-mdi-check text-success-500。
func (p *Page) copyButton(c *ui.Context, t *theme.Theme, msg string) {
	copied := !p.copiedAt.IsZero() && c.Now().Sub(p.copiedAt) < copiedFor
	if copied {
		c.After(copiedFor - c.Now().Sub(p.copiedAt))
	}
	label := locales.StartupCopyError()
	if copied {
		label = locales.StartupCopied()
	}
	btn := ui.ButtonBase(c.Key("copy")).Size(32, 32).Radius(8).Center().Shrink(0).Label(label)
	fg := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand200, theme.Brand700))
		fg = t.Pick(theme.Brand800, theme.White)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() {
		c.WriteClipboard(msg)
		p.copiedAt = c.Now()
	}).Children(func() {
		if copied {
			components.Icon(c, "check", 16, theme.Success500)
		} else {
			components.Icon(c, "content-copy", 16, fg)
		}
	})
	better.Tooltip(c, t, btn, label)
}

// exitButton：shrink-0 rounded-lg bg-brand-900 px-4 py-2 text-xs font-medium text-white hover:bg-brand-700
// dark:bg-white dark:text-brand-900 dark:hover:bg-brand-200。
func (p *Page) exitButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("exit")).Padding(8, 16).Radius(8).Shrink(0)
	bg := t.Pick(theme.Brand900, theme.White)
	if btn.Hovered() {
		bg = t.Pick(theme.Brand700, theme.Brand200)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() {
		if p.OnExit != nil {
			p.OnExit()
		}
	}).Children(func() {
		components.Text(c, locales.StartupExit(), theme.TextXS, theme.WeightMedium, t.Pick(theme.White, theme.Brand900)).NoWrap()
	})
}
