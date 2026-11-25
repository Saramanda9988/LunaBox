package plugin

// 端到端测试 packages/plugin-sdk/examples/ai-agent：在真实插件运行时里跑打包好的 pi agent，
// 用 httptest 模拟 OpenAI Chat Completions 的流式接口，验证流式文本、工具调用（web_fetch 读取本地网页）、
// 工具结果回传模型、确认对话框被拒绝时的工具错误，以及侧边栏页面的描述树渲染。
//
// 改了示例源码后先重新打包：cd packages/plugin-sdk/examples/ai-agent && npm i && npm run build

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MoeForge/moejs"

	"lunabox/backend/appconf"
)

type agentRequest struct {
	Path string
	Body string
}

type agentRecorder struct {
	mu   sync.Mutex
	reqs []agentRequest
}

func (r *agentRecorder) add(path string, body []byte) {
	r.mu.Lock()
	r.reqs = append(r.reqs, agentRequest{Path: path, Body: string(body)})
	r.mu.Unlock()
}

func (r *agentRecorder) chat() []agentRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []agentRequest
	for _, q := range r.reqs {
		if strings.HasSuffix(q.Path, "/chat/completions") {
			out = append(out, q)
		}
	}
	return out
}

func writeAgentSSE(w http.ResponseWriter, frames []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	for _, f := range frames {
		_, _ = io.WriteString(w, f)
		if fl != nil {
			fl.Flush()
		}
		time.Sleep(3 * time.Millisecond)
	}
}

func openAIChunk(delta, finish string) string {
	fin := "null"
	if finish != "" {
		fin = `"` + finish + `"`
	}
	return fmt.Sprintf(`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"test-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`+"\n\n", delta, fin)
}

// streamToolCall 把一次工具调用拆成多个参数片段流式返回。
func streamToolCall(w http.ResponseWriter, name string, args map[string]any) {
	raw, _ := json.Marshal(args)
	half := len(raw) / 2
	first, _ := json.Marshal(string(raw[:half]))
	second, _ := json.Marshal(string(raw[half:]))
	writeAgentSSE(w, []string{
		openAIChunk(fmt.Sprintf(`{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":%q,"arguments":""}}]}`, name), ""),
		openAIChunk(fmt.Sprintf(`{"tool_calls":[{"index":0,"function":{"arguments":%s}}]}`, first), ""),
		openAIChunk(fmt.Sprintf(`{"tool_calls":[{"index":0,"function":{"arguments":%s}}]}`, second), ""),
		openAIChunk(`{}`, "tool_calls"),
		"data: [DONE]\n\n",
	})
}

func streamText(w http.ResponseWriter, parts ...string) {
	frames := []string{openAIChunk(`{"role":"assistant","content":""}`, "")}
	for _, p := range parts {
		b, _ := json.Marshal(p)
		frames = append(frames, openAIChunk(fmt.Sprintf(`{"content":%s}`, b), ""))
	}
	frames = append(frames, openAIChunk(`{}`, "stop"), "data: [DONE]\n\n")
	writeAgentSSE(w, frames)
}

const agentTestPage = `<!doctype html><html><head><title>Stella of the End - Official</title>
<meta name="description" content="A visual novel by Key.">
<meta property="og:image" content="/img/cover.jpg">
<script>window.tracking = "should be stripped";</script></head>
<body><h1>Stella of the End</h1><p>Developer: Key</p><p>Release: 2024-05-31</p></body></html>`

func newAgentTestServer(rec *agentRecorder) *httptest.Server {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.add(r.URL.Path, body)
		switch {
		case r.URL.Path == "/site/game.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, agentTestPage)
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			hasToolResult := strings.Contains(string(body), `"role":"tool"`)
			switch {
			case strings.Contains(string(body), "DELETE_TEST") && !hasToolResult:
				streamToolCall(w, "delete_games", map[string]any{"ids": []string{"g1"}})
			case strings.Contains(string(body), "DELETE_TEST"):
				streamText(w, "好的，", "已取消删除。")
			case !hasToolResult:
				streamToolCall(w, "web_fetch", map[string]any{"url": srv.URL + "/site/game.html"})
			default:
				streamText(w, "这是 **Stella of the End**，", "开发商是 Key。")
			}
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

func loadAgentExample(t *testing.T, baseURL string) (*Manager, *Plugin) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "plugin-sdk", "examples", "ai-agent"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.js")); err != nil {
		t.Skipf("ai-agent bundle not built: %v", err)
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config: &appconf.AppConfig{
			Language:       "zh-CN",
			PluginDevPaths: []string{dir},
			AIProvider:     "custom",
			AIBaseURL:      baseURL + "/v1",
			AIAPIKey:       "sk-test",
			AIModel:        "test-model",
		},
		DataDir:     t.TempDir(),
		HostVersion: "2.1.0",
	})
	t.Cleanup(mgr.Shutdown)
	mgr.Discover()
	mgr.loadEnabledPlugins()
	p := mgr.activePlugin("example-ai-agent")
	if p == nil {
		for _, info := range mgr.List() {
			if info.ID == "example-ai-agent" {
				t.Fatalf("ai-agent failed to load: status=%s err=%s", info.Status, info.LastError)
			}
		}
		t.Fatal("ai-agent not discovered")
	}
	return mgr, p
}

