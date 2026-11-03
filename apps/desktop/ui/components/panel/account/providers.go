package account

import (
	"strings"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/ui/locales"
)

// authView 对应 utils/*Auth.ts 的 BangumiAuthViewState。
type authView int

const (
	authUnauthorized authView = iota
	authAuthorized
	authNeedsReauth
)

// authSnapshot 是三个服务 GetAuthStatus 返回值（vo.*AuthStatus）里卡片用到的部分。
type authSnapshot struct {
	authorized, needsReauth         bool
	userID, username, avatarURL     string
	accessTokenExpiresAt, lastError string
}

// authStatus 对应 utils/*Auth.ts 的 BangumiAuthStatus（mergeBangumiAuthStatus 的结果）。
type authStatus struct {
	state                         authView
	identity                      string
	avatarURL, expiresAt, lastErr string
}

// profileView 是 GetProfile 返回值（vo.*Profile）里卡片用到的部分。avatarExtra 是 Bangumi 的
// avatar_large、avatar_medium、avatar_small（另外两家没有）。
type profileView struct {
	username, nickname, avatarURL string
	avatarExtra                   []string
}

// texts 是每家各自的 settings.basic.<provider>* 文字。存函数，切换语言后随之更新。
type texts struct {
	authHint, needsReauth, unauthorized, lastErrorLabel     func() string
	pushLabel, pushHint, authorize, reauthorize, disconnect func() string
	reconnectHint, disconnectTitle, disconnectMsg           func() string
	authSuccess, disconnectSuccess                          func() string
	actionFailed                                            func(err string) string
	syncLabel, syncTitle, syncMsg                           func() string
}

// logoSpec 是卡片右下角的水印 logo：BangumiAccountSettings 等里 absolute 的 <img>。
type logoSpec struct {
	file          string  // providers/ 下的文件
	w, h          float32 // 显示尺寸（DIP）；h 为 0 时按图片比例（h-auto）
	right, bottom float32
}

// spec 汇总一家账号来源与另外两家不同的地方：文字、图片、配置字段与服务调用。
type spec struct {
	provider Provider
	// name 是没有资料时显示的名称，也是同步进度弹窗里的 providerName。
	name func() string
	// identity 是 utils/*Auth.ts 里 getIdentity 的兜底名称。
	identity string
	letter   string // avatarFallback 的兜底字母
	event    string // *_STATUS_SYNC_PROGRESS_EVENT
	iconBg   bool   // 图标框是否带 bg-white/70 dark:bg-brand-800/70（只有 Bangumi）
	icon     string // 未登录时 48px 图标框里的图片（assets.Provider 的来源名，compact）
	logo     logoSpec
	text     texts

	fromConfig func(cfg *appconf.AppConfig) authSnapshot // getStatusFromConfig 读取的字段
	push       func(cfg *appconf.AppConfig) **bool       // *_status_push_enabled

	status     func(svc *api.Services) (authSnapshot, error)
	profile    func(svc *api.Services) (profileView, error)
	start      func(svc *api.Services) error
	disconnect func(svc *api.Services) error
	sync       func(svc *api.Services) (vo.RemoteStatusSyncProgress, error)
}

func (sp *spec) index() int {
	for i, p := range Providers {
		if p == sp.provider {
			return i
		}
	}
	return 0
}

// specFor 返回 provider 的 spec，未知的来源当作 Bangumi。
func specFor(p Provider) *spec {
	for i := range specs {
		if specs[i].provider == p {
			return &specs[i]
		}
	}
	return &specs[0]
}

