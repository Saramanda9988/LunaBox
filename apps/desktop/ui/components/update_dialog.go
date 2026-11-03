package components

import (
	"errors"
	"runtime"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// uno.config.ts 的 accent（星光蓝），theme 包里没有。
var (
	accent100 = ui.Hex("#dbeafe")
	accent400 = ui.Hex("#60a5fa")
	accent500 = ui.Hex("#3b82f6")
	accent600 = ui.Hex("#2563eb")
	accent700 = ui.Hex("#1d4ed8")
	accent900 = ui.Hex("#1e3a8a")
)

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// 对话框的显示与关闭动画（transition-all duration-200；出现前先等 100ms，同原版的 setTimeout）。
const (
	updateDialogShowDelay = 100 * time.Millisecond
	updateDialogFade      = 200 * time.Millisecond
)

// UpdateDialogState 是 UpdateDialog.tsx 组件自己的状态（isVisible、isUpdating、progress、downloadSpeed、updateError）。
// 原版每次显示对话框都重新挂载组件；这里由调用方持有，显示对话框时清零（= UpdateDialogState{}）。
type UpdateDialogState struct {
	Updating bool                    // isUpdating
	Progress *service.UpdateProgress // update:progress 事件的最近一次进度
	Speed    float64                 // 下载速度（字节/秒）
	Error    string                  // updateError

	sample   *downloadSample // downloadSampleRef
	scroll   ScrollBar       // 更新内容的滚动位置
	openedAt time.Time       // 第一次构建的时间（之后 100ms 变为可见）
	closing  bool            // handleClose 之后淡出
	closedAt time.Time       // 开始淡出的时间；为零时下一次构建取 c.Now()
	closed   bool            // 已调用 OnClose
}

type downloadSample struct {
	downloaded int64
	at         time.Time
}

// SetProgress 对应 onWailsEvent("update:progress", …)：记录进度，下载阶段按与上一次进度的差值估算速度。
func (s *UpdateDialogState) SetProgress(v service.UpdateProgress, now time.Time) {
	if v.Phase == "downloading" {
		prev := s.sample
		if prev != nil && v.Downloaded >= prev.downloaded && now.After(prev.at) {
			s.Speed = float64(v.Downloaded-prev.downloaded) / now.Sub(prev.at).Seconds()
		} else {
			s.Speed = 0
		}
		s.sample = &downloadSample{downloaded: v.Downloaded, at: now}
	} else {
		s.sample = nil
		s.Speed = 0
	}
	s.Progress = &v
}

// updating 是正在应用内更新的对话框。原版每个 UpdateDialog 各自订阅 update:progress；后端的 applyMu 保证同一时间
// 只有一次更新，所以这里由 App 订阅一次（UpdateProgressEvent），交给发起更新的对话框。只在主线程读写。
var updating *UpdateDialogState

// UpdateProgressEvent 把 update:progress 事件交给正在更新的对话框。
func UpdateProgressEvent(v service.UpdateProgress, now time.Time) {
	if updating != nil && updating.Updating {
		updating.SetProgress(v, now)
	}
}

// StartInAppUpdate 对应 handleInAppUpdate 调用 DownloadAndApplyUpdate 的前后：进入更新中、清空错误，进度从下载 0% 开始；
// run 在后台执行 work 并把它返回的函数放回主线程（env.Env.Async）。失败或没有要更新的文件时退出更新中并显示错误；
// 成功时后端稍后退出应用，由 LunaBoxUpdater 替换文件后重新启动。
func (s *UpdateDialogState) StartInAppUpdate(run func(work func() func()), apply func() (*service.UpdateApplyResult, error)) {
	if s.Updating {
		return
	}
	s.Updating, s.Error, s.Speed, s.sample = true, "", 0, nil
	s.Progress = &service.UpdateProgress{Phase: "downloading"}
	updating = s
	run(func() func() {
		result, err := apply()
		return func() {
			if err == nil && (result == nil || !result.Started) {
				err = errors.New(locales.UpdateDialogNoFilesToUpdate())
			}
			if err != nil {
				s.Updating, s.Error = false, err.Error()
			}
		}
	})
}

// UpdateDialogProps 是 UpdateDialog 的属性。
type UpdateDialogProps struct {
	Info      *service.UpdateCheckResult // updateInfo；为空或没有更新时不显示
	Platform  string                     // store 的 platformGOOS；为空时取 runtime.GOOS
	TopOffset float32                    // 原版的 #app-modal-root 在顶栏下方，遮罩从这里开始
	OnClose   func()                     // 淡出结束后调用
	OnSkip    func(version string)
	// OnInAppUpdate 对应 handleInAppUpdate 里的 DownloadAndApplyUpdate(update_manifest_url)，调用方一般交给
	// UpdateDialogState.StartInAppUpdate。只在 Windows 且有更新清单时调用。
	OnInAppUpdate func(manifestURL string)
}

// UpdateDialog 对应 components/ui/UpdateDialog.tsx：absolute inset-0 z-50 的遮罩（bg-black/50 backdrop-blur-sm）上居中的
// max-w-md w-full mx-4 rounded-xl shadow-2xl border 面板，显示与关闭时 opacity 0 ↔ 100、scale-95 ↔ 100（duration-200）。
// 点击遮罩关闭（更新中不关闭）；原版没有 Esc。要在页面最后构建，浮层才会盖住页面内容。
func UpdateDialog(c *ui.Context, t *theme.Theme, key string, s *UpdateDialogState, p UpdateDialogProps) {
	info := p.Info
	if info == nil || !info.HasUpdate || s.closed {
		return
	}
	now := c.Now()
	if s.openedAt.IsZero() {
		s.openedAt = now
	}
	visible := false
	if s.closing {
		if s.closedAt.IsZero() {
			s.closedAt = now
		}
		if d := now.Sub(s.closedAt); d < updateDialogFade {
			c.After(updateDialogFade - d)
		} else {
			s.closed = true
			if p.OnClose != nil {
				p.OnClose()
			}
			return
		}
	} else if d := now.Sub(s.openedAt); d < updateDialogShowDelay {
		c.After(updateDialogShowDelay - d)
	} else {
		visible = true
	}

	platform := p.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	supportsInAppUpdate := platform == "windows"
	usesManualUpdate := platform == "darwin" || platform == "linux"
	showGitHub := usesManualUpdate && info.Downloads["github"] != ""
	showGitee := usesManualUpdate && info.Downloads["gitee"] != ""

	handleClose := func() {
		if !s.Updating && !s.closing {
			s.closing, s.closedAt = true, time.Time{}
		}
	}
	handleSkip := func() {
		if p.OnSkip != nil {
			p.OnSkip(info.LatestVer)
		}
		handleClose()
	}
	handleDownload := func(source string) {
		if url := info.Downloads[source]; url != "" {
			c.OpenURL(url)
		}
	}
	handleInAppUpdate := func() {
		if !supportsInAppUpdate || info.UpdateManifestURL == "" || s.Updating || p.OnInAppUpdate == nil {
			return
		}
		p.OnInAppUpdate(info.UpdateManifestURL)
	}

	Overlay(c, t, func() {
		host := ui.Box(c.Key(key)).Absolute().Top(p.TopOffset).Left(0).Right(0).Bottom(0).Modal()
		fade := host.AnimateWith("fade", theme.Bool(visible), updateDialogFade, theme.EaseTailwind)
		host.Opacity(fade).Children(func() {
			// 原版的遮罩是带 onClick 的 div（关闭按钮才有 aria-label），所以这里用不可聚焦、没有名称的 Box。
			backdrop := ui.Box(c.Key("backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Material(glass.Blur{Radius: 4}).OnClick(handleClose)
			backdrop.Children(func() {
				ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 0.5))
			})
			// 绝对定位的遮罩画在流内元素之上，所以居中的容器也绝对定位，排在遮罩之后。
			ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
				scale := 0.95 + 0.05*fade
				// relative bg-white dark:bg-brand-800 rounded-xl shadow-2xl border border-brand-200 dark:border-brand-700 max-w-md w-full mx-4
				ui.Column(c.Key("panel")).FillWidth().MaxWidth(448).Margin(0, 16).Radius(12).
					Background(t.Pick(theme.White, theme.Brand800)).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
					Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Scale(scale, scale).Children(func() {
					updateDialogHeader(c, t, handleClose)
					// px-6 pb-6 space-y-4
					ui.Column(c).FillWidth().Padding(0, 24, 24, 24).Gap(16).Children(func() {
						updateDialogVersions(c, t, info)
						updateDialogChangelog(c, t, s, info)
						// space-y-2 的操作区
						ui.Column(c).FillWidth().Gap(8).Children(func() {
							if s.Updating && s.Progress != nil {
								updateDialogProgress(c, t, s)
							}
							if supportsInAppUpdate && info.UpdateManifestURL != "" {
								updateNowButton(c, t, s.Updating).OnClick(handleInAppUpdate)
							}
							if s.Error != "" {
								updateDialogError(c, t, s.Error, showGitHub || showGitee)
							}
							if showGitHub || showGitee {
								// grid gap-2，两个按钮时 grid-cols-2
								ui.Row(c.Key("downloads")).FillWidth().Gap(8).Children(func() {
									if showGitHub {
										downloadButton(c, t, "github", "github", locales.UpdateDialogGithubDownload(),
											t.Pick(theme.Brand700, theme.Brand200), t.Pick(theme.Brand800, theme.White), t.Pick(theme.White, theme.Brand900)).
											OnClick(func() {
												if !s.Updating {
													handleDownload("github")
												}
											})
									}
									if showGitee {
										downloadButton(c, t, "gitee", "cloud-download", locales.UpdateDialogGiteeDownload(),
											theme.Neutral600, theme.Neutral700, theme.White).
											OnClick(func() {
												if !s.Updating {
													handleDownload("gitee")
												}
											})
									}
								})
							}
							skipButton(c, t).OnClick(func() {
								if !s.Updating {
									handleSkip()
								}
							})
						})
					})
				})
			})
		})
	})
}

