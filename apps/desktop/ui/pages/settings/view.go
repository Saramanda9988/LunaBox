package settings

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/ai"
	"lunabox/desktop-mygo/ui/components/panel/appdata"
	"lunabox/desktop-mygo/ui/components/panel/autobackup"
	"lunabox/desktop-mygo/ui/components/panel/background"
	"lunabox/desktop-mygo/ui/components/panel/basic"
	"lunabox/desktop-mygo/ui/components/panel/cloudbackup"
	"lunabox/desktop-mygo/ui/components/panel/dbbackup"
	"lunabox/desktop-mygo/ui/components/panel/fullbackup"
	"lunabox/desktop-mygo/ui/components/panel/gamesettings"
	"lunabox/desktop-mygo/ui/components/panel/metadata"
	"lunabox/desktop-mygo/ui/components/panel/portable"
	"lunabox/desktop-mygo/ui/components/panel/proxy"
	"lunabox/desktop-mygo/ui/components/panel/update"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

const (
	pagePadding     = 32   // p-8
	sectionMaxWidth = 1024 // max-w-5xl
	githubURL       = "https://github.com/Saramanda9988/LunaBox"
)

// section 是一个 CollapsibleSection：标题、图标与面板。
type section struct {
	id    sectionID
	key   string
	title func() string
	icon  string
	view  func(c *ui.Context, t *theme.Theme, p panel.Settings)
}

// sections 是 routes/settings.tsx 里显示的各分区，顺序与图标同原版。
func (p *Page) sections() []section { return p.sectionList(false) }

// sectionList 返回各分区；all 为 false 时按构建模式去掉“本地集成设置”。
func (p *Page) sectionList(all bool) []section {
	s := &p.panels
	list := []section{
		{sectionBasic, "basic", locales.SettingsSectionsBasic, "database-settings", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			basic.View(c, t, &s.basic, pp)
		}},
		{sectionMetadata, "metadata", locales.SettingsSectionsMetadata, "database-search", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			metadata.View(c, t, &s.metadata, pp)
		}},
		{sectionAppearance, "appearance", locales.SettingsSectionsAppearance, "palette", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			background.View(c, t, &s.background, pp)
		}},
		{sectionGame, "game", locales.SettingsSectionsGame, "timer-play-outline", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			gamesettings.View(c, t, &s.game, pp)
		}},
		{sectionProxy, "proxy", locales.SettingsSectionsProxy, "lan-connect", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			proxy.View(c, t, &s.proxy, pp)
		}},
		{sectionCloudBackup, "cloud-backup", locales.SettingsSectionsCloudBackup, "cloud-upload", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			cloudbackup.View(c, t, &s.cloudBackup, pp)
		}},
		{sectionAutoBackup, "auto-backup", locales.SettingsSectionsAutoBackup, "backup-restore", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			autobackup.View(c, t, &s.autoBackup, pp)
		}},
		{sectionAI, "ai", locales.SettingsSectionsAi, "robot-happy", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			ai.View(c, t, &s.ai, pp)
		}},
		{sectionDBBackup, "db-backup", locales.SettingsSectionsDbBackup, "database-refresh", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			dbbackup.View(c, t, &s.dbBackup, pp)
		}},
		{sectionFullDataBackup, "full-data-backup", locales.SettingsSectionsFullDataBackup, "package-variant", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			fullbackup.View(c, t, &s.fullBackup, pp)
		}},
		{sectionUpdate, "update", locales.SettingsSectionsUpdate, "update", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			update.View(c, t, &s.update, pp)
		}},
		{sectionAppData, "app-data", locales.SettingsSectionsAppData, "folder-cog-outline", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			appdata.View(c, t, &s.appData, pp)
		}},
	}
	// 便携版与 AppImage 才有“本地集成设置”。
	if mode := p.version["buildMode"]; all || mode == "portable" || mode == "appimage" {
		list = append(list, section{sectionPortableSetup, "portable-setup", locales.SettingsSectionsPortableSetup, "toolbox-outline", func(c *ui.Context, t *theme.Theme, pp panel.Settings) {
			portable.View(c, t, &s.portable, pp)
		}})
	}
	return list
}

