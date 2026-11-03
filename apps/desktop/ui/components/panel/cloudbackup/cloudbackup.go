// Package cloudbackup 对应原版 components/panel/CloudBackupSettingsPanel.tsx（设置页“云配置”分区）：云服务开关、
// 云存储提供商（S3、OneDrive、WebDAV、Umbra）的配置、授权与连接测试，以及设置备份密码的 PasswordInputModal。
// utils/cloudSync.ts 里其他面板也要用的 isCloudProviderConfigured 等在 cloudsync.go。
package cloudbackup

import (
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	testingS3, testingOneDrive, testingUmbra, testingWebDAV bool
	authorizingOneDrive, authorizingUmbra                   bool
	cancellingUmbra, revokingUmbra                          bool

	profile        *vo.UmbraUserProfile
	loadingProfile bool
	profileError   string
	profileGen     int // umbraProfileRequestRef：最近一次读取用户信息的编号，丢弃过期结果

	umbraAuthActive          bool // umbraAuthActiveRef
	umbraAuthCancelRequested bool // umbraAuthCancelRequestedRef
	umbraAuthGen             int  // 离开设置页时递增：之前开始的授权回来时面板已经“卸载”（mountedRef 为 false）

	// deps 是读取 Umbra 用户信息那个 useEffect 的依赖，变化时（以及挂载时）重新执行。
	deps      profileDeps
	depsValid bool

	modal  passwordModalState
	toasts []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）
}

// profileDeps 对应 useEffect 的依赖数组。
type profileDeps struct {
	provider, proxyMode, proxyURL string
	authenticated                 bool
	baseURL                       string
}

var seq atomic.Int64

// nextGen 返回全局递增的请求编号。
func nextGen() int { return int(seq.Add(1)) }

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// View 构建面板：space-y-4 的服务开关、提供商选择、备份密码（S3、OneDrive、WebDAV）、当前提供商的配置卡片，
// 最后是设置备份密码的弹窗。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	cfg := p.Config
	if cfg == nil {
		return
	}
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil

	root := ui.Column(c.Key("cloud-backup-panel")).FillWidth().Gap(16)
	// 原版每次进入设置页（展开分区）都重新挂载面板：读取 Umbra 用户信息的 effect 重新执行，弹窗关闭。
	if mounted := ui.Local(root, "mounted", func() bool { return false }); !*mounted {
		*mounted = true
		s.depsValid = false
		s.modal = passwordModalState{}
	}
	s.profileEffect(p)

	provider := cfg.CloudBackupProvider
	requiresPassword := provider == "s3" || provider == "onedrive" || provider == "webdav"
	root.Children(func() {
		better.SettingSwitchRow(c, t, "service-enabled", better.SettingSwitchRowProps{
			Label:   locales.SettingsCloudBackupServiceEnableLabel(),
			Hint:    locales.SettingsCloudBackupServiceEnableHint(),
			Checked: cfg.CloudBackupEnabled,
			// onServiceEnabledChange：patchLiveConfig({ cloud_backup_enabled })
			OnChange: func(v bool) { p.PatchLive(func(cfg *appconf.AppConfig) { cfg.CloudBackupEnabled = v }) },
		})
		panel.Field(c, t, "provider", locales.SettingsCloudBackupProviderLabel(), "", func() {
			value := provider
			if value == "" {
				value = "s3"
			}
			better.Select(c, t, "select", better.SelectProps{
				Value: value,
				Options: []better.SelectOption{
					{Value: "s3", Label: locales.SettingsCloudBackupProviderS3()},
					{Value: "onedrive", Label: "OneDrive"}, // i18n:keep：品牌名，原版写死
					{Value: "webdav", Label: "WebDAV"},     // i18n:keep：协议名，原版写死
					{Value: "umbra", Label: locales.SettingsCloudBackupProviderUmbra()},
				},
				OnChange: func(v string) { changeProvider(p, v) },
			})
		})
		if requiresPassword {
			s.backupPassword(c, t, p)
		}
		switch provider {
		case "s3":
			s.s3Card(c, t, p)
		case "webdav":
			s.webdavCard(c, t, p)
		case "onedrive":
			s.oneDriveCard(c, t, p)
		case "umbra":
			s.umbraCard(c, t, p)
		}
	})
	if requiresPassword {
		passwordModal(c, t, &s.modal, func(pw, confirm string) { s.setupBackupPassword(p, pw, confirm) })
	}
}

