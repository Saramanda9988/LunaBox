package plugin

import (
	"context"
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
