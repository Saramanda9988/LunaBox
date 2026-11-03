package modal

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel/edit"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// InstallConfirm 是 InstallConfirmModal 的状态：App.tsx 的 installRequest 与组件自己的 loading。根布局持有它，
// lunabox:// 安装请求（install:pending、GetPendingInstall）用 Show 打开。
type InstallConfirm struct {
	req     *vo.InstallRequest
	loading bool
}

// Show 对应 setInstallRequest(req)：显示这个请求，替换正在显示的请求。
func (m *InstallConfirm) Show(req vo.InstallRequest) { m.req, m.loading = &req, false }

// Request 是正在显示的请求，没有时为 nil。
func (m *InstallConfirm) Request() *vo.InstallRequest { return m.req }

// InstallConfirmProps 是 InstallConfirmModal 的属性。
type InstallConfirmProps struct {
	Env       *env.Env
	TopOffset float32
}

// installSourceLabels 是 META_SOURCE_LABELS；没有的来源显示原名。
func installSourceLabel(source string) string {
	switch enums.SourceType(source) {
	case enums.VNDB:
		return "VNDB"
	case enums.Bangumi:
		return "Bangumi"
	case enums.Ymgal:
		return locales.GameEditSourceYmgal()
	case enums.Steam:
		return "Steam"
	case enums.DLsite:
		return "DLsite"
	case enums.TouchGal:
		return "TouchGAL"
	case enums.Hikarinagi:
		return "Hikarinagi"
	case enums.Kungal:
		return locales.GameEditSourceKungal()
	case enums.ErogameScape:
		return "ErogameScape"
	}
	return source
}

// formatInstallSize 是 InstallConfirmModal.tsx 的 formatSize：GB 两位小数、MB 一位、KB 取整。
func formatInstallSize(bytes int64) string {
	const kb, mb, gb = 1024.0, 1024.0 * 1024, 1024.0 * 1024 * 1024
	b := float64(bytes)
	switch {
	case bytes <= 0:
		return ""
	case b >= gb:
		return fmt.Sprintf("%.2f GB", math.Floor(b/gb*100+0.5)/100)
	case b >= mb:
		return fmt.Sprintf("%.1f MB", math.Floor(b/mb*10+0.5)/10)
	}
	return fmt.Sprintf("%.0f KB", math.Floor(b/kb+0.5))
}

// InstallConfirmView 对应 components/modal/InstallConfirmModal.tsx：bg-black/60 backdrop-blur-sm 的遮罩上 w-full max-w-md
// rounded-xl border shadow-2xl overflow-hidden 的面板，依次是标题、游戏信息、警告与取消、确认下载按钮。遮罩不关闭弹窗。
// 要在页面最后构建，浮层才会盖住页面内容。
func InstallConfirmView(c *ui.Context, t *theme.Theme, m *InstallConfirm, p InstallConfirmProps) {
	req := m.req
	if req == nil {
		return
	}
	onClose := func() { m.req, m.loading = nil, false }
	confirm := func() {
		if m.loading || p.Env.Svc == nil {
			return
		}
		m.loading = true
		e, r := p.Env, *req
		e.Async(func() func() {
			_, err := e.Svc.Download.StartDownload(r)
			return func() {
				if err != nil {
					// 原版不处理 StartDownload 的错误（只关闭弹窗）；这里提示原因。
					log.Printf("install: start download: %v", err)
					e.Toast(c, env.Error(err.Error()))
				}
				if m.req == req {
					onClose()
				}
			}
		})
	}
	muted := t.Pick(theme.Brand500, theme.Brand400)
	components.Modal(c, t, "install-confirm", components.ModalProps{Open: true, TopOffset: p.TopOffset, Dim: 0.6}, func(panel ui.Element) {
		panel.Radius(12).Clip().Background(t.PanelBg).Border(t.HL, t.PanelBorder).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
			// Header：flex items-center gap-3 px-6 pt-6 pb-4
			ui.Row(c.Key("header")).AlignItems(ui.Center).Gap(12).Padding(24, 24, 16, 24).Children(func() {
				// p-2.5 rounded-xl bg-primary-100 dark:bg-primary-900/30 的 i-mdi-download-circle text-2xl
				ui.Box(c).Padding(10).Radius(12).Shrink(0).Background(t.Pick(theme.Primary100, theme.Primary900.Alpha(0.3))).Children(func() {
					components.Icon(c, "download-circle", 24, t.Pick(theme.Primary600, theme.Primary400))
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					// text-lg font-bold leading-tight
					components.Text(c, locales.InstallModalTitle(), theme.FontStep{Size: 18, Line: 22.5}, theme.WeightBold, t.TextStrong)
					components.TextMargin(c, locales.InstallModalSubtitle(), theme.TextSM, theme.WeightNormal, muted, 2, 0, 0, 0)
				})
			})
			installInfo(c, t, req, p)
			installWarning(c, t, req)
			// Actions：flex justify-end gap-3 px-6 pb-6
			ui.Row(c.Key("actions")).Justify(ui.End).Gap(12).Padding(0, 24, 24, 24).Children(func() {
				installCancelButton(c, t, m.loading).OnClick(func() {
					if !m.loading {
						onClose()
					}
				})
				installConfirmButton(c, t, m.loading).OnClick(confirm)
			})
		})
	})
}

