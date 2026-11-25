package plugin

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/common/hooks"
	"lunabox/backend/models"
)

func startTestManager(t *testing.T, plugins map[string]string, order []string) *Manager {
	t.Helper()
	tmp := t.TempDir()
	pluginsDir := filepath.Join(tmp, "plugins")
	entries := make([]appconf.PluginEntry, 0, len(order))
	for _, id := range order {
		writeTestPlugin(t, pluginsDir, id, "1.0.0", plugins[id])
		entries = append(entries, appconf.PluginEntry{ID: id, Enabled: true})
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config:      &appconf.AppConfig{Language: "zh-CN", Plugins: entries},
		DataDir:     tmp,
		HostVersion: "2.1.0",
	})
	t.Cleanup(mgr.Shutdown)
	mgr.Discover()
	return mgr
}

func enableTestPlugin(t *testing.T, mgr *Manager, id string) {
	t.Helper()
	if err := mgr.Enable(id); err != nil {
		t.Fatalf("enable %s: %v", id, err)
	}
}

// waitStorage 等插件把结果写进 ctx.storage。
func waitStorage(t *testing.T, mgr *Manager, id, key string) json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if p := mgr.activePlugin(id); p != nil {
			if raw, ok := p.storage.Get(key); ok {
				return raw
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("plugin %s never stored %q", id, key)
	return nil
}

func TestLaunchPlanInterceptorsMergeCancelAndTimeout(t *testing.T) {
	mgr := startTestManager(t, map[string]string{
		"args-plugin": `export function apply(ctx) {
  ctx.on('launch/plan', ({ plan }) => ({ plan: { args: [...plan.args, '-login', 'alice'] } }));
}`,
		"env-plugin": `export function apply(ctx) {
  ctx.on('launch/plan', async () => {
    await new Promise((r) => setTimeout(r, 50));
    return { plan: { env: { FOO: 'bar' } } };
  }, { timeout: 5000 });
}`,
		"slow-plugin": `export function apply(ctx) {
  ctx.on('launch/plan', async () => {
    await new Promise((r) => setTimeout(r, 1000));
    return { plan: { file: 'hijacked.exe' } };
  }, { timeout: 100 });
}`,
	}, []string{"args-plugin", "env-plugin", "slow-plugin"})
	for _, id := range []string{"args-plugin", "env-plugin", "slow-plugin"} {
		enableTestPlugin(t, mgr, id)
	}

	raw, err := mgr.HookBus().Intercept(context.Background(), hooks.EventLaunchPlan, hooks.LaunchPlanPayload{
		Game: models.Game{ID: "g1", Name: "Game"},
		Plan: hooks.LaunchPlan{File: "steam.exe", Args: []string{"-silent", "steam://rungameid/1"}, Dir: "C:/Steam"},
	})
	if err != nil {
		t.Fatalf("Intercept: %v", err)
	}
	var res hooks.LaunchPlanPayload
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if res.Cancel {
		t.Fatalf("unexpected cancel: %s", raw)
	}
	if got := strings.Join(res.Plan.Args, " "); got != "-silent steam://rungameid/1 -login alice" {
		t.Fatalf("args not merged: %q", got)
	}
	if res.Plan.Env["FOO"] != "bar" {
		t.Fatalf("env not merged: %v", res.Plan.Env)
	}
	if res.Plan.File != "steam.exe" || res.Plan.Dir != "C:/Steam" {
		t.Fatalf("timed-out interceptor or partial patch changed untouched fields: %+v", res.Plan)
	}
	if res.Game.ID != "g1" {
		t.Fatalf("game lost in merge: %+v", res.Game)
	}
}

func TestLaunchPrepareCancelStopsChain(t *testing.T) {
	mgr := startTestManager(t, map[string]string{
		"cancel-plugin": `export function apply(ctx) {
  ctx.on('launch/prepare', ({ options }) => options.use_steam ? { cancel: true, reason: 'pick an account first' } : undefined);
}`,
		"after-plugin": `export function apply(ctx) {
  ctx.on('launch/prepare', () => { ctx.storage.set('called', true); return { options: { run_as_admin: true } }; });
}`,
	}, []string{"cancel-plugin", "after-plugin"})
	enableTestPlugin(t, mgr, "cancel-plugin")
	enableTestPlugin(t, mgr, "after-plugin")

	yes := true
	raw, err := mgr.HookBus().Intercept(context.Background(), hooks.EventLaunchPrepare, hooks.LaunchPreparePayload{
		Game:    models.Game{ID: "g1"},
		Options: hooks.LaunchOptions{UseSteam: &yes},
	})
	if err != nil {
		t.Fatalf("Intercept: %v", err)
	}
	var res hooks.LaunchPreparePayload
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	if !res.Cancel || res.Reason != "pick an account first" {
		t.Fatalf("expected cancel, got %s", raw)
	}
	if res.Options.RunAsAdmin != nil {
		t.Fatalf("interceptor after cancel should not run: %s", raw)
	}
	if p := mgr.activePlugin("after-plugin"); p != nil {
		if _, called := p.storage.Get("called"); called {
			t.Fatalf("interceptor after cancel was called")
		}
	}
}

func TestProcessExecSpawnAndWebCompat(t *testing.T) {
	mgr := startTestManager(t, map[string]string{
		"proc": `export async function apply(ctx) {
  const r = await ctx.process.exec('echo hello-lunabox', { shell: true });
  ctx.storage.set('exec', r);
  const missing = await ctx.process.exec('lunabox-no-such-command-xyz').then(() => 'ok', (e) => 'err');
  ctx.storage.set('missing', missing);
  const failing = await ctx.process.exec('exit 3', { shell: true });
  ctx.storage.set('failing', failing.code);
  const h = ctx.process.spawn('echo spawned-out', { shell: true });
  let out = '';
  h.onStdout((t) => { out += t; });
  h.onExit(({ code }) => ctx.storage.set('spawn', { out, code }));
  ctx.storage.set('platform', ctx.process.platform);

  ctx.storage.set('uuid', crypto.randomUUID());
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  ctx.storage.set('randomNonZero', bytes.some((b) => b !== 0));
  const fd = new FormData();
  fd.append('a', '1');
  fd.append('f', new Blob(['xyz'], { type: 'text/plain' }), 'x.txt');
  ctx.storage.set('formData', { a: fd.get('a'), name: fd.get('f').name, size: fd.get('f').size, blobText: await new Blob(['hi']).text() });
  ctx.storage.set('finalization', typeof new FinalizationRegistry(() => {}).register);
}`,
	}, []string{"proc"})
	enableTestPlugin(t, mgr, "proc")

	var execRes execResult
	if err := json.Unmarshal(waitStorage(t, mgr, "proc", "exec"), &execRes); err != nil {
		t.Fatal(err)
	}
	if execRes.Code != 0 || strings.TrimSpace(execRes.Stdout) != "hello-lunabox" {
		t.Fatalf("exec result: %+v", execRes)
	}
	if got := string(waitStorage(t, mgr, "proc", "missing")); got != `"err"` {
		t.Fatalf("missing command should reject, got %s", got)
	}
	if got := string(waitStorage(t, mgr, "proc", "failing")); got != "3" {
		t.Fatalf("non-zero exit code should resolve with code, got %s", got)
	}
	var spawnRes struct {
		Out  string `json:"out"`
		Code int    `json:"code"`
	}
	if err := json.Unmarshal(waitStorage(t, mgr, "proc", "spawn"), &spawnRes); err != nil {
		t.Fatal(err)
	}
	if spawnRes.Code != 0 || strings.TrimSpace(spawnRes.Out) != "spawned-out" {
		t.Fatalf("spawn result: %+v", spawnRes)
	}

	var uuid string
	_ = json.Unmarshal(waitStorage(t, mgr, "proc", "uuid"), &uuid)
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(uuid) {
		t.Fatalf("crypto.randomUUID returned %q", uuid)
	}
	if got := string(waitStorage(t, mgr, "proc", "randomNonZero")); got != "true" {
		t.Fatalf("getRandomValues did not fill the array")
	}
	var fd struct {
		A        string `json:"a"`
		Name     string `json:"name"`
		Size     int    `json:"size"`
		BlobText string `json:"blobText"`
	}
	if err := json.Unmarshal(waitStorage(t, mgr, "proc", "formData"), &fd); err != nil || fd.A != "1" || fd.Name != "x.txt" || fd.Size != 3 || fd.BlobText != "hi" {
		t.Fatalf("FormData/Blob: %+v %v", fd, err)
	}
	if got := string(waitStorage(t, mgr, "proc", "finalization")); got != `"function"` {
		t.Fatalf("FinalizationRegistry: %s", got)
	}
}

func TestDialogConfirmSelectAndCloseOnUnload(t *testing.T) {
	mgr := startTestManager(t, map[string]string{
		"dlg": `export function apply(ctx) {
  ctx.ui.confirm({ title: 'Delete?', message: 'really', danger: true }).then((ok) => ctx.storage.set('confirm', ok));
  ctx.ui.select({ title: 'Account', options: [{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }] })
    .then((v) => ctx.storage.set('select', v === null ? 'cancelled' : v));
  ctx.ui.confirm({ title: 'never answered' });
}`,
	}, []string{"dlg"})

	var mu sync.Mutex
	var actions []UIAction
	unsub := mgr.UI().SubscribeActions(func(a UIAction) {
		mu.Lock()
		actions = append(actions, a)
		mu.Unlock()
	})
	defer unsub()
	enableTestPlugin(t, mgr, "dlg")

	dialogs := func(kind string) []UIAction {
		mu.Lock()
		defer mu.Unlock()
		var out []UIAction
		for _, a := range actions {
			if a.Type == kind {
				out = append(out, a)
			}
		}
		return out
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(dialogs("dialog")) < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	opened := dialogs("dialog")
	if len(opened) != 3 {
		t.Fatalf("expected 3 dialogs, got %d", len(opened))
	}
	if opened[0].Dialog.Kind != "confirm" || !opened[0].Dialog.Danger || opened[1].Dialog.Kind != "select" || len(opened[1].Dialog.Options) != 2 {
		t.Fatalf("unexpected dialog specs: %+v %+v", opened[0].Dialog, opened[1].Dialog)
	}

	mgr.ResolveDialog("dlg", opened[0].DialogID, json.RawMessage("true"))
	mgr.ResolveDialog("dlg", opened[1].DialogID, nil)
	if got := string(waitStorage(t, mgr, "dlg", "confirm")); got != "true" {
		t.Fatalf("confirm result %s", got)
	}
	if got := string(waitStorage(t, mgr, "dlg", "select")); got != `"cancelled"` {
		t.Fatalf("select result %s", got)
	}
	// 重复作答是空操作
	mgr.ResolveDialog("dlg", opened[0].DialogID, json.RawMessage("false"))

	if err := mgr.Disable("dlg"); err != nil {
		t.Fatal(err)
	}
	closed := dialogs("closeDialog")
	if len(closed) != 1 || closed[0].DialogID != opened[2].DialogID {
		t.Fatalf("expected only the unanswered dialog to be closed on unload, got %+v", closed)
	}
}

func TestApplyGamePatchFields(t *testing.T) {
	g := models.Game{ID: "g1", Name: "Old", Path: "C:/a.exe", SteamLaunchID: "123"}
	err := applyGamePatch(&g, json.RawMessage(`{"name":"New","path":"D:/b.exe","launch_mode":"steam","use_magpie":true,"id":"hijack","steam_launch_id":"999"}`))
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != "g1" || g.SteamLaunchID != "123" {
		t.Fatalf("read-only fields changed: %+v", g)
	}
	if g.Name != "New" || g.Path != "D:/b.exe" || g.LaunchMode != "steam" || !g.UseMagpie {
		t.Fatalf("writable fields not applied: %+v", g)
	}
	if err := applyGamePatch(&g, json.RawMessage(`{"nmae":"typo"}`)); err == nil || !strings.Contains(err.Error(), "nmae") {
		t.Fatalf("unknown field should be rejected, got %v", err)
	}
	if err := applyGamePatch(&g, json.RawMessage(`{"rating":"high"}`)); err == nil {
		t.Fatalf("wrong field type should be rejected")
	}
}

func TestMergeJSONObject(t *testing.T) {
	dst := map[string]any{"plan": map[string]any{"file": "a", "args": []any{"x"}}, "keep": 1.0}
	mergeJSONObject(dst, map[string]any{"plan": map[string]any{"args": []any{"y"}}, "cancel": true})
	plan := dst["plan"].(map[string]any)
	if plan["file"] != "a" || len(plan["args"].([]any)) != 1 || plan["args"].([]any)[0] != "y" || dst["keep"] != 1.0 || dst["cancel"] != true {
		t.Fatalf("merge result: %#v", dst)
	}
}
