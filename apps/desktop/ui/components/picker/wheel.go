package picker

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// 原版 BetterNumberWheelPicker 给 @ncdai/react-wheel-picker 1.2.2 的 WheelPicker 传的参数，以及库里由它们算出的几何量：
// 选项排在一个绕 x 轴的圆柱面上，每项占 360/visibleCount 度，圆柱半径让每项正好 optionItemHeight 高。
const (
	itemH             = 32               // optionItemHeight
	visibleCount      = 20               // visibleCount
	halfVisible       = visibleCount / 4 // 库里的 R = visibleCount >> 2：离中心超过 5 项的选项隐藏
	scrollSensitivity = 5                // scrollSensitivity：滚动 r 项用时 sqrt(r/5) 秒
	dragDecel         = 2.6 * 10         // dragSensitivity*10：松手后惯性的减速度（项/秒²）
	maxDragVelocity   = 30               // 松手速度上限（项/秒）
	wheelThrottle     = 100 * time.Millisecond
	typeaheadPause    = 500 * time.Millisecond
	clickSlop         = 5 // 按下后移动不超过 5px 算点击
)

var (
	stepRad = 2 * math.Pi / visibleCount
	// radius 是圆柱半径 U = itemH / tan(18°)；wheelHeight 是 [data-rwp] 的高度 round(2U + itemH/4) = 205。
	radius      = itemH / math.Tan(stepRad)
	wheelHeight = float32(math.Round(radius*2 + itemH*0.25))
)

// wheelProps 是一列滚轮（WheelPicker infinite）的属性。
type wheelProps struct {
	key      string
	value    int
	options  []WheelOption
	onChange func(int)
	disabled bool
}

// wheelAnim 是库里 ue() 的一段缓动：from → to，时长 dur 秒，easeOutCubic。start 在下一次构建时取 c.Now()。
type wheelAnim struct {
	active   bool
	from, to float64
	dur      float64
	start    time.Time
}

type dragSample struct {
	y float32
	t time.Time
}

// wheelDrag 是按住拖动的状态（库里的 ee.current）。
type wheelDrag struct {
	active  bool
	isClick bool
	startY  float32
	touch   float64 // 拖动中显示的位置（touchScroll）
	samples []dragSample
}

// wheelSig 是 useEffect([value, options]) 的依赖：变了就取消动画、跳到值所在的位置。
type wheelSig struct{ value, n, first, last int }

// wheelState 是一列滚轮的状态，挂在列元素上。pos 对应库里的 v.current：静止或动画中的位置（选项序号，可带小数），
// shown 是画出来的位置（拖动时是 touchScroll）。
type wheelState struct {
	pos, shown float64
	anim       wheelAnim
	drag       wheelDrag
	synced     bool
	sig        wheelSig
	scrollAt   time.Time
	typed      string
	typedAt    time.Time
}

// repeatOptions 对应库里 infinite 时的选项表：重复到至少 ceil(visibleCount/2) 项，圆柱才转得满。
func repeatOptions(options []WheelOption) []WheelOption {
	if len(options) == 0 {
		return nil
	}
	out := make([]WheelOption, 0, max(len(options), visibleCount/2))
	for len(out) < (visibleCount+1)/2 {
		out = append(out, options...)
	}
	return out
}

func wrapPos(v float64, n int) float64 {
	m := math.Mod(v, float64(n))
	if m < 0 {
		m += float64(n)
	}
	return m
}

func easeOutCubic(x float64) float64 { return math.Pow(x-1, 3) + 1 }

func sign(v float64) float64 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// scrollBy 对应库里的 P(e)：从当前位置缓动到 round(pos+delta)，用时 sqrt(距离/scrollSensitivity) 秒。
func (st *wheelState) scrollBy(delta float64) {
	t := st.pos
	target := math.Round(t + delta)
	r := math.Abs(target - t)
	if r == 0 {
		return
	}
	st.anim = wheelAnim{active: true, from: t, to: target, dur: math.Sqrt(r / scrollSensitivity)}
}

