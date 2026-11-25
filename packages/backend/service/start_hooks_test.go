package service

import (
	"context"
	"encoding/json"
	"testing"

	"lunabox/backend/common/hooks"
	"lunabox/backend/models"
	launcherpkg "lunabox/backend/service/launcher"
)

// fakeInterceptBus 只实现 Intercept：把 respond 的结果深合并进载荷，模拟插件拦截器。
type fakeInterceptBus struct {
	hooks.Bus
	respond func(event string, payload map[string]any) map[string]any
}

func (b fakeInterceptBus) Intercept(_ context.Context, event string, payload any) (json.RawMessage, error) {
	raw, _ := json.Marshal(payload)
	var cur map[string]any
	_ = json.Unmarshal(raw, &cur)
	for k, v := range b.respond(event, cur) {
		if pv, ok := v.(map[string]any); ok {
			if dv, ok := cur[k].(map[string]any); ok {
				for kk, vv := range pv {
					dv[kk] = vv
				}
				continue
			}
		}
		cur[k] = v
	}
	return json.Marshal(cur)
}

func TestInterceptLaunchPlanAppliesPatch(t *testing.T) {
	s := &StartService{ctx: context.Background()}
	plan := launcherpkg.LaunchPlan{
		File:         "C:/Steam/steam.exe",
		Args:         []string{"-silent", "steam://rungameid/1"},
		Dir:          "C:/Steam",
		DisplayName:  "steam.exe",
		DetectionDir: "D:/Games/X",
	}
	bus := fakeInterceptBus{respond: func(event string, payload map[string]any) map[string]any {
		if event != hooks.EventLaunchPlan {
			t.Fatalf("unexpected event %s", event)
		}
		return map[string]any{"plan": map[string]any{
			"file": "C:/Tools/switcher.exe",
			"args": []any{"--account", "alice"},
			"env":  map[string]any{"B": "2", "A": "1"},
		}}
	}}
	cancelled, _ := s.interceptLaunchPlan(bus, models.Game{ID: "g"}, &plan)
	if cancelled {
		t.Fatal("unexpected cancel")
	}
	if plan.File != "C:/Tools/switcher.exe" || plan.DisplayName != "" {
		t.Fatalf("file/display name: %+v", plan)
	}
	if len(plan.Args) != 2 || plan.Args[1] != "alice" || plan.Dir != "C:/Steam" {
		t.Fatalf("args/dir: %+v", plan)
	}
	if len(plan.Env) != 2 || plan.Env[0] != "A=1" || plan.Env[1] != "B=2" {
		t.Fatalf("env: %v", plan.Env)
	}
	if plan.DetectionDir != "D:/Games/X" {
		t.Fatalf("detection fields must stay untouched: %+v", plan)
	}
}

func TestInterceptLaunchCancelReasons(t *testing.T) {
	s := &StartService{ctx: context.Background()}
	plan := launcherpkg.LaunchPlan{File: "a.exe"}
	bus := fakeInterceptBus{respond: func(string, map[string]any) map[string]any {
		return map[string]any{"cancel": true}
	}}
	if cancelled, reason := s.interceptLaunchPlan(bus, models.Game{}, &plan); !cancelled || reason != defaultPluginCancelReason {
		t.Fatalf("cancel without reason: %v %q", cancelled, reason)
	}

	yes := true
	bus = fakeInterceptBus{respond: func(_ string, p map[string]any) map[string]any {
		return map[string]any{"options": map[string]any{"use_steam": false, "run_as_admin": nil}}
	}}
	opts, cancelled, _ := s.interceptLaunchPrepare(bus, models.Game{}, launcherpkg.LaunchOptions{UseSteam: &yes, RunAsAdmin: &yes})
	if cancelled || opts.UseSteam == nil || *opts.UseSteam || opts.RunAsAdmin != nil {
		t.Fatalf("prepare options not applied: %+v", opts)
	}
}
