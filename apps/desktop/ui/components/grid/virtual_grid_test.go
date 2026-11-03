package grid

import (
	"fmt"
	"testing"

	"lunabox/desktop-mygo/ui/theme"
)

func TestGeometryMatchesWeb(t *testing.T) {
	// 1280 宽窗口、展开侧边栏、150% 缩放：原版容器 951.3，6 列，卡片 148.6，行高 235。
	g := NewGeometry(1024-64-theme.DevicePx(9, 1.5), false)
	if g.Cols != 6 || fmt.Sprintf("%.1f", g.CardW) != "148.6" || g.RowH != 235 {
		t.Fatalf("geometry = %+v", g)
	}
}