// commit 对应库里的 J(e)：位置截断（不是四舍五入）到所在的选项并停在那里，值不同就回调。
func (st *wheelState) commit(e float64, items []WheelOption, value int, onChange func(int)) {
	n := len(items)
	// 指针坐标换算成 DIP 有浮点误差（拖两项可能得到 1.9999999），截断前加一点余量，免得退回上一项。
	idx := int(wrapPos(e+1e-6, n)) % n
	st.pos, st.shown = float64(idx), float64(idx)
	if v := items[idx].Value; v != value && onChange != nil {
		onChange(v)
	}
}

// fling 对应库里的 Fe(e)：以松手速度 e（项/秒）匀减速滚动并吸附到整数位置。
// 库在启动缓动后立即调用 J(当前位置)：受控用法下，截断后的值与原值不同时父组件会改值，
// useEffect 随即取消惯性、停在截断的位置；速度为 0 时同样停在截断的位置。这里照搬。
func (st *wheelState) fling(e float64, items []WheelOption, value int, onChange func(int)) {
	t := st.pos
	a := float64(dragDecel)
	if e > 0 {
		a = -dragDecel
	}
	s := math.Abs(e / a)
	target := math.Round(t + e*s + 0.5*a*s*s)
	st.anim = wheelAnim{}
	if t != target && s > 0 {
		st.anim = wheelAnim{active: true, from: t, to: target, dur: s}
	}
	st.commit(t, items, value, onChange)
}

// clickIndex 对应库里的 Ue：按库里预先算好的各项高度表（从列顶部往下，b·cos(r·18°)，r = 4…-4）
// 找出点击落在哪一项，返回相对中心的偏移；落在表外（底部约 3px）返回 false。
func clickIndex(y float32) (int, bool) {
	acc := 0.0
	for s, r := 0, halfVisible-1; r >= -halfVisible+1; s, r = s+1, r-1 {
		h := itemH * math.Cos(float64(r)*stepRad)
		if float64(y) >= acc && float64(y) <= acc+h {
			return s - (halfVisible - 1), true
		}
		acc += h
	}
	return 0, false
}

// typeahead 对应库里的 useTypeaheadSearch：500ms 内连续输入的字符拼起来，按选项文字的前缀查找；
// 只有一个字符（或同一个字符重复）时从当前项的下一项起循环查找。
func (st *wheelState) typeahead(ch string, items []WheelOption, now time.Time) (int, bool) {
	if now.Sub(st.typedAt) >= typeaheadPause {
		st.typed = ""
	}
	st.typedAt = now
	st.typed += strings.ToLower(ch)
	query := st.typed
	if r := []rune(query); len(r) > 1 && strings.Count(query, string(r[0])) == len(r) {
		query = string(r[0])
	}
	n := len(items)
	if len([]rune(query)) == 1 {
		cur := int(math.Round(st.pos))
		if cur >= n {
			cur = 0
		}
		for w := 1; w <= n; w++ {
			i := (cur + w) % n
			if strings.HasPrefix(strings.ToLower(items[i].Label), query) {
				return i, true
			}
		}
		return 0, false
	}
	for i, it := range items {
		if strings.HasPrefix(strings.ToLower(it.Label), query) {
			return i, true
		}
	}
	return 0, false
}

