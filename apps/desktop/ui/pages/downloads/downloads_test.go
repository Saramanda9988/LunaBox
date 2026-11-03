package downloads

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/card"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

type harness struct {
	*Page
	svc       *host.Backend
	tt        *ui.Tester
	reloads   int
	announced []string
}

// newHarness 写入下载任务后打开页面。服务的事件经 host.Events 同步分发给页面。
func newHarness(t *testing.T, tasks ...service.DownloadTask) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	svc.Config.TimeZone = ""
	svc.SetRuntime(&host.Runtime{Events: svc.Events})
	for _, task := range tasks {
		insertTask(t, svc, task)
	}
	svc.Download.Init(context.Background(), svc.DB, svc.Config) // 重新从数据库加载任务

	h := &harness{svc: svc}
	e := &env.Env{Backend: envtest.NewBackend(0), Svc: svc.API(), Router: ui.NewRouter(env.PathDownloads), Covers: covers.New(),
		Theme: theme.New(true, 1.5), MainWidth: 1100}
	h.Page = New(e, func() { h.reloads++ })
	h.Open()
	h.tt = ui.NewTester(h.View, 1280, 800)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(2)
	return h
}

func insertTask(t *testing.T, svc *host.Backend, task service.DownloadTask) {
	t.Helper()
	req, err := json.Marshal(task.Request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DB.Exec(`INSERT INTO download_tasks (id, request_json, status, progress, downloaded, total, error, file_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, task.ID, string(req), string(task.Status), task.Progress, task.Downloaded, task.Total,
		task.Error, task.FilePath, task.CreatedAt, task.CreatedAt); err != nil {
		t.Fatal(err)
	}
}

func at(t time.Time) *time.Time { return &t }

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, label string) {
	t.Helper()
	if err := h.tt.Click(label); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) expectNot(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if h.tt.HasText(s) {
			t.Errorf("unexpected %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) expectToast(t *testing.T, msg string) {
	t.Helper()
	h.announced = append(h.announced, h.tt.Announcements()...)
	if !slices.ContainsFunc(h.announced, func(s string) bool { return strings.Contains(s, msg) }) {
		t.Errorf("no toast %q in %q", msg, h.announced)
	}
}

func TestEmpty(t *testing.T) {
	h := newHarness(t)
	h.expect(t, locales.DownloadsTitle(), locales.DownloadsNoActive(), locales.DownloadsEmpty())
}

func TestShowsTasksByDate(t *testing.T) {
	now := time.Now()
	h := newHarness(t,
		service.DownloadTask{ID: "done", Request: vo.InstallRequest{Title: "完成的游戏", DownloadSource: "Shionlib", URL: "https://example.com/a.zip"},
			Status: service.DownloadStatusDone, CreatedAt: at(now), Progress: 100, FilePath: `D:\Games\done`},
		service.DownloadTask{ID: "failed", Request: vo.InstallRequest{Title: "失败的游戏"}, Status: service.DownloadStatusError,
			CreatedAt: at(now), Error: "connection reset"},
		service.DownloadTask{ID: "nodate", Request: vo.InstallRequest{DownloadSource: card.ImageDownloadSource}, Status: service.DownloadStatusDone},
	)
	h.expect(t, locales.DownloadsDateGroupToday(), locales.DownloadsDateGroupEarlier(),
		"完成的游戏", "Shionlib", `D:\Games\done`, locales.DownloadsStatusDone(),
		"失败的游戏", locales.DownloadsUnknownSource(), "connection reset", locales.DownloadsStatusError(),
		locales.DownloadsUnknownTitle(), locales.DownloadsImageTaskSource(), locales.DownloadsImageTaskDone(),
		locales.DownloadsImportAsGame(), locales.DownloadsRetry(), locales.DownloadsDelete())
}

// 分组与排序（taskGroups）：日期新的组在前；同一天内按状态，图片任务在前，再按创建时间新的在前；没有创建时间的归入“更早”。
func TestGroupsAndOrder(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, loc)
	tasks := []service.DownloadTask{
		{ID: "old", Status: service.DownloadStatusDone, CreatedAt: at(time.Date(2026, 9, 1, 8, 0, 0, 0, loc))},
		{ID: "nodate", Status: service.DownloadStatusDone},
		{ID: "today-done", Status: service.DownloadStatusDone, CreatedAt: at(now.Add(-time.Hour))},
		{ID: "today-done-new", Status: service.DownloadStatusDone, CreatedAt: at(now.Add(-time.Minute))},
		{ID: "today-image", Status: service.DownloadStatusDone, CreatedAt: at(now.Add(-2 * time.Hour)),
			Request: vo.InstallRequest{DownloadSource: card.ImageDownloadSource}},
		{ID: "today-error", Status: service.DownloadStatusError, CreatedAt: at(now.Add(-3 * time.Hour))},
		{ID: "yesterday", Status: service.DownloadStatusCancelled, CreatedAt: at(now.Add(-20 * time.Hour))},
	}
	gs := groups(tasks, now, loc)
	var got []string
	for _, g := range gs {
		var ids []string
		for _, task := range g.tasks {
			ids = append(ids, task.ID)
		}
		got = append(got, g.label+":"+strings.Join(ids, ","))
	}
	want := []string{
		locales.DownloadsDateGroupToday() + ":today-error,today-image,today-done-new,today-done",
		locales.DownloadsDateGroupYesterday() + ":yesterday",
		"2026年9月1日:old",
		locales.DownloadsDateGroupEarlier() + ":nodate",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("groups = %q, want %q", got, want)
	}
}

// download:progress：新任务出现在列表里，下载中显示速度、已下载与总大小，暂停失败时提示。
func TestProgressEvents(t *testing.T) {
	h := newHarness(t)
	const mb = 1 << 20
	ev := service.DownloadProgressEvent{ID: "live", Request: vo.InstallRequest{Title: "下载中的游戏", URL: "https://example.com/b.zip"},
		Status: service.DownloadStatusDownloading, CreatedAt: at(time.Now()), Progress: 10, Downloaded: mb, Total: 10 * mb}
	h.svc.Events.Emit(progressEvent, ev)
	h.frames(1)
	h.expect(t, "下载中的游戏", locales.DownloadsStatusDownloading(), locales.DownloadsActiveCount("1"), "10.0%",
		locales.DownloadsCurrentSpeed("0 B/s")+" · 1.0 MB / 10.0 MB", locales.DownloadsPause(), locales.DownloadsCancel())

	h.tt.Advance(time.Second)
	ev.Downloaded, ev.Progress = 2*mb, 20
	h.svc.Events.Emit(progressEvent, ev)
	h.frames(1)
	h.expect(t, "20.0%", locales.DownloadsCurrentSpeed("1.0 MB/s")+" · 2.0 MB / 10.0 MB")

	// 服务里没有这个任务，暂停失败
	h.click(t, locales.DownloadsPause())
	h.expectToast(t, locales.DownloadsToastPauseFailed())
}

func TestCancelPausedTask(t *testing.T) {
	h := newHarness(t, service.DownloadTask{ID: "paused", Request: vo.InstallRequest{Title: "暂停的游戏", FileName: "paused.zip"},
		Status: service.DownloadStatusPaused, CreatedAt: at(time.Now()), Progress: 40, Downloaded: 400, Total: 1000})
	h.expect(t, locales.DownloadsStatusPaused(), locales.DownloadsResume(), "40.0%", "400.0 B / 1000.0 B")
	h.click(t, locales.DownloadsCancel())
	h.expect(t, locales.DownloadsStatusCancelled(), locales.DownloadsDelete())
	h.expectNot(t, locales.DownloadsResume())
}

func TestDeleteAndCopyURL(t *testing.T) {
	h := newHarness(t, service.DownloadTask{ID: "cancelled", Request: vo.InstallRequest{Title: "取消的游戏", URL: "https://example.com/c.zip"},
		Status: service.DownloadStatusCancelled, CreatedAt: at(time.Now())})
	h.click(t, locales.DownloadsCopyURL())
	if got := h.tt.Clipboard(); got != "https://example.com/c.zip" {
		t.Errorf("clipboard = %q", got)
	}
	h.expectToast(t, locales.DownloadsToastCopyURLSuccess())

	h.click(t, locales.DownloadsDelete())
	h.expect(t, locales.DownloadsEmpty())
	if n := len(h.svc.Download.GetDownloadTasks()); n != 0 {
		t.Errorf("%d tasks left after delete", n)
	}
}

// 导入为游戏：启动路径来自请求，不弹出选择可执行文件的对话框；导入后按钮变成“已导入”，再次进入页面时仍是已导入。
func TestImportAsGame(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "game.exe")
	if err := os.WriteFile(exe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, service.DownloadTask{ID: "done", Request: vo.InstallRequest{Title: "导入的游戏", StartupPath: "game.exe"},
		Status: service.DownloadStatusDone, CreatedAt: at(time.Now()), Progress: 100, FilePath: exe})
	h.click(t, locales.DownloadsImportAsGame())
	h.expectToast(t, locales.DownloadsToastImportGameSuccess())
	h.expect(t, locales.DownloadsImported())
	if h.reloads != 1 {
		t.Errorf("library reloaded %d times, want 1", h.reloads)
	}
	var n int
	if err := h.svc.DB.QueryRow(`SELECT COUNT(*) FROM games WHERE name = ?`, "导入的游戏").Scan(&n); err != nil || n != 1 {
		t.Fatalf("imported games = %d, %v", n, err)
	}

	h.Open()
	h.frames(2)
	h.expect(t, locales.DownloadsImported())
}