// profileEffect 对应读取 Umbra 用户信息的 useEffect：依赖（提供商、代理、授权状态、服务地址）变化时，
// 已授权的 Umbra 读取用户信息，否则作废进行中的读取。
func (s *State) profileEffect(p panel.Settings) {
	cfg := p.Config
	deps := profileDeps{cfg.CloudBackupProvider, cfg.NetworkProxyMode, cfg.NetworkProxyURL, cfg.UmbraAuthenticated, cfg.UmbraBaseURL}
	if s.depsValid && deps == s.deps {
		return
	}
	s.deps, s.depsValid = deps, true
	if cfg.CloudBackupProvider != "umbra" || !cfg.UmbraAuthenticated || strings.TrimSpace(cfg.UmbraBaseURL) == "" {
		s.profileGen = nextGen()
		s.loadingProfile = false
		return
	}
	s.loadProfile(p.Env, *cfg)
}

// loadProfile 对应 loadUmbraProfile。
func (s *State) loadProfile(e *env.Env, cfg appconf.AppConfig) {
	s.profileGen = nextGen()
	gen := s.profileGen
	s.loadingProfile = true
	s.profile = nil
	s.profileError = ""
	e.Async(func() func() {
		profile, err := e.Svc.Backup.GetUmbraUserProfile(cfg)
		return func() {
			if gen != s.profileGen {
				return
			}
			s.loadingProfile = false
			if err != nil {
				s.profileError = err.Error()
				return
			}
			s.profile = profile
		}
	})
}

// clearProfile 是作废进行中的读取并清空用户信息（umbraProfileRequestRef.current += 1 与两个 set）。
func (s *State) clearProfile() {
	s.profileGen = nextGen()
	s.profile = nil
	s.profileError = ""
}

// run 在后台执行一个服务调用，结果回到主线程。
func run(e *env.Env, work func() error, apply func(err error)) {
	e.Async(func() func() {
		err := work()
		return func() { apply(err) }
	})
}

// changeProvider 对应 handleProviderChange：换提供商时清空上次云同步的状态。
func changeProvider(p panel.Settings, provider string) {
	if provider == p.Config.CloudBackupProvider {
		return
	}
	p.Change(func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = provider
		cfg.LastCloudSyncTime = ""
		cfg.LastCloudSyncStatus = "idle"
		cfg.LastCloudSyncError = ""
	})
}

// setupBackupPassword 对应 handleSetupBackupPassword：SetupCloudBackup 之后重新读取配置，整份交给 onChange。
func (s *State) setupBackupPassword(p panel.Settings, password, confirm string) {
	if jsLength(password) < minPasswordLength {
		s.toast(env.Error(locales.SettingsCloudBackupToastPasswordTooShort()))
		return
	}
	if password != confirm {
		s.toast(env.Error(locales.SettingsCloudBackupToastPasswordMismatch()))
		return
	}
	e := p.Env
	e.Async(func() func() {
		userID, err := e.Svc.Backup.SetupCloudBackup(password)
		var updated appconf.AppConfig
		if err == nil {
			updated, err = e.Svc.AppConfig.GetAppConfig()
		}
		return func() {
			if err != nil {
				s.toast(env.Error(locales.SettingsCloudBackupToastSetupFailed(err.Error())))
				return
			}
			s.toast(env.Success(locales.SettingsCloudBackupToastPasswordSetSuccess(prefix(userID, 8))))
			p.Change(func(cfg *appconf.AppConfig) { *cfg = updated })
		}
	})
}

// testConnection 对应 handleTestS3 / handleTestWebDAV / handleTestOneDrive / handleTestUmbra。
func (s *State) testConnection(p panel.Settings, busy *bool, test func(appconf.AppConfig) error, ok func() string, failed func(string) string) {
	*busy = true
	cfg := *p.Config
	run(p.Env, func() error { return test(cfg) }, func(err error) {
		*busy = false
		if err != nil {
			s.toast(env.Error(failed(err.Error())))
			return
		}
		s.toast(env.Success(ok()))
	})
}

