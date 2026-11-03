package bar

import (
	"log"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// 对应 PlayingIsland.tsx 的常量。
const (
	islandH                = 56 // ISLAND_HEIGHT（h-14）
	islandTop              = env.TopbarHeight + 12
	islandCollapseY        = 14 - islandH - 12 // COLLAPSE_TRANSLATE_Y = COLLAPSED_PEEK_HEIGHT - ISLAND_HEIGHT - EXPANDED_TOP_GAP
	islandHideDistance     = -islandCollapseY  // HIDE_DISTANCE
	islandDragThreshold    = 24                // COLLAPSE_DRAG_THRESHOLD
	islandExpandedW        = 304               // min(19rem, calc(100vw - 9rem))
	islandCollapsedW       = 240               // 15rem
	islandAutoCollapse     = 5 * time.Second   // AUTO_COLLAPSE_AFTER_PLAYING_SECONDS
	islandEnterDur         = 360 * time.Millisecond
	islandExitDur          = 220 * time.Millisecond
	islandSwitchDur        = 260 * time.Millisecond // CONTENT_SWITCH_ANIMATION_MS，也是 content-in 的时长
	islandContentOutDur    = 220 * time.Millisecond
	islandMoveDur          = 420 * time.Millisecond // transition-[width,transform,opacity] duration-[420ms]
	islandFadeDur          = 260 * time.Millisecond // 内容与拖动条 transition-opacity duration-[260ms] ease-out
	islandMarqueePeriod    = 8 * time.Second
	islandMarqueeSpacing   = 32                          // 第二份标题的 px-8
	islandTitleViewportPad = 12 + 36 + 12 + 12 + 36 + 12 // px-3、封面 h-9 w-9、两个 gap-3、结束按钮 h-9 w-9
)

var (
	easeIsland     = theme.CubicBezier(0.2, 0.9, 0.18, 1) // ease-[cubic-bezier(.2,.9,.18,1)]，也是 content-in
	easeIslandIn   = theme.CubicBezier(0.16, 1, 0.3, 1)   // playing-island-enter
	easeIslandOut  = theme.CubicBezier(0.4, 0, 1, 1)      // playing-island-leave
	easeContentOut = theme.CubicBezier(0.4, 0, 0.2, 1)    // playing-island-content-out
)

// IslandRuntime 是灵动岛显示的运行状态（store 的 GameRuntimeInfo）。
type IslandRuntime struct {
	Game      models.Game
	SessionID string
	StartTime time.Time
	State     service.GameRuntimeState
	// ActiveSeconds 是 timingMode === "active" 时的活跃时长；为 nil 时按 StartTime 计墙钟时长（useElapsedSeconds）。
	ActiveSeconds *int
}

// key 对应 getRuntimeKey：gameId:sessionId。
func (r *IslandRuntime) key() string { return r.Game.ID + ":" + r.SessionID }

// PlayingIslandProps 是灵动岛的输入：当前显示的运行状态（gameRuntimes[activeGameRuntimeId]）与动作。
type PlayingIslandProps struct {
	Env     *env.Env
	Runtime *IslandRuntime // 没有可见的运行状态（或它还没有游戏数据）时为 nil
	Count   int            // runtimeCount
	// OnSwitch 对应 selectNextGameRuntime；OnEnd 对应 EndCurrentPlaySession，done 在主线程上收到错误。
	OnSwitch func()
	OnEnd    func(gameID string, done func(err error))
}

// islandSnapshot 对应 RuntimeSnapshot：切换游戏时淡出的旧内容。
type islandSnapshot struct {
	game   models.Game
	status string
}

// PlayingIsland 是灵动岛跨帧的状态（PlayingIsland 与 PlayingIslandBody 的 useState、useRef），由 App 持有。
type PlayingIsland struct {
	render    *IslandRuntime // renderRuntime：退出动画期间保留最后的运行状态
	exitingAt time.Time      // 退出动画开始的时间（isExiting）

	// 以下是 PlayingIslandBody 的状态，岛完全消失后清零（原版组件卸载）。
	mountedAt     time.Time      // 入场动画开始的时间
	collapsed     bool           // isCollapsed
	ending        bool           // isEnding：点了结束、还没收到 ending 事件
	display       *IslandRuntime // displayRuntime
	outgoing      *islandSnapshot
	switchedAt    time.Time // 切换动画开始的时间（isSwitchingRuntime）
	autoCollapsed string    // autoCollapsedRuntimeRef
	playingSince  time.Time // 自动收起计时的起点（setTimeout 的开始）
	playingKey    string
	pressing      bool    // 按住岛（dragRef）
	pressCollapse bool    // 按下时是否收起（startCollapsed）
	pressDY       float32 // 按下后指针移动的距离（clientY - startY）
}

func (s *PlayingIsland) unmount() {
	*s = PlayingIsland{}
}

// View 对应 components/bar/PlayingIsland.tsx：顶栏下方居中的胶囊，显示正在运行的游戏、时长与结束按钮；开始游玩 5 秒后
// 自动收起到顶栏边缘，上下拖动或点击收起的岛切换展开与收起。要在页面之后、Toaster 之前构建（z-45）。
func (s *PlayingIsland) View(c *ui.Context, t *theme.Theme, p PlayingIslandProps) {
	now := c.Now()
	switch {
	case p.Runtime != nil:
		if s.render == nil {
			s.unmount()
		}
		s.render, s.exitingAt = p.Runtime, time.Time{}
	case s.render == nil:
		return
	case s.exitingAt.IsZero():
		s.exitingAt = now
	}
	exiting := !s.exitingAt.IsZero()
	if exiting {
		if d := now.Sub(s.exitingAt); d >= islandExitDur {
			s.unmount()
			return
		}
		c.AnimationFrame()
	}
	if s.mountedAt.IsZero() {
		s.mountedAt = now
	}
	s.body(c, t, p, now, exiting)
}

// statusText 对应 statusText：启动中、结束中或“已游玩 1时5分”。wall-clock 计时在下一分钟到来时重新构建。
func (s *PlayingIsland) statusText(c *ui.Context, r *IslandRuntime, now time.Time) string {
	switch {
	case r.State == service.GameRuntimeStateLaunching:
		return locales.PlayingIslandLaunching()
	case r.State == service.GameRuntimeStateEnding || s.ending:
		return locales.PlayingIslandEnding()
	}
	secs := 0
	if r.ActiveSeconds != nil {
		secs = *r.ActiveSeconds
	} else if !r.StartTime.IsZero() {
		elapsed := max(0, now.Sub(r.StartTime))
		secs = int(elapsed / time.Second)
		c.After(time.Minute - elapsed%time.Minute)
	}
	return locales.PlayingIslandElapsed(timefmt.DurationCompact(secs))
}

func (s *PlayingIsland) body(c *ui.Context, t *theme.Theme, p PlayingIslandProps, now time.Time, exiting bool) {
	// 切换到另一个运行状态时，旧的封面与文字淡出，新的淡入。
	if s.display == nil || s.display.key() == s.render.key() {
		s.display = s.render
	} else {
		s.outgoing = &islandSnapshot{game: s.display.Game, status: s.statusText(c, s.display, now)}
		s.display, s.ending, s.switchedAt = s.render, false, now
	}
	switching := !s.switchedAt.IsZero()
	if switching && now.Sub(s.switchedAt) >= islandSwitchDur {
		s.outgoing, s.switchedAt, switching = nil, time.Time{}, false
	}
	if switching {
		c.AnimationFrame()
	}
	r := s.display
	key := r.key()
	status := s.statusText(c, r, now)
	multiple := p.Count > 1

	// 开始游玩 AUTO_COLLAPSE_AFTER_PLAYING_SECONDS 秒后自动收起，每个运行状态只收起一次。
	if !exiting && !s.collapsed && r.State == service.GameRuntimeStatePlaying && s.autoCollapsed != key {
		if s.playingKey != key || s.playingSince.IsZero() {
			s.playingKey, s.playingSince = key, now
		}
		if d := now.Sub(s.playingSince); d >= islandAutoCollapse {
			s.autoCollapsed, s.collapsed = key, true
		} else {
			c.After(islandAutoCollapse - d)
		}
	} else {
		s.playingSince = time.Time{}
	}

	winW, _ := c.Size()
	frame := ui.Box(c.Key("playing-island")).Absolute().Top(islandTop).PassThrough()
	dragOffset := float32(0)
	if s.pressing {
		if s.pressCollapse {
			dragOffset = min(max(s.pressDY, 0), islandHideDistance)
		} else {
			dragOffset = max(min(s.pressDY, 0), islandCollapseY)
		}
	}
	dragging := dragOffset != 0
	moveDur := islandMoveDur
	if dragging {
		moveDur = time.Millisecond // transition-none
	}
	targetW := float32(islandCollapsedW)
	if !s.collapsed || dragOffset > 0 {
		targetW = min(islandExpandedW, winW-144)
	}
	targetY := dragOffset
	if s.collapsed {
		targetY += islandCollapseY
	}
	w := frame.AnimateWith("width", targetW, moveDur, easeIsland)
	y := frame.AnimateWith("y", targetY, moveDur, easeIsland)
	frame.Left((winW-w)/2).Width(w).Translate(0, y)

	hide := min(-dragOffset/islandHideDistance, 1)
	if s.collapsed {
		hide = 1 - min(max(dragOffset, 0)/islandHideDistance, 1)
	}
	fadeDur := islandFadeDur
	if dragging {
		fadeDur = time.Millisecond
	}

	frame.Children(func() {
		// bg-white/96 dark:bg-black，data-glass:bg-white/82 data-glass:dark:bg-black/62
		pill := ui.Box(c.Key("pill")).FillWidth().Height(islandH).Radius(999).Clip().
			Background(islandBg(t)).
			Shadow(0, 0, 0, t.HL, t.Pick(theme.Brand900.Alpha(0.1), theme.White.Alpha(0.12))). // ring-1
			Cursor(ui.CursorGrab)
		s.islandPose(c, pill, now, exiting)
		if s.collapsed {
			pill.Focusable().Label(locales.PlayingIslandExpand())
			if pill.Clicked() {
				s.collapsed = false
			}
			pill.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind == ui.InputKeyDown && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) && s.collapsed {
					s.collapsed = false
					return true
				}
				return false
			})
		}
		s.drag(pill)
		if dragging {
			pill.Cursor(ui.CursorGrabbing)
		}

		contentOpacity := pill.AnimateWith("content", 1-hide, fadeDur, theme.EaseOut)
		handleOpacity := pill.AnimateWith("handle", max(hide, theme.Bool(s.collapsed)), fadeDur, theme.EaseOut)
		pill.Children(func() {
			row := ui.Row(c.Key("content")).Fill().Gap(12).Padding(0, 12).AlignItems(ui.Center).Opacity(contentOpacity)
			if s.collapsed && !dragging {
				row.PassThrough()
			}
			row.Children(func() {
				s.cover(c, t, p, r, multiple, switching)
				s.titles(c, t, r, status, w, switching)
				s.endButton(c, t, p, r)
			})
			// 拖动条：absolute inset-x-0 bottom-1.5，h-1 w-16 rounded-full bg-brand-500/55 dark:bg-white/68。
			ui.Row(c.Key("handle")).Absolute().Left(0).Right(0).Bottom(6).Justify(ui.Center).PassThrough().
				Opacity(handleOpacity).Children(func() {
				ui.Box(c).Size(64, 4).Radius(999).Background(t.Pick(theme.Brand500.Alpha(0.55), theme.White.Alpha(0.68))).
					Shadow(0, 0, 16, 0, t.Pick(ui.RGBA(15, 23, 42, 0.1), ui.RGBA(255, 255, 255, 0.1)))
			})
		})
	})
}

