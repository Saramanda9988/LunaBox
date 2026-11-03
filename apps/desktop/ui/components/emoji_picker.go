package components

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// emoji-picker-element 的分组（allGroups.slice(1)）：分组号、导航图标、名称。
var emojiGroups = []struct {
	id          int
	emoji, name string
}{
	{0, "😀", "Smileys and emoticons"},
	{1, "👋", "People and body"},
	{3, "🐱", "Animals and nature"},
	{4, "🍎", "Food and drink"},
	{5, "🏠️", "Travel and places"},
	{6, "⚽", "Activities"},
	{7, "📝", "Objects"},
	{8, "⛔️", "Symbols"},
	{9, "🏁", "Flags"},
}

// emojiFavorites 是 MOST_COMMONLY_USED_EMOJI：底部常用栏（原版之后按使用次数排序，原型不记录次数）。
var emojiFavorites = []string{"😊", "😒", "❤️", "👍️", "😍", "😂", "😭", "☺️", "😔", "😩", "😏", "💕", "🙌", "😘"}

// 选择器的尺寸：表情 22px、格子 38px（与 emoji-picker-element 的 --emoji-size、--emoji-padding 相同）。
// 列数等于分组数，分组导航、表情网格与常用表情三行逐列对齐，面板宽度按内容收紧。
const (
	emojiSize    = 22
	emojiCell    = emojiSize + 2*8
	emojiColumns = 9 // len(emojiGroups)
	emojiPanelH  = 264
)

// EmojiPickerProps 是 EmojiPickerPopover 的属性。
type EmojiPickerProps struct {
	Value        string
	CanEdit      bool
	Compact      bool   // h-11 w-11（否则 h-12 w-12 且右侧 mr-4）
	FallbackIcon string // 没有表情时的 i-mdi-* 图标
	System       bool   // variant="system"：红色底
	OnChange     func(emoji string)
}

type emojiPickerState struct {
	open   bool
	group  int // emojiGroups 的下标
	query  string
	tone   int // 0 默认，1–5 浅 → 深
	toneUI bool
	scroll ScrollBar
}

// EmojiPicker 对应 EmojiPickerPopover：显示当前表情（或备用图标）的方块按钮，点击后在下方弹出选择面板。
// 面板不照搬 emoji-picker-element：标题行右侧是“移除”按钮，下面是搜索框与肤色、与表情列对齐的分组导航、
// 表情网格和常用表情。
func EmojiPicker(c *ui.Context, t *theme.Theme, key string, p EmojiPickerProps) {
	size := float32(48)
	if p.Compact {
		size = 44
	}
	btn := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(8).Center().Shrink(0).FocusRing(false).Label(locales.CategoriesEmojiPickerTitle())
	if !p.Compact {
		btn.Margin(0, 16, 0, 0)
	}
	st := ui.Local(btn, "emoji-picker", func() *emojiPickerState { return &emojiPickerState{} })
	s := *st
	bg, fg := t.Pick(theme.Neutral100, ui.Hex("#0f172a").Alpha(0.3)), t.Pick(theme.Neutral600, theme.Neutral400)
	if p.System {
		bg, fg = t.Pick(ui.Hex("#fee2e2"), ui.Hex("#7f1d1d").Alpha(0.3)), t.Pick(theme.Error600, theme.Error400)
	}
	if p.CanEdit && btn.Hovered() {
		bg = t.Pick(ui.Hex("#e2e8f0"), ui.Hex("#1e293b").Alpha(0.5))
	}
	btn.Background(bg)
	if p.CanEdit {
		btn.OnClick(func() { s.open = !s.open })
	} else {
		btn.Cursor(ui.CursorDefault)
		s.open = false
	}
	btn.Children(func() {
		if v := strings.TrimSpace(p.Value); v != "" {
			EmojiText(c, v, 24)
		} else {
			Icon(c, p.FallbackIcon, 24, fg)
		}
	})
	choose := func(e string) {
		s.open = false
		if p.OnChange != nil {
			p.OnChange(e)
		}
	}
	ui.PopoverBase(c, btn, &s.open, func(panel ui.Element) {
		panel.AttachTo(btn, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(8).Padding(12).Radius(12).
			Background(t.PanelBg).Border(t.HL, t.PanelBorder).
			Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			emojiPanel(c, t, s, strings.TrimSpace(p.Value) != "", choose)
		})
	})
}

