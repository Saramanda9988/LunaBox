// Package theme 是原版前端的设计令牌：uno.config.ts 的色板、Tailwind 的字号阶梯与过渡曲线，
// 以及 Chromium 把边框吸附到设备像素的规则。数值都用 tools/parity 从运行中的原版导出的计算样式校对过。
package theme

import "github.com/egoist/mygo/ui"

// uno.config.ts 的色板。
var (
	Brand50  = ui.Hex("#fdfdfd")
	Brand100 = ui.Hex("#f3f4f6")
	Brand150 = ui.Hex("#ecedf0")
	Brand200 = ui.Hex("#e5e7eb")
	Brand300 = ui.Hex("#d1d5db")
	Brand400 = ui.Hex("#9ca3af")
	Brand500 = ui.Hex("#6b7280")
	Brand600 = ui.Hex("#4b5563")
	Brand700 = ui.Hex("#44484e")
	Brand750 = ui.Hex("#303235")
	Brand800 = ui.Hex("#1c1e1f")
	Brand900 = ui.Hex("#121416")

	Neutral50  = ui.Hex("#f8fafc")
	Neutral100 = ui.Hex("#f1f5f9")
	Neutral200 = ui.Hex("#e2e8f0")
	Neutral300 = ui.Hex("#cbd5e1")
	Neutral400 = ui.Hex("#94a3b8")
	Neutral500 = ui.Hex("#64748b")
	Neutral600 = ui.Hex("#475569")
	Neutral700 = ui.Hex("#334155")
	Neutral800 = ui.Hex("#1e293b")
	Neutral900 = ui.Hex("#0f172a")

	Slate500  = ui.Hex("#64748b")
	Red500    = ui.Hex("#ef4444")
	Yellow500 = ui.Hex("#eab308")
	White     = ui.Hex("#ffffff")
	Black     = ui.Hex("#000000")
)

// uno.config.ts 的语义色（primary、success、warning、error、info）与 Tailwind 默认的 green 色阶。
// Tailwind 的 amber、red、emerald 与 warning、error、success 取值相同，直接用后者。
var (
	Primary50  = ui.Hex("#f5f3ff")
	Primary100 = ui.Hex("#ede9fe")
	Primary200 = ui.Hex("#ddd6fe")
	Primary300 = ui.Hex("#c4b5fd")
	Primary400 = ui.Hex("#a78bfa")
	Primary500 = ui.Hex("#7c6aef")
	Primary600 = ui.Hex("#6d5dd3")
	Primary700 = ui.Hex("#5b4cb8")
	Primary800 = ui.Hex("#4a3d96")
	Primary900 = ui.Hex("#2e2660")

	Success50  = ui.Hex("#ecfdf5")
	Success100 = ui.Hex("#d1fae5")
	Success200 = ui.Hex("#a7f3d0")
	Success300 = ui.Hex("#6ee7b7")
	Success400 = ui.Hex("#34d399")
	Success500 = ui.Hex("#10b981")
	Success600 = ui.Hex("#059669")
	Success700 = ui.Hex("#047857")
	Success800 = ui.Hex("#065f46")
	Success900 = ui.Hex("#064e3b")

	Warning50  = ui.Hex("#fffbeb")
	Warning100 = ui.Hex("#fef3c7")
	Warning200 = ui.Hex("#fde68a")
	Warning300 = ui.Hex("#fcd34d")
	Warning400 = ui.Hex("#fbbf24")
	Warning500 = ui.Hex("#f59e0b")
	Warning600 = ui.Hex("#d97706")
	Warning700 = ui.Hex("#b45309")
	Warning800 = ui.Hex("#92400e")
	Warning900 = ui.Hex("#78350f")

	Error50  = ui.Hex("#fef2f2")
	Error100 = ui.Hex("#fee2e2")
	Error200 = ui.Hex("#fecaca")
	Error300 = ui.Hex("#fca5a5")
	Error400 = ui.Hex("#f87171")
	Error500 = ui.Hex("#ef4444")
	Error600 = ui.Hex("#dc2626")
	Error700 = ui.Hex("#b91c1c")
	Error800 = ui.Hex("#991b1b")
	Error900 = ui.Hex("#7f1d1d")

	Info50  = ui.Hex("#f0f9ff")
	Info100 = ui.Hex("#e0f2fe")
	Info200 = ui.Hex("#bae6fd")
	Info300 = ui.Hex("#7dd3fc")
	Info400 = ui.Hex("#38bdf8")
	Info500 = ui.Hex("#0ea5e9")
	Info600 = ui.Hex("#0284c7")
	Info700 = ui.Hex("#0369a1")
	Info800 = ui.Hex("#075985")
	Info900 = ui.Hex("#0c181d")

	Green50  = ui.Hex("#f0fdf4")
	Green100 = ui.Hex("#dcfce7")
	Green200 = ui.Hex("#bbf7d0")
	Green300 = ui.Hex("#86efac")
	Green400 = ui.Hex("#4ade80")
	Green500 = ui.Hex("#22c55e")
	Green600 = ui.Hex("#16a34a")
	Green700 = ui.Hex("#15803d")
	Green800 = ui.Hex("#166534")
	Green900 = ui.Hex("#14532d")
)

