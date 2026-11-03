package stats

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// html2canvas 1.4.1（MIT）：原版截取预览 iframe 用的库，这里随程序打包，导出时注入模板页面，不再从 CDN 加载。
//
//go:embed html2canvas.min.js
var html2canvasJS string

// previewBaseURL 是模板页面的地址。没有地址时 Windows 用 NavigateToString 加载，超过 2MB（带 Base64 封面的
// 模板很容易超过）就加载失败；*.localhost 的 https 地址由 WebView 自己提供，也不是应用信任的来源。
const previewBaseURL = "https://templates.lunabox.localhost/"

const captureTimeout = time.Minute

// captureScript 对应 handleExport 的模板分支：等字体加载完、滚回顶部，用 html2canvas 按 2 倍把整个 body 画成 PNG。
var captureScript = "if (!window.html2canvas) {\n" + html2canvasJS + "\n}\n" + `
try { await document.fonts.ready } catch { await new Promise(r => setTimeout(r, 1000)) }
const target = document.body
window.scrollTo(0, 0)
document.documentElement.scrollTop = 0
target.scrollTop = 0
await new Promise(r => requestAnimationFrame(r))
const canvas = await window.html2canvas(target, {
  backgroundColor: null, scale: 2, useCORS: true, allowTaint: true, logging: false,
  x: 0, y: 0, scrollX: 0, scrollY: 0,
  windowWidth: target.scrollWidth, windowHeight: target.scrollHeight,
  onclone: doc => {
    doc.documentElement.scrollTop = 0
    doc.body.scrollTop = 0
    const style = doc.createElement("style")
    style.textContent = "* { -webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale; } html, body { margin: 0 !important; padding: 0 !important; }"
    doc.head.appendChild(style)
  },
})
return canvas.toDataURL("image/png")`

// exporter 是“美化导出”弹窗（TemplateExportModal）的状态：模板在 WebView 里预览，导出时用 html2canvas 截成 PNG。
// 封面拼贴（cover_mosaic、MosaicPosterPanel）还没有迁移，列表里只有 HTML 模板。
type exporter struct {
	open      bool
	templates []vo.TemplateInfo
	selected  string // selectedTemplateId，关闭弹窗后保留
	html      string // previewHtml
	loading   bool
	exporting bool
	gen       int // 每次关闭加一，丢弃关闭前还没返回的结果
	seq       int // 每次渲染加一，只采用最新一次渲染的结果（requestId）

	// web 是预览模板的 WebView，打开弹窗时创建、关闭时销毁。没有窗口（测试）或系统缺少 WebView 时为 nil。
	// 模板来自用户目录，页面设为 Untrusted：不能调用 Go 方法，也拿不到权限。
	web  *mygo.WebView
	list components.ScrollBar
}

// openExport 打开弹窗：加载模板列表并渲染选中的模板（原版 isOpen 变化触发的两个 useEffect）。
func (p *Page) openExport() {
	x := &p.export
	x.open, x.list = true, components.ScrollBar{}
	if x.web == nil && p.env.Win != nil && mygo.WebViewAvailable() {
		web, err := p.env.Win.NewWebView(mygo.WebViewOptions{BackgroundColor: "#ffffff", Page: mygo.PageOptions{Untrusted: true}})
		if err != nil {
			log.Printf("template export: %v", err)
		} else {
			x.web = web
		}
	}
	p.loadTemplates()
	p.renderTemplate()
}

// closeExport 关闭弹窗并销毁 WebView（它的页面进程很占内存）。
func (p *Page) closeExport() {
	x := &p.export
	x.open, x.html, x.loading, x.exporting = false, "", false, false
	x.gen++
	x.seq++
	if x.web != nil {
		x.web.Destroy()
		x.web = nil
	}
}

// loadTemplates 对应 loadTemplates：内置模板与模板目录里的自定义模板；还没有选中时选第一个。
func (p *Page) loadTemplates() {
	x := &p.export
	gen := x.gen
	p.env.Async(func() func() {
		list, err := p.env.Svc.Template.ListTemplates()
		return func() {
			if gen != x.gen {
				return
			}
			if err != nil {
				log.Printf("template export: list templates: %v", err)
				p.toast(func() env.Notice { return env.Error(locales.StatsTemplateExportToastLoadTemplatesFailed()) })
				return
			}
			x.templates = list
			if x.selected == "" && len(list) > 0 {
				p.selectTemplate(list[0].ID)
			}
		}
	})
}

func (p *Page) selectTemplate(id string) {
	x := &p.export
	if x.exporting || id == x.selected {
		return
	}
	x.selected = id
	p.renderTemplate()
}

