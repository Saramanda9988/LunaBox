package gamefield

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/tagname"
	"lunabox/desktop/ui/theme"
)

// TagsProps 是标签输入的属性。Input 是输入框里的文字，由调用方持有；输入改变时 OnInput 查询候选
// （env.SearchTags），结果放进 Suggestions。
type TagsProps struct {
	Tags        []string
	Input       *string
	Suggestions []string
	Translate   bool // enable_tag_translation：候选与胶囊显示界面语言的译名
	OnInput     func(query string)
	OnChange    func(tags []string)
}

// Tags 是标签输入：外观同 Aliases 的输入框，里面是用户标签的虚线胶囊与一个输入框；输入时在下方列出库里已有的
// 匹配标签（同筛选面板的标签筛选），点击候选添加它，Enter 添加输入的文字（新标签），输入为空时退格删除最后一个。
func Tags(c *ui.Context, t *theme.Theme, key string, p TagsProps) {
	bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600))
	box := ui.Row(c.Key(key)).FillWidth().MinHeight(40).Wrap().GapX(6).GapY(6).AlignItems(ui.Center).
		Padding(5, 10).Radius(6).Background(bg).Border(t.HL, border)
	add := func(name string) {
		name = strings.TrimSpace(name)
		*p.Input = ""
		if p.OnInput != nil {
			p.OnInput("")
		}
		if name == "" || slices.ContainsFunc(p.Tags, func(s string) bool { return strings.EqualFold(s, name) }) {
			return
		}
		p.OnChange(append(slices.Clone(p.Tags), name))
	}
	box.Children(func() {
		components.Icon(c, "tag-outline", 16, t.Pick(theme.Brand400, theme.Brand500))
		for i, tag := range p.Tags {
			name := tagname.Display(tag, p.Translate)
			aliasChip(c.Key(i), t, name, locales.NativeDeleteTag(), func() {
				p.OnChange(slices.Delete(slices.Clone(p.Tags), i, i+1))
			})
		}
		in := ui.TextInputBase(c.Key("tag-input"), p.Input).Grow(1).Basis(96).MinWidth(96).Padding(6, 4).
			FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
			Placeholder(locales.TagsInputPlaceholder()).PlaceholderColor(t.Pick(theme.Brand400, theme.Brand500)).
			Label(locales.TagsInputPlaceholder())
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		if in.Changed() && p.OnInput != nil {
			p.OnInput(*p.Input)
		}
		if in.Submitted() {
			add(*p.Input)
		}
		// 输入框为空时的退格删除最后一个标签（同 Aliases：看上一次 keyup 时是否已经为空）。
		lastKeyUp := ui.Local(in, "last-keyup-input", func() string { return "" })
		in.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyUp {
				return false
			}
			if ev.Key == ui.KeyBackspace && *p.Input == "" && *lastKeyUp == "" && len(p.Tags) > 0 {
				p.OnChange(slices.Clone(p.Tags[:len(p.Tags)-1]))
			}
			*lastKeyUp = *p.Input
			return false
		})
	})
	if box.FocusWithin() {
		box.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	if strings.TrimSpace(*p.Input) != "" && len(p.Suggestions) > 0 {
		tagSuggestions(c, t, box, p, add)
	}
}

// tagSuggestions 是输入框下方的候选（筛选面板 tagSuggestionList 的样式，按主题配色）：max-h-36 的滚动列表。
func tagSuggestions(c *ui.Context, t *theme.Theme, field ui.Element, p TagsProps, add func(string)) {
	components.Overlay(c, t, func() {
		list := ui.Column(c.Key("tag-suggestions")).AttachTo(field, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(4).
			Width(field.Bounds().W).MaxHeight(176).Padding(4).Radius(8).
			Background(t.PanelBg).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		list.Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				for _, s := range p.Suggestions {
					name := tagname.Display(s, p.Translate)
					item := ui.ButtonBase(c.Key(s)).FillWidth().Padding(6, 10).Radius(6).Justify(ui.Start).FocusRing(false).Label(name)
					if item.Hovered() {
						item.Background(t.Pick(theme.Brand100, theme.Brand700))
					}
					item.Transition(theme.ColorsTransition(150 * time.Millisecond))
					item.OnClick(func() { add(s) }).Children(func() {
						components.Text(c, name, theme.TextSM, theme.WeightNormal, t.TextStrong).SingleLine()
					})
				}
			})
		})
	})
}