// oneDriveAuth 对应 handleOneDriveAuth：在浏览器里授权（最长等待 5 分钟），拿到 refresh token 后写进草稿。
func (s *State) oneDriveAuth(p panel.Settings) {
	clientID := strings.TrimSpace(p.Config.OneDriveClientID)
	if clientID == "" {
		s.toast(env.Error(locales.SettingsCloudBackupToastOneDriveClientIdRequired()))
		return
	}
	s.authorizingOneDrive = true
	e := p.Env
	e.Async(func() func() {
		token, err := e.Svc.Backup.StartOneDriveAuth(clientID)
		return func() {
			s.authorizingOneDrive = false
			if err != nil {
				s.toast(env.Error(locales.SettingsCloudBackupToastOneDriveAuthFailed(err.Error())))
				return
			}
			p.Change(func(cfg *appconf.AppConfig) { cfg.OneDriveRefreshToken = token })
			s.toast(env.Success(locales.SettingsCloudBackupToastOneDriveAuthSuccess()))
		}
	})
}

// umbraAuth 对应 handleUmbraAuth：OAuth 与设备注册；用户取消等待时提示“已取消”。
func (s *State) umbraAuth(p panel.Settings) {
	if strings.TrimSpace(p.Config.UmbraBaseURL) == "" {
		s.toast(env.Error(locales.SettingsCloudBackupToastUmbraConfigRequired()))
		return
	}
	s.umbraAuthActive = true
	s.umbraAuthCancelRequested = false
	s.authorizingUmbra = true
	cfg, gen := *p.Config, s.umbraAuthGen
	run(p.Env, func() error { return p.Env.Svc.Backup.StartUmbraAuth(cfg) }, func(err error) {
		if gen != s.umbraAuthGen {
			return
		}
		s.umbraAuthActive = false
		s.authorizingUmbra = false
		s.cancellingUmbra = false
		switch {
		case err == nil:
			p.Change(func(cfg *appconf.AppConfig) { cfg.UmbraAuthenticated = true })
			s.toast(env.Success(locales.SettingsCloudBackupToastUmbraAuthSuccess()))
		case s.umbraAuthCancelRequested:
			s.toast(env.Success(locales.SettingsCloudBackupToastUmbraAuthCancelled()))
		default:
			s.toast(env.Error(locales.SettingsCloudBackupToastUmbraAuthFailed(err.Error())))
		}
	})
}

// cancelUmbraAuth 对应 handleCancelUmbraAuth。CancelUmbraAuth 只是取消一个 context，在主线程上调用；
// Go 版没有错误返回，原版的 umbraAuthCancelFailed 提示不会出现。
func (s *State) cancelUmbraAuth(p panel.Settings) {
	s.umbraAuthCancelRequested = true
	s.cancellingUmbra = true
	if !p.Env.Svc.Backup.CancelUmbraAuth() {
		s.umbraAuthCancelRequested = false
		s.cancellingUmbra = false
	}
}

// Leave 对应面板卸载时的 effect cleanup，离开设置页时调用：授权还在进行时取消它。之后授权的结果回来时不提示、
// 不改草稿（原版 mountedRef 为 false 时直接返回），再次进入设置页时按钮回到未授权的状态。
func Leave(s *State, p panel.Settings) {
	if !s.umbraAuthActive {
		return
	}
	s.umbraAuthGen++
	s.umbraAuthActive, s.umbraAuthCancelRequested = false, false
	s.authorizingUmbra, s.cancellingUmbra = false, false
	p.Env.Svc.Backup.CancelUmbraAuth()
}

// umbraLogout 对应 handleUmbraLogout。
func (s *State) umbraLogout(p panel.Settings) {
	s.revokingUmbra = true
	cfg := *p.Config
	run(p.Env, func() error { return p.Env.Svc.Backup.LogoutUmbra(cfg) }, func(err error) {
		s.revokingUmbra = false
		if err != nil {
			s.toast(env.Error(locales.SettingsCloudBackupToastUmbraLogoutFailed(err.Error())))
			return
		}
		p.Change(func(cfg *appconf.AppConfig) { cfg.UmbraAuthenticated = false })
		s.clearProfile()
		s.toast(env.Success(locales.SettingsCloudBackupToastUmbraLogoutSuccess()))
	})
}

// prefix 同 substring(0, n)。用户 ID 是十六进制，按字节截取即可。
func prefix(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
