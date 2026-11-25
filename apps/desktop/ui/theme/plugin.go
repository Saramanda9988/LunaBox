package theme

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
)

var (
	pluginFontMu     sync.Mutex
	registeredFonts  = map[string]bool{}
	PluginUIFont     string
	PluginMonoFont   string
)

// ApplyPluginOverrides 按插件注册顺序将色板覆盖叠加到当前帧的 *Theme 上。
// 语义名与 Palette 字段一一对应（camelCase）。glass.enabled 显式为 false 时设置 t.BgLocked，
// 由调用方跳过用户的自定义背景（后注册的 glass.enabled 覆盖先注册的）。
// material 同样以后注册者为准，着色未设置或无效时取 sidebarBg 的 defaultSidebarTintAlpha。
func (t *Theme) ApplyPluginOverrides(overrides []plugin.ThemeOverride) (glassBg string, glassBlur int, glassEnabled *bool) {
	var tint string
	for _, ov := range overrides {
		tokens := ov.Light
		if t.Dark {
			tokens = ov.Dark
		}
		for name, rawColor := range tokens {
			col, ok := parsePluginColor(rawColor)
			if !ok {
				continue
			}
			applyPaletteToken(&t.Palette, name, col)
		}
		if ov.Glass != nil {
			if ov.Glass.Enabled != nil {
				v := *ov.Glass.Enabled
				glassEnabled = &v
				t.BgLocked = !v
				if !v {
					t.Glass = false // 覆盖先注册插件打开的玻璃模式
				}
				if v {
					op := float64(t.BgOpacity)
					if ov.Glass.Opacity != nil {
						op = *ov.Glass.Opacity
					}
					if op <= 0 {
						op = 0.85
					}
					t.SetGlass(op)
				}
			} else if ov.Glass.Opacity != nil && t.Glass {
				t.SetGlass(*ov.Glass.Opacity)
			}
			if ov.Glass.Blur != nil {
				glassBlur = *ov.Glass.Blur
			}
			if strings.TrimSpace(ov.Glass.BackgroundImage) != "" {
				glassBg = ov.Glass.BackgroundImage
			}
		}
		if ov.Shape != nil {
			if v := ov.Shape.Control; v != nil && *v >= 0 {
				t.ShapeControl = float32(*v)
			}
			if v := ov.Shape.Panel; v != nil && *v >= 0 {
				t.ShapePanel = float32(*v)
			}
		}
		if b := ov.Brand; b != nil {
			if p := pickBrandImage(t.Dark, b.Wordmark, b.WordmarkDark); p != "" {
				t.BrandWordmark = p
			}
			if p := pickBrandImage(t.Dark, b.Icon, b.IconDark); p != "" {
				t.BrandIcon = p
			}
			if b.HideWordmark != nil {
				t.HideBrandWordmark = *b.HideWordmark
			}
			if b.HideIcon != nil {
				t.HideBrandIcon = *b.HideIcon
			}
		}
		if m := ov.Material; m != nil {
			switch m.Sidebar {
			case MaterialAcrylic, MaterialMica:
				t.SidebarMaterial = m.Sidebar
			default:
				t.SidebarMaterial = ""
			}
			tint = m.Tint
			if t.Dark && strings.TrimSpace(m.TintDark) != "" {
				tint = m.TintDark
			}
		}
	}
	if t.SidebarMaterial != "" {
		// 着色在色板覆盖都叠完之后再定：默认值取最终的 sidebarBg
		if col, ok := parsePluginColor(tint); ok {
			t.SidebarTint = col
		} else {
			t.SidebarTint = t.SidebarBg.Alpha(defaultSidebarTintAlpha)
		}
	}
	return glassBg, glassBlur, glassEnabled
}

// 主题插件可请求的侧边栏材质（material.sidebar）。
const (
	MaterialAcrylic = "acrylic" // 磨砂：模糊窗口后面的内容
	MaterialMica    = "mica"    // 云母：取桌面壁纸的色调，不模糊
)

// defaultSidebarTintAlpha 是插件没有给出着色时，sidebarBg 叠在材质上的不透明度。
const defaultSidebarTintAlpha = 0.6

// pickBrandImage 返回当前深浅色下的品牌图片：深色模式优先用 dark 专用图，没有时与浅色共用 base。
// 远程 URL 不支持（品牌标志每帧绘制，只接受插件目录里的本地图片）。
func pickBrandImage(dark bool, base, darkPath string) string {
	p := strings.TrimSpace(base)
	if dark && strings.TrimSpace(darkPath) != "" {
		p = strings.TrimSpace(darkPath)
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return ""
	}
	return p
}

// SyncPluginFonts 加载并注册插件声明的自定义字体，并更新 PluginUIFont / PluginMonoFont。
func SyncPluginFonts(fonts []plugin.FontRegistration) {
	pluginFontMu.Lock()
	defer pluginFontMu.Unlock()

	uiFont := ""
	monoFont := ""
	for _, f := range fonts {
		key := f.Family + "|" + f.FilePath
		if !registeredFonts[key] {
			data, err := os.ReadFile(f.FilePath)
			if err != nil {
				log.Printf("plugin font read %s failed: %v", f.FilePath, err)
				continue
			}
			if err := ui.RegisterFont(data, f.Family); err != nil {
				log.Printf("plugin font register %s failed: %v", f.Family, err)
				continue
			}
			registeredFonts[key] = true
		}
		switch f.Target {
		case "ui":
			uiFont = f.Family
		case "mono":
			monoFont = f.Family
		}
	}
	PluginUIFont = uiFont
	PluginMonoFont = monoFont
}

