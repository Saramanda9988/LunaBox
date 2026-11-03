// Package ai 对应原版 components/panel/AISettingsPanel.tsx（设置页“AI 设置”分区）：AI 服务商、接口地址、密钥、模型、
// 系统提示词、防剧透等级、WebSearch 与 MCP 服务。
package ai

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// defaultMCPPort 是 defaultMCPPort（同 appconf.DefaultMCPPort）。
const defaultMCPPort = appconf.DefaultMCPPort

// normalizeMCPPort 对应 normalizeMCPPort：不是 1–65535 的整数时用默认端口。
func normalizeMCPPort(port int) int {
	if port < 1 || port > 65535 {
		return defaultMCPPort
	}
	return port
}

// numberInputValue 是 HTML number input 的合法值（valid floating-point number）。
var numberInputValue = regexp.MustCompile(`^-?(\d+(\.\d+)?|\.\d+)([eE][+-]?\d+)?$`)

// parseNumberInput 对应 number input 的 valueAsNumber 加 Number.isInteger：返回整数值；不是整数（包括清空、"1.5"、"1e"）
// 时返回 0，normalizeMCPPort 再换成默认端口。ok 为 false 表示输入了 number input 不接受的字符，Chromium 不会让它们
// 进入输入框，这时忽略这次输入。
func parseNumberInput(s string) (int, bool) {
	for _, r := range s {
		if !strings.ContainsRune("0123456789+-.eE", r) {
			return 0, false
		}
	}
	if !numberInputValue.MatchString(s) {
		return 0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f != math.Trunc(f) || math.Abs(f) > math.MaxInt32 {
		return 0, true
	}
	return int(f), true
}

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	port string // MCP 端口输入框里的文字
}

// View 构建面板的内容（CollapsibleSection 的 space-y-4 列里的各项；原版根元素也是 space-y-4）。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	cfg := p.Config

	panel.Field(c, t, "ai_provider", locales.SettingsAiProviderLabel(), "", func() {
		better.Select(c, t, "select", better.SelectProps{
			Value: cfg.AIProvider,
			Options: []better.SelectOption{
				{Value: "", Label: locales.SettingsAiProviderPlaceholder()},
				{Value: "openai", Label: "OpenAI"},     // i18n:keep：服务商名称
				{Value: "deepseek", Label: "DeepSeek"}, // i18n:keep：服务商名称
				{Value: "custom", Label: locales.SettingsAiProviderCustom()},
			},
			OnChange: func(v string) { p.Change(func(cfg *appconf.AppConfig) { cfg.AIProvider = v }) },
		})
	})
	textField(c, t, p, "ai_base_url", "API Base URL", "https://api.openai.com/v1", "", false, // i18n:keep：原版写死的英文标签与示例地址
		func(cfg *appconf.AppConfig) *string { return &cfg.AIBaseURL })
	textField(c, t, p, "ai_api_key", "API Key", "sk-...", "", true, // i18n:keep：原版写死的英文标签与示例密钥
		func(cfg *appconf.AppConfig) *string { return &cfg.AIAPIKey })
	textField(c, t, p, "ai_model", locales.SettingsAiModelLabel(), "gpt-3.5-turbo", "", false, // i18n:keep：模型名称
		func(cfg *appconf.AppConfig) *string { return &cfg.AIModel })
	systemPrompt(c, t, p)

	// 防剧透等级
	panel.Field(c, t, "ai_spoiler_level", locales.SettingsAiSpoilerLevelLabel(), locales.SettingsAiSpoilerLevelHint(), func() {
		level := cfg.AISpoilerLevel
		if level == "" {
			level = "none"
		}
		better.Select(c, t, "select", better.SelectProps{
			Value: level,
			Options: []better.SelectOption{
				{Value: "none", Label: locales.SettingsAiSpoilerLevelOptsNone()},
				{Value: "mild", Label: locales.SettingsAiSpoilerLevelOptsMild()},
				{Value: "full", Label: locales.SettingsAiSpoilerLevelOptsFull()},
			},
			OnChange: func(v string) { p.Change(func(cfg *appconf.AppConfig) { cfg.AISpoilerLevel = v }) },
		})
	})

	// WebSearch 开关
	switchRow(c, t, "ai_web_search", locales.SettingsAiWebSearchLabel(), locales.SettingsAiWebSearchHint(), cfg.AIWebSearchEnabled,
		func(v bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.AIWebSearchEnabled = v }) })

	// Tavily API Key（WebSearch 启用时显示）
	if cfg.AIWebSearchEnabled {
		textField(c, t, p, "tavily_api_key", locales.SettingsAiTavilyKeyLabel(), "tvly-...", locales.SettingsAiTavilyKeyHint(), true, // i18n:keep：示例密钥
			func(cfg *appconf.AppConfig) *string { return &cfg.TavilyAPIKey })
	}

	// space-y-2 里的 MCP 小标题
	panel.GroupTitle(c, t, locales.SettingsAiMcpSectionLabel())
	switchRow(c, t, "mcp_enabled", locales.SettingsAiMcpEnableLabel(), locales.SettingsAiMcpEnableHint(), cfg.MCPEnabled,
		func(v bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.MCPEnabled = v }) })
	s.mcpPort(c, t, p)
}

