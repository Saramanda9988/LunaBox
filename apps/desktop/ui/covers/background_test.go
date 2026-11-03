package covers

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// 窗口看到的是 cover 裁剪后再放大 1.1 倍的中央部分；没有模糊时位图与窗口的设备像素一样宽。
func TestBackgroundImageCrop(t *testing.T) {
	// 4000×1000 的横图放进 1000×500 的窗口：cover 按高度缩放 s=0.5，看到的是中央 1000/0.55 × 500/0.55
	src := image.NewRGBA(image.Rect(0, 0, 4000, 1000))
	for x := range 4000 {
		for y := range 1000 {
			v := uint8(0)
			if x >= 2000 {
				v = 255 // 右半边白色：裁剪居中时，位图的中线正好是黑白分界
			}
			src.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	out := backgroundImage(src, BackgroundRequest{W: 1000, H: 500, Scale: 1})
	if got := out.Bounds().Size(); got != image.Pt(1000, 500) {
		t.Fatalf("size %v, want 1000×500", got)
	}
	if l, r := out.RGBAAt(498, 250).R, out.RGBAAt(502, 250).R; l > 10 || r < 245 {
		t.Errorf("edge at the middle: left %d, right %d", l, r)
	}

	// 模糊 20 DIP（150% 下标准差 33 设备像素）时缩小到标准差约 3px：宽度约为设备像素的 1/8 以内
	blurred := backgroundImage(src, BackgroundRequest{W: 1000, H: 500, Scale: 1.5, Blur: 20})
	if w := blurred.Bounds().Dx(); w > 1500/8+1 || w < 1500/11 {
		t.Errorf("blurred width %d, want about 1500/%.1f", w, 20*1.1*1.5/3)
	}
}

// 纯色不变；一个白点模糊后的标准差接近 sigma。
func TestGaussianBlur(t *testing.T) {
	flat := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for i := range flat.Pix {
		flat.Pix[i] = 200
	}
	gaussianBlur(flat, 4)
	for i, v := range flat.Pix {
		if v != 200 {
			t.Fatalf("flat image changed at %d: %d", i, v)
		}
	}

	// 一条竖线模糊后，中间那一行的分布就是一维的高斯
	const n, sigma = 201, 8.0
	line := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		line.SetRGBA(n/2, y, color.RGBA{255, 255, 255, 255})
	}
	gaussianBlur(line, sigma)
	var sum, m2 float64
	for x := range n {
		v := float64(line.RGBAAt(x, n/2).R)
		d := float64(x - n/2)
		sum += v
		m2 += v * d * d
	}
	if got := math.Sqrt(m2 / sum); math.Abs(got-sigma) > 1 {
		t.Errorf("blur spread %.2f, want %.1f", got, sigma)
	}
}
