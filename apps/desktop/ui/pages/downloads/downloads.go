// Package downloads 是下载管理页面，对应原版 routes/downloads.tsx：按日期分组的下载任务，
// 暂停、继续、重试、取消、删除记录、复制下载地址、打开所在文件夹与导入为游戏。
package downloads

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"

	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/card"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/timefmt"
)

// 服务发出的事件。
const (
	progressEvent     = "download:progress"
	gameImportedEvent = "download:game-imported"
)

// statusOrder 是 DOWNLOAD_STATUS_ORDER：同一天内按状态排序，未知状态排在 done 的位置。
var statusOrder = map[service.DownloadStatus]int{
	service.DownloadStatusDownloading: 0,
	service.DownloadStatusExtracting:  1,
	service.DownloadStatusPaused:      2,
	service.DownloadStatusPending:     3,
	service.DownloadStatusError:       4,
	service.DownloadStatusDone:        5,
	service.DownloadStatusCancelled:   6,
}

// Page 是下载管理页面的状态，每次进入页面时重置（原版重新挂载组件）。
type Page struct {
	env            *env.Env
	libraryChanged func() // 导入为游戏后刷新游戏库

	tasks     []service.DownloadTask
	importing string          // importingTaskId
	imported  map[string]bool // importedTaskIds
	gen       int             // 每次进入页面加一，丢弃上一次进入时还没返回的结果
	seq       int             // 每次获取任务列表加一，只采用最新的列表
	scroll    components.ScrollBar
	toasts    []func() env.Notice // 待显示的提示（显示时按当前语言取）
}

// New 创建页面并订阅下载事件；任务在 Open 时加载。libraryChanged 可以为 nil。
func New(e *env.Env, libraryChanged func()) *Page {
	p := &Page{env: e, libraryChanged: libraryChanged, imported: map[string]bool{}}
	if e.Svc != nil {
		e.Svc.Events.On(progressEvent, p.applyProgress)
		e.Svc.Events.On(gameImportedEvent, p.markImported)
	}
	return p
}

// Open 进入页面：清空状态并加载任务（loadTasks）。
func (p *Page) Open() {
	p.tasks, p.importing, p.imported = nil, "", map[string]bool{}
	p.toasts = nil
	p.scroll = components.ScrollBar{}
	p.gen++
	p.load(true)
}

// load 对应 loadTasks；checkImported 为 false 时只刷新任务列表（进度事件里的 GetDownloadTasks）。
func (p *Page) load(checkImported bool) {
	p.seq++
	gen, seq := p.gen, p.seq
	p.env.Async(func() func() {
		list := p.env.Svc.Download.GetDownloadTasks()
		var states []vo.DownloadImportState
		var err error
		if checkImported {
			states, err = p.env.Svc.Download.CheckDownloadImportStates(importRequests(list))
		}
		return func() {
			if gen != p.gen {
				return
			}
			if seq == p.seq {
				p.tasks = list
			}
			if checkImported && err == nil { // 原版 markImportedTasks 失败时保留原来的标记
				p.imported = map[string]bool{}
				for _, s := range states {
					if s.Imported {
						p.imported[s.TaskID] = true
					}
				}
			}
		}
	})
}

// importRequests 是 markImportedTasks 的请求：按文件路径或元数据来源判断任务是否已经导入。
func importRequests(tasks []service.DownloadTask) []vo.DownloadImportStateRequest {
	reqs := make([]vo.DownloadImportStateRequest, 0, len(tasks))
	for _, t := range tasks {
		reqs = append(reqs, vo.DownloadImportStateRequest{
			TaskID: t.ID, FilePath: t.FilePath, MetaSource: t.Request.MetaSource, MetaID: t.Request.MetaID,
		})
	}
	return reqs
}

