package pluginui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

func TestRenderUITreeAndErrorBoundary(t *testing.T) {
	e := &env.Env{MainWidth: 800}
	mgr := New(e, HostCallbacks{})
	mgr.DevMode = true

	inst := newInstance("inst-1", "demo", plugin.SlotSettingsSection, "sec", "")
	inst.Version = 1
	inst.Tree = &plugin.UINode{
		Type:  "Column",
		Key:   "root",
		Props: map[string]any{"gap": "md"},
		Children: []*plugin.UINode{
			{Type: "Text", Props: map[string]any{"text": "Plugin Title", "size": "lg", "weight": "bold"}},
			{Type: "Input", Key: "q", Props: map[string]any{"value": "initial", "placeholder": "Search"}, Events: map[string]string{"change": "h1"}},
			{Type: "Button", Key: "go", Props: map[string]any{"label": "Run", "variant": "primary"}, Events: map[string]string{"press": "h2"}},
			{Type: "Markdown", Key: "md", Props: map[string]any{"content": "# Heading\n\n- item 1\n> quote"}},
			{Type: "Badge", Key: "b1", Props: map[string]any{"label": "OK", "tone": "success"}},
			{Type: "Progress", Key: "p1", Props: map[string]any{"value": 0.5, "label": "50%"}},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(false, 1)
		mgr.RenderInstance(c, th, inst)
	}, 600, 400)
	tt.Advance(20 * time.Millisecond)

	if st, ok := inst.Inputs["q"]; !ok || st.Value != "initial" {
		t.Fatalf("expected input state to be initialized to 'initial', got %+v", st)
	}

	// 测试错误边界占位
	errInst := newInstance("inst-err", "broken-plugin", plugin.SlotSettingsSection, "err", "")
	errInst.Error = "ReferenceError: boom"
	tt2 := ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1)
		mgr.RenderInstance(c, th, errInst)
	}, 600, 200)
	tt2.Advance(20 * time.Millisecond)

	if want := locales.NativePluginsUiError("broken-plugin"); want == "" {
		t.Fatalf("expected non-empty error boundary message")
	}
}

func TestRenderSpringPressBox(t *testing.T) {
	e := &env.Env{MainWidth: 800}
	mgr := New(e, HostCallbacks{})

	inst := newInstance("inst-whale", "whale-pet", plugin.SlotAppDecoration, "whale-pet-mascot", "")
	inst.Version = 1
	inst.Tree = &plugin.UINode{
		Type: "Box",
		Key:  "whale-spring-box",
		Props: map[string]any{
			"width":       "full",
			"height":      "full",
			"springPress": true,
			"squishX":     1.05,
			"squishY":     0.88,
			"flipX":       true,
		},
		Events: map[string]string{
			"pressIn":  "h1",
			"pressOut": "h2",
		},
		Children: []*plugin.UINode{
			{
				Type: "Image",
				Key:  "whale-img",
				Props: map[string]any{
					"src":         "",
					"width":       "full",
					"height":      "full",
					"fit":         "contain",
					"transparent": true,
				},
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(false, 1)
		ui.Box(c).Size(136, 136).Children(func() {
			mgr.RenderInstance(c, th, inst)
		})
	}, 200, 200)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
}

func TestRenderHeroBannerHalfStarAndSafeIcon(t *testing.T) {
	e := &env.Env{MainWidth: 1200}
	mgr := New(e, HostCallbacks{})

	inst := newInstance("inst-hero", "hikarinagi-detail", plugin.SlotGameDetailHero, "hikarinagi-hero", "g-1")
	inst.Version = 1
	inst.Tree = &plugin.UINode{
		Type: "Column",
		Key:  "root",
		Children: []*plugin.UINode{
			{
				Type: "HeroBanner",
				Key:  "hero-g1",
				Props: map[string]any{
					"height":   460.0,
					"title":    "樱之诗",
					"subtitle": "サクラノ詩",
					"meta": []any{
						map[string]any{"icon": "palette", "text": "枕"},
						map[string]any{"icon": "nonexistent-icon-xyz", "text": "回退测试"},
					},
					"rating": map[string]any{
						"score":  8.9, // 4.45 stars -> triggers star-half-full
						"total":  420.0,
						"played": 1200.0,
					},
					"pager": map[string]any{
						"current": 2.0,
						"total":   6.0,
					},
				},
				Events: map[string]string{
					"prev": "h-prev",
					"next": "h-next",
				},
			},
			{
				Type: "Icon",
				Key:  "icon-missing",
				Props: map[string]any{
					"name": "definitely-missing-icon",
					"size": 16.0,
					"tone": "warning",
				},
			},
			{
				Type: "Icon",
				Key:  "icon-half-star",
				Props: map[string]any{
					"name": "star-half-full",
					"size": 16.0,
					"tone": "warning",
				},
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1)
		mgr.RenderInstance(c, th, inst)
	}, 1200, 600)
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
}

func TestLoadImageSourceUntypedNilAndHeroBannerUncachedLogo(t *testing.T) {
	e := &env.Env{MainWidth: 1200}
	mgr := New(e, HostCallbacks{})

	if src := mgr.LoadImageSource("/definitely/missing/logo.png"); src != nil {
		t.Fatalf("expected untyped nil ui.ImageSource for uncached bitmap, got %T (%v)", src, src)
	}
	if src := mgr.LoadImageSource("/definitely/missing/logo.svg"); src != nil {
		t.Fatalf("expected untyped nil ui.ImageSource for uncached svg, got %T (%v)", src, src)
	}

	inst := newInstance("inst-hero-logo", "hikarinagi-detail", plugin.SlotGameDetailHero, "hikarinagi-hero", "g-logo")
	inst.Version = 1
	inst.Tree = &plugin.UINode{
		Type: "HeroBanner",
		Key:  "hero-g-logo",
		Props: map[string]any{
			"height": 460.0,
			"banner": "/definitely/missing/banner.webp",
			"logo":   "/definitely/missing/logo-uncached.png",
			"title":  "樱之诗",
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1)
		mgr.RenderInstance(c, th, inst)
	}, 1200, 600)
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
}

