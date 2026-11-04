package gamefield

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// AliasesProps 是别名输入的属性：Adding、Draft 是“添加中”与输入框里的草稿，由调用方持有；OnChange 收到新的别名列表。
type AliasesProps struct {
	Aliases  []string
	Adding   *bool
	Draft    *string
	OnChange func([]string)
}

// Aliases 对应“游戏别名”的输入框：glass-input flex min-h-10 w-full flex-wrap items-center gap-1.5 rounded-md border
// border-brand-300 bg-white px-2.5 py-[5px] focus-within:ring-2 focus-within:ring-neutral-500 dark:border-brand-600
// dark:bg-brand-700；别名胶囊、添加中的输入框或虚线的“+”按钮。标题与说明由调用方构建。
func Aliases(c *ui.Context, t *theme.Theme, key string, p AliasesProps) {
	bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600))
	box := ui.Row(c.Key(key)).FillWidth().MinHeight(40).Wrap().GapX(6).GapY(6).AlignItems(ui.Center).
		Padding(5, 10).Radius(6).Background(bg).Border(t.HL, border)
	box.Children(func() {
		for i, alias := range p.Aliases {
			aliasChip(c.Key(i), t, alias, locales.GameEditRemoveAlias(alias), func() { removeAlias(p, i) })
		}
		if *p.Adding {
			aliasInput(c, t, p)
		} else {
			addAliasButton(c, t, func() { *p.Adding = true })
		}
	})
	if box.FocusWithin() {
		box.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
}

// aliasChip（标签输入里的胶囊也用它）：inline-flex max-w-full items-center gap-1 rounded-full border border-dashed border-brand-400/80 bg-white/70
// py-1.5 pl-3 pr-2 text-xs text-brand-700 dark:border-brand-500/70 dark:bg-brand-900/45 dark:text-brand-200；
// 删除按钮 h-4 w-4 rounded-full text-brand-400 hover:bg-red-50 hover:text-red-500 dark:text-brand-500
// dark:hover:bg-red-500/12 dark:hover:text-red-300。
func aliasChip(c *ui.Context, t *theme.Theme, alias, removeLabel string, onRemove func()) {
	chip := ui.Row(c).MaxWidthPercent(100).Clip().Gap(4).Padding(6, 8, 6, 12).Radius(999).AlignItems(ui.Center).
		Border(t.HL, t.Pick(theme.Brand400.Alpha(0.8), theme.Brand500.Alpha(0.7))).BorderStyle(ui.BorderDashed).
		Background(t.Pick(theme.White.Alpha(0.7), theme.Brand900.Alpha(0.45))).
		Transition(theme.ColorsTransition(200 * time.Millisecond))
	chip.Children(func() {
		// min-w-0 truncate px-0.5：SingleLine 在 150% 下常因差一点点宽度把短别名也显示成省略号，
		// 所以用不换行的文字，过长的别名由胶囊（max-w-full）裁掉。
		components.TextMargin(c, alias, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand200), 0, 2, 0, 2).
			NoWrap().MinWidth(0).Shrink(1)
		label := removeLabel
		btn := ui.ButtonBase(c.Key("remove")).Size(16, 16).Radius(999).Center().Shrink(0).FocusRing(false).Label(label)
		color := t.Pick(theme.Brand400, theme.Brand500)
		if btn.Hovered() {
			btn.Background(t.Pick(ui.Hex("#fef2f2"), theme.Red500.Alpha(0.12)))
			color = t.Pick(theme.Red500, ui.Hex("#fca5a5"))
		}
		btn.Transition(theme.ColorsTransition(200 * time.Millisecond))
		btn.OnClick(onRemove).Children(func() { components.Icon(c, "close", 12, color) })
	})
}

// addAliasButton：inline-flex h-7 w-7 rounded-full border border-dashed border-brand-300 text-brand-500
// hover:border-brand-500 hover:bg-brand-50 hover:text-brand-700 dark:border-brand-600 dark:text-brand-400
// dark:hover:border-brand-400 dark:hover:bg-brand-800/50 dark:hover:text-brand-200，i-mdi-plus text-sm。
func addAliasButton(c *ui.Context, t *theme.Theme, onClick func()) {
	btn := ui.ButtonBase(c.Key("add-alias")).Size(28, 28).Radius(999).Center().Shrink(0).FocusRing(false).Label(locales.GameEditAddAlias())
	border, color := t.Pick(theme.Brand300, theme.Brand600), t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		border, color = t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.Brand700, theme.Brand200)
		btn.Background(t.Pick(theme.Brand50, theme.Brand800.Alpha(0.5)))
	}
	btn.Border(t.HL, border).BorderStyle(ui.BorderDashed).Transition(theme.ColorsTransition(200 * time.Millisecond))
	btn.OnClick(onClick).Children(func() { components.Icon(c, "plus", 14, color) })
}

