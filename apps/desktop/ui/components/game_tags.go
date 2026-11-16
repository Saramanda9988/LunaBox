package components

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/tagname"
	"lunabox/desktop/ui/theme"
)

// TagEditor 是“添加标签”输入框的状态（isAdding、inputValue），由调用方持有：添加成功后调用方清空它。
type TagEditor struct {
	Adding bool
	Input  string
}

// TagGroup 是详情页合并显示的一个标签（原版没有）：同一个游戏里名称相同（不区分大小写）的标签，
// 通常是几个来源都给了这个标签。
type TagGroup struct {
	Key  string           // 去掉首尾空白的小写名称：元素的 key 与选中状态
	Name string           // 显示与筛选用的名称：组里第一个（权重最高的）标签的写法
	Tags []models.GameTag // 合并进来的标签，删除时一起删
}

// GroupTags 按名称合并标签（去掉首尾空白、不区分大小写），顺序是每组第一次出现的位置。
func GroupTags(tags []models.GameTag) []TagGroup {
	var groups []TagGroup
	index := map[string]int{}
	for _, tag := range tags {
		key := strings.ToLower(strings.TrimSpace(tag.Name))
		if i, ok := index[key]; ok {
			groups[i].Tags = append(groups[i].Tags, tag)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, TagGroup{Key: key, Name: tag.Name, Tags: []models.GameTag{tag}})
	}
	return groups
}

// Scraped 报告组里有没有自动获取的标签（删除后重新获取元数据时可能再出现）；没有时整组都是用户标签。
func (g TagGroup) Scraped() bool {
	return slices.ContainsFunc(g.Tags, func(t models.GameTag) bool { return t.Source != "user" })
}

// Spoiler 报告整组是不是剧透：每个来源都标了剧透才算。
func (g TagGroup) Spoiler() bool {
	return !slices.ContainsFunc(g.Tags, func(t models.GameTag) bool { return !t.IsSpoiler })
}

// Sources 是组里标签的来源，按出现顺序去重。
func (g TagGroup) Sources() []string {
	var out []string
	for _, t := range g.Tags {
		if !slices.Contains(out, t.Source) {
			out = append(out, t.Source)
		}
	}
	return out
}

// GameTagsProps 是 GameTags 的属性。增删标签与点击跳转都交给调用方。
type GameTagsProps struct {
	Groups    []TagGroup   // GroupTags 合并后的标签；调用方在标签变化时算一次
	Translate tagname.Mode // 标签名的显示方式（enable_tag_translation、show_tag_original_name）
	Editor    *TagEditor
	OnClick   func(g TagGroup)
	OnDelete  func(g TagGroup)  // 悬停时的叉：删除这个标签（合并进来的几个来源一起删）
	OnAdd     func(name string) // 提交了非空的标签名；成功后调用方把 Editor 复位
	// Tooltip 在每个标签上构建悬停提示（来源）；为 nil 时不提示。
	Tooltip func(anchor ui.Element, g TagGroup)
}

// GameTags 对应 GameTags.tsx：flex flex-wrap items-center gap-2 的标签胶囊，末尾是虚线的“添加标签”按钮，
// 点击后变成输入框：Enter 或失去焦点时提交，Esc 取消，空白时直接收起。
// 原版没有的：同名标签合并成一个胶囊（批量删除在编辑页的“游戏标签”里）。
func GameTags(c *ui.Context, t *theme.Theme, p GameTagsProps) {
	ui.Row(c.Key("game-tags")).Wrap().GapX(8).GapY(8).AlignItems(ui.Center).Children(func() {
		for _, g := range p.Groups {
			tagPill(c, t, g, p)
		}
		if p.Editor.Adding {
			tagInput(c, t, p)
		} else {
			addTagButton(c, t, func() { p.Editor.Adding = true })
		}
	})
}

// tagInput：w-32 rounded-full border border-brand-300 bg-white/90 px-3 py-1.5 text-xs，聚焦时 border-brand-500
// bg-white ring-2 ring-brand-200（dark:border-brand-400 dark:bg-brand-900 dark:ring-brand-700），自动聚焦。
func tagInput(c *ui.Context, t *theme.Theme, p GameTagsProps) {
	ed := p.Editor
	in := ui.TextInputBase(c.Key("tag-input"), &ed.Input).Width(128).Padding(6, 12).Radius(999).AutoFocus().
		FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).TextColor(t.TextStrong).
		Placeholder(locales.TagsInputPlaceholder()).PlaceholderColor(t.Pick(theme.Brand400, theme.Brand500)).Label(locales.TagsInputPlaceholder())
	if theme.UIFont != "" {
		in.Font(theme.UIFont)
	}
	focused := in.Focused()
	if focused {
		in.Border(t.HL, t.Pick(theme.Brand500, theme.Brand400)).Background(t.Pick(theme.White, theme.Brand900)).
			Shadow(0, 0, 0, 2, t.Pick(theme.Brand200, theme.Brand700))
	} else {
		in.Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).Background(t.Pick(theme.White.Alpha(0.9), theme.Brand900.Alpha(0.8)))
	}
	submit := func() {
		name := strings.TrimSpace(ed.Input)
		if name == "" {
			ed.Adding = false
			return
		}
		if p.OnAdd != nil {
			p.OnAdd(name)
		}
	}
	in.OnSubmit(submit).OnShortcut(0, ui.KeyEscape, func() { ed.Adding, ed.Input = false, "" })
	// onBlur：上一帧有焦点、这一帧没有时提交。
	had := ui.Local(in, "had-focus", func() bool { return false })
	if *had && !focused && ed.Adding {
		submit()
	}
	*had = focused
}

