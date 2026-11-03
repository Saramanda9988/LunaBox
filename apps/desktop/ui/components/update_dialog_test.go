package components

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

func testUpdateInfo() *service.UpdateCheckResult {
	return &service.UpdateCheckResult{
		HasUpdate: true, CurrentVer: "1.0.0", LatestVer: "1.1.0", ReleaseDate: "2026-10-01",
		Changelog:         []string{"line 1", "line 2"},
		Downloads:         map[string]string{"github": "https://github.com/x/releases", "gitee": "https://gitee.com/x/releases"},
		UpdateManifestURL: "https://example.invalid/manifest.json",
	}
}

// macOS、Linux：没有应用内更新，GitHub 与 Gitee 两个下载按钮打开下载页。
func TestUpdateDialogManualDownload(t *testing.T) {
	var st UpdateDialogState
	info := testUpdateInfo()
	th := theme.New(true, 1)
	var skipped []string
	closed := 0
	tt := ui.NewTester(func(c *ui.Context) {
		UpdateDialog(c, th, "update", &st, UpdateDialogProps{
			Info: info, Platform: "darwin",
			OnSkip:  func(v string) { skipped = append(skipped, v) },
			OnClose: func() { closed++ },
		})
	}, 800, 800)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	for _, s := range []string{locales.UpdateDialogGithubDownload(), locales.UpdateDialogGiteeDownload(), "line 1", "line 2", "v1.1.0"} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	if tt.HasText(locales.UpdateDialogUpdateNow()) {
		t.Error("in-app update shown on macOS")
	}
	if err := tt.Click(locales.UpdateDialogGiteeDownload()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := tt.OpenedURLs(); !slices.Equal(got, []string{"https://gitee.com/x/releases"}) {
		t.Errorf("opened %q", got)
	}
	if err := tt.Click(locales.UpdateDialogSkipVersion()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !slices.Equal(skipped, []string{"1.1.0"}) || closed != 1 {
		t.Fatalf("skipped %q, closed %d", skipped, closed)
	}
	if tt.HasText(locales.UpdateDialogSkipVersion()) {
		t.Error("dialog still shown after closing")
	}
}

// 更新中：显示进度与速度，关闭与跳过都被忽略。
func TestUpdateDialogProgress(t *testing.T) {
	st := UpdateDialogState{Updating: true}
	start := time.Unix(1000, 0)
	st.SetProgress(service.UpdateProgress{Phase: "downloading", File: "LunaBox.exe", Downloaded: 0, Total: 4 << 20, Percent: 0}, start)
	st.SetProgress(service.UpdateProgress{Phase: "downloading", File: "LunaBox.exe", Downloaded: 2 << 20, Total: 4 << 20, Percent: 50}, start.Add(2*time.Second))
	if st.Speed != 1<<20 {
		t.Fatalf("speed = %v", st.Speed)
	}
	th := theme.New(false, 1)
	closed, update := 0, 0
	tt := ui.NewTester(func(c *ui.Context) {
		UpdateDialog(c, th, "update", &st, UpdateDialogProps{
			Info: testUpdateInfo(), Platform: "windows",
			OnClose: func() { closed++ }, OnInAppUpdate: func(string) { update++ },
		})
	}, 800, 800)
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	for _, s := range []string{locales.UpdateDialogDownloading(), "50%", "LunaBox.exe", locales.UpdateDialogCurrentSpeed("1.0 MB/s"),
		locales.UpdateDialogDownloadingButton()} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click(locales.UpdateDialogClose()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if closed != 0 || st.closing {
		t.Fatal("closed while updating")
	}

	st.SetProgress(service.UpdateProgress{Phase: "ready"}, start.Add(3*time.Second))
	st.Error = "network down"
	tt.Frame()
	for _, s := range []string{locales.UpdateDialogRestarting(), locales.UpdateDialogUpdateFailed(), "network down"} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	if st.Speed != 0 || tt.HasText("50%") {
		t.Errorf("speed %v after download", st.Speed)
	}

	st.Updating, st.Progress = false, nil
	tt.Frame()
	if err := tt.Click(locales.UpdateDialogUpdateNow()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if update != 1 {
		t.Errorf("in-app update called %d times", update)
	}
}

func TestFormatFileSize(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{0, "0 B"}, {-5, "0 B"}, {math.NaN(), "0 B"}, {math.Inf(1), "0 B"},
		{512, "512.0 B"}, {1024, "1.0 KB"}, {1280, "1.3 KB"}, {1536 * 1024, "1.5 MB"}, {5 << 30, "5.0 GB"},
	} {
		if got := FormatFileSize(tc.in); got != tc.want {
			t.Errorf("FormatFileSize(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
