package card

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// CategoryCard 的网格：grid-cols-[repeat(auto-fill,minmax(min(11rem,100%),1fr))] gap-4，卡片 max-w-48。
const (
	CategoryMinWidth = 176 // 11rem
	CategoryMaxWidth = 192 // max-w-48
	CategoryGap      = 16  // gap-4
	categoryTitleGap = 10  // mt-2.5
)

// CategoryCardHeight 是宽 w 的卡片的高度：aspect-[4/5] 的封面区加 mt-2.5 与一行 text-sm 的标题。
func CategoryCardHeight(w float32) float32 { return w*5/4 + categoryTitleGap + theme.TextSM.Line }

// coverSlot 是 COVER_STACK_SLOTS 的一项：基础变换（baseClass）与悬停时外层再加的变换（hoverClass）。
type coverSlot struct {
	key                 string
	front               bool
	scale, rotate, x, y float32
	hoverRotate, hx, hy float32
}

// coverSlots 对应 COVER_STACK_SLOTS，顺序即层叠顺序（z-0、z-10、z-20）。
var coverSlots = []coverSlot{
	// -rotate-10 -translate-x-4 translate-y-1 scale-[0.82]；悬停 -rotate-7 -translate-x-[13px] translate-y-1
	{key: "back", scale: 0.82, rotate: -10, x: -16, y: 4, hoverRotate: -7, hx: -13, hy: 4},
	// rotate-1 scale-90；悬停 rotate-1 -translate-x-[3px] -translate-y-0.5
	{key: "middle", scale: 0.9, rotate: 1, hoverRotate: 1, hx: -3, hy: -2},
	// rotate-10 translate-x-4 translate-y-1；悬停 rotate-7 translate-x-[13px] translate-y-1
	{key: "front", front: true, scale: 1, rotate: 10, x: 16, y: 4, hoverRotate: 7, hx: 13, hy: 4},
}

// CategoryCardProps 是 CategoryCard 的输入。
type CategoryCardProps struct {
	Category vo.CategoryVO
	Width    float32
	Covers   func(g *models.Game) *ui.Bitmap // 预览游戏的封面（封面缓存），未就绪为 nil
	BlurNSFW bool

	SelectionMode     bool
	Selected          bool
	SelectionDisabled bool // 系统收藏夹不能批量选择
	OnSelectChange    func(selected bool)
	OnOpen            func()
}

// CategoryName 是收藏夹显示的名称：系统收藏夹显示为“最喜欢的游戏”（categories.favorites）。
func CategoryName(cat vo.CategoryVO) string {
	if cat.IsSystem {
		return locales.CategoriesFavorites()
	}
	return cat.Name
}