// emojiPanel 是弹出面板的内容：宽度是九列表情加右侧的滚动条槽，各行右边都留出同样的槽，与网格对齐。
func emojiPanel(c *ui.Context, t *theme.Theme, s *emojiPickerState, hasValue bool, choose func(string)) {
	text := t.Pick(theme.Brand900, theme.Brand100)
	muted := t.Pick(theme.Brand500, theme.Brand400)
	hover := t.Pick(theme.Brand100, theme.Brand700)
	gridW := float32(emojiColumns * emojiCell)
	gutter := t.Gutter()
	searching := len([]rune(strings.TrimSpace(s.query))) >= 2 // MIN_SEARCH_TEXT_LENGTH

	ui.Column(c.Key("picker")).Width(gridW + gutter).Children(func() {
		// 标题与“移除”（没有图标时禁用）
		ui.Row(c).Padding(0, gutter, 0, 4).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			Text(c, locales.CategoriesEmojiPickerTitle(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
			clear := ui.ButtonBase(c.Key("clear")).Height(28).Padding(0, 8).Gap(4).Radius(6).AlignItems(ui.Center).FocusRing(false).
				Label(locales.CategoriesEmojiPickerClear())
			color := muted
			if !hasValue {
				Disable(clear, 0.45)
			} else if clear.Hovered() {
				clear.Background(hover)
				color = t.Pick(theme.Error600, theme.Error400)
			}
			clear.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { choose("") }).Children(func() {
				Icon(c, "close-circle-outline", 14, color)
				Text(c, locales.CategoriesEmojiPickerClear(), theme.TextXS, theme.WeightMedium, color).NoWrap()
			})
		})

		// 搜索框（左侧放大镜，有内容时右侧清除）与肤色
		ui.Row(c).Margin(8, 0, 0, 0).Padding(0, gutter, 0, 0).Gap(6).AlignItems(ui.Center).Children(func() {
			box := ui.Row(c.Key("search-box")).Grow(1).Basis(0).MinWidth(0).Height(34).Radius(8).AlignItems(ui.Center).
				Background(t.Pick(theme.White, theme.Brand900)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand700))
			box.Children(func() {
				Icon(c, "magnify", 16, muted).Margin(0, 0, 0, 10)
				in := ui.TextInputBase(c.Key("search"), &s.query).Grow(1).Basis(0).MinWidth(0).Padding(0, 8).
					FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(text).Placeholder("Search").Label("Search")
				if theme.UIFont != "" {
					in.Font(theme.UIFont)
				}
				if in.Focused() {
					box.BorderColor(t.InputFocus)
				}
				if s.query != "" {
					x := ui.ButtonBase(c.Key("clear-search")).Size(26, 26).Margin(0, 4, 0, 0).Radius(6).Center().FocusRing(false).
						Label(locales.FilterBarClearSearch())
					if x.Hovered() {
						x.Background(hover)
					}
					x.OnClick(func() { s.query = "" }).Children(func() { Icon(c, "close", 14, muted) })
				}
			})
			skinToneButton(c, t, s, hover)
		})

		// 分组导航：每个按钮占一列，下面 2px 的指示条（搜索时淡出）
		ui.Column(c.Key("nav")).Margin(8, 0, 0, 0).Width(gridW).Children(func() {
			ui.Row(c).Children(func() {
				for i, g := range emojiGroups {
					b := ui.ButtonBase(c.Key("nav-"+g.name)).Size(emojiCell, emojiCell).Radius(8).Center().FocusRing(false).Label(g.name)
					hovered := b.Hovered()
					if hovered {
						b.Background(hover)
					}
					lit := b.AnimateWith("lit", theme.Bool(hovered || i == s.group && !searching), 150*time.Millisecond, theme.EaseTailwind)
					b.OnClick(func() { s.group, s.query, s.scroll = i, "", ScrollBar{} }).Children(func() {
						EmojiText(c, g.emoji, 20).Opacity(0.6 + 0.4*lit)
					})
				}
			})
			ind := ui.Box(c.Key("indicator")).Height(2).FillWidth().Margin(2, 0, 0, 0)
			x := ind.AnimateWith("x", float32(s.group), 250*time.Millisecond, theme.EaseCSS)
			opacity := ind.AnimateWith("opacity", theme.Bool(!searching), 100*time.Millisecond, ui.Linear)
			color := t.Pick(theme.Neutral500, theme.Neutral400)
			ind.Draw(func(pt *ui.Painter, r ui.Rect) {
				w := r.W / float32(len(emojiGroups))
				pt.Fill(ui.Rect{X: r.X + x*w + 8, Y: r.Y, W: w - 16, H: r.H}, color.Alpha(float32(color.A)/255*opacity), 1)
			})
		})
		ui.Box(c).Height(t.HL).Background(t.Divider)

		// 当前分组或搜索结果
		ScrollArea(c, t, "tabpanel", &s.scroll, func(w ui.Element) { w.Height(emojiPanelH) }, func(sc ui.Element) {
			sc.Padding(4, gutter, 4, 0).Children(func() {
				var list []assets.Emoji
				label := emojiGroups[s.group].name
				if searching {
					list, label = searchEmoji(s.query), ""
				} else {
					for _, e := range assets.Emojis() {
						if e.Group == emojiGroups[s.group].id {
							list = append(list, e)
						}
					}
				}
				if label != "" {
					Text(c, label, theme.TextXS, theme.WeightMedium, muted).Margin(4, 8)
				} else if len(list) == 0 {
					ui.Column(c).Height(emojiPanelH - 8).Center().Gap(8).Children(func() {
						Icon(c, "magnify", 32, muted)
						Text(c, "No results", theme.TextSM, theme.WeightNormal, muted)
					})
				}
				emojiGrid(c, s, list, hover, choose)
			})
		})

		// 常用表情
		ui.Row(c.Key("favorites")).Padding(4, gutter, 0, 0).BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Divider).Children(func() {
			for _, e := range emojiFavorites[:emojiColumns] {
				emojiButton(c, e, hover, func() { choose(e) })
			}
		})
	})
}

