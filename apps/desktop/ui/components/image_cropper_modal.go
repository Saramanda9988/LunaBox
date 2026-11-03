package components

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// cropDisplayMaxWidth 是裁剪弹窗里位图保留的最大宽度（像素）。弹窗最宽 896 DIP，200% 缩放下约 1800px；
// 裁剪框始终按原图的像素尺寸换算，缩小只影响显示。
const cropDisplayMaxWidth = 2048

// CropImage 是 ImageCropperModal 显示的图片：缩小后的位图与原图的像素尺寸（naturalWidth / naturalHeight）。
type CropImage struct {
	Bitmap        *ui.Bitmap
	Width, Height int
}

// LoadCropImage 读取并解码 path 指向的图片（png、jpg、gif、webp、bmp，与 SelectAndCropBackgroundImage 的过滤器相同）。
// 会阻塞，在后台 goroutine 里调用（Env.Async）。
func LoadCropImage(path string) (*CropImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, fmt.Errorf("decode %s: empty image", path)
	}
	return &CropImage{Bitmap: ui.NewBitmap(assets.Downscale(img, cropDisplayMaxWidth)), Width: b.Dx(), Height: b.Dy()}, nil
}

// CropRect 是原图像素坐标里的裁剪区域（handleConfirm 的 cropData）。
type CropRect struct {
	X, Y, Width, Height int
}

// ImageCropperProps 是 ImageCropperModal 的属性。
type ImageCropperProps struct {
	Open bool
	// Image 是待裁剪的图片；解码完成前为 nil（原版 <img> 加载完成前裁剪区是空的）。
	Image *CropImage
	// WindowWidth、WindowHeight 决定默认的宽高比（windowWidth / windowHeight）与“当前窗口”按钮的文字。
	WindowWidth, WindowHeight int
	TopOffset                 float32
	OnConfirm                 func(CropRect)
	OnCancel                  func()
}

// react-image-crop 的样式（ReactCrop.scss）。
var (
	cropHandleBg     = ui.RGBA(0, 0, 0, 0.2)        // --rc-drag-handle-bg-colour
	cropBorderColor  = ui.RGBA(255, 255, 255, 0.7)  // --rc-border-color
	cropFocusColor   = ui.Hex("#0088ff")            // --rc-focus-color
	cropAntLight     = theme.White                  // marching-ants 的 #fff
	cropAntDark      = ui.Hex("#444444")            // marching-ants 的 #444
	cropMaskColor    = ui.RGBA(0, 0, 0, 0.5)        // crop-mask 的 fill="black" fillOpacity=0.5
	cropHandleSize   = float32(12)                  // --rc-drag-handle-size
	cropBarSize      = float32(6)                   // --rc-drag-bar-size
	cropAntWidth     = float32(10)                  // $antWidth
	cropAntSpeed     = float32(20)                  // marching-ants 1s 移动 $doubleAntWidth
	cropperChromeH   = float32(16*6 + 28 + 28 + 36) // 页头、宽高比栏、页脚的高度（不含分隔线）
	cropperMaxWidth  = float32(896)                 // max-w-4xl
	cropperAreaInset = float32(16)                  // 裁剪区 p-4
)

// cropBox 是 react-image-crop 的 Crop：x、y、width、height，单位是显示尺寸的像素（PixelCrop）或百分比（PercentCrop）。
type cropBox struct{ x, y, w, h float64 }

func toPixelCrop(c cropBox, w, h float64) cropBox {
	return cropBox{c.x * w / 100, c.y * h / 100, c.w * w / 100, c.h * h / 100}
}

func toPercentCrop(c cropBox, w, h float64) cropBox {
	return cropBox{c.x / w * 100, c.y / h * 100, c.w / w * 100, c.h / h * 100}
}

func clampF(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }

// jsRound 同 Math.round（.5 向正无穷取整）。
func jsRound(v float64) float64 { return math.Floor(v + 0.5) }

// centerAspectCrop 对应 ImageCropperModal.tsx 的 centerAspectCrop：centerCrop(makeAspectCrop({unit: "%", width: 90}, …))。
func centerAspectCrop(w, h, aspect float64) cropBox {
	px := cropBox{w: 90 * w / 100}
	px.h = px.w / aspect
	if px.y+px.h > h {
		px.h = h - px.y
		px.w = px.h * aspect
	}
	if px.x+px.w > w {
		px.w = w - px.x
		px.h = px.w / aspect
	}
	px.x, px.y = (w-px.w)/2, (h-px.h)/2
	return toPercentCrop(px, w, h)
}

