package better

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// 与 BetterImageViewer.tsx 相同的缩放范围与步长。
const (
	viewerMinScale  = 0.5
	viewerMaxScale  = 5
	viewerScaleStep = 0.25
)

// ImageViewerProps 是 BetterImageViewer 的属性。
type ImageViewerProps struct {
	Open      *bool
	Title     string
	Image     *ui.Bitmap // 大图；加载中为 nil 时先显示 Preview
	Preview   *ui.Bitmap // 缩略图
	TopOffset float32
	OnSave    func() // 非空时工具栏有“另存为”
}

type viewerState struct {
	scale  float32
	offset ui.Point
}

// ImageViewer 对应 BetterImageViewer：全屏 backdrop-blur-md 浮层，顶部标题与关闭按钮，中间可滚轮缩放、
// 拖动的图片（max-h-[calc(100dvh-8.5rem)] max-w-full object-contain），底部放大、缩小、复位、另存为工具栏。
// Esc 或点击图片外的空白关闭；关闭时缩放与位置复位。
func ImageViewer(c *ui.Context, t *theme.Theme, key string, p ImageViewerProps) {
	if !*p.Open {
		return
	}
	closeViewer := func() { *p.Open = false }
	winW, winH := c.Size()
	components.Overlay(c, t, func() {
		host := ui.Box(c.Key(key)).Absolute().Top(p.TopOffset).Left(0).Right(0).Bottom(0).Modal().Label(p.Title)
		host.OnShortcut(0, ui.KeyEscape, closeViewer) // Modal 把键盘留在浮层里，Esc 要挂在浮层上
		st := ui.Local(host, "viewer", func() *viewerState { return &viewerState{scale: 1} })
		s := *st
		zoom := func(d float32) { s.scale = min(viewerMaxScale, max(viewerMinScale, s.scale+d)) }
		host.Children(func() {
			// absolute inset-0 bg-transparent backdrop-blur-md
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Material(glass.Blur{Radius: 12})

			// flex-1 items-center justify-center px-8 pb-28 pt-16：点击空白关闭，滚轮缩放
			stage := ui.ButtonBase(c.Key("stage")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(64, 32, 112, 32).
				Center().FocusRing(false).Label(locales.GameImageViewerClose())
			stage.OnClick(closeViewer).HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputScroll || ev.DY == 0 {
					return false
				}
				if ev.DY > 0 {
					zoom(-viewerScaleStep)
				} else {
					zoom(viewerScaleStep)
				}
				return true
			})
			stage.Children(func() { viewerImage(c, p, s, winW-64, winH-136) })

			viewerHeader(c, t, p.Title, closeViewer)
			viewerToolbar(c, t, s, p.OnSave, zoom)
		})
	})
}

// viewerImage 是可拖动的图片：transition-transform duration-100 ease-out，拖动时 cursor-grabbing。
func viewerImage(c *ui.Context, p ImageViewerProps, s *viewerState, maxW, maxH float32) {
	img := p.Image
	if img == nil {
		img = p.Preview
	}
	if img == nil {
		components.Spinner(c, 30, theme.Brand500)
		return
	}
	// 浏览器按图片的像素尺寸（1px = 1 CSS 像素）显示，受 max-w / max-h 限制并保持比例。
	pw, ph := img.Size()
	w, h := float32(pw), float32(ph)
	if p.Image == nil && p.Preview != nil {
		w, h = maxW, maxW*h/w // 缩略图只是占位，按大图通常的尺寸铺开
	}
	if f := min(1, maxW/w, maxH/h); f < 1 {
		w, h = w*f, h*f
	}
	box := ui.Box(c.Key("image")).Size(w, h).Shrink(0)
	box.Draggable(&s.offset, ui.DraggableOptions{Animation: ui.Animation{Duration: 100 * time.Millisecond, Ease: theme.EaseTailwind}})
	cursor := ui.CursorGrab
	if box.Dragging() {
		cursor = ui.CursorGrabbing
	}
	scale := box.AnimateWith("scale", s.scale, 100*time.Millisecond, theme.EaseTailwind)
	box.Scale(scale, scale).Cursor(cursor).Children(func() {
		ui.Image(c, img).Fill().Fit(ui.Contain)
	})
}

