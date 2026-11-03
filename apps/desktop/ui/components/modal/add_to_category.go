package modal

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// categoryNameMaxLength 是 consts/category.ts 的 CATEGORY_NAME_MAX_LENGTH。
const categoryNameMaxLength = 60

// AddToCategory 是 AddToCategoryModal 的状态：所有收藏夹、勾选、搜索与新建表单。页面持有它，用 Show 打开。
type AddToCategory struct {
	open       bool
	categories []vo.CategoryVO
	selected   []string
	query      string
	creating   bool // isCreateFormOpen
	busy       bool // isCreating
	name       string
	emoji      string
	scroll     components.ScrollBar
	gen        int // 每次 Show 加一，关闭后返回的新建结果不再生效
}

// AddToCategoryProps 是 AddToCategoryModal 的属性。
type AddToCategoryProps struct {
	Env         *env.Env
	Title       string // 默认“添加到收藏”（addToCategory.title）
	ConfirmText string // 默认“确定”（common.confirm）
	OnSave      func(selected []string)
}

// Show 用全部收藏夹与初始勾选（initialSelectedIds）打开弹窗。
func (m *AddToCategory) Show(categories []vo.CategoryVO, initial []string) {
	*m = AddToCategory{open: true, categories: categories, selected: slices.Clone(initial), gen: m.gen + 1}
}

// Close 关闭弹窗。
func (m *AddToCategory) Close() { m.open, m.gen = false, m.gen+1 }

// IsOpen 报告弹窗是否打开。
func (m *AddToCategory) IsOpen() bool { return m.open }

// CreateName 是新建表单里的名称（测试用）。
func (m *AddToCategory) CreateName() string { return m.name }

// create 对应 handleCreateCategory：新建后选中它。
func (m *AddToCategory) create(c *ui.Context, e *env.Env) {
	name := strings.TrimSpace(m.name)
	if name == "" || m.busy {
		return
	}
	existing := map[string]bool{}
	for _, cat := range m.categories {
		existing[cat.ID] = true
	}
	m.busy = true
	emoji, gen := strings.TrimSpace(m.emoji), m.gen
	e.Async(func() func() {
		err := e.Svc.Category.AddCategory(name, emoji)
		var all []vo.CategoryVO
		if err == nil {
			all, err = e.Svc.Category.GetCategories()
		}
		return func() {
			if gen != m.gen || !m.open {
				return
			}
			m.busy = false
			if err != nil {
				e.Toast(c, env.Error(locales.CategoriesToastCreateFailed()))
				return
			}
			m.categories = all
			for _, cat := range all {
				if !existing[cat.ID] && !slices.Contains(m.selected, cat.ID) {
					m.selected = append(m.selected, cat.ID)
				}
			}
			m.query, m.creating, m.name, m.emoji = "", false, "", ""
			e.Toast(c, env.Success(locales.CategoriesToastCreateSuccess()))
		}
	})
}

// View 对应 AddToCategoryModal：w-full max-w-lg max-h-[78vh] rounded-xl border shadow-xl 的竖排面板。
// 要在页面最后构建，浮层才会盖住页面内容。
func (m *AddToCategory) View(c *ui.Context, t *theme.Theme, p AddToCategoryProps) {
	_, winH := c.Size()
	title, confirm := p.Title, p.ConfirmText
	if title == "" {
		title = locales.AddToCategoryTitle()
	}
	if confirm == "" {
		confirm = locales.CommonConfirm()
	}
	closeModal := m.Close
	components.Modal(c, t, "category-modal", components.ModalProps{Open: m.open, TopOffset: env.TopbarHeight, MaxWidth: 512},
		func(panel ui.Element) {
			panel.MaxHeight(winH*0.78).Radius(12).Clip().Background(t.PanelBg).Border(t.HL, t.PanelBorder).
				Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
			panel.Children(func() {
				categoryHeader(c, t, title, len(m.selected), closeModal)
				m.form(c, t, p.Env)
				// 面板 max-h-[78vh] 减去标题（p-6 + 两行）、表单（pt-4 + h-11）与底部按钮（p-4 + py-2.5）
				m.list(c, t, winH*0.78-(48+28+20+2*t.HL)-(16+44)-(32+44+t.HL))
				// border-t p-4 flex gap-3
				ui.Row(c).Padding(16).Gap(12).BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Divider).Children(func() {
					cancel := ui.ButtonBase(c.Key("cancel")).Grow(1).Basis(0).Padding(10, 0).Radius(12).Center().FocusRing(false).
						Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
					if cancel.Hovered() {
						cancel.Background(t.Pick(theme.Brand50, theme.Brand700))
					}
					cancel.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(closeModal).Children(func() {
						components.Text(c, locales.CommonCancel(), theme.TextBase, theme.WeightMedium, t.Pick(theme.Brand600, theme.Brand400))
					})
					ok := ui.ButtonBase(c.Key("confirm")).Grow(1).Basis(0).Padding(10, 0).Radius(12).Center().FocusRing(false)
					bg := theme.Neutral600
					if ok.Hovered() {
						bg = theme.Neutral700
					}
					ok.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() {
						selected := slices.Clone(m.selected)
						closeModal()
						if p.OnSave != nil {
							p.OnSave(selected)
						}
					}).Children(func() {
						components.Text(c, confirm, theme.TextBase, theme.WeightMedium, theme.White)
					})
				})
			})
		})
}