// containCrop 对应 react-image-crop utils.ts 的 containCrop：把裁剪框限制在容器里，保持宽高比与最小、最大尺寸。
func containCrop(c cropBox, aspect float64, ord string, cw, ch, minW, minH, maxW, maxH float64) cropBox {
	cc := c
	mw, mh := math.Min(minW, cw), math.Min(minH, ch)
	xw, xh := math.Min(maxW, cw), math.Min(maxH, ch)
	if aspect != 0 {
		if aspect > 1 {
			if minH != 0 {
				mw = minH * aspect
			}
			mh = mw / aspect
			xw = maxW * aspect
		} else {
			if minW != 0 {
				mh = minW / aspect
			}
			mw = mh * aspect
			xh = maxH / aspect
		}
	}
	if cc.y < 0 {
		cc.h = math.Max(cc.h+cc.y, mh)
		cc.y = 0
	}
	if cc.x < 0 {
		cc.w = math.Max(cc.w+cc.x, mw)
		cc.x = 0
	}
	if over := cw - (cc.x + cc.w); over < 0 {
		cc.x = math.Min(cc.x, cw-mw)
		cc.w += over
	}
	if over := ch - (cc.y + cc.h); over < 0 {
		cc.y = math.Min(cc.y, ch-mh)
		cc.h += over
	}
	west, north := ord == "sw" || ord == "nw", ord == "nw" || ord == "ne"
	if cc.w < mw {
		if west {
			cc.x -= mw - cc.w
		}
		cc.w = mw
	}
	if cc.h < mh {
		if north {
			cc.y -= mh - cc.h
		}
		cc.h = mh
	}
	if cc.w > xw {
		if west {
			cc.x -= xw - cc.w
		}
		cc.w = xw
	}
	if cc.h > xh {
		if north {
			cc.y -= xh - cc.h
		}
		cc.h = xh
	}
	if aspect != 0 {
		if cur := cc.w / cc.h; cur < aspect {
			nh := math.Max(cc.w/aspect, mh)
			if north {
				cc.y -= nh - cc.h
			}
			cc.h = nh
		} else if cur > aspect {
			nw := math.Max(cc.h*aspect, mw)
			if west {
				cc.x -= nw - cc.w
			}
			cc.w = nw
		}
	}
	return cc
}

// cropDrag 是 ReactCrop 的 evData：按下时的指针与裁剪框起点，以及当前的指针位置（都是图片框里的坐标）。
type cropDrag struct {
	startX, startY         float64
	startCropX, startCropY float64
	x, y                   float64
	isResize               bool
	ord                    string // 拖动的手柄（nw、n、ne、e、se、s、sw、w）；画新的裁剪框时为空
}

// cropperState 是弹窗打开期间的状态：ImageCropperModal 的 crop、completedCrop、aspect 与 ReactCrop 的拖动状态。
type cropperState struct {
	image     *CropImage // 已按它初始化裁剪框的图片（onImageLoad）
	aspect    float64    // 0 为自由（aspect 为 undefined）
	crop      *cropBox   // 百分比（onChange 的 percentCrop）；nil 为还没有裁剪框
	completed *cropBox   // completedCrop（onComplete），同样存百分比

	pressing    bool // mouseDownOnCrop
	dragStarted bool
	newCrop     bool // newCropIsBeingDrawn：画新的裁剪框时隐藏手柄
	drag        cropDrag
}

// onImageLoad 对应 onImageLoad：按当前宽高比（自由时用图片的宽高比）放一个居中、宽 90% 的裁剪框。裁剪框第一次出现时
// ReactCrop 的 componentDidUpdate 调用 onComplete，completedCrop 随之确定。
func (s *cropperState) onImageLoad(img *CropImage, w, h float64) {
	first := s.crop == nil
	s.image = img
	aspect := s.aspect
	if aspect == 0 {
		aspect = w / h
	}
	crop := centerAspectCrop(w, h, aspect)
	s.crop = &crop
	if first {
		done := crop
		s.completed = &done
	}
}

// setAspect 对应 handleAspectChange。与原版相同，只改 crop、不调用 onComplete：换了宽高比后不拖动就确认，
// 用的仍是换之前的 completedCrop。
func (s *cropperState) setAspect(aspect, w, h float64) {
	s.aspect = aspect
	if s.image == nil {
		return
	}
	if aspect == 0 {
		aspect = w / h
	}
	crop := centerAspectCrop(w, h, aspect)
	s.crop = &crop
}

// visible 对应 ReactCrop--invisible-crop 的反面：还没开始拖动的 0×0 裁剪框（点了一下图片）不显示。
func (s *cropperState) visible() bool {
	return s.crop != nil && (s.dragStarted || s.crop.w != 0 || s.crop.h != 0)
}

// handleOrds 是 renderCropSelection 里手柄的顺序（后面的画在上面）；固定宽高比时只有四角（ReactCrop--fixed-aspect）。
var (
	cornerOrds = []string{"nw", "ne", "se", "sw"}
	handleOrds = []string{"nw", "n", "ne", "e", "se", "s", "sw", "w"}
	barOrds    = []string{"n", "e", "s", "w"}
)

