package ui

import (
	"encoding/json"
	"strings"
	"time"

	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
)

// 应用级的服务事件，对应 hooks/useAppRuntimeEffects.ts、useExitSyncToast.ts、useDownloadNotifications.ts 与
// useCoverImageDownloadNotifications.ts。
const (
	eventQuitSyncRequested   = "app:quit-sync-requested" // bootstrap 在退出前同步时发出
	eventProtocolLaunchError = "protocol-launch:error"
	eventCloudSyncStatus     = "cloud-sync:status-changed" // useCloudSync
	eventInstallPending      = "install:pending"           // bootstrap.EventInstallPending 与 CLI 的 IPC 服务
	eventScheduledDBBackup   = "database-backup:scheduled"

	eventDownloadProgress     = "download:progress"
	eventDownloadImported     = "download:game-imported"
	eventDownloadImportFailed = "download:game-import-failed"
	eventDownloadMetaFailed   = "download:metadata-failed"
	eventCoverImageDownload   = "cover-image:download"
	eventUpdateProgress       = "update:progress" // UpdateService.DownloadAndApplyUpdate，交给正在更新的 UpdateDialog
)

// accountEvents 是有授权状态（<name>:auth-status-changed）与游玩状态推送失败事件（<name>:status-push-failed）的账号来源。
var accountEvents = []struct {
	name          string
	failed        func(game, err string) string
	unknownGame   func() string
	unknownReason func() string
}{
	{"bangumi", locales.SettingsBasicBangumiStatusPushFailed,
		locales.SettingsBasicBangumiStatusPushFailedUnknownGame, locales.SettingsBasicBangumiStatusPushFailedUnknownReason},
	{"hikarinagi", locales.SettingsBasicHikarinagiStatusPushFailed,
		locales.SettingsBasicHikarinagiStatusPushFailedUnknownGame, locales.SettingsBasicHikarinagiStatusPushFailedUnknownReason},
	{"kungal", locales.SettingsBasicKungalStatusPushFailed,
		locales.SettingsBasicKungalStatusPushFailedUnknownGame, locales.SettingsBasicKungalStatusPushFailedUnknownReason},
}

// imageDownloadSource 是 IMAGE_DOWNLOAD_SOURCE：批量下载封面的任务。
const imageDownloadSource = "cover-image-batch"

// useExitSyncToast 的 EXIT_SYNC_TOAST_ID、EXIT_SUCCESS_DELAY_MS 与 EXIT_ERROR_DELAY_MS。
const (
	exitSyncToastID  = "exit-sync"
	exitSuccessDelay = 700 * time.Millisecond
	exitErrorDelay   = 1600 * time.Millisecond
)

// toastDismiss 是 notices 里表示 toast.dismiss(id) 的类型。
const toastDismiss = "dismiss"

// subscribe 订阅应用级事件；没有后端服务（只测游戏库的测试）时不订阅。
func (a *App) subscribe() {
	svc := a.env.Svc
	if svc == nil || svc.Events == nil {
		return
	}
	svc.Events.On(eventQuitSyncRequested, func(any) { a.runQuitSync() })
	svc.Events.On(eventProtocolLaunchError, a.protocolLaunchError)
	svc.Events.On(eventCloudSyncStatus, func(data any) {
		provider := ""
		if cfg := a.env.Config(); cfg != nil {
			provider = cfg.CloudBackupProvider
		}
		a.sidebar.Cloud.StatusChanged(provider, data)
		if st, ok := data.(vo.CloudSyncStatus); ok {
			a.settings.ApplyCloudSyncStatus(st)
		}
		a.invalidate()
	})
	svc.Events.On(eventInstallPending, a.installPending)
	svc.Events.On(eventScheduledDBBackup, a.scheduledDBBackup)
	for _, ev := range accountEvents {
		svc.Events.On(ev.name+":auth-status-changed", func(any) {
			a.settings.Reload()
			a.invalidate()
		})
		svc.Events.On(ev.name+":status-push-failed", func(data any) {
			var p vo.BangumiStatusPushFailureEvent // 三种事件的 game_id、game_name、error 相同
			decodeEvent(data, &p)
			game := strings.TrimSpace(p.GameName)
			if game == "" {
				game = ev.unknownGame()
			}
			reason := strings.TrimSpace(p.Error)
			if reason == "" {
				reason = ev.unknownReason()
			}
			id := p.GameID
			if id == "" {
				id = "unknown"
			}
			a.notify(env.Notice{Type: "error", ID: ev.name + "-status-push-failed-" + id, Msg: ev.failed(game, reason)})
		})
	}
	svc.Events.On(eventDownloadProgress, a.downloadProgress)
	svc.Events.On(eventDownloadImported, func(any) {
		a.library.Reload()
		a.home.Refresh()
	})
	svc.Events.On(eventDownloadImportFailed, func(data any) { a.downloadTaskError(data, false) })
	svc.Events.On(eventDownloadMetaFailed, func(data any) { a.downloadTaskError(data, true) })
	svc.Events.On(eventCoverImageDownload, a.coverImageDownload)
	svc.Events.On(eventUpdateProgress, func(data any) {
		var p service.UpdateProgress
		if decodeEvent(data, &p) {
			components.UpdateProgressEvent(p, time.Now())
			a.invalidate()
		}
	})
}

