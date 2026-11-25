//go:build pispike

// Feasibility spike: runs the pi agent stack (@earendil-works/pi-ai + pi-agent-core)
// bundled into packages/plugin-sdk/examples/pi-agent-spike/main.js inside the real
// plugin runtime, against httptest servers that emulate the OpenAI Chat Completions
// and Anthropic Messages SSE streaming APIs.
//
// Build the bundle first (cd packages/plugin-sdk/examples/pi-agent-spike && npm i && node build.mjs), then:
//
//	cd packages/backend && go test -tags pispike -run TestPiSpike -p 4 -v ./plugin/
package plugin

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

type piSpikeResult struct {
	ToolCalls    []json.RawMessage `json:"toolCalls"`
	Deltas       []string          `json:"deltas"`
	Text         string            `json:"text"`
	Events       []string          `json:"events"`
	ErrorMessage string            `json:"errorMessage"`
	StopReason   string            `json:"stopReason"`
	MessageRoles []string          `json:"messageRoles"`
}

type piSpikeRequest struct {
	Path    string
	Headers http.Header
	Body    string
}

type piSpikeRecorder struct {
	mu   sync.Mutex
	reqs []piSpikeRequest
}

func (r *piSpikeRecorder) add(req *http.Request, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, piSpikeRequest{Path: req.URL.Path, Headers: req.Header.Clone(), Body: string(body)})
}

func (r *piSpikeRecorder) all() []piSpikeRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]piSpikeRequest(nil), r.reqs...)
}