// aliasInput 是添加中的别名输入框（BetterInput variant="unstyled"）：w-40 focus:w-56 rounded-full border
// border-brand-300 bg-white/90 px-3 py-1.5 text-xs focus:border-brand-500 focus:bg-white focus:ring-2 focus:ring-brand-200
// dark:border-brand-600 dark:bg-brand-900/80 dark:focus:border-brand-400 dark:focus:bg-brand-900 dark:focus:ring-brand-700。
// Enter 添加，Esc 放弃，草稿为空时退格删除最后一个别名，失去焦点时添加。
func aliasInput(c *ui.Context, t *theme.Theme, p AliasesProps) {
	in := aliasInputBox(c, t, p)
	switch {
	case in.Submitted():
		addAlias(p)
	case in.Shortcut(0, ui.KeyEscape):
		*p.Draft, *p.Adding = "", false
	case Blurred(in):
		addAlias(p)
	}
}

// aliasInputBox 构建别名输入框本身。
func aliasInputBox(c *ui.Context, t *theme.Theme, p AliasesProps) ui.Element {
	in := ui.TextInputBase(c.Key("alias-input"), p.Draft).Padding(6, 12).Radius(999).AutoFocus().
		FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).TextColor(t.TextStrong).
		Placeholder(locales.GameEditAliasPlaceholder()).PlaceholderColor(t.Pick(theme.Brand400, theme.Brand500)).Label(locales.GameEditAliases())
	if theme.UIFont != "" {
		in.Font(theme.UIFont)
	}
	// 输入框自己处理退格的 keydown，只把 keyup 交给 HandleInput。上一次 keyup 时草稿已经为空、这次仍为空，
	// 说明这次退格没有删掉文字（原版 keydown 时 !aliasDraft），于是删除最后一个别名。
	lastKeyUp := ui.Local(in, "last-keyup-draft", func() string { return "" })
	// 原版组字时按下的 Enter 只选定候选，松开时再添加（submitAliasAfterComposition）。输入法的按键不交给应用，
	// 所以记下组字刚刚结束：紧接着松开的键是 Enter 时添加，是别的键（空格、数字选词，Esc 取消）时不添加。
	composing, composed := ui.Local(in, "composing", func() bool { return false }), ui.Local(in, "composed", func() bool { return false })
	if now := in.Composing(); now != *composing {
		*composing, *composed = now, !now
	}
	in.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyUp {
			return false
		}
		afterCompose := *composed
		*composed = false
		if ev.Key == ui.KeyEnter && afterCompose {
			addAlias(p)
			return true
		}
		if ev.Key == ui.KeyBackspace && *p.Draft == "" && *lastKeyUp == "" && len(p.Aliases) > 0 {
			removeAlias(p, len(p.Aliases)-1)
		}
		*lastKeyUp = *p.Draft
		return false
	})
	focused := in.Focused()
	border, bg := t.Pick(theme.Brand300, theme.Brand600), t.Pick(theme.White.Alpha(0.9), theme.Brand900.Alpha(0.8))
	if focused {
		border, bg = t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.White, theme.Brand900)
		in.Shadow(0, 0, 0, 2, t.Pick(theme.Brand200, theme.Brand700))
	}
	w := in.AnimateWith("width", 160+64*theme.Bool(focused), 200*time.Millisecond, theme.EaseTailwind)
	in.Width(w).Shrink(0).Border(t.HL, border).Background(bg).Transition(theme.ColorsTransition(200 * time.Millisecond))
	return in
}

// addAlias 对应 addAlias：去掉首尾空白，忽略大小写重复，然后退出添加状态。
func addAlias(p AliasesProps) {
	alias := strings.TrimSpace(*p.Draft)
	*p.Draft, *p.Adding = "", false
	if alias == "" {
		return
	}
	for _, a := range p.Aliases {
		if strings.EqualFold(a, alias) {
			return
		}
	}
	p.OnChange(append(slices.Clone(p.Aliases), alias))
}

// removeAlias 对应 removeAlias。
func removeAlias(p AliasesProps, i int) {
	if i < len(p.Aliases) {
		p.OnChange(slices.Delete(slices.Clone(p.Aliases), i, i+1))
	}
}
