package gamefield

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// Rating 对应评分：BetterInput type="number" min=0 max=10 step=0.1（flex-1），右侧 text-sm 的“/ 10”。
// text 是输入框里的文字（允许“8.”这样的中间状态），由调用方持有；评分改变时调用 onChange（空为 0）。
func Rating(c *ui.Context, t *theme.Theme, text *string, onChange func(float64)) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		prev := *text
		var in ui.Element
		ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			in = better.Input(c, t, "input", text, better.InputProps{Placeholder: locales.GameEditRatingPlaceholder(), Label: locales.GameEditRating()})
		})
		if in.Changed() {
			if rating, ok := changeRating(text, prev); ok {
				onChange(rating)
			}
		}
		components.Text(c, "/ 10", theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
	})
}

// changeRating 对应 number input 的 onChange：空为 0，否则限制在 0–10。number input 不接受的字符不会进入输入框
// （恢复成 prev）；“-”“.”这样还没输完的文字保留，评分不变（返回 false）。
func changeRating(text *string, prev string) (float64, bool) {
	v := strings.TrimSpace(*text)
	if v == "" {
		return 0, true
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		if v != "-" && v != "." && !strings.HasSuffix(v, "e") {
			*text = prev
		}
		return 0, false
	}
	rating := min(10, max(0, f))
	if rating != f {
		*text = FormatRating(rating)
	}
	return rating, true
}

// FormatRating 是 value={game.rating > 0 ? game.rating : ""}。
func FormatRating(r float64) string {
	if r <= 0 {
		return ""
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}
