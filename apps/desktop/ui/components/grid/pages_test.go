package grid

import (
	"testing"

	"lunabox/backend/models"
	"lunabox/desktop/ui/env"
)

// 拖动滚动条停在页尾附近时，第一次补的窗口只覆盖到可见范围开头；返回后应从第一个缺口继续补，
// 而不是重算出同一个已加载的窗口后停下。
func TestEnsureFillsWholeVisibleRange(t *testing.T) {
	const total = 2000
	p := NewPages(&env.Env{})
	p.Reload("q", func(offset, limit int) ([]models.Game, int, error) {
		end := min(total, offset+limit)
		return make([]models.Game, max(0, end-offset)), total, nil
	})

	// 11 列、9 行（含上下 overscan），从 1199 开始：首个窗口 [960, 1200) 只覆盖 1199。
	start, end := 1199, 1199+99-1
	for range 5 { // 每次加载返回后页面重建，VirtualGrid 会再次报告同一范围
		p.Ensure(start, end)
	}
	for i := start; i <= end; i++ {
		if p.Games[i] == nil {
			t.Fatalf("index %d still missing after Ensure", i)
		}
	}
}
