package startup

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop/ui/locales"
)

func TestShowsRestoreProgress(t *testing.T) {
	p := &Page{Restore: &service.RestoreProgress{Stage: service.RestoreStageFiles, Step: 3, Steps: 4, Done: 512, Total: 1024}}
	tt := newTester(p)
	for _, s := range []string{
		locales.NativeStartupRestoreTitle(), locales.NativeStartupRestoreSubtitle(),
		locales.NativeStartupRestoreStep("3", "4"), locales.NativeStartupRestoreFiles(), "50%", "512.0 B / 1.0 KB",
	} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	for _, s := range []string{locales.StartupTitle(), locales.StartupExit()} {
		if tt.HasText(s) {
			t.Errorf("restore view shows %q", s)
		}
	}

	// 进度未知的阶段不显示百分比；恢复结束后显示正在启动、不再显示第几步。
	p.Restore = &service.RestoreProgress{Stage: service.RestoreStageDatabase, Step: 2, Steps: 4}
	tt.Frame()
	if !tt.HasText(locales.NativeStartupRestoreDatabase()) || tt.HasText("0%") {
		t.Errorf("database stage texts = %q", tt.Texts())
	}
	p.Restore = &service.RestoreProgress{Stage: service.RestoreStageDone}
	tt.Frame()
	if !tt.HasText(locales.NativeStartupRestoreStarting()) || tt.HasText(locales.NativeStartupRestoreStep("2", "4")) {
		t.Errorf("done stage texts = %q", tt.Texts())
	}
}

func newTester(p *Page) *ui.Tester {
	tt := ui.NewTester(p.View, Width, Height)
	tt.SetDark(true)
	tt.Frame()
	return tt
}

func TestShowsErrorAndExits(t *testing.T) {
	exited := 0
	p := &Page{Message: "打开数据库失败: IO Error", OnExit: func() { exited++ }}
	tt := newTester(p)
	for _, s := range []string{locales.StartupTitle(), locales.StartupDetail(), locales.StartupErrorSubtitle(), p.Message, locales.StartupExit()} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click(locales.StartupExit()); err != nil {
		t.Fatal(err)
	}
	if exited != 1 {
		t.Fatalf("OnExit called %d times", exited)
	}
}

func TestUnknownError(t *testing.T) {
	tt := newTester(&Page{})
	if !tt.HasText(locales.StartupUnknownError()) {
		t.Errorf("missing unknown error in %q", tt.Texts())
	}
}

func TestCopyShowsCopiedThenResets(t *testing.T) {
	p := &Page{Message: "boom"}
	tt := newTester(p)
	if err := tt.Click(locales.StartupCopyError()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(locales.StartupCopied()); err != nil {
		t.Fatalf("copy button not relabelled: %v", err)
	}
	tt.Advance(copiedFor + 100*time.Millisecond)
	tt.Frame()
	if err := tt.Click(locales.StartupCopyError()); err != nil {
		t.Fatalf("copy button did not reset: %v", err)
	}
}
