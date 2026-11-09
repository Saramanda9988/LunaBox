package metadata

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"strconv"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/metasource"
	"lunabox/desktop/ui/theme"
)

// defaultEnabledSources 对应 DEFAULT_ENABLED_METADATA_SOURCES。
var defaultEnabledSources = []enums.SourceType{enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam}

// normalizeEnabledSources 对应 normalizeEnabledMetadataSources：小写、去空白、去重、去掉未知来源；结果为空时是默认来源。
func normalizeEnabledSources(sources []string) []enums.SourceType {
	out := make([]enums.SourceType, 0, len(sources))
	seen := map[enums.SourceType]bool{}
	for _, s := range sources {
		v := enums.SourceType(strings.ToLower(strings.TrimSpace(s)))
		if seen[v] || !metasource.Known(v) {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return append([]enums.SourceType(nil), defaultEnabledSources...)
	}
	return out
}

// AllSources 是 ALL_METADATA_SOURCES（也是 importFlow.ts 的 DEFAULT_METADATA_SOURCE_ORDER）。
func AllSources() []enums.SourceType { return metasource.All() }

// EnabledSources 对应 store 的 enabledMetadataSources：配置里启用的元数据来源（normalizeEnabledMetadataSources）。
func EnabledSources(cfg *appconf.AppConfig) []enums.SourceType {
	if cfg == nil {
		return normalizeEnabledSources(nil)
	}
	return normalizeEnabledSources(cfg.MetadataSources)
}

// sourceItems 是 MetadataSettingsPanel 的 sourceItems 的顺序：与 ALL_METADATA_SOURCES 不同，同原版。
var sourceItems = metasource.Ordered(
	enums.Bangumi, enums.VNDB, enums.Hikarinagi, enums.Kungal, enums.Steam,
	enums.DLsite, enums.TouchGal, enums.Ymgal, enums.ErogameScape,
)

// sourceItemLabel 是 sourceItems 的 label：原版这里 Ymgal 不翻译，其余同 metasource.Label。
func sourceItemLabel(source enums.SourceType) string {
	if source == enums.Ymgal {
		return "Ymgal"
	}
	return metasource.Label(source)
}

// logoHeight 是位图解码后保留的最大高度（像素）：图标高 22px，200% 缩放下 44px，留些余量。
const logoHeight = 96

// logo 是一个来源的 logo。原版的 img 带 brightness-0 opacity-80 dark:invert dark:opacity-90，只保留轮廓：
// 位图预先做成黑、白两份；SVG 用 ui.Icon 按文字颜色画轮廓。
type logo struct {
	black, white *ui.Bitmap
	svg          *ui.SVG
	w, h         float32 // 原始尺寸（SVG 取 width/height 或 viewBox），用于 w-auto 按比例算宽度
}

var (
	logosMu sync.Mutex
	logos   = map[string]*logo{}
)

// sourceLogo 返回来源的 logo；没有时为 nil。
func sourceLogo(source enums.SourceType) *logo {
	name := metasource.LogoFile(source, false)
	if name == "" {
		return nil
	}
	logosMu.Lock()
	defer logosMu.Unlock()
	if l, ok := logos[name]; ok {
		return l
	}
	data, err := assets.ProviderFile(name)
	if err != nil {
		panic(err)
	}
	l := &logo{}
	if strings.HasSuffix(name, ".svg") {
		l.svg = ui.MustParseSVG(data)
		l.w, l.h = svgSize(string(data))
	} else {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			panic("metadata: decode " + name + ": " + err.Error())
		}
		b := img.Bounds()
		l.w, l.h = float32(b.Dx()), float32(b.Dy())
		if b.Dy() > logoHeight {
			img = assets.Downscale(img, b.Dx()*logoHeight/b.Dy())
		}
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
func svgSize(s string) (w, h float32) {
	attr := func(name string) string {
		i := strings.Index(s, " "+name+"=\"")
		if i < 0 {
			return ""
		}
		rest := s[i+len(name)+3:]
		return rest[:strings.IndexByte(rest, '"')]
	}
	parse := func(v string) float32 {
		f, _ := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 32)
		return float32(f)
	}
	w, h = parse(attr("width")), parse(attr("height"))
	if vb := strings.Fields(attr("viewBox")); (w == 0 || h == 0) && len(vb) == 4 {
		w, h = parse(vb[2]), parse(vb[3])
	}
	return w, h
}

// logoImage：h-[22px] w-auto object-contain brightness-0 opacity-80 dark:invert dark:opacity-90。
func logoImage(c *ui.Context, t *theme.Theme, l *logo, h float32) ui.Element {
	w := h * l.w / max(1, l.h)
	if l.svg != nil {
		return ui.Icon(c, l.svg).Size(w, h).Shrink(0).TextColor(t.Pick(theme.Black.Alpha(0.8), theme.White.Alpha(0.9)))
	}
	bmp, opacity := l.black, float32(0.8)
	if t.Dark {
		bmp, opacity = l.white, 0.9
	}
	return ui.Image(c, bmp).Size(w, h).Fit(ui.Contain).Opacity(opacity).Shrink(0)
}
