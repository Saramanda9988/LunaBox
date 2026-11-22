package plugin

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/hooks"
	"lunabox/backend/utils/metadata"
)

func TestScopeLIFOAndIdempotent(t *testing.T) {
	root := NewScope(context.Background())
	var order []string

	d1 := root.Effect(func() { order = append(order, "e1") })
	root.Effect(func() { order = append(order, "e2") })
	child := root.Fork()
	child.Effect(func() { order = append(order, "c1") })
	child.Effect(func() { order = append(order, "c2") })
	root.Effect(func() { order = append(order, "e3") })

	// 提前单独撤销 e1（幂等）
	d1()
	d1()
	if len(order) != 1 || order[0] != "e1" {
		t.Fatalf("unexpected order after early dispose: %v", order)
	}

	root.Dispose()
	root.Dispose() // 幂等

	want := []string{"e1", "c2", "c1", "e3", "e2"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("LIFO order mismatch: got %v, want %v", order, want)
	}
	if root.Context().Err() == nil || child.Context().Err() == nil {
		t.Fatalf("contexts should be canceled after Dispose")
	}
}

func TestManifestValidationAndSemverRange(t *testing.T) {
	m := Manifest{
		ID:         "vndb-cn-mirror",
		Version:    "0.3.1",
		APIVersion: 1,
		Engines:    EnginesDecl{LunaBox: ">=2.1.0 <3"},
		Main:       "main.js",
		Name:       map[string]string{"zh-CN": "VNDB 镜像", "en-US": "VNDB Mirror"},
	}
	if err := m.Validate("2.1.0"); err != nil {
		t.Fatalf("expected compatible, got %v", err)
	}
	if err := m.Validate("2.0.9"); err == nil {
		t.Fatalf("expected incompatible for 2.0.9")
	}
	if err := m.Validate("3.0.0"); err == nil {
		t.Fatalf("expected incompatible for 3.0.0")
	}
	if got := m.DisplayName("zh-CN"); got != "VNDB 镜像" {
		t.Fatalf("unexpected zh-CN name: %q", got)
	}
	if got := m.DisplayName("ja-JP"); got != "VNDB Mirror" {
		t.Fatalf("expected en-US fallback, got %q", got)
	}
}

func TestUITreeDecodeAndLimits(t *testing.T) {
	valid := `{"t":"Column","k":"root","p":{"gap":"md"},"c":[{"t":"Text","p":{"text":"hello"}}]}`
	tree, err := DecodeAndValidateUITree([]byte(valid))
	if err != nil || tree == nil || tree.Type != "Column" || len(tree.Children) != 1 {
		t.Fatalf("valid tree failed: tree=%+v err=%v", tree, err)
	}

	// 超大字符串应被拒绝
	big := strings.Repeat("a", MaxUITreeStringSize+1)
	raw, _ := json.Marshal(map[string]any{"t": "Text", "p": map[string]any{"text": big}})
	if _, err := DecodeAndValidateUITree(raw); err == nil {
		t.Fatalf("expected error for >64KiB string")
	}
}