// updateDialogHeader：flex items-start gap-4 p-6 pb-4。左侧 w-12 h-12 rounded-full bg-accent-100 dark:bg-accent-900/30 的
// i-mdi-download-circle text-3xl text-accent-600 dark:text-accent-400；标题 text-xl font-bold mb-1，说明 text-sm
// text-brand-600 dark:text-brand-400；右侧 w-8 h-8 rounded-lg 的关闭按钮（hover:bg-brand-100 dark:hover:bg-brand-700）。
func updateDialogHeader(c *ui.Context, t *theme.Theme, onClose func()) {
	ui.Row(c.Key("header")).FillWidth().Padding(24, 24, 16, 24).Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Box(c).Size(48, 48).Radius(999).Center().Shrink(0).Background(t.Pick(accent100, accent900.Alpha(0.3))).Children(func() {
			Icon(c, "download-circle", 30, t.Pick(accent600, accent400))
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			TextMargin(c, locales.UpdateDialogNewVersion(), theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 4, 0)
			Text(c, locales.UpdateDialogAvailable(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
		})
		// 更新中原版是 disabled（没有 disabled 样式）；MyGo 会把禁用的元素画淡，所以只在 handleClose 里忽略点击。
		btn := ui.ButtonBase(c.Key("close")).Size(32, 32).Radius(8).Center().Shrink(0).FocusRing(false).
			Label(locales.UpdateDialogClose())
		if btn.Hovered() {
			btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		}
		btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onClose).Children(func() {
			Icon(c, "close", 20, t.Pick(theme.Brand500, theme.Brand400))
		})
	})
}

