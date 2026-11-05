package components

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/theme"
)

// ProgressBar 是 MyGo 的 ui.Progress 换上原版配色：h-1.5 的圆角条，轨道 bg-brand-200 dark:bg-brand-700，填充 fill。
// value 在 0 到 1 之间时填充到 value，宽度变化有 300ms 的过渡（原版 transition-all duration-300）；
// 为负数时是 MyGo 的进度未知样式（一段来回滑动的填充）。返回 ui.Progress 本身，调用方可以再设透明度等。
//
// MyGo 用主题的 Accent 画填充、Border 画轨道，而主窗口把 Accent 设成了文字色（见 ui/app.go 的 applyTheme），
// 所以套一层 ui.Themed 只给这根进度条换颜色。
func ProgressBar(c *ui.Context, t *theme.Theme, value float64, fill ui.Color) ui.Element {
	mt := *c.Theme()
	mt.Accent, mt.Border = fill, t.Pick(theme.Brand200, theme.Brand700)
	wrap := ui.Themed(c, &mt).FillWidth()
	if value >= 0 {
		value = float64(wrap.AnimateWith("value", float32(min(value, 1)), 300*time.Millisecond, theme.EaseTailwind))
	}
	var bar ui.Element
	wrap.Children(func() {
		bar = ui.Progress(c, value).FillWidth().Height(6)
	})
	return bar
}
