// Package backup 对应原版 components/panel/GameBackupPanel.tsx（详情页“备份”标签页）：
// 存档的本地备份（创建、恢复、删除、打开备份文件夹）与云端备份（上传、列表、从云端恢复）。
package backup

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// uno.config.ts 里本面板用到、theme 包没有的颜色。
var (
	neutral900 = ui.Hex("#0f172a")
	success100 = ui.Hex("#d1fae5")
	success400 = ui.Hex("#34d399")
	success600 = ui.Hex("#059669")
	success700 = ui.Hex("#047857")
	success900 = ui.Hex("#064e3b")
	error100   = ui.Hex("#fee2e2")
	error900   = ui.Hex("#7f1d1d")
)

// textLG 是 text-lg。
var textLG = theme.FontStep{Size: 18, Line: 28}

// State 是面板跨帧保留的状态，由详情页持有；进入另一个游戏时清零。
type State struct {
	gameID string // 已加载的游戏；与当前游戏不同时重新加载

	localGen, cloudGen int // 最近一次请求的编号，丢弃过期结果

	backups      []models.GameBackup
	loadingLocal bool
	cloudStatus  *vo.CloudBackupStatus
	cloudBackups []vo.CloudBackupItem
	loadingCloud bool
	backingUp    bool
	uploading    bool

	confirm confirmState
	toasts  []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）
}

// confirmState 对应原版的 confirmConfig。标题与说明存函数，切换语言后弹窗里的文字随之更新。
type confirmState struct {
	open      bool
	title     func() string
	message   func() string
	danger    bool
	onConfirm func()
}

var seq atomic.Int64

// nextGen 返回全局递增的请求编号：State 清零后编号也不会与旧请求重复。
func nextGen() int { return int(seq.Add(1)) }

// View 构建面板：space-y-6 的备份操作卡片、本地备份卡片、云端备份卡片（或云备份未启用提示），最后是确认弹窗。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Game) {
	if p.Game == nil {
		return
	}
	if s.gameID != p.Game.ID {
		s.open(p)
	}
	for _, msg := range s.toasts {
		p.Env.Toast(c, msg)
	}
	s.toasts = nil

	cloudEnabled := s.cloudEnabled()
	ui.Column(c.Key("backup-panel")).Gap(24).Children(func() {
		s.actionsCard(c, t, p)
		s.localCard(c, t, p, cloudEnabled)
		if cloudEnabled {
			s.cloudCard(c, t, p)
		} else {
			cloudHint(c, t)
		}
	})

	title, message := "", ""
	if s.confirm.title != nil {
		title, message = s.confirm.title(), s.confirm.message()
	}
	components.ConfirmModal(c, t, "backup-confirm", components.ConfirmProps{
		Open:      &s.confirm.open,
		Title:     title,
		Message:   message,
		Danger:    s.confirm.danger,
		TopOffset: panel.TopOffset,
		OnConfirm: s.confirm.onConfirm,
	})
}

func (s *State) cloudEnabled() bool {
	return s.cloudStatus != nil && s.cloudStatus.Configured && s.cloudStatus.Enabled
}

// open 对应挂载时的两个 useEffect：读取本地备份与云备份状态，云备份可用时再读取云端列表。
func (s *State) open(p panel.Game) {
	*s = State{gameID: p.Game.ID}
	s.loadBackups(p.Env, nil)
	s.cloudGen = nextGen()
	e, gen := p.Env, s.cloudGen
	e.Async(func() func() {
		status := e.Svc.Backup.GetCloudBackupStatus()
		return func() {
			if gen != s.cloudGen {
				return
			}
			s.cloudStatus = &status
			if s.cloudEnabled() {
				s.loadCloudBackups(e)
			}
		}
	})
}

// loadBackups 对应 loadBackups：读取期间显示加载图标；done 在读取完成后执行（原版 await loadBackups() 之后的提示）。
func (s *State) loadBackups(e *env.Env, done func()) {
	s.localGen = nextGen()
	s.loadingLocal = true
	gen, id := s.localGen, s.gameID
	e.Async(func() func() {
		backups, err := e.Svc.Backup.GetGameBackups(id)
		return func() {
			if gen != s.localGen {
				return
			}
			s.loadingLocal = false
			if err == nil {
				s.backups = backups
			}
			if done != nil {
				done()
			}
		}
	})
}