// categoryHeader：p-6 border-b flex justify-between gap-4，标题 text-xl font-bold，下面 mt-1 text-xs 的已选数量。
func categoryHeader(c *ui.Context, t *theme.Theme, title string, count int, onClose func()) {
	ui.Row(c).Padding(24).Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Children(func() {
		ui.Column(c).MinWidth(0).Children(func() {
			components.Text(c, title, theme.TextXL, theme.WeightBold, t.TextStrong).SingleLine()
			components.TextMargin(c, locales.AddToCategorySelectedCount(strconv.Itoa(count)), theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
		})
		// rounded-lg p-2 text-brand-500 hover:bg-brand-100 hover:text-brand-700
		btn := ui.ButtonBase(c.Key("close")).Padding(8).Radius(8).Shrink(0).FocusRing(false).Label(locales.CommonCancel())
		color := t.Pick(theme.Brand500, theme.Brand400)
		if btn.Hovered() {
			btn.Background(t.Pick(theme.Brand100, theme.Brand700))
			color = t.Pick(theme.Brand700, theme.White)
		}
		btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onClose).Children(func() {
			components.Icon(c, "close", 20, color)
		})
	})
}

// form：px-6 pt-4 flex gap-3：搜索框或新建表单，右侧 h-11 w-11 rounded-full 的新建 / 确认按钮。
func (m *AddToCategory) form(c *ui.Context, t *theme.Theme, e *env.Env) {
	ui.Row(c.Key("category-form")).Padding(16, 24, 0, 24).Gap(12).AlignItems(ui.Center).Children(func() {
		if m.creating {
			components.EmojiPicker(c, t, "emoji", components.EmojiPickerProps{
				Value: m.emoji, CanEdit: true, Compact: true, FallbackIcon: "folder-outline",
				OnChange: func(e string) { m.emoji = e },
			})
			if utf8.RuneCountInString(m.name) > categoryNameMaxLength {
				m.name = string([]rune(m.name)[:categoryNameMaxLength])
			}
			categoryInput(c, t, "name", &m.name, locales.CategoriesModalNamePlaceholder(), 12, 96, func() {
				components.Text(c, fmt.Sprintf("%d/%d", utf8.RuneCountInString(m.name), categoryNameMaxLength),
					theme.TextXS, theme.WeightNormal, t.TextFaint).Absolute().Right(40).Top(14).PassThrough()
				inputClearButton(c, t, locales.CommonCancel(), func() { m.creating, m.name, m.emoji = false, "", "" })
			}).OnSubmit(func() { m.create(c, e) })
		} else {
			categoryInput(c, t, "search", &m.query, locales.AddToCategorySearchPlaceholder(), 40, 40, func() {
				ui.Box(c).Absolute().Left(12).Top(0).Bottom(0).Center().PassThrough().Children(func() {
					components.Icon(c, "magnify", 20, theme.Brand400)
				})
				if m.query != "" {
					inputClearButton(c, t, locales.AddToCategoryClearSearch(), func() { m.query = "" })
				}
			})
		}
		// h-11 w-11 rounded-full bg-neutral-600 text-white shadow-sm，disabled:opacity-50
		disabled := m.creating && (strings.TrimSpace(m.name) == "" || m.busy)
		label, icon := locales.CategoriesNewCategory(), "plus"
		switch {
		case m.busy:
			label, icon = locales.AddToCategoryCreating(), "loading"
		case m.creating:
			label, icon = locales.CategoriesModalCreate(), "check"
		}
		btn := ui.ButtonBase(c.Key("create")).Size(44, 44).Radius(999).Center().Shrink(0).FocusRing(false).Label(label).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
		bg := theme.Neutral600
		if btn.Hovered() && !disabled {
			bg = theme.Neutral700
		}
		btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
		if disabled {
			components.Disable(btn, 0.5)
		}
		btn.OnClick(func() {
			if m.creating {
				m.create(c, e)
				return
			}
			// openCreateForm：把搜索词带进名称
			name := []rune(strings.TrimSpace(m.query))
			m.name = string(name[:min(len(name), categoryNameMaxLength)])
			m.query, m.creating = "", true
		}).Children(func() {
			if m.busy {
				components.Spinner(c, 24, theme.White)
			} else {
				components.Icon(c, icon, 24, theme.White)
			}
		})
	})
}

// categoryInput：glass-input h-11 w-full rounded-lg border border-brand-200 bg-white text-sm（dark:bg-brand-900），自动聚焦。
func categoryInput(c *ui.Context, t *theme.Theme, key string, value *string, placeholder string, padL, padR float32, extra func()) ui.Element {
	var in ui.Element
	ui.Box(c.Key(key + "-wrap")).Grow(1).Basis(0).MinWidth(0).Children(func() {
		in = ui.TextInputBase(c.Key(key), value).FillWidth().Height(44).Padding(0, padR, 0, padL).Radius(8).AutoFocus().
			FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
			Background(t.Pick(theme.White, theme.Brand900)).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
			Placeholder(placeholder).Label(placeholder)
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		extra()
	})
	return in
}