// startup 在窗口创建后做 App.tsx 挂载时的事：取启动期间没处理的安装请求（GetPendingInstall），检查更新（useUpdateCheck）。
func (a *App) startup() {
	svc := a.env.Svc
	if svc == nil {
		return
	}
	if svc.Download != nil {
		if req := svc.Download.GetPendingInstall(); req != nil {
			a.installPending(req)
		}
	}
	if !a.opts.SkipUpdateCheck && svc.Update != nil {
		a.env.Async(func() func() {
			result, err := svc.Update.CheckForUpdatesOnStartup()
			return func() {
				if err != nil {
					// 原版是写死的英文 “Failed to check updates on startup:”，这里用设置页同样的“检查更新失败”。
					a.notify(env.Error(locales.SettingsUpdateCheckFailedTitle() + "\n" + err.Error()))
					return
				}
				if result != nil && result.HasUpdate {
					a.updateInfo, a.updateDialog = result, components.UpdateDialogState{}
				}
			}
		})
	}
}

// skipVersion 对应 useUpdateCheck 的 handleSkipVersion。
func (a *App) skipVersion(ver string) {
	if err := a.env.Svc.Update.SkipVersion(ver); err != nil {
		a.notify(env.Error(locales.SettingsUpdateSkipFailed(err.Error())))
		return
	}
	if a.updateInfo != nil {
		info := *a.updateInfo
		info.HasUpdate = false
		a.updateInfo = &info
	}
}

func (a *App) invalidate() {
	if a.env.Win != nil {
		a.env.Win.Invalidate()
	}
}

// notify 在下一帧显示提示（事件回调里没有 ui.Context）。
func (a *App) notify(n env.Notice) {
	a.notices = append(a.notices, n)
	a.invalidate()
}

// dismiss 在下一帧关闭 id 对应的提示（toast.dismiss）。
func (a *App) dismiss(id string) { a.notify(env.Notice{Type: toastDismiss, ID: id}) }

// decodeEvent 把事件数据转成 v：服务发出的结构体或 map 与 v 字段的 JSON 名称对应（同原版前端收到的 JSON）。
func decodeEvent(data any, v any) bool {
	b, err := json.Marshal(data)
	return err == nil && json.Unmarshal(b, v) == nil
}

// runQuitSync 对应 useExitSyncToast：显示“正在备份数据库”，创建并上传数据库备份，成功或失败都提示后稍等再 SafeQuit。
func (a *App) runQuitSync() {
	if a.quitSyncing {
		return
	}
	a.quitSyncing = true
	a.notify(env.Notice{Type: "loading", ID: exitSyncToastID, Duration: -1,
		Msg: locales.ExitSyncToastSyncingTitle() + "\n" + locales.ExitSyncToastSyncingMessage()})
	svc := a.env.Svc
	a.env.Async(func() func() {
		_, err := svc.Backup.CreateAndUploadDBBackupForQuit()
		return func() {
			delay := exitSuccessDelay
			if err != nil {
				delay = exitErrorDelay
				a.notify(env.Notice{Type: "error", ID: exitSyncToastID, Duration: exitErrorDelay + 1200*time.Millisecond,
					Msg: locales.ExitSyncToastErrorTitle() + "\n" + locales.ExitSyncToastErrorMessage(err.Error())})
			} else {
				a.notify(env.Notice{Type: "success", ID: exitSyncToastID, Duration: exitSuccessDelay + time.Second,
					Msg: locales.ExitSyncToastSuccessTitle() + "\n" + locales.ExitSyncToastSuccessMessage()})
			}
			time.AfterFunc(delay, svc.AppConfig.SafeQuit)
		}
	})
}

// protocolLaunchError 对应 protocol-launch:error：显示窗口并提示快捷启动失败的原因。缺少 Wine 运行器时
// （只在 macOS、Linux 上）提示里带“打开启动配置”按钮，跳到这个游戏详情页的启动标签页。
func (a *App) protocolLaunchError(data any) {
	ev, ok := data.(vo.ProtocolLaunchErrorEvent)
	if !ok {
		return
	}
	msg := strings.TrimSpace(ev.Message)
	detail := strings.TrimSpace(ev.Detail)
	if detail != "" {
		msg += "\n" + detail
	}
	if a.env.Win != nil {
		a.env.Win.Show()
	}
	n := env.Notice{Type: "error", ID: "protocol-launch-error", Msg: msg}
	if ev.Kind == "missing-config" && ev.ConfigKey == "wine_runner" && ev.GameID != "" {
		id := ev.GameID
		n.Action, n.OnAction = locales.GameLaunchOpenLaunchConfig(), func() { a.openGameLaunchSettings(id) }
	}
	a.notify(n)
}

