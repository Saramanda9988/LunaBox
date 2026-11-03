package account

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// 断点按窗口宽度（Tailwind 的 sm、lg）。
const (
	breakpointSM = 640
	breakpointLG = 1024
)

// badgeText 是 text-[11px]：只设字号，行高继承根元素的 1.5。
var badgeText = theme.FontStep{Size: 11, Line: 16.5}

// view 是一帧里卡片要显示的内容（组件函数体开头算出的 auth、displayName 等）。
type view struct {
	auth        authStatus
	displayName string
	username    string
	avatarURL   string
	letter      string // avatarFallback
	authorized  bool
	showProfile bool // shouldShowProfile
	loading     bool // isStatusLoading || isProfileLoading
}

func (s *State) viewOf(p panel.Settings, sp *spec, cs *cardState) view {
	auth := sp.mergeAuth(p.Config, cs.snapshot)
	var pr profileView
	if cs.profile != nil {
		pr = *cs.profile
	}
	v := view{
		auth:        auth,
		displayName: firstNonEmpty(pr.nickname, pr.username, auth.identity),
		username:    firstNonEmpty(pr.username, auth.identity),
		avatarURL:   firstNonEmpty(append([]string{pr.avatarURL, auth.avatarURL}, pr.avatarExtra...)...),
		authorized:  auth.state == authAuthorized,
		loading:     cs.statusLoading || cs.profileLoading,
	}
	v.showProfile = v.authorized && v.displayName != ""
	v.letter = sp.letter
	if r, _ := utf8.DecodeRuneInString(strings.TrimSpace(v.displayName)); r != utf8.RuneError {
		v.letter = strings.ToUpper(string(r))
	}
	return v
}

// card 对应组件外层的 div：glass-panel relative isolate min-h-[132px] min-w-0 overflow-hidden rounded-2xl border
// transition-colors duration-200 sm:h-[190px] lg:h-[160px]；展开时 border-brand-300/90 bg-brand-50/70 shadow-sm
// dark:border-brand-600/90 dark:bg-brand-900/35，收起时 border-brand-200/80 bg-white/55 hover:border-brand-300/80
// dark:border-brand-700/80 dark:bg-brand-900/25 dark:hover:border-brand-600/80。右下角是水印 logo，里面是展开或收起的内容。
func (s *State) card(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, cs *cardState, props CardProps) {
	winW, _ := c.Size()
	card := ui.Column(c.Key("account-" + string(sp.provider))).FillWidth().MinWidth(0).Radius(16).Clip().
		AlignSelf(ui.Stretch)
	// 挂载时（第一次构建这张卡片，或离开设置页、收起分区后重新出现）读取授权状态：原版的 useEffect(refreshStatus)。
	mounted := false
	ui.Local(card, "mounted", func() bool { mounted = true; return true })
	if mounted {
		s.refreshStatus(p, sp, cs, nil)
	}

	fixed := winW >= breakpointSM
	switch {
	case winW >= breakpointLG:
		card.Height(160)
	case fixed:
		card.Height(190)
	default:
		card.MinHeight(132)
	}
	var border, bg ui.Color
	if props.Expanded {
		border = t.Pick(theme.Brand300.Alpha(0.9), theme.Brand600.Alpha(0.9))
		bg = t.Pick(theme.Brand50.Alpha(0.7), theme.Brand900.Alpha(0.35))
		if !t.Glass { // glass-panel 的 shadow-none
			card.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
		}
	} else {
		border = t.Pick(theme.Brand200.Alpha(0.8), theme.Brand700.Alpha(0.8))
		if card.Hovered() {
			border = t.Pick(theme.Brand300.Alpha(0.8), theme.Brand600.Alpha(0.8))
		}
		bg = t.Pick(theme.White.Alpha(0.55), theme.Brand900.Alpha(0.25))
	}
	bg, border = t.GlassPanel(bg, border) // 玻璃模式下 data-glass: 的底色、边框盖过展开、悬停的取值
	card.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(200 * time.Millisecond))

	// <img> pointer-events-none absolute z-0 opacity-30 dark:opacity-25：画在卡片背景之上、内容之下
	// （MyGo 的绝对定位元素画在流内元素之上，所以不用子元素）。
	wm, logo, dark, inset := loadWatermark(sp.logo.file), sp.logo, t.Dark, t.HL
	// overflow-hidden：Clip 只裁子元素，不裁自己的 Draw，卡片窄时 logo 会画出圆角边框，按 padding box 裁一次。
	card.Draw(func(pt *ui.Painter, r ui.Rect) {
		pad := ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}
		pt.Clip(pad, 16-inset, func() { paintWatermark(pt, r, inset, wm, logo, dark) })
	})

	// account-choice-content-transition：opacity 100ms ease；隐藏时 pointer-events-none。
	opacity := card.AnimateWith("content", theme.Bool(props.ContentVisible), 100*time.Millisecond, theme.EaseCSS)
	v := s.viewOf(p, sp, cs)
	card.Children(func() {
		if props.Expanded {
			s.expanded(c, t, p, sp, cs, v, fixed, winW >= breakpointLG, opacity)
		} else {
			collapsed(c, t, p, sp, v, opacity, props)
		}
		if !props.ContentVisible {
			// pointer-events-none：MyGo 的 PassThrough 不作用于子元素，用一层盖住内容的空白元素接住指针。
			ui.Box(c.Key("inert")).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		}
	})
}

