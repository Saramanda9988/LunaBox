// Package dbbackup 对应原版 components/panel/DBBackupPanel.tsx（设置页“数据库备份”分区）：立即备份（按配置同时上传云端）、
// 本地备份列表（上传、恢复、删除）、云端备份列表（从云端恢复）或云备份未启用提示。恢复只是安排在下次启动时执行，
// 确认后提示并在 1.5 秒后退出（SafeQuit）。
package dbbackup

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// quitDelay 是安排恢复之后到退出的时间（setTimeout(() => SafeQuit(), 1500)）。
const quitDelay = 1500 * time.Millisecond

// backupAPI 是面板用到的 BackupService 方法（测试里替换云端相关的调用）。
type backupAPI interface {
	GetDBBackups() (*vo.DBBackupStatus, error)
	GetCloudDBBackups() ([]vo.CloudBackupItem, error)
	CreateAndUploadDBBackup() (*vo.DBBackupInfo, error)
	UploadDBBackupToCloud(backupPath string) error
	DeleteDBBackup(backupPath string) error
	ScheduleDBRestore(backupPath string) error
	ScheduleDBRestoreFromCloud(cloudKey string) error
}

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	localGen, cloudGen int // 最近一次请求的编号，丢弃过期结果

	status       *vo.DBBackupStatus // dbBackups
	cloudBackups []vo.CloudBackupItem
	loadingLocal bool
	loadingCloud bool
	cloudWas     bool // 上一次构建时的 cloudEnabled（依赖 cloudEnabled 的 effect）

	backingUp bool   // isBackingUp
	restoring string // restoringBackup：正在安排恢复的备份路径或云端 key
	uploading string // uploadingBackup

	confirm confirmState
	toasts  []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）

	quitPending bool      // 已安排恢复，等待退出
	quitAt      time.Time // 退出的时间；为零时下一次构建取 c.Now()

	api  backupAPI // 为空时用 Env.Svc.Backup
	quit func()    // 为空时用 Env.Svc.AppConfig.SafeQuit
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

// nextGen 返回全局递增的请求编号。
func nextGen() int { return int(seq.Add(1)) }

func (s *State) backup(p panel.Settings) backupAPI {
	if s.api != nil {
		return s.api
	}
	return p.Env.Svc.Backup
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// View 构建面板：space-y-6 的备份操作卡片、本地备份卡片、云端备份卡片（或云备份未启用提示），最后是确认弹窗。
// 原版读 store 的 config（生效的配置），这里同样读 Env.Config()。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	Background(c, s, p)

	cfg := p.Env.Config()
	cloudEnabled := isCloudProviderConfigured(cfg)
	root := ui.Column(c.Key("db-backup")).FillWidth().Gap(24)
	// 原版每次挂载（进入设置页、第一次展开分区）都重新读取；分区收起后这里不再构建内容，再次展开时同样重新读取。
	if mounted := ui.Local(root, "mounted", func() bool { return false }); !*mounted {
		*mounted = true
		s.mount(p, cloudEnabled)
	} else if cloudEnabled != s.cloudWas {
		s.cloudChanged(p, cloudEnabled)
	}
	root.Children(func() {
		s.actionsCard(c, t, p, cfg, cloudEnabled)
		s.localCard(c, t, p, cfg, cloudEnabled)
		if cloudEnabled {
			s.cloudCard(c, t, p, cfg)
		} else {
			cloudHint(c, t)
		}
	})

	title, message := "", ""
	if s.confirm.title != nil {
		title, message = s.confirm.title(), s.confirm.message()
	}
	components.ConfirmModal(c, t, "db-backup-confirm", components.ConfirmProps{
		Open:      &s.confirm.open,
		Title:     title,
		Message:   message,
		Danger:    s.confirm.danger,
		TopOffset: panel.TopOffset,
		OnConfirm: s.confirm.onConfirm,
	})
}

// mount 对应挂载时的两个 useEffect：读取本地备份；云备份可用时读取云端列表。
func (s *State) mount(p panel.Settings, cloudEnabled bool) {
	s.loadLocal(p, nil)
	s.cloudChanged(p, cloudEnabled)
}

