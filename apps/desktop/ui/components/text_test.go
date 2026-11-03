package components

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// Text 的占位高度必须等于 CSS 行高，后面元素的位置才会与浏览器一致。
func TestLineBoxHeights(t *testing.T) {
	cases := []struct {
		s string
		f theme.FontStep
		w int
	}{
		{"游戏库", theme.Text4XL, theme.WeightBold},
		{"共 12 个游戏", theme.TextBase, theme.WeightNormal},
		{"橘光柠水随想曲", theme.FontStep{Size: 14, Line: 17.5}, theme.WeightBold},
		{"ゆずソフト", theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal},
		{"Navel", theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal},
		{"添加游戏", theme.TextSM, theme.WeightMedium},
	}
	for i, k := range cases {
		mark := fmt.Sprintf("mark%d", i)
		view := func(c *ui.Context) {
			ui.Column(c).Children(func() {
				Text(c, k.s, k.f, k.w, theme.White)
				ui.Text(c, mark)
			})
		}
		tt := ui.NewTester(view, 800, 400)
		tt.SetScale(1.5)
		tt.Frame()
		r, _ := tt.Find(mark)
		if math.Abs(float64(r.Y-k.f.Line)) > 0.5 {
			t.Errorf("%s (%v): next element at y=%.2f, want %.1f", k.s, k.f, r.Y, k.f.Line)
		}
	}
}

// 带换行的文字按一行的溢出算，而不是几行的总高。
func TestTextOverflowMultiline(t *testing.T) {
	one := TextOverflow("游戏库 Navel", theme.TextSM, theme.WeightNormal)
	two := TextOverflow("游戏库 Navel\n游戏库 Navel", theme.TextSM, theme.WeightNormal)
	if one != two {
		t.Errorf("multiline overflow = %.2f, want %.2f (same as one line)", two, one)
	}
}
