package library

import (
	"log"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// missingPaths 是批量模式下“检查失效路径”的状态：扫描后游戏库只显示启动路径已不存在的游戏，并全部选中，
// 用户确认后用现有的批量操作处理（多数是删除改名后重新入库留下的旧条目）。
type missingPaths struct {
	ids      []string // 非空时只显示这些游戏
	scanning bool
}

func (m *missingPaths) active() bool { return len(m.ids) > 0 }

// listKey 附加到查询条件的标识上，扫描结果不同时重新请求。
func (m *missingPaths) listKey() string {
	if !m.active() {
		return ""
	}
	return "|ids:" + strings.Join(m.ids, ",")
}

// scanMissingPaths 检查全部游戏的启动路径；有失效的游戏时清除搜索与筛选（保留排序），只显示并选中它们。
// 所在磁盘不可用的游戏无法判断，只提示数量，不选中。
func (l *Page) scanMissingPaths(c *ui.Context) {
	if l.missing.scanning || l.env.Svc == nil {
		return
	}
	l.missing.scanning = true
	svc := l.env.Svc.Game
	l.env.Async(func() func() {
		result, err := svc.FindGamesWithMissingPath()
		return func() {
			l.missing.scanning = false
			if err != nil {
				log.Printf("library: find games with missing path: %v", err)
				l.env.Toast(c, env.Error(locales.NativeMissingPathScanFailed()))
				return
			}
			if n := len(result.Unavailable); n > 0 {
				l.env.Toast(c, env.Info(locales.NativeMissingPathUnavailable(strconv.Itoa(n))))
			}
			if len(result.Missing) == 0 {
				l.env.Toast(c, env.Info(locales.NativeMissingPathNone()))
				return
			}
			ids := make([]string, 0, len(result.Missing))
			clear(l.selected)
			for _, g := range result.Missing {
				ids = append(ids, g.GameID)
				l.selected[g.GameID] = true
			}
			l.missing.ids = ids
			l.filters.ClearConditions()
			l.Reload()
			l.env.Toast(c, env.Success(locales.NativeMissingPathFound(strconv.Itoa(len(ids)))))
		}
	})
}

// exitMissingPaths 回到全部游戏；不在“失效路径”视图里时什么也不做。
func (l *Page) exitMissingPaths() {
	if !l.missing.active() {
		return
	}
	l.missing.ids = nil
	l.Reload()
}

// missingPathsBanner 是“失效路径”视图的提示条：说明当前只显示哪些游戏，右侧回到全部游戏。
func (l *Page) missingPathsBanner(c *ui.Context, t *theme.Theme) {
	if !l.missing.active() {
		return
	}
	color := t.Pick(theme.Warning800, theme.Warning200)
	banner := ui.Row(c.Key("missing-paths")).AlignItems(ui.Center).Gap(8).Padding(8, 12).Radius(8).
		Border(t.HL, t.Pick(theme.Warning300, theme.Warning700.Alpha(0.6))).
		Background(t.Pick(theme.Warning50, theme.Warning900.Alpha(0.35)))
	banner.Children(func() {
		components.Icon(c, "folder-alert-outline", 18, color)
		components.Text(c, locales.NativeMissingPathFilter(strconv.Itoa(len(l.missing.ids))), theme.TextSM, theme.WeightMedium, color).Grow(1)
		label := locales.NativeMissingPathFilterExit()
		btn := ui.ButtonBase(c.Key("exit")).Padding(4, 10).Radius(6).Gap(4).AlignItems(ui.Center).FocusRing(false).Label(label).
			Border(t.HL, t.Pick(theme.Warning300, theme.Warning700))
		if btn.Hovered() {
			btn.Background(t.Pick(theme.Warning100, theme.Warning800.Alpha(0.5)))
		}
		btn.OnClick(l.exitMissingPaths).Children(func() {
			components.Icon(c, "close", 14, color)
			components.Text(c, label, theme.TextXS, theme.WeightMedium, color).NoWrap()
		})
	})
}
