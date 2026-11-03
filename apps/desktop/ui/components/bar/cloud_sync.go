package bar

import (
	"fmt"
	"log"
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components/panel/cloudbackup"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
)

// cloudSyncToastID 是“正在同步”提示的 id：同步结束后换成成功或失败的提示。
const cloudSyncToastID = "cloud-sync-now"

// CloudSync 对应 hooks/useCloudSync.ts：侧边栏云同步按钮读到的同步状态与“立即同步”。
type CloudSync struct {
	status   *vo.CloudSyncStatus // providerSyncStatus.status
	provider string              // providerSyncStatus.provider：读到状态时的云服务提供商
	syncing  bool                // syncingNow
	deps     string              // 上一次 refresh 时依赖的配置（原版 effect 的依赖数组）
	hovered  bool                // 上一帧按钮是否悬停，进入悬停时刷新（onMouseEnter）
}

// StatusChanged 对应 cloud-sync:status-changed 事件，由 App 订阅（主线程）。服务已经把结果写进共用的配置
// （store 的 applyCloudSyncStatus），这里只记下进度。
func (s *CloudSync) StatusChanged(provider string, data any) {
	status, ok := data.(vo.CloudSyncStatus)
	if !ok {
		return
	}
	s.status, s.provider = &status, provider
}

// refresh 对应 refreshSyncStatus：服务与同步都开启时读取同步状态，否则清空。GetCloudSyncStatus 只读内存里的状态，
// 在主线程同步调用。
func (s *CloudSync) refresh(e *env.Env, cfg *appconf.AppConfig) {
	if cfg == nil || !cfg.CloudBackupEnabled || !cfg.CloudSyncEnabled || e == nil || e.Svc == nil || e.Svc.CloudSync == nil {
		s.status = nil
		return
	}
	status := e.Svc.CloudSync.GetCloudSyncStatus()
	s.status, s.provider = &status, cfg.CloudBackupProvider
}

// track 在配置里与同步有关的字段变化后刷新状态（原版 useEffect 的依赖）；hovered 从无到有时也刷新。
func (s *CloudSync) track(e *env.Env, cfg *appconf.AppConfig, hovered bool) {
	deps := ""
	if cfg != nil {
		deps = fmt.Sprint(cfg.BackupUserID, cfg.CloudBackupEnabled, cfg.CloudBackupProvider, cfg.CloudSyncEnabled,
			cfg.AutoCloudSyncEnabled, cfg.LastCloudSyncError, cfg.LastCloudSyncStatus, cfg.LastCloudSyncTime,
			cfg.OneDriveRefreshToken != "", cfg.S3AccessKey, cfg.S3Endpoint, cfg.TimeZone, cfg.UmbraAuthenticated,
			cfg.UmbraBaseURL, cfg.WebDAVURL)
	}
	enter := hovered && !s.hovered
	s.hovered = hovered
	if deps != s.deps || enter {
		s.deps = deps
		s.refresh(e, cfg)
	}
}

// effective 对应 getEffectiveCloudSyncStatus：读到当前提供商的状态时用它，否则由配置拼出。
func (s *CloudSync) effective(cfg *appconf.AppConfig) vo.CloudSyncStatus {
	provider := ""
	if cfg != nil {
		provider = cfg.CloudBackupProvider
	}
	if s.status != nil && s.provider == provider {
		return *s.status
	}
	st := vo.CloudSyncStatus{LastSyncStatus: "idle", Configured: cloudbackup.ProviderConfigured(cfg)}
	if cfg != nil {
		st.Enabled = cfg.CloudSyncEnabled
		st.LastSyncTime = cfg.LastCloudSyncTime
		if cfg.LastCloudSyncStatus != "" {
			st.LastSyncStatus = cfg.LastCloudSyncStatus
		}
		st.LastSyncError = cfg.LastCloudSyncError
	}
	return st
}

// configured 对应 syncConfigured；busy 对应 syncBusy。
func (*CloudSync) configured(cfg *appconf.AppConfig, st vo.CloudSyncStatus) bool {
	return st.Configured || cloudbackup.ProviderConfigured(cfg)
}

func (s *CloudSync) busy(st vo.CloudSyncStatus) bool { return s.syncing || st.Syncing }

// canSyncNow 对应 canSyncNow。
func (s *CloudSync) canSyncNow(cfg *appconf.AppConfig) bool {
	st := s.effective(cfg)
	return cfg != nil && cfg.CloudBackupEnabled && cfg.CloudSyncEnabled && s.configured(cfg, st) && !s.busy(st)
}

// syncNow 对应 handleSyncNow：提示“正在同步”，在后台 SyncNow，结束后提示结果并重新读取状态。
func (s *CloudSync) syncNow(c *ui.Context, e *env.Env, cfg *appconf.AppConfig) {
	if !s.canSyncNow(cfg) || e == nil || e.Svc == nil || e.Svc.CloudSync == nil {
		return
	}
	s.syncing = true
	provider := cfg.CloudBackupProvider
	e.Toast(c, env.Notice{Type: "loading", ID: cloudSyncToastID, Msg: locales.SettingsCloudBackupSyncingNow()})
	svc := e.Svc.CloudSync
	e.Async(func() func() {
		status, err := svc.SyncNow()
		return func() {
			s.syncing = false
			if err != nil {
				log.Printf("cloud sync: %v", err)
				e.Toast(c, env.Notice{Type: "error", ID: cloudSyncToastID, Msg: locales.SettingsCloudBackupToastSyncFailed(err.Error())})
			} else {
				s.status, s.provider = &status, provider
				e.Toast(c, env.Notice{Type: "success", ID: cloudSyncToastID, Msg: locales.SettingsCloudBackupToastSyncSuccess()})
			}
			s.refresh(e, e.Config())
		}
	})
}

// cloudSyncDetail 对应 cloudSyncDetail：同步中按阶段显示的进度。
func cloudSyncDetail(st vo.CloudSyncStatus) string {
	cur, total := strconv.Itoa(st.SyncCurrent), strconv.Itoa(st.SyncTotal)
	switch st.SyncStage {
	case "reading_remote":
		if st.SyncTotal > 0 {
			return locales.SettingsCloudBackupSyncDetailDownloadingChunks(cur, total)
		}
		return locales.SettingsCloudBackupSyncDetailReadingRemote()
	case "uploading_covers":
		return locales.SettingsCloudBackupSyncDetailUploadingCovers(cur, total)
	case "uploading_files":
		return locales.SettingsCloudBackupSyncDetailUploadingFiles(cur, total)
	case "finalizing":
		return locales.SettingsCloudBackupSyncDetailFinalizing()
	}
	return locales.SettingsCloudBackupSyncDetailPreparing()
}