// emojiGrid：emojiColumns 列 38px 的格子。
func emojiGrid(c *ui.Context, s *emojiPickerState, list []assets.Emoji, hover ui.Color, choose func(string)) {
	ui.Grid(c.Key("grid")).Columns(emojiColumns).Children(func() {
		for _, e := range list {
			u := e.Unicode
			if s.tone > 0 && len(e.Skins) >= s.tone {
				u = e.Skins[s.tone-1]
			}
			emojiButton(c, u, hover, func() { choose(u) }).JustifySelf(ui.Center)
		}
	})
}

// emojiButton：38px 的圆角格子，悬停时加底色。
func emojiButton(c *ui.Context, e string, hover ui.Color, onClick func()) ui.Element {
	b := ui.ButtonBase(c.Key(e)).Size(emojiCell, emojiCell).Radius(8).Center().FocusRing(false).Label(e)
	if b.Hovered() {
		b.Background(hover)
	}
	b.OnClick(onClick).Children(func() { EmojiText(c, e, emojiSize) })
	return b
}

// skinToneButton 显示当前肤色的 🖐️，点击在下方展开六种肤色（与按钮右对齐，不超出面板）。
func skinToneButton(c *ui.Context, t *theme.Theme, s *emojiPickerState, hover ui.Color) {
	tones := []string{"🖐️", "🖐🏻", "🖐🏼", "🖐🏽", "🖐🏾", "🖐🏿"}
	names := []string{"Default", "Light", "Medium-Light", "Medium", "Medium-Dark", "Dark"}
	b := ui.ButtonBase(c.Key("skintone")).Size(34, 34).Radius(8).Center().Shrink(0).FocusRing(false).
		Label("Choose a skin tone (currently " + names[s.tone] + ")")
	if b.Hovered() || s.toneUI {
		b.Background(hover)
	}
	b.OnClick(func() { s.toneUI = !s.toneUI }).Children(func() { EmojiText(c, tones[s.tone], 20) })
	ui.PopoverBase(c, b, &s.toneUI, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorBottomRight, ui.AnchorTopRight).Top(4).Padding(4).Radius(10).Background(t.PanelBg).
			Border(t.HL, t.PanelBorder).Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			for i, tone := range tones {
				o := ui.ButtonBase(c.Key("tone-"+names[i])).Size(34, 34).Radius(8).Center().FocusRing(false).Label(names[i])
				if o.Hovered() || i == s.tone {
					o.Background(hover)
				}
				o.OnClick(func() { s.tone, s.toneUI = i, false }).Children(func() { EmojiText(c, tone, 20) })
			}
		})
	})
}

// searchEmoji 对应 getEmojiBySearchQuery：查询拆成词，每个词都要是名称或标签里某个词的前缀。
func searchEmoji(query string) []assets.Emoji {
	words := strings.Fields(strings.ToLower(query))
	var out []assets.Emoji
	for _, e := range assets.Emojis() {
		tokens := append(strings.Fields(strings.ToLower(e.Annotation)), e.Tags...)
		match := true
		for _, w := range words {
			found := false
			for _, tk := range tokens {
				if strings.HasPrefix(tk, w) {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			out = append(out, e)
		}
	}
	return out
}

// EmojiText 用彩色表情字体显示表情（--emoji-font-family 在 Windows 上是 Segoe UI Emoji）。
func EmojiText(c *ui.Context, s string, size float32) ui.Element {
	return ui.Text(c, s).Font("Segoe UI Emoji").FontSize(size).FixedLineHeight(size).NoWrap()
}