// updateDialogVersions：p-4 bg-brand-50 dark:bg-brand-900/50 rounded-lg space-y-2，每行 flex justify-between items-center
// text-sm：当前版本（font-mono font-medium）、最新版本（font-mono font-semibold text-accent-600 dark:text-accent-400）、发布日期。
func updateDialogVersions(c *ui.Context, t *theme.Theme, info *service.UpdateCheckResult) {
	label := t.Pick(theme.Brand600, theme.Brand400)
	row := func(key, name string, value func()) {
		ui.Row(c.Key(key)).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
			Text(c, name, theme.TextSM, theme.WeightNormal, label)
			value()
		})
	}
	ui.Column(c.Key("versions")).FillWidth().Padding(16).Radius(8).Gap(8).
		Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.5))).Children(func() {
		row("current", locales.UpdateDialogCurrentVersion(), func() {
			Text(c, "v"+info.CurrentVer, theme.TextSM, theme.WeightMedium, t.TextStrong).Font(monoFont).NoWrap()
		})
		row("latest", locales.UpdateDialogLatestVersion(), func() {
			Text(c, "v"+info.LatestVer, theme.TextSM, theme.WeightSemibold, t.Pick(accent600, accent400)).Font(monoFont).NoWrap()
		})
		row("release", locales.UpdateDialogReleaseDate(), func() {
			Text(c, info.ReleaseDate, theme.TextSM, theme.WeightNormal, t.TextStrong).NoWrap()
		})
	})
}

