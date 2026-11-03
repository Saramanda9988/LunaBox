package components

import (
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// 对应 AppToaster.tsx 的常量。原版最多同时显示 4 条，MyGo 只交给视图最新的 3 条。
const (
	toastWidth    = 380
	toastGap      = 12 // TOAST_GAP：展开后卡片之间的间距
	toastPeek     = 10 // TOAST_PEEK：收起时后面的卡片露出的高度
	toastEdge     = 16
	toastLife     = 4 * time.Second // DEFAULT_TOAST_DURATION
	toastMinH     = 70              // 只有标题的卡片：p-4 上下 + h-9 图标 + 边框，新卡片量到高度之前用它算入场位移
	toastStackDur = 500 * time.Millisecond
)

// 原版 ToastGlyph 的内联 SVG（viewBox 20）。
var (
	toastSuccessGlyph = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path fill="currentColor" d="M16.704 5.29a1 1 0 0 1 .006 1.414l-7.2 7.262a1 1 0 0 1-1.42 0L3.29 9.127a1 1 0 0 1 1.42-1.406l4.09 4.127 6.49-6.545a1 1 0 0 1 1.414-.013Z"/></svg>`))
	toastErrorGlyph   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path fill="currentColor" d="M10 2.5A7.5 7.5 0 1 0 10 17.5 7.5 7.5 0 0 0 10 2.5Zm2.85 9.15a.9.9 0 1 1-1.27 1.27L10 11.27 8.42 12.92a.9.9 0 1 1-1.27-1.27L8.73 10 7.08 8.42a.9.9 0 1 1 1.27-1.27L10 8.73l1.58-1.65a.9.9 0 1 1 1.27 1.27L11.27 10l1.58 1.65Z"/></svg>`))
	toastInfoGlyph    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path fill="currentColor" d="M10 3a7 7 0 1 0 0 14 7 7 0 0 0 0-14Zm.75 9.5a.75.75 0 0 1-1.5 0v-3a.75.75 0 0 1 1.5 0v3Zm0-5.5a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z"/></svg>`))
)

// toastTone 对应 getToastTone：图标底色、图标色与倒计时圆环的颜色。
type toastTone struct{ wrap, icon, ring ui.Color }

func getToastTone(t *theme.Theme, typ string) toastTone {
	switch typ {
	case "success":
		return toastTone{
			wrap: t.Pick(theme.Success500.Alpha(0.15), theme.Success500.Alpha(0.2)),
			icon: t.Pick(theme.Success600, theme.Success400),
			ring: t.Pick(theme.Success500, theme.Success400),
		}
	case "error":
		return toastTone{
			wrap: t.Pick(theme.Error500.Alpha(0.15), theme.Error500.Alpha(0.2)),
			icon: t.Pick(theme.Error600, theme.Error400),
			ring: t.Pick(theme.Error500, theme.Error400),
		}
	case "loading":
		return toastTone{
			wrap: t.Pick(theme.Info500.Alpha(0.15), theme.Info500.Alpha(0.2)),
			icon: t.Pick(theme.Info600, theme.Info400),
			ring: t.Pick(theme.Info500, theme.Info400),
		}
	}
	return toastTone{
		wrap: t.Pick(theme.Primary500.Alpha(0.15), theme.Primary500.Alpha(0.2)),
		icon: t.Pick(theme.Primary600, theme.Primary400),
		ring: t.Pick(theme.Primary500, theme.Primary400),
	}
}

// splitToastMessage 对应原版同名函数：多行消息第一行作标题；“标题：详情”形式拆开；过长的截断标题、完整内容放进详情。
func splitToastMessage(msg string) (title, details string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "", ""
	}
	var lines []string
	for _, l := range strings.Split(msg, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > 1 {
		return lines[0], strings.Join(lines[1:], "\n")
	}
	// /^(.{4,90}?)[：:](.+)$/：标题 4～90 个字符，取第一个冒号。
	rs := []rune(msg)
	for i := 4; i <= 90 && i < len(rs)-1; i++ {
		if rs[i] != ':' && rs[i] != '：' { // i18n:keep：原版正则里的全角冒号
			continue
		}
		title, details := strings.TrimSpace(string(rs[:i])), strings.TrimSpace(string(rs[i+1:]))
		if title != "" && len([]rune(details)) > 4 {
			return title, details
		}
		break
	}
	if len(rs) > 110 {
		return strings.TrimRightFunc(string(rs[:92]), func(r rune) bool { return r == ' ' || r == '\t' }) + "...", msg
	}
	return msg, ""
}

// toastDuration 是提示显示的总时长，0 表示一直显示（与 ui.Toast.Timeout 的约定相同）。
func toastDuration(ts ui.Toast) time.Duration {
	switch {
	case ts.Timeout < 0:
		return 0
	case ts.Timeout == 0:
		return toastLife
	}
	return ts.Timeout
}

// Toaster 对应 components/ui/AppToaster.tsx，代替 MyGo 自带的提示样式：右上角（顶栏下方 top）层叠的卡片，
// 最新的在最前，后面的缩小并露出 10px；指针停留或焦点在其中时展开成列表、暂停计时。
// 卡片左侧是按类型（ui.Toast.Type：success、error、loading，其余为普通）着色的图标与倒计时圆环，
// 长消息拆成标题与可展开的详情，右上角是关闭按钮。每帧在 View 里调用一次。
func Toaster(c *ui.Context, t *theme.Theme, top float32) {
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		winW, _ := c.Size()
		viewport.Justify(ui.Start).AlignItems(ui.End).Padding(top, toastEdge, 0, toastEdge)
		stack := ui.Box(c.Key("app-toasts")).Width(min(toastWidth, winW-2*toastEdge))
		heights := ui.Local(stack, "heights", func() map[string]float32 { return map[string]float32{} })
		expanded := stack.Hovered() || stack.FocusWithin()

		// 最新的在 toasts 末尾，原版 index 0 是最新的一条。
		n := len(toasts)
		height := func(i int) float32 { return (*heights)[toasts[n-1-i].ID] }
		var front, total float32
		offsets := make([]float32, n)
		for i := range n {
			offsets[i] = total
			total += height(i) + toastGap
		}
		if n > 0 {
			front, total = height(0), total-toastGap
		}
		if expanded {
			stack.Height(max(0, total))
		} else {
			stack.Height(max(0, front+float32(n-1)*toastPeek))
		}

		seen := make(map[string]bool, n)
		stack.Children(func() {
			// 先建旧的，最新的画在最上面（原版的 zIndex）。
			for k, ts := range toasts {
				seen[ts.ID] = true
				idx := n - 1 - k
				h := toastStackItem(c, t, ts, idx, height(idx), offsets[idx], front, expanded)
				switch {
				case h <= 0:
					c.AnimationFrame() // 下一帧量到高度后再排
				case h != (*heights)[ts.ID]:
					// 高度变了（新卡片、展开详情）：量到的是上一帧的布局，再要一帧按新高度排，不等下一次输入。
					(*heights)[ts.ID] = h
					c.AnimationFrame()
				}
			}
		})
		for id := range *heights {
			if !seen[id] {
				delete(*heights, id)
			}
		}
	})
}

// toastStackItem 对应 ToastStackItem：按层叠位置摆放一条提示（own 是它上一帧的高度），返回这次量到的高度（含边框）。
func toastStackItem(c *ui.Context, t *theme.Theme, ts ui.Toast, idx int, own, offset, front float32, expanded bool) float32 {
	parts := ui.ToastBase(c, ts)
	item := parts.Root
	behind := idx > 0 && !expanded

	y, scale := float32(idx*toastPeek), 1-float32(idx)*0.045
	if expanded {
		y, scale = offset, 1
	}
	s := item.AnimateWith("scale", scale, toastStackDur, theme.EaseDrawer)
	item.Absolute().Left(0).Right(0).Top(y).Column().Clip().
		Radius(16).Border(t.HL, t.Pick(theme.Brand200.Alpha(0.9), theme.Brand700.Alpha(0.7))).
		Background(t.Pick(theme.White, theme.Brand800)).TextColor(t.Pick(theme.Brand900, theme.Brand50)).
		Scale(s, s).TransformOrigin(0.5, 0).
		Transition(ui.ElementTransition{
			Position: true, Size: true, Duration: toastStackDur, Ease: theme.EaseDrawer,
			// animate-app-toast-enter / leave：从视口边缘外滑入，向上 16px 缩小淡出。
			Enter: &ui.Motion{Y: -(toastMinH + 24), Scale: 0.96},
			Exit:  &ui.Motion{Y: -16, Scale: 0.96},
		})
	// shadow-lg shadow-brand-900/8，暗色 shadow-black/25。
	shadow := t.Pick(theme.Brand900.Alpha(0.08), ui.RGBA(0, 0, 0, 0.25))
	item.Shadow(0, 10, 15, -3, shadow).Shadow(0, 4, 6, -4, shadow)
	if t.Glass {
		// app-toast-stack-surface：data-glass:bg-white/85 data-glass:dark:bg-brand-800/85 data-glass:backdrop-blur-xl。
		// 模糊的背景用 Material，半透明底色画在它上面、内容之下。
		bg := t.Pick(theme.White.Alpha(0.85), theme.Brand800.Alpha(0.85))
		item.Material(glass.Blur{Radius: 24}).Draw(func(p *ui.Painter, r ui.Rect) { p.Fill(r, bg, 16) })
	}

	// 收起时每张都与最前面的一样高，展开时用自己上一帧量到的高度。
	switch {
	case !expanded && front > 0:
		item.Height(front)
	case own > 0:
		item.Height(own)
	}
	var outer float32
	item.Children(func() {
		// 原版量 contentRef.offsetHeight + 2（上下边框）。
		content := ui.Column(c.Key("content")).Shrink(0)
		if m := content.LayoutBounds().H; m > 0 {
			outer = m + 2*t.HL
		}
		content.Opacity(content.AnimateWith("opacity", 1-theme.Bool(behind), 200*time.Millisecond, theme.EaseTailwind))
		content.Children(func() { toastCard(c, t, ts, parts, expanded) })
	})
	return outer
}

// toastCard 对应 ToastCard：图标与倒计时圆环、标题、可展开的详情和关闭按钮。
func toastCard(c *ui.Context, t *theme.Theme, ts ui.Toast, parts ui.ToastParts, paused bool) {
	tone := getToastTone(t, ts.Type)
	title, details := splitToastMessage(ts.Title)
	if ts.Description != "" {
		title, details = ts.Title, ts.Description
	}

	// flex items-start gap-3.5 p-4 pr-11
	ui.Row(c).AlignItems(ui.Start).Gap(14).Padding(16, 44, 16, 16).Children(func() {
		wrap := ui.Box(c).Size(36, 36).Radius(999).Center().Background(tone.wrap).Shrink(0)
		wrap.Children(func() {
			if life := toastDuration(ts); life > 0 && ts.Type != "loading" {
				toastRing(c, tone.ring, life, parts.Left(), c.Now(), paused)
			}
			switch ts.Type {
			case "success":
				ui.Icon(c, toastSuccessGlyph).Size(18, 18).TextColor(tone.icon)
			case "error":
				ui.Icon(c, toastErrorGlyph).Size(18, 18).TextColor(tone.icon)
			case "loading":
				Spinner(c, 18, tone.icon)
			default:
				ui.Icon(c, toastInfoGlyph).Size(18, 18).TextColor(tone.icon)
			}
		})

		ui.Column(c).Grow(1).Basis(0).MinHeight(36).Justify(ui.Center).Children(func() {
			// text-[14px] leading-snug font-medium
			Text(c, title, theme.FontStep{Size: 14, Line: 19.25}, theme.WeightMedium, t.Pick(theme.Brand900, theme.Brand50))
			if details != "" {
				toastDetails(c, t, details)
			}
			if ts.Action != "" {
				toastAction(c, t, ts.Action, parts)
			}
		})
	})

	// absolute right-3 top-3 h-7 w-7 rounded-full
	btn := parts.CloseButton().Absolute().Right(12).Top(12).Size(28, 28).Radius(999).Center().FocusRing(false).
		Label(locales.CommonToastDismiss())
	color := t.Pick(theme.Brand400, theme.Brand500)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand500.Alpha(0.1), theme.Brand400.Alpha(0.1)))
		color = t.Pick(theme.Brand700, theme.Brand200)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		Icon(c, "close", 16, color)
	})
}

// toastAction 对应 useAppRuntimeEffects 里 renderProtocolLaunchConfigToast 的按钮：mt-3 inline-flex items-center gap-1
// rounded-md bg-neutral-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-700，i-mdi-cog-play-outline text-sm。
// 原版那条提示整张卡片是自己画的；这里沿用 AppToaster 的卡片，只把按钮放在标题下面。
func toastAction(c *ui.Context, t *theme.Theme, label string, parts ui.ToastParts) {
	btn := parts.ActionButton().Row().AlignItems(ui.Center).AlignSelf(ui.Start).Gap(4).Margin(12, 0, 0, 0).Padding(6, 12).
		Radius(6).FocusRing(false)
	bg := theme.Neutral600
	if btn.Hovered() {
		bg = theme.Neutral700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		Icon(c, "cog-play-outline", 14, theme.White)
		Text(c, label, theme.TextXS, theme.WeightMedium, theme.White).NoWrap()
	})
}

// toastDetails 是“显示详情 / 隐藏详情”按钮与展开后的等宽文字框（max-h-40 overflow-auto）。
func toastDetails(c *ui.Context, t *theme.Theme, details string) {
	box := ui.Column(c.Key("details")).Margin(10, 0, 0, 0).AlignItems(ui.Start)
	open := ui.Local(box, "open", func() bool { return false })
	box.Children(func() {
		// inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium text-primary-600
		btn := ui.ButtonBase(c.Key("toggle")).Row().AlignItems(ui.Center).Gap(4).Padding(4, 10).Radius(999).FocusRing(false).
			Expanded(*open)
		if btn.Hovered() {
			btn.Background(t.Pick(theme.Brand500.Alpha(0.1), theme.Brand400.Alpha(0.1)))
		}
		if btn.Clicked() {
			*open = !*open
			c.AnimationFrame() // 下一帧量详情展开后的高度
		}
		color := t.Pick(theme.Primary600, theme.Primary400)
		icon, label := "chevron-down", locales.CommonToastShowDetails()
		if *open {
			icon, label = "chevron-up", locales.CommonToastHideDetails()
		}
		btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
			Icon(c, icon, 14, color)
			Text(c, label, theme.FontStep{Size: 11, Line: 16.5}, theme.WeightMedium, color).NoWrap()
		})
		if !*open {
			return
		}
		// mt-2 max-h-40 overflow-auto rounded-lg border border-brand-500/10 bg-brand-500/5 p-3 text-[12px] font-mono
		ui.Scroll(c.Key("text")).Margin(8, 0, 0, 0).MaxHeight(160).AlignSelf(ui.Stretch).Radius(8).Padding(12).
			Border(t.HL, t.Pick(theme.Brand500.Alpha(0.1), theme.Brand400.Alpha(0.1))).
			Background(t.Pick(theme.Brand500.Alpha(0.05), theme.Brand400.Alpha(0.05))).
			Children(func() {
				Text(c, details, theme.FontStep{Size: 12, Line: 19.5}, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand300)).
					Font("Consolas").Selectable()
			})
	})
}

// toastRing 是图标外 -inset-1 的倒计时圆环：从 12 点钟方向顺时针剩下的部分随时间缩短。
// MyGo 只在构建时给出剩余时间，圆环在两次构建之间按绘制时间推算，暂停时不动。
func toastRing(c *ui.Context, color ui.Color, life, left time.Duration, at time.Time, paused bool) {
	ui.Box(c).Absolute().Left(-4).Top(-4).Size(44, 44).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		rem := left
		if !paused {
			rem -= p.Now().Sub(at)
			if rem > 0 {
				p.AnimationFrame()
			}
		}
		frac := min(1, max(0, float32(rem)/float32(life)))
		scale := r.W / 44
		cx, cy, rad, w := r.X+r.W/2, r.Y+r.H/2, 20*scale, 1.75*scale
		var track ui.Path
		p.StrokePath(track.Circle(cx, cy, rad), w, color.Alpha(0.15))
		if frac > 0 {
			var arc ui.Path
			p.StrokePath(topCircle(&arc, cx, cy, rad).Trim(0, frac), w, color)
		}
	})
}

// topCircle 加一个从 12 点钟方向起顺时针的圆（原版 -rotate-90 的 SVG circle），不闭合，以便 Trim。
func topCircle(p *ui.Path, cx, cy, r float32) *ui.Path {
	const k = 0.5522847498
	return p.MoveTo(cx, cy-r).
		CubeTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy).
		CubeTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r).
		CubeTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy).
		CubeTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r)
}