// wheelColumn 是一列 [data-rwp]：flex-1、高 205、overflow-hidden，上下 20% 用 mask-image 渐隐。
// 选项是 text-xl font-semibold tabular-nums text-brand-500/45 dark:text-brand-400/35，按圆柱投影画（正交投影，
// 第 k 项中心在 U·sin(18°·k)、高度乘 cos(18°·k)）；中间 h-8 的高亮窗里平铺地画同一串选项，
// 颜色 text-brand-900 dark:text-white。可以聚焦：↑↓ 移一项，输入数字按前缀跳转。
func wheelColumn(c *ui.Context, t *theme.Theme, p wheelProps) {
	col := ui.Box(c.Key(p.key)).Grow(1).Basis(0).MinWidth(0).Height(wheelHeight).Clip().FocusRing(false)
	st := ui.Local(col, "wheel", func() wheelState { return wheelState{} })
	items := repeatOptions(p.options)
	n := len(items)
	if n == 0 {
		return
	}

	sig := wheelSig{value: p.value, n: len(p.options), first: p.options[0].Value, last: p.options[len(p.options)-1].Value}
	if !st.synced || st.sig != sig {
		st.synced, st.sig = true, sig
		for i, it := range items {
			if it.Value == p.value {
				st.anim = wheelAnim{}
				st.pos, st.shown = float64(i), float64(i)
				break
			}
		}
	}
	if st.anim.active && !st.drag.active {
		now := c.Now()
		if st.anim.start.IsZero() {
			st.anim.start = now
		}
		a := st.anim
		if el := now.Sub(a.start).Seconds(); el < a.dur {
			st.pos = wrapPos(a.from+easeOutCubic(el/a.dur)*(a.to-a.from), n)
			st.shown = st.pos
			c.AnimationFrame()
		} else {
			st.anim = wheelAnim{}
			st.commit(a.to, items, p.value, p.onChange)
			c.AnimationFrame() // 回调改了父组件的状态，再构建一次
		}
	}

	if !p.disabled {
		col.Focusable().HandleInput(func(ev ui.InputEvent) bool {
			return st.input(ev, items, p.value, p.onChange)
		})
	}

	shown := st.shown
	optionColor := t.Pick(theme.Brand500.Alpha(0.45), theme.Brand400.Alpha(0.35))
	highlightColor := t.Pick(theme.Brand900, theme.White)
	col.Draw(func(pt *ui.Painter, r ui.Rect) {
		drawWheel(pt, r, items, shown, optionColor, highlightColor)
	})
}

// input 处理一列滚轮的输入（库里挂在元素与 document 上的监听）。
func (st *wheelState) input(ev ui.InputEvent, items []WheelOption, value int, onChange func(int)) bool {
	n := len(items)
	switch ev.Kind {
	case ui.InputScroll:
		// 每 100ms 最多响应一次，每次按 deltaY 的符号移一项；页面不随之滚动。
		now := ev.Time
		if now.Sub(st.scrollAt) < wheelThrottle {
			return true
		}
		if d := sign(float64(ev.DY)); d != 0 {
			st.scrollAt = now
			st.scrollBy(d)
		}
		return true
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		st.anim = wheelAnim{}
		st.drag = wheelDrag{active: true, isClick: true, startY: ev.Y, touch: st.pos,
			samples: []dragSample{{ev.Y, ev.Time}}}
		return true
	case ui.InputPointerMove:
		if !st.drag.active {
			return false
		}
		d := &st.drag
		if d.isClick && math.Abs(float64(ev.Y-d.startY)) > clickSlop {
			d.isClick = false
		}
		d.samples = append(d.samples, dragSample{ev.Y, ev.Time})
		if len(d.samples) > 5 {
			d.samples = d.samples[1:]
		}
		d.touch = wrapPos(st.pos+float64(d.startY-ev.Y)/itemH, n)
		st.shown = d.touch
		return true
	case ui.InputPointerUp:
		if !st.drag.active {
			return false
		}
		d := st.drag
		st.drag = wheelDrag{}
		if d.isClick {
			if off, ok := clickIndex(d.startY); ok {
				st.scrollBy(float64(off))
			}
			return true
		}
		velocity := 0.0
		if k := len(d.samples); k > 1 {
			a, b := d.samples[k-2], d.samples[k-1]
			if dt := float64(b.t.Sub(a.t).Milliseconds()); dt > 0 {
				v := float64(a.y-b.y) / itemH * 1000 / dt
				velocity = math.Min(math.Abs(v), maxDragVelocity) * sign(v)
			}
		}
		st.pos = d.touch
		st.fling(velocity, items, value, onChange)
		return true
	case ui.InputKeyDown:
		switch ev.Key {
		case ui.KeyUp:
			st.scrollBy(-1)
			return true
		case ui.KeyDown:
			st.scrollBy(1)
			return true
		case ui.KeyHome, ui.KeyEnd:
			return true // infinite 模式下库里不处理，也不让焦点组拿去
		}
		return false // ←→ 交给外层的 FocusGroup，在几列之间移动焦点
	case ui.InputText:
		for _, r := range ev.Text {
			if i, ok := st.typeahead(string(r), items, ev.Time); ok {
				st.scrollBy(float64(i) - math.Round(st.pos))
			}
		}
		return ev.Text != ""
	}
	return false
}