var specs = []spec{
	{
		provider: Bangumi,
		name:     func() string { return "Bangumi" },
		identity: "Bangumi",
		letter:   "B",
		event:    "bangumi:status-sync-progress",
		iconBg:   true,
		icon:     "bangumi",
		logo:     logoSpec{file: "bangumi-logo.png", w: 192, right: 16, bottom: 16}, // w-48 h-auto bottom-4 right-4
		text: texts{
			authHint: locales.SettingsBasicBangumiAuthHint, needsReauth: locales.SettingsBasicBangumiAuthNeedsReauth,
			unauthorized: locales.SettingsBasicBangumiAuthUnauthorized, lastErrorLabel: locales.SettingsBasicBangumiAuthLastErrorLabel,
			pushLabel: locales.SettingsBasicBangumiStatusPushLabel, pushHint: locales.SettingsBasicBangumiStatusPushHint,
			authorize: locales.SettingsBasicBangumiAuthorize, reauthorize: locales.SettingsBasicBangumiReauthorize,
			disconnect: locales.SettingsBasicBangumiDisconnect, reconnectHint: locales.SettingsBasicBangumiAuthReconnectHint,
			disconnectTitle: locales.SettingsBasicBangumiDisconnectConfirmTitle, disconnectMsg: locales.SettingsBasicBangumiDisconnectConfirmMsg,
			authSuccess: locales.SettingsBasicBangumiAuthSuccess, disconnectSuccess: locales.SettingsBasicBangumiDisconnectSuccess,
			actionFailed: locales.SettingsBasicBangumiAuthActionFailed,
			syncLabel:    locales.SettingsBasicBangumiSyncAllLabel, syncTitle: locales.SettingsBasicBangumiSyncAllConfirmTitle,
			syncMsg: locales.SettingsBasicBangumiSyncAllConfirmMsg,
		},
		fromConfig: func(cfg *appconf.AppConfig) authSnapshot {
			return configSnapshot(cfg.BangumiAccessToken, cfg.BangumiRefreshToken, cfg.BangumiAuthorizedUsername,
				cfg.BangumiAuthorizedUserID, cfg.BangumiAuthorizedAvatarURL, cfg.BangumiTokenExpiresAt, cfg.BangumiAuthError)
		},
		push: func(cfg *appconf.AppConfig) **bool { return &cfg.BangumiStatusPushEnabled },
		status: func(svc *api.Services) (authSnapshot, error) {
			st, err := svc.Bangumi.GetAuthStatus()
			return authSnapshot{st.Authorized, st.NeedsReauthorization, st.UserID, st.Username, st.AvatarURL,
				st.AccessTokenExpiresAt, st.LastError}, err
		},
		profile: func(svc *api.Services) (profileView, error) {
			pr, err := svc.Bangumi.GetProfile()
			return profileView{pr.Username, pr.Nickname, pr.AvatarURL, []string{pr.AvatarLarge, pr.AvatarMedium, pr.AvatarSmall}}, err
		},
		start: func(svc *api.Services) error {
			_, err := svc.Bangumi.StartAuth()
			return err
		},
		disconnect: func(svc *api.Services) error {
			_, err := svc.Bangumi.Disconnect()
			return err
		},
		sync: func(svc *api.Services) (vo.RemoteStatusSyncProgress, error) {
			return svc.Bangumi.SyncAllGameStatuses()
		},
	},
	{
		provider: Hikarinagi,
		name:     func() string { return "Hikarinagi" },
		identity: "Hikarinagi",
		letter:   "H",
		event:    "hikarinagi:status-sync-progress",
		icon:     "hikarinagi",
		logo:     logoSpec{file: "hikarinagi-logo.svg", w: 192, right: 16, bottom: 16}, // w-48 h-auto bottom-4 right-4
		text: texts{
			authHint: locales.SettingsBasicHikarinagiAuthHint, needsReauth: locales.SettingsBasicHikarinagiAuthNeedsReauth,
			unauthorized: locales.SettingsBasicHikarinagiAuthUnauthorized, lastErrorLabel: locales.SettingsBasicHikarinagiAuthLastErrorLabel,
			pushLabel: locales.SettingsBasicHikarinagiStatusPushLabel, pushHint: locales.SettingsBasicHikarinagiStatusPushHint,
			authorize: locales.SettingsBasicHikarinagiAuthorize, reauthorize: locales.SettingsBasicHikarinagiReauthorize,
			disconnect: locales.SettingsBasicHikarinagiDisconnect, reconnectHint: locales.SettingsBasicHikarinagiAuthReconnectHint,
			disconnectTitle: locales.SettingsBasicHikarinagiDisconnectConfirmTitle, disconnectMsg: locales.SettingsBasicHikarinagiDisconnectConfirmMsg,
			authSuccess: locales.SettingsBasicHikarinagiAuthSuccess, disconnectSuccess: locales.SettingsBasicHikarinagiDisconnectSuccess,
			actionFailed: locales.SettingsBasicHikarinagiAuthActionFailed,
			syncLabel:    locales.SettingsBasicHikarinagiSyncAllLabel, syncTitle: locales.SettingsBasicHikarinagiSyncAllConfirmTitle,
			syncMsg: locales.SettingsBasicHikarinagiSyncAllConfirmMsg,
		},
		fromConfig: func(cfg *appconf.AppConfig) authSnapshot {
			return configSnapshot(cfg.HikarinagiAccessToken, cfg.HikarinagiRefreshToken, cfg.HikarinagiAuthorizedUsername,
				cfg.HikarinagiAuthorizedUserID, cfg.HikarinagiAuthorizedAvatarURL, cfg.HikarinagiTokenExpiresAt, cfg.HikarinagiAuthError)
		},
		push: func(cfg *appconf.AppConfig) **bool { return &cfg.HikarinagiStatusPushEnabled },
		status: func(svc *api.Services) (authSnapshot, error) {
			st, err := svc.Hikarinagi.GetAuthStatus()
			return authSnapshot{st.Authorized, st.NeedsReauthorization, st.UserID, st.Username, st.AvatarURL,
				st.AccessTokenExpiresAt, st.LastError}, err
		},
		profile: func(svc *api.Services) (profileView, error) {
			pr, err := svc.Hikarinagi.GetProfile()
			return profileView{username: pr.Username, nickname: pr.Nickname, avatarURL: pr.AvatarURL}, err
		},
		start: func(svc *api.Services) error {
			_, err := svc.Hikarinagi.StartAuth()
			return err
		},
		disconnect: func(svc *api.Services) error {
			_, err := svc.Hikarinagi.Disconnect()
			return err
		},
		sync: func(svc *api.Services) (vo.RemoteStatusSyncProgress, error) {
			return svc.Hikarinagi.SyncAllGameStatuses()
		},
	},
	{
		provider: Kungal,
		name:     locales.SettingsBasicKungalProviderName,
		identity: "鲲 Galgame", // i18n:keep：原版 kungalAuth.ts 的 getIdentity 写死的品牌名
		letter:   "K",
		event:    "kungal:status-sync-progress",
		icon:     "kungal",
		logo:     logoSpec{file: "kungal-logo.webp", w: 96, h: 96, right: 16, bottom: 12}, // h-24 w-24 object-contain bottom-3 right-4
		text: texts{
			authHint: locales.SettingsBasicKungalAuthHint, needsReauth: locales.SettingsBasicKungalAuthNeedsReauth,
			unauthorized: locales.SettingsBasicKungalAuthUnauthorized, lastErrorLabel: locales.SettingsBasicKungalAuthLastErrorLabel,
			pushLabel: locales.SettingsBasicKungalStatusPushLabel, pushHint: locales.SettingsBasicKungalStatusPushHint,
			authorize: locales.SettingsBasicKungalAuthorize, reauthorize: locales.SettingsBasicKungalReauthorize,
			disconnect: locales.SettingsBasicKungalDisconnect, reconnectHint: locales.SettingsBasicKungalAuthReconnectHint,
			disconnectTitle: locales.SettingsBasicKungalDisconnectConfirmTitle, disconnectMsg: locales.SettingsBasicKungalDisconnectConfirmMsg,
			authSuccess: locales.SettingsBasicKungalAuthSuccess, disconnectSuccess: locales.SettingsBasicKungalDisconnectSuccess,
			actionFailed: locales.SettingsBasicKungalAuthActionFailed,
			syncLabel:    locales.SettingsBasicKungalSyncAllLabel, syncTitle: locales.SettingsBasicKungalSyncAllConfirmTitle,
			syncMsg: locales.SettingsBasicKungalSyncAllConfirmMsg,
		},
		fromConfig: func(cfg *appconf.AppConfig) authSnapshot {
			return configSnapshot(cfg.KungalAccessToken, cfg.KungalRefreshToken, cfg.KungalAuthorizedUsername,
				cfg.KungalAuthorizedUserID, cfg.KungalAuthorizedAvatarURL, cfg.KungalTokenExpiresAt, cfg.KungalAuthError)
		},
		push: func(cfg *appconf.AppConfig) **bool { return &cfg.KungalStatusPushEnabled },
		status: func(svc *api.Services) (authSnapshot, error) {
			st, err := svc.Kungal.GetAuthStatus()
			return authSnapshot{st.Authorized, st.NeedsReauthorization, st.UserID, st.Username, st.AvatarURL,
				st.AccessTokenExpiresAt, st.LastError}, err
		},
		profile: func(svc *api.Services) (profileView, error) {
			pr, err := svc.Kungal.GetProfile()
			return profileView{username: pr.Username, avatarURL: pr.AvatarURL}, err
		},
		start: func(svc *api.Services) error {
			_, err := svc.Kungal.StartAuth()
			return err
		},
		disconnect: func(svc *api.Services) error {
			_, err := svc.Kungal.Disconnect()
			return err
		},
		sync: func(svc *api.Services) (vo.RemoteStatusSyncProgress, error) {
			return svc.Kungal.SyncAllGameStatuses()
		},
	},
}