// islandPose 是入场与退场动画（animate-playing-island-enter / leave）：以中心缩放并淡入淡出。
func (s *PlayingIsland) islandPose(c *ui.Context, pill ui.Element, now time.Time, exiting bool) {
	sx, sy, op := float32(1), float32(1), float32(1)
	switch {
	case exiting:
		k := easeIslandOut(min(float32(now.Sub(s.exitingAt))/float32(islandExitDur), 1))
		sx, sy, op = lerp(1, 0.22, k), lerp(1, 0.74, k), 1-k
	case now.Sub(s.mountedAt) < islandEnterDur:
		// {0%{opacity:0;scaleX(.18) scaleY(.72)}62%{opacity:1;scaleX(1.05) scaleY(1.02)}100%{scale(1)}}，每段各自缓动。
		p := float32(now.Sub(s.mountedAt)) / float32(islandEnterDur)
		if p < 0.62 {
			k := easeIslandIn(p / 0.62)
			sx, sy, op = lerp(0.18, 1.05, k), lerp(0.72, 1.02, k), k
		} else {
			k := easeIslandIn((p - 0.62) / 0.38)
			sx, sy = lerp(1.05, 1, k), lerp(1.02, 1, k)
		}
		c.AnimationFrame()
	}
	pill.Scale(sx, sy).Opacity(op)
}