// viewerHeader：absolute inset-x-0 top-0 flex justify-between px-6 py-3，标题 text-sm font-medium truncate，
// 关闭按钮 h-9 w-9 rounded-full border bg-white/85 backdrop-blur-xl shadow-sm。
func viewerHeader(c *ui.Context, t *theme.Theme, title string, onClose func()) {
	ui.Row(c.Key("header")).Absolute().Top(0).Left(0).Right(0).Padding(12, 24).Gap(16).AlignItems(ui.Center).
		Justify(ui.SpaceBetween).Children(func() {
		components.Text(c, title, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine().Grow(1).Basis(0).MinWidth(0)
		btn := viewerGlassButton(c, t, "close", locales.GameImageViewerClose(), false)
		btn.Border(t.HL, t.Pick(theme.Brand200.Alpha(0.8), theme.Brand600.Alpha(0.8))).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.1))
		bg, fg := t.Pick(theme.White.Alpha(0.85), theme.Brand800.Alpha(0.85)), t.Pick(theme.Brand600, theme.Brand300)
		if btn.Hovered() {
			bg, fg = t.Pick(theme.White, theme.Brand800), t.Pick(theme.Brand900, theme.White)
		}
		btn.Background(bg).OnClick(onClose).Children(func() { components.Icon(c, "close", 20, fg) })
	})
}

// viewerToolbar：absolute bottom-7 居中的 flex gap-1 rounded-full border bg-white/85 p-1 shadow-lg backdrop-blur-xl。
func viewerToolbar(c *ui.Context, t *theme.Theme, s *viewerState, onSave func(), zoom func(float32)) {
	ui.Row(c.Key("toolbar")).Absolute().Left(0).Right(0).Bottom(28).Justify(ui.Center).PassThrough().Children(func() {
		bar := ui.Row(c).Padding(4).Gap(4).Radius(999).Material(glass.Blur{Radius: 24}).
			Border(t.HL, t.Pick(theme.Brand200.Alpha(0.8), theme.Brand600.Alpha(0.8))).
			Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
		// 毛玻璃之上的 bg-white/85：画在 bar 自己的绘制层，按钮在它之上。
		tint := t.Pick(theme.White.Alpha(0.85), theme.Brand800.Alpha(0.85))
		bar.Draw(func(pt *ui.Painter, r ui.Rect) { pt.Fill(r, tint, r.H/2) })
		bar.Children(func() {
			viewerToolButton(c, t, "zoom-in", "magnify-plus-outline", locales.GameImageViewerZoomIn(), s.scale >= viewerMaxScale, func() { zoom(viewerScaleStep) })
			viewerToolButton(c, t, "zoom-out", "magnify-minus-outline", locales.GameImageViewerZoomOut(), s.scale <= viewerMinScale, func() { zoom(-viewerScaleStep) })
			viewerToolButton(c, t, "reset", "restore", locales.GameImageViewerReset(), false, func() { s.scale, s.offset = 1, ui.Point{} })
			if onSave != nil {
				viewerToolButton(c, t, "save", "content-save-outline", locales.GameImageViewerSave(), false, onSave)
			}
		})
	})
}

// viewerToolButton：h-9 w-9 rounded-full text-brand-600，悬停 bg-brand-100 text-brand-900，disabled:opacity-45。
func viewerToolButton(c *ui.Context, t *theme.Theme, key, icon, label string, disabled bool, onClick func()) {
	btn := viewerGlassButton(c, t, key, label, disabled)
	fg := t.Pick(theme.Brand600, theme.Brand300)
	if btn.Hovered() && !disabled {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		fg = t.Pick(theme.Brand900, theme.White)
	}
	if !disabled {
		btn.OnClick(onClick)
	}
	btn.Children(func() { components.Icon(c, icon, 20, fg) })
}

func viewerGlassButton(c *ui.Context, t *theme.Theme, key, label string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(36, 36).Radius(999).Center().Shrink(0).FocusRing(false).Label(label).
		Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.45)
	}
	return btn
}
