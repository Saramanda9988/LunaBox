package ui

import (
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// routes 是路由表，对应原版 frontend/src/routes/*.tsx。侧边栏用 Router.Push 切换页面；
// Alt+←/→ 与鼠标侧键前进后退由 ui.Router 处理。
func (a *App) routes(c *ui.Context) ui.Element {
	// 原版每次进入详情页都重新挂载组件、重新加载数据；一帧可能构建多次，只在位置变化时加载一次。
	loc := a.router.Location()
	entered := loc != a.location
	if entered {
		a.detail.Leave()   // 离开详情页前补存未保存的修改
		a.settings.Flush() // 离开设置页前补存还没保存的草稿
	}
	a.location = loc
	return a.router.View(c, func(r *ui.Route) {
		switch {
		case r.Match(env.PathHome):
			if entered {
				a.home.Open()
			}
			r.Title(locales.HomeTitle())
			a.home.View(c)
		case r.Match(env.PathLibrary):
			r.Title(locales.LibraryTitle())
			a.library.View(c)
		case r.Match(env.PathCategories):
			if entered {
				a.categories.Open(c.Now())
			}
			r.Title(locales.CategoriesTitle())
			a.categories.View(c)
		case r.Match(env.PathCategory):
			if entered {
				a.category.Open(r.Param("id"), c.Now())
			}
			r.Title(a.category.Title())
			a.category.View(c)
		case r.Match(env.PathStats):
			if entered {
				a.stats.Open(c.Now())
			}
			r.Title(locales.StatsTitle())
			a.stats.View(c)
		case r.Match(env.PathDownloads):
			if entered {
				a.downloads.Open()
			}
			r.Title(locales.DownloadsTitle())
			a.downloads.View(c)
		case r.Match(env.PathSettings):
			if entered {
				a.settings.Open()
			}
			r.Title(locales.SettingsTitle())
			a.settings.View(c)
		case r.Match(env.PathGame):
			if entered {
				a.detail.Open(r.Param("id"), c.Now())
			}
			r.Title(a.detail.Title())
			a.detail.View(c)
		default:
			r.Title(locales.NativeNotFound())
			a.notFoundPage(c, locales.NativeNotFoundPage(r.Path()))
		}
	})
}

func (a *App) notFoundPage(c *ui.Context, title string) {
	t := a.env.Theme
	ui.Column(c).Fill().Center().Children(func() {
		components.Text(c, title, theme.TextXL, theme.WeightBold, t.TextStrong)
	})
}
