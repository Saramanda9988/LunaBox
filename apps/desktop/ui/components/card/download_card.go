package card

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// ImageDownloadSource 是封面批量下载任务的 download_source（IMAGE_DOWNLOAD_SOURCE）。
const ImageDownloadSource = "cover-image-batch"

// DownloadCardProps 是 DownloadCard 的属性：任务与各个操作的回调。
type DownloadCardProps struct {
	Task         service.DownloadTask
	OnCancel     func(id string)
	OnPause      func(id string)
	OnResume     func(id string)
	OnRetry      func(id string)
	OnDelete     func(id string)
	OnCopyURL    func(url string)
	OnOpenFolder func(id string)
	OnImport     func(id string)
	Importing    bool
	Imported     bool
}

// downloadSample 是 DownloadCard 的 downloadSampleRef 与 downloadSpeed：上一次进度的字节数与时间，以及由此算出的速度。
type downloadSample struct {
	status     service.DownloadStatus
	downloaded int64
	at         time.Time
	speed      float64
}

// DownloadCard 对应 DownloadCard.tsx：glass-card flex flex-col gap-3 rounded-xl border border-brand-200 bg-white/90 p-4
// shadow-sm transition-all duration-300 hover:shadow-md（dark:border-brand-700 dark:bg-brand-800/80）。
// 第一行是标题与来源、状态徽标和操作按钮，下面按状态显示进度条、完成信息或错误。
func DownloadCard(c *ui.Context, t *theme.Theme, p DownloadCardProps) ui.Element {
	task := p.Task
	isImage := task.Request.DownloadSource == ImageDownloadSource
	bg, border := t.GlassCard(t.Pick(theme.White.Alpha(0.9), theme.Brand800.Alpha(0.8)), t.Pick(theme.Brand200, theme.Brand700))
	card := ui.Column(c.Key(task.ID)).Gap(12).Padding(16).Radius(12).Border(t.HL, border).Background(bg)
	// shadow-sm 到 hover:shadow-md 的过渡（transition-all duration-300）；glass-card 是 shadow-none。
	h := card.AnimateWith("hover", theme.Bool(card.Hovered()), 300*time.Millisecond, theme.EaseTailwind)
	if !t.Glass {
		card.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05*(1-h))).
			Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1*h)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1*h))
	}
	speed := sampleSpeed(c, card, task, isImage)

	card.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				title := task.Request.Title
				if title == "" {
					title = locales.DownloadsUnknownTitle()
				}
				components.Text(c, title, theme.TextSM, theme.WeightSemibold, t.TextStrong).SingleLine()
				source := task.Request.DownloadSource
				switch {
				case source == "":
					source = locales.DownloadsUnknownSource()
				case isImage:
					source = locales.DownloadsImageTaskSource()
				}
				components.TextMargin(c, source, theme.TextXS, theme.WeightNormal, t.TextMuted, 2, 0, 0, 0).SingleLine()
			})
			ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
				statusBadge(c, t, task.Status)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() { downloadActions(c, t, p, isImage) })
			})
		})
		downloadDetails(c, t, task, isImage, speed)
	})
	return card
}

// sampleSpeed 对应 DownloadCard 里计算 downloadSpeed 的 useEffect：只在下载中（不是图片任务）时，
// 用相邻两次进度的字节差除以时间差；状态或字节数变化时才重新计算（effect 的依赖），所以一帧构建多次结果不变。
func sampleSpeed(c *ui.Context, card ui.Element, task service.DownloadTask, isImage bool) float64 {
	s := ui.Local(card, "speed", func() downloadSample { return downloadSample{} })
	if task.Status != service.DownloadStatusDownloading || isImage {
		*s = downloadSample{status: task.Status}
		return 0
	}
	if s.status == task.Status && s.downloaded == task.Downloaded && !s.at.IsZero() {
		return s.speed
	}
	now := c.Now()
	speed := 0.0
	if !s.at.IsZero() && task.Downloaded >= s.downloaded && now.After(s.at) {
		speed = float64(task.Downloaded-s.downloaded) / now.Sub(s.at).Seconds()
	}
	*s = downloadSample{status: task.Status, downloaded: task.Downloaded, at: now, speed: speed}
	return speed
}

