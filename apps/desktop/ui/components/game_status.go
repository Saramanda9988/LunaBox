package components

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// GameStatuses 是原版 statusConfig 的顺序。
var GameStatuses = []enums.GameStatus{
	enums.StatusNotStarted, enums.StatusWantToPlay, enums.StatusPlaying,
	enums.StatusCompleted, enums.StatusOnHold, enums.StatusDropped,
}

// GameStatusLabel 是状态的文案（common.notStarted 等），按当前语言。
func GameStatusLabel(s enums.GameStatus) string {
	switch s {
	case enums.StatusWantToPlay:
		return locales.CommonWantToPlay()
	case enums.StatusPlaying:
		return locales.CommonPlaying()
	case enums.StatusCompleted:
		return locales.CommonCompleted()
	case enums.StatusOnHold:
		return locales.CommonOnHold()
	case enums.StatusDropped:
		return locales.CommonDropped()
	}
	return locales.CommonNotStarted()
}

// GameStatusStyle 对应 statusConfig：状态的图标与胶囊颜色（游戏库批量菜单、详情页状态按钮共用）。
func GameStatusStyle(t *theme.Theme, s enums.GameStatus) (icon string, bg, fg ui.Color) {
	switch s {
	case enums.StatusWantToPlay: // info
		return "bookmark-outline", t.Pick(ui.Hex("#e0f2fe"), ui.Hex("#0c181d")), t.Pick(ui.Hex("#0369a1"), ui.Hex("#7dd3fc"))
	case enums.StatusPlaying: // neutral
		return "gamepad-variant", t.Pick(theme.Neutral100, ui.Hex("#0f172a")), t.Pick(theme.Neutral700, theme.Neutral300)
	case enums.StatusCompleted: // yellow
		return "trophy", t.Pick(ui.Hex("#fef9c3"), ui.Hex("#713f12")), t.Pick(ui.Hex("#a16207"), ui.Hex("#fde047"))
	case enums.StatusOnHold: // orange
		return "pause-circle-outline", t.Pick(ui.Hex("#ffedd5"), ui.Hex("#7c2d12")), t.Pick(ui.Hex("#c2410c"), ui.Hex("#fdba74"))
	case enums.StatusDropped: // rose
		return "delete-outline", t.Pick(ui.Hex("#ffe4e6"), ui.Hex("#881337")), t.Pick(ui.Hex("#be123c"), ui.Hex("#fda4af"))
	}
	// gray
	return "clock-outline", t.Pick(ui.Hex("#f3f4f6"), ui.Hex("#374151")), t.Pick(ui.Hex("#374151"), ui.Hex("#d1d5db"))
}