// textField 是 label 加 BetterInput（type="text" 或 "password"）的 space-y-2，可带提示。field 返回配置里对应的字段，
// 输入时写回草稿（handleChange）。
func textField(c *ui.Context, t *theme.Theme, p panel.Settings, key, label, placeholder, hint string, password bool,
	field func(cfg *appconf.AppConfig) *string,
) {
	panel.Field(c, t, key, label, hint, func() {
		v := *field(p.Config)
		better.Input(c, t, "input", &v, better.InputProps{Placeholder: placeholder, Label: label, Password: password})
		if v != *field(p.Config) {
			p.Change(func(cfg *appconf.AppConfig) { *field(cfg) = v })
		}
	})
}

// switchRow 对应 WebSearch、MCP 开关：flex items-center justify-between gap-4，左侧 flex-1 space-y-2 的标签与提示，
// 右侧 BetterSwitch。与 SettingSwitchRow 不同，原版这里的 label 没有 htmlFor，点击标签不切换。
func switchRow(c *ui.Context, t *theme.Theme, key, label, hint string, checked bool, onChange func(bool)) {
	ui.Row(c.Key(key)).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			panel.FieldLabel(c, t, label)
			panel.FieldHint(c, t, hint)
		})
		better.Switch(c, t, "switch", checked, false, onChange)
	})
}

// systemPrompt 对应系统提示词：label、flex flex-wrap gap-2 的预设按钮（enums.PromptType 里非空的值）、rows=4 的文本框与提示。
func systemPrompt(c *ui.Context, t *theme.Theme, p panel.Settings) {
	ui.Column(c.Key("ai_system_prompt")).FillWidth().Gap(8).Children(func() {
		panel.FieldLabel(c, t, locales.SettingsAiSystemPromptLabel())
		ui.Row(c.Key("presets")).FillWidth().Wrap().Gap(8).Children(func() {
			for _, pr := range enums.Prompts {
				if pr.Value == "" {
					continue
				}
				prompt := string(pr.Value)
				promptButton(c, t, pr.TSName, locales.Key("settings.ai.promptLabels."+pr.TSName), func() {
					p.Change(func(cfg *appconf.AppConfig) { cfg.AISystemPrompt = prompt })
				})
			}
		})
		// glass-input w-full px-3 py-2 border rounded-md shadow-sm focus:ring-2 focus:ring-neutral-500 text-sm dark:text-white
		v := p.Config.AISystemPrompt
		ta := better.Textarea(c, t, "textarea", &v, better.TextareaProps{
			Placeholder: locales.SettingsAiSystemPromptPlaceholder(),
			Label:       locales.SettingsAiSystemPromptLabel(),
			Rows:        4,
			Font:        theme.TextSM,
		})
		// <textarea> 是行内块，在 space-y-2 的块里自成一个行盒，基线下方留出字体的降部：Chromium 量出它与下面的提示
		// 相隔 14px（不是 8px），所以底部另加 6px。
		ta.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)).Margin(0, 0, 6, 0)
		if v != p.Config.AISystemPrompt {
			p.Change(func(cfg *appconf.AppConfig) { cfg.AISystemPrompt = v })
		}
		panel.FieldHint(c, t, locales.SettingsAiSystemPromptHint())
	})
}

// promptButton：px-2 py-1 text-xs bg-brand-100 dark:bg-brand-700 text-brand-600 dark:text-brand-300 rounded
// hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors。
func promptButton(c *ui.Context, t *theme.Theme, key, label string, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Padding(4, 8).Radius(4).FocusRing(false)
	bg := t.Pick(theme.Brand100, theme.Brand700)
	if btn.Hovered() {
		bg = t.Pick(theme.Brand200, theme.Brand600)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onClick).Children(func() {
		components.Text(c, label, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300)).NoWrap()
	})
}

// mcpPort 对应 MCP 端口：BetterInput type="number" min=1 max=65535，值总是有效端口（effectiveMCPPort）；
// 输入不是 1–65535 的整数时（包括清空）写入默认端口，与 handleMCPPortChange 的 normalizeMCPPort(valueAsNumber) 相同。
// ↑ / ↓ 同 number input 按 1 增减。下方是端口说明与 MCP 地址。
func (s *State) mcpPort(c *ui.Context, t *theme.Theme, p panel.Settings) {
	effective := normalizeMCPPort(p.Config.MCPPort)
	endpoint := "http://127.0.0.1:" + strconv.Itoa(effective) + "/mcp" // i18n:keep：地址
	ui.Column(c.Key("mcp_port")).FillWidth().Gap(8).Children(func() {
		panel.FieldLabel(c, t, locales.SettingsAiMcpPortLabel())
		s.port = strconv.Itoa(effective) // 受控的 value={effectiveMCPPort}
		prev := s.port
		set := func(port int) {
			if port != p.Config.MCPPort {
				p.Change(func(cfg *appconf.AppConfig) { cfg.MCPPort = port })
			}
		}
		in := better.Input(c, t, "input", &s.port, better.InputProps{Label: locales.SettingsAiMcpPortLabel()})
		in.OnTextKey(func(k ui.TextKey) bool {
			switch k.Key {
			case ui.KeyUp:
				set(min(65535, effective+1))
			case ui.KeyDown:
				set(max(1, effective-1))
			default:
				return false
			}
			return true
		})
		if s.port != prev {
			if port, ok := parseNumberInput(s.port); ok {
				set(normalizeMCPPort(port))
			}
			s.port = strconv.Itoa(normalizeMCPPort(p.Config.MCPPort))
		}
		panel.FieldHint(c, t, locales.SettingsAiMcpPortHint())
		panel.FieldHint(c, t, locales.SettingsAiMcpEndpointHint(endpoint))
	})
}
