package modal

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// defaultTimeZone 是检测不到系统时区时的默认选项（browserTimezone || "Asia/Shanghai"）。
const defaultTimeZone = "Asia/Shanghai"

// commonTimeZones 是 TimezoneSelectModal.tsx 的 COMMON_TIMEZONES（标签原版就是英文，不翻译）。
var commonTimeZones = []better.SelectOption{
	{Value: "Asia/Shanghai", Label: "China Standard Time (UTC+8)"},
	{Value: "Asia/Tokyo", Label: "Japan Standard Time (UTC+9)"},
	{Value: "Asia/Seoul", Label: "Korea Standard Time (UTC+9)"},
	{Value: "Asia/Singapore", Label: "Singapore Time (UTC+8)"},
	{Value: "Asia/Bangkok", Label: "Bangkok Time (UTC+7)"},
	{Value: "Asia/Dubai", Label: "Dubai Time (UTC+4)"},
	{Value: "Europe/London", Label: "London Time (UTC+0)"},
	{Value: "Europe/Paris", Label: "Paris Time (UTC+1)"},
	{Value: "Europe/Berlin", Label: "Berlin Time (UTC+1)"},
	{Value: "Europe/Moscow", Label: "Moscow Time (UTC+3)"},
	{Value: "America/New_York", Label: "New York Time (UTC-5)"},
	{Value: "America/Chicago", Label: "Chicago Time (UTC-6)"},
	{Value: "America/Denver", Label: "Denver Time (UTC-7)"},
	{Value: "America/Los_Angeles", Label: "Los Angeles Time (UTC-8)"},
	{Value: "America/Sao_Paulo", Label: "São Paulo Time (UTC-3)"},
	{Value: "Australia/Sydney", Label: "Sydney Time (UTC+10)"},
	{Value: "Pacific/Auckland", Label: "Auckland Time (UTC+12)"},
	{Value: "UTC", Label: "Coordinated Universal Time (UTC)"},
}

// TimezoneSelect 是 TimezoneSelectModal 的状态（selectedTimezone），由根布局持有。
type TimezoneSelect struct {
	selected string
}

// Selected 是当前选中的时区（测试用）。
func (m *TimezoneSelect) Selected() string { return m.selected }

// TimezoneSelectProps 是 TimezoneSelectModal 的属性。
type TimezoneSelectProps struct {
	Open      bool
	Detected  string // browserTimezone：系统的 IANA 时区，不知道时为空
	TopOffset float32
	OnConfirm func(timezone string)
}

// TimezoneSelectView 对应 components/modal/TimezoneSelectModal.tsx：bg-black/50 backdrop-blur-sm 的遮罩上 max-w-lg 的面板，
// 地球图标、标题与说明，检测到的时区与 BetterSelect，右下角确认按钮。没有关闭方式，只能选好后确认。
func TimezoneSelectView(c *ui.Context, t *theme.Theme, m *TimezoneSelect, p TimezoneSelectProps) {
	if !p.Open {
		return
	}
	if m.selected == "" {
		m.selected = p.Detected
		if m.selected == "" {
			m.selected = defaultTimeZone
		}
	}
	components.Modal(c, t, "timezone-select", components.ModalProps{Open: true, TopOffset: p.TopOffset, MaxWidth: 512}, func(panel ui.Element) {
		components.ModalPanel(t, panel).Children(func() {
			// flex items-start gap-4 mb-6
			ui.Row(c.Key("header")).AlignItems(ui.Start).Gap(16).Margin(0, 0, 24, 0).Children(func() {
				// p-2 rounded-full bg-primary-100 dark:bg-primary-900/30 的 i-mdi-earth text-2xl
				ui.Box(c).Padding(8).Radius(999).Shrink(0).Background(t.Pick(theme.Primary100, theme.Primary900.Alpha(0.3))).Children(func() {
					components.Icon(c, "earth", 24, t.Pick(theme.Primary600, theme.Primary400))
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.TextMargin(c, locales.TimezoneModalTitle(), theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 8, 0)
					// text-sm leading-relaxed
					components.Text(c, locales.TimezoneModalDesc(), theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
				})
			})
			ui.Column(c.Key("field")).FillWidth().Margin(0, 0, 24, 0).Children(func() {
				// block text-sm font-medium mb-2，检测到的时区是 font-semibold text-primary-600 dark:text-primary-400
				ui.Row(c.Key("detected")).Wrap().Gap(4).Margin(0, 0, 8, 0).Children(func() {
					components.Text(c, locales.TimezoneModalDetectedLabel(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
					detected := p.Detected
					if detected == "" {
						detected = locales.TimezoneModalUnknown()
					}
					components.Text(c, detected, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Primary600, theme.Primary400))
				})
				better.Select(c, t, "timezone", better.SelectProps{
					Value:       m.selected,
					Options:     commonTimeZones,
					Placeholder: locales.TimezoneModalPlaceholder(),
					OnChange:    func(v string) { m.selected = v },
				})
			})
			ui.Row(c.Key("actions")).Justify(ui.End).Gap(12).Children(func() {
				timezoneConfirmButton(c, t).OnClick(func() {
					if p.OnConfirm != nil {
						p.OnConfirm(m.selected)
					}
				})
			})
		})
	})
}

// timezoneConfirmButton：px-6 py-2.5 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg
// shadow-sm shadow-primary-200 dark:shadow-none。
func timezoneConfirmButton(c *ui.Context, t *theme.Theme) ui.Element {
	btn := ui.ButtonBase(c.Key("confirm")).Padding(10, 24).Radius(8).FocusRing(false)
	bg := theme.Primary600
	if btn.Hovered() {
		bg = theme.Primary700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !t.Dark {
		btn.Shadow(0, 1, 2, 0, theme.Primary200)
	}
	btn.Children(func() {
		components.Text(c, locales.TimezoneModalConfirmBtn(), theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}
