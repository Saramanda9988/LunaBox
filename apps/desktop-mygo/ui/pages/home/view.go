package home

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel/homerail"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

const (
	edge          = 32  // inset-x-8、top-8、left-8、right-8、bottom-8
	pickerBottom  = 288 // 封面选择面板展开时内容的 bottom-[18rem]
	heroMaxW      = 512 // max-w-lg
	heroCoverMaxH = 288 // max-h-72
)

// shadowLG 是 shadow-lg。
func shadowLG(e ui.Element) ui.Element {
	return e.Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
}

// coverURL 是 cover_url || cover_source_url。
func coverURL(g *models.Game) string {
	if g.CoverURL != "" {
		return g.CoverURL
	}
	return g.CoverSourceURL
}

// View 构建首页。加载中不显示内容；没有首页数据时是重试；没有游玩记录时引导去游戏库。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	for _, n := range p.toasts {
		p.env.Toast(c, n())
	}
	p.toasts = nil

	switch {
	case p.loading:
		ui.Box(c.Key("home-loading")).Fill()
		return
	case p.data == nil:
		p.noData(c, t)
		return
	}
	now := c.Now()
	items := p.items()
	if p.data.LastPlayed == nil || p.selected(items) == nil {
		p.hero.update(nil, now)
		p.noRecord(c, t)
		return
	}
	// 先推进轮播，再按（可能刚换的）选中游戏更新封面大图与背景的过渡。
	next := p.tick(items, now)
	sel := p.selected(items)
	cfg := p.env.Config()
	g := sel.Game
	running, _ := p.launcher.Status(g.ID)
	playing := sel.IsPlaying || running
	url := coverURL(&g)
	for _, d := range []time.Duration{
		next,
		p.hero.update(&heroSnap{game: g, playing: playing, lastPlayed: sel.LastPlayedAt, total: sel.TotalPlayedDur}, now),
		p.bg.update(g, url, now),
	} {
		if d > 0 {
			c.After(d)
		}
	}
	// 主色从大图算出（useImageAccentRgb，Large 顺带排队加载大图）；没有封面时回到默认色。
	p.env.Covers.Large(&g)
	if url == "" {
		p.accent = covers.DefaultAccent
	} else if col, ok := p.env.Covers.Accent(&g); ok {
		p.accent = col
	}

	bgCustom := cfg != nil && cfg.BackgroundEnabled
	showBackground := !bgCustom || !cfg.BackgroundHideGameCover
	showHeroCover := !bgCustom || !cfg.BackgroundHideGameHeroCover
	hasPicker := len(items) > 1
	showPicker := hasPicker && p.expanded
	blurNSFW := cfg == nil || cfg.BlurNSFWGameCovers

	// h-full flex flex-col，里面 flex-1 relative overflow-hidden 的舞台
	ui.Column(c.Key("home")).Fill().Children(func() {
		stage := ui.Box(c.Key("stage")).Grow(1).Basis(0).Clip()
		bottom := float32(edge)
		if showPicker {
			bottom = pickerBottom
		}
		// transition-[bottom] duration-300 ease-out
		bottom = stage.AnimateWith("content-bottom", bottom, 300*time.Millisecond, theme.EaseOut)
		stage.Children(func() {
			if showBackground {
				p.background(c, t, g, url)
			}
			header(c, t, locales.HomeWelcomeBack())
			p.heroBlock(c, t, bottom, showPicker, showHeroCover, blurNSFW)
			ui.Row(c.Key("cta")).Absolute().Right(edge).Bottom(bottom).Children(func() {
				if playing {
					gamingBadge(c)
				} else {
					btn := better.Button(c, t, "continue", better.ButtonProps{
						Variant: better.ButtonPrimary, Size: better.ButtonLG, Icon: "play", Label: locales.HomeContinueGame(),
					})
					// !h-auto gap-2 rounded-xl px-6 py-3 shadow-lg
					shadowLG(btn.Radius(12).Padding(0, 24).Gap(8)).OnClick(func() { p.continueGame(g) })
				}
			})
			if hasPicker {
				games := make([]models.Game, len(items))
				for i, it := range items {
					games[i] = it.Game
				}
				cc := p.env.Covers
				p.rail.View(c, t, homerail.Props{
					Games:            games,
					Expanded:         p.expanded,
					LibraryStats:     p.libraryStats,
					SelectedID:       g.ID,
					Covers:           func(g *models.Game) (*ui.Bitmap, bool) { return cc.Get(g), cc.Missing(g) },
					OnExpandedChange: func(v bool) { p.expanded = v },
					OnSelect:         func(id string) { p.activeID = id },
				})
			}
		})
	})
}

