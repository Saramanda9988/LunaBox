package panel

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

// Settings 是设置页（routes/settings.tsx）传给各设置面板的内容，对应原版面板的 formData、onChange 等属性。
// 设置面板与详情页的标签页一样每个一个子包（basic、metadata、background 等），面板自己调用 Env.Svc 里的真实服务。
type Settings struct {
	Env *env.Env
	// Config 是 draftConfig：页面持有的草稿。面板只读它，修改通过 Change 或 PatchLive 提交。
	Config *appconf.AppConfig
	// Change 对应 onChange（setDraftConfig）：fn 立即改草稿（同一遍构建里随后读到的就是新值），
	// 最后一次修改 250ms 后页面自动保存（UpdateAppConfig）。
	Change func(fn func(cfg *appconf.AppConfig))
	// PatchLive 对应 patchLiveConfig：同时修改草稿与生效的配置，并立即保存。
	PatchLive func(fn func(cfg *appconf.AppConfig))
	// Refresh 对应 onConfigRefresh（store 的 fetchConfig）：服务自己改了配置（授权、登出、安排恢复等）之后，
	// 从服务重新读取配置，覆盖草稿。
	Refresh func()
}

// Toast 显示提示（react-hot-toast 的 toast.success / toast.error）。
func (p Settings) Toast(c *ui.Context, n env.Notice) { p.Env.Toast(c, n) }

// FieldLabel：block text-sm font-medium text-brand-700 dark:text-brand-300（设置面板里 label 与控件之间的间距
// 来自外层的 space-y-2）。
func FieldLabel(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.Text(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel)
}

// GroupTitle：block text-sm font-semibold text-brand-700 dark:text-brand-300（面板里一组设置的小标题）。
func GroupTitle(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.Text(c, s, theme.TextSM, theme.WeightSemibold, t.SectionLabel)
}

// FieldHint：text-xs text-brand-500 dark:text-brand-400。
func FieldHint(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.Text(c, s, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
}

// Field 是设置面板最常见的一项：space-y-2 的列，依次是 label、控件与提示（hint 为空时不显示）。
func Field(c *ui.Context, t *theme.Theme, key, label, hint string, control func()) ui.Element {
	return ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		FieldLabel(c, t, label)
		control()
		if hint != "" {
			FieldHint(c, t, hint)
		}
	})
}

// Divided 是 border-t border-brand-200 pt-6 dark:border-brand-700 的一组设置（原版常写成 mt-6 border-t pt-6；放在
// space-y-* 的父元素里时 mt-6 被 space-y 的外边距盖掉，间距由父元素的 Gap 决定）。
func Divided(c *ui.Context, t *theme.Theme, key string, pt float32, content func()) ui.Element {
	return ui.Column(c.Key(key)).FillWidth().Padding(pt, 0, 0, 0).
		BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(content)
}