// loadCloudBackups 对应 loadCloudBackups。
func (s *State) loadCloudBackups(e *env.Env) {
	s.cloudGen = nextGen()
	s.loadingCloud = true
	gen, id := s.cloudGen, s.gameID
	e.Async(func() func() {
		items, err := e.Svc.Backup.GetCloudGameBackups(id)
		return func() {
			if gen != s.cloudGen {
				return
			}
			s.loadingCloud = false
			if err == nil {
				s.cloudBackups = items
			}
		}
	})
}

// run 在后台执行一个服务调用，结果回到主线程（游戏已切换时丢弃）。
func (s *State) run(e *env.Env, work func() error, apply func(err error)) {
	id := s.gameID
	e.Async(func() func() {
		err := work()
		return func() {
			if s.gameID == id {
				apply(err)
			}
		}
	})
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// createBackup 对应 handleCreateBackup。
func (s *State) createBackup(p panel.Game) {
	if p.Game.SavePath == "" {
		s.toast(env.Error(locales.GameBackupToastSavePathRequired()))
		return
	}
	s.backingUp = true
	id := p.Game.ID
	s.run(p.Env, func() error {
		_, err := p.Env.Svc.Backup.CreateBackup(id)
		return err
	}, func(err error) {
		if err != nil {
			s.backingUp = false
			s.toast(env.Error(locales.GameBackupToastBackupFailed(err.Error())))
			return
		}
		s.loadBackups(p.Env, func() {
			s.backingUp = false
			s.toast(env.Success(locales.GameBackupToastBackupCreated()))
		})
	})
}

// askRestore 对应 handleRestoreBackup。
func (s *State) askRestore(p panel.Game, b models.GameBackup) {
	created, loc := b.CreatedAt, timefmt.Location(p.Env.Config())
	path := b.Path
	s.confirm = confirmState{
		open:  true,
		title: locales.GameBackupModalRestoreTitle,
		message: func() string {
			return locales.GameBackupModalRestoreMsg(timefmt.DateTime(created, loc))
		},
		onConfirm: func() {
			s.run(p.Env, func() error { return p.Env.Svc.Backup.RestoreBackup(path) }, func(err error) {
				if err != nil {
					s.toast(env.Error(locales.GameBackupToastRestoreFailed(err.Error())))
					return
				}
				s.toast(env.Success(locales.GameBackupToastRestored()))
			})
		},
	}
}

// askDelete 对应 handleDeleteBackup。
func (s *State) askDelete(p panel.Game, b models.GameBackup) {
	path := b.Path
	s.confirm = confirmState{
		open:    true,
		title:   locales.GameBackupModalDeleteTitle,
		message: locales.GameBackupModalDeleteMsg,
		danger:  true,
		onConfirm: func() {
			s.run(p.Env, func() error { return p.Env.Svc.Backup.DeleteBackup(path) }, func(err error) {
				if err != nil {
					s.toast(env.Error(locales.GameBackupToastDeleteFailed(err.Error())))
					return
				}
				s.loadBackups(p.Env, func() { s.toast(env.Success(locales.GameBackupToastDeleted())) })
			})
		},
	}
}

// openFolder 对应 handleOpenBackupFolder。
func (s *State) openFolder(p panel.Game) {
	id := p.Game.ID
	s.run(p.Env, func() error { return p.Env.Svc.Backup.OpenBackupFolder(id) }, func(err error) {
		if err != nil {
			s.toast(env.Error(locales.GameBackupToastOpenFolderFailed(err.Error())))
		}
	})
}

// upload 对应 handleUploadToCloud。
func (s *State) upload(p panel.Game, b models.GameBackup) {
	s.uploading = true
	id, path := p.Game.ID, b.Path
	s.run(p.Env, func() error { return p.Env.Svc.Backup.UploadGameBackupToCloud(id, path) }, func(err error) {
		s.uploading = false
		if err != nil {
			s.toast(env.Error(locales.GameBackupToastUploadFailed(err.Error())))
			return
		}
		s.loadCloudBackups(p.Env)
		s.toast(env.Success(locales.GameBackupToastUploaded()))
	})
}

// askRestoreCloud 对应 handleRestoreFromCloud。
func (s *State) askRestoreCloud(p panel.Game, item vo.CloudBackupItem) {
	id, key, name := p.Game.ID, item.Key, item.Name
	s.confirm = confirmState{
		open:    true,
		title:   locales.GameBackupModalRestoreCloudTitle,
		message: func() string { return locales.GameBackupModalRestoreCloudMsg(name) },
		onConfirm: func() {
			s.run(p.Env, func() error { return p.Env.Svc.Backup.RestoreFromCloud(key, id) }, func(err error) {
				if err != nil {
					s.toast(env.Error(locales.GameBackupToastRestoreFailed(err.Error())))
					return
				}
				s.toast(env.Success(locales.GameBackupToastRestoredFromCloud()))
			})
		},
	}
}

// card：glass-card bg-white dark:bg-brand-800 p-6 rounded-lg。
func card(c *ui.Context, t *theme.Theme, key string) ui.Element {
	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), ui.Transparent)
	e := ui.Column(c.Key(key)).Padding(24).Radius(8).Background(bg)
	if t.Glass {
		e.Border(t.HL, border)
	}
	return e
}