func (s *cropperState) handles() []string {
	if s.aspect != 0 {
		return cornerOrds
	}
	return handleOrds
}

// handlePoint 返回手柄中心在裁剪框里的位置（.ord-* 的 top/left 加 translate(±50%)）。
func handlePoint(ord string, w, h float64) (float64, float64) {
	x, y := w/2, h/2
	switch ord {
	case "nw", "w", "sw":
		x = 0
	case "ne", "e", "se":
		x = w
	}
	switch ord {
	case "nw", "n", "ne":
		y = 0
	case "sw", "s", "se":
		y = h
	}
	return x, y
}

// hit 返回 (x, y) 处是裁剪框（onCropPointerDown）还是图片（onComponentPointerDown），以及按下的手柄。
// 顺序同 DOM 的叠放：手柄在拖动条之上，拖动条在裁剪框之上。
func (s *cropperState) hit(x, y, w, h float64) (onCrop bool, ord string) {
	if !s.visible() {
		return false, ""
	}
	px := toPixelCrop(*s.crop, w, h)
	lx, ly := x-px.x, y-px.y
	if !s.newCrop {
		hs := s.handles()
		half := float64(cropHandleSize) / 2
		for i := len(hs) - 1; i >= 0; i-- {
			hx, hy := handlePoint(hs[i], px.w, px.h)
			if math.Abs(lx-hx) <= half && math.Abs(ly-hy) <= half {
				return true, hs[i]
			}
		}
		if s.aspect == 0 {
			bar := float64(cropBarSize) / 2
			for i := len(barOrds) - 1; i >= 0; i-- {
				o, in := barOrds[i], false
				switch o {
				case "n", "s":
					by := 0.0
					if o == "s" {
						by = px.h
					}
					in = lx >= 0 && lx <= px.w && math.Abs(ly-by) <= bar
				default:
					bx := 0.0
					if o == "e" {
						bx = px.w
					}
					in = ly >= 0 && ly <= px.h && math.Abs(lx-bx) <= bar
				}
				if in {
					return true, o
				}
			}
		}
	}
	return lx >= 0 && ly >= 0 && lx <= px.w && ly <= px.h, ""
}

// pointerDown 对应 onCropPointerDown（onCrop）与 onComponentPointerDown。
func (s *cropperState) pointerDown(x, y, w, h float64, onCrop bool, ord string) {
	if onCrop {
		px := toPixelCrop(*s.crop, w, h)
		startX, startY, cropX, cropY := x, y, px.x, px.y
		if ord != "" {
			// 起点改成对角，拖动手柄时从对角开始缩放。
			var fromX, fromY float64
			switch ord {
			case "ne", "e":
				fromX, fromY = x-(px.x+px.w), y-px.y
				cropX, cropY = px.x, px.y+px.h
			case "se", "s":
				fromX, fromY = x-(px.x+px.w), y-(px.y+px.h)
				cropX, cropY = px.x, px.y
			case "sw", "w":
				fromX, fromY = x-px.x, y-(px.y+px.h)
				cropX, cropY = px.x+px.w, px.y
			case "nw", "n":
				fromX, fromY = x-px.x, y-px.y
				cropX, cropY = px.x+px.w, px.y+px.h
			}
			startX, startY = cropX+fromX, cropY+fromY
		}
		s.drag = cropDrag{startX: startX, startY: startY, startCropX: cropX, startCropY: cropY, x: x, y: y, isResize: ord != "", ord: ord}
		s.pressing = true
		return
	}
	next := toPercentCrop(cropBox{x: x, y: y}, w, h)
	s.drag = cropDrag{startX: x, startY: y, startCropX: x, startCropY: y, x: x, y: y, isResize: true}
	s.pressing = true
	s.crop = &next
	s.newCrop = true
}

// pointerMove 对应 onDocPointerMove。
func (s *cropperState) pointerMove(x, y, w, h float64) {
	if s.crop == nil || !s.pressing {
		return
	}
	s.dragStarted = true
	s.drag.x, s.drag.y = x, y
	var next cropBox
	if s.drag.isResize {
		next = s.resizeCrop(w, h)
	} else {
		next = s.dragCrop(w, h)
	}
	pc := toPercentCrop(next, w, h)
	s.crop = &pc
}

// pointerUp 对应 onDocPointerDone：拖动结束时调用 onComplete。
func (s *cropperState) pointerUp() {
	if !s.pressing {
		return
	}
	s.pressing, s.dragStarted, s.newCrop = false, false, false
	if s.crop != nil {
		done := *s.crop
		s.completed = &done
	}
}

// dragCrop 对应 dragCrop：平移裁剪框，限制在图片里。
func (s *cropperState) dragCrop(w, h float64) cropBox {
	next := toPixelCrop(*s.crop, w, h)
	d := s.drag
	next.x = clampF(d.startCropX+d.x-d.startX, 0, w-next.w)
	next.y = clampF(d.startCropY+d.y-d.startY, 0, h-next.h)
	return next
}