func writeTestPlugin(t *testing.T, rootDir, id, version, code string) string {
	t.Helper()
	dir := filepath.Join(rootDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"id":         id,
		"version":    version,
		"apiVersion": 1,
		"engines":    map[string]string{"lunabox": ">=2.0.0 <3"},
		"main":       "main.js",
		"name":       map[string]string{"zh-CN": "测试插件 " + id, "en-US": "Test " + id},
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPluginRuntimeMetadataHooksThemeAndUI(t *testing.T) {
	tmp := t.TempDir()
	pluginsDir := filepath.Join(tmp, "plugins")

	code := `
import { useState, Column, Text, Button } from 'lunabox';

export function apply(ctx) {
  // 1. Storage
  ctx.storage.set('greeting', 'hello');
  if (ctx.storage.get('greeting') !== 'hello') {
    throw new Error('storage mismatch');
  }

  // 2. Metadata source
  ctx.metadata.registerSource({
    id: 'mock-src',
    name: { 'zh-CN': '模拟源', 'en-US': 'Mock Src' },
    link: (id) => 'https://example.com/' + id,
    normalizeId: (input) => /^m\d+$/.test(input) ? input : null,
    rateLimit: { perSecond: 10 },
    async fetch(id) {
      return { game: { name: 'Game ' + id, source_id: id, company: 'MockStudio' }, tags: [{ name: 'ADV', weight: 1 }] };
    },
    async search(name) {
      return [{ game: { name: name + ' Result', source_id: 'm1' } }];
    },
  });

  // 3. Theme + Decoration
  ctx.theme.extend({
    light: { appBg: '#fafafa' },
    dark: { appBg: '#101014' },
  });
  ctx.theme.registerDecoration({
    id: 'mascot',
    layer: 'foreground',
    anchor: 'bottom-right',
    offset: { x: 16, y: 12 },
    size: { width: 200, height: 300 },
    opacity: 0.25,
    pointerEvents: 'none',
    image: 'assets/mascot.png',
  });

  // 4. Hooks: waterfall & bail
  ctx.on('metadata/before-search', (payload) => ({ name: payload.name + ' [hooked]' }));
  ctx.on('game/before-launch', (game) => {
    if (game.id === 'blocked') return { cancel: true, reason: 'blocked by plugin' };
  });

  // 5. UI Slot with useState
  function CounterPanel() {
    const [count, setCount] = useState(0);
    return Column({ gap: 'md', children: [
      Text({ k: 'cnt', text: 'count=' + count }),
      Button({ k: 'inc', label: 'Add', onPress: () => setCount((c) => c + 1) }),
    ]});
  }
  ctx.ui.slot('settings.section', { id: 'counter', title: 'Counter', render: CounterPanel });
}
`
	writeTestPlugin(t, pluginsDir, "demo-plugin", "1.0.0", code)

	cfg := &appconf.AppConfig{
		Language: "zh-CN",
		Plugins:  []appconf.PluginEntry{{ID: "demo-plugin", Enabled: true}},
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config:      cfg,
		DataDir:     tmp,
		HostVersion: "2.1.0",
	})
	defer mgr.Shutdown()

	mgr.Discover()
	if err := mgr.Enable("demo-plugin"); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	// 验证元数据源注册、校验、拉取与搜索
	srcType := enums.SourceType("plugin:demo-plugin/mock-src")
	ext, ok := metadata.LookupExternal(srcType)
	if !ok {
		t.Fatalf("expected external metadata source %s to be registered", srcType)
	}
	if nid, valid := ext.NormalizeID("m42"); !valid || nid != "m42" {
		t.Fatalf("NormalizeID failed: %q %v", nid, valid)
	}
	if _, valid := ext.NormalizeID("bad"); valid {
		t.Fatalf("NormalizeID should reject invalid ID")
	}
	if link := ext.Link("m42"); link != "https://example.com/m42" {
		t.Fatalf("unexpected Link: %q", link)
	}
	res, err := ext.FetchContext(context.Background(), "m42", "zh-CN")
	if err != nil || res.Game.Name != "Game m42" || len(res.Tags) != 1 {
		t.Fatalf("FetchContext failed: res=%+v err=%v", res, err)
	}
	cands, err := ext.SearchContext(context.Background(), "Sakura", "zh-CN", 5)
	if err != nil || len(cands) != 1 || cands[0].Game.Name != "Sakura Result" {
		t.Fatalf("SearchContext failed: cands=%+v err=%v", cands, err)
	}

	// 验证主题与装饰注册
	if len(mgr.UI().ThemeOverrides()) != 1 {
		t.Fatalf("expected 1 theme override")
	}
	if len(mgr.UI().Decorations()) != 1 || mgr.UI().Decorations()[0].PointerEvents != "none" {
		t.Fatalf("expected 1 pointerEvents=none decoration")
	}

	// 验证钩子 waterfall 与 bail
	wfRaw, err := mgr.HookBus().Waterfall(context.Background(), hooks.EventMetadataBeforeSearch, hooks.MetadataBeforeSearchPayload{
		Name:   "Clannad",
		Source: enums.VNDB,
	})
	if err != nil || !strings.Contains(string(wfRaw), "Clannad [hooked]") {
		t.Fatalf("waterfall failed: raw=%s err=%v", string(wfRaw), err)
	}
	bailRaw, matched, err := mgr.HookBus().Bail(context.Background(), hooks.EventGameBeforeLaunch, map[string]any{"id": "blocked"})
	if err != nil || !matched || !strings.Contains(string(bailRaw), "blocked by plugin") {
		t.Fatalf("bail failed: raw=%s matched=%v err=%v", string(bailRaw), matched, err)
	}

	// 验证描述树挂载、按钮点击事件与 useState 更新
	var mu sync.Mutex
	var trees []UITreeUpdate
	treeCh := make(chan UITreeUpdate, 4)
	unsub := mgr.UI().SubscribeTreeUpdates(func(u UITreeUpdate) {
		mu.Lock()
		trees = append(trees, u)
		mu.Unlock()
		treeCh <- u
	})
	defer unsub()

	instID := "inst-counter-1"
	mgr.MountSlot(instID, "demo-plugin", SlotSettingsSection, "counter", map[string]any{})

	var first UITreeUpdate
	select {
	case first = <-treeCh:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for initial UI tree")
	}
	if first.Error != "" || first.Tree == nil || len(first.Tree.Children) != 2 {
		t.Fatalf("unexpected initial tree: %+v", first)
	}
	if first.Tree.Children[0].Props["text"] != "count=0" {
		t.Fatalf("expected count=0, got %+v", first.Tree.Children[0].Props)
	}
	pressID := first.Tree.Children[1].Events["press"]
	if pressID == "" {
		t.Fatalf("missing press handler id")
	}

	mgr.DispatchEvent(UIEvent{
		InstanceID: instID,
		PluginID:   "demo-plugin",
		HandlerID:  pressID,
		Version:    first.Version,
	})

	select {
	case second := <-treeCh:
		if second.Tree == nil || second.Tree.Children[0].Props["text"] != "count=1" {
			t.Fatalf("expected count=1 after click, got %+v", second)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for updated UI tree")
	}

	// 验证卸载彻底撤销（无泄漏）
	if err := mgr.Disable("demo-plugin"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
	if _, ok := metadata.LookupExternal(srcType); ok {
		t.Fatalf("external source should be removed after disable")
	}
	if len(mgr.UI().ThemeOverrides()) != 0 || len(mgr.UI().Decorations()) != 0 || len(mgr.UI().Slots(SlotSettingsSection)) != 0 {
		t.Fatalf("UI registrations should all be revoked after disable")
	}
	if mgr.HookBus().HasListeners(hooks.EventMetadataBeforeSearch) {
		t.Fatalf("hook listeners should be revoked after disable")
	}
}

func TestInstallUpgradeAndUninstallZip(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "zip-demo.zip")

	makeZip := func(ver string) {
		f, err := os.Create(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		w1, _ := zw.Create("zip-demo/plugin.json")
		_, _ = w1.Write([]byte(`{"id":"zip-demo","version":"` + ver + `","apiVersion":1,"main":"main.js","name":{"en-US":"Zip Demo"}}`))
		w2, _ := zw.Create("zip-demo/main.js")
		_, _ = w2.Write([]byte(`export function apply(ctx) { ctx.storage.set('v', '` + ver + `'); }`))
		_ = zw.Close()
		_ = f.Close()
	}

	makeZip("1.0.0")
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config:      &appconf.AppConfig{Language: "zh-CN"},
		DataDir:     tmp,
		HostVersion: "2.1.0",
	})
	defer mgr.Shutdown()

	preview, err := mgr.InspectZip(zipPath)
	if err != nil || preview.Manifest.ID != "zip-demo" || preview.IsUpgrade {
		t.Fatalf("InspectZip failed: %+v err=%v", preview, err)
	}
	info, err := mgr.InstallFromZip(zipPath)
	if err != nil || info.Status != StatusActive {
		t.Fatalf("InstallFromZip failed: %+v err=%v", info, err)
	}

	// 升级到 1.1.0
	makeZip("1.1.0")
	preview2, err := mgr.InspectZip(zipPath)
	if err != nil || !preview2.IsUpgrade || preview2.ExistingVersion != "1.0.0" {
		t.Fatalf("InspectZip upgrade failed: %+v err=%v", preview2, err)
	}
	info2, err := mgr.InstallFromZip(zipPath)
	if err != nil || info2.Version != "1.1.0" {
		t.Fatalf("Upgrade failed: %+v err=%v", info2, err)
	}

	// 卸载并删除数据
	if err := mgr.Uninstall("zip-demo", true); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "plugins", "zip-demo")); !os.IsNotExist(err) {
		t.Fatalf("plugin dir should be deleted")
	}
	if _, err := os.Stat(filepath.Join(tmp, "plugins-data", "zip-demo")); !os.IsNotExist(err) {
		t.Fatalf("plugin data dir should be deleted")
	}
}

func TestLoadAllSDKExamplePlugins(t *testing.T) {
	examplesDir := filepath.Join("..", "..", "plugin-sdk", "examples")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Skipf("examples dir not found: %v", err)
	}
	var devPaths []string
	for _, e := range entries {
		if e.IsDir() {
			devPaths = append(devPaths, filepath.Join(examplesDir, e.Name()))
		}
	}
	if len(devPaths) < 6 {
		t.Fatalf("expected 6 example plugins, got %d", len(devPaths))
	}

	tmp := t.TempDir()
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config: &appconf.AppConfig{
			Language:       "zh-CN",
			PluginDevPaths: devPaths,
		},
		DataDir:     tmp,
		HostVersion: "2.1.0",
	})
	defer mgr.Shutdown()

	mgr.Discover()
	mgr.loadEnabledPlugins()

	for _, info := range mgr.List() {
		if info.Status != StatusActive {
			t.Errorf("example plugin %s failed to activate: status=%s err=%s", info.ID, info.Status, info.LastError)
		}
	}
}