// badgeColors 是 StatusBadge 的 map：浅色主题的背景与文字，深色主题的背景与文字。
func badgeColors(t *theme.Theme, status service.DownloadStatus) (bg, fg ui.Color) {
	switch status {
	case service.DownloadStatusDownloading, service.DownloadStatusExtracting:
		return t.Pick(theme.Info100, theme.Info900.Alpha(0.4)), t.Pick(theme.Info700, theme.Info300)
	case service.DownloadStatusDone:
		return t.Pick(theme.Success100, theme.Success900.Alpha(0.4)), t.Pick(theme.Success700, theme.Success300)
	case service.DownloadStatusError:
		return t.Pick(theme.Error100, theme.Error900.Alpha(0.4)), t.Pick(theme.Error700, theme.Error300)
	case service.DownloadStatusCancelled:
		return t.Pick(theme.Brand100, theme.Brand800), t.Pick(theme.Brand600, theme.Brand400)
	}
	// pending、paused，以及未知状态（map[status] ?? map.pending）
	return t.Pick(theme.Warning100, theme.Warning900.Alpha(0.4)), t.Pick(theme.Warning700, theme.Warning300)
}

// statusLabel 是 StatusBadge 的文字，未知状态显示“等待中”。
func statusLabel(status service.DownloadStatus) string {
	switch status {
	case service.DownloadStatusDownloading:
		return locales.DownloadsStatusDownloading()
	case service.DownloadStatusExtracting:
		return locales.DownloadsStatusExtracting()
	case service.DownloadStatusPaused:
		return locales.DownloadsStatusPaused()
	case service.DownloadStatusDone:
		return locales.DownloadsStatusDone()
	case service.DownloadStatusError:
		return locales.DownloadsStatusError()
	case service.DownloadStatusCancelled:
		return locales.DownloadsStatusCancelled()
	}
	return locales.DownloadsStatusPending()
}

