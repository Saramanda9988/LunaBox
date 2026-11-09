package app

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestFitWorkArea(t *testing.T) {
	wa := mygo.Rectangle{Width: 1707, Height: 1067}
	for _, c := range []struct {
		w, h         int
		wa           mygo.Rectangle
		wantW, wantH int
	}{
		{2560, 1440, wa, 1707, 1067},                                              // 别的屏幕上存的大窗口：缩到工作区
		{1230, 800, wa, 1230, 800},                                                // 放得下：不变
		{0, 0, wa, minWidth, minHeight},                                           // 没有配置：最小尺寸
		{2560, 1440, mygo.Rectangle{}, 2560, 1440},                                // 拿不到显示器：不限制
		{1230, 800, mygo.Rectangle{Width: 800, Height: 500}, minWidth, minHeight}, // 工作区比最小尺寸还小
	} {
		if w, h := fitWorkArea(c.w, c.h, c.wa); w != c.wantW || h != c.wantH {
			t.Errorf("fitWorkArea(%d, %d, %v) = %d×%d, want %d×%d", c.w, c.h, c.wa, w, h, c.wantW, c.wantH)
		}
	}
}
