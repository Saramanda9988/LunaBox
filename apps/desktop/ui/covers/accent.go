package covers

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
)

// DefaultAccent 是 DEFAULT_IMAGE_ACCENT_RGB（rgb(71, 85, 105)）：没有封面或还没算出主色时用它。
var DefaultAccent = ui.RGB(71, 85, 105)

// accentSize 是 fast-average-color 的 speed 模式把图片缩到的最大边长（MAX_SIZE）。
const accentSize = 100

// accentColor 对应 utils/detectImageBrightness.ts 的 detectImageAccentRgb：fast-average-color 的 sqrt 算法
// （speed 模式，先把长边缩到 100px），再与 slate-600 按 72% : 28% 混合，各通道限制在 32–224。
func accentColor(img image.Image) ui.Color {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return DefaultAccent
	}
	dw := accentSize
	if h > w {
		dw = int(math.Round(float64(w) * accentSize / float64(h)))
	}
	small := img
	if dw >= 10 && dw < w { // MIN_SIZE：缩得太小时用原图
		small = assets.Downscale(img, dw)
	}
	sb := small.Bounds()
	var r, g, bl, a float64
	for y := sb.Min.Y; y < sb.Max.Y; y++ {
		for x := sb.Min.X; x < sb.Max.X; x++ {
			pr, pg, pb, pa := small.At(x, y).RGBA()
			if pa == 0 {
				continue
			}
			// RGBA() 是预乘的 16 位值，换回非预乘的 0–255 与 0–255 的 alpha。
			alpha := float64(pa) / 257
			fr, fg, fb := float64(pr)*255/float64(pa), float64(pg)*255/float64(pa), float64(pb)*255/float64(pa)
			r += fr * fr * alpha
			g += fg * fg * alpha
			bl += fb * fb * alpha
			a += alpha
		}
	}
	if a == 0 {
		return DefaultAccent
	}
	mix := func(sum, neutral float64) uint8 {
		v := math.Round(math.Sqrt(sum/a))*0.72 + neutral*0.28
		return uint8(max(32, min(224, math.Round(v))))
	}
	return ui.RGB(mix(r, 71), mix(g, 85), mix(bl, 105))
}
