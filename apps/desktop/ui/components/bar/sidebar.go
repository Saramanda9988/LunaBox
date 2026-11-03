package bar

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

const (
	sidebarOpenWidth   = 256 // 16rem
	sidebarClosedWidth = 64  // 4rem
)

// Sidebar 是侧边栏的展开状态。
type Sidebar struct {
	Open        bool
	delayedOpen bool      // 底部按钮的布局，在淡出一半时才切换
	fadeAt      time.Time // 切换开始的时间
	toggled     bool      // 刚切换，下一次构建时以 c.Now() 作为 fadeAt
	width       float32   // 当前动画中的宽度

	Cloud CloudSync // 底部云同步按钮的状态
}

// NewSidebar 按配置的 sidebar_open 创建状态。
func NewSidebar(open bool) Sidebar {
	return Sidebar{Open: open, delayedOpen: open, width: sidebarWidth(open)}
}

// Toggle 展开或收起。
func (s *Sidebar) Toggle() {
	s.Open = !s.Open
	s.toggled = true
}

// SetOpen 立即切换到目标状态，不播放动画（截图调试用）。
func (s *Sidebar) SetOpen(open bool) {
	s.Open, s.delayedOpen, s.fadeAt, s.toggled, s.width = open, open, time.Time{}, false, sidebarWidth(open)
}

// Width 是当前帧（动画中）的宽度，主内容区据此计算网格列数。
func (s *Sidebar) Width() float32 {
	if s.width == 0 {
		return sidebarWidth(s.Open)
	}
	return s.width
}

func sidebarWidth(open bool) float32 {
	if open {
		return sidebarOpenWidth
	}
	return sidebarClosedWidth
}

type navItem struct {
	path  string
	label func() string // 每帧按当前语言取
	icon  string
}

var navItems = []navItem{
	{env.PathHome, locales.SideBarHome, "home"},
	{env.PathLibrary, locales.SideBarLibrary, "gamepad-variant"},
	{env.PathStats, locales.SideBarStats, "chart-bar"},
	{env.PathCategories, locales.SideBarCategories, "format-list-bulleted"},
}

// SideBarProps 是侧边栏的输入。导航用 Router：当前路径决定高亮，点击 Push 对应路径。
type SideBarProps struct {
	State  *Sidebar
	Router *ui.Router
	Config *appconf.AppConfig // 云同步状态，可为 nil
	Env    *env.Env           // 立即同步用的服务与提示；测试里可为 nil
}

// SideBar 对应 SideBar.tsx。开启自定义背景时底色是 rgba(var(--sidebar-bg-rgb), bgOpacity)，
// 右边框 border-white/20 dark:border-white/10（没有 backdrop-blur）。
func SideBar(c *ui.Context, t *theme.Theme, p SideBarProps) {
	s := p.State
	now := c.Now()

	bg, border := t.SidebarBg, t.SidebarBorder
	if t.Glass {
		bg, border = t.SidebarBg.Alpha(t.BgOpacity), t.Pick(theme.White.Alpha(0.2), theme.White.Alpha(0.1))
	}
	aside := ui.Column(c).Shrink(0).Background(bg).BorderWidth(0, t.HL, 0, 0).BorderColor(border)
	// style={{ transition: "width 300ms ease" }}
	s.width = aside.AnimateWith("width", sidebarWidth(s.Open), 300*time.Millisecond, theme.EaseCSS)
	aside.Width(s.width)

	// transition-all duration-300 的标签/标题，展开程度 0..1
	reveal := aside.AnimateWith("reveal", theme.Bool(s.Open), 300*time.Millisecond, theme.EaseTailwind)

	// 底部按钮：先淡出 150ms，再切换布局并淡入（SideBar.tsx 的 isFading/delayedOpen）。
	fadeOpacity := float32(1)
	if s.toggled {
		s.fadeAt, s.toggled = now, false
	}
	if !s.fadeAt.IsZero() {
		since := now.Sub(s.fadeAt)
		if since < 150*time.Millisecond {
			fadeOpacity = 0
			c.After(150*time.Millisecond - since)
		} else {
			s.delayedOpen = s.Open
			s.fadeAt = time.Time{}
		}
	}

	aside.Children(func() {
		sidebarHeader(c, t, s, reveal)
		ui.Column(c).Grow(1).Padding(16, 8).Gap(8).Children(func() {
			for _, item := range navItems {
				navLink(c, t, p.Router, item, reveal)
			}
		})
		sidebarFooter(c, t, p, fadeOpacity)
	})
}

func sidebarHeader(c *ui.Context, t *theme.Theme, s *Sidebar, reveal float32) {
	ui.Row(c).Height(64).PaddingX(12).Shrink(0).Children(func() {
		title := assets.Title(t.Dark)
		tw, th := title.Size()
		// opacity-100 max-w-[200px] pl-1 ↔ opacity-0 max-w-0 pl-0
		ui.Row(c).Gap(4).Clip().MaxWidth(200*reveal).Padding(0, 0, 0, 4*reveal).Opacity(reveal).Shrink(1).Children(func() {
			ui.Image(c, assets.AppIcon(t.Dark)).Size(32, 32).Shrink(0)
			ui.Image(c, title).Size(24*float32(tw)/float32(th), 24).Shrink(0)
		})
		justify := ui.Center
		if s.Open {
			justify = ui.End
		}
		ui.Row(c).Grow(1).MinWidth(40).Justify(justify).Children(func() {
			btn := better.IconButton(c, t, "toggle", "menu", 8, false, true)
			btn.Label(locales.SideBarToggle()).OnClick(s.Toggle)
		})
	})
}

