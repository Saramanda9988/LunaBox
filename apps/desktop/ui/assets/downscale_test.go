package assets

import (
	"image"
	"image/color"
	"math/rand/v2"
	"runtime"
	"testing"

	xdraw "golang.org/x/image/draw"
)

func noise(w, h int, alpha bool) []color.NRGBA {
	rnd := rand.New(rand.NewPCG(1, 2))
	px := make([]color.NRGBA, w*h)
	for i := range px {
		// 平滑的渐变加噪声，接近照片；纯噪声缩小后都是平均值，看不出权重的差别
		x, y := i%w, i/w
		a := uint8(255)
		if alpha {
			a = uint8((x * 255 / w) | 1)
		}
		px[i] = color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), uint8(rnd.IntN(256)), a}
	}
	return px
}

// Downscale 与 draw.BiLinear 的结果只差舍入。
func TestDownscaleMatchesBiLinear(t *testing.T) {
	const w, h = 997, 1501
	px := noise(w, h, false)
	rgba := image.NewRGBA(image.Rect(10, 20, 10+w, 20+h))
	ycc := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for y := range h {
		for x := range w {
			c := px[y*w+x]
			rgba.Set(10+x, 20+y, c)
			yy, cb, cr := color.RGBToYCbCr(c.R, c.G, c.B)
			ycc.Y[ycc.YOffset(x, y)] = yy
			ycc.Cb[ycc.COffset(x, y)], ycc.Cr[ycc.COffset(x, y)] = cb, cr
		}
	}
	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, c := range noise(w, h, true) {
		nrgba.SetNRGBA(i%w, i/w, c)
	}
	for name, src := range map[string]image.Image{"rgba": rgba, "ycbcr": ycc, "nrgba": nrgba} {
		for _, dw := range []int{360, 100, 37} {
			got := Downscale(src, dw).(*image.RGBA)
			b := src.Bounds()
			want := image.NewRGBA(image.Rect(0, 0, dw, b.Dy()*dw/b.Dx()))
			xdraw.BiLinear.Scale(want, want.Bounds(), src, b, xdraw.Src, nil)
			if got.Rect != want.Rect {
				t.Fatalf("%s %d: size %v, want %v", name, dw, got.Rect, want.Rect)
			}
			worst := 0
			for i := range got.Pix {
				worst = max(worst, abs(int(got.Pix[i])-int(want.Pix[i])))
			}
			if worst > 2 {
				t.Errorf("%s %d: max channel difference %d, want <= 2", name, dw, worst)
			}
		}
	}
}

func abs(v int) int { return max(v, -v) }

// 缩小大图时的内存与目标图同一量级，不随原图的高度增长。
func TestDownscaleMemory(t *testing.T) {
	src := image.NewYCbCr(image.Rect(0, 0, 4096, 2465), image.YCbCrSubsampleRatio420)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	Downscale(src, 360)
	runtime.ReadMemStats(&after)
	// 目标图 360×216×4 ≈ 0.3MB；draw.BiLinear 要 360×2465×32 ≈ 28MB
	if got := after.TotalAlloc - before.TotalAlloc; got > 2<<20 {
		t.Errorf("Downscale allocated %.1f MB, want < 2 MB", float64(got)/(1<<20))
	}
}

func BenchmarkDownscale(b *testing.B) {
	src := image.NewYCbCr(image.Rect(0, 0, 4096, 2465), image.YCbCrSubsampleRatio420)
	for b.Loop() {
		Downscale(src, 360)
	}
}

func BenchmarkBiLinear(b *testing.B) {
	src := image.NewYCbCr(image.Rect(0, 0, 4096, 2465), image.YCbCrSubsampleRatio420)
	dst := image.NewRGBA(image.Rect(0, 0, 360, 216))
	for b.Loop() {
		xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	}
}