// installInfo：mx-6 mb-5 rounded-lg bg-brand-50 dark:bg-brand-900/50 border p-4 space-y-2。每行 flex gap-2：
// w-14 shrink-0 的 text-xs 名称与 text-sm 的值。
func installInfo(c *ui.Context, t *theme.Theme, req *vo.InstallRequest, p InstallConfirmProps) {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	plain := t.Pick(theme.Brand700, theme.Brand300)
	link := t.Pick(theme.Primary600, theme.Primary400)
	row := func(key, label string, top bool, value func()) {
		r := ui.Row(c.Key(key)).Gap(8).AlignItems(ui.Center)
		if top {
			r.AlignItems(ui.Start)
		}
		r.Children(func() {
			l := components.Text(c, label, theme.TextXS, theme.WeightNormal, muted).Width(56).Shrink(0)
			if top {
				l.Margin(2, 0, 0, 0) // pt-0.5
			}
			value()
		})
	}
	ui.Column(c.Key("info")).Margin(0, 24, 20, 24).Padding(16).Gap(8).Radius(8).
		Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.5))).Border(t.HL, t.PanelBorder).Children(func() {
		row("name", locales.InstallModalName(), true, func() {
			title := req.Title
			if title == "" {
				title = locales.InstallModalUnknown()
			}
			// text-sm font-semibold break-all leading-snug
			components.Text(c, title, theme.FontStep{Size: 14, Line: 19.25}, theme.WeightSemibold, t.TextStrong).
				Grow(1).Basis(0).MinWidth(0)
		})
		if req.DownloadSource != "" {
			row("source", locales.InstallModalSource(), false, func() {
				components.Text(c, req.DownloadSource, theme.TextSM, theme.WeightMedium, link)
			})
		}
		if req.MetaSource != "" && req.MetaID != "" {
			erogameScape := ""
			if cfg := p.Env.Config(); cfg != nil {
				erogameScape = cfg.ErogameScapeBaseURL
			}
			href := edit.SourceURL(enums.SourceType(req.MetaSource), req.MetaID, erogameScape)
			row("meta", installSourceLabel(req.MetaSource), false, func() {
				if href == "" {
					components.Text(c, req.MetaID, theme.TextSM, theme.WeightNormal, plain).Font(monoFont)
					return
				}
				// cursor-pointer border-0 bg-transparent p-0 text-sm text-primary-600 font-mono hover:underline
				btn := ui.ButtonBase(c.Key("meta-link")).FocusRing(false).OnClick(func() { c.OpenURL(href) })
				btn.Children(func() {
					txt := components.Text(c, req.MetaID, theme.TextSM, theme.WeightNormal, link).Font(monoFont)
					if btn.Hovered() {
						txt.Underline()
					}
				})
			})
		}
		if req.Size > 0 {
			row("size", locales.InstallModalSize(), false, func() {
				components.Text(c, formatInstallSize(req.Size), theme.TextSM, theme.WeightNormal, plain).Font(monoFont)
			})
		}
	})
}

// installWarning：mx-6 mb-5 flex items-start gap-2 text-xs text-amber-700 dark:text-amber-400 bg-amber-50
// dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/50 rounded-lg px-3 py-2.5，i-mdi-alert-outline mt-0.5。
// 没有 checksum_algo 与 checksum 时提示无法校验。
func installWarning(c *ui.Context, t *theme.Theme, req *vo.InstallRequest) {
	msg := locales.InstallModalWarning()
	if req.ChecksumAlgo == "" && req.Checksum == "" {
		msg = locales.InstallModalMissingChecksumWarning()
	}
	fg := t.Pick(theme.Warning700, theme.Warning400)
	ui.Row(c.Key("warning")).Margin(0, 24, 20, 24).Padding(10, 12).Gap(8).AlignItems(ui.Start).Radius(8).
		Background(t.Pick(theme.Warning50, theme.Warning900.Alpha(0.2))).Border(t.HL, t.Pick(theme.Warning200, theme.Warning800.Alpha(0.5))).
		Children(func() {
			components.Icon(c, "alert-outline", 12, fg).Margin(2, 0, 0, 0).Shrink(0)
			components.Text(c, msg, theme.TextXS, theme.WeightNormal, fg).Grow(1).Basis(0).MinWidth(0)
		})
}

// installCancelButton：px-4 py-2 text-sm rounded-lg border border-brand-300 dark:border-brand-600 text-brand-700
// dark:text-brand-300 hover:bg-brand-100 dark:hover:bg-brand-700 disabled:opacity-50。
func installCancelButton(c *ui.Context, t *theme.Theme, loading bool) ui.Element {
	btn := ui.ButtonBase(c.Key("cancel")).Padding(8, 16).Radius(8).FocusRing(false).
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	if btn.Hovered() && !loading {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
	}
	if loading {
		components.Disable(btn, 0.5)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Text(c, locales.CommonCancel(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand300)).NoWrap()
	})
	return btn
}

// installConfirmButton：px-4 py-2 text-sm rounded-lg bg-primary-600 hover:bg-primary-700 text-white font-medium flex
// items-center gap-2，下载开始前显示 i-mdi-loading animate-spin，disabled:opacity-50。
func installConfirmButton(c *ui.Context, t *theme.Theme, loading bool) ui.Element {
	btn := ui.ButtonBase(c.Key("confirm")).Row().AlignItems(ui.Center).Gap(8).Padding(8, 16).Radius(8).FocusRing(false)
	bg := theme.Primary600
	if btn.Hovered() && !loading {
		bg = theme.Primary700
	}
	if loading {
		components.Disable(btn, 0.5)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		if loading {
			components.Spinner(c, 14, theme.White)
		}
		components.Text(c, locales.InstallModalConfirm(), theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}