// active 与 TanStack Router 的 activeProps 一致：完整匹配，或在其下的子路径（首页只完整匹配）。
func active(r *ui.Router, path string) bool {
	cur := r.Path()
	if path == env.PathHome {
		return cur == path
	}
	return cur == path || len(cur) > len(path) && cur[:len(path)] == path && cur[len(path)] == '/'
}

func navLink(c *ui.Context, t *theme.Theme, r *ui.Router, item navItem, reveal float32) {
	on := active(r, item.path)
	label := item.label()
	link := ui.ButtonBase(c.Key(item.path)).Padding(8).Radius(12).AlignItems(ui.Center).
		Justify(ui.Start).FocusRing(false).Label(label)
	color := t.NavText
	switch {
	case on:
		link.Background(t.IfGlass(t.NavActiveBg, theme.White.Alpha(0.2), theme.Black.Alpha(0.2)))
		color = t.NavActive
	case link.Hovered():
		link.Background(t.IfGlass(t.NavHover, theme.White.Alpha(0.1), theme.Black.Alpha(0.1)))
	}
	link.Transition(theme.ColorsTransition(150 * time.Millisecond))
	color = link.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	link.OnClick(func() { r.Push(item.path) }).Children(func() {
		ui.Box(c).Size(32, 32).Center().Shrink(0).Children(func() {
			components.Icon(c, item.icon, 20, color)
		})
		// w-[120px] ml-2 opacity-100 ↔ w-0 ml-0 opacity-0
		ui.Row(c).Width(120*reveal).Margin(0, 0, 0, 8*reveal).Opacity(reveal).Clip().Shrink(0).Children(func() {
			components.Text(c, label, theme.TextBase, theme.WeightNormal, color).NoWrap().Shrink(0)
		})
	})
}

func sidebarFooter(c *ui.Context, t *theme.Theme, p SideBarProps, opacity float32) {
	ui.Box(c).Padding(16).Shrink(0).Children(func() {
		var row ui.Element
		if p.State.delayedOpen {
			row = ui.Row(c).Justify(ui.End).Gap(4)
		} else {
			row = ui.Column(c).AlignItems(ui.Center).Gap(8)
		}
		op := row.AnimateWith("fade", opacity, 150*time.Millisecond, theme.EaseTailwind)
		row.Opacity(op).Children(func() {
			cloudSyncButton(c, t, p)

			dl := better.IconButton(c, t, "downloads", "download", 10, active(p.Router, env.PathDownloads), true)
			dl.Label(locales.DownloadsTitle()).OnClick(func() { p.Router.Push(env.PathDownloads) })

			st := better.IconButton(c, t, "settings", "cog", 10, active(p.Router, env.PathSettings), true)
			st.Label(locales.SettingsTitle()).OnClick(func() { p.Router.Push(env.PathSettings) })
		})
	})
}

