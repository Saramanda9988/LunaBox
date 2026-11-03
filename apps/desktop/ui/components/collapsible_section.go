package components

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// CollapsibleSectionProps 是 CollapsibleSection 的属性。
type CollapsibleSectionProps struct {
	Title string
	Icon  string // i-mdi-* 图标名
	Open  *bool  // 页面持有的展开状态，初值即 defaultOpen
}

// CollapsibleSection 对应 components/ui/CollapsibleSection.tsx：overflow-hidden rounded-xl border border-brand-200 bg-brand-50
// dark:border-brand-700 dark:bg-brand-800；按钮 flex w-full items-center justify-between p-4，标题 text-lg font-semibold
// text-brand-900 dark:text-white，左侧 text-xl text-neutral-500 dark:text-neutral-400 的图标（gap-2），右侧
// i-mdi-chevron-down text-xl text-brand-500，展开时 rotate-180（duration-200）；内容 grid-rows 0fr ↔ 1fr 与 opacity 同时过渡
// 200ms，space-y-4 p-5。
//
// 原版第一次展开之后才挂载内容（hasOpened），收起后仍保留；这里收起动画结束后就不再构建内容，面板的状态由页面持有，
// 再次展开时不会丢失。content 构建 space-y-4 的列里的各项。返回 <section> 元素。
func CollapsibleSection(c *ui.Context, t *theme.Theme, key string, p CollapsibleSectionProps, content func()) ui.Element {
	bg, border := t.GlassSection(t.Pick(theme.Brand50, theme.Brand800), t.Pick(theme.Brand200, theme.Brand700)) // glass-settings-section
	return ui.Column(c.Key(key)).FillWidth().Radius(12).Clip().Shrink(0).
		Background(bg).Border(t.HL, border).
		Children(func() {
			parts := ui.CollapsibleBase(c, p.Open)
			progress := parts.Progress()
			// 标题按钮：data-glass:bg-white/20 data-glass:dark:bg-black/20
			parts.Trigger.FillWidth().Padding(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Label(p.Title).
				Background(t.IfGlass(ui.Transparent, theme.White.Alpha(0.2), theme.Black.Alpha(0.2))).
				Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
						Icon(c, p.Icon, 20, t.Pick(theme.Neutral500, theme.Neutral400))
						Text(c, p.Title, theme.TextLG, theme.WeightSemibold, t.TextStrong)
					})
					Icon(c, "chevron-down", 20, theme.Brand500).Rotate(180 * progress)
				})
			parts.Panel(func() {
				ui.Column(c.Key("content")).FillWidth().Padding(20).Gap(16).Opacity(progress).Children(content)
			})
		})
}