// drag 对应 handlePointerDown / Move / Up：展开时向上拖、收起时向下拖，超过 COLLAPSE_DRAG_THRESHOLD 松开即切换。
// 按在封面与结束按钮上的不算（INTERACTIVE_SELECTOR）：它们自己接住按下。
func (s *PlayingIsland) drag(pill ui.Element) {
	_, dy, pressed := pill.Dragged()
	if pressed {
		if !s.pressing {
			s.pressing, s.pressCollapse, s.pressDY = true, s.collapsed, 0
		}
		s.pressDY += dy
		return
	}
	if !s.pressing {
		return
	}
	if _, _, ended := pill.DragEnded(); ended {
		switch {
		case s.pressCollapse && s.pressDY >= islandDragThreshold:
			s.collapsed = false
		case !s.pressCollapse && s.pressDY <= -islandDragThreshold:
			s.collapsed = true
		}
	}
	s.pressing, s.pressDY = false, 0 // 松开，或 onPointerCancel
}

func lerp(a, b, k float32) float32 { return a + (b-a)*k }

// contentIn 与 contentOut 是 animate-playing-island-content-in / content-out（淡入淡出并去掉或加上 2px 模糊）的进度。
func contentIn(now, since time.Time) float32 {
	return easeIsland(min(float32(now.Sub(since))/float32(islandSwitchDur), 1))
}