// openGameLaunchSettings 对应 App.tsx 的同名函数：打开游戏详情页的启动标签页。
func (a *App) openGameLaunchSettings(id string) {
	path := env.GamePath(id)
	open := a.router.Location() == path
	a.detail.ShowTab(id, "launch", open)
	if !open {
		a.router.Push(path)
	}
	a.invalidate()
}

// installPending 对应 install:pending 与 GetPendingInstall：显示安装确认弹窗与主窗口。
func (a *App) installPending(data any) {
	var req vo.InstallRequest
	switch v := data.(type) {
	case vo.InstallRequest:
		req = v
	case *vo.InstallRequest:
		if v == nil {
			return
		}
		req = *v
	default:
		return
	}
	a.install.Show(req)
	if a.env.Win != nil {
		a.env.Win.Show()
	}
	a.invalidate()
}

// scheduledDBBackup 对应 database-backup:scheduled：定时数据库备份开始、完成与失败的提示。
func (a *App) scheduledDBBackup(data any) {
	var ev struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	decodeEvent(data, &ev)
	const id = "scheduled-database-backup"
	switch ev.Status {
	case "started":
		a.notify(env.Notice{Type: "loading", ID: id, Duration: -1, Msg: locales.SettingsAutoBackupScheduledDbBackupStarted()})
	case "completed":
		a.notify(env.Notice{Type: "success", ID: id, Msg: locales.SettingsAutoBackupScheduledDbBackupCompleted()})
	case "failed":
		reason := ev.Error
		if reason == "" {
			reason = locales.SettingsAutoBackupUnknownError()
		}
		a.notify(env.Notice{Type: "error", ID: id, Msg: locales.SettingsAutoBackupScheduledDbBackupFailed(reason)})
	}
}

// downloadProgress 对应 useDownloadNotifications 的 download:progress：任务第一次变成完成或出错时提示。
func (a *App) downloadProgress(data any) {
	ev, ok := data.(service.DownloadProgressEvent)
	if !ok {
		return
	}
	prev := a.downloadStatus[ev.ID]
	a.downloadStatus[ev.ID] = ev.Status
	title := strings.TrimSpace(ev.Request.Title)
	if title == "" {
		title = locales.DownloadsUnknownTitle()
	}
	switch {
	case ev.Status == service.DownloadStatusDone && prev != service.DownloadStatusDone:
		n := env.Notice{Type: "success", ID: "download-done-" + ev.ID, Msg: locales.DownloadsToastDownloadCompleted(title)}
		switch {
		case ev.Request.DownloadSource == imageDownloadSource:
			n.Msg = locales.DownloadsImageTaskToastDone()
		case ev.Error == service.DownloadManualExtractFlag:
			n.Msg += "\n" + locales.DownloadsToastManualExtractRequired()
		}
		a.notify(n)
	case ev.Status == service.DownloadStatusError && prev != service.DownloadStatusError:
		msg := locales.DownloadsToastDownloadFailed(title)
		if e := strings.TrimSpace(ev.Error); e != "" {
			msg += "\n" + e
		}
		a.notify(env.Notice{Type: "error", ID: "download-error-" + ev.ID, Msg: msg})
	}
}

// downloadTaskError 对应 download:game-import-failed（自动导入失败，错误提示）与 download:metadata-failed
// （已导入但元数据获取失败，普通提示；原版带 ⚠️ 图标，components.Toaster 还不支持自定义图标）。
func (a *App) downloadTaskError(data any, metadata bool) {
	ev, _ := data.(map[string]string)
	title := strings.TrimSpace(ev["title"])
	if title == "" {
		title = locales.DownloadsUnknownTitle()
	}
	reason := strings.TrimSpace(ev["error"])
	if reason == "" {
		reason = locales.CommonUnknownError()
	}
	key := ev["task_id"]
	if key == "" {
		key = title
	}
	if metadata {
		a.notify(env.Notice{ID: "download-metadata-failed-" + key, Msg: locales.DownloadsToastMetadataFailed(title, reason)})
		return
	}
	a.notify(env.Notice{Type: "error", ID: "download-import-failed-" + key, Msg: locales.DownloadsToastAutoImportFailed(title, reason)})
}

// coverImageDownload 对应 useCoverImageDownloadNotifications：封面图下载中、成功、失败的提示，取消时关闭提示。
// 成功后游戏库重新查询（invalidateAllGameLists），详情页自己换上新封面（game.Page.coverDownloaded）。
func (a *App) coverImageDownload(data any) {
	ev, _ := data.(map[string]string)
	id := "cover-image-" + ev["game_id"]
	switch ev["status"] {
	case "started":
		a.notify(env.Notice{Type: "loading", ID: id, Msg: locales.CoverImageDownloadToastStarted()})
	case "done":
		a.library.Reload()
		a.notify(env.Notice{Type: "success", ID: id, Msg: locales.CoverImageDownloadToastSuccess()})
	case "cancelled":
		a.dismiss(id)
	default:
		msg := locales.CoverImageDownloadToastFailed()
		if e := strings.TrimSpace(ev["error"]); e != "" {
			msg += "\n" + e
		}
		a.notify(env.Notice{Type: "error", ID: id, Msg: msg})
	}
}