// header：absolute inset-x-8 top-8 z-10，h1 text-4xl font-bold text-brand-900 dark:text-white，
// 下面 mt-2 text-brand-500 dark:text-brand-400 的欢迎语（外层 mx-auto max-w-8xl 在 UnoCSS 里没有定义，不限宽）。
func header(c *ui.Context, t *theme.Theme, welcome string) {
	ui.Column(c.Key("header")).Absolute().Left(edge).Right(edge).Top(edge).Children(func() {
		components.Text(c, locales.HomeTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
		components.TextMargin(c, welcome, theme.TextBase, theme.WeightNormal, t.TextMuted, 8, 0, 0, 0)
	})
}

// background 是选中游戏的封面背景：absolute inset-0 overflow-hidden，封面 object-cover 并模糊 16px（四周外扩 3 倍
// 模糊半径，免得边缘透出），换游戏时旧封面在 1200ms 内淡出（ease-in-out）；上面依次是 bg-white/50 dark:bg-black/60、
// 封面主色（opacity-[0.08] dark:opacity-[0.12]，颜色 1400ms 过渡）与 bg-gradient-to-t from-black/10 via-transparent
// dark:from-black/40 的底部渐变。原版的 NSFW 模糊被内联的 blur(16px) 覆盖，这里同样只模糊 16px。
func (p *Page) background(c *ui.Context, t *theme.Theme, g models.Game, url string) {
	full := func(e ui.Element) ui.Element { return e.Absolute().Top(0).Left(0).Right(0).Bottom(0) }
	full(ui.Box(c.Key("bg"))).Clip().PassThrough().Children(func() {
		const spread = 3 * backgroundBlur
		ui.Box(c.Key("bg-images")).Absolute().Top(-spread).Left(-spread).Right(-spread).Bottom(-spread).Children(func() {
			if bmp := p.env.Covers.Get(&g); bmp != nil {
				full(ui.Image(c.Key("bg:"+url), bmp)).Fit(ui.Cover).Blur(backgroundBlur)
			}
			if f := &p.bg; f.prevURL != "" && f.prevURL != url {
				if bmp := p.env.Covers.Get(&f.prev); bmp != nil {
					img := full(ui.Image(c.Key("bg-prev:"+f.prevURL), bmp)).Fit(ui.Cover)
					// isBackgroundCrossfading：换图的那一帧不透明，下一帧起淡出。
					k := img.AnimateTo("fade", theme.Bool(!c.Now().After(f.since)), ui.Animation{Duration: backgroundCrossfade, Ease: theme.EaseTailwind})
					img.Opacity(k).Blur(backgroundBlur)
				}
			}
		})
		full(ui.Box(c.Key("bg-veil"))).Background(t.Pick(theme.White.Alpha(0.5), theme.Black.Alpha(0.6)))
		accentOpacity := float32(0.08)
		if t.Dark {
			accentOpacity = 0.12
		}
		accent := full(ui.Box(c.Key("bg-accent")))
		accent.Background(accent.AnimateColor("accent", p.accent, ui.Animation{Duration: 1400 * time.Millisecond, Ease: theme.EaseTailwind})).
			Opacity(accentOpacity)
		full(ui.Box(c.Key("bg-shade"))).LinearGradient(ui.LinearGradient{
			From: theme.Black.Alpha(0), To: t.Pick(theme.Black.Alpha(0.1), theme.Black.Alpha(0.4)), Angle: 180, Start: 0.5, End: 1, Oklab: true,
		})
	})
}

// heroBlock：absolute bottom-8 left-8 max-w-lg z-10（面板展开时 bottom-[18rem]），里面是 renderHeroContent：
// 封面大图（GameCoverImage）与标题、上次游玩时间、总时长。换游戏时旧内容先淡出再换上新内容（useSnapshotVisibilityTransition），
// 封面 duration-[760ms] delay-75 / duration-[220ms] 地淡入淡出、上移 20px、缩放到 0.99，文字 duration-[680ms] delay-150 /
// duration-[180ms] 地淡入淡出、上移 16px、模糊 2px（都是 ease-out）。
func (p *Page) heroBlock(c *ui.Context, t *theme.Theme, bottom float32, showPicker, showCover, blurNSFW bool) {
	winW, _ := c.Size()
	maxW := min(heroMaxW, p.env.MainWidth-edge)
	hero := ui.Column(c.Key("hero")).Absolute().Left(edge).Bottom(bottom).MaxWidth(maxW).AlignItems(ui.Start)
	s := p.hero.shown
	if s == nil {
		return
	}
	in := func(d, delay time.Duration) ui.Animation {
		return ui.Animation{Duration: d, Ease: theme.EaseOut, Delay: delay}
	}
	coverA, metaA := in(220*time.Millisecond, 0), in(180*time.Millisecond, 0)
	if p.hero.visible {
		coverA, metaA = in(760*time.Millisecond, 75*time.Millisecond), in(680*time.Millisecond, 150*time.Millisecond)
	}
	kc := hero.AnimateTo("cover", theme.Bool(p.hero.visible), coverA)
	km := hero.AnimateTo("meta", theme.Bool(p.hero.visible), metaA)
	// 封面在面板展开时收起：max-h-72 → max-h-0、mb-4 → mb-0、-translate-y-2、opacity-0（transition-all duration-300 ease-out）。
	kp := hero.AnimateWith("picker", theme.Bool(!showPicker), 300*time.Millisecond, theme.EaseOut)
	hero.Children(func() {
		g := &s.game
		if showCover && coverURL(g) != "" {
			p.heroCover(c, t, g, coverBreakpoint(winW, maxW), kc, kp, blurNSFW)
		}
		meta := ui.Column(c.Key("hero-meta")).Opacity(km).Translate(0, (1-km)*16)
		if r := (1 - km) * 2; r > 0.05 {
			meta.Blur(r)
		}
		meta.Children(func() { p.heroMeta(c, t, s) })
	})
}

// coverBreakpoint 是封面大图的最大宽度：max-w-full sm:max-w-sm md:max-w-md lg:max-w-lg（断点按窗口宽度）。
func coverBreakpoint(winW, containerW float32) float32 {
	w := containerW
	switch {
	case winW >= 1024:
		w = min(w, 512)
	case winW >= 768:
		w = min(w, 448)
	case winW >= 640:
		w = min(w, 384)
	}
	return w
}

// heroCover 是 GameCoverImage：w-fit max-w-full rounded-2xl bg-brand-200 dark:bg-brand-800，
// shadow-[0_8px_30px_rgb(0,0,0,0.3)] ring-2 ring-white/20 dark:ring-white/10，图片 block max-h-72 object-contain，
// 点击进入详情；NSFW 封面模糊，悬停时变清晰（revealNSFWOnHover）。kc 是淡入进度，kp 是面板收起时的展开进度。
func (p *Page) heroCover(c *ui.Context, t *theme.Theme, g *models.Game, maxW, kc, kp float32, blurNSFW bool) {
	// 大图按图片的像素尺寸缩小到 max-w、max-h-72 之内（不放大）；还没加载完时先用卡片封面，只取比例。
	bmp, natural := p.env.Covers.Large(g), true
	if bmp == nil {
		bmp, natural = p.env.Covers.Get(g), false
	}
	var w, h float32
	if bmp != nil {
		bw, bh := bmp.Size()
		if bw > 0 && bh > 0 {
			s := min(maxW/float32(bw), heroCoverMaxH/float32(bh))
			if natural {
				s = min(s, 1)
			}
			w, h = float32(bw)*s, float32(bh)*s
		}
	}
	// -translate-y-2 用上下外边距代替 Translate（占位不变）：变换下的裁剪每帧都要在 CPU 上重新光栅化蒙版，收起时会掉帧。
	lift := 8 * (1 - kp)
	wrap := ui.Box(c.Key("hero-cover")).Width(w).Height(min(h, heroCoverMaxH*kp)).Margin(-lift, 0, 16*kp+lift, 0).
		Opacity(kp).Shrink(0)
	if kp < 1 {
		wrap.Clip()
	}
	if bmp == nil || w == 0 {
		return
	}
	wrap.Children(func() {
		// Shrink(0)：收起时封面保持原尺寸，只被外层的 max-h 从底部裁掉；否则会跟着压扁，图片 Contain 缩小、两侧露出底色。
		box := ui.Box(c.Key("cover")).Size(w, h).Shrink(0).Radius(16).Background(t.Pick(theme.Brand200, theme.Brand800)).
			Shadow(0, 8, 30, 0, ui.RGBA(0, 0, 0, 0.3)).Shadow(0, 0, 0, 2, t.Pick(theme.White.Alpha(0.2), theme.White.Alpha(0.1))).
			Opacity(kc).Translate(0, (1-kc)*20).Scale(0.99+0.01*kc, 0.99+0.01*kc).Cursor(ui.CursorPointer).Label(g.Name)
		// overflow-hidden 不用 Clip：换游戏时封面要淡入、上移、缩放 760ms，变换下的圆角裁剪每帧都要在 CPU 上光栅化
		// 一张封面大小的蒙版再上传（150% 缩放下约 1.3MB 一帧）。图片的比例与 box 相同，直接按圆角画出即可；
		// 只有要模糊的 NSFW 封面需要裁掉模糊溢出的部分，仍然裁剪。
		blur := g.IsNSFW && blurNSFW
		if blur {
			box.Clip()
		}
		id := g.ID
		box.OnClick(func() { p.openDetail(id) }).Children(func() {
			img := ui.Image(c, bmp).Fill().Fit(ui.Contain).Radius(16)
			if blur {
				components.CoverBlur(c, box, img, true)
			}
		})
	})
}

// heroMeta：h1 text-4xl font-bold text-brand-900 dark:text-white mb-2（悬停 text-neutral-600 dark:text-neutral-300，点击进入详情）；
// 正在游玩时是 min-h-[2.75rem] 的“正在游玩”（text-base font-medium text-brand-700 dark:text-white/85），否则是 min-h-[2.75rem] gap-1
// 的两行 text-sm：上次游玩时间（text-brand-700 dark:text-white/80）与总时长（text-brand-600 dark:text-white/70，没有时透明占位）。
// 原版文字的 drop-shadow 没有还原。
func (p *Page) heroMeta(c *ui.Context, t *theme.Theme, s *heroSnap) {
	title := ui.ButtonBase(c.Key("hero-title")).Padding(0).Justify(ui.Start).AlignItems(ui.Start).Margin(0, 0, 8, 0).FocusRing(false).Cursor(ui.CursorPointer).Label(s.game.Name)
	color := t.TextStrong
	if title.Hovered() {
		color = t.Pick(theme.Neutral600, theme.Neutral300)
	}
	color = title.AnimateColor("color", color, theme.Tween(150*time.Millisecond))
	id := s.game.ID
	title.OnClick(func() { p.openDetail(id) }).Children(func() {
		components.Text(c, s.game.Name, theme.Text4XL, theme.WeightBold, color)
	})
	if s.playing {
		ui.Row(c.Key("playing")).MinHeight(44).AlignItems(ui.Center).Children(func() {
			components.Text(c, locales.HomePlayingNow(), theme.TextBase, theme.WeightMedium, t.Pick(theme.Brand700, theme.White.Alpha(0.85)))
		})
		return
	}
	ui.Column(c.Key("played")).MinHeight(44).Gap(4).Children(func() {
		when := timefmt.DateTime(s.lastPlayed, timefmt.Location(p.env.Config()))
		components.Text(c, locales.HomeLastPlayed(when), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand700, theme.White.Alpha(0.8)))
		total, opacity := " ", float32(0)
		if s.total > 0 {
			total, opacity = locales.HomeTotalPlayTime()+timefmt.Duration(s.total), 1
		}
		components.Text(c, total, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.White.Alpha(0.7))).Opacity(opacity)
	})
}