// View 构建页面：mx-auto max-w-8xl space-y-6 p-8 的标题、max-w-5xl space-y-6 的各分区与页脚
// （max-w-8xl 在 UnoCSS 里没有定义，不限宽）。
// 页面在 <main class="overflow-auto"> 里滚动，内容超出时才占用滚动条的宽度。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	if p.failed != nil {
		p.env.Toast(c, env.Error(p.failed()))
		p.failed = nil
	}
	if wait := p.tick(c.Now()); wait > 0 {
		c.After(wait)
	}
	if !p.loaded {
		p.preparing(c, t)
		return
	}
	p.debugScrollTo(c)

	gutter := float32(0)
	if p.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	pageW := p.env.MainWidth - gutter
	contentW := pageW - 2*pagePadding
	props := p.props()
	components.ScrollArea(c, t, "settings-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(0, gutter, 0, 0) // 滚动条占用内容盒右侧的宽度，mx-auto 在剩下的宽度里居中
		sc.Children(func() {
			ui.Column(c.Key("page")).Width(pageW).AlignSelf(ui.Center).Padding(pagePadding).Gap(24).Children(func() {
				// flex items-center justify-between：text-4xl font-bold text-brand-900 dark:text-white
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					components.Text(c, locales.SettingsTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
				})
				ui.Column(c.Key("sections")).Width(min(contentW, sectionMaxWidth)).AlignSelf(ui.Center).Gap(24).Children(func() {
					for _, sec := range p.sections() {
						components.CollapsibleSection(c, t, sec.key, components.CollapsibleSectionProps{
							Title: sec.title(), Icon: sec.icon, Open: &p.open[sec.id],
						}, func() { sec.view(c, t, props) }).Bind(&p.handles[sec.id])
					}
				})
				p.footer(c, t)
			})
		})
	})
}

// debugScrollTo 把 SetDebugState 指定的分区滚到主内容区顶部下方 24px 处。前几帧的布局可能还是缩放 1 的（细线 1px），
// 所以打开后 1.5 秒内每帧都重新对齐，之后不再干预滚动。
func (p *Page) debugScrollTo(c *ui.Context) {
	if !p.debugScroll {
		return
	}
	now := c.Now()
	if p.debugUntil.IsZero() {
		p.debugUntil = now.Add(1500 * time.Millisecond)
	}
	if now.After(p.debugUntil) {
		p.debugScroll = false
		return
	}
	c.AnimationFrame()
	for _, sec := range p.sections() {
		if sec.key != p.debug {
			continue
		}
		if b := p.handles[sec.id].Bounds(c); b.H > 0 && p.scroll.State.MaxY > 0 {
			p.scroll.State.Y = max(0, min(p.scroll.State.Y+b.Y-(env.TopbarHeight+24), p.scroll.State.MaxY))
		}
	}
}

// preparing 是读不到配置时的占位：flex min-h-[80vh] flex-col items-center justify-center space-y-4 text-brand-500，
// i-mdi-cog-outline animate-spin-slow text-6xl 与 text-xl 的“正在准备设置...”。
func (p *Page) preparing(c *ui.Context, t *theme.Theme) {
	_, winH := c.Size()
	ui.Column(c).FillWidth().MinHeight(winH * 0.8).Center().Gap(16).Children(func() {
		ic := components.Icon(c, "cog-outline", 60, theme.Brand500)
		ic.Rotate(ic.Loop("spin-slow", 3*time.Second, ui.Linear) * 360)
		components.Text(c, locales.SettingsPreparingSettings(), theme.TextXL, theme.WeightNormal, theme.Brand500)
	})
}

// footer：pt-4 pb-8 text-center text-brand-500 dark:text-brand-400 flex flex-col items-center，署名、版本与 GitHub 按钮。
func (p *Page) footer(c *ui.Context, t *theme.Theme) {
	color := t.Pick(theme.Brand500, theme.Brand400)
	ui.Column(c.Key("footer")).FillWidth().Padding(16, 0, 32, 0).AlignItems(ui.Center).Children(func() {
		components.Text(c, "Lunabox made with LunaRain_079 & Contributors.", theme.TextXS, theme.WeightNormal, color) // i18n:keep：原版写死的署名
		if v := p.version; v != nil {
			// mt-1 text-xs opacity-80。原版 JSX 的换行让 "Built at" 与时间之间没有空格。
			line := "Version " + v["version"] + " (" + v["commit"] + ") | " + v["buildMode"] + " | Built at" + v["buildTime"] // i18n:keep：原版写死的英文
			components.TextMargin(c, line, theme.TextXS, theme.WeightNormal, color, 4, 0, 0, 0).Opacity(0.8)
		}
		p.githubButton(c, t, color)
	})
}

// githubButton：mt-6 flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium border border-brand-200
// dark:border-brand-700/80 transition-colors，hover:bg-brand-100 hover:text-brand-800 dark:hover:bg-brand-800
// dark:hover:text-brand-100；左侧 i-mdi-github text-xl。
func (p *Page) githubButton(c *ui.Context, t *theme.Theme, color ui.Color) {
	btn := ui.ButtonBase(c.Key("github")).Margin(24, 0, 0, 0).Padding(8, 16).Radius(12).Gap(8).AlignItems(ui.Center).
		FocusRing(false).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700.Alpha(0.8))).Label(locales.SettingsGithub())
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100, theme.Brand800))
		color = t.Pick(theme.Brand800, theme.Brand100)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	btn.OnClick(func() { c.OpenURL(githubURL) })
	btn.Children(func() {
		components.Icon(c, "github", 20, color)
		components.Text(c, locales.SettingsGithub(), theme.TextSM, theme.WeightMedium, color).NoWrap()
	})
}
