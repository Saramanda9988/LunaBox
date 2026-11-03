package startup

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
)

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
