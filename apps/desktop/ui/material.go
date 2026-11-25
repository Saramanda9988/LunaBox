package ui

import (
	"log"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop/ui/theme"
)

// MaterialVibrancy 把主题插件请求的侧边栏材质（material.sidebar）换成 MyGo 的窗口材质。
// Windows 11 22H2 起有 Acrylic（磨砂）与 Mica（云母）；macOS 都用系统侧边栏材质（本身就是跟随深浅色的磨砂）；
// Linux 没有材质，设置了也不显示，界面按 c.Vibrancy() 回退到不透明底色。
func MaterialVibrancy(kind string) mygo.Vibrancy {
	switch kind {
	case theme.MaterialAcrylic, theme.MaterialMica:
	default:
		return mygo.VibrancyNone
	}
	if runtime.GOOS == "darwin" {
		return mygo.VibrancySidebar
	}
	if kind == theme.MaterialMica {
		return mygo.VibrancyMica
	}
	return mygo.VibrancyAcrylic
}

// CreationVibrancy 返回主窗口创建时要带上的材质。Windows 只能开关创建时就带材质的窗口（创建时决定窗口经
// DirectComposition 呈现），所以按上次运行记下的 cfg.WindowMaterial 创建：没用过材质主题的用户窗口保持原样。
// macOS 随时可以设置材质，创建时不带。
func CreationVibrancy(cfg *appconf.AppConfig) mygo.Vibrancy {
	if runtime.GOOS != "windows" || cfg == nil {
		return mygo.VibrancyNone
	}
	return MaterialVibrancy(cfg.WindowMaterial)
}

// syncMaterial 让主窗口的材质跟随主题插件的 material.sidebar，窗口真正显示出材质时置 t.MaterialOn
// （根布局不画底色，侧边栏改画插件的着色）。covered 为 true 时有背景图铺满窗口、会盖住材质，不启用。
//
// 插件还没加载完时沿用上次运行记下的材质，免得启动时先撤掉再装上；加载完后把插件请求的材质记进配置，
// 下次启动时 Windows 按它创建窗口（CreationVibrancy）。所以 Windows 上第一次启用材质主题要重启一次才显示，
// 之后开关即时生效。
func (a *App) syncMaterial(c *ui.Context, t *theme.Theme, cfg *appconf.AppConfig, covered bool) {
	started := true
	if a.env.Svc != nil && a.env.Svc.Plugin != nil {
		started = a.env.Svc.Plugin.Started()
	}
	requested := t.SidebarMaterial
	if !started && cfg != nil {
		requested = cfg.WindowMaterial
	}
	want := requested
	if covered {
		want = ""
	}
	if win := a.env.Win; win != nil {
		if v := MaterialVibrancy(want); v != a.vibrancy {
			a.vibrancy = v
			win.SetVibrancy(v)
			c.After(100 * time.Millisecond) // c.Vibrancy() 在窗口换好材质之后的那一帧才更新
		}
	}
	t.MaterialOn = want != "" && c.Vibrancy()

	// 每次运行对同一个请求只保存一次，保存失败不逐帧重试（下次启动加载完插件后会再记）
	if started && cfg != nil && cfg.WindowMaterial != requested && !(a.materialSaving && a.materialSaved == requested) {
		a.materialSaved, a.materialSaving = requested, true
		if err := a.env.PatchConfig(func(cfg *appconf.AppConfig) { cfg.WindowMaterial = requested }); err != nil {
			log.Printf("save window material failed: %v", err)
		}
	}
}
