package components

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/theme"
)

// BrandAside 是启动窗（StartupWindow.tsx）与初始设置向导左侧的图标区：relative flex shrink-0 items-center
// justify-center overflow-hidden border-r border-brand-200 bg-brand-100 dark:border-white/8 dark:bg-brand-900，
// 中间是图标与标题图。width 是 w-[282px] 之类的宽度。背景是流动的 Warp 着色器（原生版新增，取代原版的两个装饰圆环）；
// 没有 GPU 加速时（含测试）只是纯色背景。
func BrandAside(c *ui.Context, t *theme.Theme, width float32) {
	ui.Column(c.Key("brand-aside")).Width(width).Shrink(0).Center().Clip().Background(t.Pick(theme.Brand100, theme.Brand900)).
		BorderWidth(0, t.HL, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.White.Alpha(0.08))).
		Draw(func(p *ui.Painter, r ui.Rect) { drawWarp(p, r, t) }).
		Children(func() { brandMark(c, t) })
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