// Tailwind 默认色板里导入弹窗用到的颜色（uno.config.ts 没有覆盖它们）。
var (
	Yellow50  = ui.Hex("#fefce8")
	Yellow100 = ui.Hex("#fef9c3")
	Yellow200 = ui.Hex("#fef08a")
	Yellow300 = ui.Hex("#fde047")
	Yellow400 = ui.Hex("#facc15")
	Yellow600 = ui.Hex("#ca8a04")
	Yellow700 = ui.Hex("#a16207")
	Yellow800 = ui.Hex("#854d0e")
	Yellow900 = ui.Hex("#713f12")
	Yellow950 = ui.Hex("#422006")

	Orange50  = ui.Hex("#fff7ed")
	Orange100 = ui.Hex("#ffedd5")
	Orange300 = ui.Hex("#fdba74")
	Orange400 = ui.Hex("#fb923c")
	Orange500 = ui.Hex("#f97316")
	Orange600 = ui.Hex("#ea580c")
	Orange700 = ui.Hex("#c2410c")
	Orange900 = ui.Hex("#7c2d12")

	Sky50  = ui.Hex("#f0f9ff")
	Sky100 = ui.Hex("#e0f2fe")
	Sky200 = ui.Hex("#bae6fd")
	Sky300 = ui.Hex("#7dd3fc")
	Sky400 = ui.Hex("#38bdf8")
	Sky500 = ui.Hex("#0ea5e9")
	Sky600 = ui.Hex("#0284c7")
	Sky700 = ui.Hex("#0369a1")
	Sky800 = ui.Hex("#075985")
	Sky900 = ui.Hex("#0c4a6e")
	Sky950 = ui.Hex("#082f49")

	Gray50  = ui.Hex("#f9fafb")
	Gray100 = ui.Hex("#f3f4f6")
	Gray300 = ui.Hex("#d1d5db")
	Gray400 = ui.Hex("#9ca3af")
	Gray600 = ui.Hex("#4b5563")
	Gray700 = ui.Hex("#374151")
	Gray900 = ui.Hex("#111827")

	Purple500 = ui.Hex("#a855f7")
	Purple600 = ui.Hex("#9333ea")
	Purple700 = ui.Hex("#7e22ce")
	Rose500   = ui.Hex("#f43f5e")
	Rose600   = ui.Hex("#e11d48")
	Rose700   = ui.Hex("#be123c")
	Slate300  = ui.Hex("#cbd5e1")
	Slate600  = ui.Hex("#475569")
	Slate700  = ui.Hex("#334155")
	Slate800  = ui.Hex("#1e293b")
)

// Palette 是组件里 `x dark:y` 两套取值在当前主题下的结果。
type Palette struct {
	Dark bool

	Text, TextStrong, TextMuted, TextFaint ui.Color // brand-900/100, 900/white, 500/400, 400/500

	AppBg                  ui.Color // main: bg-brand-100 dark:bg-brand-900
	TopbarBg, TopbarBorder ui.Color
	ControlIcon            ui.Color // 窗口按钮图标 text-brand-600 dark:text-brand-400
	ControlHover           ui.Color

	SidebarBg, SidebarBorder ui.Color
	NavText, NavHover        ui.Color // text-brand-700 dark:text-brand-300 / hover:bg-brand-100 dark:bg-brand-700
	NavActiveBg, NavActive   ui.Color

	InputBg, InputBorder, InputFocus, InputIcon, Placeholder ui.Color

	ToolBtnBg, ToolBtnBorder, ToolBtnHover, ToolBtnIcon, ToolBtnText ui.Color // glass-btn-neutral
	ToolBtnActiveBg, ToolBtnActiveBorder, ToolBtnActiveText          ui.Color

	PrimaryBtn, PrimaryBtnHover ui.Color

	CardBg, CardBorder, CardBorderHover, CoverBg ui.Color

	ScrollThumb, ScrollThumbHover ui.Color

	PanelBg, PanelBorder, Divider ui.Color // 下拉菜单、抽屉
	MenuHover, MenuText           ui.Color
	SectionLabel                  ui.Color // text-sm font-medium text-brand-700 dark:text-brand-300

	ChipBg, ChipText, ChipHover, ChipActiveBg, ChipActiveText, ChipRing ui.Color

	SelectBg, SelectBorder, SelectText ui.Color
	ListFocus, ListSelected            ui.Color
	SwitchOff                          ui.Color

	TooltipBg, TooltipText, TooltipBorder ui.Color
}