// statusBadge 对应 StatusBadge：rounded-full px-2 py-0.5 text-xs font-medium。
func statusBadge(c *ui.Context, t *theme.Theme, status service.DownloadStatus) {
	bg, fg := badgeColors(t, status)
	ui.Box(c.Key("status")).Padding(2, 8).Radius(999).Background(bg).Children(func() {
		components.Text(c, statusLabel(status), theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
}

// actionColors 是一个操作按钮的文字色与悬停时的背景、文字色。
type actionColors struct{ fg, hoverBg, hoverFg ui.Color }

// neutralAction 是 DOWNLOAD_ACTION_BUTTON_NEUTRAL_CLASS：text-brand-600 hover:bg-brand-100 hover:text-brand-900
// （dark:text-brand-300 dark:hover:bg-brand-700/70 dark:hover:text-white），玻璃模式下
// data-glass:hover:bg-white/15 data-glass:dark:hover:bg-black/20。
func neutralAction(t *theme.Theme) actionColors {
	hover := t.IfGlass(t.Pick(theme.Brand100, theme.Brand700.Alpha(0.7)), theme.White.Alpha(0.15), theme.Black.Alpha(0.2))
	return actionColors{t.Pick(theme.Brand600, theme.Brand300), hover, t.Pick(theme.Brand900, theme.White)}
}

// toneAction 是暂停、继续、重试、取消与删除按钮：text-x-600 hover:bg-x-100（dark:text-x-300 dark:hover:bg-x-900/40），
// 玻璃模式下 data-glass:hover:bg-x-500/15（glass 是 x-500）。
func toneAction(t *theme.Theme, fg, fgDark, hover, hoverDark, glass ui.Color) actionColors {
	c := t.Pick(fg, fgDark)
	return actionColors{c, t.IfGlass(t.Pick(hover, hoverDark.Alpha(0.4)), glass.Alpha(0.15), glass.Alpha(0.15)), c}
}

// downloadActions 是 flex items-center gap-1 的操作按钮：导入为游戏（已完成且有文件、不是图片任务）、复制下载地址、
// 打开所在文件夹、暂停（下载中）、继续（已暂停）、重试（出错），最后是取消（进行中）或删除记录。
func downloadActions(c *ui.Context, t *theme.Theme, p DownloadCardProps, isImage bool) {
	task := p.Task
	id := task.ID
	status := task.Status
	if status == service.DownloadStatusDone && task.FilePath != "" && !isImage {
		label, icon := locales.DownloadsImportAsGame(), "gamepad-variant-outline"
		colors := neutralAction(t)
		switch {
		case p.Imported:
			label, icon = locales.DownloadsImported(), "check-circle-outline"
			// text-success-600 dark:text-success-300，disabled:opacity-70
			fg := t.Pick(theme.Success600, theme.Success300)
			colors = actionColors{fg, ui.Transparent, fg}
		case p.Importing:
			label = locales.DownloadsImporting()
		}
		opacity := float32(0.4)
		if p.Imported {
			opacity = 0.7
		}
		btn := actionButton(c, t, "import", label, colors, p.Importing || p.Imported, opacity, func() { p.OnImport(id) })
		btn.Children(func() {
			if p.Importing {
				components.Spinner(c, 20, colors.fg)
			} else {
				components.Icon(c, icon, 20, actionFg(btn, colors, p.Importing || p.Imported))
			}
		})
	}
	url := task.Request.URL
	iconAction(c, t, "copy-url", locales.DownloadsCopyURL(), "link", neutralAction(t), url == "", func() { p.OnCopyURL(url) })
	iconAction(c, t, "open-folder", locales.DownloadsOpenFolder(), "folder-open-outline", neutralAction(t), task.FilePath == "",
		func() { p.OnOpenFolder(id) })
	if status == service.DownloadStatusDownloading && !isImage {
		iconAction(c, t, "pause", locales.DownloadsPause(), "pause",
			toneAction(t, theme.Warning600, theme.Warning300, theme.Warning100, theme.Warning900, theme.Warning500), false, func() { p.OnPause(id) })
	}
	if status == service.DownloadStatusPaused && !isImage {
		iconAction(c, t, "resume", locales.DownloadsResume(), "play",
			toneAction(t, theme.Success600, theme.Success300, theme.Success100, theme.Success900, theme.Success500), false, func() { p.OnResume(id) })
	}
	if status == service.DownloadStatusError {
		iconAction(c, t, "retry", locales.DownloadsRetry(), "refresh",
			toneAction(t, theme.Info600, theme.Info300, theme.Info100, theme.Info900, theme.Info500), false, func() { p.OnRetry(id) })
	}
	danger := toneAction(t, theme.Error500, theme.Error400, theme.Error100, theme.Error900, theme.Error500)
	switch status {
	case service.DownloadStatusPending, service.DownloadStatusDownloading, service.DownloadStatusExtracting, service.DownloadStatusPaused:
		iconAction(c, t, "cancel", locales.DownloadsCancel(), "close", danger, false, func() { p.OnCancel(id) })
	default:
		iconAction(c, t, "delete", locales.DownloadsDelete(), "delete", danger, false, func() { p.OnDelete(id) })
	}
}

// iconAction 是带一个 i-mdi-* text-xl 图标的操作按钮。
func iconAction(c *ui.Context, t *theme.Theme, key, label, icon string, colors actionColors, disabled bool, onClick func()) {
	btn := actionButton(c, t, key, label, colors, disabled, 0.4, onClick)
	btn.Children(func() { components.Icon(c, icon, 20, actionFg(btn, colors, disabled)) })
}

// actionFg 是按钮当前的图标颜色（悬停时 hover:text-*）。
func actionFg(btn ui.Element, colors actionColors, disabled bool) ui.Color {
	if btn.Hovered() && !disabled {
		return colors.hoverFg
	}
	return colors.fg
}

// actionButton 是 DOWNLOAD_ACTION_BUTTON_CLASS：flex size-9 rounded-full bg-transparent
// transition-[color,background-color,transform] duration-150 hover:scale-105 active:scale-90
// focus-visible:ring-2 focus-visible:ring-primary-500/60，disabled:opacity-40（不缩放、悬停不换背景），外面包着 BetterTooltip。
func actionButton(c *ui.Context, t *theme.Theme, key, label string, colors actionColors, disabled bool, disabledOpacity float32, onClick func()) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(36, 36).Radius(999).Center().Shrink(0).FocusRing(false).Label(label)
	scale := float32(1)
	switch {
	case disabled:
	case btn.Pressed():
		scale = 0.9
	case btn.Hovered():
		scale = 1.05
	}
	if btn.Hovered() && !disabled {
		btn.Background(colors.hoverBg)
	}
	s := btn.AnimateWith("scale", scale, 150*time.Millisecond, theme.EaseTailwind)
	btn.Scale(s, s).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if btn.FocusVisible() {
		btn.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			pt.Stroke(ui.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}, theme.Primary500.Alpha(0.6), r.H/2+1, 2)
		})
	}
	if disabled {
		components.Disable(btn, disabledOpacity)
	} else {
		btn.OnClick(onClick)
	}
	better.Tooltip(c, t, btn, label)
	return btn
}