func contentOut(now, since time.Time) float32 {
	return easeContentOut(min(float32(now.Sub(since))/float32(islandContentOutDur), 1))
}

// cover：relative h-9 w-9 shrink-0 的封面按钮（多个游戏时点击切换，悬停时封面变暗模糊并显示箭头），右上角是游戏数角标；
// 切换时旧封面在上面淡出。
func (s *PlayingIsland) cover(c *ui.Context, t *theme.Theme, p PlayingIslandProps, r *IslandRuntime, multiple, switching bool) {
	now := c.Now()
	ui.Box(c.Key("cover")).Size(36, 36).Shrink(0).Children(func() {
		in := float32(1)
		if switching {
			in = contentIn(now, s.switchedAt)
		}
		btn := ui.ButtonBase(c.Key("switch")).Fill().Radius(999).Clip().FocusRing(false).
			Background(t.Pick(theme.Brand100, theme.Brand800)).
			Label(locales.PlayingIslandSwitchGame(strconv.Itoa(p.Count))).Opacity(in)
		hover := float32(0)
		if multiple { // 只有一个游戏时按钮 disabled，cursor-default，点击什么也不做
			btn.Cursor(ui.CursorPointer)
			hover = btn.AnimateWith("hover", theme.Bool(btn.Hovered()), 180*time.Millisecond, theme.EaseTailwind)
			if p.OnSwitch != nil {
				btn.OnClick(p.OnSwitch)
			}
		}
		if switching {
			btn.Blur(2 * (1 - in))
		}
		btn.Children(func() {
			s.coverContent(c, t, p, &r.Game, btn, hover)
			if multiple {
				// absolute inset-0 bg-brand-900/22 dark:bg-black/28，i-mdi-arrow-right text-lg text-white，悬停时显示。
				ui.Box(c.Key("arrow")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().
					Background(t.Pick(theme.Brand900.Alpha(0.22), theme.Black.Alpha(0.28))).Opacity(hover).Children(func() {
					components.Icon(c, "arrow-right", 18, theme.White)
				})
			}
			// ring-1 ring-brand-900/10 dark:ring-white/16（悬停时 /18、/24）
			ring := t.Pick(theme.Brand900.Alpha(0.1+0.08*hover), theme.White.Alpha(0.16+0.08*hover))
			ui.Box(c.Key("ring")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(999).PassThrough().Border(t.HL, ring)
		})
		if s.outgoing != nil {
			ui.Box(c.Key("outgoing-cover")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(999).Clip().PassThrough().
				Background(t.Pick(theme.Brand100, theme.Brand800)).Opacity(1 - contentOut(now, s.switchedAt)).
				Blur(2 * contentOut(now, s.switchedAt)).Children(func() {
				s.coverContent(c, t, p, &s.outgoing.game, ui.Element{}, 0)
			})
		}
		if multiple {
			// absolute -right-1 -top-1 h-4 min-w-4 rounded-full bg-white px-1 text-[10px] font-semibold leading-none
			// text-brand-900 ring-1 ring-brand-900/16 shadow-[0_2px_7px_rgba(0,0,0,0.18)]
			ui.Row(c.Key("count")).Absolute().Top(-4).Right(-4).Height(16).MinWidth(16).Padding(0, 4).Radius(999).
				Center().PassThrough().Background(theme.White).
				Shadow(0, 0, 0, t.HL, t.Pick(theme.Brand900.Alpha(0.16), theme.Black.Alpha(0.5))).
				Shadow(0, 2, 7, 0, ui.RGBA(0, 0, 0, 0.18)).Children(func() {
				components.Text(c, strconv.Itoa(p.Count), theme.FontStep{Size: 10, Line: 10}, theme.WeightSemibold, theme.Brand900).NoWrap()
			})
		}
	})
}

// coverContent 对应 renderCoverContent：封面图（GameCoverImage，NSFW 按设置模糊），没有封面时是游戏手柄图标；
// 多个游戏时悬停变暗模糊（group-hover:blur-[1.5px] group-hover:brightness-75）。
func (s *PlayingIsland) coverContent(c *ui.Context, t *theme.Theme, p PlayingIslandProps, g *models.Game, host ui.Element, hover float32) {
	var bmp *ui.Bitmap
	if p.Env != nil && p.Env.Covers != nil && (g.CoverURL != "" || g.CoverSourceURL != "") {
		bmp = p.Env.Covers.Get(g)
	}
	if bmp == nil {
		ui.Box(c.Key("placeholder")).Fill().Center().Children(func() {
			components.Icon(c, "gamepad-variant", 18, t.Pick(theme.Brand500, theme.White.Alpha(0.65)))
		})
		return
	}
	img := ui.Image(c, bmp).Fill().Fit(ui.Cover)
	switch cfg := p.Env.Config(); {
	case g.IsNSFW && (cfg == nil || cfg.BlurNSFWGameCovers) && host.Valid():
		components.CoverBlur(c, host, img, false)
	case g.IsNSFW && (cfg == nil || cfg.BlurNSFWGameCovers):
		img.Blur(18) // 切换时淡出的旧封面没有悬停
	case hover > 0:
		img.Blur(1.5 * hover)
	}
	if hover > 0 {
		ui.Box(c.Key("dim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Background(theme.Black.Alpha(0.25 * hover))
	}
}

// titles：relative min-w-0 flex-1 的标题（text-sm font-semibold leading-5，放不下时 8 秒一轮的跑马灯）与状态
// （text-xs leading-4 text-brand-600 dark:text-white/68）；切换时旧的标题与状态盖在上面淡出。
func (s *PlayingIsland) titles(c *ui.Context, t *theme.Theme, r *IslandRuntime, status string, islandW float32, switching bool) {
	now := c.Now()
	statusColor := t.Pick(theme.Brand600, theme.White.Alpha(0.68))
	titleFont := theme.FontStep{Size: 14, Line: 20}
	statusFont := theme.FontStep{Size: 12, Line: 16}
	ui.Column(c.Key("titles")).Grow(1).Basis(0).MinWidth(0).Clip().Children(func() {
		in := float32(1)
		if switching {
			in = contentIn(now, s.switchedAt)
		}
		viewport := islandW - islandTitleViewportPad
		titleW, _ := c.MeasureText(0, ui.Span{Text: r.Game.Name, Font: theme.UIFont, Size: titleFont.Size, Weight: theme.WeightSemibold})
		// 原版收起（且不在拖动）时不测量标题，不滚动。
		marquee := titleW > viewport+1 && (!s.collapsed || s.pressing)
		title := ui.Box(c.Key("title")).FillWidth().Height(titleFont.Line).ClipX().Opacity(in)
		if switching {
			title.Blur(2 * (1 - in))
		}
		title.Children(func() {
			if !marquee {
				components.Text(c, r.Game.Name, titleFont, theme.WeightSemibold, t.TextStrong).SingleLine()
				return
			}
			// animate-playing-island-marquee：{0%,16%{translateX(0)}84%,100%{translateX(-50%)}}，8s linear infinite。
			// 内容是标题、px-8 的第二份标题，-50% 正好是第二份标题的起点，下一轮从 0 开始时看不出跳动。
			k := title.Loop("marquee", islandMarqueePeriod, ui.Linear)
			shift := float32(0)
			switch {
			case k >= 0.84:
				shift = 1
			case k > 0.16:
				shift = (k - 0.16) / 0.68
			}
			ui.Row(c.Key("marquee")).Absolute().Top(0).Left(0).Translate(-shift*(titleW+islandMarqueeSpacing), 0).Children(func() {
				components.Text(c, r.Game.Name, titleFont, theme.WeightSemibold, t.TextStrong).NoWrap()
				ui.Box(c).Width(islandMarqueeSpacing).Shrink(0)
				components.Text(c, r.Game.Name, titleFont, theme.WeightSemibold, t.TextStrong).NoWrap()
				ui.Box(c).Width(islandMarqueeSpacing).Shrink(0)
			})
		})
		st := components.Text(c, status, statusFont, theme.WeightNormal, statusColor).SingleLine().Opacity(in)
		if switching {
			st.Blur(2 * (1 - in))
		}
		if o := s.outgoing; o != nil {
			out := contentOut(now, s.switchedAt)
			ui.Column(c.Key("outgoing-titles")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
				Background(islandBg(t)).Opacity(1 - out).Blur(2 * out).Children(func() {
				components.Text(c, o.game.Name, titleFont, theme.WeightSemibold, t.TextStrong).SingleLine()
				components.Text(c, o.status, statusFont, theme.WeightNormal, statusColor).SingleLine()
			})
		}
	})
}

// endButton：h-9 w-9 rounded-full 的结束按钮（text-brand-700 dark:text-white，hover:bg-brand-900/8 dark:hover:bg-white/12，
// active:scale-95，disabled:opacity-55）；结束中显示加载图标。
func (s *PlayingIsland) endButton(c *ui.Context, t *theme.Theme, p PlayingIslandProps, r *IslandRuntime) {
	ending := r.State == service.GameRuntimeStateEnding || s.ending
	btn := ui.ButtonBase(c.Key("end")).Size(36, 36).Shrink(0).Radius(999).Center().FocusRing(false).
		Label(locales.PlayingIslandEnd())
	color := t.Pick(theme.Brand700, theme.White)
	scale := float32(1)
	switch {
	case ending:
		components.Disable(btn, 0.55)
	case btn.Pressed():
		scale = 0.95
		fallthrough
	case btn.Hovered():
		btn.Background(t.Pick(theme.Brand900.Alpha(0.08), theme.White.Alpha(0.12)))
	}
	btn.Transition(theme.ColorsTransition(150*time.Millisecond)).Scale(scale, scale)
	if !ending {
		btn.OnClick(func() { s.end(c, p, r.Game.ID) })
	}
	btn.Children(func() {
		if ending {
			components.Spinner(c, 20, color)
		} else {
			components.Icon(c, "stop", 20, color)
		}
	})
}

// end 对应 handleEndPlay：结束当前会话，成功时提示；失败时提示并恢复按钮。
func (s *PlayingIsland) end(c *ui.Context, p PlayingIslandProps, gameID string) {
	if gameID == "" || s.ending || p.OnEnd == nil {
		return
	}
	s.ending = true
	p.OnEnd(gameID, func(err error) {
		if err != nil {
			log.Printf("Failed to end current play session: %v", err)
			p.Env.Toast(c, env.Error(locales.PlayingIslandToastEndFailed()))
			if s.display != nil && s.display.Game.ID == gameID {
				s.ending = false
			}
			return
		}
		p.Env.Toast(c, env.Success(locales.PlayingIslandToastEndSuccess()))
	})
}

// islandBg 是灵动岛的底色：bg-white/96 dark:bg-black，玻璃模式下 data-glass:bg-white/82 data-glass:dark:bg-black/62。
func islandBg(t *theme.Theme) ui.Color {
	return t.IfGlass(t.Pick(theme.White.Alpha(0.96), theme.Black), theme.White.Alpha(0.82), theme.Black.Alpha(0.62))
}
