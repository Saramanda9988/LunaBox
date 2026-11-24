package pluginui

import (
	"testing"

	"lunabox/backend/plugin"
	"lunabox/desktop/api"
	"lunabox/desktop/ui/env"
)

type slotCallsStub struct {
	api.PluginService
	mounts, updates, unmounts int
}

func (s *slotCallsStub) MountSlot(string, string, string, string, any)  { s.mounts++ }
func (s *slotCallsStub) UpdateSlot(string, string, string, string, any) { s.updates++ }
func (s *slotCallsStub) UnmountSlot(string, string)                     { s.unmounts++ }

// 详情页切换游玩状态等会改变插槽 props：实例应原地更新并保留上一棵树，
// 否则重建期间 RenderSlot 报告未就绪，页面会闪回宿主的回退布局。
func TestEnsureMountedKeepsTreeWhenPropsChange(t *testing.T) {
	stub := &slotCallsStub{}
	m := New(&env.Env{Svc: &api.Services{Plugin: stub}}, HostCallbacks{})
	props := map[string]any{"gameId": "g1", "status": "playing"}
	inst := m.EnsureMounted(plugin.SlotGameDetailHero, "demo", "hero", "g1", props)
	m.applyTreeUpdate(plugin.UITreeUpdate{InstanceID: inst.ID, PluginID: "demo", Version: 1, Tree: &plugin.UINode{Type: "Column"}})

	if again := m.EnsureMounted(plugin.SlotGameDetailHero, "demo", "hero", "g1", props); again != inst || stub.updates != 0 {
		t.Fatalf("same props should reuse the instance without updating (updates=%d)", stub.updates)
	}
	props2 := map[string]any{"gameId": "g1", "status": "completed"}
	got := m.EnsureMounted(plugin.SlotGameDetailHero, "demo", "hero", "g1", props2)
	if got != inst || got.Tree == nil {
		t.Fatal("changed props should keep the instance and its last tree")
	}
	if stub.mounts != 1 || stub.updates != 1 || stub.unmounts != 0 {
		t.Fatalf("expected one mount and one in-place update, got mounts=%d updates=%d unmounts=%d", stub.mounts, stub.updates, stub.unmounts)
	}
}
