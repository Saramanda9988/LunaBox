package gamefield

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/tagname"
	"lunabox/desktop/ui/theme"
)

// TagSelectProps 是游戏标签管理的属性（原版没有）：Selected 是选中的 TagGroup.Key，Adding、Draft 是“添加中”与
// 输入框里的草稿，都由调用方持有。
type TagSelectProps struct {
	Groups    []components.TagGroup
	Selected  map[string]bool
	Translate tagname.Mode
	Adding    *bool
	Draft     *string
	OnToggle  func(g components.TagGroup)
	OnRemove  func(g components.TagGroup)
	OnAdd     func(name string) // 提交了非空、不与已有标签重名（不区分大小写）的标签名
}

// TagSelect 是编辑页的游戏标签：外观同 Aliases 的输入框，里面是合并后的标签胶囊（同 aliasChip，带删除按钮），
// 末尾是添加中的输入框或虚线的“+”。点胶囊切换选中；选中只换边框与底色，尺寸不变，不会引起换行。
// 用户标签是虚线边框，自动获取的是实线。
func TagSelect(c *ui.Context, t *theme.Theme, key string, p TagSelectProps) {
	bg, border := t.FieldColors()
	box := ui.Row(c.Key(key)).FillWidth().MinHeight(40).Wrap().GapX(6).GapY(6).AlignItems(ui.Center).
		Padding(5, 10).Radius(6).Background(bg).Border(t.HL, border)
	box.Children(func() {
		for _, g := range p.Groups {
			selectableTagChip(c.Key(g.Key), t, g, p)
		}
		if *p.Adding {
			tagSelectInput(c, t, p)
		} else {
			addAliasButton(c, t, "add-tag", locales.TagsAdd(), func() { *p.Adding = true })
		}
	})
	// 只在输入新标签时显示聚焦描边：点胶囊选中也会让焦点落进框里，那时不需要。
	if *p.Adding && box.FocusWithin() {
		box.Shadow(0, 0, 0, 2, t.FieldFocus())
	}
}

// tagSelectInput 是添加中的标签输入框，样式同别名输入框：Enter 或失去焦点时添加，Esc 放弃。
func tagSelectInput(c *ui.Context, t *theme.Theme, p TagSelectProps) {
	in := ui.TextInputBase(c.Key("tag-input"), p.Draft).Padding(6, 12).Radius(999).AutoFocus().
		FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).TextColor(t.TextStrong).
		Placeholder(locales.TagsInputPlaceholder()).PlaceholderColor(t.FieldPlaceholder()).
		Label(locales.TagsInputPlaceholder())
	if theme.UIFont != "" {
		in.Font(theme.UIFont)
	}
	focused := in.Focused()
	border, bg := t.Pick(theme.Brand300, theme.Brand600), t.Pick(theme.White.Alpha(0.9), theme.Brand900.Alpha(0.8))
	if focused {
		border, bg = t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.White, theme.Brand900)
		in.Shadow(0, 0, 0, 2, t.Pick(theme.Brand200, theme.Brand700))
	}
	w := in.AnimateWith("width", 160+64*theme.Bool(focused), 200*time.Millisecond, theme.EaseTailwind)
	in.Width(w).Shrink(0).Border(t.HL, border).Background(bg).Transition(theme.ColorsTransition(200 * time.Millisecond))

	submit := func() {
		name := strings.TrimSpace(*p.Draft)
		*p.Draft, *p.Adding = "", false
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		for _, g := range p.Groups {
			if g.Key == key {
				return
			}
		}
		p.OnAdd(name)
	}
	switch {
	case in.Submitted():
		submit()
	case in.Shortcut(0, ui.KeyEscape):
		*p.Draft, *p.Adding = "", false
	case Blurred(in):
		submit()
	}
}

// selectableTagChip 与 aliasChip 同尺寸：py-1.5 pl-3 pr-2 rounded-full border text-xs，右侧 h-4 w-4 的删除按钮。
// 选中时是天蓝色边框与半透明的淡蓝底色。
func selectableTagChip(c *ui.Context, t *theme.Theme, g components.TagGroup, p TagSelectProps) {
	selected := p.Selected[g.Key]
	name := tagname.Display(g.Name, p.Translate)
	chip := ui.Row(c).MaxWidthPercent(100).Clip().Gap(4).Padding(6, 8, 6, 12).Radius(999).AlignItems(ui.Center).
		Transition(theme.ColorsTransition(200 * time.Millisecond))
	hovered := chip.Hovered()
	text := t.Pick(theme.Brand700, theme.Brand200)
	switch {
	case selected:
		chip.Border(t.HL, t.Pick(theme.Sky500, theme.Sky400)).
			Background(t.Pick(theme.Sky500.Alpha(0.12), theme.Sky400.Alpha(0.18)))
		text = t.Pick(theme.Sky700, theme.Sky200)
	case g.Scraped():
		border := t.Pick(theme.Brand200, theme.Brand600)
		if hovered {
			border = t.Pick(theme.Brand400, theme.Brand500)
		}
		chip.Border(t.HL, border).Background(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand800.Alpha(0.55)))
	default:
		border := t.Pick(theme.Brand400.Alpha(0.8), theme.Brand500.Alpha(0.7))
		if hovered {
			border = t.Pick(theme.Brand500, theme.Brand400)
		}
		chip.Border(t.HL, border).BorderStyle(ui.BorderDashed).Background(t.Pick(theme.White.Alpha(0.7), theme.Brand900.Alpha(0.45)))
	}
	chip.Children(func() {
		label := ui.ButtonBase(c.Key("toggle")).MinWidth(0).Shrink(1).FocusRing(false).Cursor(ui.CursorPointer).Label(name)
		label.OnClick(func() { p.OnToggle(g) }).Children(func() {
			components.TextMargin(c, name, theme.TextXS, theme.WeightNormal, text, 0, 2, 0, 2).NoWrap().MinWidth(0).Shrink(1)
		})
		btn := ui.ButtonBase(c.Key("remove")).Size(16, 16).Radius(999).Center().Shrink(0).FocusRing(false).Label(locales.NativeDeleteTag())
		color := t.Pick(theme.Brand400, theme.Brand500)
		if selected {
			color = t.Pick(theme.Sky600, theme.Sky300)
		}
		if btn.Hovered() {
			btn.Background(t.Pick(ui.Hex("#fef2f2"), theme.Red500.Alpha(0.12)))
			color = t.Pick(theme.Red500, ui.Hex("#fca5a5"))
		}
		btn.Transition(theme.ColorsTransition(200 * time.Millisecond))
		btn.OnClick(func() { p.OnRemove(g) }).Children(func() { components.Icon(c, "close", 12, color) })
	})
}