// updateDialogChangelog：max-h-64 overflow-y-auto p-4 bg-brand-50 dark:bg-brand-900/50 rounded-lg；标题 text-sm
// font-semibold mb-2，内容 text-xs text-brand-700 dark:text-brand-300 space-y-1 whitespace-pre-wrap，每行一个 div。
// 滚动条（::-webkit-scrollbar）在内边距外侧，内容超出时占用右侧的宽度。
func updateDialogChangelog(c *ui.Context, t *theme.Theme, s *UpdateDialogState, info *service.UpdateCheckResult) {
	gutter := float32(0)
	if s.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	ScrollArea(c, t, "changelog", &s.scroll, func(w ui.Element) {
		w.FillWidth().MaxHeight(256).Radius(8).Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.5)))
	}, func(sc ui.Element) {
		sc.Children(func() {
			ui.Column(c).FillWidth().Padding(16, 16+gutter, 16, 16).Children(func() {
				TextMargin(c, locales.UpdateDialogChangelog(), theme.TextSM, theme.WeightSemibold, t.TextStrong, 0, 0, 8, 0)
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					for i, line := range info.Changelog {
						Text(c.Key(i), line, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand300))
					}
				})
			})
		})
	})
}

// updateDialogProgress：space-y-2 px-0.5 py-1。第一行 text-xs 的阶段说明（truncate）与百分比（font-mono font-medium），
// 中间 h-1.5 rounded-full bg-brand-200 dark:bg-brand-700 的进度条（bg-accent-500，duration-300；下载以外的阶段占满并
// animate-pulse），最后一行是文件名与当前速度。
func updateDialogProgress(c *ui.Context, t *theme.Theme, s *UpdateDialogState) {
	pr := s.Progress
	downloading := pr.Phase == "downloading"
	var text string
	switch pr.Phase {
	case "preparing":
		text = locales.UpdateDialogPreparing()
	case "fallback":
		text = locales.UpdateDialogFallback()
	case "ready":
		text = locales.UpdateDialogRestarting()
	default:
		text = locales.UpdateDialogDownloading()
	}
	ui.Column(c.Key("progress")).FillWidth().Padding(4, 2).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			Text(c, text, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).SingleLine().Shrink(1).MinWidth(0)
			if downloading {
				Text(c, strconv.Itoa(pr.Percent)+"%", theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100)).
					Font(monoFont).NoWrap().Shrink(0)
			}
		})
		track := ui.Row(c.Key("bar")).FillWidth().Height(6).Radius(999).Clip().Background(t.Pick(theme.Brand200, theme.Brand700))
		frac := float32(1)
		if downloading {
			frac = float32(max(2, min(100, pr.Percent))) / 100
		}
		frac = track.AnimateWith("width", frac, 300*time.Millisecond, theme.EaseTailwind)
		track.Children(func() {
			bar := ui.Box(c).Grow(frac).Basis(0).Radius(999).Background(accent500)
			if !downloading {
				bar.Opacity(Pulse(bar))
			}
			if frac < 1 {
				ui.Box(c).Grow(1 - frac).Basis(0)
			}
		})
		muted := t.Pick(theme.Brand500, theme.Brand400)
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			Text(c, pr.File, theme.TextXS, theme.WeightNormal, muted).SingleLine().Shrink(1).MinWidth(0)
			if downloading {
				Text(c, locales.UpdateDialogCurrentSpeed(FormatFileSize(s.Speed)+"/s"), theme.TextXS, theme.WeightNormal, muted).
					FontFeatures("tnum").NoWrap().Shrink(0)
			}
		})
	})
}

