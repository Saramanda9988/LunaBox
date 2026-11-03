package theme

import "github.com/egoist/mygo/ui"

// 自定义背景的“玻璃”模式：对应根布局（routes/__root.tsx）的 data-glass="true"——开启 background_enabled 且有
// background_image 时，组件换成 uno.config.ts 里 data-glass: 变体与 glass-* 快捷类的半透明配色。
// 原版的弹窗（ModalPortal 里 data-glass="false"）与挂到 body 上的浮层（Tooltip、Headless UI 的 anchor 弹出层）
// 不在 data-glass 根之下，保持原样，所以这里也不改它们。
//
// 原版大部分玻璃元素没有 backdrop-blur（“沿用用户配置的背景图模糊，避免逐层叠加滤镜”），只是半透明的底色与边框；
// 用 glass.Blur 的只有原版带 backdrop-blur 的少数元素，GameCard 正文的 backdrop-blur 没有照搬（一屏几十张卡片，
// 每张都要在 GPU 上模糊一次），只保留它的半透明底色。

// SetGlass 打开玻璃模式；opacity 是 background_opacity（main 与侧边栏底色的不透明度）。
func (t *Theme) SetGlass(opacity float64) {
	t.Glass = true
	t.BgOpacity = float32(opacity)
}

// MainBg 是 <main> 的底色：bg-brand-100 dark:bg-brand-900，玻璃模式下是 rgba(var(--main-bg-rgb), opacity)。
func (t *Theme) MainBg() ui.Color {
	if t.Glass {
		return t.AppBg.Alpha(t.BgOpacity)
	}
	return t.AppBg
}

// IfGlass 对应一对 data-glass:X data-glass:dark:Y：玻璃模式下取 light、dark（按主题），否则取 normal。
func (t *Theme) IfGlass(normal, light, dark ui.Color) ui.Color {
	if t.Glass {
		return t.Pick(light, dark)
	}
	return normal
}

// GlassCard 对应 glass-card：data-glass:bg-white/36 dark:bg-black/12，border-brand-300/65 dark:border-white/12，shadow-none。
// 返回背景与边框；不是玻璃模式时原样返回。阴影由调用方在 t.Glass 时去掉。
func (t *Theme) GlassCard(bg, border ui.Color) (ui.Color, ui.Color) {
	if !t.Glass {
		return bg, border
	}
	return t.Pick(White.Alpha(0.36), Black.Alpha(0.12)), t.Pick(Brand300.Alpha(0.65), White.Alpha(0.12))
}

// GlassPanel 对应 glass-panel：data-glass:bg-white/28 dark:bg-black/8，border-brand-300/55 dark:border-white/10，shadow-none。
func (t *Theme) GlassPanel(bg, border ui.Color) (ui.Color, ui.Color) {
	if !t.Glass {
		return bg, border
	}
	return t.Pick(White.Alpha(0.28), Black.Alpha(0.08)), t.Pick(Brand300.Alpha(0.55), White.Alpha(0.10))
}

// GlassInput 对应 glass-input：data-glass:bg-white/55 dark:bg-black/10，border-brand-400/50 dark:border-white/18。
func (t *Theme) GlassInput(bg, border ui.Color) (ui.Color, ui.Color) {
	if !t.Glass {
		return bg, border
	}
	return t.Pick(White.Alpha(0.55), Black.Alpha(0.10)), t.Pick(Brand400.Alpha(0.5), White.Alpha(0.18))
}

// GlassSection 对应 glass-settings-section：data-glass:bg-white/32 dark:bg-black/12，border-brand-300/65 dark:border-white/12。
func (t *Theme) GlassSection(bg, border ui.Color) (ui.Color, ui.Color) {
	if !t.Glass {
		return bg, border
	}
	return t.Pick(White.Alpha(0.32), Black.Alpha(0.12)), t.Pick(Brand300.Alpha(0.65), White.Alpha(0.12))
}

// BtnColors 是按钮的一组颜色。
type BtnColors struct{ Bg, Hover, Border, Text ui.Color }

// GlassBtn 是 glass-btn-* 的颜色名。
type GlassBtn int

const (
	GlassNeutral GlassBtn = iota // glass-btn-neutral，以及其他颜色名（glass-btn-none 也落到这里，见 GlassButton）
	GlassPrimary                 // glass-btn-primary
	GlassError                   // glass-btn-error
)

// GlassButton 对应 glass-btn-<color>：玻璃模式下换成 data-glass:border border-white/30 dark:border-white/15 与对应颜色的底色、文字、悬停色。
// 原版 BetterButton 的 ghost 是 glass-btn-none，但 uno.config.ts 里 /^glass-btn-(.*)$/ 排在 /^glass-btn-none$/ 之前，
// UnoCSS 取第一个匹配的快捷类，所以它实际也是 neutral。
func (t *Theme) GlassButton(kind GlassBtn, normal BtnColors) BtnColors {
	if !t.Glass {
		return normal
	}
	c := BtnColors{Border: t.Pick(White.Alpha(0.30), White.Alpha(0.15))}
	switch kind {
	case GlassPrimary:
		c.Bg, c.Hover, c.Text = Neutral600.Alpha(0.7), Neutral600.Alpha(0.9), White
	case GlassError:
		c.Bg, c.Hover, c.Text = Error500.Alpha(0.7), Error500.Alpha(0.85), White
	default:
		c.Bg, c.Hover = t.Pick(White.Alpha(0.30), Black.Alpha(0.30)), t.Pick(White.Alpha(0.45), Black.Alpha(0.45))
		c.Text = t.Pick(Neutral900, Neutral100)
	}
	return c
}

// WithoutGlass 在 fn 里关掉玻璃模式，之后恢复：对应原版 ModalPortal 的 data-glass="false"，以及挂到 body 上、
// 不在 data-glass 根之下的浮层（Headless UI 的 Dialog、anchor 弹出层与 BetterTooltip）。
func (t *Theme) WithoutGlass(fn func()) {
	saved := t.Glass
	t.Glass = false
	defer func() { t.Glass = saved }()
	fn()
}