func TestRenderHeroBannerFullBleedWithCenteredMaxWidth(t *testing.T) {
	const winW = 1800
	const maxContentW = 1200

	e := &env.Env{MainWidth: winW}
	mgr := New(e, HostCallbacks{})

	inst := newInstance("inst-hero-wide", "hikarinagi-detail", plugin.SlotGameDetailHero, "hikarinagi-hero", "g-wide")
	inst.Version = 1
	inst.Tree = &plugin.UINode{
		Type: "HeroBanner",
		Key:  "hero-wide",
		Props: map[string]any{
			"height":          560.0,
			"contentMaxWidth": float64(maxContentW),
			"title":           "美好的每一天",
			"backSlot":        "back",
			"coverSlot":       "cover",
		},
	}

	natives := NativeSlots{
		"back": func(c *ui.Context, _ *theme.Theme) {
			ui.Box(c.Key("native-back")).Size(60, 24).Label("返回按钮")
		},
		"cover": func(c *ui.Context, _ *theme.Theme) {
			ui.Box(c.Key("native-cover")).Size(196, 260).Label("游戏封面")
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1)
		ui.Column(c).Width(winW).Children(func() {
			mgr.renderHeroBanner(c, th, inst, "hero-wide", inst.Tree, natives).Label("hero-bg-root")
			ui.Column(c.Key("body-wrap")).FillWidth().Padding(8, 32, 0, 32).Children(func() {
				ui.Column(c.Key("body-inner")).Width(maxContentW).Margin(0, ui.Auto).Label("body-inner-root").Children(func() {
					ui.Box(c.Key("body-marker")).Size(80, 24).Label("正文标记")
				})
			})
		})
	}, winW, 900)
	tt.Frame()

	heroRect, ok := tt.Find("hero-bg-root")
	if !ok {
		t.Fatalf("hero-bg-root not found")
	}
	bodyRect, ok := tt.Find("body-inner-root")
	if !ok {
		t.Fatalf("body-inner-root not found")
	}
	if heroRect.W != winW {
		t.Fatalf("expected hero background to span full width %d, got %v", winW, heroRect.W)
	}
	if heroRect.H < 560 {
		t.Fatalf("expected hero height >= 560, got %v", heroRect.H)
	}
	wantLeft := float32((winW - maxContentW) / 2)
	if bodyRect.X != wantLeft || bodyRect.W != maxContentW {
		t.Fatalf("expected body inner rect X=%v W=%d, got %+v", wantLeft, maxContentW, bodyRect)
	}

	titleRect, ok := tt.Find("美好的每一天")
	if !ok {
		t.Fatalf("title not found in rendered tree")
	}
	if titleRect.X != bodyRect.X {
		t.Fatalf("expected hero title left edge (%v) to align with body left edge (%v)", titleRect.X, bodyRect.X)
	}

	coverRect, ok := tt.Find("游戏封面")
	if !ok {
		t.Fatalf("cover not found in rendered tree")
	}
	coverRight := coverRect.X + coverRect.W
	bodyRight := bodyRect.X + bodyRect.W
	if diff := bodyRight - coverRight; diff < -2 || diff > 2 {
		t.Fatalf("expected hero cover right edge (%v) to align with body right edge (%v)", coverRight, bodyRight)
	}

	backRect, ok := tt.Find("返回按钮")
	if !ok {
		t.Fatalf("back button not found in rendered tree")
	}
	// back button sits inside pill with 8px left padding
	if diff := backRect.X - (bodyRect.X + 8); diff < -2 || diff > 2 {
		t.Fatalf("expected back button to align with body left edge (+8px pill pad): backX=%v bodyX=%v", backRect.X, bodyRect.X)
	}
}

