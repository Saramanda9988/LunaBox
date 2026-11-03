package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// TextareaProps 是多行输入框的属性。
type TextareaProps struct {
	Placeholder string
	Label       string         // 无障碍名称，默认用 Placeholder
	Rows        int            // rows，决定高度，默认 3
	Font        theme.FontStep // 默认 text-base
	Disabled    bool
}

// Textarea 是原版 <textarea className="glass-input w-full px-3 py-2 border rounded-md resize-none ..."> 的默认样式：
// border-brand-300 dark:border-brand-600 bg-white dark:bg-brand-700，聚焦时 ring-2 ring-neutral-500。
// 返回输入框元素，调用方可以再改圆角、内边距、颜色（如评价页的 rounded-xl bg-brand-50 px-5 py-4）。
func Textarea(c *ui.Context, t *theme.Theme, key string, value *string, p TextareaProps) ui.Element {
	f := p.Font
	if f.Size == 0 {
		f = theme.TextBase
	}
	rows := p.Rows
	if rows <= 0 {
		rows = 3
	}
	ta := ui.TextAreaBase(c.Key(key), value).FillWidth().Padding(8, 12).Radius(6).
		Height(float32(rows)*f.Line + 16 + 2*t.HL).FontSize(f.Size).FixedLineHeight(f.Line).TextColor(t.TextStrong).
		Placeholder(p.Placeholder)
	if theme.UIFont != "" {
		ta.Font(theme.UIFont)
	}
	label := p.Label
	if label == "" {
		label = p.Placeholder
	}
	if label != "" {
		ta.Label(label)
	}
	bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600)) // glass-input
	ta.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if ta.Focused() {
		ta.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	if p.Disabled {
		components.Disable(ta, 0.6)
	}
	return ta
}