// renderTemplate 对应 renderPreview：后端准备数据（封面转成 Base64）并渲染模板，再交给 WebView。
// 失败时保留上一次的预览。
func (p *Page) renderTemplate() {
	x := &p.export
	if p.stats == nil || x.selected == "" {
		return
	}
	x.seq++
	x.loading = true
	seq, stats, summary, id := x.seq, *p.stats, p.aiSummary[p.period], x.selected
	p.env.Async(func() func() {
		data, err := p.env.Svc.Template.PrepareExportData(stats, summary)
		var resp vo.RenderTemplateResponse
		if err == nil {
			resp, err = p.env.Svc.Template.RenderTemplate(vo.RenderTemplateRequest{TemplateID: id, Data: data})
		}
		return func() {
			if seq != x.seq {
				return
			}
			x.loading = false
			if err != nil {
				log.Printf("template export: render %s: %v", id, err)
				p.toast(func() env.Notice { return env.Error(locales.StatsTemplateExportToastRenderFailed()) })
				return
			}
			x.html = resp.HTML
			if x.web != nil {
				x.web.Page().LoadHTML(x.html, previewBaseURL)
			}
		}
	})
}

// exportImage 对应 handleExport：在模板页面里截图，保存对话框选好位置后关闭弹窗。
func (p *Page) exportImage() {
	x := &p.export
	web, gen := x.web, x.gen
	if web == nil || x.html == "" || x.loading || x.exporting {
		return
	}
	x.exporting = true
	p.env.Async(func() func() {
		ctx, cancel := context.WithTimeout(context.Background(), captureTimeout)
		defer cancel()
		res, err := web.Page().EvalContext(ctx, captureScript)
		data, _ := res.(string)
		if err == nil && data == "" {
			err = errors.New("html2canvas returned no image")
		}
		if err == nil {
			err = p.env.Svc.Template.ExportRenderedHTML(data)
		}
		return func() {
			if gen != x.gen {
				return
			}
			x.exporting = false
			if err != nil {
				log.Printf("template export: export image: %v", err)
				msg := err.Error()
				p.toast(func() env.Notice { return env.Error(locales.StatsTemplateExportToastExportFailed(msg)) })
				return
			}
			p.closeExport()
		}
	})
}

// openTemplatesDir 对应 handleOpenTemplatesDir。
func (p *Page) openTemplatesDir() {
	p.env.Async(func() func() {
		err := p.env.Svc.Template.OpenTemplatesDir()
		return func() {
			if err != nil {
				log.Printf("template export: open templates dir: %v", err)
				p.toast(func() env.Notice { return env.Error(locales.StatsTemplateExportToastOpenDirFailed()) })
			}
		}
	})
}

func (x *exporter) selectedTemplate() (vo.TemplateInfo, bool) {
	for _, tmpl := range x.templates {
		if tmpl.ID == x.selected {
			return tmpl, true
		}
	}
	return vo.TemplateInfo{}, false
}

// exportModal 对应 TemplateExportModal：w-full max-w-6xl h-[85vh] rounded-xl bg-white shadow-xl dark:bg-brand-800 border
// 的面板，标题栏、左侧 w-64 的模板列表、右侧预览与底栏，之间是 border-brand-200 dark:border-brand-700 的细线。
// 原版点遮罩和按 Esc 都不关闭。
func (p *Page) exportModal(c *ui.Context, t *theme.Theme) {
	x := &p.export
	_, winH := c.Size()
	line := t.Pick(theme.Brand200, theme.Brand700)
	components.Modal(c, t, "export-modal", components.ModalProps{
		Open: x.open, TopOffset: env.TopbarHeight, MaxWidth: 1152, Label: locales.StatsTemplateExportTitle(),
	}, func(panel ui.Element) {
		panel.Height(winH*0.85).Radius(12).Clip().Background(t.Pick(theme.White, theme.Brand800)).Border(t.HL, line).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1)).Children(func() {
			p.exportHeader(c, t)
			ui.Box(c).Height(t.HL).Background(line).Shrink(0)
			ui.Row(c.Key("body")).Grow(1).Basis(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				p.templateList(c, t, line)
				ui.Box(c).Width(t.HL).AlignSelf(ui.Stretch).Background(line).Shrink(0)
				p.templatePreview(c, t, line)
			})
			ui.Box(c).Height(t.HL).Background(line).Shrink(0)
			p.exportFooter(c, t)
		})
	})
}