// applyProgress 处理 download:progress：新任务追加到末尾，已有的合并进度；完成时重新获取全部任务。
func (p *Page) applyProgress(data any) {
	ev, ok := data.(service.DownloadProgressEvent)
	if !ok {
		return
	}
	idx := slices.IndexFunc(p.tasks, func(t service.DownloadTask) bool { return t.ID == ev.ID })
	if idx < 0 {
		p.tasks = append(p.tasks, service.DownloadTask{
			ID: ev.ID, Request: ev.Request, Status: ev.Status, CreatedAt: ev.CreatedAt, Progress: ev.Progress,
			Downloaded: ev.Downloaded, Total: ev.Total, Error: ev.Error, FilePath: ev.FilePath,
		})
	} else {
		// { ...prev, ...evt }：事件经 JSON 传给原版时，omitempty 的空字段不会覆盖原来的值。
		t := &p.tasks[idx]
		t.Request, t.Status, t.Progress, t.Downloaded, t.Total = ev.Request, ev.Status, ev.Progress, ev.Downloaded, ev.Total
		if ev.CreatedAt != nil {
			t.CreatedAt = ev.CreatedAt
		}
		if ev.Error != "" {
			t.Error = ev.Error
		}
		if ev.FilePath != "" {
			t.FilePath = ev.FilePath
		}
	}
	if ev.Status == service.DownloadStatusDone {
		p.load(false)
	}
}

// markImported 处理 download:game-imported：把任务标为已导入。
func (p *Page) markImported(data any) {
	ev, _ := data.(map[string]string)
	if id := strings.TrimSpace(ev["task_id"]); id != "" {
		p.imported[id] = true
	}
}

func (p *Page) toast(n func() env.Notice) { p.toasts = append(p.toasts, n) }

// successCopied 是 handleCopyURL 成功的提示。MyGo 写剪贴板不报告失败，所以没有 copyURLFailed。
func successCopied() env.Notice { return env.Success(locales.DownloadsToastCopyURLSuccess()) }

// run 在后台调用服务，失败时提示 failed（为 nil 时不提示，同原版没有 catch 的操作）。
func (p *Page) run(work func() error, failed func() string, done func()) {
	p.env.Async(func() func() {
		err := work()
		return func() {
			switch {
			case err == nil:
				if done != nil {
					done()
				}
			case failed != nil:
				p.toast(func() env.Notice { return env.Error(failed()) })
			}
		}
	})
}

// cancel 对应 handleCancel。
func (p *Page) cancel(id string) {
	p.run(func() error { return p.env.Svc.Download.CancelDownload(id) }, nil, nil)
}

// pause 对应 handlePause。
func (p *Page) pause(id string) {
	p.run(func() error { return p.env.Svc.Download.PauseDownload(id) }, locales.DownloadsToastPauseFailed, nil)
}

// resume 对应 handleResume。
func (p *Page) resume(id string) {
	p.run(func() error { return p.env.Svc.Download.ResumeDownload(id) }, locales.DownloadsToastResumeFailed, nil)
}

// retry 对应 handleRetry。
func (p *Page) retry(id string) {
	p.run(func() error { return p.env.Svc.Download.RetryDownload(id) }, locales.DownloadsToastRetryFailed, nil)
}

// remove 对应 handleDelete：删除成功后从列表里去掉。
func (p *Page) remove(id string) {
	p.run(func() error { return p.env.Svc.Download.DeleteDownloadTask(id) }, nil, func() {
		p.tasks = slices.DeleteFunc(p.tasks, func(t service.DownloadTask) bool { return t.ID == id })
	})
}

// openFolder 对应 handleOpenFolder。
func (p *Page) openFolder(id string) {
	p.run(func() error { return p.env.Svc.Download.OpenDownloadTaskLocation(id) }, locales.DownloadsToastOpenFolderFailed, nil)
}

// importAsGame 对应 handleImportAsGame。服务可能弹出选择可执行文件的对话框，所以在后台调用。
func (p *Page) importAsGame(id string) {
	i := slices.IndexFunc(p.tasks, func(t service.DownloadTask) bool { return t.ID == id })
	if i < 0 || p.tasks[i].Request.DownloadSource == card.ImageDownloadSource {
		return
	}
	switch {
	case p.tasks[i].FilePath == "":
		p.toast(func() env.Notice { return env.Error(locales.DownloadsToastNoFilePath()) })
		return
	case p.imported[id]:
		p.toast(func() env.Notice { return env.Success(locales.DownloadsToastAlreadyImported()) })
		return
	}
	p.importing = id
	p.env.Async(func() func() {
		err := p.env.Svc.Download.ImportDownloadTaskAsGame(id)
		return func() {
			if p.importing == id {
				p.importing = ""
			}
			switch {
			case err == nil:
				p.toast(func() env.Notice { return env.Success(locales.DownloadsToastImportGameSuccess()) })
				p.load(true)
				if p.libraryChanged != nil {
					p.libraryChanged()
				}
			case strings.Contains(err.Error(), "select executable cancelled"):
				// 原版是带 ⚠️ 图标的普通提示；Toaster 不支持自定义图标
				p.toast(func() env.Notice { return env.Info(locales.DownloadsToastSelectExecutableCancelled()) })
			default:
				p.toast(func() env.Notice { return env.Error(locales.DownloadsToastImportGameFailed()) })
			}
		}
	})
}

