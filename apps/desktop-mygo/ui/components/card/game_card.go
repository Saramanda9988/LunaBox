// Package card 对应原版 frontend/src/components/card：游戏卡片与加载占位卡片。
package card

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// CoverRatio 是封面高宽比：竖版 aspect-[3/3.6]，横版 aspect-video。
func CoverRatio(landscape bool) float32 {
	if landscape {
		return 9.0 / 16
	}
	return 3.6 / 3
}

// GameCardProps 是 GameCard 的输入。
type GameCardProps struct {
	Game      *models.Game
	Width     float32
	Landscape bool
	Cover     *ui.Bitmap // 已解码的封面，未就绪为 nil
	NoCover   bool       // 没有封面或加载失败，显示 i-mdi-image-off
	SortField enums.GameListSortBy
	BlurNSFW  bool   // config.blur_nsfw_game_covers：NSFW 游戏的封面模糊
	Highlight string // 搜索词，标题与开发商里命中的部分高亮

	Batch, Selected bool
	// Running、Ending 对应 gameRuntimes 里有这个游戏、state === "ending"：开始游戏按钮禁用，图标换成游戏手柄或加载中。
	Running, Ending bool
	OnToggleSelect  func()
	OnPlay          func()
	OnDetail        func()

	// Overlay 对应 VirtualGameGrid 的 renderOverlay：叠在卡片最上层（相对卡片的边框盒定位），
	// hover 是悬停（group-hover）的 transition-opacity 进度。
	Overlay func(hover float32)
}

// GameCard 对应 GameCard.tsx：rounded-xl border 卡片，封面 + 标题/开发商，悬停显示开始游戏与详情按钮；
// 批量模式下点击切换选中，左上角显示勾选框。
func GameCard(c *ui.Context, t *theme.Theme, p GameCardProps) {
	g := p.Game
	// 玻璃模式：data-glass:border-white/22 data-glass:bg-transparent data-glass:hover:border-white/35
	// （dark:border-white/12 dark:hover:border-white/22）。
	bg, border, borderHover := t.CardBg, t.CardBorder, t.CardBorderHover
	if t.Glass {
		bg = ui.Transparent
		border, borderHover = t.Pick(theme.White.Alpha(0.22), theme.White.Alpha(0.12)), t.Pick(theme.White.Alpha(0.35), theme.White.Alpha(0.22))
	}
	card := ui.Column(c).Width(p.Width).Shrink(0).Radius(12).Clip().Background(bg).Border(t.HL, border)
	hovered := card.Hovered()
	if hovered {
		card.BorderColor(borderHover)
	}
	card.Transition(theme.ColorsTransition(200 * time.Millisecond))
	selected := p.Selected
	if p.Batch {
		card.Cursor(ui.CursorPointer)
		if card.Clicked() && p.OnToggleSelect != nil {
			p.OnToggleSelect()
			selected = !selected
		}
	}
	overlay := card.AnimateWith("hover", theme.Bool(hovered && !p.Batch), 200*time.Millisecond, theme.EaseTailwind)

	card.Children(func() {
		if p.Batch {
			selectionBadge(c, t, selected, p.OnToggleSelect)
		}
		innerW := p.Width - 2*t.HL
		// 嵌套裁剪时 MyGo 只在着色器里按最内层裁剪的形状切，外层只用矩形 scissor，
		// 所以封面自己的裁剪要带上卡片内侧的顶部圆角，否则图片会盖住卡片圆角。
		innerR := 12 - t.HL
		ui.Box(c).Height(innerW*CoverRatio(p.Landscape)).Radius(innerR, innerR, 0, 0).Clip().Background(t.CoverBg).Shrink(0).Children(func() {
			switch {
			case p.Cover != nil:
				img := ui.Image(c, p.Cover).Fill().Fit(ui.Cover)
				if g.IsNSFW && p.BlurNSFW {
					components.CoverBlur(c, card, img, false)
				}
			case p.NoCover:
				ui.Box(c).Fill().Center().Children(func() {
					components.Icon(c, "image-off", 36, theme.Brand400)
				})
			}

			if g.Status == enums.StatusCompleted {
				ui.Box(c).Absolute().Top(6).Right(6).Size(24, 24).Radius(999).Center().Background(theme.Yellow500).
					Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1)).
					Children(func() { components.Icon(c, "trophy", 14, theme.White) })
			}

			if s := SortFieldText(g, p.SortField); s != "" {
				sortStrip(c, s)
			}

			if !p.Batch {
				hoverOverlay(c, p, overlay)
			}
		})

		pad := []float32{4, 8, 8, 8} // px-2 pt-1 pb-2
		if p.Landscape {
			pad = []float32{8, 12, 12, 12} // px-3 pt-2 pb-3
		}
		// 玻璃模式：data-glass:bg-white/8 data-glass:dark:bg-black/12。原版还有 backdrop-blur-24（暗色 12）与 backdrop-saturate，
		// 这里没有照搬：一屏几十张卡片各自在 GPU 上模糊一次，滚动时开销太大，背景图本身已经模糊过。
		ui.Column(c).Padding(pad...).Background(t.IfGlass(t.CardBg, theme.White.Alpha(0.08), theme.Black.Alpha(0.12))).Children(func() {
			components.HighlightText(c, t, g.Name, p.Highlight, theme.FontStep{Size: 14, Line: 17.5}, theme.WeightBold, t.TextStrong)
			company := g.Company
			if company == "" {
				company = "Unknown Developer"
			}
			components.HighlightText(c, t, company, p.Highlight, theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal, t.TextMuted)
		})
		if p.Batch && selected {
			// ring-2 ring-inset ring-neutral-500 dark:ring-neutral-400。CSS 的内阴影画在子元素下面，会被封面与信息区的
			// 背景盖住，只露出一部分；这里画在内容之上，整圈都看得见。
			ui.Box(c.Key("selected-ring")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(12-t.HL).
				Border(2, t.Pick(theme.Neutral500, theme.Neutral400)).PassThrough()
		}
		if p.Overlay != nil {
			p.Overlay(card.AnimateWith("overlay", theme.Bool(hovered), 150*time.Millisecond, theme.EaseTailwind))
		}
	})
}

