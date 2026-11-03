package components

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// ImportModalProps 是 ImportModalContainer 的属性。
type ImportModalProps struct {
	Open      bool
	Title     string
	Icon      string   // iconClassName 里的 i-mdi-* 图标
	IconSize  float32  // iconClassName 的 text-*（如 text-3xl 是 30）
	IconColor ui.Color // iconClassName 的 text-*-*
	// Image 是 GameImportModal 的 iconSrc（h-9 w-9 shrink-0 object-contain），优先于 Icon。
	Image     ui.ImageSource
	TopOffset float32
	// MaxWidth 是 maxWidthClassName；0 为默认的 max-w-4xl xl:max-w-5xl 2xl:max-w-7xl（按窗口宽度 896、1024、1280）。
	MaxWidth float32
	OnClose  func()
}

// ImportModalContainer 对应 components/ui/import/ImportModalContainer.tsx：遮罩 absolute inset-0 z-50 flex items-center
// justify-center bg-black/50 backdrop-blur-sm（没有 p-4，也不能点遮罩关闭），面板 flex max-h-[90vh] w-full max-w-* flex-col
// rounded-xl bg-white shadow-2xl transition-[max-width] duration-200 dark:bg-brand-800；页头 flex items-center justify-between
// border-b p-6 是图标、text-2xl font-bold 的标题与关闭按钮，内容区 flex-1 overflow-y-auto p-6。要在页面最后构建。
func ImportModalContainer(c *ui.Context, t *theme.Theme, key string, p ImportModalProps, content func()) {
	if !p.Open {
		return
	}
	winW, winH := c.Size()
	maxW := p.MaxWidth
	if maxW == 0 {
		switch {
		case winW >= 1536:
			maxW = 1280
		case winW >= 1280:
			maxW = 1024
		default:
			maxW = 896
		}
	}
	Overlay(c, t, func() {
		host := ui.Box(c.Key(key)).Absolute().Top(p.TopOffset).Left(0).Right(0).Bottom(0).Modal()
		host.Children(func() {
			ui.Box(c.Key("backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Material(glass.Blur{Radius: 4}).Children(func() {
				ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 0.5))
			})
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
				pnl := ui.Column(c.Key("panel")).FillWidth()
				sb := ui.Local(pnl, "scroll", func() ScrollBar { return ScrollBar{} })
				panelMaxH := winH * 0.9
				pnl.MaxWidth(pnl.AnimateWith("max-width", maxW, 200*time.Millisecond, theme.EaseTailwind)).MaxHeight(panelMaxH).
					Radius(12).Background(t.PanelBg).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25))
				pnl.Children(func() {
					// 页头：p-6，内容最高是 text-2xl 的行高 32。
					header := float32(24*2+32) + t.HL
					ui.Row(c.Key("header")).FillWidth().Padding(24).AlignItems(ui.Center).Justify(ui.SpaceBetween).
						BorderWidth(0, 0, t.HL, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
						ui.Row(c).Gap(12).AlignItems(ui.Center).MinWidth(0).Children(func() {
							switch {
							case p.Image != nil:
								ui.Image(c, p.Image).Size(36, 36).Fit(ui.Contain).Shrink(0)
							case p.Icon != "":
								Icon(c, p.Icon, p.IconSize, p.IconColor)
							}
							Text(c, p.Title, theme.Text2XL, theme.WeightBold, t.TextStrong)
						})
						importModalCloseButton(c, t, p.OnClose)
					})
					gutter := float32(0)
					if sb.Overflowing() {
						gutter = t.Gutter()
					}
					ScrollArea(c, t, "body", sb, func(w ui.Element) { w.FillWidth().MaxHeight(max(0, panelMaxH-header)) }, func(sc ui.Element) {
						sc.Children(func() {
							ui.Column(c).FillWidth().Padding(24, 24+gutter, 24, 24).Children(content)
						})
					})
				})
			})
		})
	})
}

// importModalCloseButton 是 i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700
// dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200 的按钮。按钮本身就是图标（mask 填满 1em 见方的
// border-box，p-1 不缩小图标）；mask 图标的颜色是 background-color: currentColor，hover:bg-* 的优先级更高，所以悬停时
// 图标变成 brand-100（深色 brand-700），文字颜色的 hover 不起作用。
func importModalCloseButton(c *ui.Context, t *theme.Theme, onClose func()) {
	btn := ui.ButtonBase(c.Key("close")).Size(24, 24).Center().Shrink(0).FocusRing(false).Label(locales.CommonClose()) // 原版没有无障碍名称
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand100, theme.Brand700)
	}
	if onClose != nil {
		btn.OnClick(onClose)
	}
	btn.Children(func() { Icon(c, "close", 24, color) })
}