// pointRegion 对应 getPointRegion（没有最小尺寸）：指针在起点的哪个象限。
func (s *cropperState) pointRegion() string {
	top, left := s.drag.y < s.drag.startCropY, s.drag.x < s.drag.startCropX
	switch {
	case left && top:
		return "nw"
	case left:
		return "sw"
	case top:
		return "ne"
	}
	return "se"
}

// resizeCrop 对应 resizeCrop（没有 minWidth、minHeight、maxWidth、maxHeight）。
func (s *cropperState) resizeCrop(w, h float64) cropBox {
	d, aspect := s.drag, s.aspect
	next := toPixelCrop(*s.crop, w, h)
	area := s.pointRegion()
	ord := d.ord
	if ord == "" {
		ord = area
	}
	xDiff, yDiff := d.x-d.startX, d.y-d.startY
	// 原版：(minWidth && ord === "nw") || ord === "w" || ord === "sw"，minWidth 为 0 时西侧的手柄不能拖过对边。
	if ord == "w" || ord == "sw" {
		xDiff = math.Min(xDiff, 0)
	}
	if ord == "n" || ord == "ne" {
		yDiff = math.Min(yDiff, 0)
	}
	var tmp cropBox
	switch area {
	case "ne":
		tmp.x, tmp.w = d.startCropX, xDiff
		if aspect != 0 {
			tmp.h = tmp.w / aspect
		} else {
			tmp.h = math.Abs(yDiff)
		}
		tmp.y = d.startCropY - tmp.h
	case "se":
		tmp.x, tmp.y, tmp.w = d.startCropX, d.startCropY, xDiff
		if aspect != 0 {
			tmp.h = tmp.w / aspect
		} else {
			tmp.h = yDiff
		}
	case "sw":
		tmp.x, tmp.y, tmp.w = d.startCropX+xDiff, d.startCropY, math.Abs(xDiff)
		if aspect != 0 {
			tmp.h = tmp.w / aspect
		} else {
			tmp.h = yDiff
		}
	case "nw":
		tmp.x, tmp.w = d.startCropX+xDiff, math.Abs(xDiff)
		if aspect != 0 {
			tmp.h = tmp.w / aspect
			tmp.y = d.startCropY - tmp.h
		} else {
			tmp.h = math.Abs(yDiff)
			tmp.y = d.startCropY + yDiff
		}
	}
	contained := containCrop(tmp, aspect, area, w, h, 0, 0, w, h)
	switch {
	case aspect != 0 || len(ord) == 2:
		next = contained
	case ord == "e" || ord == "w":
		next.x, next.w = contained.x, contained.w
	default:
		next.y, next.h = contained.y, contained.h
	}
	next.x = clampF(next.x, 0, w-next.w)
	next.y = clampF(next.y, 0, h-next.h)
	return next
}

// nudgeStep 是方向键的步长：1 像素，Shift 10、Ctrl（macOS 上是 Command）100（nudgeStep、nudgeStepMedium、nudgeStepLarge）。
func nudgeStep(mods ui.Modifiers) float64 {
	switch {
	case mods&ui.Cmd != 0:
		return 100
	case mods&ui.Shift != 0:
		return 10
	}
	return 1
}

// keyDown 对应 onComponentKeyDown：方向键移动裁剪框，并调用 onComplete。
func (s *cropperState) keyDown(key ui.Key, mods ui.Modifiers, w, h float64) bool {
	if s.crop == nil {
		return false
	}
	step := nudgeStep(mods)
	next := toPixelCrop(*s.crop, w, h)
	switch key {
	case ui.KeyLeft:
		next.x -= step
	case ui.KeyRight:
		next.x += step
	case ui.KeyUp:
		next.y -= step
	case ui.KeyDown:
		next.y += step
	default:
		return false
	}
	next.x = clampF(next.x, 0, w-next.w)
	next.y = clampF(next.y, 0, h-next.h)
	pc := toPercentCrop(next, w, h)
	done := pc
	s.crop, s.completed = &pc, &done
	return true
}

