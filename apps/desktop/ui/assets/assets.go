// Package assets 内嵌原版前端用到的图标与图片：icons/ 是从 @iconify-json/mdi 导出的 i-mdi-* 图标，
// 其余图片（importers、providers 与 PNG）复制自 apps/desktop/frontend/src/assets。
package assets

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"
)

//go:embed icons importers providers *.png *.webp
var files embed.FS

var (
	iconsMu sync.Mutex
	icons   = map[string]*ui.SVG{}
)

// Icon 按 i-mdi-* 的名字（不含前缀）返回图标，例如 Icon("gamepad-variant")。
func Icon(name string) *ui.SVG {
	iconsMu.Lock()
	defer iconsMu.Unlock()
	s, ok := icons[name]
	if !ok {
		data, err := files.ReadFile("icons/" + name + ".svg")
		if err != nil {
			panic("assets: missing icon " + name + "，从 @iconify-json/mdi 导出到 ui/assets/icons") // i18n:keep：给开发者看的错误
		}
		s = ui.MustParseSVG(data)
		icons[name] = s
	}
	return s
}

// 窗口控制按钮的图标，沿用 TopBar.tsx 里的内联 SVG。
var (
	Minimise = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M0 6h12" stroke="currentColor" stroke-width="1.5"/></svg>`))
	Maximise = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M1 1h10v10H1V1z" stroke="currentColor" stroke-width="1.5"/></svg>`))
	Restore  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M3 3h6v6H3V3z" stroke="currentColor" stroke-width="1.5"/><path d="M5 1h6v6" stroke="currentColor" stroke-width="1.5"/></svg>`))
	Close    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" fill="none"><path d="M1 1l10 10M11 1L1 11" stroke="currentColor" stroke-width="1.5"/></svg>`))
)

type bitmaps struct {
	appIcon, appIconDark  *ui.Bitmap
	titleLight, titleDark *ui.Bitmap // titleLight 用于浅色主题（深色文字）
	importers             map[string]*ui.Bitmap
}

var loadBitmaps = sync.OnceValue(func() *bitmaps {
	b := &bitmaps{
		appIcon:     bitmap("appicon.png", 96),
		appIconDark: bitmap("appicon-dark.png", 96),
		titleLight:  bitmap("topbar-title-dark.png", 260),
		titleDark:   bitmap("topbar-title.png", 260),
		importers:   map[string]*ui.Bitmap{},
	}
	for _, name := range []string{"potatovn", "playnite", "yukihub", "vnite", "reinamanager"} {
		b.importers[name] = bitmap("importers/"+name+".png", 72)
	}
	return b
})

// AppIcon 是侧边栏的应用图标。
func AppIcon(dark bool) *ui.Bitmap {
	if dark {
		return loadBitmaps().appIconDark
	}
	return loadBitmaps().appIcon
}

// Title 是顶栏与侧边栏的“LunaBox”字标。
func Title(dark bool) *ui.Bitmap {
	if dark {
		return loadBitmaps().titleDark
	}
	return loadBitmaps().titleLight
}

// Importer 返回导入来源的图标：potatovn、playnite、yukihub、vnite、reinamanager。
func Importer(name string) *ui.Bitmap { return loadBitmaps().importers[name] }

// lunaArt 是“添加游戏”弹窗两张选项卡片上的插画（branding/luna1.webp、luna2.webp），用到时才解码。
var lunaArt = sync.OnceValue(func() [2]*ui.Bitmap {
	return [2]*ui.Bitmap{bitmap("luna1.webp", 640), bitmap("luna2.webp", 640)}
})

// Luna 返回插画：0 是本地游戏，1 是远程游戏。
func Luna(i int) *ui.Bitmap { return lunaArt()[i] }

// bitmap 解码内嵌图片并缩到 maxWidth 像素宽，避免 1024px 的图标常驻显存。
func bitmap(name string, maxWidth int) *ui.Bitmap {
	data, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return ui.NewBitmap(Downscale(img, maxWidth))
}

// 元数据来源的图标，对应 utils/metadataSources.ts 的 METADATA_SOURCE_ICONS 与 METADATA_SOURCE_COMPACT_ICONS。
var providerFiles = map[string][2]string{ // 来源 → {logo, compact}
	"bangumi":      {"bangumi-logo.png", "bangumi-icon.png"},
	"vndb":         {"vndb-logo.svg", "vndb-logo.svg"},
	"ymgal":        {"ymgal-logo.png", "ymgal-logo.png"},
	"steam":        {"steam-logo.png", "steam-logo.png"},
	"dlsite":       {"dlsite-logo.png", "dlsite-logo.png"},
	"touchgal":     {"touchgal-logo.webp", "touchgal-logo.webp"},
	"hikarinagi":   {"hikarinagi-logo.svg", "hikarinagi-icon.webp"},
	"kungal":       {"kungal-logo.webp", "kungal-logo.webp"},
	"erogamescape": {"erogamescape-logo.png", "erogamescape-logo.png"},
}

var (
	providersMu sync.Mutex
	providers   = map[string]ui.ImageSource{}
)

// Provider 对应 getMetadataSourceIcon：来源（enums.SourceType 的值）的图标，compact 为紧凑版；没有图标时为 nil。
func Provider(source string, compact bool) ui.ImageSource {
	pair, ok := providerFiles[source]
	if !ok {
		return nil
	}
	name := pair[0]
	if compact {
		name = pair[1]
	}
	providersMu.Lock()
	defer providersMu.Unlock()
	if img, ok := providers[name]; ok {
		return img
	}
	var img ui.ImageSource
	if strings.HasSuffix(name, ".svg") {
		data, err := files.ReadFile("providers/" + name)
		if err != nil {
			panic(err)
		}
		img = ui.MustParseSVG(data)
	} else {
		img = bitmap("providers/"+name, 96)
	}
	providers[name] = img
	return img
}

// ProviderFile 返回来源图标的原始文件（providers/ 下的文件名），给需要自己处理图片的地方用
// （编辑页把 logo 做成 brightness-0 的黑白轮廓）。
func ProviderFile(name string) ([]byte, error) { return files.ReadFile("providers/" + name) }