// exportHeader：flex items-center justify-between px-6 py-4；i-mdi-image-filter-hdr text-2xl text-neutral-600
// dark:text-neutral-400 与 text-xl font-bold 的标题（gap-3），关闭按钮 p-2 rounded-lg 的 i-mdi-close text-xl。
func (p *Page) exportHeader(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("header")).Padding(16, 24).AlignItems(ui.Center).Justify(ui.SpaceBetween).Shrink(0).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).MinWidth(0).Children(func() {
			components.Icon(c, "image-filter-hdr", 24, t.Pick(theme.Neutral600, theme.Neutral400))
			components.Text(c, locales.StatsTemplateExportTitle(), theme.TextXL, theme.WeightBold, t.Pick(theme.Brand900, theme.White)).SingleLine()
		})
		// hover:bg-brand-100 dark:hover:bg-brand-700 transition-colors；图标 text-brand-600 dark:text-brand-400
		x := ui.ButtonBase(c.Key("close")).Padding(8).Radius(8).FocusRing(false).Label(locales.CommonClose())
		if x.Hovered() {
			x.Background(t.Pick(theme.Brand100, theme.Brand700))
		}
		x.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(p.closeExport).Children(func() {
			components.Icon(c, "close", 20, t.Pick(theme.Brand600, theme.Brand400))
		})
	})
}

// templateList 是左侧 w-64 flex flex-col：p-4 的标题与说明，overflow-y-auto p-2 space-y-1 的模板列表，
// p-3 的“打开模板目录”，之间 border-b/border-t。
func (p *Page) templateList(c *ui.Context, t *theme.Theme, line ui.Color) {
	x := &p.export
	muted := t.Pick(theme.Brand500, theme.Brand400)
	ui.Column(c.Key("templates")).Width(256).Shrink(0).Children(func() {
		ui.Column(c).Padding(16).Children(func() {
			components.TextMargin(c, locales.StatsTemplateExportSelectTemplate(), theme.TextSM, theme.WeightSemibold,
				t.Pick(theme.Brand900, theme.White), 0, 0, 4, 0)
			components.Text(c, locales.StatsTemplateExportSelectTemplateHint(), theme.TextXS, theme.WeightNormal, muted)
		})
		ui.Box(c).Height(t.HL).Background(line).Shrink(0)
		components.ScrollArea(c, t, "list", &x.list, func(w ui.Element) { w.FillWidth().Grow(1).Basis(0).MinHeight(0) }, func(sc ui.Element) {
			gutter := float32(0)
			if x.list.Overflowing() {
				gutter = t.Gutter()
			}
			sc.Padding(8, 8+gutter, 8, 8).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					for _, tmpl := range x.templates {
						p.templateItem(c.Key(tmpl.ID), t, tmpl)
					}
				})
			})
		})
		ui.Box(c).Height(t.HL).Background(line).Shrink(0)
		ui.Column(c).Padding(12).Children(func() {
			// w-full flex items-center justify-center gap-2 px-3 py-2 text-sm text-brand-600 dark:text-brand-400
			// hover:bg-brand-50 dark:hover:bg-brand-700/50 rounded-lg
			btn := ui.ButtonBase(c.Key("open-dir")).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Radius(8).FocusRing(false)
			if btn.Hovered() {
				btn.Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.5)))
			}
			fg := t.Pick(theme.Brand600, theme.Brand400)
			btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(p.openTemplatesDir).Children(func() {
				components.Icon(c, "folder-open", 14, fg)
				components.Text(c, locales.StatsTemplateExportOpenTemplatesDir(), theme.TextSM, theme.WeightNormal, fg).NoWrap()
			})
		})
	})
}

// templateItem：w-full text-left px-3 py-2.5 rounded-lg；选中 bg-neutral-100 dark:bg-neutral-900 text-neutral-700
// dark:text-neutral-300，否则 text-brand-700 dark:text-brand-300 hover:bg-brand-50 dark:hover:bg-brand-700/50。
// 内置模板 i-mdi-check-decagram text-neutral-500，自定义模板 i-mdi-file-document-outline text-brand-400；
// 名称 font-medium text-sm，说明 text-xs mt-1 ml-6 line-clamp-2。导出中不响应点击（原版 disabled，样式不变）。
func (p *Page) templateItem(c *ui.Context, t *theme.Theme, tmpl vo.TemplateInfo) {
	x := &p.export
	btn := ui.ButtonBase(c).FillWidth().Padding(10, 12).Radius(8).FocusRing(false).Label(tmpl.Name)
	fg := t.Pick(theme.Brand700, theme.Brand300)
	switch {
	case tmpl.ID == x.selected:
		btn.Background(t.Pick(theme.Neutral100, theme.Neutral900))
		fg = t.Pick(theme.Neutral700, theme.Neutral300)
	case btn.Hovered():
		btn.Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.5)))
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { p.selectTemplate(tmpl.ID) }).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				icon, color := "file-document-outline", theme.Brand400
				if tmpl.IsBuiltin {
					icon, color = "check-decagram", theme.Neutral500
				}
				components.Icon(c, icon, 16, color)
				components.Text(c, tmpl.Name, theme.TextSM, theme.WeightMedium, fg).Grow(1).Basis(0).MinWidth(0)
			})
			if tmpl.Description != "" {
				components.TextMargin(c, tmpl.Description, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 24).
					MaxLines(2)
			}
		})
	})
}