// inputClearButton：absolute right-2 rounded-md p-1.5 text-brand-400 hover:bg-brand-100 hover:text-brand-700，i-mdi-close text-base。
func inputClearButton(c *ui.Context, t *theme.Theme, label string, onClick func()) {
	ui.Box(c).Absolute().Right(8).Top(0).Bottom(0).Center().Children(func() {
		btn := ui.ButtonBase(c.Key("clear")).Padding(6).Radius(6).FocusRing(false).Label(label)
		color := theme.Brand400
		if btn.Hovered() {
			btn.Background(t.Pick(theme.Brand100, theme.Brand700))
			color = t.Pick(theme.Brand700, theme.Brand200)
		}
		btn.OnClick(onClick).Children(func() { components.Icon(c, "close", 16, color) })
	})
}

// list：min-h-0 flex-1 overflow-y-auto p-6 的 space-y-2 列表；没有结果时 min-h-44 的提示。
func (m *AddToCategory) list(c *ui.Context, t *theme.Theme, maxH float32) {
	q := strings.ToLower(strings.TrimSpace(m.query))
	var list []vo.CategoryVO
	for _, cat := range m.categories {
		if q == "" || strings.Contains(strings.ToLower(categoryName(cat)+" "+cat.Name), q) {
			list = append(list, cat)
		}
	}
	gutter := float32(0)
	if m.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	// min-h-0 flex-1 overflow-y-auto：MyGo 的 Basis(0) 在高度不定的列里会把它压成 0，所以直接限制最大高度。
	components.ScrollArea(c, t, "category-list", &m.scroll, func(w ui.Element) { w.MaxHeight(max(0, maxH)) }, func(sc ui.Element) {
		sc.Padding(24, 24+gutter, 24, 24).Children(func() {
			if len(list) == 0 {
				icon, msg := "folder-plus-outline", locales.AddToCategoryEmpty()
				if len(m.categories) > 0 {
					icon, msg = "folder-search-outline", locales.AddToCategoryNoResults()
				}
				ui.Column(c).MinHeight(176).Padding(0, 24).Center().Children(func() {
					components.Icon(c, icon, 36, t.TextMuted).Margin(0, 0, 12, 0)
					components.Text(c, msg, theme.TextSM, theme.WeightMedium, t.TextMuted)
				})
				return
			}
			ui.Column(c).Gap(8).Children(func() {
				for _, cat := range list {
					categoryRow(c.Key(cat.ID), t, cat, slices.Contains(m.selected, cat.ID), func() {
						if i := slices.Index(m.selected, cat.ID); i >= 0 {
							m.selected = slices.Delete(m.selected, i, i+1)
						} else {
							m.selected = append(m.selected, cat.ID)
						}
					})
				}
			})
		})
	})
}

// categoryName：系统收藏夹显示为“最喜欢的游戏”。
func categoryName(cat vo.CategoryVO) string {
	if cat.IsSystem {
		return locales.CategoriesFavorites()
	}
	return cat.Name
}

// categoryRow：w-full rounded-lg bg-brand-50 p-3 flex gap-3，悬停 bg-brand-100；右侧 check-circle / circle-outline。
func categoryRow(c *ui.Context, t *theme.Theme, cat vo.CategoryVO, selected bool, onToggle func()) {
	row := ui.ButtonBase(c).FillWidth().Padding(12).Radius(8).Gap(12).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).
		Label(categoryName(cat))
	bg := t.Pick(theme.Brand50, theme.Brand900)
	if row.Hovered() {
		bg = t.Pick(theme.Brand100, theme.Brand700)
	}
	row.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onToggle).Children(func() {
		ui.Box(c).Width(32).Center().Shrink(0).Children(func() {
			if e := strings.TrimSpace(cat.Emoji); e != "" {
				ui.Text(c, e).Font("Segoe UI Emoji").FontSize(24).FixedLineHeight(24).NoWrap()
			} else {
				icon := "folder-outline"
				if cat.IsSystem {
					icon = "heart"
				}
				components.Icon(c, icon, 20, t.TextMuted)
			}
		})
		components.Text(c, categoryName(cat), theme.TextBase, theme.WeightMedium, t.TextStrong).SingleLine().Grow(1).Basis(0).MinWidth(0)
		components.Text(c, locales.AddToCategoryGameCount(strconv.Itoa(cat.GameCount)), theme.TextSM, theme.WeightNormal, t.TextMuted).NoWrap()
		icon, color := "circle-outline", t.Pick(theme.Brand300, theme.Brand600)
		if selected {
			icon, color = "check-circle", t.Pick(theme.Neutral600, theme.Neutral300)
		}
		components.Icon(c, icon, 20, color)
	})
}