// cloudSyncButton 对应侧边栏的云同步按钮（点击立即同步，useCloudSync 的 handleSyncNow）与悬停面板。
func cloudSyncButton(c *ui.Context, t *theme.Theme, p SideBarProps) {
	cfg := p.Config
	s := &p.State.Cloud
	serviceOn := cfg != nil && cfg.CloudBackupEnabled
	syncOn := serviceOn && cfg.CloudSyncEnabled

	btn := ui.ButtonBase(c.Key("cloud")).Padding(10).Radius(12).Center().FocusRing(false).Label(locales.SideBarCloudSync())
	s.track(p.Env, cfg, syncOn && btn.Hovered())
	st := s.effective(cfg)
	busy := s.busy(st)
	configured := s.configured(cfg, st)

	name, color := "cloud-sync-outline", t.NavText
	switch {
	case !serviceOn:
		name, color = "cloud-off-outline", t.TextFaint
	case !syncOn:
		name, color = "cloud-outline", t.TextFaint
	case busy:
		name = "loading"
	case st.LastSyncStatus == "success":
		name, color = "cloud-check-outline", theme.Success500
	case st.LastSyncStatus == "failed":
		name, color = "cloud-alert-outline", theme.Error500
	}

	if syncOn && btn.Hovered() {
		btn.Background(t.IfGlass(t.NavHover, theme.White.Alpha(0.1), theme.Black.Alpha(0.1)))
	}
	if !s.canSyncNow(cfg) {
		btn.Opacity(0.75) // canSyncNow 为假时 opacity-75
	}
	if syncOn {
		btn.OnClick(func() { s.syncNow(c, p.Env, cfg) })
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		if name == "loading" {
			components.Spinner(c, 20, color)
		} else {
			components.Icon(c, name, 20, color)
		}
	})
	if !syncOn {
		return
	}
	show := btn.AnimateWith("panel", theme.Bool(btn.Hovered()), 200*time.Millisecond, theme.EaseTailwind)
	if show == 0 {
		return
	}
	ui.Overlay(c, func() {
		// glass-panel，data-glass:bg-white/78 data-glass:dark:bg-black/42
		_, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand200.Alpha(0.8), theme.Brand700.Alpha(0.8)))
		panel := ui.Column(c).Width(176).Padding(10).Gap(8).Radius(8).
			Background(t.IfGlass(t.Pick(theme.White.Alpha(0.92), theme.Brand900.Alpha(0.88)), theme.White.Alpha(0.78), theme.Black.Alpha(0.42))).
			Border(t.HL, border).Opacity(show).PassThrough()
		if !t.Glass {
			panel.Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
		}
		if p.State.Open {
			// bottom-full left-1/2 mb-3 -translate-x-1/2，悬停时 translate-y-2 → 0
			panel.AttachTo(btn, ui.AnchorTop, ui.AnchorBottom).Translate(0, -12+8*(1-show))
		} else {
			panel.AttachTo(btn, ui.AnchorBottomRight, ui.AnchorBottomLeft).Translate(12+8*(1-show), 0)
		}
		small := theme.FontStep{Size: 10, Line: 15}
		tiny := theme.FontStep{Size: 9, Line: 13.5}
		panel.Children(func() {
			// flex flex-col items-start gap-1.5
			ui.Column(c).Gap(6).AlignItems(ui.Start).Children(func() {
				components.Text(c, locales.SideBarCloudSync(), small, theme.WeightMedium, t.TextMuted).NoWrap()
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if !configured {
						components.Text(c, locales.SettingsCloudBackupSyncNotConfigured(), tiny, theme.WeightMedium, t.Pick(theme.Warning600, theme.Warning400)).Shrink(1)
					}
					label, bg, fg, ring := cloudStatus(st.LastSyncStatus, t)
					components.Text(c, label, tiny, theme.WeightSemibold, fg).Padding(2, 6).Radius(999).Background(bg).Border(1, ring)
				})
			})
			if busy {
				// m-0 text-[10px] font-medium leading-4 text-brand-600 dark:text-brand-300
				components.Text(c, cloudSyncDetail(st), theme.FontStep{Size: 10, Line: 16}, theme.WeightMedium, t.Pick(theme.Brand600, theme.Brand300))
			}
			ui.Column(c).Gap(4).Children(func() {
				components.Text(c, locales.SettingsCloudBackupSyncLastTimeLabel(), small, theme.WeightNormal, t.TextFaint).NoWrap()
				when := formatCloudTime(st.LastSyncTime, timefmt.Location(cfg))
				components.Text(c, when, small, theme.WeightMedium, t.Text).NoWrap()
			})
			if st.LastSyncError != "" {
				// mt-0.5 max-w-[12rem] text-[9px] leading-3 text-error-600 dark:text-error-400
				components.Text(c, st.LastSyncError, theme.FontStep{Size: 9, Line: 12}, theme.WeightNormal, t.Pick(theme.Error600, theme.Error400)).
					Margin(2, 0, 0, 0).MaxWidth(192)
			}
		})
	})
}

// cloudStatus 对应 utils/cloudSync.ts 的 getCloudSyncStatusLabel 与 getCloudSyncStatusClass（ring-1 画成 1px 边框）。
func cloudStatus(status string, t *theme.Theme) (label string, bg, fg, ring ui.Color) {
	switch status {
	case "success":
		return locales.SettingsCloudBackupSyncStatusSuccess(), t.Pick(theme.Success100, theme.Success900.Alpha(0.4)), t.Pick(theme.Success700, theme.Success300), t.Pick(theme.Success200, theme.Success800.Alpha(0.6))
	case "failed":
		return locales.SettingsCloudBackupSyncStatusFailed(), t.Pick(theme.Error100, theme.Error900.Alpha(0.4)), t.Pick(theme.Error700, theme.Error300), t.Pick(theme.Error200, theme.Error800.Alpha(0.6))
	case "syncing":
		return locales.SettingsCloudBackupSyncStatusSyncing(), t.Pick(theme.Warning100, theme.Warning900.Alpha(0.4)), t.Pick(theme.Warning700, theme.Warning300), t.Pick(theme.Warning200, theme.Warning800.Alpha(0.6))
	}
	return locales.SettingsCloudBackupSyncStatusIdle(), t.Pick(theme.Brand100, theme.Brand700.Alpha(0.7)), t.Pick(theme.Brand700, theme.Brand200), t.Pick(theme.Brand200, theme.Brand600.Alpha(0.7))
}

// formatCloudTime 对应 formatCloudSyncTime：没有同步过时是 syncNever，否则是不带秒的本地日期时间。
func formatCloudTime(s string, loc *time.Location) string {
	if s == "" {
		return locales.SettingsCloudBackupSyncNever()
	}
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return timefmt.DateTimeMinutes(ts, loc)
}
