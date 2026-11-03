package components

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// nsfwBlur 是 uno.config.ts 的 nsfw-cover-blur：filter: blur(18px)。
const nsfwBlur = 18

// CoverBlur 对应 ProxyImage 的 NSFW 封面模糊：在 host 里紧跟封面图片 img 之后调用，模糊图片本身
// （Element.Blur 即 filter: blur()，其后构建的角标、覆盖条不受影响）。reveal 对应 revealNSFWOnHover：悬停 host 时
// 300ms 内变清晰（hover:nsfw-cover-reveal transition-[filter] duration-300），并显示 GameCoverImage 的眼睛水印
// （i-mdi-eye-outline size-24 text-white opacity-[0.22]，悬停时淡出）。
func CoverBlur(c *ui.Context, host, img ui.Element, reveal bool) {
	k := host.AnimateWith("nsfw-reveal", theme.Bool(reveal && host.Hovered()), 300*time.Millisecond, theme.EaseTailwind)
	if r := nsfwBlur * (1 - k); r > 0.1 {
		img.Blur(r)
	}
	if reveal && k < 1 {
		ui.Box(c.Key("nsfw-mark")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().
			Opacity(1 - k).Children(func() {
			Icon(c, "eye-outline", 96, theme.White.Alpha(0.22))
		})
	}
}
