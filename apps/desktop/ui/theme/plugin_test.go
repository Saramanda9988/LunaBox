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

func TestPluginShapeAndActionTokens(t *testing.T) {
	th := New(true, 1)
	if th.ControlRadius(999) != 999 || th.PanelRadius(12) != 12 {
		t.Fatalf("default radii should fall back: control=%v panel=%v", th.ControlRadius(999), th.PanelRadius(12))
	}
	if th.ActionBtnBg != Brand700 || th.ActionBtnRing != Brand400 || th.MenuSelected != Brand700 || th.MenuCheck != Neutral300 {
		t.Fatal("unexpected dark defaults for action/menu tokens")
	}
	eight, zero := 8.0, 0.0
	th.ApplyPluginOverrides([]plugin.ThemeOverride{{
		Dark:  map[string]string{"actionBtnBg": "#262626", "actionBtnRing": "#45d4c9", "menuSelected": "#00000000", "menuCheck": "#45d4c9"},
		Shape: &plugin.ShapeOverride{Control: &eight, Panel: &zero},
	}})
	if th.ControlRadius(999) != 8 || th.PanelRadius(12) != 0 {
		t.Fatalf("shape override not applied: control=%v panel=%v", th.ControlRadius(999), th.PanelRadius(12))
	}
	if th.ActionBtnBg != ui.Hex("#262626") || th.ActionBtnRing != ui.Hex("#45d4c9") || th.MenuSelected.A != 0 || th.MenuCheck != ui.Hex("#45d4c9") {
		t.Fatalf("action/menu tokens not applied: %v %v %v %v", th.ActionBtnBg, th.ActionBtnRing, th.MenuSelected, th.MenuCheck)
	}
}

func TestPluginBrandOverride(t *testing.T) {
	yes, no := true, false
	ov := []plugin.ThemeOverride{{
		PluginID: "p", ID: "t",
		Brand: &plugin.BrandOverride{
			Wordmark:     "/plugins/p/assets/mark.svg",
			WordmarkDark: "/plugins/p/assets/mark-dark.svg",
			Icon:         "https://example.com/icon.png", // 远程图片不支持，保留内置图标
			HideIcon:     &yes,
			HideWordmark: &no,
		},
	}}

	def := New(false, 1)
	if def.BrandWordmark != "" || def.BrandIcon != "" || def.HideBrandWordmark || def.HideBrandIcon {
		t.Fatalf("default theme has brand overrides: %+v", def)
	}

	light := New(false, 1)
	light.ApplyPluginOverrides(ov)
	if light.BrandWordmark != "/plugins/p/assets/mark.svg" || light.BrandIcon != "" {
		t.Fatalf("light brand = %q / %q", light.BrandWordmark, light.BrandIcon)
	}
	if !light.HideBrandIcon || light.HideBrandWordmark {
		t.Fatalf("light hide flags = wordmark %v icon %v", light.HideBrandWordmark, light.HideBrandIcon)
	}

	dark := New(true, 1)
	dark.ApplyPluginOverrides(ov)
	if dark.BrandWordmark != "/plugins/p/assets/mark-dark.svg" {
		t.Fatalf("dark wordmark = %q", dark.BrandWordmark)
	}
}