// activeCount 是下载中、等待中与解压中的任务数。
func (p *Page) activeCount() int {
	n := 0
	for _, t := range p.tasks {
		switch t.Status {
		case service.DownloadStatusDownloading, service.DownloadStatusPending, service.DownloadStatusExtracting:
			n++
		}
	}
	return n
}

// group 是 DownloadTaskGroup：一天（或“更早”）的任务。label 在构建时按当前语言取。
type group struct {
	key   string
	label string
	tasks []service.DownloadTask
}

// groups 对应 taskGroups：按创建时间所在的日期（配置的时区）分组，日期新的在前；同一天内按状态、
// 图片任务在前、创建时间新的在前排序。今天、昨天显示相对日期，其余显示完整日期，没有创建时间的归入“更早”。
func groups(tasks []service.DownloadTask, now time.Time, loc *time.Location) []group {
	type dated struct {
		task      service.DownloadTask
		timestamp int64
		dateKey   string
	}
	list := make([]dated, 0, len(tasks))
	for _, t := range tasks {
		d := dated{task: t}
		if t.CreatedAt != nil {
			d.timestamp = t.CreatedAt.UnixMilli()
			d.dateKey = dateKey(*t.CreatedAt, loc)
		}
		list = append(list, d)
	}
	slices.SortStableFunc(list, func(a, b dated) int {
		if a.dateKey != b.dateKey {
			return cmpDesc(a.timestamp, b.timestamp)
		}
		if n := compareWithinDate(a.task, b.task); n != 0 {
			return n
		}
		return cmpDesc(a.timestamp, b.timestamp)
	})

	today := dateKey(now, loc)
	yesterday := dateKey(now.Add(-24*time.Hour), loc)
	var out []group
	index := map[string]int{}
	for _, d := range list {
		key := d.dateKey
		if key == "" {
			key = "earlier"
		}
		i, ok := index[key]
		if !ok {
			var label string
			switch {
			case d.dateKey == today:
				label = locales.DownloadsDateGroupToday()
			case d.dateKey == yesterday:
				label = locales.DownloadsDateGroupYesterday()
			case d.task.CreatedAt != nil:
				// formatLocalDate(…, { year: "numeric", month: "long", day: "numeric" })
				label = i18n.FormatDate(timefmt.FormatLocale(), d.task.CreatedAt.In(loc), i18n.DateOptions{Skeleton: "yMMMMd"})
			default:
				label = locales.DownloadsDateGroupEarlier()
			}
			i = len(out)
			index[key] = i
			out = append(out, group{key: key, label: label})
		}
		out[i].tasks = append(out[i].tasks, d.task)
	}
	return out
}

// dateKey 对应 formatLocalDateKey：时区里的 YYYY-MM-DD。
func dateKey(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006-01-02") }

// compareWithinDate 对应 compareTasksWithinDate。
func compareWithinDate(a, b service.DownloadTask) int {
	if n := order(a.Status) - order(b.Status); n != 0 {
		return n
	}
	aImage := a.Request.DownloadSource == card.ImageDownloadSource
	bImage := b.Request.DownloadSource == card.ImageDownloadSource
	switch {
	case aImage && !bImage:
		return -1
	case !aImage && bImage:
		return 1
	}
	return 0
}

func order(s service.DownloadStatus) int {
	if n, ok := statusOrder[s]; ok {
		return n
	}
	return 5
}

func cmpDesc(a, b int64) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	}
	return 0
}
