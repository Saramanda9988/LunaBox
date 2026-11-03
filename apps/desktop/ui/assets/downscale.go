package assets

import (
	"image"
	"image/draw"
	"math"
)

// Downscale 等比缩小到最大宽度，权重与 draw.BiLinear 相同（缩小时三角核按比例放宽，每个源像素都参与）。
// draw.BiLinear 先把横向的结果存进“目标宽 × 原图高”的 float64 缓冲：4096×2465 的封面缩到 360px 宽要 28MB，
// 游戏库一屏封面的解码垃圾就有两三百 MB。这里逐行流式计算，只留纵向核覆盖的那几十行。
func Downscale(img image.Image, maxWidth int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxWidth {
		return img
	}
	dw, dh := maxWidth, max(1, b.Dy()*maxWidth/b.Dx())
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	xs, ys := tentTaps(dw, b.Dx()), tentTaps(dh, b.Dy())

	// ring 存横向缩好的源行（预乘 RGBA），源行 sy 在 ring[sy%len(ring)]；纵向核最多跨 span 行。
	span := 0
	for _, t := range ys {
		span = max(span, len(t.w))
	}
	ring := make([][]float32, span)
	for i := range ring {
		ring[i] = make([]float32, dw*4)
	}
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), 1)) // 一行源像素，draw.Draw 对 YCbCr、NRGBA 等有快速路径
	next := 0                                         // 下一个要横向缩放的源行
	acc := make([]float32, dw*4)

	for y, ty := range ys {
		for ; next < ty.lo+len(ty.w); next++ {
			draw.Draw(src, src.Rect, img, image.Pt(b.Min.X, b.Min.Y+next), draw.Src)
			row := ring[next%span]
			for x, tx := range xs {
				var r, g, bl, a float32
				p := src.Pix[tx.lo*4:]
				for k, w := range tx.w {
					q := p[k*4 : k*4+4 : k*4+4]
					r += w * float32(q[0])
					g += w * float32(q[1])
					bl += w * float32(q[2])
					a += w * float32(q[3])
				}
				row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = r, g, bl, a
			}
		}
		clear(acc)
		for k, w := range ty.w {
			row := ring[(ty.lo+k)%span]
			for i, v := range row {
				acc[i] += w * v
			}
		}
		out := dst.Pix[y*dst.Stride:]
		for i := 0; i < dw*4; i += 4 {
			a := to8(acc[i+3])
			out[i], out[i+1], out[i+2], out[i+3] = min(to8(acc[i]), a), min(to8(acc[i+1]), a), min(to8(acc[i+2]), a), a
		}
	}
	return dst
}

// tap 是一个目标像素的权重：源像素 lo 起的 len(w) 个，已归一化。
type tap struct {
	lo int
	w  []float32
}

// tentTaps 对应 x/image/draw 的 newDistrib（BiLinear，Support 1）：把 sn 个源像素缩成 dn 个时每个目标像素的权重。
func tentTaps(dn, sn int) []tap {
	scale := float64(sn) / float64(dn)
	half, arg := 1.0, 1.0
	if scale > 1 {
		half, arg = scale, 1/scale
	}
	taps := make([]tap, dn)
	for x := range taps {
		center := (float64(x)+0.5)*scale - 0.5
		lo := max(int(math.Floor(center-half)), 0)
		hi := max(min(int(math.Ceil(center+half)), sn), lo)
		w := make([]float32, hi-lo)
		total := 0.0
		for i := range w {
			if t := math.Abs((center - float64(lo+i)) * arg); t < 1 {
				w[i] = float32(1 - t)
				total += 1 - t
			}
		}
		for i := range w {
			w[i] = float32(float64(w[i]) / total)
		}
		taps[x] = tap{lo: lo, w: w}
	}
	return taps
}

func to8(v float32) uint8 {
	return uint8(min(max(v+0.5, 0), 255))
}