// cloudChanged 对应依赖 cloudEnabled 的 useEffect：可用时读取云端列表，否则清空。
func (s *State) cloudChanged(p panel.Settings, cloudEnabled bool) {
	s.cloudWas = cloudEnabled
	if cloudEnabled {
		s.loadCloud(p, nil)
		return
	}
	s.cloudGen = nextGen()
	s.cloudBackups, s.loadingCloud = nil, false
}

// Background 显示后台结果的提示，并在安排恢复 1.5 秒后退出。原版的面板第一次展开后一直挂载，提示是全局的，
// setTimeout 也不随面板卸载取消；这里的面板在分区收起、离开设置页后不再构建，所以设置页每一帧都调用它（View 也调用）。
func Background(c *ui.Context, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil
	s.tickQuit(c, p)
}

// tickQuit 在安排恢复 1.5 秒后退出（帧时钟：安排时记下标记，下一次构建取 c.Now()）。
func (s *State) tickQuit(c *ui.Context, p panel.Settings) {
	if !s.quitPending {
		return
	}
	now := c.Now()
	if s.quitAt.IsZero() {
		s.quitAt = now.Add(quitDelay)
	}
	if wait := s.quitAt.Sub(now); wait > 0 {
		c.After(wait)
		return
	}
	s.quitPending, s.quitAt = false, time.Time{}
	if s.quit != nil {
		s.quit()
	} else {
		p.Env.Svc.AppConfig.SafeQuit()
	}
}

func (s *State) scheduleQuit() { s.quitPending, s.quitAt = true, time.Time{} }

// loadLocal 对应 loadDBBackups：读取期间显示加载图标；done 在读取完成后执行（原版 await loadDBBackups() 之后的代码）。
func (s *State) loadLocal(p panel.Settings, done func()) {
	s.localGen = nextGen()
	s.loadingLocal = true
	gen, api := s.localGen, s.backup(p)
	p.Env.Async(func() func() {
		status, err := api.GetDBBackups()
		return func() {
			if gen != s.localGen {
				return
			}
			s.loadingLocal = false
			if err == nil {
				s.status = status
			}
			if done != nil {
				done()
			}
		}
	})
}

// loadCloud 对应 loadCloudDBBackups：失败时清空列表。
func (s *State) loadCloud(p panel.Settings, done func()) {
	s.cloudGen = nextGen()
	s.loadingCloud = true
	gen, api := s.cloudGen, s.backup(p)
	p.Env.Async(func() func() {
		items, err := api.GetCloudDBBackups()
		return func() {
			if gen != s.cloudGen {
				return
			}
			s.loadingCloud = false
			s.cloudBackups = items
			if err != nil {
				s.cloudBackups = nil
			}
			if done != nil {
				done()
			}
		}
	})
}

// run 在后台执行一个服务调用，结果回到主线程。
func run(e *env.Env, work func() error, apply func(err error)) {
	e.Async(func() func() {
		err := work()
		return func() { apply(err) }
	})
}

func (s *State) disabled() bool { return s.restoring != "" || s.uploading != "" || s.backingUp }

// createBackup 对应 handleCreateBackup：创建（并按配置上传）后重新读取本地与云端列表再提示。只有云端上传失败时，
// 后端返回“本地备份成功，但云端上传失败”，这时提示本地备份成功与错误本身。
func (s *State) createBackup(p panel.Settings, cfg *appconf.AppConfig, cloudEnabled bool) {
	if s.backingUp {
		return
	}
	s.backingUp = true
	api := s.backup(p)
	autoUpload := cfg != nil && cfg.AutoUploadDBToCloud
	run(p.Env, func() error {
		_, err := api.CreateAndUploadDBBackup()
		return err
	}, func(err error) {
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "本地备份成功") || strings.Contains(msg, "local backup") { // i18n:keep：匹配后端的错误文字
				s.loadLocal(p, func() {
					s.backingUp = false
					s.toast(env.Success(locales.SettingsDbBackupToastLocalBackupSuccess()))
					s.toast(env.Error(msg))
				})
				return
			}
			s.backingUp = false
			s.toast(env.Error(locales.SettingsDbBackupToastBackupFailed(msg)))
			return
		}
		finish := func() {
			s.backingUp = false
			if cloudEnabled && autoUpload {
				s.toast(env.Success(locales.SettingsDbBackupToastBackupSuccessCloud()))
			} else {
				s.toast(env.Success(locales.SettingsDbBackupToastBackupSuccess()))
			}
		}
		s.loadLocal(p, func() {
			if cloudEnabled {
				s.loadCloud(p, finish)
				return
			}
			finish()
		})
	})
}