// heading：h3 text-lg font-semibold text-brand-900 dark:text-white。
func heading(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.Text(c, s, textLG, theme.WeightSemibold, t.TextStrong)
}

// actionsCard 是备份操作区：标题与存档路径（mt-1 text-sm），右侧 flex gap-2 的两个按钮。
// 内层 flex 带 mb-4，所以卡片底部比顶部多 16。
func (s *State) actionsCard(c *ui.Context, t *theme.Theme, p panel.Game) {
	card(c, t, "actions").Children(func() {
		ui.Row(c).Margin(0, 0, 16, 0).Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				heading(c, t, locales.GameBackupTitle())
				path := locales.GameBackupPathPlaceholder()
				if p.Game.SavePath != "" {
					path = locales.GameBackupPathLabel(p.Game.SavePath)
				}
				components.TextMargin(c, path, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
			})
			ui.Row(c).Gap(8).Shrink(0).Children(func() {
				openFolderButton(c, t).OnClick(func() { s.openFolder(p) })
				backupButton(c, t, s.backingUp, p.Game.SavePath == "").OnClick(func() { s.createBackup(p) })
			})
		})
	})
}

// openFolderButton：glass-btn-none px-4 py-2 text-brand-600 dark:text-brand-400 hover:bg-brand-100
// dark:hover:bg-brand-700 rounded-md transition-colors（按钮继承 16px/24px 的字号与行高）。
// glass-btn-none 实际是 neutral 的玻璃按钮（见 theme.GlassButton）。
func openFolderButton(c *ui.Context, t *theme.Theme) ui.Element {
	btn := ui.ButtonBase(c.Key("open-folder")).Padding(8, 16).Radius(6).FocusRing(false)
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: ui.Transparent, Hover: t.Pick(theme.Brand100, theme.Brand700),
		Text: t.Pick(theme.Brand600, theme.Brand400)})
	bg := g.Bg
	if btn.Hovered() {
		bg = g.Hover
	}
	btn.Background(bg)
	if t.Glass {
		btn.Border(t.HL, g.Border)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Text(c, locales.GameBackupOpenFolder(), theme.TextBase, theme.WeightNormal, g.Text).NoWrap()
	})
	return btn
}

// backupButton：glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700
// disabled:opacity-50 flex items-center gap-2，备份中前面是 i-mdi-loading animate-spin。
func backupButton(c *ui.Context, t *theme.Theme, busy, noPath bool) ui.Element {
	btn := ui.ButtonBase(c.Key("backup-now")).Padding(8, 16).Radius(6).Gap(8).AlignItems(ui.Center).FocusRing(false)
	disabled := busy || noPath
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: theme.Neutral600, Hover: theme.Neutral700, Text: theme.White})
	bg := g.Bg
	if btn.Hovered() && !disabled {
		bg = g.Hover
	}
	if t.Glass {
		btn.Border(t.HL, g.Border)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	label := locales.GameBackupBackupNow()
	if busy {
		label = locales.GameBackupBackingUp()
	}
	btn.Children(func() {
		if busy {
			components.Spinner(c, 16, g.Text)
		}
		components.Text(c, label, theme.TextBase, theme.WeightNormal, g.Text).NoWrap()
	})
	return btn
}

