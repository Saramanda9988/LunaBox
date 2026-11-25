package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moejs"
)

type aiStreamRequest struct {
	System   string           `json:"system,omitempty"`
	Messages []aiChatMessage  `json:"messages"`
	Tools    []json.RawMessage `json:"tools,omitempty"`
	Model    string           `json:"model,omitempty"`
	BaseURL  string           `json:"baseUrl,omitempty"`
	APIKey   string           `json:"apiKey,omitempty"`
	MaxTurns int              `json:"maxTurns,omitempty"`
}

type aiChatMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []aiToolCallEntry `json:"tool_calls,omitempty"`
}

type aiToolCallEntry struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (p *Plugin) resolveAIEndpoint(req aiStreamRequest) (apiURL, apiKey, model string, err error) {
	baseURL := strings.TrimSpace(req.BaseURL)
	apiKey = strings.TrimSpace(req.APIKey)
	model = strings.TrimSpace(req.Model)

	if cfg := p.manager.deps.Config; cfg != nil {
		if baseURL == "" {
			baseURL = strings.TrimSpace(cfg.AIBaseURL)
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(cfg.AIAPIKey)
		}
		if model == "" {
			model = strings.TrimSpace(cfg.AIModel)
		}
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	if apiKey == "" {
		return "", "", "", fmt.Errorf("未配置 AI API Key")
	}
	apiURL = strings.TrimRight(baseURL, "/") + "/chat/completions"
	return apiURL, apiKey, model, nil
}

func (p *Plugin) buildAIAPI() map[string]any {
	return map[string]any{
		"getConfig": p.safeNative("ai.getConfig", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			cfg := p.manager.deps.Config
			configured := false
			provider, model := "", ""
			if cfg != nil {
				configured = strings.TrimSpace(cfg.AIAPIKey) != ""
				provider = cfg.AIProvider
				model = cfg.AIModel
			}
			return toJSValue(p.rt, map[string]any{
				"configured": configured,
				"provider":   provider,
				"model":      model,
			})
		}),
		// getCredentials 把宿主设置页配置的 AI 凭据交给插件（插件完全信任），
		// 供插件自带的 agent 运行时（如 pi）直接调用模型，宿主不必维护各家协议适配。
		"getCredentials": p.safeNative("ai.getCredentials", func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
			out := map[string]any{"configured": false, "provider": "", "baseUrl": "", "apiKey": "", "model": ""}
			if cfg := p.manager.deps.Config; cfg != nil {
				baseURL := strings.TrimRight(strings.TrimSpace(cfg.AIBaseURL), "/")
				if baseURL == "" {
					baseURL = "https://api.openai.com/v1"
				}
				apiKey := strings.TrimSpace(cfg.AIAPIKey)
				out = map[string]any{
					"configured": apiKey != "",
					"provider":   cfg.AIProvider,
					"baseUrl":    baseURL,
					"apiKey":     apiKey,
					"model":      strings.TrimSpace(cfg.AIModel),
				}
			}
			return toJSValue(p.rt, out)
		}),
		"complete": p.asyncNative("ai.complete", func(ctx context.Context, args []json.RawMessage) (any, error) {
			var req aiStreamRequest
			if len(args) > 0 {
				if err := json.Unmarshal(args[0], &req); err != nil {
					return nil, err
				}
			}
			apiURL, apiKey, model, err := p.resolveAIEndpoint(req)
			if err != nil {
				return nil, err
			}
			msgs := buildAIMessages(req)
			body := map[string]any{
				"model":    model,
				"messages": msgs,
				"stream":   false,
			}
			if len(req.Tools) > 0 {
				body["tools"] = req.Tools
			}
			rawBody, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(rawBody))
			if err != nil {
				return nil, err
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)

			client := p.manager.newPluginHTTPClient(90 * time.Second)
			resp, err := client.Do(httpReq)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			respBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("AI 请求失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
			}
			var parsed struct {
				Choices []struct {
					Message aiChatMessage `json:"message"`
				} `json:"choices"`
				Usage map[string]any `json:"usage"`
			}
			if err := json.Unmarshal(respBytes, &parsed); err != nil {
				return nil, err
			}
			content := ""
			var toolCalls []aiToolCallEntry
			if len(parsed.Choices) > 0 {
				content = parsed.Choices[0].Message.Content
				toolCalls = parsed.Choices[0].Message.ToolCalls
			}
			return map[string]any{
				"content":   content,
				"toolCalls": toolCalls,
				"usage":     parsed.Usage,
			}, nil
		}),
		"stream": p.safeNative("ai.stream", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			reqVal := moejs.Arg(args, 0)
			handlersVal := moejs.Arg(args, 1)
			if !reqVal.IsObject() || !handlersVal.IsObject() {
				return moejs.Undefined(), throwJSError(r, "ai.stream 需要 req 和 handlers 参数")
			}
			rawReq, err := p.rt.AppendJSON(nil, reqVal)
			if err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			var req aiStreamRequest
			if err := json.Unmarshal(rawReq, &req); err != nil {
				return moejs.Undefined(), throwJSError(r, err.Error())
			}
			onDelta, _ := p.rt.Get(handlersVal, "onDelta")
			onToolCall, _ := p.rt.Get(handlersVal, "onToolCall")
			onDone, _ := p.rt.Get(handlersVal, "onDone")
			onError, _ := p.rt.Get(handlersVal, "onError")

			streamCtx, cancel := context.WithCancel(p.scope.Context())
			dispose := p.scope.Effect(cancel)

			go p.runAIStream(streamCtx, req, onDelta, onToolCall, onDone, onError, dispose)

			return p.rt.Function("abort", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
	}
}