// nudgeCrop 对应 react-image-crop utils.ts 的 nudgeCrop：方向键把手柄 ord 往那个方向移动 step，
// 角上的手柄同时移动两条边（按住左上角按左键，裁剪框向左上同时变大）。
func nudgeCrop(c cropBox, key ui.Key, step float64, ord string) cropBox {
	grow := func(dx, dy, dw, dh float64) { c.x, c.y, c.w, c.h = c.x+dx*step, c.y+dy*step, c.w+dw*step, c.h+dh*step }
	switch key {
	case ui.KeyLeft:
		switch ord {
		case "nw":
			grow(-1, -1, 1, 1)
		case "w":
			grow(-1, 0, 1, 0)
		case "sw":
			grow(-1, 0, 1, 1)
		case "ne":
			grow(0, 1, -1, -1)
		case "e":
			grow(0, 0, -1, 0)
		case "se":
			grow(0, 0, -1, -1)
		}
	case ui.KeyRight:
		switch ord {
		case "nw":
			grow(1, 1, -1, -1)
		case "w":
			grow(1, 0, -1, 0)
		case "sw":
			grow(1, 0, -1, -1)
		case "ne":
			grow(0, -1, 1, 1)
		case "e":
			grow(0, 0, 1, 0)
		case "se":
			grow(0, 0, 1, 1)
		}
	case ui.KeyUp:
		switch ord {
		case "nw":
			grow(-1, -1, 1, 1)
		case "n":
			grow(0, -1, 0, 1)
		case "ne":
			grow(0, -1, 1, 1)
		case "sw":
			grow(1, 0, -1, -1)
		case "s":
			grow(0, 0, 0, -1)
		case "se":
			grow(0, 0, -1, -1)
		}
	case ui.KeyDown:
		switch ord {
		case "nw":
			grow(1, 1, -1, -1)
		case "n":
			grow(0, 1, 0, -1)
		case "ne":
			grow(0, 1, -1, -1)
		case "sw":
			grow(-1, 0, 1, 1)
		case "s":
			grow(0, 0, 0, 1)
		case "se":
			grow(0, 0, 1, 1)
		}
	}
	return c
}

// handleKeyDown 对应 onHandlerKeyDown：方向键按手柄 ord 缩放裁剪框（nudgeCrop，再按宽高比限制在图片里），并调用 onComplete。
func (s *cropperState) handleKeyDown(ord string, key ui.Key, mods ui.Modifiers, w, h float64) {
	if s.crop == nil {
		return
	}
	next := nudgeCrop(toPixelCrop(*s.crop, w, h), key, nudgeStep(mods), ord)
	pc := toPercentCrop(containCrop(next, s.aspect, ord, w, h, 0, 0, w, h), w, h)
	done := pc
	s.crop, s.completed = &pc, &done
}

// handleLabel 是手柄的无障碍名称（ReactCrop 的 ariaLabels.*DragHandle）。
func handleLabel(ord string) string {
	switch ord {
	case "nw":
		return locales.NativeImageCropperHandleNw()
	case "n":
		return locales.NativeImageCropperHandleN()
	case "ne":
		return locales.NativeImageCropperHandleNe()
	case "e":
		return locales.NativeImageCropperHandleE()
	case "se":
		return locales.NativeImageCropperHandleSe()
	case "s":
		return locales.NativeImageCropperHandleS()
	case "sw":
		return locales.NativeImageCropperHandleSw()
	}
	return locales.NativeImageCropperHandleW()
}

// nudgeMods 是方向键可以带的修饰键（Ctrl 优先于 Shift，见 nudgeStep）；手柄的快捷键要按修饰键逐个声明。
var nudgeMods = []ui.Modifiers{0, ui.Shift, ui.Cmd, ui.Cmd | ui.Shift}

// confirmRect 对应 handleConfirm：把 completedCrop 换算成原图像素，按端点取整并限制在原图范围内。
func (s *cropperState) confirmRect() (CropRect, bool) {
	if s.completed == nil || s.image == nil {
		return CropRect{}, false
	}
	nw, nh := float64(s.image.Width), float64(s.image.Height)
	c := *s.completed
	x := math.Min(nw, math.Max(0, jsRound(c.x*nw/100)))
	y := math.Min(nh, math.Max(0, jsRound(c.y*nh/100)))
	right := math.Min(nw, math.Max(x, jsRound((c.x+c.w)*nw/100)))
	bottom := math.Min(nh, math.Max(y, jsRound((c.y+c.h)*nh/100)))
	if math.IsNaN(x + y + right + bottom) {
		return CropRect{}, false
	}
	return CropRect{X: int(x), Y: int(y), Width: int(right - x), Height: int(bottom - y)}, true
}

// cropCursor 是 .ord-* 的 cursor（n-resize 等在 Windows 上就是双向箭头）。
func cropCursor(ord string) ui.Cursor {
	switch ord {
	case "nw", "se":
		return ui.CursorResizeNWSE
	case "ne", "sw":
		return ui.CursorResizeNESW
	case "n", "s":
		return ui.CursorResizeNS
	case "e", "w":
		return ui.CursorResizeEW
	}
	return ui.CursorMove
}