// localCard 是本地备份列表：标题行（flex items-center gap-2 mb-4，自动备份开启时带徽标），
// 加载中是 py-8 的旋转图标，空时是 py-8 的提示，否则 space-y-3 的备份行。
func (s *State) localCard(c *ui.Context, t *theme.Theme, p panel.Game, cloudEnabled bool) {
	card(c, t, "local").Children(func() {
		ui.Row(c).Margin(0, 0, 16, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			heading(c, t, locales.GameBackupLocalBackups())
			if cfg := p.Env.Config(); cfg != nil && cfg.AutoBackupGameSave {
				autoBackupBadge(c, t)
			}
		})
		switch {
		case s.loadingLocal:
			loadingBlock(c)
		case len(s.backups) == 0:
			emptyBlock(c, locales.GameBackupNoLocalBackups())
		default:
			loc := timefmt.Location(p.Env.Config())
			ui.Column(c).Gap(12).Children(func() {
				for _, b := range s.backups {
					s.backupRow(c.Key(b.Path), t, p, b, loc, cloudEnabled)
				}
			})
		}
	})
}

// autoBackupBadge：px-2 py-0.5 text-xs font-medium bg-success-100 text-success-700 dark:bg-success-900/30
// dark:text-success-400 rounded-full flex items-center gap-1，i-mdi-shield-check text-sm。
func autoBackupBadge(c *ui.Context, t *theme.Theme) {
	fg := t.Pick(success700, success400)
	ui.Row(c).Padding(2, 8).Gap(4).Radius(999).AlignItems(ui.Center).Background(t.Pick(success100, success900.Alpha(0.3))).Children(func() {
		components.Icon(c, "shield-check", 14, fg)
		components.Text(c, locales.GameBackupAutoBackupEnabled(), theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
}

// loadingBlock：flex justify-center py-8，i-mdi-loading animate-spin text-2xl text-brand-500。
func loadingBlock(c *ui.Context) {
	ui.Row(c).Padding(32, 0).Justify(ui.Center).Children(func() {
		components.Spinner(c, 24, theme.Brand500)
	})
}

// emptyBlock：text-center py-8 text-brand-500。
func emptyBlock(c *ui.Context, s string) {
	ui.Column(c).Padding(32, 0).AlignItems(ui.Center).Children(func() {
		components.Text(c, s, theme.TextBase, theme.WeightNormal, theme.Brand500)
	})
}

// backupRow：flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg；
// 左侧 gap-4 的 i-mdi-archive text-2xl 与时间、大小，右侧 gap-2 的上传（云备份可用时）、恢复、删除按钮。
func (s *State) backupRow(c *ui.Context, t *theme.Theme, p panel.Game, b models.GameBackup, loc *time.Location, cloudEnabled bool) {
	// data-glass:bg-white/1 data-glass:dark:bg-black/1
	ui.Row(c).Padding(16).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(16).
		Background(t.IfGlass(t.Pick(theme.Brand50, theme.Brand700), theme.White.Alpha(0.01), theme.Black.Alpha(0.01))).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
			components.Icon(c, "archive", 24, theme.Brand500)
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				components.Text(c, timefmt.DateTime(b.CreatedAt, loc), theme.TextBase, theme.WeightMedium, t.TextStrong)
				components.Text(c, locales.GameBackupSize()+FormatFileSize(b.Size), theme.TextSM, theme.WeightNormal, theme.Brand500)
			})
		})
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			if cloudEnabled {
				up := iconButton(c, t, "upload", locales.NativeBackupUploadToCloud(), "cloud-upload", theme.Neutral600,
					t.Pick(theme.Neutral100, neutral900), s.uploading)
				if s.uploading {
					// animate-pulse 只作用在图标上；按钮本身 disabled:opacity-50。
					up.Opacity(0.5 * components.Pulse(up))
				}
				up.OnClick(func() { s.upload(p, b) })
			}
			iconButton(c, t, "restore", locales.GameBackupModalRestoreTitle(), "backup-restore", success600, t.Pick(success100, success900), false).
				OnClick(func() { s.askRestore(p, b) })
			iconButton(c, t, "delete", locales.GameBackupModalDeleteTitle(), "delete", theme.Error600, t.Pick(error100, error900), false).
				OnClick(func() { s.askDelete(p, b) })
		})
	})
}

// iconButton：p-2 rounded transition-colors 的图标按钮，i-mdi-* text-xl，悬停换背景，disabled:opacity-50。
func iconButton(c *ui.Context, t *theme.Theme, key, label, icon string, color, hover ui.Color, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8).Radius(4).FocusRing(false).Label(label)
	if btn.Hovered() && !disabled {
		btn.Background(hover)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() { components.Icon(c, icon, 20, color) })
	return btn
}