// downloadDetails 是第一行下面的内容：进行中的进度条，图片任务完成的说明，已完成的文件路径（自动解压失败时带提示），
// 或出错的原因。
func downloadDetails(c *ui.Context, t *theme.Theme, task service.DownloadTask, isImage bool, speed float64) {
	muted := t.TextMuted
	switch task.Status {
	case service.DownloadStatusDownloading, service.DownloadStatusPending, service.DownloadStatusPaused, service.DownloadStatusExtracting:
		progressBlock(c, t, task, isImage, speed)
	case service.DownloadStatusDone:
		switch {
		case isImage:
			// flex items-center gap-1 text-xs，i-mdi-image-multiple-outline
			msg := locales.DownloadsImageTaskDone()
			if task.Error != "" {
				msg = locales.DownloadsImageTaskDoneWithError()
			}
			ui.Row(c.Key("image-done")).Gap(4).AlignItems(ui.Center).Children(func() {
				components.Icon(c, "image-multiple-outline", 12, muted)
				components.Text(c, msg, theme.TextXS, theme.WeightNormal, muted)
			})
		case task.FilePath != "":
			// flex items-center gap-1 text-xs，i-mdi-folder-check 与 truncate 的路径
			ui.Row(c.Key("file-path")).Gap(4).AlignItems(ui.Center).Children(func() {
				components.Icon(c, "folder-check", 12, muted)
				components.Text(c, task.FilePath, theme.TextXS, theme.WeightNormal, muted).SingleLine().MinWidth(0).Shrink(1)
			})
			if task.Error == service.DownloadManualExtractFlag {
				detailNote(c, "manual-extract", "alert-outline", locales.DownloadsManualExtractHint(), t.Pick(theme.Warning600, theme.Warning300))
			}
		}
	case service.DownloadStatusError:
		if task.Error != "" {
			detailNote(c, "error", "alert-circle", task.Error, t.Pick(theme.Error500, theme.Error400))
		}
	}
}

// detailNote：flex items-start gap-1 text-xs，图标 shrink-0 mt-0.5，文字可换行。
func detailNote(c *ui.Context, key, icon, msg string, color ui.Color) {
	ui.Row(c.Key(key)).Gap(4).AlignItems(ui.Start).Children(func() {
		components.Icon(c, icon, 12, color).Margin(2, 0, 0, 0)
		components.Text(c, msg, theme.TextXS, theme.WeightNormal, color).Grow(1).Basis(0).MinWidth(0)
	})
}

// progressBlock 是 space-y-1 的进度条与进度文字：h-2 rounded-full bg-brand-200 dark:bg-brand-700 的槽，
// bg-info-500 的条（transition-all duration-300，解压时 animate-pulse）；下面一行 flex justify-between text-xs，
// 左边百分比，右边图片张数，或下载速度（下载中）与已下载/总大小。
func progressBlock(c *ui.Context, t *theme.Theme, task service.DownloadTask, isImage bool, speed float64) {
	progress := max(0, min(100, task.Progress))
	ui.Column(c.Key("progress")).Gap(4).Children(func() {
		track := ui.Box(c.Key("track")).Height(8).FillWidth().Radius(999).Clip().Background(t.Pick(theme.Brand200, theme.Brand700))
		w := track.AnimateWith("width", float32(progress), 300*time.Millisecond, theme.EaseTailwind)
		track.Children(func() {
			bar := ui.Box(c).Height(8).WidthPercent(w).Radius(999).Background(theme.Info500)
			if task.Status == service.DownloadStatusExtracting {
				bar.Opacity(components.Pulse(bar))
			}
		})
		ui.Row(c).Justify(ui.SpaceBetween).Gap(8).Children(func() {
			muted := t.TextMuted
			components.Text(c, toFixed1(progress)+"%", theme.TextXS, theme.WeightNormal, muted).NoWrap()
			components.Text(c, progressDetail(task, isImage, speed), theme.TextXS, theme.WeightNormal, muted).NoWrap()
		})
	})
}

// progressDetail 是进度条右下角的文字：图片任务是“3 / 10 张”，否则是“当前速度 1.0 MB/s · 10.0 MB / 100.0 MB”。
func progressDetail(task service.DownloadTask, isImage bool, speed float64) string {
	if isImage {
		s := strconv.FormatInt(task.Downloaded, 10)
		if task.Total > 0 {
			s += " / " + strconv.FormatInt(task.Total, 10)
		}
		return s + " " + locales.DownloadsImageTaskUnit()
	}
	var s string
	if task.Status == service.DownloadStatusDownloading {
		s = locales.DownloadsCurrentSpeed(components.FormatFileSize(speed)+"/s") + " · "
	}
	s += components.FormatFileSize(float64(task.Downloaded))
	if task.Total > 0 {
		s += " / " + components.FormatFileSize(float64(task.Total))
	}
	return s
}

// toFixed1 对应 Number.toFixed(1)：恰好一半时进位（Go 的 %.1f 取偶数）。
func toFixed1(v float64) string {
	return fmt.Sprintf("%.1f", math.Floor(v*10+0.5)/10)
}