// tagPill 对应 TagPill：rounded-full px-3 py-1.5 text-xs；用户标签是虚线边框。剧透标签在点开前遮住文字。
func tagPill(c *ui.Context, t *theme.Theme, g TagGroup, p GameTagsProps) {
	pill := ui.Row(c.Key(g.Key)).MaxWidthPercent(100).Padding(6, 12).Radius(999).AlignItems(ui.Center)
	if !g.Scraped() {
		pill.Border(t.HL, t.Pick(theme.Brand400.Alpha(0.8), theme.Brand500.Alpha(0.7))).BorderStyle(ui.BorderDashed).
			Background(t.Pick(theme.White.Alpha(0.7), theme.Brand900.Alpha(0.45)))
	} else {
		pill.Border(t.HL, t.Pick(theme.Brand200.Alpha(0.9), theme.Brand700.Alpha(0.8))).
			Background(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand800.Alpha(0.55)))
	}
	revealed := ui.Local(pill, "revealed", func() bool { return false })
	spoiler := g.Spoiler() && !*revealed
	hovered := pill.Hovered()
	name := tagname.Display(g.Name, p.Translate)
	pill.Children(func() {
		label := ui.ButtonBase(c.Key("label")).Padding(0, 2).Radius(999).FocusRing(false).Label(name)
		color := t.Pick(theme.Brand700, theme.Brand200)
		if label.Hovered() {
			color = t.Pick(theme.Brand900, theme.White)
		}
		color = label.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
		switch {
		case spoiler:
			label.OnClick(func() { *revealed = true })
		case p.OnClick != nil:
			label.OnClick(func() { p.OnClick(g) })
		}
		label.Children(func() {
			Text(c, name, theme.TextXS, theme.WeightNormal, color).SingleLine()
			// 剧透：blur-sm（4px）hover:blur-none。文字之上叠一层背景模糊，效果等同于模糊文字本身。
			if spoiler && !label.Hovered() {
				ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).PassThrough().Material(glass.Blur{Radius: 4})
			}
		})
		if p.OnDelete != nil {
			tagDeleteButton(c, t, hovered, func() { p.OnDelete(g) })
		}
	})
	if p.Tooltip != nil {
		p.Tooltip(pill, g)
	}
}

// tagDeleteButton：absolute -right-1 -top-1 h-4.5 w-4.5 rounded-full border shadow-sm，悬停胶囊时淡入。
func tagDeleteButton(c *ui.Context, t *theme.Theme, show bool, onClick func()) {
	btn := ui.ButtonBase(c.Key("delete")).Absolute().Right(-4).Top(-4).Size(18, 18).Radius(999).Center().
		FocusRing(false).Label(locales.NativeDeleteTag()).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	// dark:bg-brand-950 不在色板里，深色主题下也是 bg-white。
	bg, border, fg := theme.White, t.Pick(theme.Brand200, theme.Brand700), t.Pick(theme.Brand400, theme.Brand500)
	if btn.Hovered() {
		bg = t.Pick(ui.Hex("#fef2f2"), theme.Red500.Alpha(0.12))
		border = t.Pick(ui.Hex("#fecaca"), theme.Red500.Alpha(0.4))
		fg = t.Pick(theme.Red500, ui.Hex("#fca5a5"))
	}
	btn.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(200 * time.Millisecond))
	opacity := btn.AnimateWith("show", theme.Bool(show || btn.Hovered()), 200*time.Millisecond, theme.EaseTailwind)
	if opacity == 0 {
		btn.Invisible()
		return
	}
	btn.Opacity(opacity).OnClick(onClick).Children(func() {
		Icon(c, "close", 10, fg)
	})
}

// addTagButton：inline-flex gap-1.5 rounded-full border border-dashed px-3 py-1.5 text-xs。
func addTagButton(c *ui.Context, t *theme.Theme, onAdd func()) {
	btn := ui.ButtonBase(c.Key("add-tag")).Padding(6, 12).Radius(999).Gap(6).AlignItems(ui.Center).FocusRing(false)
	border, fg, bg := t.Pick(theme.Brand300, theme.Brand600), t.Pick(theme.Brand500, theme.Brand400), ui.Transparent
	if btn.Hovered() {
		border, fg, bg = t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.Brand600, theme.Brand200), t.Pick(theme.Brand50, theme.Brand800.Alpha(0.5))
	}
	btn.Border(t.HL, border).BorderStyle(ui.BorderDashed).Background(bg).Transition(theme.ColorsTransition(200 * time.Millisecond))
	if onAdd != nil {
		btn.OnClick(onAdd)
	}
	btn.Children(func() {
		Icon(c, "plus", 14, fg)
		Text(c, locales.TagsAdd(), theme.TextXS, theme.WeightNormal, fg).NoWrap()
	})
}