// CategoryCard 对应 CategoryCard.tsx：aspect-[4/5] rounded-xl 的封面区里三张错开旋转的预览封面（悬停时展开），
// 右下角游戏数量；下面 mt-2.5 居中的表情与名称。批量模式下点击切换选中，右上角是勾选按钮。
func CategoryCard(c *ui.Context, t *theme.Theme, p CategoryCardProps) {
	cat := p.Category
	name := CategoryName(cat)
	w := p.Width
	h := w * 5 / 4
	locked := p.SelectionMode && p.SelectionDisabled
	click := func() {
		if !p.SelectionMode {
			if p.OnOpen != nil {
				p.OnOpen()
			}
			return
		}
		if !p.SelectionDisabled && p.OnSelectChange != nil {
			p.OnSelectChange(!p.Selected)
		}
	}

	article := ui.Column(c).Width(w).Shrink(0)
	// group-hover / group-focus-within：预览封面展开（transition-transform duration-500 ease-out）
	fan := article.AnimateWith("fan", theme.Bool(article.Hovered() || article.FocusWithin()), 500*time.Millisecond, theme.EaseOut)
	article.Children(func() {
		// bg-brand-150/65 dark:bg-brand-800/55，data-glass:bg-brand-100/35 data-glass:dark:bg-black/12
		box := ui.Box(c.Key("cover")).Size(w, h).Radius(12).
			Background(t.IfGlass(t.Pick(theme.Brand150.Alpha(0.65), theme.Brand800.Alpha(0.55)), theme.Brand100.Alpha(0.35), theme.Black.Alpha(0.12)))
		if p.SelectionMode && p.Selected {
			// ring-2 ring-primary-500 ring-offset-2 ring-offset-white（dark：ring-primary-400 ring-offset-brand-900）
			box.Shadow(0, 0, 0, 2, t.Pick(theme.White, theme.Brand900)).Shadow(0, 0, 0, 4, t.Pick(theme.Primary500, theme.Primary400))
		}
		box.Children(func() {
			btn := ui.ButtonBase(c.Key("open")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(12).FocusRing(false).Label(name)
			if locked {
				btn.Opacity(0.7).Cursor(ui.CursorNotAllowed)
			}
			btn.OnClick(click).Children(func() {
				coverStack(c, t, p, w, h, fan)
				countBadge(c, t, cat.GameCount)
			})
			if p.SelectionMode {
				selectButton(c, t, p, name)
			}
		})

		// mt-2.5 block w-full text-center；h3 flex items-center justify-center gap-1.5 text-sm font-medium
		title := ui.ButtonBase(c.Key("title")).FillWidth().Margin(categoryTitleGap, 0, 0, 0).Gap(6).Center().FocusRing(false).Label(name)
		if locked {
			title.Opacity(0.7).Cursor(ui.CursorNotAllowed)
		}
		title.OnClick(click).Children(func() {
			if e := strings.TrimSpace(cat.Emoji); e != "" {
				components.EmojiText(c, e, theme.TextSM.Size).Shrink(0)
			}
			components.Text(c, name, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand900, theme.Brand100)).SingleLine().MinWidth(0).Shrink(1)
		})
	})
}

// coverStack 是三张预览封面：不足三张时重复（1 张时 [0,0,0]，2 张时 [0,0,1]），没有封面时是空白卡片，
// 最前面一张显示图标。每张 absolute left-[47%] top-1/2 aspect-[2/3] w-[68%]，居中定位。
func coverStack(c *ui.Context, t *theme.Theme, p CategoryCardProps, w, h, fan float32) {
	var games []vo.CategoryPreviewGameVO
	for _, g := range p.Category.PreviewGames {
		if g.CoverURL != "" || g.CoverSourceURL != "" {
			games = append(games, g)
		}
		if len(games) == 3 {
			break
		}
	}
	switch len(games) {
	case 1:
		games = []vo.CategoryPreviewGameVO{games[0], games[0], games[0]}
	case 2:
		games = []vo.CategoryPreviewGameVO{games[0], games[0], games[1]}
	}
	sw := w * 0.68
	sh := sw * 3 / 2
	for i, s := range coverSlots {
		slot := ui.Box(c.Key(s.key)).Absolute().Left(w*0.47-sw/2).Top(h/2-sh/2).Size(sw, sh).PassThrough()
		slot.Children(func() {
			// 外层是悬停时的变换，内层是基础变换：rotate、translate 按 transform 的顺序（先缩放、再旋转、再平移）。
			hover := ui.Box(c).Fill().Rotate(s.hoverRotate*fan).Translate(s.hx*fan, s.hy*fan)
			hover.Children(func() {
				// rounded-lg border-[3px] border-white overflow-hidden shadow-md shadow-black/15（dark：border-brand-900 shadow-black/30）
				shadow := ui.RGBA(0, 0, 0, 0.15)
				if t.Dark {
					shadow = ui.RGBA(0, 0, 0, 0.3)
				}
				card := ui.Box(c).Fill().Radius(8).Border(3, t.Pick(theme.White, theme.Brand900)).
					Shadow(0, 4, 6, -1, shadow).Shadow(0, 2, 4, -2, shadow).
					Scale(s.scale, s.scale).Rotate(s.rotate).Translate(s.x, s.y)
				if len(games) == 0 {
					// 空收藏夹：bg-brand-150 dark:bg-brand-750，最前面一张显示 text-5xl 的图标
					card.Background(t.Pick(theme.Brand150, theme.Brand750)).Center()
					if s.front {
						card.Children(func() {
							icon, color := "folder-open-outline", t.Pick(theme.Brand400, theme.Brand500)
							if p.Category.IsSystem {
								icon, color = "heart-outline", theme.Error400
							}
							components.Icon(c, icon, 48, color)
						})
					}
					return
				}
				card.Background(t.Pick(theme.Brand200, theme.Brand700))
				g := games[i]
				game := &models.Game{ID: g.ID, CoverURL: g.CoverURL, CoverSourceURL: g.CoverSourceURL, IsNSFW: g.IsNSFW}
				var bmp *ui.Bitmap
				if p.Covers != nil {
					bmp = p.Covers(game)
				}
				if bmp != nil {
					// overflow-hidden 不用 Clip：旋转的元素裁剪时 MyGo 要在 CPU 上为每张封面光栅化一张裁剪蒙版并上传，
					// 侧边栏展开、收起时卡片每帧变宽，三张封面的蒙版每帧重建（每秒几百 MB 的纹理）。封面本身按边框内侧的
					// 圆角（8 - 3）画出，效果相同；只有要模糊的 NSFW 封面需要裁掉模糊溢出的部分，仍然裁剪。
					blur := g.IsNSFW && p.BlurNSFW
					if blur {
						card.Clip()
					}
					card.Children(func() {
						img := ui.Image(c, bmp).Fill().Fit(ui.Cover).Radius(5)
						if blur {
							components.CoverBlur(c, card, img, false)
						}
					})
				}
			})
		})
	}
}