// collapsed 对应收起时的 <button>：relative z-10 flex h-full min-h-[132px] w-full items-center p-3 text-left
// focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand-400，点击展开（onExpand）。
// 里面是 flex min-w-0 flex-1 items-center gap-3：头像、名称与说明（min-w-0 flex-1 space-y-1）、i-mdi-chevron-right。
func collapsed(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, v view, opacity float32, props CardProps) {
	name := sp.name()
	if v.showProfile {
		name = v.displayName
	}
	btn := ui.ButtonBase(c.Key("collapsed")).FillWidth().Grow(1).MinHeight(132).Padding(12).AlignItems(ui.Center).
		FocusRing(false).Label(name).Expanded(false).Opacity(opacity)
	if props.ContentVisible && props.OnExpand != nil {
		btn.OnClick(props.OnExpand)
	}
	if btn.FocusVisible() {
		btn.DrawOver(func(pt *ui.Painter, r ui.Rect) { pt.Stroke(r, theme.Brand400, 0, 2) })
	}
	btn.Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, t, p, sp, v)
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
				// flex min-w-0 items-center gap-2：truncate text-sm font-semibold text-brand-800 dark:text-brand-100，
				// 加载时 i-mdi-loading shrink-0 animate-spin text-brand-400。
				ui.Row(c).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
					components.Text(c, name, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100)).
						SingleLine().MinWidth(0).Shrink(1)
					if v.loading {
						components.Spinner(c, 16, theme.Brand400)
					}
				})
				// block (truncate) text-xs text-brand-500 dark:text-brand-400
				muted := t.Pick(theme.Brand500, theme.Brand400)
				switch {
				case v.showProfile:
					components.Text(c, "@"+v.username, theme.TextXS, theme.WeightNormal, muted).SingleLine()
				case v.auth.state == authNeedsReauth:
					components.Text(c, sp.text.needsReauth(), theme.TextXS, theme.WeightNormal, muted)
				default:
					components.Text(c, sp.text.unauthorized(), theme.TextXS, theme.WeightNormal, muted)
				}
			})
			components.Icon(c, "chevron-right", 18, theme.Brand400) // text-lg
		})
	})
}

// avatar 是 48px 的头像：有资料时是头像图片（rounded-2xl border border-brand-200/70 object-cover shadow-sm
// dark:border-brand-700/70）或首字母（bg-brand-200/80 text-sm font-semibold text-brand-700 dark:bg-brand-700/80
// dark:text-brand-200）；没有资料时是来源图标（overflow-hidden rounded-2xl border border-brand-200/80
// dark:border-brand-700/80，Bangumi 另有 bg-white/70 dark:bg-brand-800/70）。
func avatar(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, v view) {
	if v.showProfile {
		if v.avatarURL == "" {
			ui.Box(c.Key("avatar-letter")).Size(48, 48).Shrink(0).Radius(16).Center().
				Background(t.Pick(theme.Brand200.Alpha(0.8), theme.Brand700.Alpha(0.8))).Children(func() {
				components.Text(c, v.letter, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand700, theme.Brand200))
			})
			return
		}
		// 头像是 /local/avatars/… 的缓存或远程地址，用封面缓存在后台读取、解码。
		var bmp *ui.Bitmap
		if p.Env != nil && p.Env.Covers != nil {
			bmp = p.Env.Covers.Get(&models.Game{CoverURL: v.avatarURL})
		}
		ui.Box(c.Key("avatar-image")).Size(48, 48).Shrink(0).Radius(16).Clip().
			Border(t.HL, t.Pick(theme.Brand200.Alpha(0.7), theme.Brand700.Alpha(0.7))).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).Children(func() {
			if bmp != nil {
				ui.Image(c, bmp).Fill().Fit(ui.Cover)
			}
		})
		return
	}
	box := ui.Box(c.Key("avatar-icon")).Size(48, 48).Shrink(0).Radius(16).Clip().
		Border(t.HL, t.Pick(theme.Brand200.Alpha(0.8), theme.Brand700.Alpha(0.8)))
	if sp.iconBg {
		box.Background(t.Pick(theme.White.Alpha(0.7), theme.Brand800.Alpha(0.7)))
	}
	box.Children(func() {
		if img := assets.Provider(sp.icon, true); img != nil {
			ui.Image(c, img).Fill().Fit(ui.Cover) // h-full w-full object-cover
		}
	})
}

