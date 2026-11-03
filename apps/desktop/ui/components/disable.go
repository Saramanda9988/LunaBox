package components

import "github.com/egoist/mygo/ui"

// Disable 禁用元素（不响应点击、不获得焦点）并让它看起来是 CSS 的 disabled:opacity-*。
// MyGo 绘制禁用的元素时会再把不透明度减半，所以这里设成两倍；高于 0.5 的不透明度（opacity-60 等）
// 在禁用状态下做不到，只能是 0.5。
func Disable(e ui.Element, opacity float32) ui.Element {
	return e.Disabled(true).Opacity(min(1, opacity*2))
}