// ActiveUIFont 返回当前生效的界面字体（插件覆盖优先，回退到系统 UIFont）。
func ActiveUIFont() string {
	pluginFontMu.Lock()
	f := PluginUIFont
	pluginFontMu.Unlock()
	if f != "" {
		return f
	}
	return UIFont
}

// ActiveMonoFont 返回当前生效的等宽字体。
func ActiveMonoFont() string {
	pluginFontMu.Lock()
	f := PluginMonoFont
	pluginFontMu.Unlock()
	if f != "" {
		return f
	}
	return "Consolas"
}

func parsePluginColor(raw string) (ui.Color, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || !strings.HasPrefix(s, "#") {
		return ui.Color{}, false
	}
	hex := s[1:]
	if len(hex) != 3 && len(hex) != 4 && len(hex) != 6 && len(hex) != 8 {
		return ui.Color{}, false
	}
	return ui.Hex(s), true
}

func applyPaletteToken(p *Palette, key string, c ui.Color) {
	switch key {
	case "text":
		p.Text = c
	case "textStrong":
		p.TextStrong = c
	case "textMuted":
		p.TextMuted = c
	case "textFaint":
		p.TextFaint = c
	case "appBg":
		p.AppBg = c
	case "topbarBg":
		p.TopbarBg = c
	case "topbarBorder":
		p.TopbarBorder = c
	case "controlIcon":
		p.ControlIcon = c
	case "controlHover":
		p.ControlHover = c
	case "sidebarBg":
		p.SidebarBg = c
	case "sidebarBorder":
		p.SidebarBorder = c
	case "navText":
		p.NavText = c
	case "navHover":
		p.NavHover = c
	case "navActiveBg":
		p.NavActiveBg = c
	case "navActive":
		p.NavActive = c
	case "inputBg":
		p.InputBg = c
	case "inputBorder":
		p.InputBorder = c
	case "inputFocus":
		p.InputFocus = c
	case "inputIcon":
		p.InputIcon = c
	case "placeholder":
		p.Placeholder = c
	case "toolBtnBg":
		p.ToolBtnBg = c
	case "toolBtnBorder":
		p.ToolBtnBorder = c
	case "toolBtnHover":
		p.ToolBtnHover = c
	case "toolBtnIcon":
		p.ToolBtnIcon = c
	case "toolBtnText":
		p.ToolBtnText = c
	case "toolBtnActiveBg":
		p.ToolBtnActiveBg = c
	case "toolBtnActiveBorder":
		p.ToolBtnActiveBorder = c
	case "toolBtnActiveText":
		p.ToolBtnActiveText = c
	case "primaryBtn":
		p.PrimaryBtn = c
	case "primaryBtnHover":
		p.PrimaryBtnHover = c
	case "primaryBtnText":
		p.PrimaryBtnText = c
	case "accent":
		p.Accent = c
	case "accentText":
		p.AccentText = c
	case "accentSoft":
		p.AccentSoft = c
	case "accentSoftText":
		p.AccentSoftText = c
	case "tabText":
		p.TabText = c
	case "tabHover":
		p.TabHover = c
	case "tabActive":
		p.TabActive = c
	case "tabIndicator":
		p.TabIndicator = c
	case "actionBtnBg":
		p.ActionBtnBg = c
	case "actionBtnHover":
		p.ActionBtnHover = c
	case "actionBtnIcon":
		p.ActionBtnIcon = c
	case "actionBtnIconHover":
		p.ActionBtnIconHover = c
	case "actionBtnRing":
		p.ActionBtnRing = c
	case "actionBtnRingOffset":
		p.ActionBtnRingOffset = c
	case "cardBg":
		p.CardBg = c
	case "cardBorder":
		p.CardBorder = c
	case "cardBorderHover":
		p.CardBorderHover = c
	case "coverBg":
		p.CoverBg = c
	case "scrollThumb":
		p.ScrollThumb = c
	case "scrollThumbHover":
		p.ScrollThumbHover = c
	case "panelBg":
		p.PanelBg = c
	case "panelBorder":
		p.PanelBorder = c
	case "divider":
		p.Divider = c
	case "menuHover":
		p.MenuHover = c
	case "menuText":
		p.MenuText = c
	case "menuSelected":
		p.MenuSelected = c
	case "menuCheck":
		p.MenuCheck = c
	case "sectionLabel":
		p.SectionLabel = c
	case "chipBg":
		p.ChipBg = c
	case "chipText":
		p.ChipText = c
	case "chipHover":
		p.ChipHover = c
	case "chipActiveBg":
		p.ChipActiveBg = c
	case "chipActiveText":
		p.ChipActiveText = c
	case "chipRing":
		p.ChipRing = c
	case "selectBg":
		p.SelectBg = c
	case "selectBorder":
		p.SelectBorder = c
	case "selectText":
		p.SelectText = c
	case "listFocus":
		p.ListFocus = c
	case "listSelected":
		p.ListSelected = c
	case "switchOff":
		p.SwitchOff = c
	case "switchThumb":
		p.SwitchThumb = c
	case "tooltipBg":
		p.TooltipBg = c
	case "tooltipText":
		p.TooltipText = c
	case "tooltipBorder":
		p.TooltipBorder = c
	case "dangerBtn":
		p.DangerBtn = c
	case "dangerBtnHover":
		p.DangerBtnHover = c
	case "dangerBtnText":
		p.DangerBtnText = c
	default:
		return
	}
	if p.custom == nil {
		p.custom = map[string]bool{}
	}
	p.custom[key] = true
}