// ImageCropperModal 对应 components/modal/ImageCropperModal.tsx：absolute inset-0 z-50 bg-black/50 backdrop-blur-sm 的遮罩
// （不响应点击与 Esc），居中的 max-h-[90vh] w-[90vw] max-w-4xl rounded-lg bg-white shadow-xl dark:bg-brand-800 面板：
// 页头（标题与关闭）、宽高比按钮、裁剪区（react-image-crop 的 ReactCrop）与页脚（取消、确认裁剪）。
// 裁剪区的交互同 ReactCrop：拖动裁剪框平移，拖动手柄缩放（固定宽高比时只有四角），在图片上拖动画新的裁剪框，
// 方向键移动裁剪框。确认时发出原图像素坐标的裁剪区域。
func ImageCropperModal(c *ui.Context, t *theme.Theme, key string, p ImageCropperProps) {
	if !p.Open {
		return
	}
	winW, winH := c.Size()
	panelW := min(winW*0.9, cropperMaxWidth)
	Modal(c, t, key, ModalProps{Open: true, TopOffset: p.TopOffset, MaxWidth: panelW}, func(panel ui.Element) {
		st := ui.Local(panel, "cropper", func() *cropperState {
			s := &cropperState{}
			if p.WindowHeight > 0 {
				s.aspect = float64(p.WindowWidth) / float64(p.WindowHeight)
			}
			return s
		})
		s := *st
		defaultAspect := 0.0
		if p.WindowHeight > 0 {
			defaultAspect = float64(p.WindowWidth) / float64(p.WindowHeight)
		}

		// 图片按原图的像素尺寸（1px = 1 DIP）显示，受裁剪区的内容框限制（maxWidth: cropAreaSize.width，
		// maxHeight: cropAreaSize.height || 60vh）。裁剪区在面板的 90vh 里放不下时由 flex-1 min-h-0 压缩。
		var imgW, imgH float32
		if p.Image != nil {
			maxW := panelW - 2*cropperAreaInset
			maxH := min(winH*0.6, winH*0.9-cropperChromeH-3*t.HL-2*cropperAreaInset)
			imgW, imgH = float32(p.Image.Width), float32(p.Image.Height)
			if f := min(1, maxW/imgW, maxH/imgH); f < 1 {
				imgW, imgH = imgW*f, imgH*f
			}
			imgW, imgH = max(imgW, 1), max(imgH, 1)
			if s.image != p.Image {
				s.onImageLoad(p.Image, float64(imgW), float64(imgH))
			}
		}

		panel.MaxHeight(winH*0.9).Radius(8).Clip().Background(t.Pick(theme.White, theme.Brand800)).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			cropperHeader(c, t, p.OnCancel)
			cropperAspectBar(c, t, s, p, defaultAspect, float64(imgW), float64(imgH))

			// min-h-0 flex-1 overflow-hidden p-4 flex items-center justify-center bg-brand-50 dark:bg-brand-900
			ui.Box(c.Key("crop-area")).FillWidth().Grow(1).MinHeight(0).Padding(cropperAreaInset).Center().Clip().
				Background(t.Pick(theme.Brand50, theme.Brand900)).Children(func() {
				if p.Image != nil {
					cropStage(c, t, s, p.Image, imgW, imgH)
				}
			})

			// flex items-center justify-end gap-3 p-4 border-t
			ui.Row(c.Key("footer")).FillWidth().Padding(16).Gap(12).Justify(ui.End).AlignItems(ui.Center).
				BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
				cancel := cropperFooterButton(c, t, "cancel", locales.CommonCancel(), false)
				if p.OnCancel != nil {
					cancel.OnClick(p.OnCancel)
				}
				cropperFooterButton(c, t, "confirm", locales.NativeImageCropperConfirm(), true).OnClick(func() {
					if p.Image == nil || s.image != p.Image {
						return // 图片还没加载：completedCrop 为 undefined
					}
					if r, ok := s.confirmRect(); ok && p.OnConfirm != nil {
						p.OnConfirm(r)
					}
				})
			})
		})
	})
}

// cropperHeader：flex items-center justify-between p-4 border-b，标题 text-lg font-semibold text-brand-900
// dark:text-brand-100，关闭按钮 i-mdi-close text-xl text-brand-400 hover:text-brand-600 dark:hover:text-brand-200。
func cropperHeader(c *ui.Context, t *theme.Theme, onCancel func()) {
	ui.Row(c.Key("header")).FillWidth().Padding(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		Text(c, locales.NativeImageCropperTitle(), theme.TextLG, theme.WeightSemibold, t.Pick(theme.Brand900, theme.Brand100))
		btn := ui.ButtonBase(c.Key("close")).FocusRing(false).Label(locales.NativeImageCropperClose())
		color := theme.Brand400
		if btn.Hovered() {
			color = t.Pick(theme.Brand600, theme.Brand200)
		}
		color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
		if onCancel != nil {
			btn.OnClick(onCancel)
		}
		btn.Children(func() { Icon(c, "close", 20, color) })
	})
}

