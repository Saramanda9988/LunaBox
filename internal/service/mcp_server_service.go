package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/version"
)

const (
	mcpProtocolVersion        = "2025-06-18"
	mcpListGamesMaxLimit      = 50
	mcpPlaySessionsMaxLimit   = 100
	mcpMetadataSearchMaxLimit = 20
	mcpHTTPPath               = "/mcp"
)

type MCPServerService struct {
	ctx         context.Context
	readService *MCPReadService

	mu      sync.Mutex
	server  *http.Server
	port    int
	enabled bool
}

func NewMCPServerService() *MCPServerService {
	return &MCPServerService{}
}

func (s *MCPServerService) Init(ctx context.Context) {
	s.ctx = ctx
}

func (s *MCPServerService) SetReadService(readService *MCPReadService) {
	s.readService = readService
}

func (s *MCPServerService) ApplyConfig(config appconf.AppConfig) error {
	enabled := config.MCPEnabled
	port := appconf.NormalizeMCPPort(config.MCPPort)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil && s.enabled == enabled && s.port == port {
		return nil
	}

	if s.server != nil {
		if err := s.shutdownLocked(); err != nil {
			return err
		}
	}

	s.enabled = enabled
	s.port = port
	if !enabled {
		applog.LogInfof(s.ctx, "MCP HTTP server disabled")
		return nil
	}

	if s.readService == nil {
		return fmt.Errorf("MCP read service is not initialized")
	}

	if err := s.startLocked(port); err != nil {
		s.enabled = false
		s.port = 0
		return err
	}

	applog.LogInfof(s.ctx, "MCP HTTP server listening on http://127.0.0.1:%d%s", port, mcpHTTPPath)
	return nil
}

func (s *MCPServerService) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdownLocked()
}

func (s *MCPServerService) startLocked(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("listen MCP HTTP server on port %d: %w", port, err)
	}

	mux := http.NewServeMux()
	mux.Handle(mcpHTTPPath, newMCPHTTPHandler(s.readService))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "LunaBox MCP server is available at %s\n", mcpHTTPPath)
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.server = server
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			applog.LogErrorf(s.ctx, "MCP HTTP server stopped unexpectedly: %v", serveErr)
		}
	}()

	return nil
}

func (s *MCPServerService) shutdownLocked() error {
	if s.server == nil {
		s.enabled = false
		s.port = 0
		return nil
	}

	server := s.server
	s.server = nil

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	s.enabled = false
	s.port = 0
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown MCP HTTP server: %w", err)
	}
	return nil
}

type mcpHTTPHandler struct {
	readService *MCPReadService
}

type mcpJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *mcpJSONRPCError `json:"error,omitempty"`
}

type mcpJSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

type mcpToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content           []mcpToolContent `json:"content"`
	StructuredContent any              `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError,omitempty"`
}

func newMCPHTTPHandler(readService *MCPReadService) http.Handler {
	return &mcpHTTPHandler{readService: readService}
}

func (h *mcpHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := validateMCPOrigin(r); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if err := validateMCPProtocolVersion(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		setMCPCORSHeaders(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodPost:
		h.handlePOST(w, r)
		return
	default:
		setMCPCORSHeaders(w, r)
		http.Error(w, "MCP endpoint only accepts POST JSON-RPC requests", http.StatusMethodNotAllowed)
		return
	}
}

func (h *mcpHTTPHandler) handlePOST(w http.ResponseWriter, r *http.Request) {
	setMCPCORSHeaders(w, r)
	defer r.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeMCPHTTPResponse(w, http.StatusBadRequest, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &mcpJSONRPCError{Code: -32700, Message: "parse error"},
		})
		return
	}

	var req mcpJSONRPCRequest
	if err := json.Unmarshal(bytes.TrimSpace(rawBody), &req); err != nil {
		writeMCPHTTPResponse(w, http.StatusBadRequest, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &mcpJSONRPCError{Code: -32700, Message: "parse error"},
		})
		return
	}

	if req.JSONRPC != "2.0" {
		writeMCPHTTPResponse(w, http.StatusBadRequest, mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      normalizeJSONRPCID(req.ID),
			Error:   &mcpJSONRPCError{Code: -32600, Message: "invalid jsonrpc version"},
		})
		return
	}

	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := h.handleRequest(req)
	writeMCPHTTPResponse(w, http.StatusOK, resp)
}