// templatePreview 是右侧 flex-1 flex flex-col bg-brand-50 dark:bg-brand-900/50：px-4 py-3 border-b bg-white
// dark:bg-brand-800 的模板信息，flex-1 p-4 的预览（rounded-lg shadow-lg bg-white 的页面、加载中或提示）。
func (p *Page) templatePreview(c *ui.Context, t *theme.Theme, line ui.Color) {
	x := &p.export
	muted := t.Pick(theme.Brand500, theme.Brand400)
	ui.Column(c.Key("preview")).Grow(1).Basis(0).MinWidth(0).Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.5))).Children(func() {
		if tmpl, ok := x.selectedTemplate(); ok {
			ui.Column(c.Key("info")).Padding(12, 16).Background(t.Pick(theme.White, theme.Brand800)).Children(func() {
				components.Text(c, tmpl.Name, theme.TextBase, theme.WeightSemibold, t.Pick(theme.Brand900, theme.White)).SingleLine()
				meta := ""
				if tmpl.Author != "" {
					meta = locales.StatsTemplateExportAuthorPrefix() + " " + tmpl.Author + " · "
				}
				meta += locales.StatsTemplateExportVersionPrefix() + " " + tmpl.Version
				if tmpl.IsBuiltin {
					meta += locales.StatsTemplateExportBuiltinTag()
				}
				components.Text(c, meta, theme.TextXS, theme.WeightNormal, muted).SingleLine()
			})
			ui.Box(c).Height(t.HL).Background(line).Shrink(0)
		}
		area := ui.Column(c.Key("area")).Grow(1).Basis(0).MinHeight(0).Padding(16)
		switch {
		case x.web == nil:
			area.Center().Children(func() {
				components.Text(c, locales.NativeTemplateExportWebViewUnavailable(), theme.TextSM, theme.WeightNormal, muted).
					MaxWidth(480).TextAlign(ui.Center)
			})
		case x.loading:
			area.Center().Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					components.Spinner(c, 24, muted)
					components.Text(c, locales.StatsTemplateExportRendering(), theme.TextBase, theme.WeightNormal, muted)
				})
			})
		case x.html != "":
			area.Children(func() {
				ui.WebView(c, x.web).Grow(1).Radius(8).Background(theme.White).
					Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
			})
		default:
			area.Center().Children(func() {
				components.Text(c, locales.StatsTemplateExportSelectToPreview(), theme.TextBase, theme.WeightNormal, muted)
			})
		}
	})
}

// exportFooter：flex items-center justify-between px-6 py-4 bg-brand-50 dark:bg-brand-900/30；左侧 text-xs 的提示
// （i-mdi-information-outline mr-1），右侧 gap-3 的取消与导出按钮。
func (p *Page) exportFooter(c *ui.Context, t *theme.Theme) {
	x := &p.export
	muted := t.Pick(theme.Brand500, theme.Brand400)
	ui.Row(c.Key("footer")).Padding(16, 24).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Shrink(0).
		Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.3))).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).MinWidth(0).Children(func() {
			components.Icon(c, "information-outline", 12, muted)
			components.Text(c, locales.StatsTemplateExportTipText(), theme.TextXS, theme.WeightNormal, muted).SingleLine()
		})
		ui.Row(c).Gap(12).Shrink(0).Children(func() {
			components.ModalTextButton(c, t, "cancel", locales.StatsTemplateExportCancelBtn()).OnClick(p.closeExport)
			p.exportButton(c, x)
		})
	})
}

// exportButton：flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-neutral-600 hover:bg-neutral-700
// rounded-lg shadow-sm disabled:opacity-50；导出中显示 i-mdi-loading animate-spin，否则 i-mdi-download。
func (p *Page) exportButton(c *ui.Context, x *exporter) {
	enabled := x.web != nil && x.html != "" && !x.loading && !x.exporting
	btn := ui.ButtonBase(c.Key("export")).Padding(8, 16).Gap(8).AlignItems(ui.Center).Radius(8).FocusRing(false)
	bg := theme.Neutral600
	if enabled && btn.Hovered() {
		bg = theme.Neutral700
	}
	shadowSM(btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)))
	if !enabled {
		components.Disable(btn, 0.5)
	}
	label := locales.StatsTemplateExportExportBtn()
	if x.exporting {
		label = locales.StatsTemplateExportExporting()
	}
	btn.Label(label).OnClick(p.exportImage).Children(func() {
		if x.exporting {
			components.Spinner(c, 14, theme.White)
		} else {
			components.Icon(c, "download", 14, theme.White)
		}
		components.Text(c, label, theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
}
