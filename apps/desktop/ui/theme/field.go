package theme

import "github.com/egoist/mygo/ui"

// 表单输入框（better.Input / Textarea / NumberInput / ActionInput、编辑页字段、标签输入等）的配色。
// 内置主题保持原版的 bg-white dark:bg-brand-700、border-brand-300 dark:border-brand-600、
// 聚焦 neutral-500 与 placeholder:text-brand-400 dark:text-brand-500；
// 主题插件覆盖 inputBg / inputBorder / inputFocus / placeholder 后跟随插件。

// FieldColors 返回输入框的底色与边框（玻璃模式下换成半透明的 glass-input）。
func (t *Theme) FieldColors() (bg, border ui.Color) {
	return t.GlassInput(
		t.Themed("inputBg", t.InputBg, t.Pick(White, Brand700)),
		t.Themed("inputBorder", t.InputBorder, t.Pick(Brand300, Brand600)),
	)
}

// FieldFocus 返回输入框聚焦时的边框与外圈颜色。
func (t *Theme) FieldFocus() ui.Color {
	return t.Themed("inputFocus", t.InputFocus, Neutral500)
}

// FieldPlaceholder 返回输入框占位文字的颜色。
func (t *Theme) FieldPlaceholder() ui.Color {
	return t.Themed("placeholder", t.Placeholder, t.Pick(Brand400, Brand500))
}
