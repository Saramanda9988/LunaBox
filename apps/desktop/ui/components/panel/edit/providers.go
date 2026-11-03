package edit

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"

	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/theme"
)

// logoHeight 是解码后保留的最大高度（像素）：图标最高 h-10，200% 缩放下 80px 足够。
const logoHeight = 96

// logo 是一个来源的图标。原版的 logo 都带 brightness-0（浅色 opacity-80，深色再 invert、opacity-90），
// 也就是只保留轮廓：位图预先做成黑、白两份；SVG 用 ui.Icon 按文字颜色画轮廓。
type logo struct {
	black, white *ui.Bitmap // 只保留透明度的黑色、白色位图
	color        *ui.Bitmap // 原色（bangumi、hikarinagi 的方形图标用 object-cover 原样显示）
	svg          *ui.SVG
	w, h         float32 // 原始尺寸（SVG 是 viewBox），用于 w-auto 按比例算宽度
}

var logoFiles = map[enums.SourceType]string{
	enums.Bangumi:      "bangumi-logo.png",
	enums.VNDB:         "vndb-logo.svg",
	enums.Ymgal:        "ymgal-logo.png",
	enums.Steam:        "steam-logo.png",
	enums.DLsite:       "dlsite-logo.png",
	enums.TouchGal:     "touchgal-logo.webp",
	enums.Hikarinagi:   "hikarinagi-logo.svg",
	enums.ErogameScape: "erogamescape-logo.png",
}

// compactFiles 是 getMetadataSourceIcon(source, "compact") 与 logo 不同的来源。
var compactFiles = map[enums.SourceType]string{
	enums.Bangumi:    "bangumi-icon.png",
	enums.Hikarinagi: "hikarinagi-icon.webp",
}

var (
	logosMu sync.Mutex
	logos   = map[string]*logo{}
)

// sourceLogo 对应 getMetadataSourceIcon(source)；没有图标时返回 nil。只在主线程调用。
func sourceLogo(source enums.SourceType) *logo { return loadLogo(logoFiles[source]) }

// sourceCompactIcon 对应 getMetadataSourceIcon(source, "compact")。
func sourceCompactIcon(source enums.SourceType) *logo {
	if name, ok := compactFiles[source]; ok {
		return loadLogo(name)
	}
	return sourceLogo(source)
}

func loadLogo(name string) *logo {
	if name == "" {
		return nil
	}
	logosMu.Lock()
	defer logosMu.Unlock()
	if l, ok := logos[name]; ok {
		return l
	}
	data, err := assets.ProviderFile(name) // ui/assets/providers，复制自原版 assets/providers
	if err != nil {
		panic(err)
	}
	l := &logo{}
	if strings.HasSuffix(name, ".svg") {
		l.svg = ui.MustParseSVG(data)
		l.w, l.h = svgSize(data)
	} else {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			panic("edit: decode " + name + ": " + err.Error())
		}
		b := img.Bounds()
		l.w, l.h = float32(b.Dx()), float32(b.Dy())
		if b.Dy() > logoHeight {
			img = assets.Downscale(img, b.Dx()*logoHeight/b.Dy())
		}
		l.color = ui.NewBitmap(img)
		l.black, l.white = ui.NewBitmap(silhouette(img, 0)), ui.NewBitmap(silhouette(img, 255))
	}
	logos[name] = l
	return l
}

// silhouette 是 CSS 的 brightness(0)（v=0），再 invert（v=255）：颜色全部变成 v，只保留透明度。
func silhouette(img image.Image, v uint8) image.Image {
	b := img.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{v, v, v, uint8(a >> 8)})
		}
	}
	return out
}

// svgSize 读取 SVG 的 width/height，没有时取 viewBox。
func svgSize(data []byte) (w, h float32) {
	s := string(data)
	attr := func(name string) string {
		i := strings.Index(s, " "+name+"=\"")
		if i < 0 {
			return ""
		}
		rest := s[i+len(name)+3:]
		return rest[:strings.IndexByte(rest, '"')]
	}
	var vw, vh float32
	if vb := strings.Fields(attr("viewBox")); len(vb) == 4 {
		vw, vh = parseFloat(vb[2]), parseFloat(vb[3])
	}
	w, h = parseFloat(attr("width")), parseFloat(attr("height"))
	if w == 0 || h == 0 {
		w, h = vw, vh
	}
	return w, h
}

// silhouetteColor 是 brightness-0 opacity-80 dark:invert dark:opacity-90 的颜色。
func silhouetteColor(t *theme.Theme) ui.Color {
	return t.Pick(theme.Black.Alpha(0.8), theme.White.Alpha(0.9))
}

// logoImage 按给定高度显示来源 logo 的轮廓（w-auto，宽度不超过 maxW），对应 brightness-0 opacity-80 dark:invert dark:opacity-90。
func logoImage(c *ui.Context, t *theme.Theme, l *logo, h, maxW float32) ui.Element {
	w := h * l.w / max(1, l.h)
	if w > maxW {
		w, h = maxW, maxW*l.h/max(1, l.w)
	}
	return silhouetteImage(c, t, l, w, h)
}

// silhouetteImage 在 w×h 的盒子里以 object-contain 显示 logo 的轮廓。
func silhouetteImage(c *ui.Context, t *theme.Theme, l *logo, w, h float32) ui.Element {
	if l.svg != nil {
		// ui.Icon 按文字颜色画 SVG 的轮廓，保持比例。
		iw, ih := w, w*l.h/max(1, l.w)
		if ih > h {
			iw, ih = h*l.w/max(1, l.h), h
		}
		return ui.Box(c).Size(w, h).Center().Shrink(0).Children(func() {
			ui.Icon(c, l.svg).Size(iw, ih).TextColor(silhouetteColor(t))
		})
	}
	bmp, opacity := l.black, float32(0.8)
	if t.Dark {
		bmp, opacity = l.white, 0.9
	}
	return ui.Image(c, bmp).Size(w, h).Fit(ui.Contain).Opacity(opacity).Shrink(0)
}