func buildAIMessages(req aiStreamRequest) []aiChatMessage {
	msgs := make([]aiChatMessage, 0, len(req.Messages)+1)
	if strings.TrimSpace(req.System) != "" {
		msgs = append(msgs, aiChatMessage{Role: "system", Content: req.System})
	}
	msgs = append(msgs, req.Messages...)
	return msgs
}

func (p *Plugin) runAIStream(
	ctx context.Context,
	req aiStreamRequest,
	onDelta, onToolCall, onDone, onError moejs.Value,
	cleanup func(),
) {
	defer cleanup()

	emitErr := func(err error) {
		if ctx.Err() != nil || !onError.IsObject() {
			return
		}
		msg := err.Error()
		p.Post(func(rt *moejs.Runtime) {
			v, _ := toJSValue(rt, map[string]any{"message": msg})
			_, _ = rt.CallFunction(onError, moejs.Undefined(), v)
		})
	}

	apiURL, apiKey, model, err := p.resolveAIEndpoint(req)
	if err != nil {
		emitErr(err)
		return
	}

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 5
	}
	msgs := buildAIMessages(req)
	client := p.manager.newPluginHTTPClient(0) // 流式不设整包超时，由 ctx 控制

	var lastUsage map[string]any

	for turn := 0; turn < maxTurns; turn++ {
		if ctx.Err() != nil {
			return
		}
		payload := map[string]any{
			"model":    model,
			"messages": msgs,
			"stream":   true,
		}
		if len(req.Tools) > 0 {
			payload["tools"] = req.Tools
		}
		rawBody, _ := json.Marshal(payload)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(rawBody))
		if err != nil {
			emitErr(err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			emitErr(err)
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			emitErr(fmt.Errorf("AI 流式请求失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body))))
			return
		}

		// 30ms 节流合并高频 token 流
		var (
			bufMu      sync.Mutex
			deltaBuf   strings.Builder
			fullText   strings.Builder
			toolMap    = make(map[int]*aiToolCallEntry)
			toolOrder  []int
			flushTick  = time.NewTicker(30 * time.Millisecond)
			flushDone  = make(chan struct{})
		)
		flushDelta := func() {
			bufMu.Lock()
			chunk := deltaBuf.String()
			deltaBuf.Reset()
			bufMu.Unlock()
			if chunk != "" && onDelta.IsObject() {
				p.Post(func(rt *moejs.Runtime) {
					_, _ = rt.CallFunction(onDelta, moejs.Undefined(), moejs.String(chunk))
				})
			}
		}
		go func() {
			defer flushTick.Stop()
			for {
				select {
				case <-flushTick.C:
					flushDelta()
				case <-flushDone:
					flushDelta()
					return
				case <-ctx.Done():
					return
				}
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataStr == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content   string `json:"content"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Type     string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
				} `json:"choices"`
				Usage map[string]any `json:"usage"`
			}
			if json.Unmarshal([]byte(dataStr), &chunk) != nil {
				continue
			}
			if chunk.Usage != nil {
				lastUsage = chunk.Usage
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			d := chunk.Choices[0].Delta
			if d.Content != "" {
				bufMu.Lock()
				deltaBuf.WriteString(d.Content)
				bufMu.Unlock()
				fullText.WriteString(d.Content)
			}
			for _, tc := range d.ToolCalls {
				entry, exists := toolMap[tc.Index]
				if !exists {
					entry = &aiToolCallEntry{Type: "function"}
					toolMap[tc.Index] = entry
					toolOrder = append(toolOrder, tc.Index)
				}
				if tc.ID != "" {
					entry.ID = tc.ID
				}
				if tc.Type != "" {
					entry.Type = tc.Type
				}
				if tc.Function.Name != "" {
					entry.Function.Name += tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					entry.Function.Arguments += tc.Function.Arguments
				}
			}
		}
		scanErr := scanner.Err()
		resp.Body.Close()
		close(flushDone)
		if scanErr != nil {
			emitErr(scanErr)
			return
		}

		// 若没有 tool_calls 或未注册 onToolCall，结束循环
		if len(toolOrder) == 0 || !onToolCall.IsObject() {
			break
		}

		calls := make([]aiToolCallEntry, 0, len(toolOrder))
		for _, idx := range toolOrder {
			calls = append(calls, *toolMap[idx])
		}
		msgs = append(msgs, aiChatMessage{
			Role:      "assistant",
			Content:   fullText.String(),
			ToolCalls: calls,
		})

		for _, tc := range calls {
			var parsedArgs any
			if json.Unmarshal([]byte(tc.Function.Arguments), &parsedArgs) != nil {
				parsedArgs = tc.Function.Arguments
			}
			callPayload := map[string]any{
				"id":        tc.ID,
				"name":      tc.Function.Name,
				"arguments": parsedArgs,
				"rawArgs":   tc.Function.Arguments,
			}
			rawRes, err := p.Call(ctx, onToolCall, callPayload)
			toolOutput := ""
			if err != nil {
				toolOutput = fmt.Sprintf(`{"error":%q}`, err.Error())
			} else if len(rawRes) > 0 {
				var asStr string
				if json.Unmarshal(rawRes, &asStr) == nil {
					toolOutput = asStr
				} else {
					toolOutput = string(rawRes)
				}
			}
			msgs = append(msgs, aiChatMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
			})
		}
	}

	if onDone.IsObject() && ctx.Err() == nil {
		p.Post(func(rt *moejs.Runtime) {
			v, _ := toJSValue(rt, lastUsage)
			_, _ = rt.CallFunction(onDone, moejs.Undefined(), v)
		})
	}
}

// ─────────────────────────── ctx.http.sse ───────────────────────────

func (p *Plugin) buildHTTPAPI() map[string]any {
	return map[string]any{
		"sse": p.safeNative("http.sse", func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			targetURL := strings.TrimSpace(argString(args, 0))
			initVal := moejs.Arg(args, 1)
			handlersVal := moejs.Arg(args, 2)
			if targetURL == "" {
				return moejs.Undefined(), throwJSError(r, "http.sse 需要 URL")
			}
			// 兼容省略 init 直接传 (url, handlers)
			if handlersVal.IsUndefined() && initVal.IsObject() {
				if _, err := p.rt.Get(initVal, "onEvent"); err == nil {
					handlersVal = initVal
					initVal = moejs.Undefined()
				}
			}
			var initOpts struct {
				Method  string            `json:"method"`
				Headers map[string]string `json:"headers"`
				Body    string            `json:"body"`
			}
			initOpts.Method = http.MethodGet
			if initVal.IsObject() {
				if raw, err := p.rt.AppendJSON(nil, initVal); err == nil {
					_ = json.Unmarshal(raw, &initOpts)
					if initOpts.Method == "" {
						initOpts.Method = http.MethodGet
					}
				}
			}
			var onEvent, onDone, onError moejs.Value
			if handlersVal.IsObject() {
				onEvent, _ = p.rt.Get(handlersVal, "onEvent")
				onDone, _ = p.rt.Get(handlersVal, "onDone")
				onError, _ = p.rt.Get(handlersVal, "onError")
			}

			sseCtx, cancel := context.WithCancel(p.scope.Context())
			dispose := p.scope.Effect(cancel)

			go func() {
				defer dispose()
				var bodyReader io.Reader
				if initOpts.Body != "" {
					bodyReader = strings.NewReader(initOpts.Body)
				}
				req, err := http.NewRequestWithContext(sseCtx, initOpts.Method, targetURL, bodyReader)
				if err != nil {
					p.emitSSEError(sseCtx, onError, err)
					return
				}
				req.Header.Set("Accept", "text/event-stream")
				for k, v := range initOpts.Headers {
					req.Header.Set(k, v)
				}
				client := p.manager.newPluginHTTPClient(0)
				resp, err := client.Do(req)
				if err != nil {
					p.emitSSEError(sseCtx, onError, err)
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
					p.emitSSEError(sseCtx, onError, fmt.Errorf("SSE 请求失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body))))
					return
				}
				scanner := bufio.NewScanner(resp.Body)
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				eventName := "message"
				eventID := ""
				var dataLines []string
				flush := func() {
					if len(dataLines) == 0 {
						return
					}
					ev := map[string]any{
						"event": eventName,
						"id":    eventID,
						"data":  strings.Join(dataLines, "\n"),
					}
					eventName = "message"
					dataLines = nil
					if onEvent.IsObject() && sseCtx.Err() == nil {
						p.Post(func(rt *moejs.Runtime) {
							v, _ := toJSValue(rt, ev)
							_, _ = rt.CallFunction(onEvent, moejs.Undefined(), v)
						})
					}
				}
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						flush()
						continue
					}
					if strings.HasPrefix(line, ":") {
						continue
					}
					if strings.HasPrefix(line, "event:") {
						eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
					} else if strings.HasPrefix(line, "id:") {
						eventID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
					} else if strings.HasPrefix(line, "data:") {
						dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
					}
				}
				flush()
				if err := scanner.Err(); err != nil {
					p.emitSSEError(sseCtx, onError, err)
					return
				}
				if onDone.IsObject() && sseCtx.Err() == nil {
					p.Post(func(rt *moejs.Runtime) {
						_, _ = rt.CallFunction(onDone, moejs.Undefined())
					})
				}
			}()

			return p.rt.Function("abort", 0, func(_ *moejs.Realm, _ moejs.Value, _ []moejs.Value) (moejs.Value, error) {
				dispose()
				return moejs.Undefined(), nil
			}), nil
		}),
	}
}

func (p *Plugin) emitSSEError(ctx context.Context, onError moejs.Value, err error) {
	if ctx.Err() != nil || !onError.IsObject() {
		return
	}
	msg := err.Error()
	p.Post(func(rt *moejs.Runtime) {
		v, _ := toJSValue(rt, map[string]any{"message": msg})
		_, _ = rt.CallFunction(onError, moejs.Undefined(), v)
	})
}