// NewPalette 返回浅色或深色主题的色板。
func NewPalette(dark bool) Palette {
	if dark {
		return Palette{
			Dark: true,
			Text: Brand100, TextStrong: White, TextMuted: Brand400, TextFaint: Brand500,
			AppBg:    Brand900,
			TopbarBg: Brand800, TopbarBorder: Brand700.Alpha(0.5),
			ControlIcon: Brand400, ControlHover: Brand700,
			SidebarBg: Brand800, SidebarBorder: Brand700,
			NavText: Brand300, NavHover: Brand700, NavActiveBg: Brand700, NavActive: Brand100,
			InputBg: Brand900, InputBorder: Brand700, InputFocus: Neutral500, InputIcon: Brand400, Placeholder: Brand400,
			ToolBtnBg: Brand700, ToolBtnBorder: Brand700, ToolBtnHover: Brand600, ToolBtnIcon: Brand300, ToolBtnText: Brand300,
			ToolBtnActiveBg: Brand600, ToolBtnActiveBorder: Brand500, ToolBtnActiveText: White,
			PrimaryBtn: Neutral600, PrimaryBtnHover: Neutral700,
			CardBg: Brand800, CardBorder: Brand700, CardBorderHover: Brand600, CoverBg: Brand700,
			ScrollThumb: Brand600, ScrollThumbHover: Brand500,
			PanelBg: Brand800, PanelBorder: Brand700, Divider: Brand700, MenuHover: Brand700, MenuText: Brand200,
			SectionLabel: Brand300,
			ChipBg:       Brand900.Alpha(0.5), ChipText: Brand400, ChipHover: Brand700.Alpha(0.7),
			ChipActiveBg: Brand700, ChipActiveText: Brand200, ChipRing: Brand500,
			SelectBg: Brand700, SelectBorder: Brand600, SelectText: White,
			ListFocus: Brand700, ListSelected: Brand750,
			SwitchOff: Brand750,
			TooltipBg: Brand100, TooltipText: Brand900, TooltipBorder: Brand200,
		}
	}
	return Palette{
		Text: Brand900, TextStrong: Brand900, TextMuted: Brand500, TextFaint: Brand400,
		AppBg:    Brand100,
		TopbarBg: Brand50, TopbarBorder: Brand200.Alpha(0.5),
		ControlIcon: Brand600, ControlHover: Brand200,
		SidebarBg: White, SidebarBorder: Brand200,
		NavText: Brand700, NavHover: Brand100, NavActiveBg: Brand200, NavActive: Brand900,
		InputBg: White, InputBorder: Brand300, InputFocus: Neutral600, InputIcon: Brand700, Placeholder: Brand700,
		ToolBtnBg: Brand150, ToolBtnBorder: Brand200, ToolBtnHover: Brand200, ToolBtnIcon: Brand600, ToolBtnText: Brand700,
		ToolBtnActiveBg: Brand200, ToolBtnActiveBorder: Brand300, ToolBtnActiveText: Brand900,
		PrimaryBtn: Neutral600, PrimaryBtnHover: Neutral700,
		CardBg: White, CardBorder: Brand100, CardBorderHover: Brand300, CoverBg: Brand200,
		ScrollThumb: ui.Hex("#cccccc"), ScrollThumbHover: ui.Hex("#aaaaaa"),
		PanelBg: White, PanelBorder: Brand200, Divider: Brand200, MenuHover: Brand100, MenuText: Brand700,
		SectionLabel: Brand700,
		ChipBg:       Brand50, ChipText: Brand500, ChipHover: Brand100,
		ChipActiveBg: Brand100, ChipActiveText: Brand700, ChipRing: Brand300,
		SelectBg: White, SelectBorder: Brand300, SelectText: Brand900,
		ListFocus: Neutral100, ListSelected: Neutral50,
		SwitchOff: Brand300,
		TooltipBg: Brand900, TooltipText: White, TooltipBorder: White.Alpha(0.1),
	}
}

// Pick 返回 `light dark:dark` 在当前主题下的取值，用于只出现一次的颜色。
func (p *Palette) Pick(light, dark ui.Color) ui.Color {
	if p.Dark {
		return dark
	}
	return light
}