// gamingBadge 是正在游玩时代替“继续游戏”的标签：inline-flex gap-2 rounded-xl bg-success-600 px-6 py-3 text-sm font-medium
// text-white shadow-lg，i-mdi-gamepad-variant text-xl animate-pulse。
func gamingBadge(c *ui.Context) {
	badge := shadowLG(ui.Row(c.Key("gaming")).Gap(8).AlignItems(ui.Center).Padding(12, 24).Radius(12).Background(theme.Success600))
	badge.Children(func() {
		ic := components.Icon(c, "gamepad-variant", 20, theme.White)
		ic.Opacity(components.Pulse(ic))
		components.Text(c, locales.HomeGaming(), theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
}

// noData 是没有首页数据（加载失败）时的页面：居中的 text-brand-500 提示与 px-4 py-2 bg-neutral-500 text-white rounded
// hover:bg-neutral-600 的重试按钮（space-y-4）。
func (p *Page) noData(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("home-empty")).Fill().Center().Gap(16).Children(func() {
		components.Text(c, locales.HomeNoData(), theme.TextBase, theme.WeightNormal, theme.Brand500)
		btn := ui.ButtonBase(c.Key("retry")).Padding(8, 16).Radius(4).FocusRing(false).Label(locales.HomeRetry())
		bg := theme.Neutral500
		if btn.Hovered() {
			bg = theme.Neutral600
		}
		btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).
			OnClick(func() { p.fetch(true, true) }).Children(func() {
			components.Text(c, locales.HomeRetry(), theme.TextBase, theme.WeightNormal, theme.White).NoWrap()
		})
	})
}

// noRecord 是还没有游玩记录时的页面：左上角的标题与欢迎语，中间 i-mdi-gamepad-variant-outline text-8xl
// （text-brand-300 dark:text-brand-700，mb-4）、text-2xl font-bold 的“暂无游玩记录”（mb-2）、提示（mb-6）与
// 去游戏库的按钮（flex gap-2 px-6 py-3 bg-neutral-600 hover:bg-neutral-700 text-white rounded-xl shadow-lg
// hover:scale-105 font-medium）。
func (p *Page) noRecord(c *ui.Context, t *theme.Theme) {
	ui.Box(c.Key("home-none")).Fill().Children(func() {
		header(c, t, locales.HomeWelcome())
		ui.Column(c).Fill().Center().Children(func() {
			components.Icon(c, "gamepad-variant-outline", 96, t.Pick(theme.Brand300, theme.Brand700)).Margin(0, 0, 16, 0)
			components.TextMargin(c, locales.HomeNoPlayRecord(), theme.Text2XL, theme.WeightBold, t.Pick(theme.Brand700, theme.Brand300), 0, 0, 8, 0)
			components.TextMargin(c, locales.HomeNoPlayRecordHint(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.White.Alpha(0.7)), 0, 0, 24, 0)
			btn := ui.ButtonBase(c.Key("browse")).Row().Gap(8).AlignItems(ui.Center).Padding(12, 24).Radius(12).FocusRing(false).
				Label(locales.HomeBrowseLibrary())
			hovered := btn.Hovered()
			// glass-btn-neutral
			g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: theme.Neutral600, Hover: theme.Neutral700, Text: theme.White})
			bg := g.Bg
			if hovered {
				bg = g.Hover
			}
			if t.Glass {
				btn.Border(t.HL, g.Border)
			}
			sc := btn.AnimateWith("scale", 1+0.05*theme.Bool(hovered), 150*time.Millisecond, theme.EaseTailwind)
			shadowLG(btn.Background(bg).Scale(sc, sc).Transition(theme.ColorsTransition(150 * time.Millisecond))).
				OnClick(func() { p.env.Router.Push(env.PathLibrary) }).Children(func() {
				components.Icon(c, "gamepad-variant", 20, g.Text)
				components.Text(c, locales.HomeBrowseLibrary(), theme.TextBase, theme.WeightMedium, g.Text).NoWrap()
			})
		})
	})
}