func (h *mcpHTTPHandler) handleRequest(req mcpJSONRPCRequest) mcpJSONRPCResponse {
	switch req.Method {
	case "initialize":
		return mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": mcpProtocolVersion,
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "lunabox",
					"version": version.Version,
				},
				"instructions": "LunaBox exposes read-only game data tools. Respect spoiler_context.global_level for spoiler-sensitive fields.",
			},
		}
	case "ping":
		return mcpJSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": h.toolDefinitions(),
			},
		}
	case "tools/call":
		result, err := h.handleToolCall(req.Params)
		if err != nil {
			return mcpJSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &mcpJSONRPCError{Code: -32602, Message: err.Error()},
			}
		}
		return mcpJSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	default:
		return mcpJSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcpJSONRPCError{Code: -32601, Message: "method not found"},
		}
	}
}

func (h *mcpHTTPHandler) handleToolCall(rawParams json.RawMessage) (mcpToolResult, error) {
	var params mcpToolCallParams
	if err := decodeMCPParams(rawParams, &params); err != nil {
		return mcpToolResult{}, fmt.Errorf("invalid tool call params: %w", err)
	}

	switch params.Name {
	case "list_games":
		var args vo.MCPListGamesRequest
		if err := decodeMCPArgs(params.Arguments, &args); err != nil {
			return mcpToolResult{}, err
		}
		result, err := h.readService.ListGames(args.Limit, args.Offset)
		return buildMCPToolResult(result, err), nil
	case "get_game":
		var args vo.MCPGetGameRequest
		if err := decodeMCPArgs(params.Arguments, &args); err != nil {
			return mcpToolResult{}, err
		}
		result, err := h.readService.GetGame(string(args.GameID))
		return buildMCPToolResult(result, err), nil
	case "start_game":
		var args vo.MCPStartGameRequest
		if err := decodeMCPArgs(params.Arguments, &args); err != nil {
			return mcpToolResult{}, err
		}
		result, err := h.readService.StartGame(string(args.GameID))
		return buildMCPToolResult(result, err), nil
	case "get_play_sessions":
		var args vo.MCPGetPlaySessionsRequest
		if err := decodeMCPArgs(params.Arguments, &args); err != nil {
			return mcpToolResult{}, err
		}
		result, err := h.readService.GetPlaySessions(string(args.GameID), args.Limit, args.Offset)
		return buildMCPToolResult(result, err), nil
	case "search_metadata_by_name":
		var args vo.MCPMetadataSearchRequest
		if err := decodeMCPArgs(params.Arguments, &args); err != nil {
			return mcpToolResult{}, err
		}
		result, err := h.readService.SearchMetadataByName(args.Name, args.Limit)
		return buildMCPToolResult(result, err), nil
	case "get_game_statistic":
		var args vo.MCPGameStatisticRequest
		if err := decodeMCPArgs(params.Arguments, &args); err != nil {
			return mcpToolResult{}, err
		}
		result, err := h.readService.GetGameStatistic(enums.Period(strings.TrimSpace(args.Period)))
		return buildMCPToolResult(result, err), nil
	default:
		return mcpToolResult{}, fmt.Errorf("unknown tool: %s", params.Name)
	}
}