// cloudCard 是云端备份列表：标题（i-mdi-cloud text-xl text-neutral-500）与刷新按钮，下面同本地列表。
func (s *State) cloudCard(c *ui.Context, t *theme.Theme, p panel.Game) {
	card(c, t, "cloud").Children(func() {
		ui.Row(c).Margin(0, 0, 16, 0).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Icon(c, "cloud", 20, theme.Neutral500)
				heading(c, t, locales.GameBackupCloudBackups())
			})
			// p-2 text-brand-600 hover:bg-brand-100 dark:hover:bg-brand-700 rounded disabled:opacity-50
			btn := ui.ButtonBase(c.Key("refresh")).Padding(8).Radius(4).FocusRing(false).Label(locales.NativeBackupRefreshCloud())
			if btn.Hovered() && !s.loadingCloud {
				btn.Background(t.Pick(theme.Brand100, theme.Brand700))
			}
			btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
			if s.loadingCloud {
				components.Disable(btn, 0.5)
			}
			btn.OnClick(func() { s.loadCloudBackups(p.Env) }).Children(func() {
				ic := components.Icon(c, "refresh", 20, theme.Brand600)
				if s.loadingCloud {
					ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
				}
			})
		})
		switch {
		case s.loadingCloud:
			loadingBlock(c)
		case len(s.cloudBackups) == 0:
			emptyBlock(c, locales.GameBackupNoCloudBackups())
		default:
			loc := timefmt.Location(p.Env.Config())
			ui.Column(c).Gap(12).Children(func() {
				for _, item := range s.cloudBackups {
					s.cloudRow(c.Key(item.Key), t, p, item, loc)
				}
			})
		}
	})
}

// cloudRow：flex items-center justify-between p-4 bg-neutral-50 dark:bg-neutral-900/30 rounded-lg，
// i-mdi-cloud-check text-2xl text-neutral-500，名称（没有时是时间）与时间，右侧从云端恢复按钮。
func (s *State) cloudRow(c *ui.Context, t *theme.Theme, p panel.Game, item vo.CloudBackupItem, loc *time.Location) {
	when := timefmt.DateTime(item.CreatedAt, loc)
	name := item.Name
	if name == "" {
		name = when
	}
	// data-glass:bg-white/1 data-glass:dark:bg-black/1
	ui.Row(c).Padding(16).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(16).
		Background(t.IfGlass(t.Pick(theme.Neutral50, neutral900.Alpha(0.3)), theme.White.Alpha(0.01), theme.Black.Alpha(0.01))).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
			components.Icon(c, "cloud-check", 24, theme.Neutral500)
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				components.Text(c, name, theme.TextBase, theme.WeightMedium, t.TextStrong)
				components.Text(c, when, theme.TextSM, theme.WeightNormal, theme.Brand500)
			})
		})
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			iconButton(c, t, "restore-cloud", locales.GameBackupModalRestoreCloudTitle(), "cloud-download", success600, t.Pick(success100, success900), false).
				OnClick(func() { s.askRestoreCloud(p, item) })
		})
	})
}

// cloudHint 是云备份未启用提示：glass-panel bg-brand-50 dark:bg-brand-800 p-4 rounded-lg border
// border-brand-200 dark:border-brand-700，flex items-center gap-3，i-mdi-cloud-off-outline text-2xl text-brand-400。
func cloudHint(c *ui.Context, t *theme.Theme) {
	bg, border := t.GlassPanel(t.Pick(theme.Brand50, theme.Brand800), t.Pick(theme.Brand200, theme.Brand700))
	ui.Row(c.Key("cloud-hint")).Padding(16).Radius(8).Gap(12).AlignItems(ui.Center).
		Background(bg).Border(t.HL, border).Children(func() {
		components.Icon(c, "cloud-off-outline", 24, theme.Brand400)
		ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
			components.Text(c, locales.GameBackupCloudNotEnabled(), theme.TextBase, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
			components.Text(c, locales.GameBackupCloudConfigHint(), theme.TextSM, theme.WeightNormal, theme.Brand500)
		})
	})
}

// FormatFileSize 对应 utils/size.ts 的 formatFileSize：二进制单位，保留一位小数。
func FormatFileSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	v, i := float64(bytes), 0
	for v >= 1024 && i < len(sizes)-1 {
		v /= 1024
		i++
	}
	// toFixed(1) 在恰好一半时进位（1280 B 是 1.3 KB），Go 的 %.1f 取偶数，所以先自己舍入。
	return fmt.Sprintf("%.1f %s", math.Floor(v*10+0.5)/10, sizes[i])
}
