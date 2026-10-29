package theme

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Theme 是一帧里组件需要的全部样式环境：色板，以及窗口所在显示器的缩放。
type Theme struct {
	Palette
	Scale float64 // 显示器缩放比例
	HL    float32 // 一个物理像素的 DIP 宽度，即 1px 边框的实际宽度

	Glass     bool    // 根布局的 data-glass：开启了自定义背景（见 glass.go）
	BgOpacity float32 // background_opacity：玻璃模式下 main 与侧边栏底色的不透明度
}

// New 按系统主题和显示器缩放创建 Theme。
func New(dark bool, scale float64) *Theme {
	if scale <= 0 {
		scale = 1
	}
	return &Theme{Palette: NewPalette(dark), Scale: scale, HL: Hairline(scale)}
}

// Gutter 是 style.css 里 9px 滚动条槽吸附到设备像素后的宽度。
func (t *Theme) Gutter() float32 { return DevicePx(9, t.Scale) }

// Hairline 返回一个物理像素的 DIP 宽度：Chromium 会把 1px 边框吸附到设备像素。
func Hairline(scale float64) float32 {
	if scale <= 0 {
		return 1
	}
	return float32(math.Max(1, math.Floor(scale)) / scale)
}

// DevicePx 把 DIP 尺寸向下取整到设备像素（滚动条槽宽等）。
func DevicePx(v float32, scale float64) float32 {
	if scale <= 0 {
		return v
	}
	return float32(math.Floor(float64(v)*scale) / scale)
}

// Tailwind 的字号阶梯：字号与行高（px = DIP）。
type FontStep struct{ Size, Line float32 }

var (
	TextXS   = FontStep{12, 16}
	TextSM   = FontStep{14, 20}
	TextBase = FontStep{16, 24}
	TextLG   = FontStep{18, 28}
	TextXL   = FontStep{20, 28}
	Text2XL  = FontStep{24, 32}
	Text3XL  = FontStep{30, 36}
	Text4XL  = FontStep{36, 40}
)

const (
	WeightNormal   = 400
	WeightMedium   = 500
	WeightSemibold = 600
	WeightBold     = 700
)

// UIFont 是 CSS system-ui 在本机对应的字体（中文 Windows 上是 Microsoft YaHei UI）。
// 原版字体栈的 ui-sans-serif 在 Chromium 中不被支持，所以实际生效的就是它。
var UIFont = systemUIFont()

// CubicBezier 返回 CSS cubic-bezier() 的缓动函数。
func CubicBezier(x1, y1, x2, y2 float64) ui.Easing {
	sample := func(a1, a2, t float64) float64 {
		u := 1 - t
		return 3*u*u*t*a1 + 3*u*t*t*a2 + t*t*t
	}
	return func(x float32) float32 {
		if x <= 0 {
			return 0
		}
		if x >= 1 {
			return 1
		}
		// 二分求 t 使 bezierX(t) = x。
		lo, hi := 0.0, 1.0
		t := float64(x)
		for range 24 {
			if sample(x1, x2, t) < float64(x) {
				lo = t
			} else {
				hi = t
			}
			t = (lo + hi) / 2
		}
		return float32(sample(y1, y2, t))
	}
}

var (
	EaseCSS      = CubicBezier(0.25, 0.1, 0.25, 1) // CSS 的 ease
	EaseTailwind = CubicBezier(0.4, 0, 0.2, 1)     // Tailwind transition 默认曲线，也是 ease-out
	EaseIn       = CubicBezier(0.4, 0, 1, 1)       // Tailwind ease-in
	EaseOut      = CubicBezier(0, 0, 0.2, 1)       // UnoCSS ease-out
	EaseDrawer   = CubicBezier(0.22, 1, 0.36, 1)
	EasePulse    = CubicBezier(0.4, 0, 0.6, 1) // animate-pulse
)

// ColorsTransition 对应 Tailwind 的 transition-colors（默认 150ms）。
func ColorsTransition(d time.Duration) ui.ElementTransition {
	return ui.ElementTransition{Colors: true, Duration: d, Ease: EaseTailwind}
}

// Tween 是 Tailwind 默认曲线下的一段过渡。
func Tween(d time.Duration) ui.Animation { return ui.Animation{Duration: d, Ease: EaseTailwind} }

// Bool 把布尔值变成动画目标 0/1。
func Bool(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
