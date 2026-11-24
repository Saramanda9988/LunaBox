package bar

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/theme"
)

// brandWordmark 返回侧边栏 / 顶栏的字标：主题插件通过 brand.wordmark 替换的图片，否则是内置的 LunaBox 字标；
// 主题声明 hideWordmark 时为 nil。插件图片读取失败时回退到内置字标。
func brandWordmark(t *theme.Theme) ui.ImageSource {
	if t.HideBrandWordmark {
		return nil
	}
	if img := assets.LocalImage(t.BrandWordmark, 520); img != nil {
		return img
	}
	return assets.Title(t.Dark)
}

// brandIcon 返回侧边栏的应用图标：主题插件通过 brand.icon 替换的图片，否则是内置图标；主题声明 hideIcon 时为 nil。
func brandIcon(t *theme.Theme) ui.ImageSource {
	if t.HideBrandIcon {
		return nil
	}
	if img := assets.LocalImage(t.BrandIcon, 96); img != nil {
		return img
	}
	return assets.AppIcon(t.Dark)
}

// brandImage 按高度 h 等比绘制品牌图片（宽度由图片自身的宽高比决定）。
func brandImage(c *ui.Context, src ui.ImageSource, h float32) ui.Element {
	w, ih := ui.ImageSize(src)
	if ih <= 0 {
		w, ih = 1, 1
	}
	return ui.Image(c, src).Size(h*w/ih, h)
}