// expanded 对应展开时的内容：relative z-10 flex h-full flex-col gap-3 overflow-y-auto p-3。sm 以上卡片定高，
// 内容超出时滚动；更窄时卡片随内容变高。
func (s *State) expanded(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, cs *cardState, v view,
	fixed, wide bool, opacity float32) {
	content := func() {
		s.header(c, t, p, sp, cs, v, wide)
		if v.authorized {
			// h-px w-full shrink-0 bg-brand-200/50 dark:bg-brand-700/50
			ui.Box(c.Key("divider")).FillWidth().Height(1).Shrink(0).Background(t.Pick(theme.Brand200.Alpha(0.5), theme.Brand700.Alpha(0.5)))
			pushRow(c, t, p, sp)
		}
	}
	if !fixed {
		ui.Column(c.Key("expanded")).FillWidth().Grow(1).Padding(12).Gap(12).Opacity(opacity).Children(content)
		return
	}
	gutter := float32(0)
	if cs.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	components.ScrollArea(c, t, "expanded", &cs.scroll, func(w ui.Element) {
		w.FillWidth().Grow(1).Basis(0).Opacity(opacity)
	}, func(sc ui.Element) {
		sc.Children(func() {
			ui.Column(c.Key("content")).FillWidth().Padding(12, 12+gutter, 12, 12).Gap(12).Children(content)
		})
	})
}

// header：flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between。左侧 min-w-0 flex flex-1 items-center
// gap-3 的头像与名称，右侧 flex self-end gap-2 lg:self-auto 的按钮。
func (s *State) header(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, cs *cardState, v view, wide bool) {
	hdr := ui.Row(c.Key("header")).FillWidth().Gap(16)
	if wide {
		hdr.AlignItems(ui.Center).Justify(ui.SpaceBetween)
	} else {
		hdr.Column()
	}
	hdr.Children(func() {
		left := ui.Row(c.Key("identity")).MinWidth(0).Gap(12).AlignItems(ui.Center)
		if wide {
			left.Grow(1).Basis(0)
		} else {
			left.FillWidth()
		}
		left.Children(func() {
			avatar(c, t, p, sp, v)
			identity(c, t, sp, v)
		})
		actions := ui.Row(c.Key("actions")).Gap(8).Shrink(0).AlignItems(ui.Center)
		if !wide {
			actions.AlignSelf(ui.End)
		}
		actions.Children(func() { s.actions(c, t, p, sp, cs, v) })
	})
}

// identity：min-w-0 space-y-1 的名称行（flex flex-wrap items-center gap-2：名称、加载图标、需要重新授权的标签）、
// @用户名或说明，以及上次的授权错误。
func identity(c *ui.Context, t *theme.Theme, sp *spec, v view) {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	ui.Column(c.Key("names")).MinWidth(0).Shrink(1).Gap(4).Children(func() {
		ui.Row(c.Key("title")).Wrap().Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
			name := sp.name()
			if v.showProfile {
				name = v.displayName
			}
			// truncate text-sm font-semibold text-brand-800 dark:text-brand-100
			components.Text(c, name, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100)).
				SingleLine().MinWidth(0).Shrink(1)
			if v.loading {
				components.Spinner(c, 16, theme.Brand400) // i-mdi-loading animate-spin text-brand-400
			}
			if v.auth.state == authNeedsReauth {
				// rounded-full bg-warning-100 px-2 py-0.5 text-[11px] font-semibold text-warning-700
				// dark:bg-warning-900/30 dark:text-warning-300
				ui.Box(c.Key("reauth")).Padding(2, 8).Radius(999).Shrink(0).
					Background(t.Pick(theme.Warning100, theme.Warning900.Alpha(0.3))).Children(func() {
					components.Text(c, sp.text.needsReauth(), badgeText, theme.WeightSemibold, t.Pick(theme.Warning700, theme.Warning300)).NoWrap()
				})
			}
		})
		switch {
		case v.showProfile:
			components.Text(c, "@"+v.username, theme.TextXS, theme.WeightNormal, muted).SingleLine() // truncate text-xs
		case v.auth.state == authNeedsReauth:
			components.Text(c, sp.text.reconnectHint(), theme.TextXS, theme.WeightNormal, muted)
		default:
			components.Text(c, sp.text.authHint(), theme.TextXS, theme.WeightNormal, muted)
		}
		if v.auth.lastErr != "" {
			// text-xs text-warning-700 dark:text-warning-300
			components.Text(c, sp.text.lastErrorLabel()+": "+v.auth.lastErr, theme.TextXS, theme.WeightNormal,
				t.Pick(theme.Warning700, theme.Warning300))
		}
	})
}