type agentSnapshot struct {
	Running  bool `json:"running"`
	Messages []struct {
		Role   string `json:"role"`
		Error  string `json:"error"`
		Blocks []struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
			Call *struct {
				Name   string `json:"name"`
				Label  string `json:"label"`
				Args   string `json:"args"`
				Status string `json:"status"`
				Result string `json:"result"`
			} `json:"call"`
		} `json:"blocks"`
	} `json:"messages"`
}

func agentSend(t *testing.T, p *Plugin, text string) agentSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var fn moejs.Value
	if err := p.runSync(ctx, func(rt *moejs.Runtime) error {
		v, ok := rt.Export("sendForTest")
		if !ok || v.IsUndefined() {
			return fmt.Errorf("export sendForTest not found")
		}
		fn = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := p.Call(ctx, fn, text)
	if err != nil {
		t.Fatalf("sendForTest: %v", err)
	}
	var snap agentSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("decode snapshot: %v (%s)", err, raw)
	}
	return snap
}

func TestAIAgentExampleStreamsToolCallsAndRenders(t *testing.T) {
	rec := &agentRecorder{}
	srv := newAgentTestServer(rec)
	defer srv.Close()
	mgr, p := loadAgentExample(t, srv.URL)

	snap := agentSend(t, p, "帮我看看这个游戏的官网")
	if snap.Running || len(snap.Messages) != 2 {
		t.Fatalf("expected one user + one assistant message, got %+v", snap)
	}
	reply := snap.Messages[1]
	if reply.Role != "assistant" || reply.Error != "" {
		t.Fatalf("assistant reply: %+v", reply)
	}
	if len(reply.Blocks) != 2 || reply.Blocks[0].Kind != "tool" || reply.Blocks[1].Kind != "text" {
		t.Fatalf("expected [tool, text] blocks, got %+v", reply.Blocks)
	}
	call := reply.Blocks[0].Call
	if call.Name != "web_fetch" || call.Label != "读取网页" || call.Status != "done" {
		t.Fatalf("tool call: %+v", call)
	}
	if !strings.Contains(call.Args, "/site/game.html") {
		t.Fatalf("tool args should be the streamed JSON: %s", call.Args)
	}
	if !strings.Contains(call.Result, "Stella of the End") || !strings.Contains(call.Result, srv.URL+"/img/cover.jpg") || strings.Contains(call.Result, "should be stripped") {
		t.Fatalf("web_fetch result: %s", call.Result)
	}
	if reply.Blocks[1].Text != "这是 **Stella of the End**，开发商是 Key。" {
		t.Fatalf("streamed text: %q", reply.Blocks[1].Text)
	}
	chats := rec.chat()
	if len(chats) != 2 || !strings.Contains(chats[1].Body, "Developer: Key") {
		t.Fatalf("second model request should carry the tool result: %d requests", len(chats))
	}
	if !strings.Contains(chats[0].Body, `"stream":true`) || !strings.Contains(chats[0].Body, `"name":"search_games"`) {
		t.Fatalf("first request should stream and declare the library tools: %s", chats[0].Body)
	}

	// 侧边栏页面：挂载后描述树里应有工具卡片与回复正文
	trees := make(chan UITreeUpdate, 8)
	unsub := mgr.UI().SubscribeTreeUpdates(func(u UITreeUpdate) { trees <- u })
	defer unsub()
	mgr.MountSlot("test-page", "example-ai-agent", SlotSidebarPage, "ai-agent", map[string]any{})
	select {
	case u := <-trees:
		if u.Error != "" || u.Tree == nil {
			t.Fatalf("page render error: %s", u.Error)
		}
		raw, _ := json.Marshal(u.Tree)
		for _, want := range []string{"读取网页", "这是 **Stella of the End**", "Collapsible", "Markdown", "Textarea", "帮我看看这个游戏的官网"} {
			if !strings.Contains(string(raw), want) {
				t.Errorf("rendered page is missing %q", want)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("page never rendered")
	}
}

func TestAIAgentExampleRejectedConfirmBecomesToolError(t *testing.T) {
	rec := &agentRecorder{}
	srv := newAgentTestServer(rec)
	defer srv.Close()
	mgr, p := loadAgentExample(t, srv.URL)

	// 删除游戏始终要确认：拦下对话框并拒绝
	unsub := mgr.UI().SubscribeActions(func(a UIAction) {
		if a.Type == "dialog" && a.Dialog != nil && a.Dialog.Danger {
			go mgr.ResolveDialog(a.PluginID, a.DialogID, json.RawMessage("false"))
		}
	})
	defer unsub()

	snap := agentSend(t, p, "DELETE_TEST 删掉 g1")
	reply := snap.Messages[len(snap.Messages)-1]
	if len(reply.Blocks) < 1 || reply.Blocks[0].Call == nil {
		t.Fatalf("expected a tool block, got %+v", reply.Blocks)
	}
	call := reply.Blocks[0].Call
	if call.Name != "delete_games" || call.Status != "error" || !strings.Contains(call.Result, "拒绝") {
		t.Fatalf("rejected delete should be a tool error: %+v", call)
	}
	chats := rec.chat()
	if len(chats) != 2 || !strings.Contains(chats[1].Body, "拒绝") {
		t.Fatalf("the rejection should be reported back to the model")
	}
}

func countUINodes(n *UINode) int {
	if n == nil {
		return 0
	}
	total := 1
	for _, ch := range n.Children {
		total += countUINodes(ch)
	}
	return total
}

// 很长的对话（大量工具调用）也要留在描述树 2000 个节点的上限内：更早的消息与工具卡片会被折叠。
func TestAIAgentExampleLongSessionStaysUnderNodeLimit(t *testing.T) {
	srv := newAgentTestServer(&agentRecorder{})
	defer srv.Close()
	mgr, p := loadAgentExample(t, srv.URL)

	var ui []map[string]any
	for i := 0; i < 60; i++ {
		ui = append(ui, map[string]any{"id": fmt.Sprintf("u%d", i), "role": "user", "at": i, "blocks": []any{map[string]any{"kind": "text", "text": fmt.Sprintf("问题 %d", i)}}})
		var blocks []any
		for j := 0; j < 45; j++ {
			blocks = append(blocks, map[string]any{"kind": "tool", "call": map[string]any{"id": fmt.Sprintf("c%d-%d", i, j), "name": "search_games", "label": "搜索游戏库", "args": `{"query":"x"}`, "status": "done", "result": "{}"}})
		}
		blocks = append(blocks, map[string]any{"kind": "text", "text": fmt.Sprintf("回答 %d", i)})
		ui = append(ui, map[string]any{"id": fmt.Sprintf("a%d", i), "role": "assistant", "at": i, "blocks": blocks})
	}
	raw, _ := json.Marshal([]any{map[string]any{"id": "long", "title": "很长的对话", "createdAt": 1, "updatedAt": 2, "messages": []any{}, "ui": ui}})
	p.storage.Set("sessions", raw)
	if err := mgr.Reload("example-ai-agent"); err != nil {
		t.Fatal(err)
	}

	trees := make(chan UITreeUpdate, 8)
	unsub := mgr.UI().SubscribeTreeUpdates(func(u UITreeUpdate) { trees <- u })
	defer unsub()
	mgr.MountSlot("long-page", "example-ai-agent", SlotSidebarPage, "ai-agent", map[string]any{})
	select {
	case u := <-trees:
		if u.Error != "" || u.Tree == nil {
			t.Fatalf("long session failed to render: %s", u.Error)
		}
		if n := countUINodes(u.Tree); n > MaxUITreeNodes {
			t.Fatalf("tree has %d nodes", n)
		}
		out, _ := json.Marshal(u.Tree)
		for _, want := range []string{"回答 59", "次工具调用已折叠", "未显示"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("long session view is missing %q", want)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("page never rendered")
	}
}