// sortStrip 是封面底部的排序字段覆盖条：bg-gradient-to-t from-black/75 via-black/45 to-transparent px-2 pt-5 pb-1.5。
func sortStrip(c *ui.Context, s string) {
	// 高度 = pt-5 + 16 + pb-1.5 = 42。三段渐变拆成上下两半；全部绝对定位，按顺序绘制，文字在最上层。
	const h = 20 + 16 + 6
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(h).PassThrough().Children(func() {
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(h / 2).
			LinearGradient(ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0), To: ui.RGBA(0, 0, 0, 0.45), Angle: 180, Oklab: true})
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(h / 2).
			LinearGradient(ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0.45), To: ui.RGBA(0, 0, 0, 0.75), Angle: 180, Oklab: true})
		ui.Box(c).Absolute().Left(8).Right(8).Bottom(6).Children(func() {
			components.Text(c, s, theme.TextXS, theme.WeightSemibold, theme.White).SingleLine()
		})
	})
}

// hoverOverlay：absolute inset-0 bg-black/50，开始游戏 / 详情两个圆形按钮。游戏运行时开始游戏按钮
// disabled:opacity-65 disabled:hover:scale-100，图标是 i-mdi-gamepad-variant，结束中是 i-mdi-loading animate-spin。
func hoverOverlay(c *ui.Context, p GameCardProps, opacity float32) {
	ov := ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(8).
		Background(ui.RGBA(0, 0, 0, 0.5)).Opacity(opacity)
	if opacity == 0 {
		ov.Invisible()
	}
	ov.Children(func() {
		icon := "play"
		switch {
		case p.Ending:
			icon = "loading"
		case p.Running:
			icon = "gamepad-variant"
		}
		play := roundOverlayButton(c, "play", icon, theme.Neutral600, theme.Neutral500, true, p.Running)
		play.Label(locales.GameCardStartGame())
		if p.OnPlay != nil && !p.Running {
			play.OnClick(p.OnPlay)
		}
		info := roundOverlayButton(c, "info", "information-variant", theme.White.Alpha(0.2), theme.White.Alpha(0.3), false, false)
		info.Label(locales.NativeViewDetails())
		if p.OnDetail != nil {
			info.OnClick(p.OnDetail)
		}
	})
}

