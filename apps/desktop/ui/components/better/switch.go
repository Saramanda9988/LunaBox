package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// Switch 对应 BetterSwitch：w-11 h-6 rounded-full，滑块 w-5 h-5，translate-x-0.5 ↔ 5.5（duration-100），
// 关 bg-brand-300 dark:bg-brand-750，开 bg-neutral-600，hover:brightness-110。
func Switch(c *ui.Context, t *theme.Theme, key string, checked, disabled bool, onChange func(bool)) {
	track := ui.ButtonBase(c.Key(key)).Size(44, 24).Radius(999).FocusRing(false).Justify(ui.Start).AlignItems(ui.Center)
	bg := t.SwitchOff
	if checked {
		bg = theme.Neutral600
	}
	switch {
	case disabled:
		components.Disable(track, 0.5)
	case track.Hovered():
		bg = bg.Mix(theme.White, 0.1)
	}
	track.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	x := track.AnimateWith("knob", 2+20*theme.Bool(checked), 100*time.Millisecond, theme.EaseTailwind)
	track.OnClick(func() { onChange(!checked) }).Children(func() {
		ui.Box(c).Size(20, 20).Radius(999).Background(theme.White).Translate(x, 0)
	})
}
