package better

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// NumberInputSize 对应 BetterNumberInput 的 size。
type NumberInputSize int

const (
	NumberInputMD NumberInputSize = iota // h-11 w-36 rounded-xl，步进按钮 w-9（默认）
	NumberInputSM                        // h-9 w-25 rounded-lg，步进按钮 w-7
)

// NumberInputProps 是 BetterNumberInput 的属性。Min、Max 为 nil 时不限制。
type NumberInputProps struct {
	Min, Max *int
	Step     int // 默认 1
	Unit     string
	Size     NumberInputSize
	Disabled bool
	Label    string // 无障碍名称
	OnChange func(value int)
}

// Bound 返回 v 的指针，用于 NumberInputProps 的 Min、Max。
func Bound(v int) *int { return &v }

type numberInputState struct {
	draft   string
	focused bool
	nudge   int // OnTextKey 里按下的 ↑ / ↓，下一遍构建时应用
}

func (p NumberInputProps) clamp(v int) int {
	if p.Min != nil && v < *p.Min {
		return *p.Min
	}
	if p.Max != nil && v > *p.Max {
		return *p.Max
	}
	return v
}

func (p NumberInputProps) inRange(v int) bool {
	return (p.Min == nil || v >= *p.Min) && (p.Max == nil || v <= *p.Max)
}

// parseDraft 同 Number.parseInt：取开头的整数部分。
func parseDraft(s string) (int, bool) {
	s = strings.TrimSpace(s)
	end := 0
	if end < len(s) && s[end] == '-' {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v, err := strconv.ParseInt(s[:end], 10, 64)
	if err != nil {
		return 0, false
	}
	return int(max(math.MinInt32, min(math.MaxInt32, v))), true
}

// digitsOnly 同 replace(/[^\d-]/g, "")。
func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, s)
}

// NumberInput 对应 BetterNumberInput：inline-flex items-center gap-2；带边框的框 border-brand-300 bg-white
// dark:border-brand-600 dark:bg-brand-700，focus-within:border-neutral-500 ring-2 ring-neutral-500/30，disabled:opacity-60；
// 输入框 font-medium tabular-nums text-sm text-brand-900 dark:text-white；右侧上下两个步进按钮（border-l border-brand-200
// bg-brand-50/80 dark:border-brand-600 dark:bg-brand-700，hover:bg-brand-200 dark:hover:bg-brand-700）。
//
// 输入时只在数字在范围内时提交；失去焦点或按 Enter 时把草稿收回到范围内（原版 Enter 是 blur，MyGo 不能让输入框失去焦点）；
// ↑ / ↓ 按 Step 增减。返回外层的行。
func NumberInput(c *ui.Context, t *theme.Theme, key string, value int, p NumberInputProps) ui.Element {
	if p.Step == 0 {
		p.Step = 1
	}
	h, w, radius, padX, stepW, iconSize := float32(44), float32(144), float32(12), float32(16), float32(36), float32(16)
	if p.Size == NumberInputSM {
		h, w, radius, padX, stepW, iconSize = 36, 100, 8, 12, 28, 14
	}
	row := ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(8).Shrink(0)
	st := ui.Local(row, "state", func() *numberInputState { return &numberInputState{draft: strconv.Itoa(value)} })
	s := *st
	commit := func(v int) {
		if v != value && p.OnChange != nil {
			p.OnChange(v)
		}
	}
	nudge := func(dir int) {
		if p.Disabled {
			return
		}
		base := value
		if v, ok := parseDraft(s.draft); ok {
			base = v
		}
		next := p.clamp(base + p.Step*dir)
		s.draft = strconv.Itoa(next)
		commit(next)
	}
	if s.nudge != 0 {
		dir := s.nudge
		s.nudge = 0
		nudge(dir)
	}

	row.Children(func() {
		bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600)) // glass-input
		box := ui.Row(c.Key("box")).Size(w, h).Radius(radius).Clip().AlignItems(ui.Stretch).Background(bg)
		if p.Disabled {
			components.Disable(box, 0.6)
		}
		var in ui.Element
		box.Children(func() {
			draft := s.draft
			in = ui.TextInputBase(c.Key("input"), &draft).Grow(1).Basis(0).MinWidth(0).Padding(0, padX).
				FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).FontWeight(theme.WeightMedium).
				TextColor(t.Pick(theme.Brand900, theme.White))
			if theme.UIFont != "" {
				in.Font(theme.UIFont)
			}
			if p.Label != "" {
				in.Label(p.Label)
			}
			in.OnTextKey(func(k ui.TextKey) bool {
				switch k.Key {
				case ui.KeyUp:
					s.nudge = 1
				case ui.KeyDown:
					s.nudge = -1
				default:
					return false
				}
				return true
			})
			focused := in.Focused()
			switch {
			case in.Changed():
				s.draft = digitsOnly(draft)
				if v, ok := parseDraft(s.draft); ok && p.inRange(v) {
					commit(v)
				}
			case in.Submitted() || (s.focused && !focused):
				next := p.clamp(value)
				if v, ok := parseDraft(s.draft); ok {
					next = p.clamp(v)
				}
				s.draft = strconv.Itoa(next)
				commit(next)
			case !focused:
				s.draft = strconv.Itoa(value)
			}
			s.focused = focused

			ui.Column(c.Key("steppers")).Width(stepW).Shrink(0).
				BorderWidth(0, 0, 0, t.HL).BorderColor(t.Pick(theme.Brand200, theme.Brand600)).
				Background(t.Pick(theme.Brand50.Alpha(0.8), theme.Brand700)).Children(func() {
				stepButton(c, t, "up", "chevron-up", iconSize, p.Disabled, func() { nudge(1) })
				ui.Box(c).Height(t.HL).Background(t.Pick(theme.Brand200, theme.Brand700)).Shrink(0)
				stepButton(c, t, "down", "chevron-down", iconSize, p.Disabled, func() { nudge(-1) })
			})
		})
		if in.Focused() {
			border = theme.Neutral500
			box.Shadow(0, 0, 0, 2, theme.Neutral500.Alpha(0.3))
		}
		box.Border(t.HL, border).Transition(theme.ColorsTransition(200 * time.Millisecond))
		if p.Unit != "" {
			components.Text(c, p.Unit, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
		}
	})
	return row
}

// stepButton 是 BetterNumberInput 的步进按钮：flex-1，text-brand-500 dark:text-brand-400，
// hover:bg-brand-200 hover:text-brand-800 dark:hover:bg-brand-700 dark:hover:text-white。原版 tabIndex=-1，不获得焦点，
// 所以用可点击的 Box 而不是按钮。
func stepButton(c *ui.Context, t *theme.Theme, key, icon string, size float32, disabled bool, onClick func()) {
	btn := ui.Box(c.Key(key)).Grow(1).Basis(0).Center()
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() && !disabled {
		btn.Background(t.Pick(theme.Brand200, theme.Brand700))
		color = t.Pick(theme.Brand800, theme.White)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !disabled {
		btn.OnClick(onClick)
	}
	btn.Children(func() { components.Icon(c, icon, size, color) })
}