// actions：已授权时是两个 size="sm" !rounded-full 的图标按钮（同步全部状态、断开连接），未授权时是“授权”按钮。
func (s *State) actions(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, cs *cardState, v view) {
	if v.authorized {
		better.Button(c, t, "sync-all", better.ButtonProps{
			Variant: better.ButtonSecondary, Size: better.ButtonSM, Icon: "cloud-sync-outline",
			Loading: cs.syncing, Disabled: cs.disconnecting,
		}).Radius(999).Label(sp.text.syncLabel()).OnClick(func() { cs.syncConfirm = true })
		better.Button(c, t, "disconnect", better.ButtonProps{
			Variant: better.ButtonDanger, Size: better.ButtonSM, Icon: "link-off",
			Loading: cs.disconnecting, Disabled: cs.syncing,
		}).Radius(999).Label(sp.text.disconnect()).OnClick(func() { cs.disconnectConfirm = true })
		return
	}
	label := sp.text.authorize()
	if v.auth.state == authNeedsReauth {
		label = sp.text.reauthorize()
	}
	better.Button(c, t, "authorize", better.ButtonProps{
		Variant: better.ButtonPrimary, Icon: "account-key-outline", Loading: cs.authorizing, Label: label,
	}).OnClick(func() { s.authorize(p, sp, cs) })
}

// pushRow：flex items-center justify-between gap-4，左侧 flex-1 space-y-1 的 label（block text-sm font-medium
// text-brand-700 dark:text-brand-300）与说明（text-xs text-brand-500 dark:text-brand-400），右侧 BetterSwitch。
// 开关改草稿（onChange），未设置时视为开启（?? true）。
func pushRow(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec) {
	checked := true
	if p.Config != nil {
		if v := *sp.push(p.Config); v != nil {
			checked = *v
		}
	}
	ui.Row(c.Key("status-push")).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			components.Text(c, sp.text.pushLabel(), theme.TextSM, theme.WeightMedium, t.SectionLabel)
			panel.FieldHint(c, t, sp.text.pushHint())
		})
		better.Switch(c, t, string(sp.provider)+"_status_push_enabled", checked, false, func(on bool) {
			if p.Change == nil {
				return
			}
			p.Change(func(cfg *appconf.AppConfig) { *sp.push(cfg) = &on })
		})
	})
}

// modals 是卡片的三个弹窗：同步全部状态的确认、同步进度（RemoteStatusSyncProgressModal）、断开连接的确认（danger）。
func (s *State) modals(c *ui.Context, t *theme.Theme, p panel.Settings, sp *spec, cs *cardState) {
	key := string(sp.provider)
	components.ConfirmModal(c, t, "account-sync-confirm-"+key, components.ConfirmProps{
		Open:        &cs.syncConfirm,
		Title:       sp.text.syncTitle(),
		Message:     sp.text.syncMsg(),
		ConfirmText: locales.SettingsBasicRemoteStatusSyncStart(),
		TopOffset:   panel.TopOffset,
		OnConfirm:   func() { s.startSync(p, sp, cs) },
	})
	progressModal(c, t, "account-sync-progress-"+key, cs, sp.name())
	components.ConfirmModal(c, t, "account-disconnect-confirm-"+key, components.ConfirmProps{
		Open:        &cs.disconnectConfirm,
		Title:       sp.text.disconnectTitle(),
		Message:     sp.text.disconnectMsg(),
		ConfirmText: sp.text.disconnect(),
		Danger:      true,
		TopOffset:   panel.TopOffset,
		OnConfirm:   func() { s.disconnect(p, sp, cs) },
	})
}