func (h *mcpHTTPHandler) toolDefinitions() []mcpToolDefinition {
	return []mcpToolDefinition{
		{
			Name:        "list_games",
			Description: "List local LunaBox games using lightweight catalog fields only. This tool is read-only and excludes summaries, progress notes, routes, local paths, save paths, and process names.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     mcpListGamesMaxLimit,
						"description": "Maximum number of games to return. Defaults to 20.",
					},
					"offset": map[string]any{
						"type":        "integer",
						"minimum":     0,
						"description": "Zero-based offset for pagination.",
					},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        "get_game",
			Description: "Get detailed local game context by stable LunaBox game_id string. The id is not a numeric index. Includes metadata, tags, the latest progress snapshot when present, and spoiler_context.global_level.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"game_id": map[string]any{
						"type":        "string",
						"description": "Stable LunaBox local game ID string, not a numeric index.",
					},
				},
				"required":             []string{"game_id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "start_game",
			Description: "Launch a local LunaBox game by stable game_id string using the same GUI backend flow as the app, including play-session tracking when launch succeeds.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"game_id": map[string]any{
						"type":        "string",
						"description": "Stable LunaBox local game ID string, not a numeric index.",
					},
				},
				"required":             []string{"game_id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "get_play_sessions",
			Description: "Get bounded play-session history for one local game by stable game_id string. Sessions are ordered from newest to oldest by start_time. This tool is read-only.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"game_id": map[string]any{
						"type":        "string",
						"description": "Stable LunaBox local game ID string, not a numeric index.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     mcpPlaySessionsMaxLimit,
						"description": "Maximum number of play sessions to return. Defaults to 20.",
					},
					"offset": map[string]any{
						"type":        "integer",
						"minimum":     0,
						"description": "Zero-based offset for pagination.",
					},
				},
				"required":             []string{"game_id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "search_metadata_by_name",
			Description: "Search remote metadata by name using only metadata sources currently enabled in LunaBox configuration. Returns spoiler-sensitive fields together with spoiler_context.global_level.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Game name or search query.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     mcpMetadataSearchMaxLimit,
						"description": "Maximum number of metadata candidates to return. Defaults to 5.",
					},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "get_game_statistic",
			Description: "Return structured play statistics aligned with LunaBox built-in AI summary data, without prompt generation, model calls, or WebSearch. Includes spoiler_context.global_level.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"period": map[string]any{
						"type":        "string",
						"enum":        []string{"week", "month"},
						"description": "Statistic period. Defaults to week.",
					},
				},
				"additionalProperties": false,
			},
		},
	}
}

func validateMCPOrigin(r *http.Request) error {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return nil
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("invalid Origin header")
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "localhost", "127.0.0.1":
		return nil
	default:
		return fmt.Errorf("cross-origin MCP requests are not allowed")
	}
}

func validateMCPProtocolVersion(r *http.Request) error {
	versionHeader := strings.TrimSpace(r.Header.Get("MCP-Protocol-Version"))
	if versionHeader == "" || versionHeader == mcpProtocolVersion {
		return nil
	}
	return fmt.Errorf("unsupported MCP protocol version: %s", versionHeader)
}

func setMCPCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, MCP-Protocol-Version")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
}

func writeMCPHTTPResponse(w http.ResponseWriter, status int, resp mcpJSONRPCResponse) {
	payload, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal response failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

func normalizeJSONRPCID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func decodeMCPParams(raw json.RawMessage, out any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func decodeMCPArgs(raw json.RawMessage, out any) error {
	if len(bytes.TrimSpace(raw)) == 0 || strings.EqualFold(string(bytes.TrimSpace(raw)), "null") {
		raw = []byte("{}")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func buildMCPToolResult(result any, err error) mcpToolResult {
	if err != nil {
		return mcpToolResult{
			Content: []mcpToolContent{
				{
					Type: "text",
					Text: err.Error(),
				},
			},
			IsError: true,
		}
	}

	payload, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return mcpToolResult{
			Content: []mcpToolContent{
				{
					Type: "text",
					Text: marshalErr.Error(),
				},
			},
			IsError: true,
		}
	}

	return mcpToolResult{
		Content: []mcpToolContent{
			{
				Type: "text",
				Text: string(payload),
			},
		},
		StructuredContent: result,
	}
}
