package components

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// Overlay 是 ui.Overlay，并在浮层里关掉玻璃模式（theme.WithoutGlass）：原版的弹窗、抽屉与弹出层都挂在
// data-glass="false" 的 ModalPortal 里或 body 上，不随自定义背景变成半透明。
func Overlay(c *ui.Context, t *theme.Theme, fn func()) {
	ui.Overlay(c, func() { t.WithoutGlass(fn) })
}

// ModalProps 是弹窗外壳的属性。
type ModalProps struct {
	Open      bool
	TopOffset float32 // 原版的 #app-modal-root 在顶栏下方，遮罩从这里开始
	MaxWidth  float32 // 面板的 max-w-*，默认 max-w-md（448）
	OnClose   func()  // 非空时 Esc 与点击遮罩关闭
	Label     string  // 遮罩的无障碍名称（关闭）
	Dim       float32 // 遮罩的不透明度，默认 bg-black/50（0.5）
}

// Modal 对应 ModalPortal 加原版弹窗的公共外壳：absolute inset-0 z-50 flex items-center justify-center
// bg-black/50 backdrop-blur-sm p-4。panel 是居中的 w-full max-w-* 容器，由 build 设置样式并构建内容
// （常用 ModalPanel）。要在页面最后构建，浮层才会盖住页面内容。
func Modal(c *ui.Context, t *theme.Theme, key string, p ModalProps, build func(panel ui.Element)) {
	if !p.Open {
		return
	}
	maxW := p.MaxWidth
	if maxW == 0 {
		maxW = 448
	}
	dim := p.Dim
	if dim == 0 {
		dim = 0.5
	}
	Overlay(c, t, func() {
		host := ui.Box(c.Key(key)).Absolute().Top(p.TopOffset).Left(0).Right(0).Bottom(0).Modal()
		if p.OnClose != nil {
			host.OnShortcut(0, ui.KeyEscape, p.OnClose) // Modal 把键盘留在浮层里，Esc 要挂在浮层上
		}
		host.Children(func() {
			backdrop := ui.ButtonBase(c.Key("backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Material(glass.Blur{Radius: 4}).FocusRing(false).Label(p.Label)
			if p.OnClose != nil {
				backdrop.OnClick(p.OnClose)
			}
			backdrop.Children(func() {
				ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, dim))
			})
			// 绝对定位的遮罩画在流内元素之上，所以居中的容器也绝对定位，排在遮罩之后。
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Padding(16).PassThrough().Children(func() {
				panel := ui.Column(c.Key("panel")).FillWidth().MaxWidth(maxW)
				build(panel)
			})
		})
	})
}

// ModalPanel 是最常见的面板样式：rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border。
func ModalPanel(t *theme.Theme, panel ui.Element) ui.Element {
	return panel.Radius(12).Padding(24).Background(t.PanelBg).Border(t.HL, t.PanelBorder).
		Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
}

// ConfirmProps 是 ConfirmModal 的属性。
type ConfirmProps struct {
	Open        *bool
	Title       string
	Message     string
	ConfirmText string // 默认“确定”（common.confirm）
	CancelText  string // 默认“取消”
	Danger      bool   // type="danger"
	TopOffset   float32
	OnConfirm   func()
}

// ConfirmModal 对应 ConfirmModal.tsx：左侧圆形图标，标题 text-xl font-bold，说明 text-sm leading-relaxed，
// 右下角取消与确认按钮（mt-8 gap-3）。确认后关闭。
func ConfirmModal(c *ui.Context, t *theme.Theme, key string, p ConfirmProps) {
	closeModal := func() { *p.Open = false }
	Modal(c, t, key, ModalProps{Open: *p.Open, TopOffset: p.TopOffset}, func(panel ui.Element) {
		ModalPanel(t, panel).Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				icon, bg, fg := "information", t.Pick(theme.Neutral100, ui.Hex("#0f172a").Alpha(0.3)), t.Pick(theme.Neutral600, theme.Neutral400)
				if p.Danger {
					icon, bg, fg = "alert-circle", t.Pick(ui.Hex("#fee2e2"), ui.Hex("#7f1d1d").Alpha(0.3)), t.Pick(theme.Error600, theme.Error400)
				}
				ui.Box(c).Padding(8).Radius(999).Background(bg).Shrink(0).Children(func() {
					Icon(c, icon, 24, fg)
				})
				ui.Column(c).Grow(1).Basis(0).Children(func() {
					TextMargin(c, p.Title, theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 8, 0)
					Text(c, p.Message, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
				})
			})
			ui.Row(c).Margin(32, 0, 0, 0).Gap(12).Justify(ui.End).Children(func() {
				cancel := p.CancelText
				if cancel == "" {
					cancel = locales.CommonCancel()
				}
				ModalTextButton(c, t, "cancel", cancel).OnClick(closeModal)
				confirm := p.ConfirmText
				if confirm == "" {
					confirm = locales.CommonConfirm()
				}
				ModalPrimaryButton(c, t, "confirm", confirm, p.Danger).OnClick(func() {
					if p.OnConfirm != nil {
						p.OnConfirm()
					}
					closeModal()
				})
			})
		})
	})
}

// ModalTextButton 是弹窗的取消按钮：px-4 py-2 text-sm font-medium rounded-lg，悬停 bg-brand-100 dark:bg-brand-700。
func ModalTextButton(c *ui.Context, t *theme.Theme, key, label string) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 16).Radius(8).FocusRing(false)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		Text(c, label, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300)).NoWrap()
	})
	return btn
}

// ModalPrimaryButton 是弹窗的确认按钮：px-4 py-2 text-sm font-medium text-white rounded-lg shadow-sm，
// bg-neutral-600（danger 时 bg-error-600），浅色主题带同色系的淡阴影。
func ModalPrimaryButton(c *ui.Context, t *theme.Theme, key, label string, danger bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 16).Radius(8).FocusRing(false)
	bg, hover, shadow := theme.Neutral600, theme.Neutral700, ui.Hex("#e2e8f0")
	if danger {
		bg, hover, shadow = theme.Error600, theme.Error700, ui.Hex("#fecaca")
	}
	if btn.Hovered() {
		bg = hover
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !t.Dark {
		btn.Shadow(0, 1, 2, 0, shadow)
	}
	btn.Children(func() {
		Text(c, label, theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}
