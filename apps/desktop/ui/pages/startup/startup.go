// Package startup 是启动失败时显示的窗口，对应原版 components/startup/StartupWindow.tsx：
// 左边是图标与标题图，右边是错误标题、可复制的错误详情与“退出”按钮。
// 启动时执行设置页安排的恢复期间也用这个窗口显示恢复进度（Page.Restore），原版没有。
package startup

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
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
	// Restore 不为 nil 时窗口显示启动时恢复备份的进度，不显示错误与“退出”按钮。
	Restore *service.RestoreProgress

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
		components.BrandAside(c, t, 282, false) // aside：w-[282px]
		// flex min-w-0 flex-1 flex-col bg-white px-9 pb-7 pt-8 dark:bg-brand-800
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Padding(32, 36, 28, 36).
			Background(t.Pick(theme.White, theme.Brand800)).Children(func() {
			if p.Restore != nil {
				restoreHeader(c, t)
				restoreBox(c, t, *p.Restore)
				return
			}
			header(c, t)
			p.errorBox(c, t, msg)
			// mt-5 flex justify-end
			ui.Row(c).FillWidth().Margin(20, 0, 0, 0).Justify(ui.End).Children(func() {
				p.exitButton(c, t)
			})
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

// restoreHeader 与 header 相同的排版：i-mdi-backup-restore 与“正在恢复数据”、说明。
func restoreHeader(c *ui.Context, t *theme.Theme) {
	ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(14).Children(func() {
		components.Icon(c, "backup-restore", 28, t.Pick(theme.Primary500, theme.Primary400)).Margin(2, 0, 0, 0).Shrink(0)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, locales.NativeStartupRestoreTitle(), textTitle, theme.WeightSemibold, t.Pick(theme.Brand900, theme.White))
			components.TextMargin(c, locales.NativeStartupRestoreDetail(), textDetail, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
		})
	})
}

// restoreBox 与 errorBox 相同的卡片：标题栏是“恢复进度”与第几步，下面是阶段说明与百分比、进度条（进度未知时占满并闪烁）
// 和已处理的大小。
func restoreBox(c *ui.Context, t *theme.Theme, pr service.RestoreProgress) {
	border := t.Pick(theme.Brand200, theme.Brand700)
	muted := t.Pick(theme.Brand500, theme.Brand400)
	known := pr.Total > 0 && pr.Stage != service.RestoreStageDone
	ui.Column(c.Key("restore")).FillWidth().Grow(1).Basis(0).MinHeight(0).Margin(24, 0, 0, 0).Radius(12).Clip().
		Border(t.HL, border).Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.7))).Children(func() {
		ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 16).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			BorderWidth(0, 0, t.HL, 0).BorderColor(border).Children(func() {
			components.Text(c, locales.NativeStartupRestoreSubtitle(), theme.TextXS, theme.WeightMedium, muted)
			if pr.Steps > 0 && pr.Stage != service.RestoreStageDone {
				components.Text(c, locales.NativeStartupRestoreStep(strconv.Itoa(pr.Step), strconv.Itoa(pr.Steps)), theme.TextXS, theme.WeightNormal, muted).
					FontFeatures("tnum").NoWrap()
			}
		})
		ui.Column(c).FillWidth().Grow(1).Basis(0).MinHeight(0).Padding(0, 16).Justify(ui.Center).Gap(10).Children(func() {
			percent := 0
			if known {
				percent = int(min(100, max(0, pr.Done*100/pr.Total)))
			}
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				components.Text(c, restoreStageText(pr.Stage), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100)).
					SingleLine().Shrink(1).MinWidth(0)
				if known {
					components.Text(c, strconv.Itoa(percent)+"%", theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100)).
						Font("Consolas").NoWrap().Shrink(0)
				}
			})
			// 每个阶段一条进度条（按阶段区分元素），换阶段时从头开始，不会从上一阶段的 100% 倒退回去；
			// 进度未知（导入数据库、替换、正在启动）时是来回滑动的填充。
			value := -1.0
			if known {
				value = float64(max(2, percent)) / 100
			}
			ui.Column(c.Key("bar-" + string(pr.Stage))).FillWidth().Children(func() {
				components.ProgressBar(c, t, value, t.Pick(theme.Primary500, theme.Primary400))
			})
			size := ""
			if known {
				size = components.FormatFileSize(float64(pr.Done)) + " / " + components.FormatFileSize(float64(pr.Total))
			}
			// 进度未知时也占一行，阶段切换时卡片里的内容不上下跳动。
			components.Text(c, size, theme.TextXS, theme.WeightNormal, muted).FontFeatures("tnum").SingleLine().MinHeight(16)
		})
	})
}

func restoreStageText(stage service.RestoreStage) string {
	switch stage {
	case service.RestoreStageExtract:
		return locales.NativeStartupRestoreExtract()
	case service.RestoreStageDatabase:
		return locales.NativeStartupRestoreDatabase()
	case service.RestoreStageApply:
		return locales.NativeStartupRestoreApply()
	default:
		return locales.NativeStartupRestoreStarting()
	}
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