// updateNowButton：w-full px-4 py-2.5 text-sm font-medium text-white bg-accent-600 hover:bg-accent-700 rounded-lg
// flex items-center justify-center gap-2，i-mdi-update（更新中 i-mdi-loading animate-spin）text-lg；
// disabled:opacity-70 disabled:cursor-wait（MyGo 的禁用元素最高 0.5）。
func updateNowButton(c *ui.Context, t *theme.Theme, updating bool) ui.Element {
	btn := ui.ButtonBase(c.Key("update-now")).FillWidth().Padding(10, 16).Radius(8).Gap(8).Center().FocusRing(false)
	bg := accent600
	if btn.Hovered() && !updating {
		bg = accent700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	label := locales.UpdateDialogUpdateNow()
	if updating {
		Disable(btn, 0.5)
		label = locales.UpdateDialogDownloadingButton()
	}
	btn.Children(func() {
		if updating {
			Spinner(c, 18, theme.White)
		} else {
			Icon(c, "update", 18, theme.White)
		}
		Text(c, label, theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}

// updateDialogError：rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-700 dark:border-red-700
// dark:bg-red-900/20 dark:text-red-300；标题 font-medium，错误 mt-1 break-words，可手动下载时再加一行 mt-1 的提示。
func updateDialogError(c *ui.Context, t *theme.Theme, msg string, manual bool) {
	fg := t.Pick(theme.Error700, theme.Error300)
	ui.Column(c.Key("update-error")).FillWidth().Padding(12).Radius(8).
		Background(t.Pick(theme.Error50, theme.Error900.Alpha(0.2))).Border(t.HL, t.Pick(theme.Error200, theme.Error700)).Children(func() {
		Text(c, locales.UpdateDialogUpdateFailed(), theme.TextXS, theme.WeightMedium, fg)
		TextMargin(c, msg, theme.TextXS, theme.WeightNormal, fg, 4, 0, 0, 0)
		if manual {
			TextMargin(c, locales.UpdateDialogManualFallback(), theme.TextXS, theme.WeightNormal, fg, 4, 0, 0, 0)
		}
	})
}

// downloadButton 是 macOS、Linux 的手动下载按钮：w-full px-4 py-2.5 text-sm font-medium rounded-lg flex items-center
// justify-center gap-2，图标 text-lg。GitHub 是 bg-brand-700 hover:bg-brand-800 dark:bg-brand-200 dark:hover:bg-white
// dark:text-brand-900，Gitee 是 bg-neutral-600 hover:bg-neutral-700 text-white。
func downloadButton(c *ui.Context, t *theme.Theme, key, icon, label string, bg, hover, fg ui.Color) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Grow(1).Basis(0).Padding(10, 16).Radius(8).Gap(8).Center().FocusRing(false)
	if btn.Hovered() {
		bg = hover
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		Icon(c, icon, 18, fg)
		Text(c, label, theme.TextSM, theme.WeightMedium, fg).NoWrap()
	})
	return btn
}

// skipButton：w-full px-4 py-2.5 text-sm font-medium text-brand-600 dark:text-brand-400 hover:bg-brand-100
// dark:hover:bg-brand-700 border border-brand-300 dark:border-brand-600 rounded-lg。
func skipButton(c *ui.Context, t *theme.Theme) ui.Element {
	btn := ui.ButtonBase(c.Key("skip")).FillWidth().Padding(10, 16).Radius(8).Center().FocusRing(false).
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		Text(c, locales.UpdateDialogSkipVersion(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand600, theme.Brand400)).NoWrap()
	})
	return btn
}
