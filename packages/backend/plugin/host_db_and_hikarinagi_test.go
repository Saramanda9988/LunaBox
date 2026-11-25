package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lunabox/backend/appconf"
)

func TestPluginIsolatedSQLite(t *testing.T) {
	tmp := t.TempDir()
	pluginsDir := filepath.Join(tmp, "plugins")

	code := `
import { useState, useEffect, Column, Text } from 'lunabox';

export function apply(ctx) {
  function DBCheck() {
    const [status, setStatus] = useState('init');
    useEffect(() => {
      (async () => {
        try {
          await ctx.db.exec('CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY, v TEXT NOT NULL)');
          await ctx.db.transaction([
            { sql: 'INSERT INTO kv (k, v) VALUES (?, ?)', params: ['a', '1'] },
            { sql: 'INSERT INTO kv (k, v) VALUES (?, ?)', params: ['b', '2'] },
          ]);
          // 触发主键冲突，验证事务原子回滚
          let rolledBack = false;
          try {
            await ctx.db.transaction([
              { sql: 'INSERT INTO kv (k, v) VALUES (?, ?)', params: ['c', '3'] },
              { sql: 'INSERT INTO kv (k, v) VALUES (?, ?)', params: ['a', 'dup'] },
            ]);
          } catch (_) {
            rolledBack = true;
          }
          const rows = await ctx.db.query('SELECT k, v FROM kv ORDER BY k');
          const one = await ctx.db.queryOne('SELECT v FROM kv WHERE k = ?', ['b']);
          const missing = await ctx.db.queryOne('SELECT v FROM kv WHERE k = ?', ['c']);
          if (!rolledBack || rows.length !== 2 || !one || one.v !== '2' || missing !== null) {
            setStatus('bad:' + JSON.stringify({ rolledBack, rows, one, missing }));
            return;
          }
          setStatus('ok:' + rows.map((r) => r.k + '=' + r.v).join(','));
        } catch (err) {
          setStatus('err:' + String(err));
        }
      })();
    }, []);
    return Column({ children: [Text({ k: 'st', text: status })] });
  }
  ctx.ui.slot('settings.section', { id: 'db-check', title: 'DB Check', render: DBCheck });
}
`
	writeTestPlugin(t, pluginsDir, "sqlite-tester", "1.0.0", code)

	cfg := &appconf.AppConfig{
		Language: "zh-CN",
		Plugins:  []appconf.PluginEntry{{ID: "sqlite-tester", Enabled: true}},
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config:      cfg,
		DataDir:     tmp,
		HostVersion: "2.1.0",
	})
	defer mgr.Shutdown()

	mgr.Discover()
	if err := mgr.Enable("sqlite-tester"); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	// 惰性打开：在插件未调用 ctx.db.* 前不应创建 plugin.db
	dbPath := filepath.Join(tmp, "plugins-data", "sqlite-tester", "plugin.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected plugin.db to be lazily created, got stat err=%v", err)
	}

	treeCh := make(chan UITreeUpdate, 8)
	unsub := mgr.UI().SubscribeTreeUpdates(func(u UITreeUpdate) {
		treeCh <- u
	})
	defer unsub()

	mgr.MountSlot("inst-db-1", "sqlite-tester", SlotSettingsSection, "db-check", map[string]any{})

	deadline := time.After(5 * time.Second)
	for {
		select {
		case u := <-treeCh:
			if u.Error != "" {
				t.Fatalf("unexpected UI error: %s", u.Error)
			}
			if u.Tree != nil && len(u.Tree.Children) > 0 {
				if txt, _ := u.Tree.Children[0].Props["text"].(string); txt == "ok:a=1,b=2" {
					if _, err := os.Stat(dbPath); err != nil {
						t.Fatalf("expected plugin.db to exist after query: %v", err)
					}
					return
				} else if txt != "init" {
					t.Fatalf("unexpected DB check result: %q", txt)
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for SQLite plugin check")
		}
	}
}

func TestHikarinagiDetailPluginSlotsAndCachedRender(t *testing.T) {
	exampleDir, err := filepath.Abs(filepath.Join("..", "..", "plugin-sdk", "examples", "hikarinagi-detail"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(exampleDir, "main.js")); err != nil {
		t.Fatalf("hikarinagi-detail example missing: %v", err)
	}

	tmp := t.TempDir()
	// 预先往插件独立 SQLite 写入一条模拟 galgame.get 缓存，测试完全离线的全链路渲染
	pluginDataDir := filepath.Join(tmp, "plugins-data", "hikarinagi-detail")
	if err := os.MkdirAll(pluginDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(pluginDataDir, "plugin.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE galgame_cache (
			hikarinagi_id TEXT PRIMARY KEY,
			payload TEXT NOT NULL,
			fetched_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE game_bindings (
			local_game_id TEXT PRIMARY KEY,
			hikarinagi_id TEXT NOT NULL,
			manual INTEGER NOT NULL DEFAULT 0,
			matched_title TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL
		);
	`)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	nowMs := time.Now().UnixMilli()
	sampleDetail := map[string]any{
		"id":                     1024,
		"origin_title":           "サクラノ詩",
		"trans_title":            "樱之诗",
		"en_title":               "Sakura no Uta",
		"aliases":                []string{"樱诗"},
		"adv_type":               "视觉小说",
		"origin_lang":            "ja",
		"nsfw":                   true,
		"developer":              "枕",
		"platforms":              []string{"Windows"},
		"homepage":               "https://www.makura-soft.com/",
		"release_status":         "RELEASED",
		"release_date":           "2015-10-23",
		"release_date_precision": "DAY",
		"release_date_tbd":       false,
		"logos": []map[string]any{
			{"url": "https://cdn.example.com/logo-zh.png", "language": "zh-Hans", "width": 640, "height": 200},
		},
		"images": []map[string]any{
			{"url": "https://cdn.example.com/ss1.webp", "width": 1920, "height": 1080, "sexual": 0, "violence": 0},
			{"url": "https://cdn.example.com/ss2.webp", "width": 1920, "height": 1080, "sexual": 1, "violence": 0},
		},
		"covers": []map[string]any{
			{"url": "https://cdn.example.com/cover.webp", "width": 800, "height": 1120, "kind": "DIG", "votes": 12},
		},
		"rating": map[string]any{
			"score": 9.3,
			"total": 420,
			"count": map[string]any{"10": 260, "9": 100, "8": 40, "7": 20},
		},
		"tags": []map[string]any{
			{"name": "神作", "likes": 88},
			{"name": "艺术", "likes": 52},
		},
		"external_source": map[string]any{
			"bangumi": "22423",
			"vndb":    "v562",
		},
		"steam_apps": []map[string]any{},
		"prices": []map[string]any{
			{"version": "通常版", "amount": 9800, "currency": "JPY", "tax_included": false},
		},
	}
	payloadBytes, _ := json.Marshal(sampleDetail)
	_, err = db.Exec(
		`INSERT INTO galgame_cache (hikarinagi_id, payload, fetched_at, updated_at) VALUES (?, ?, ?, ?)`,
		"1024", string(payloadBytes), nowMs, nowMs,
	)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	cfg := &appconf.AppConfig{
		Language:       "zh-CN",
		PluginDevPaths: []string{exampleDir},
		Plugins:        []appconf.PluginEntry{{ID: "hikarinagi-detail", Enabled: true}},
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config:      cfg,
		DataDir:     tmp,
		HostVersion: "2.1.0",
	})
	defer mgr.Shutdown()

	mgr.Discover()
	if err := mgr.Enable("hikarinagi-detail"); err != nil {
		t.Fatalf("Enable hikarinagi-detail failed: %v", err)
	}

	heroSlots := mgr.UI().Slots(SlotGameDetailHero)
	if len(heroSlots) != 1 || heroSlots[0].ID != "hikarinagi-hero" {
		t.Fatalf("unexpected hero slots: %+v", heroSlots)
	}
	if len(heroSlots[0].HideTabs) != 1 || heroSlots[0].HideTabs[0] != "progress" {
		t.Fatalf("expected HideTabs=[progress], got %+v", heroSlots[0].HideTabs)
	}

	tabSlots := mgr.UI().Slots(SlotGameDetailTab)
	if len(tabSlots) != 2 {
		t.Fatalf("expected 2 tab slots (overview + screenshots), got %+v", tabSlots)
	}
	foundOverview, foundScreenshots := false, false
	for _, s := range tabSlots {
		if s.ID == "overview" && s.Order < 0 {
			foundOverview = true
		}
		if s.ID == "screenshots" && s.Order == 1 {
			foundScreenshots = true
		}
	}
	if !foundOverview || !foundScreenshots {
		t.Fatalf("unexpected tab slots: %+v", tabSlots)
	}

	asideSlots := mgr.UI().Slots(SlotGameDetailAside)
	if len(asideSlots) != 1 || asideSlots[0].ID != "detail-info" {
		t.Fatalf("unexpected aside slots: %+v", asideSlots)
	}

	updates := make(chan UITreeUpdate, 32)
	unsub := mgr.UI().SubscribeTreeUpdates(func(u UITreeUpdate) {
		updates <- u
	})
	defer unsub()

	props := map[string]any{
		"gameId":   "g-1",
		"blurNsfw": true,
		"game": map[string]any{
			"id":          "g-1",
			"name":        "樱之诗",
			"source_type": "hikarinagi",
			"source_id":   "1024",
		},
	}

	mgr.MountSlot("hero::1", "hikarinagi-detail", SlotGameDetailHero, "hikarinagi-hero", props)
	mgr.MountSlot("overview::1", "hikarinagi-detail", SlotGameDetailTab, "overview", props)
	mgr.MountSlot("tab::1", "hikarinagi-detail", SlotGameDetailTab, "screenshots", props)
	mgr.MountSlot("aside::1", "hikarinagi-detail", SlotGameDetailAside, "detail-info", props)

	gotHeroReady := false
	gotOverviewReady := false
	gotTabReady := false
	gotAsideReady := false
	deadline := time.After(5 * time.Second)

	for !(gotHeroReady && gotOverviewReady && gotTabReady && gotAsideReady) {
		select {
		case u := <-updates:
			if u.Error != "" {
				t.Fatalf("slot %s returned error: %s", u.InstanceID, u.Error)
			}
			if u.Tree == nil {
				continue
			}
			raw, _ := json.Marshal(u.Tree)
			s := string(raw)
			switch u.InstanceID {
			case "hero::1":
				if u.Tree.Type == "Stack" &&
					!containsStr(s, `"t":"HeroBanner"`) &&
					stringsContainsAll(
						s,
						"https://cdn.example.com/logo-zh.png",
						"https://cdn.example.com/ss1.webp",
						`"name":"back"`,
						`"name":"cover"`,
						`"name":"toolbar"`,
						"9.3",
						"上一张头图",
						"下一张头图",
						// 头图遮罩明暗自适应：浅色主题用偏白的 fadeBg 融出，深色主题保留深色 scrim
						`"light":"fadeBg@0.90"`,
						`"dark":"scrim@0.84"`,
					) {
					gotHeroReady = true
				}
			case "overview::1":
				// 没有 Hikarinagi 简介时交给宿主原生简介（自带标题）；别名已移到信息栏
				if stringsContainsAll(s, "标签", "NativeSlot", `"name":"summary"`) &&
					!containsStr(s, "樱诗") &&
					!containsStr(s, "Hikarinagi 社区标签") &&
					!containsStr(s, "神作") {
					gotOverviewReady = true
				}
			case "tab::1":
				if stringsContainsAll(s, "游戏截图", "https://cdn.example.com/ss1.webp", "https://cdn.example.com/ss2.webp") &&
					!containsStr(s, "封面与透明徽标") &&
					!containsStr(s, "透明标题徽标") &&
					!containsStr(s, "收录封面") {
					gotTabReady = true
				}
			case "aside::1":
				if stringsContainsAll(s, "游戏信息", "サクラノ詩", "枕", "Bangumi", "VNDB", "别名", "樱诗") &&
					!containsStr(s, "Hikarinagi #") &&
					!containsStr(s, "#1024") &&
					!containsStr(s, "社区评分") &&
					!containsStr(s, "条目关联与缓存") &&
					!containsStr(s, `"t":"Card"`) {
					gotAsideReady = true
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for hydrated trees: hero=%v overview=%v tab=%v aside=%v", gotHeroReady, gotOverviewReady, gotTabReady, gotAsideReady)
		}
	}
}

func stringsContainsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsStr(s, sub) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestHikarinagiThemePluginLocksBackground(t *testing.T) {
	exampleDir, err := filepath.Abs(filepath.Join("..", "..", "plugin-sdk", "examples", "hikarinagi-theme"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(exampleDir, "main.js")); err != nil {
		t.Fatalf("hikarinagi-theme example missing: %v", err)
	}
	cfg := &appconf.AppConfig{
		Language:       "zh-CN",
		PluginDevPaths: []string{exampleDir},
		Plugins:        []appconf.PluginEntry{{ID: "hikarinagi-theme", Enabled: true}},
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{Config: cfg, DataDir: t.TempDir(), HostVersion: "2.1.0"})
	defer mgr.Shutdown()

	mgr.Discover()
	if err := mgr.Enable("hikarinagi-theme"); err != nil {
		t.Fatalf("Enable hikarinagi-theme failed: %v", err)
	}

	ovs := mgr.UI().ThemeOverrides()
	if len(ovs) != 1 || ovs[0].ID != "hikarinagi-hina" {
		t.Fatalf("unexpected theme overrides: %+v", ovs)
	}
	ov := ovs[0]
	if ov.Glass == nil || ov.Glass.Enabled == nil || *ov.Glass.Enabled {
		t.Fatalf("expected glass.enabled=false by default, got %+v", ov.Glass)
	}
	if ov.Light["primaryBtn"] != "#39c5bb" || ov.Light["primaryBtnText"] != "#0a0a0a" || ov.Dark["primaryBtn"] != "#45d4c9" {
		t.Fatalf("unexpected accent tokens: light=%v dark=%v", ov.Light, ov.Dark)
	}
	if ov.Shape == nil || ov.Shape.Control == nil || *ov.Shape.Control != 8 || ov.Shape.Panel == nil || *ov.Shape.Panel != 8 {
		t.Fatalf("expected shape control/panel = 8, got %+v", ov.Shape)
	}
	if ov.Dark["actionBtnRing"] != "#45d4c9" || ov.Light["menuSelected"] != "#00000000" {
		t.Fatalf("unexpected toolbar/menu tokens: light=%v dark=%v", ov.Light, ov.Dark)
	}
	// 品牌标志：字标替换为插件目录下的 Hikarinagi 字标（解析为绝对路径），默认隐藏图标、显示字标
	wantMark := filepath.Join(exampleDir, "assets", "hikarinagi-wordmark.svg")
	if b := ov.Brand; b == nil || b.Wordmark != wantMark || b.HideIcon == nil || !*b.HideIcon ||
		b.HideWordmark == nil || *b.HideWordmark {
		t.Fatalf("unexpected brand override: %+v (want wordmark %s)", ov.Brand, wantMark)
	}
	if _, err := os.Stat(wantMark); err != nil {
		t.Fatalf("wordmark asset missing: %v", err)
	}
	// 侧边栏材质：默认磨砂，着色是 88% 的纯白 / 纯黑，导航状态改用半透明色
	if m := ov.Material; m == nil || m.Sidebar != "acrylic" || m.Tint != "#ffffffe0" || m.TintDark != "#000000e0" {
		t.Fatalf("unexpected material override: %+v", ov.Material)
	}
	if ov.Light["navActiveBg"] != "#39c5bb29" || ov.Dark["sidebarBorder"] != "#ffffff1a" {
		t.Fatalf("unexpected sidebar tokens on material: light=%v dark=%v", ov.Light, ov.Dark)
	}

	// 关闭材质后重新注册：不再声明 material，导航状态回到预先合成的不透明色
	if err := mgr.SetConfigValues("hikarinagi-theme", map[string]any{"sidebarMaterial": "off"}); err != nil {
		t.Fatalf("SetConfigValues failed: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		ovs = mgr.UI().ThemeOverrides()
		if len(ovs) == 1 && ovs[0].Material == nil && ovs[0].Light["navActiveBg"] == "#e7f8f7" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("sidebarMaterial=off not applied: %+v", ovs)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 关闭字标后重新注册，hideWordmark 为 true
	if err := mgr.SetConfigValues("hikarinagi-theme", map[string]any{"showWordmark": false}); err != nil {
		t.Fatalf("SetConfigValues failed: %v", err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		ovs = mgr.UI().ThemeOverrides()
		if len(ovs) == 1 && ovs[0].Brand != nil && ovs[0].Brand.HideWordmark != nil && *ovs[0].Brand.HideWordmark {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("showWordmark=false not applied: %+v", ovs)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 关闭锁定后应重新注册且不再声明 glass，交还用户的背景设置
	if err := mgr.SetConfigValues("hikarinagi-theme", map[string]any{"lockBackground": false}); err != nil {
		t.Fatalf("SetConfigValues failed: %v", err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		ovs = mgr.UI().ThemeOverrides()
		if len(ovs) == 1 && ovs[0].Glass == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lockBackground=false not applied: %+v", ovs)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
