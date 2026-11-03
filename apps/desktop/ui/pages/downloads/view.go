package downloads

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/card"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

const (
	pagePadding = 32   // p-8
	listMaxW    = 1024 // max-w-5xl
)

// View 构建页面：h-full w-full overflow-y-auto scrollbar-stable p-8，里面是 space-y-6 的标题与任务列表
// （外层 mx-auto max-w-8xl 在 UnoCSS 里没有定义，不限宽）。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	for _, n := range p.toasts {
		p.env.Toast(c, n())
	}
	p.toasts = nil

	gutter := t.Gutter()
	contentW := p.env.MainWidth - 2*pagePadding - gutter
	services := c.Services()
	components.ScrollArea(c, t, "downloads-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			ui.Column(c).Width(contentW).Gap(24).Children(func() {
				p.header(c, t)
				listW := min(listMaxW, contentW)
				gs := groups(p.tasks, c.Now(), timefmt.Location(p.env.Config()))
				if len(gs) == 0 {
					emptyState(c, t, listW)
					return
				}
				// mx-auto w-full max-w-5xl space-y-8
				ui.Column(c.Key("groups")).Width(listW).Margin(0, ui.Auto).Gap(32).Children(func() {
					for _, g := range gs {
						p.section(c, t, g, services)
					}
				})
			})
		})
	})
}

// header：h1 text-4xl font-bold text-brand-900 dark:text-white，下面 mt-1 text-sm text-brand-500 dark:text-brand-400
// 的进行中任务数。
func (p *Page) header(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("header")).Children(func() {
		components.Text(c, locales.DownloadsTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
		sub := locales.DownloadsNoActive()
		if n := p.activeCount(); n > 0 {
			sub = locales.DownloadsActiveCount(strconv.Itoa(n))
		}
		components.TextMargin(c, sub, theme.TextSM, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
	})
}

// emptyState：glass-panel mx-auto flex min-h-[24rem] w-full max-w-5xl flex-col items-center justify-center gap-3
// rounded-2xl border border-dashed border-brand-300 text-brand-400 select-none（dark:border-brand-700 dark:text-brand-500），
// i-mdi-download-off text-5xl 与 text-sm 的提示。
func emptyState(c *ui.Context, t *theme.Theme, width float32) {
	fg := t.Pick(theme.Brand400, theme.Brand500)
	bg, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand300, theme.Brand700))
	ui.Column(c.Key("empty")).Width(width).Margin(0, ui.Auto).MinHeight(384).Gap(12).Center().Radius(16).Background(bg).
		Border(t.HL, border).BorderStyle(ui.BorderDashed).Children(func() {
		components.Icon(c, "download-off", 48, fg)
		components.Text(c, locales.DownloadsEmpty(), theme.TextSM, theme.WeightNormal, fg)
	})
}

// section 是一天的任务：h2 mb-3 px-1 text-base font-medium text-brand-600 dark:text-brand-300
// （data-glass:text-brand-700 dark:text-brand-200），下面 grid grid-cols-1 gap-3 的 DownloadCard。
func (p *Page) section(c *ui.Context, t *theme.Theme, g group, services ui.Services) {
	ui.Column(c.Key(g.key)).Label(g.label).Children(func() {
		heading := t.IfGlass(t.Pick(theme.Brand600, theme.Brand300), theme.Brand700, theme.Brand200)
		components.TextMargin(c, g.label, theme.TextBase, theme.WeightMedium, heading, 0, 4, 12, 4)
		ui.Column(c).Gap(12).Children(func() {
			for _, task := range g.tasks {
				card.DownloadCard(c, t, card.DownloadCardProps{
					Task:     task,
					OnCancel: p.cancel,
					OnPause:  p.pause,
					OnResume: p.resume,
					OnRetry:  p.retry,
					OnDelete: p.remove,
					OnCopyURL: func(url string) {
						if url == "" {
							return
						}
						services.WriteClipboard(url)
						p.toast(successCopied)
					},
					OnOpenFolder: p.openFolder,
					OnImport:     p.importAsGame,
					Importing:    p.importing == task.ID,
					Imported:     p.imported[task.ID],
				})
			}
		})
	})
}
