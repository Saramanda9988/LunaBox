package account

import (
	"bytes"
	"image"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
)

// 水印 logo 的不透明度：opacity-30 dark:opacity-25。
const (
	logoOpacityLight = 0.3
	logoOpacityDark  = 0.25
)

// watermark 是一张水印 logo：位图按原尺寸解码（assets.Provider 会缩到 96px，w-48 的 logo 会糊），
// SVG 预先包一层 <g opacity> 得到浅色与深色主题的两份（Painter.Image 画 SVG 时没有不透明度参数）。
type watermark struct {
	bitmap   *ui.Bitmap
	svgLight *ui.SVG
	svgDark  *ui.SVG
	w, h     float32 // 图片自身的尺寸，用于 h-auto 的比例
}

var (
	watermarksMu sync.Mutex
	watermarks   = map[string]*watermark{}
)

// loadWatermark 读取并缓存 providers/ 下的 logo。
func loadWatermark(file string) *watermark {
	watermarksMu.Lock()
	defer watermarksMu.Unlock()
	if wm, ok := watermarks[file]; ok {
		return wm
	}
	data, err := assets.ProviderFile(file)
	if err != nil {
		panic(err)
	}
	wm := &watermark{}
	if strings.HasSuffix(file, ".svg") {
		wm.svgLight = ui.MustParseSVG(withOpacity(data, "0.3"))
		wm.svgDark = ui.MustParseSVG(withOpacity(data, "0.25"))
		wm.w, wm.h = wm.svgLight.Size()
	} else {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
		wm.bitmap = ui.NewBitmap(img)
		w, h := wm.bitmap.Size()
		wm.w, wm.h = float32(w), float32(h)
	}
	watermarks[file] = wm
	return wm
}

// withOpacity 把 SVG 根元素的内容包进 <g opacity="…">。
func withOpacity(data []byte, opacity string) []byte {
	s := string(data)
	open := strings.Index(s, "<svg")
	if open < 0 {
		return data
	}
	end := strings.Index(s[open:], ">")
	closing := strings.LastIndex(s, "</svg>")
	if end < 0 || closing < 0 {
		return data
	}
	end += open + 1
	return []byte(s[:end] + `<g opacity="` + opacity + `">` + s[end:closing] + "</g>" + s[closing:])
}

// paintWatermark 在卡片的背景之上、内容之下画 logo（pointer-events-none absolute z-0 object-contain），
// box 是卡片的边框盒，inset 是边框宽度（absolute 相对 padding box）。
func paintWatermark(p *ui.Painter, box ui.Rect, inset float32, wm *watermark, l logoSpec, dark bool) {
	w, h := l.w, l.h
	if h == 0 && wm.w > 0 {
		h = w * wm.h / wm.w
	}
	dst := ui.Rect{X: box.X + box.W - inset - l.right - w, Y: box.Y + box.H - inset - l.bottom - h, W: w, H: h}
	if wm.bitmap != nil {
		// object-contain：在 w×h 的盒子里按比例居中。
		if wm.w > 0 && wm.h > 0 {
			scale := min(w/wm.w, h/wm.h)
			iw, ih := wm.w*scale, wm.h*scale
			dst = ui.Rect{X: dst.X + (w-iw)/2, Y: dst.Y + (h-ih)/2, W: iw, H: ih}
		}
		opacity := float32(logoOpacityLight)
		if dark {
			opacity = logoOpacityDark
		}
		p.ImageRegion(wm.bitmap, ui.Rect{}, dst, ui.ImageOptions{Tint: ui.RGBA(255, 255, 255, opacity)})
		return
	}
	svg := wm.svgLight
	if dark {
		svg = wm.svgDark
	}
	p.Image(svg, dst, ui.Contain)
}