// cropperAspectBar：flex items-center gap-2 p-4 border-b，“宽高比：”（text-sm text-brand-600 dark:text-brand-300）与
// 当前窗口、自由、16:9、4:3、1:1 五个按钮。
func cropperAspectBar(c *ui.Context, t *theme.Theme, s *cropperState, p ImageCropperProps, defaultAspect, w, h float64) {
	ui.Row(c.Key("aspects")).FillWidth().Padding(16).Gap(8).AlignItems(ui.Center).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		Text(c, locales.NativeImageCropperAspect(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).NoWrap()
		options := []struct {
			key    string
			label  string
			aspect float64
		}{
			{"window", locales.NativeImageCropperAspectWindow(strconv.Itoa(p.WindowWidth), strconv.Itoa(p.WindowHeight)), defaultAspect},
			{"free", locales.NativeImageCropperAspectFree(), 0},
			{"16:9", "16:9", 16.0 / 9}, // i18n:keep：宽高比
			{"4:3", "4:3", 4.0 / 3},    // i18n:keep：宽高比
			{"1:1", "1:1", 1},          // i18n:keep：宽高比
		}
		for _, o := range options {
			// 原版比较 aspect === 选项的值：窗口正好是 16:9 时两个按钮都高亮。
			active := s.aspect == o.aspect
			btn := ui.ButtonBase(c.Key("aspect-"+o.key)).Padding(4, 12).Radius(6).FocusRing(false).Shrink(0)
			bg, fg := theme.Neutral600, theme.White
			if !active {
				bg, fg = t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand700, theme.Brand300)
				if btn.Hovered() {
					bg = t.Pick(theme.Brand200, theme.Brand600)
				}
			}
			aspect := o.aspect
			btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).
				OnClick(func() { s.setAspect(aspect, w, h) }).Children(func() {
				Text(c, o.label, theme.TextSM, theme.WeightNormal, fg).NoWrap()
			})
		}
	})
}

// cropperFooterButton：px-4 py-2 rounded-md text-sm font-medium transition-colors；取消 bg-brand-100 text-brand-700
// hover:bg-brand-200（dark:bg-brand-700 text-brand-300 hover:bg-brand-600），确认 bg-neutral-600 text-white hover:bg-neutral-700。
func cropperFooterButton(c *ui.Context, t *theme.Theme, key, label string, primary bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 16).Radius(6).FocusRing(false)
	bg, fg := t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand700, theme.Brand300)
	if btn.Hovered() {
		bg = t.Pick(theme.Brand200, theme.Brand600)
	}
	if primary {
		bg, fg = theme.Neutral600, theme.White
		if btn.Hovered() {
			bg = theme.Neutral700
		}
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		Text(c, label, theme.TextSM, theme.WeightMedium, fg).NoWrap()
	})
	return btn
}