// roundOverlayButton：h-8 w-8 rounded-full，悬停换底色。icon 为 loading 时是旋转的加载图标。
// 不做悬停缩放：圆角裁剪的封面里一有变换，MyGo 就要为封面与卡片各光栅化一张裁剪蒙版，扫过一排卡片内存就一路上涨。
func roundOverlayButton(c *ui.Context, key, icon string, bg, bgHover ui.Color, shadow, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(999).Center().FocusRing(false)
	btn.Background(bg)
	switch {
	case disabled:
		components.Disable(btn, 0.65)
	case btn.Hovered():
		btn.Background(bgHover)
	}
	if shadow {
		btn.Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
	}
	btn.Children(func() {
		if icon == "loading" {
			components.Spinner(c, 18, theme.White)
		} else {
			components.Icon(c, icon, 18, theme.White)
		}
	})
	return btn
}

// selectionBadge：批量模式下左上角的圆形勾选框。
func selectionBadge(c *ui.Context, t *theme.Theme, selected bool, onToggle func()) {
	badge := ui.ButtonBase(c.Key("select")).Absolute().Left(8).Top(8).Size(24, 24).Radius(999).Center().
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).FocusRing(false)
	color := ui.Transparent
	if selected {
		badge.Background(theme.Neutral600).Border(t.HL, theme.Neutral600)
		color = theme.White
	} else {
		badge.Background(t.Pick(theme.White.Alpha(0.9), theme.Brand800.Alpha(0.9))).
			Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	}
	if onToggle != nil {
		badge.OnClick(onToggle)
	}
	badge.Children(func() {
		components.Icon(c, "check", 14, color)
	})
}

// Placeholder 对应 GameCardPlaceholder：数据未到达的格子，以及加载中的骨架卡片。
func Placeholder(c *ui.Context, t *theme.Theme, width float32, landscape bool) {
	bg, border := t.GlassCard(t.CardBg, t.CardBorder) // glass-card
	card := ui.Column(c).Width(width).Shrink(0).Radius(12).Clip().Background(bg).Border(t.HL, border).PassThrough()
	card.Opacity(components.Pulse(card)).Children(func() {
		ui.Box(c).Height((width - 2*t.HL) * CoverRatio(landscape)).Background(t.CoverBg.Alpha(0.8))
		ui.Column(c).Padding(4, 8, 8, 8).Gap(4).Children(func() {
			ui.Box(c).Height(16).WidthPercent(80).Radius(4).Background(t.CoverBg)
			ui.Box(c).Height(12).WidthPercent(60).Radius(4).Background(t.CoverBg.Alpha(0.8))
		})
	})
}

// SortFieldText 对应 formatSortFieldValue：名称、开发商已在卡片底部展示，不再覆盖。
func SortFieldText(g *models.Game, sortBy enums.GameListSortBy) string {
	switch sortBy {
	case enums.GameListSortByLastPlayedAt:
		if g.LastPlayedAt == nil {
			return locales.CommonNever()
		}
		return timefmt.Date(*g.LastPlayedAt, time.Local)
	case enums.GameListSortByCreatedAt:
		return timefmt.Date(g.CreatedAt, time.Local)
	case enums.GameListSortByRating:
		return fmt.Sprintf("%.1f/10.0", g.Rating)
	case enums.GameListSortByReleaseDate:
		if g.ReleaseDate == "" {
			return locales.CommonUnknownDate()
		}
		return g.ReleaseDate
	}
	return ""
}
