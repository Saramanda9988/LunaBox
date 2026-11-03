package components

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// CategoryNameMaxLength 是 consts/category.ts 的 CATEGORY_NAME_MAX_LENGTH。
const CategoryNameMaxLength = 60

// CategoryModalProps 是 CategoryModal 的属性。
type CategoryModalProps struct {
	Open      bool
	Value     *string
	Edit      bool // mode="edit"：标题“重命名收藏夹”，按钮“保存”
	TopOffset float32
	OnClose   func()
	OnSubmit  func()
}

// CategoryModal 对应 components/modal/CategoryModal.tsx：w-full max-w-md rounded-xl bg-white p-6 shadow-xl
// （dark:bg-brand-800）的面板，标题 text-xl font-bold mb-4，名称输入框右侧是字数，右下角取消与创建（保存）。
func CategoryModal(c *ui.Context, t *theme.Theme, key string, p CategoryModalProps) {
	if utf8.RuneCountInString(*p.Value) > CategoryNameMaxLength { // maxLength
		*p.Value = string([]rune(*p.Value)[:CategoryNameMaxLength])
	}
	title, submit := locales.CategoriesModalAddTitle(), locales.CategoriesModalCreate()
	if p.Edit {
		title, submit = locales.CategoriesModalEditTitle(), locales.CommonSave()
	}
	Modal(c, t, key, ModalProps{Open: p.Open, TopOffset: p.TopOffset}, func(panel ui.Element) {
		panel.Radius(12).Padding(24).Background(t.PanelBg).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			TextMargin(c, title, theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 16, 0)
			// relative mb-4；input w-full rounded-lg border border-brand-300 p-2 pr-16（dark:border-brand-600 dark:bg-brand-700）
			ui.Box(c.Key("name-wrap")).Margin(0, 0, 16, 0).Children(func() {
				in := ui.TextInputBase(c.Key("name"), p.Value).FillWidth().Padding(8, 64, 8, 8).Radius(8).AutoFocus().
					FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
					Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).
					Placeholder(locales.CategoriesModalNamePlaceholder()).Label(locales.CategoriesModalNamePlaceholder())
				if theme.UIFont != "" {
					in.Font(theme.UIFont)
				}
				if in.Focused() { // focus:ring-2 focus:ring-neutral-500
					in.Shadow(0, 0, 0, 2, theme.Neutral500)
				}
				// pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs tabular-nums
				ui.Box(c).Absolute().Top(0).Bottom(0).Right(12).Center().PassThrough().Children(func() {
					Text(c, fmt.Sprintf("%d/%d", utf8.RuneCountInString(*p.Value), CategoryNameMaxLength),
						theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand400, theme.Brand500))
				})
			})
			// flex justify-end gap-2
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				cancel := ui.ButtonBase(c.Key("cancel")).Padding(8, 16).Radius(8).FocusRing(false)
				if cancel.Hovered() {
					cancel.Background(t.Pick(theme.Brand100, theme.Brand700))
				}
				cancel.OnClick(p.OnClose).Children(func() {
					Text(c, locales.CommonCancel(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand300)).NoWrap()
				})
				ok := ui.ButtonBase(c.Key("submit")).Padding(8, 16).Radius(8).FocusRing(false).Label(submit)
				bg := theme.Neutral600
				if ok.Hovered() {
					bg = theme.Neutral700
				}
				ok.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
				if strings.TrimSpace(*p.Value) == "" {
					Disable(ok, 0.5)
				} else {
					ok.OnClick(p.OnSubmit)
				}
				ok.Children(func() {
					Text(c, submit, theme.TextBase, theme.WeightNormal, theme.White).NoWrap()
				})
			})
		})
	})
}