// countBadge：absolute bottom-2 right-2 rounded-full bg-white/85 px-2 py-0.5 text-xs font-medium tabular-nums shadow-sm
// （dark：bg-black/55 text-brand-100），i-mdi-layers-triple-outline size-3。
func countBadge(c *ui.Context, t *theme.Theme, count int) {
	color := t.Pick(theme.Brand900, theme.Brand100)
	ui.Row(c.Key("count")).Absolute().Right(8).Bottom(8).Padding(2, 8).Gap(4).Radius(999).AlignItems(ui.Center).PassThrough().
		Background(t.Pick(theme.White.Alpha(0.85), theme.Black.Alpha(0.55))).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).
		Children(func() {
			components.Icon(c, "layers-triple-outline", 12, color)
			components.Text(c, strconv.Itoa(count), theme.TextXS, theme.WeightMedium, color).NoWrap()
		})
}

// selectButton：absolute right-3 top-3 h-7 w-7 rounded-full border shadow-md，hover:scale-105。
// 系统收藏夹显示锁；否则是勾选圆圈，选中时 bg-primary-500。
func selectButton(c *ui.Context, t *theme.Theme, p CategoryCardProps, name string) {
	btn := ui.ButtonBase(c.Key("select")).Absolute().Top(12).Right(12).Size(28, 28).Radius(999).Center().FocusRing(false).Label(name).
		Border(t.HL, theme.Brand200).Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
	scale := float32(1)
	if btn.Hovered() {
		scale = 1.05
	}
	s := btn.AnimateWith("scale", scale, 150*time.Millisecond, theme.EaseTailwind)
	btn.Scale(s, s)
	if p.SelectionDisabled {
		btn.Cursor(ui.CursorNotAllowed) // disabled：只换光标，锁不变淡
		btn.Children(func() { components.Icon(c, "lock", 16, t.Pick(theme.Brand400, theme.Brand500)) })
		return
	}
	btn.OnClick(func() {
		if p.OnSelectChange != nil {
			p.OnSelectChange(!p.Selected)
		}
	})
	btn.Children(func() {
		inner := ui.Box(c).Absolute().Top(-t.HL).Left(-t.HL).Right(-t.HL).Bottom(-t.HL).Radius(999).Center()
		color := ui.Transparent
		if p.Selected {
			inner.Background(theme.Primary500).Border(t.HL, t.Pick(theme.Primary500, theme.Primary400))
			color = theme.White
		} else {
			inner.Background(t.Pick(theme.White.Alpha(0.9), theme.Brand800.Alpha(0.9))).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
		}
		inner.Children(func() { components.Icon(c, "check", 16, color) })
	})
}