// configSnapshot 对应 getStatusFromConfig：有 access token 或 refresh token 即已授权，有授权错误即需要重新授权。
func configSnapshot(accessToken, refreshToken, username, userID, avatarURL, expiresAt, lastError string) authSnapshot {
	return authSnapshot{
		authorized:           strings.TrimSpace(accessToken) != "" || strings.TrimSpace(refreshToken) != "",
		needsReauth:          strings.TrimSpace(lastError) != "",
		userID:               userID,
		username:             username,
		avatarURL:            avatarURL,
		accessTokenExpiresAt: expiresAt,
		lastError:            lastError,
	}
}

// mergeAuth 对应 merge*AuthStatus：有服务返回的快照时用快照，否则从配置推导。
func (sp *spec) mergeAuth(cfg *appconf.AppConfig, snapshot *authSnapshot) authStatus {
	src := authSnapshot{}
	if snapshot != nil {
		src = *snapshot
	} else if cfg != nil {
		src = sp.fromConfig(cfg)
	}
	st := authStatus{
		state:     deriveState(src.authorized, src.needsReauth),
		identity:  firstNonEmpty(src.username, src.userID, sp.identity),
		avatarURL: strings.TrimSpace(src.avatarURL),
		expiresAt: strings.TrimSpace(src.accessTokenExpiresAt),
		lastErr:   strings.TrimSpace(src.lastError),
	}
	return st
}

// deriveState 对应 deriveState：needs_reauth 优先于 authorized。
func deriveState(authorized, needsReauth bool) authView {
	switch {
	case needsReauth:
		return authNeedsReauth
	case authorized:
		return authAuthorized
	}
	return authUnauthorized
}

// firstNonEmpty 返回第一个去掉首尾空白后不为空的值（原版的 a?.trim() || b?.trim() || …）。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
