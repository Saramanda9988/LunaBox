package picker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/theme"
)

// TimeWheelInputProps 是 BetterTimeWheelInput 的属性：24 小时制的 "HH:mm"。
type TimeWheelInputProps struct {
	Value       string
	OnChange    func(value string)
	ActionLabel string // 时钟按钮的无障碍名称，也是悬停提示
	Disabled    bool
}

var clockPattern = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})$`)

// ParseClockMinutes 同 parseClockMinutes："HH:mm" 转成一天中的分钟数，格式不对时是 0。
func ParseClockMinutes(value string) int {
	m := clockPattern.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	mm, _ := strconv.Atoi(m[2])
	if h > 23 || mm > 59 {
		return 0
	}
	return h*60 + mm
}

// FormatClockMinutes 同 formatClockMinutes：分钟数（按一天取模）转成 "HH:mm"。
func FormatClockMinutes(v int) string {
	const day = 24 * 60
	v = (v%day + day) % day
	return fmt.Sprintf("%02d:%02d", v/60, v%60)
}

// TimeWheelInput 对应 BetterTimeWheelInput：只读的 BetterActionInput（font-medium tabular-nums，右侧 i-mdi-clock-outline）
// 作为 Popover 的按钮，点击后在右下方（anchor="bottom end"，--anchor-gap:8px）弹出 w-72 rounded-xl border border-brand-200
// bg-white p-3 shadow-xl dark:border-brand-700 dark:bg-brand-800 的面板，里面是 BetterTimeWheelPicker。
// 宽度填满父元素（原版由 className 决定，用到的地方都是 w-full）。返回按钮元素。
func TimeWheelInput(c *ui.Context, t *theme.Theme, key string, p TimeWheelInputProps) ui.Element {
	minutes := ParseClockMinutes(p.Value)
	btn := ui.ButtonBase(c.Key(key)).FillWidth().MinWidth(0).Gap(4).Padding(0, 4, 0, 0).AlignItems(ui.Center).
		Justify(ui.Start).Radius(6).FocusRing(false).Background(t.Pick(theme.White, theme.Brand700)).
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	if p.ActionLabel != "" {
		btn.Label(p.ActionLabel)
	}
	open := ui.Local(btn, "open", func() bool { return false })
	if p.Disabled {
		components.Disable(btn, 0.6)
		*open = false
	} else {
		btn.OnClick(func() { *open = !*open })
	}
	if btn.Focused() {
		btn.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	var clock ui.Element
	btn.Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).Children(func() {
			components.Text(c, FormatClockMinutes(minutes), theme.TextBase, theme.WeightMedium, t.Pick(theme.Brand900, theme.White)).SingleLine()
		})
		clock = ui.Row(c.Key("clock")).Size(40, 40).Center().Shrink(0)
		color := t.Pick(theme.Brand500, theme.Brand400)
		if clock.Hovered() && !p.Disabled {
			color = t.Pick(theme.Brand900, theme.Brand100)
		}
		color = clock.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
		clock.Children(func() { components.Icon(c, "clock-outline", 20, color) })
	})
	better.Tooltip(c, t, clock, p.ActionLabel)
	ui.PopoverBase(c, btn, open, func(panel ui.Element) {
		winW, _ := c.Size()
		panel.AttachTo(btn, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(8, 0, 0, 0).
			Width(min(288, winW-32)).Padding(12).Radius(12).Background(t.Pick(theme.White, theme.Brand800)).
			Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1)).
			OnShortcut(0, ui.KeyEscape, func() { *open = false })
		panel.Children(func() {
			TimeWheelPicker(c, t, "wheel", TimeWheelProps{
				ValueMinutes: minutes,
				Disabled:     p.Disabled,
				OnChange: func(v int) {
					if p.OnChange != nil {
						p.OnChange(FormatClockMinutes(v))
					}
				},
			})
		})
	})
	return btn
}