// wheelMask 是 mask-image: linear-gradient(to bottom, transparent, black 20%, black 80%, transparent) 在 y 处的不透明度。
func wheelMask(y, h float32) float32 {
	edge := h * 0.2
	switch {
	case y <= 0 || y >= h:
		return 0
	case y < edge:
		return y / edge
	case y > h-edge:
		return (h - y) / edge
	}
	return 1
}

const maskStrip = 2 // 渐隐区里按 2 DIP 高的横条分段画，每条取条中心的不透明度

func wheelSpan(label string, color ui.Color) ui.Span {
	return ui.Span{Text: label, Font: theme.UIFont, Size: 20, Weight: theme.WeightSemibold, Color: color, Features: "tnum"}
}

// drawWheel 画一列滚轮：圆柱上的选项（渐隐），再在中间 32px 的高亮窗里平铺地画高亮色的选项。
func drawWheel(p *ui.Painter, r ui.Rect, items []WheelOption, shown float64, optionColor, highlightColor ui.Color) {
	n := len(items)
	cx, mid := r.X+r.W/2, r.Y+r.H/2
	base := int(math.Floor(shown))
	for k := base - halfVisible; k <= base+halfVisible+1; k++ {
		d := float64(k) - shown
		if math.Abs(d) > halfVisible {
			continue
		}
		scale := float32(math.Cos(d * stepRad))
		if scale < 0.01 {
			continue
		}
		cy := mid + float32(radius*math.Sin(d*stepRad))
		label := items[((k%n)+n)%n].Label
		top, bot := cy-itemH/2*scale, cy+itemH/2*scale
		draw := func(color ui.Color) {
			span := wheelSpan(label, color)
			w, h := p.MeasureText(0, span)
			p.Transform(ui.Scaling(1, scale), cx, cy, func() {
				p.RichText(cx-w/2, cy-h/2, 0, span)
			})
		}
		if wheelMask(top-r.Y, r.H) >= 1 && wheelMask(bot-r.Y, r.H) >= 1 {
			draw(optionColor)
			continue
		}
		for y := top; y < bot; y += maskStrip {
			y1 := min(y+maskStrip, bot)
			a := wheelMask((y+y1)/2-r.Y, r.H)
			if a <= 0 {
				continue
			}
			p.Clip(ui.Rect{X: r.X, Y: y, W: r.W, H: y1 - y}, 0, func() { draw(optionColor.Alpha(a)) })
		}
	}
	band := ui.Rect{X: r.X, Y: mid - itemH/2, W: r.W, H: itemH}
	p.Clip(band, 0, func() {
		for k := base - 1; k <= base+2; k++ {
			cy := mid + float32((float64(k)-shown)*itemH)
			span := wheelSpan(items[((k%n)+n)%n].Label, highlightColor)
			w, h := p.MeasureText(0, span)
			p.RichText(cx-w/2, cy-h/2, 0, span)
		}
	})
}