// cropStage 是 ReactCrop：图片、裁剪框外的半透明遮罩（crop-mask）、裁剪框（crop-selection，marching ants 边框）与手柄。
// 指针与方向键都由图片框处理（坐标都在图片框里），子元素只提供光标。
func cropStage(c *ui.Context, t *theme.Theme, s *cropperState, img *CropImage, w, h float32) {
	fw, fh := float64(w), float64(h)
	// .ReactCrop 是 cursor: crosshair。按住时 MyGo 显示按下的元素（图片框）的光标，所以换成按下处的样式：
	// 裁剪框 move、手柄与拖动条各自的 resize，画新的裁剪框时仍是 crosshair。
	cursor := ui.CursorCrosshair
	if s.pressing && !(s.drag.isResize && s.drag.ord == "") {
		cursor = cropCursor(s.drag.ord)
	}
	stage := ui.Box(c.Key("crop-stage")).Size(w, h).Shrink(0).Focusable().FocusRing(false).
		Label(locales.NativeImageCropperPreview()).Cursor(cursor)
	focusVisible := stage.FocusVisible()
	stage.HandleInput(func(ev ui.InputEvent) bool {
		x, y := float64(ev.X), float64(ev.Y)
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			onCrop, ord := s.hit(x, y, fw, fh)
			s.pointerDown(x, y, fw, fh, onCrop, ord)
			return true
		case ui.InputPointerMove:
			if !s.pressing {
				return false
			}
			s.pointerMove(x, y, fw, fh)
			return true
		case ui.InputPointerUp:
			if !s.pressing {
				return false
			}
			s.pointerUp()
			return true
		case ui.InputKeyDown:
			return s.keyDown(ev.Key, ev.Mods, fw, fh)
		}
		return false
	})
	stage.Children(func() {
		ui.Image(c, img.Bitmap).Fill().Fit(ui.Contain)
		if !s.visible() {
			return
		}
		px := toPixelCrop(*s.crop, fw, fh)
		cx, cy, cw, ch := float32(px.x), float32(px.y), float32(px.w), float32(px.h)
		ui.Box(c.Key("mask")).Absolute().Top(0).Left(0).Size(w, h).PassThrough().Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: cy}, cropMaskColor, 0)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + cy + ch, W: r.W, H: r.H - cy - ch}, cropMaskColor, 0)
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + cy, W: cx, H: ch}, cropMaskColor, 0)
			pt.Fill(ui.Rect{X: r.X + cx + cw, Y: r.Y + cy, W: r.W - cx - cw, H: ch}, cropMaskColor, 0)
		})
		sel := ui.Box(c.Key("selection")).Absolute().Left(cx).Top(cy).Size(cw, ch).Cursor(ui.CursorMove).
			Label(locales.NativeImageCropperCropArea())
		sel.Draw(func(pt *ui.Painter, r ui.Rect) {
			drawMarchingAnts(pt, r)
			if focusVisible {
				// :focus { outline: 2px solid var(--rc-focus-color); outline-offset: -1px }
				pt.Stroke(ui.Rect{X: r.X - 1, Y: r.Y - 1, W: r.W + 2, H: r.H + 2}, cropFocusColor, 0, 2)
			}
		})
		sel.Children(func() {
			if s.newCrop {
				return // ReactCrop--new-crop 隐藏拖动条与手柄
			}
			if s.aspect == 0 {
				// 拖动条（drag-bar）不可见，只提供光标。
				for _, o := range barOrds {
					bar := ui.Box(c.Key("bar-" + o)).Absolute().Cursor(cropCursor(o))
					switch o {
					case "n":
						bar.Left(0).Top(-cropBarSize/2).Size(cw, cropBarSize)
					case "s":
						bar.Left(0).Top(ch-cropBarSize/2).Size(cw, cropBarSize)
					case "e":
						bar.Left(cw-cropBarSize/2).Top(0).Size(cropBarSize, ch)
					case "w":
						bar.Left(-cropBarSize/2).Top(0).Size(cropBarSize, ch)
					}
				}
			}
			// 手柄 tabIndex=0（Tab 依次经过裁剪框与各个手柄），聚焦时方向键缩放裁剪框。指针仍由图片框处理（手柄没有
			// HandleInput），按在手柄上同样把焦点移到手柄上。
			for _, o := range s.handles() {
				hx, hy := handlePoint(o, px.w, px.h)
				handle := ui.Box(c.Key("handle-"+o)).Absolute().Left(float32(hx)-cropHandleSize/2).Top(float32(hy)-cropHandleSize/2).
					Size(cropHandleSize, cropHandleSize).Border(t.HL, cropBorderColor).Cursor(cropCursor(o)).
					Focusable().FocusRing(false).Role(ui.RoleButton).Label(handleLabel(o))
				bg := cropHandleBg
				if handle.FocusVisible() {
					bg = cropFocusColor // .ReactCrop__drag-handle:focus { background: var(--rc-focus-color) }，同裁剪框只在键盘聚焦时显示
				}
				handle.Background(bg)
				for _, key := range []ui.Key{ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown} {
					for _, mods := range nudgeMods {
						handle.OnShortcut(mods, key, func() { s.handleKeyDown(o, key, mods, fw, fh) })
					}
				}
			}
		})
	})
}

// drawMarchingAnts 画 crop-selection 的边框：四条 1px 的虚线（10px 一段，#fff 与 #444 各半），每秒移动 20px
// （@keyframes marching-ants）：上边向右、下边向左、左边向上、右边向下。
func drawMarchingAnts(pt *ui.Painter, r ui.Rect) {
	if r.W <= 0 && r.H <= 0 {
		return
	}
	now := pt.Now()
	phase := float32(now.UnixNano()%int64(time.Second)) / float32(time.Second)
	shift := cropAntSpeed * phase
	pt.AnimationFrame()
	// dashes 画一条边：底色 #444，再画白色的段；offset 是 background-position 的偏移。
	dashes := func(length, offset float32, rect func(from, to float32) ui.Rect) {
		if length <= 0 {
			return
		}
		pt.Fill(rect(0, length), cropAntDark, 0)
		start := float32(math.Mod(float64(offset), float64(cropAntWidth)))
		if start > 0 {
			start -= cropAntWidth
		}
		for a := start; a < length; a += cropAntWidth {
			from, to := max(a, 0), min(a+cropAntWidth/2, length)
			if to > from {
				pt.Fill(rect(from, to), cropAntLight, 0)
			}
		}
	}
	dashes(r.W, shift, func(a, b float32) ui.Rect { return ui.Rect{X: r.X + a, Y: r.Y, W: b - a, H: 1} })
	dashes(r.W, -shift, func(a, b float32) ui.Rect { return ui.Rect{X: r.X + a, Y: r.Y + r.H - 1, W: b - a, H: 1} })
	dashes(r.H, -shift, func(a, b float32) ui.Rect { return ui.Rect{X: r.X, Y: r.Y + a, W: 1, H: b - a} })
	dashes(r.H, shift, func(a, b float32) ui.Rect { return ui.Rect{X: r.X + r.W - 1, Y: r.Y + a, W: 1, H: b - a} })
}
