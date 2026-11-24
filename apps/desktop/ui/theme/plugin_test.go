package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
)

func TestApplyPluginOverridesLocksBackground(t *testing.T) {
	off, on := false, true
	cases := []struct {
		name   string
		glass  []*plugin.GlassOverride
		locked bool
	}{
		{"no glass", []*plugin.GlassOverride{nil}, false},
		{"enabled false", []*plugin.GlassOverride{{Enabled: &off}}, true},
		{"later enabled wins", []*plugin.GlassOverride{{Enabled: &off}, {Enabled: &on}}, false},
		{"later disabled wins", []*plugin.GlassOverride{{Enabled: &on}, {Enabled: &off}}, true},
	}
	for _, tc := range cases {
		th := New(false, 1)
		var ovs []plugin.ThemeOverride
		for _, g := range tc.glass {
			ovs = append(ovs, plugin.ThemeOverride{Glass: g})
		}
		th.ApplyPluginOverrides(ovs)
		if th.BgLocked != tc.locked {
			t.Errorf("%s: BgLocked = %v, want %v", tc.name, th.BgLocked, tc.locked)
		}
		if tc.locked && th.Glass {
			t.Errorf("%s: glass mode should stay off when locked", tc.name)
		}
	}
}

func TestApplyPluginOverridesPrimaryBtnText(t *testing.T) {
	th := New(true, 1)
	if th.PrimaryBtnText != White {
		t.Fatalf("default PrimaryBtnText = %v, want white", th.PrimaryBtnText)
	}
	th.ApplyPluginOverrides([]plugin.ThemeOverride{{Dark: map[string]string{"primaryBtnText": "#000000"}}})
	if th.PrimaryBtnText != ui.Hex("#000000") {
		t.Fatalf("PrimaryBtnText = %v, want #000000", th.PrimaryBtnText)
	}
}