func TestThemedFollowsOnlyOverriddenTokens(t *testing.T) {
	th := New(false, 1)
	// 未覆盖时各组件保持原来的固定配色
	if bg, border := th.FieldColors(); bg != White || border != Brand300 {
		t.Fatalf("unexpected default field colors: %v %v", bg, border)
	}
	if th.FieldFocus() != Neutral500 || th.FieldPlaceholder() != Brand400 {
		t.Fatalf("unexpected default focus/placeholder: %v %v", th.FieldFocus(), th.FieldPlaceholder())
	}
	if th.Customized("inputBg") || th.Themed("menuCheck", th.MenuCheck, Black) != Black {
		t.Fatal("tokens should not count as customized before a plugin overrides them")
	}
	if th.DangerBtn != Error500 || th.DangerBtnHover != Error600 || th.DangerBtnText != White || th.SwitchThumb != White {
		t.Fatalf("unexpected light defaults for danger/switch tokens: %v %v %v %v", th.DangerBtn, th.DangerBtnHover, th.DangerBtnText, th.SwitchThumb)
	}

	th.ApplyPluginOverrides([]plugin.ThemeOverride{{Light: map[string]string{
		"inputBg": "#fafafa", "inputFocus": "#39c5bb", "menuCheck": "#1f827b",
		"dangerBtn": "#dc2626", "switchThumb": "#f5f5f5", "notAToken": "#000000",
	}}})
	if bg, border := th.FieldColors(); bg != ui.Hex("#fafafa") || border != Brand300 {
		t.Fatalf("only inputBg should follow the plugin: %v %v", bg, border)
	}
	if th.FieldFocus() != ui.Hex("#39c5bb") || th.FieldPlaceholder() != Brand400 {
		t.Fatalf("focus should follow, placeholder should not: %v %v", th.FieldFocus(), th.FieldPlaceholder())
	}
	if th.Themed("menuCheck", th.MenuCheck, Black) != ui.Hex("#1f827b") || th.Themed("menuSelected", th.MenuSelected, Black) != Black {
		t.Fatal("Themed should follow only overridden tokens")
	}
	if th.DangerBtn != ui.Hex("#dc2626") || th.SwitchThumb != ui.Hex("#f5f5f5") {
		t.Fatalf("danger/switch overrides not applied: %v %v", th.DangerBtn, th.SwitchThumb)
	}
	if th.Customized("notAToken") {
		t.Fatal("unknown keys must not be recorded as customized")
	}
	// 每帧重建的主题不继承上一帧的覆盖记录
	if New(false, 1).Customized("inputBg") {
		t.Fatal("a fresh theme should not carry overrides")
	}
}

func TestPluginMaterialOverride(t *testing.T) {
	ov := []plugin.ThemeOverride{{
		PluginID: "p", ID: "t",
		Light:    map[string]string{"sidebarBg": "#ffffff"},
		Material: &plugin.MaterialOverride{Sidebar: "acrylic", Tint: "#e8f7f5a8", TintDark: "#071c1ab3"},
	}}

	def := New(false, 1)
	if def.SidebarMaterial != "" || def.MaterialOn {
		t.Fatalf("default theme requests a material: %q %v", def.SidebarMaterial, def.MaterialOn)
	}

	light := New(false, 1)
	light.ApplyPluginOverrides(ov)
	if light.SidebarMaterial != MaterialAcrylic || light.SidebarTint != ui.Hex("#e8f7f5a8") {
		t.Fatalf("light material = %q tint %v", light.SidebarMaterial, light.SidebarTint)
	}
	if light.MaterialOn {
		t.Fatal("MaterialOn is set by App once the window shows the material, not by the overrides")
	}

	dark := New(true, 1)
	dark.ApplyPluginOverrides(ov)
	if dark.SidebarTint != ui.Hex("#071c1ab3") {
		t.Fatalf("dark tint = %v", dark.SidebarTint)
	}

	// 没给着色时取最终 sidebarBg 的默认不透明度；未知的材质视为不使用
	noTint := New(false, 1)
	noTint.ApplyPluginOverrides([]plugin.ThemeOverride{{
		PluginID: "p", ID: "t", Light: map[string]string{"sidebarBg": "#123456"},
		Material: &plugin.MaterialOverride{Sidebar: "mica"},
	}})
	if noTint.SidebarMaterial != MaterialMica || noTint.SidebarTint != ui.Hex("#123456").Alpha(defaultSidebarTintAlpha) {
		t.Fatalf("default tint = %q %v", noTint.SidebarMaterial, noTint.SidebarTint)
	}

	// 后注册的插件以未知值关掉材质
	off := New(false, 1)
	off.ApplyPluginOverrides(append(ov, plugin.ThemeOverride{PluginID: "q", ID: "t",
		Material: &plugin.MaterialOverride{Sidebar: "liquid"}}))
	if off.SidebarMaterial != "" {
		t.Fatalf("unknown material kept: %q", off.SidebarMaterial)
	}
}
