package plugins

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

func TestEmptyPluginsPage(t *testing.T) {
	e := &env.Env{
		Theme:     theme.New(false, 1),
		MainWidth: 960,
	}
	page := New(e)
	page.Open()

	tt := ui.NewTester(func(c *ui.Context) {
		page.View(c)
	}, 960, 720)
	tt.Frame()
	if _, ok := tt.Find(locales.NativeSettingsSectionsPlugins()); !ok {
		t.Fatalf("expected plugins page title %q", locales.NativeSettingsSectionsPlugins())
	}
	if _, ok := tt.Find(locales.NativePluginsEmpty()); !ok {
		t.Fatalf("expected empty state text %q", locales.NativePluginsEmpty())
	}
}
