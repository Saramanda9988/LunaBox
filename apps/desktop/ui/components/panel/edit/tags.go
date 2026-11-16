package edit

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/gamefield"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/tagname"
	"lunabox/desktop/ui/theme"
)

// tagsField 是“游戏标签”（原版没有）：合并后的标签，点标签选中，标题右侧全选与删除所选，胶囊上的叉删除一个，
// 末尾的“+”添加用户标签。
func (e *editor) tagsField(c *ui.Context) {
	t, s, p := e.t, e.s, e.p
	if p.OnDeleteTags == nil || p.OnAddTag == nil {
		return
	}
	// 只保留还在的标签的选中状态（删除后、重新获取元数据后）。
	present := make(map[string]bool, len(p.Tags))
	var selected []components.TagGroup
	for _, g := range p.Tags {
		present[g.Key] = true
		if s.tagSelected[g.Key] {
			selected = append(selected, g)
		}
	}
	for key := range s.tagSelected {
		if !present[key] {
			delete(s.tagSelected, key)
		}
	}
	all := len(selected) == len(p.Tags)

	ui.Column(c.Key("tags-field")).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
			gamefield.Label(c, t, locales.NativeGameEditTags())
			if len(p.Tags) == 0 {
				return
			}
			ui.Row(c).AlignItems(ui.Center).Gap(4).Margin(0, 0, 4, 0).Children(func() {
				label, icon := locales.CommonSelectAll(), "checkbox-multiple-marked-outline"
				if all {
					label, icon = locales.NativeTagsSelectNone(), "checkbox-blank-outline"
				}
				better.Button(c, t, "select-all-tags", better.ButtonProps{
					Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: icon, Label: label,
					OnClick: func() {
						s.tagSelected = map[string]bool{}
						if !all {
							for _, g := range p.Tags {
								s.tagSelected[g.Key] = true
							}
						}
					},
				})
				if len(selected) > 0 {
					// 无底色的按钮，图标与文字是红色。
					label := locales.NativeTagsDeleteSelected(strconv.Itoa(len(selected)))
					better.Button(c, t, "delete-selected-tags", better.ButtonProps{
						Variant: better.ButtonGhost, Size: better.ButtonSM, Label: label,
						OnClick: func() { p.OnDeleteTags(c, selected) },
						Children: func(ui.Color) {
							red := t.Pick(theme.Red500, ui.Hex("#fca5a5"))
							components.Icon(c, "delete-outline", 16, red)
							components.Text(c, label, theme.FontStep{Size: 12, Line: 12}, theme.WeightMedium, red).NoWrap()
						},
					}).Label(label)
				}
			})
		})
		gamefield.TagSelect(c, t, "tags", gamefield.TagSelectProps{
			Groups: p.Tags, Selected: s.tagSelected, Translate: tagname.ModeOf(p.Env.Config()),
			Adding: &s.tagAdding, Draft: &s.tagDraft,
			OnAdd:  func(name string) { p.OnAddTag(c, name) },
			OnToggle: func(g components.TagGroup) {
				if s.tagSelected[g.Key] {
					delete(s.tagSelected, g.Key)
					return
				}
				if s.tagSelected == nil {
					s.tagSelected = map[string]bool{}
				}
				s.tagSelected[g.Key] = true
			},
			OnRemove: func(g components.TagGroup) { p.OnDeleteTags(c, []components.TagGroup{g}) },
		})
		gamefield.Hint(c, t, locales.NativeGameEditTagsHint(), false)
	})
}
