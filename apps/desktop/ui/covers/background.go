package covers

import (
	"image"
	"image/draw"
	"math"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
)

// bgZoom 是背景层的 transform: scale(1.1)。
const bgZoom = 1.1

// BackgroundRequest 描述根布局的背景层（__root.tsx）：absolute inset-0 bg-cover bg-center 的 background_image，
// filter: blur(Blur px)，再 scale(1.1)。W、H 是窗口大小（DIP），Scale 是显示器缩放。
type BackgroundRequest struct {
	Src   string
	Blur  float32
	W, H  float32
	Scale float64
}

// sameAs 报告两次请求画出来的背景是否相同：窗口大小变化不到一成时沿用旧图（显示时按 Cover 裁剪），不反复重算。
func (r BackgroundRequest) sameAs(o BackgroundRequest) bool {
	near := func(a, b float32) bool { return a > 0 && b > 0 && math.Abs(float64(a/b)-1) < 0.1 }
	return r.Src == o.Src && r.Blur == o.Blur && r.Scale == o.Scale && near(r.W, o.W) && near(r.H, o.H)
}

// background 是背景层的状态，只在主线程读写。
type background struct {
	want    BackgroundRequest // 最近一次请求
	have    BackgroundRequest // bmp 对应的请求
	bmp     *ui.Bitmap
	running bool
	failed  string // 加载失败的地址，不再重试
}

// Background 返回窗口可见部分的背景图：已经按 cover、居中、放大 1.1 倍裁好，并模糊过，显示时铺满窗口即可。
// 原版每帧由浏览器模糊整张背景；这里在后台解码、裁剪、缩小、模糊一次，结果是窗口大小（模糊时更小）的一张位图，
// 滚动时不必在 GPU 上反复模糊。请求变化时先返回旧图，算好后重绘。src 为空时释放。只在主线程调用。
func (cc *Cache) Background(req BackgroundRequest) *ui.Bitmap {
	bg := &cc.bg
	if req.Src == "" || req.W <= 0 || req.H <= 0 {
		bg.want, bg.have, bg.bmp = BackgroundRequest{}, BackgroundRequest{}, nil
		return nil
	}
	bg.want = req
	if bg.bmp != nil && bg.have.sameAs(req) || bg.running || bg.failed == req.Src || cc.win == nil {
		return bg.bmp
	}
	bg.running = true
	go func() {
		bmp, err := cc.renderBackground(req)
		cc.win.Update(func() {
			bg.running = false
			if err != nil {
				bg.failed = req.Src
				return
			}
			if bg.want.Src == "" {
				return // 已关闭背景
			}
			bg.bmp, bg.have = bmp, req
			bg.failed = ""
		})
	}()
	return bg.bmp
}

// renderBackground 读取并解码 req.Src，裁出窗口可见的部分，缩小到模糊后看不出差别的分辨率，再模糊。
func (cc *Cache) renderBackground(req BackgroundRequest) (*ui.Bitmap, error) {
	var bmp *ui.Bitmap
	err := cc.decode(req.Src, func(img image.Image, _ string) { bmp = ui.NewBitmap(backgroundImage(img, req)) })
	return bmp, err
}

// backgroundImage 对应 bg-cover bg-center 加 scale(1.1)：窗口 W×H 里看到的是图片中央 W/(s·1.1)×H/(s·1.1) 的部分，
// s 是 cover 的缩放比例。blur 作用在放大之前，所以看到的模糊半径是 1.1·Blur。
func backgroundImage(img image.Image, req BackgroundRequest) *image.RGBA {
	b := img.Bounds()
	s := max(float64(req.W)/float64(b.Dx()), float64(req.H)/float64(b.Dy()))
	cw := min(b.Dx(), max(1, int(math.Round(float64(req.W)/(s*bgZoom)))))
	ch := min(b.Dy(), max(1, int(math.Round(float64(req.H)/(s*bgZoom)))))
	crop := image.Rect(0, 0, cw, ch).Add(b.Min).Add(image.Pt((b.Dx()-cw)/2, (b.Dy()-ch)/2))
	if si, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		img = si.SubImage(crop)
	}

	// 模糊的标准差（设备像素）。模糊越大，位图可以越小：缩小到标准差约 3px，再由 GPU 双线性放大，看不出差别。
	sigma := float64(req.Blur) * bgZoom * req.Scale
	k := min(max(sigma/3, 1), 8)
	width := max(1, int(float64(req.W)*req.Scale/k))
	small := assets.Downscale(img, width)
	sb := small.Bounds()
	out, ok := small.(*image.RGBA)
	if !ok || sb.Min != (image.Point{}) {
		out = image.NewRGBA(image.Rect(0, 0, sb.Dx(), sb.Dy()))
		draw.Draw(out, out.Rect, small, sb.Min, draw.Src)
	}
	// 位图宽 sb.Dx() 像素对应窗口宽 W DIP
	gaussianBlur(out, float64(req.Blur)*bgZoom*float64(sb.Dx())/float64(req.W))
	return out
}

// gaussianBlur 用三次方框模糊近似标准差 sigma（像素）的高斯模糊，边缘取最近的像素（原版放大 1.1 倍正是为了藏住
// filter: blur 在边缘变透明的那一圈）。
func gaussianBlur(img *image.RGBA, sigma float64) {
	if sigma < 0.5 {
		return
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	tmp := make([]uint8, len(img.Pix))
	for _, size := range boxSizes(sigma) {
		r := (size - 1) / 2
		boxBlur(img.Pix, tmp, w, h, 4, img.Stride, r) // 横向：一行 w 个像素，相邻像素差 4 字节
		boxBlur(tmp, img.Pix, h, w, img.Stride, 4, r) // 纵向：一列 h 个像素，相邻像素差一行
	}
}

// boxSizes 返回三次方框模糊的宽度（奇数），合起来的方差等于 sigma²（Kovesi, "Fast Almost-Gaussian Filtering"）。
func boxSizes(sigma float64) [3]int {
	ideal := math.Sqrt(12*sigma*sigma/3 + 1)
	lo := int(ideal)
	if lo%2 == 0 {
		lo--
	}
	m := int(math.Round((12*sigma*sigma - 3*float64(lo*lo) - 12*float64(lo) - 9) / (-4*float64(lo) - 4)))
	var sizes [3]int
	for i := range sizes {
		sizes[i] = lo
		if i >= m {
			sizes[i] = lo + 2
		}
	}
	return sizes
}

// boxBlur 对 lines 条线各做一次半径 r 的方框模糊：每条线 n 个像素，像素间隔 step 字节，线的起点间隔 stride 字节。
func boxBlur(src, dst []uint8, n, lines, step, stride, r int) {
	if r <= 0 {
		copy(dst, src)
		return
	}
	div := uint32(2*r + 1)
	for l := range lines {
		base := l * stride
		at := func(i int) int { return base + min(max(i, 0), n-1)*step }
		var sum [4]uint32
		for i := -r; i <= r; i++ {
			p := at(i)
			for c := range 4 {
				sum[c] += uint32(src[p+c])
			}
		}
		for i := range n {
			o := base + i*step
			for c := range 4 {
				dst[o+c] = uint8((sum[c] + div/2) / div)
			}
			add, sub := at(i+r+1), at(i-r)
			for c := range 4 {
				sum[c] += uint32(src[add+c]) - uint32(src[sub+c])
			}
		}
	}
}
