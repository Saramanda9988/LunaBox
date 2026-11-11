package components

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/theme"
)

// BrandAside 是启动窗（StartupWindow.tsx）左侧的图标区，初始设置向导也用它：relative flex shrink-0 items-center
// justify-center overflow-hidden border-r border-brand-200 bg-brand-100 dark:border-white/8 dark:bg-brand-900，
// 两个装饰圆环，中间是图标与标题图。width 是 w-[282px] 之类的宽度。animated 时背景是流动的 Warp 着色器、没有圆环
// （原生版新增，初始设置向导用）；没有 GPU 加速时（含测试）只是纯色背景。
func BrandAside(c *ui.Context, t *theme.Theme, width float32, animated bool) {
	aside := ui.Column(c.Key("brand-aside")).Width(width).Shrink(0).Center().Clip().Background(t.Pick(theme.Brand100, theme.Brand900)).
		BorderWidth(0, t.HL, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.White.Alpha(0.08)))
	if animated {
		aside.Draw(func(p *ui.Painter, r ui.Rect) { drawWarp(p, r, t) })
	}
	aside.Children(func() {
		if animated {
			// 图标与标题图直接放在流动的背景上，不要圆环。
			brandMark(c, t)
			return
		}
		// -left-30 -top-34 h-72 w-72 rounded-full border border-primary-300/25 dark:border-primary-400/12
		ui.Box(c).Absolute().Left(-120).Top(-136).Size(288, 288).Radius(999).PassThrough().
			Border(t.HL, t.Pick(theme.Primary300.Alpha(0.25), theme.Primary400.Alpha(0.12)))
		// -bottom-38 -right-32 h-72 w-72 rounded-full border border-primary-300/18 dark:border-primary-400/10
		ui.Box(c).Absolute().Right(-128).Bottom(-152).Size(288, 288).Radius(999).PassThrough().
			Border(t.HL, t.Pick(theme.Primary300.Alpha(0.18), theme.Primary400.Alpha(0.10)))
		brandMark(c, t)
	})
}

// brandMark：relative flex -translate-y-1 flex-col items-center gap-5，图标与标题图。
func brandMark(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("mark")).AlignItems(ui.Center).Gap(20).Translate(0, -4).Children(func() {
		ui.Image(c, assets.AppIcon(t.Dark)).Size(96, 96).Radius(23)
		title := assets.Title(t.Dark)
		tw, th := title.Size()
		ui.Image(c, title).Size(152, 152*float32(th)/float32(tw)) // w-38 h-auto
	})
}
