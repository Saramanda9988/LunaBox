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

func TestPluginOverridesAccentAndTabTokens(t *testing.T) {
	th := New(false, 1)
	// 默认值与改造前的硬编码颜色一致
	if th.Accent != Primary500 || th.AccentText != Primary600 || th.TabActive != Brand700 || th.TabIndicator != Neutral500 {
		t.Fatalf("unexpected light defaults: accent=%v accentText=%v tabActive=%v tabIndicator=%v",
			th.Accent, th.AccentText, th.TabActive, th.TabIndicator)
	}
	if th.PrimaryOr(Neutral500, Black) != Neutral500 {
		t.Fatal("PrimaryOr should return the default while PrimaryBtn is built-in")
	}
	th.ApplyPluginOverrides([]plugin.ThemeOverride{{Light: map[string]string{
		"accent": "#39c5bb", "accentText": "#1f827b", "accentSoft": "#e7f8f7", "accentSoftText": "#1f827b",
		"tabText": "#525252", "tabHover": "#171717", "tabActive": "#1f827b", "tabIndicator": "#39c5bb",
		"primaryBtn": "#39c5bb",
	}}})
	if th.Accent != ui.Hex("#39c5bb") || th.AccentSoft != ui.Hex("#e7f8f7") || th.AccentSoftText != ui.Hex("#1f827b") {
		t.Fatalf("accent tokens not applied: %v %v %v", th.Accent, th.AccentSoft, th.AccentSoftText)
	}
	if th.TabText != ui.Hex("#525252") || th.TabHover != ui.Hex("#171717") || th.TabActive != ui.Hex("#1f827b") || th.TabIndicator != ui.Hex("#39c5bb") {
		t.Fatalf("tab tokens not applied: %v %v %v %v", th.TabText, th.TabHover, th.TabActive, th.TabIndicator)
	}
	if th.PrimaryOr(Neutral500, Black) != Black {
		t.Fatal("PrimaryOr should return the custom color once PrimaryBtn is overridden")
	}
}