func writeSSE(w http.ResponseWriter, frames []string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	for _, f := range frames {
		_, _ = io.WriteString(w, f)
		if fl != nil {
			fl.Flush()
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// OpenAI Chat Completions streaming emulator: tool call first, text once a tool result is present.
func newOpenAISpikeServer(t *testing.T, rec *piSpikeRecorder) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.add(r, body)
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.Error(w, `{"error":{"message":"unexpected path"}}`, http.StatusNotFound)
			return
		}
		chunk := func(delta string, finish string, extra string) string {
			fin := "null"
			if finish != "" {
				fin = `"` + finish + `"`
			}
			return fmt.Sprintf(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"spike-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]%s}`+"\n\n", delta, fin, extra)
		}
		usage := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"spike-model","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19}}` + "\n\n"
		if !strings.Contains(string(body), `"role":"tool"`) {
			writeSSE(w, []string{
				chunk(`{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_game_count","arguments":""}}]}`, "", ""),
				chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"status\":"}}]}`, "", ""),
				chunk(`{"tool_calls":[{"index":0,"function":{"arguments":"\"installed\"}"}}]}`, "", ""),
				chunk(`{}`, "tool_calls", ""),
				usage,
				"data: [DONE]\n\n",
			})
			return
		}
		writeSSE(w, []string{
			chunk(`{"role":"assistant","content":""}`, "", ""),
			chunk(`{"content":"You have "}`, "", ""),
			chunk(`{"content":"42 games."}`, "", ""),
			chunk(`{}`, "stop", ""),
			usage,
			"data: [DONE]\n\n",
		})
	}))
}

// Anthropic Messages streaming emulator: tool_use first, text once a tool_result is present.
func newAnthropicSpikeServer(t *testing.T, rec *piSpikeRecorder) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.add(r, body)
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			http.Error(w, `{"type":"error","error":{"type":"not_found_error","message":"unexpected path"}}`, http.StatusNotFound)
			return
		}
		ev := func(name, data string) string { return "event: " + name + "\ndata: " + data + "\n\n" }
		start := ev("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-spike","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":12,"output_tokens":1}}}`)
		if strings.Contains(string(body), `"tools"`) && !strings.Contains(string(body), `"tool_result"`) {
			writeSSE(w, []string{
				start,
				ev("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_game_count","input":{}}}`),
				ev("ping", `{"type":"ping"}`),
				ev("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"status\":"}}`),
				ev("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"installed\"}"}}`),
				ev("content_block_stop", `{"type":"content_block_stop","index":0}`),
				ev("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":7}}`),
				ev("message_stop", `{"type":"message_stop"}`),
			})
			return
		}
		writeSSE(w, []string{
			start,
			ev("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
			ev("ping", `{"type":"ping"}`),
			ev("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"You have "}}`),
			ev("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"42 games."}}`),
			ev("content_block_stop", `{"type":"content_block_stop","index":0}`),
			ev("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":7}}`),
			ev("message_stop", `{"type":"message_stop"}`),
		})
	}))
}

func loadPiSpikePlugin(t *testing.T) (*Manager, *Plugin) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "plugin-sdk", "examples", "pi-agent-spike"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.js")); err != nil {
		t.Skipf("pi-agent-spike bundle not built: %v", err)
	}
	mgr := NewManager()
	mgr.Init(context.Background(), Dependencies{
		Config:      &appconf.AppConfig{Language: "en-US", PluginDevPaths: []string{dir}},
		DataDir:     t.TempDir(),
		HostVersion: "2.1.0",
	})
	mgr.Discover()
	start := time.Now()
	mgr.loadEnabledPlugins()
	t.Logf("plugin load (compile + link + apply) took %s", time.Since(start))

	mgr.mu.RLock()
	mp := mgr.plugins["example-pi-agent-spike"]
	mgr.mu.RUnlock()
	if mp == nil || mp.runtime == nil {
		status, lastErr := PluginStatus(""), ""
		if mp != nil {
			status, lastErr = mp.status, mp.lastError
		}
		mgr.Shutdown()
		t.Fatalf("pi-agent-spike failed to load: status=%s err=%s", status, lastErr)
	}
	return mgr, mp.runtime
}

func callPiSpikeExport(t *testing.T, p *Plugin, name string, opts map[string]any) piSpikeResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var fn moejs.Value
	if err := p.runSync(ctx, func(rt *moejs.Runtime) error {
		v, ok := rt.Export(name)
		if !ok || v.IsUndefined() {
			return fmt.Errorf("export %s not found", name)
		}
		fn = v
		return nil
	}); err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}
	start := time.Now()
	raw, err := p.Call(ctx, fn, opts)
	t.Logf("%s took %s", name, time.Since(start))
	if err != nil {
		t.Fatalf("%s failed: %v", name, err)
	}
	var res piSpikeResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("decode %s result: %v (%s)", name, err, raw)
	}
	t.Logf("%s result: %s", name, raw)
	return res
}

func checkPiSpikeResult(t *testing.T, res piSpikeResult, reqs []piSpikeRequest) {
	t.Helper()
	if res.ErrorMessage != "" {
		t.Fatalf("agent error: %s (stopReason=%s)", res.ErrorMessage, res.StopReason)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("expected exactly one get_game_count call, got %d", len(res.ToolCalls))
	}
	if !strings.Contains(string(res.ToolCalls[0]), `"params":{"status":"installed"}`) {
		t.Fatalf("tool should receive streamed+validated args, got %s", res.ToolCalls[0])
	}
	if res.Text != "You have 42 games." {
		t.Fatalf("unexpected final text %q (deltas=%q)", res.Text, res.Deltas)
	}
	if len(res.Deltas) < 2 {
		t.Fatalf("expected streamed deltas, got %q", res.Deltas)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 provider requests, got %d", len(reqs))
	}
	if !strings.Contains(reqs[1].Body, "42") {
		t.Fatalf("second request should carry the tool result, body=%s", reqs[1].Body)
	}
}

func TestPiSpikeOpenAICompatible(t *testing.T) {
	rec := &piSpikeRecorder{}
	srv := newOpenAISpikeServer(t, rec)
	defer srv.Close()

	mgr, p := loadPiSpikePlugin(t)
	defer mgr.Shutdown()

	res := callPiSpikeExport(t, p, "runOpenAI", map[string]any{
		"baseUrl": srv.URL + "/v1",
		"apiKey":  "sk-spike",
	})
	reqs := rec.all()
	for i, r := range reqs {
		t.Logf("req %d %s auth=%q ua=%q body=%s", i, r.Path, r.Headers.Get("Authorization"), r.Headers.Get("User-Agent"), r.Body)
	}
	checkPiSpikeResult(t, res, reqs)
	if got := reqs[0].Headers.Get("Authorization"); got != "Bearer sk-spike" {
		t.Fatalf("unexpected Authorization header %q", got)
	}
}

func TestPiSpikeAnthropic(t *testing.T) {
	rec := &piSpikeRecorder{}
	srv := newAnthropicSpikeServer(t, rec)
	defer srv.Close()

	mgr, p := loadPiSpikePlugin(t)
	defer mgr.Shutdown()

	res := callPiSpikeExport(t, p, "runAnthropic", map[string]any{
		"baseUrl":  srv.URL,
		"apiKey":   "sk-ant-spike",
		"withTool": true,
	})
	reqs := rec.all()
	for i, r := range reqs {
		t.Logf("req %d %s x-api-key=%q ua=%q body=%s", i, r.Path, r.Headers.Get("X-Api-Key"), r.Headers.Get("User-Agent"), r.Body)
	}
	checkPiSpikeResult(t, res, reqs)
	if got := reqs[0].Headers.Get("X-Api-Key"); got != "sk-ant-spike" {
		t.Fatalf("unexpected x-api-key header %q", got)
	}
}

func TestPiSpikeProbeGlobals(t *testing.T) {
	mgr, p := loadPiSpikePlugin(t)
	defer mgr.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var fn moejs.Value
	if err := p.runSync(ctx, func(rt *moejs.Runtime) error {
		v, ok := rt.Export("probeGlobals")
		if !ok {
			return fmt.Errorf("probeGlobals not exported")
		}
		fn = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := p.Call(ctx, fn)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("globals: %s", raw)
}

// Aborting mid-stream (agent.abort()) must cancel the in-flight fetch and settle the run.
func TestPiSpikeOpenAIAbort(t *testing.T) {
	closed := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, `data: {"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"tok%d "},"finish_reason":null}]}`+"\n\n", i)
			fl.Flush()
			select {
			case <-r.Context().Done():
				closed <- struct{}{}
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}))
	defer srv.Close()

	mgr, p := loadPiSpikePlugin(t)
	defer mgr.Shutdown()

	start := time.Now()
	res := callPiSpikeExport(t, p, "runOpenAI", map[string]any{
		"baseUrl":           srv.URL + "/v1",
		"apiKey":            "sk-spike",
		"noTool":            true,
		"abortOnFirstDelta": true,
	})
	if res.StopReason != "aborted" {
		t.Fatalf("expected stopReason=aborted, got %q (err=%q)", res.StopReason, res.ErrorMessage)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("abort took too long: %s", time.Since(start))
	}
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatalf("server connection was not closed after abort")
	}
}

// A stream longer than the plugin http.Client timeout (30s in startPluginRuntime). Slow: set PI_SPIKE_LONG=1.
func TestPiSpikeLongStream(t *testing.T) {
	if os.Getenv("PI_SPIKE_LONG") == "" {
		t.Skip("set PI_SPIKE_LONG=1 (takes ~35s)")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for i := 0; i < 35; i++ {
			fmt.Fprintf(w, `data: {"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"t%d "},"finish_reason":null}]}`+"\n\n", i)
			fl.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Second):
			}
		}
		io.WriteString(w, `data: {"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	mgr, p := loadPiSpikePlugin(t)
	defer mgr.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var fn moejs.Value
	_ = p.runSync(ctx, func(rt *moejs.Runtime) error { fn, _ = rt.Export("runOpenAI"); return nil })
	raw, err := p.Call(ctx, fn, map[string]any{"baseUrl": srv.URL + "/v1", "apiKey": "k", "noTool": true})
	if err != nil {
		t.Fatal(err)
	}
	var res piSpikeResult
	_ = json.Unmarshal(raw, &res)
	t.Logf("deltas=%d stopReason=%s err=%q", len(res.Deltas), res.StopReason, res.ErrorMessage)
	if res.ErrorMessage != "" || len(res.Deltas) != 35 {
		t.Fatalf("long stream cut short: %d/35 deltas, err=%q", len(res.Deltas), res.ErrorMessage)
	}
}