// askRestore 对应 handleRestoreDB。ScheduleDBRestore 只检查文件并改配置，在主线程调用。
func (s *State) askRestore(p panel.Settings, path string) {
	s.confirm = confirmState{
		open:    true,
		title:   locales.SettingsDbBackupModalRestoreTitle,
		message: locales.SettingsDbBackupModalRestoreMsg,
		onConfirm: func() {
			s.restoring = path
			if err := s.backup(p).ScheduleDBRestore(path); err != nil {
				s.toast(env.Error(locales.SettingsDbBackupToastRestoreScheduleFailed(err.Error())))
				s.restoring = ""
				return
			}
			s.toast(env.Success(locales.SettingsDbBackupToastRestoreScheduled()))
			s.scheduleQuit()
		},
	}
}

// askDelete 对应 handleDeleteDBBackup。
func (s *State) askDelete(p panel.Settings, path string) {
	s.confirm = confirmState{
		open:    true,
		title:   locales.SettingsDbBackupModalDeleteTitle,
		message: locales.SettingsDbBackupModalDeleteMsg,
		danger:  true,
		onConfirm: func() {
			api := s.backup(p)
			run(p.Env, func() error { return api.DeleteDBBackup(path) }, func(err error) {
				if err != nil {
					s.toast(env.Error(locales.SettingsDbBackupToastDeleteFailed(err.Error())))
					return
				}
				s.loadLocal(p, func() { s.toast(env.Success(locales.SettingsDbBackupToastDeleteSuccess())) })
			})
		},
	}
}

// upload 对应 handleUploadDBBackup。
func (s *State) upload(p panel.Settings, path string) {
	s.uploading = path
	api := s.backup(p)
	run(p.Env, func() error { return api.UploadDBBackupToCloud(path) }, func(err error) {
		if err != nil {
			s.uploading = ""
			s.toast(env.Error(locales.SettingsDbBackupToastUploadFailed(err.Error())))
			return
		}
		s.loadCloud(p, func() {
			s.uploading = ""
			s.toast(env.Success(locales.SettingsDbBackupToastUploadSuccess()))
		})
	})
}

// askRestoreCloud 对应 handleRestoreFromCloud：下载云端备份（网络）后安排恢复。
func (s *State) askRestoreCloud(p panel.Settings, key string) {
	s.confirm = confirmState{
		open:    true,
		title:   locales.SettingsDbBackupModalRestoreCloudTitle,
		message: locales.SettingsDbBackupModalRestoreCloudMsg,
		onConfirm: func() {
			s.restoring = key
			api := s.backup(p)
			run(p.Env, func() error { return api.ScheduleDBRestoreFromCloud(key) }, func(err error) {
				if err != nil {
					s.toast(env.Error(locales.SettingsDbBackupToastRestoreScheduleFailed(err.Error())))
					s.restoring = ""
					return
				}
				s.toast(env.Success(locales.SettingsDbBackupToastRestoreScheduled()))
				s.scheduleQuit()
			})
		},
	}
}

// isCloudProviderConfigured 对应 utils/cloudSync.ts 的 isCloudProviderConfigured（cloudbackup 包另有一份，避免面板之间互相依赖）。
func isCloudProviderConfigured(cfg *appconf.AppConfig) bool {
	if cfg == nil || !cfg.CloudBackupEnabled {
		return false
	}
	if cfg.CloudBackupProvider == "umbra" {
		return cfg.UmbraBaseURL != "" && cfg.UmbraAuthenticated
	}
	if cfg.BackupUserID == "" {
		return false
	}
	switch cfg.CloudBackupProvider {
	case "onedrive":
		return cfg.OneDriveRefreshToken != ""
	case "webdav":
		return cfg.WebDAVURL != ""
	}
	return cfg.S3Endpoint != "" && cfg.S3AccessKey != ""
}

// formatTime 对应 formatLocalDateTime(timeString, config?.time_zone)：后端的 RFC3339 字符串。
func formatTime(s string, loc *time.Location) string {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return timefmt.DateTime(tm, loc)
}
