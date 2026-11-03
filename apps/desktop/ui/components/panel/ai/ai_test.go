package ai

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

type harness struct {
	env     *env.Env
	cfg     appconf.AppConfig
	state   State
	tt      *ui.Tester
	changes int
}

// newHarness 打开面板：Change 改本地的配置，外面套一层设置分区的 p-5 space-y-4。
func newHarness(t *testing.T) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{cfg: *svc.Config}
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 1400)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(2)
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	change := func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg); h.changes++ }
	p := panel.Settings{Env: h.env, Config: &h.cfg, Change: change, PatchLive: change, Refresh: func() {}}
	ui.Column(c).Fill().Padding(20).Gap(16).Background(t.PanelBg).Children(func() {
		View(c, t, &h.state, p)
	})
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) find(t *testing.T, s string) ui.Rect {
	t.Helper()
	r, ok := h.tt.Find(s)
	if !ok {
		t.Fatalf("no %q in %q", s, h.tt.Texts())
	}
	return r
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

// typeInto 点击 label 下方的输入框（space-y-2：label 高 20，间距 8），全选后输入 s。
func (h *harness) typeInto(t *testing.T, label, s string) {
	t.Helper()
	r := h.find(t, label)
	h.tt.ClickAt(r.X+40, r.Y+r.H+8+20)
	h.frames(1)
	h.tt.Key(ui.Cmd, ui.KeyA)
	if s == "" {
		h.tt.Key(0, ui.KeyBackspace)
	} else {
		h.tt.Type(s)
	}
	h.frames(2)
}

func TestRenders(t *testing.T) {
	h := newHarness(t)
	for _, s := range []string{
		locales.SettingsAiProviderLabel(), locales.SettingsAiProviderPlaceholder(), "API Base URL", "API Key",
		locales.SettingsAiModelLabel(), locales.SettingsAiSystemPromptLabel(), locales.SettingsAiSystemPromptHint(),
		locales.SettingsAiPromptLabelsDEFAULTSYSTEM(), locales.SettingsAiPromptLabelsMEOWZAKO(), locales.SettingsAiPromptLabelsSTRICTTUTOR(),
		locales.SettingsAiSpoilerLevelLabel(), locales.SettingsAiSpoilerLevelOptsNone(), locales.SettingsAiSpoilerLevelHint(),
		locales.SettingsAiWebSearchLabel(), locales.SettingsAiWebSearchHint(),
		locales.SettingsAiMcpSectionLabel(), locales.SettingsAiMcpEnableLabel(), locales.SettingsAiMcpEnableHint(),
		locales.SettingsAiMcpPortLabel(), locales.SettingsAiMcpPortHint(), locales.SettingsAiMcpEndpointHint("http://127.0.0.1:39200/mcp"),
	} {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
	if h.tt.HasText(locales.SettingsAiTavilyKeyLabel()) {
		t.Error("Tavily key shown while WebSearch is off")
	}
}

func TestProviderAndSpoilerSelects(t *testing.T) {
	h := newHarness(t)
	h.click(t, locales.SettingsAiProviderPlaceholder())
	h.click(t, "DeepSeek")
	if h.cfg.AIProvider != "deepseek" {
		t.Errorf("provider %q", h.cfg.AIProvider)
	}
	h.click(t, "DeepSeek")
	h.click(t, locales.SettingsAiProviderCustom())
	if h.cfg.AIProvider != "custom" {
		t.Errorf("provider %q", h.cfg.AIProvider)
	}
	h.click(t, locales.SettingsAiSpoilerLevelOptsNone())
	h.click(t, locales.SettingsAiSpoilerLevelOptsFull())
	if h.cfg.AISpoilerLevel != "full" {
		t.Errorf("spoiler level %q", h.cfg.AISpoilerLevel)
	}
}

func TestTextFields(t *testing.T) {
	h := newHarness(t)
	h.typeInto(t, "API Base URL", "https://api.deepseek.com")
	h.typeInto(t, "API Key", "sk-test")
	h.typeInto(t, locales.SettingsAiModelLabel(), "deepseek-chat")
	if h.cfg.AIBaseURL != "https://api.deepseek.com" || h.cfg.AIAPIKey != "sk-test" || h.cfg.AIModel != "deepseek-chat" {
		t.Errorf("base url %q key %q model %q", h.cfg.AIBaseURL, h.cfg.AIAPIKey, h.cfg.AIModel)
	}
	// 草稿从别处改变时输入框跟着变。
	h.cfg.AIModel = "gpt-4o"
	h.frames(2)
	h.typeInto(t, locales.SettingsAiModelLabel(), "")
	if h.cfg.AIModel != "" {
		t.Errorf("cleared model %q", h.cfg.AIModel)
	}
}

func TestSystemPrompt(t *testing.T) {
	h := newHarness(t)
	h.click(t, locales.SettingsAiPromptLabelsMEOWZAKO())
	if h.cfg.AISystemPrompt != string(enums.MeowZakoPrompt) {
		t.Errorf("prompt %q", h.cfg.AISystemPrompt)
	}
	h.click(t, locales.SettingsAiPromptLabelsSTRICTTUTOR())
	if h.cfg.AISystemPrompt != string(enums.StrictTutorPrompt) {
		t.Errorf("prompt %q", h.cfg.AISystemPrompt)
	}
	// 文本框在预设按钮下方。
	btn := h.find(t, locales.SettingsAiPromptLabelsSTRICTTUTOR())
	h.tt.ClickAt(btn.X+40, btn.Y+btn.H+4+8+20)
	h.frames(1)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type("hello")
	h.frames(2)
	if h.cfg.AISystemPrompt != "hello" {
		t.Errorf("typed prompt %q", h.cfg.AISystemPrompt)
	}
}

func TestWebSearchAndMCPSwitches(t *testing.T) {
	h := newHarness(t)
	// 原版的 label 没有 htmlFor：点击标签不切换。
	h.click(t, locales.SettingsAiWebSearchLabel())
	if h.cfg.AIWebSearchEnabled {
		t.Fatal("clicking the label toggled WebSearch")
	}
	r := h.find(t, locales.SettingsAiWebSearchLabel())
	// 开关在行的右端：面板 1024 - 2×20 的内容宽，BetterSwitch 宽 44。
	h.tt.ClickAt(1024-20-22, r.Y+r.H)
	h.frames(2)
	if !h.cfg.AIWebSearchEnabled {
		t.Fatalf("WebSearch switch did not turn it on")
	}
	if !h.tt.HasText(locales.SettingsAiTavilyKeyLabel()) || !h.tt.HasText(locales.SettingsAiTavilyKeyHint()) {
		t.Errorf("Tavily key missing in %q", h.tt.Texts())
	}
	h.typeInto(t, locales.SettingsAiTavilyKeyLabel(), "tvly-1")
	if h.cfg.TavilyAPIKey != "tvly-1" {
		t.Errorf("tavily key %q", h.cfg.TavilyAPIKey)
	}

	r = h.find(t, locales.SettingsAiMcpEnableLabel())
	h.tt.ClickAt(1024-20-22, r.Y+r.H)
	h.frames(2)
	if !h.cfg.MCPEnabled {
		t.Error("MCP switch did not turn it on")
	}
}

func TestMCPPort(t *testing.T) {
	h := newHarness(t)
	h.typeInto(t, locales.SettingsAiMcpPortLabel(), "8080")
	if h.cfg.MCPPort != 8080 || !h.tt.HasText(locales.SettingsAiMcpEndpointHint("http://127.0.0.1:8080/mcp")) {
		t.Errorf("port %d, texts %q", h.cfg.MCPPort, h.tt.Texts())
	}
	h.tt.Key(0, ui.KeyUp)
	h.frames(2)
	if h.cfg.MCPPort != 8081 {
		t.Errorf("arrow up: %d", h.cfg.MCPPort)
	}
	// 不是 1–65535 的整数时用默认端口。
	for _, s := range []string{"70000", "0", ""} {
		h.cfg.MCPPort = 8080
		h.frames(1)
		h.typeInto(t, locales.SettingsAiMcpPortLabel(), s)
		if h.cfg.MCPPort != defaultMCPPort {
			t.Errorf("typed %q: port %d", s, h.cfg.MCPPort)
		}
	}
	// number input 不接受字母：输入被忽略。
	h.cfg.MCPPort = 8080
	h.frames(1)
	h.tt.Type("x")
	h.frames(2)
	if h.cfg.MCPPort != 8080 {
		t.Errorf("letter changed the port to %d", h.cfg.MCPPort)
	}
	// 配置里的端口无效时显示默认端口。
	h.cfg.MCPPort = 0
	h.frames(2)
	if !h.tt.HasText(locales.SettingsAiMcpEndpointHint("http://127.0.0.1:39200/mcp")) {
		t.Errorf("endpoint for an invalid port missing in %q", h.tt.Texts())
	}
}

func TestParseNumberInput(t *testing.T) {
	for s, want := range map[string]int{"80": 80, "1e3": 1000, "1.5": 0, "": 0, "-": 0, "8e": 0, "+80": 0, "-5": -5} {
		got, ok := parseNumberInput(s)
		if !ok || got != want {
			t.Errorf("parseNumberInput(%q) = %d, %v; want %d", s, got, ok, want)
		}
	}
	if _, ok := parseNumberInput("8a"); ok {
		t.Error("letters are not accepted")
	}
}
