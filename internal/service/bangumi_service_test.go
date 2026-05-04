package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

type rewriteHostTransport struct {
	base          *url.URL
	baseTransport http.RoundTripper
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	cloned.Host = t.base.Host
	return t.baseTransport.RoundTrip(cloned)
}

type fakeBangumiService struct {
	pushCalls int
	pushErr   error
}

func (f *fakeBangumiService) getValidAccessToken(context.Context) (string, error) {
	return "token", nil
}

func (f *fakeBangumiService) refreshAccessToken(context.Context) (string, error) {
	return "token", nil
}

func (f *fakeBangumiService) upsertSubjectCollectionStatus(context.Context, string, enums.GameStatus) error {
	f.pushCalls++
	return f.pushErr
}

func (f *fakeBangumiService) isGameEligibleForStatusPush(game models.Game) bool {
	return game.SourceType == enums.Bangumi && strings.TrimSpace(game.SourceID) != ""
}

func TestBangumiOAuthCallbackRejectsInvalidState(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	session := &bangumiAuthSession{
		state:      "expected-state",
		resultChan: make(chan bangumiAuthResult, 1),
	}

	req := httptest.NewRequest(http.MethodGet, bangumiOAuthCallbackPath+"?code=test-code&state=wrong-state", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	session.handleOAuthCallback(recorder, req)

	result := <-session.resultChan
	if !strings.Contains(result.Error, "状态") {
		t.Fatalf("expected state validation error, got %#v", result)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 callback page, got %d", recorder.Code)
	}
}

func TestBangumiServiceRefreshesExpiredToken(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	restoreEmit := emitRuntimeEvent
	emitRuntimeEvent = func(context.Context, string, ...interface{}) {}
	t.Cleanup(func() {
		emitRuntimeEvent = restoreEmit
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/access_token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form failed: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("expected refresh_token grant, got %q", got)
		}
		if got := r.Form.Get("refresh_token"); got != "refresh-old" {
			t.Fatalf("expected refresh-old token, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL failed: %v", err)
	}

	config := &appconf.AppConfig{
		BangumiAccessToken:    "access-old",
		BangumiRefreshToken:   "refresh-old",
		BangumiTokenExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
	}

	service := NewBangumiService()
	service.clientID = "client-id"
	service.clientSecret = "client-secret"
	service.httpClient = &http.Client{
		Transport: rewriteHostTransport{
			base:          baseURL,
			baseTransport: http.DefaultTransport,
		},
		Timeout: bangumiHTTPTimeout,
	}
	service.Init(context.Background(), nil, config)

	token, err := service.getValidAccessToken(context.Background())
	if err != nil {
		t.Fatalf("expected refresh success, got error: %v", err)
	}

	if token != "access-new" {
		t.Fatalf("expected refreshed access token, got %q", token)
	}
	if config.BangumiAccessToken != "access-new" {
		t.Fatalf("expected config access token to update, got %q", config.BangumiAccessToken)
	}
	if config.BangumiRefreshToken != "refresh-new" {
		t.Fatalf("expected config refresh token to update, got %q", config.BangumiRefreshToken)
	}
	if strings.TrimSpace(config.BangumiTokenExpiresAt) == "" {
		t.Fatal("expected config expiry metadata to be written")
	}
}

func TestBangumiStatusMappingAndEligibility(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	cases := []struct {
		status   enums.GameStatus
		expected string
		ok       bool
	}{
		{enums.StatusNotStarted, "wish", true},
		{enums.StatusPlaying, "doing", true},
		{enums.StatusCompleted, "done", true},
		{enums.StatusOnHold, "on_hold", true},
		{"dropped", "", false},
	}

	for _, tc := range cases {
		got, ok := mapGameStatusToBangumiCollectionType(tc.status)
		if got != tc.expected || ok != tc.ok {
			t.Fatalf("mapping mismatch for %s: got (%q, %v), want (%q, %v)", tc.status, got, ok, tc.expected, tc.ok)
		}
	}

	service := NewBangumiService()
	if !service.isGameEligibleForStatusPush(models.Game{SourceType: enums.Bangumi, SourceID: "123"}) {
		t.Fatal("expected Bangumi game with source_id to be eligible")
	}
	if service.isGameEligibleForStatusPush(models.Game{SourceType: enums.Local, SourceID: "123"}) {
		t.Fatal("expected non-Bangumi game to be ineligible")
	}
	if service.isGameEligibleForStatusPush(models.Game{SourceType: enums.Bangumi, SourceID: "   "}) {
		t.Fatal("expected missing Bangumi source_id to be ineligible")
	}
}

func TestUpdateGamePreservesLocalStatusWhenBangumiPushFails(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	restoreEmit := emitRuntimeEvent
	emitRuntimeEvent = func(context.Context, string, ...interface{}) {}
	t.Cleanup(func() {
		emitRuntimeEvent = restoreEmit
	})

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb failed: %v", err)
	}
	defer db.Close()

	queries := []string{
		`CREATE TABLE games (
			id TEXT PRIMARY KEY,
			name TEXT,
			cover_url TEXT,
			company TEXT,
			summary TEXT,
			rating DOUBLE DEFAULT 0,
			release_date TEXT DEFAULT '',
			path TEXT,
			save_path TEXT,
			process_name TEXT DEFAULT '',
			status TEXT DEFAULT 'not_started',
			source_type TEXT,
			cached_at TIMESTAMP,
			source_id TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			use_locale_emulator BOOLEAN DEFAULT FALSE,
			use_magpie BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE sync_tombstones (
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			parent_id TEXT DEFAULT '',
			secondary_id TEXT DEFAULT '',
			deleted_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (entity_type, entity_id, parent_id, secondary_id)
		)`,
		`CREATE TABLE play_sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			duration INTEGER,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("create schema failed: %v", err)
		}
	}

	if _, err := db.Exec(
		`INSERT INTO games (id, name, cover_url, company, summary, rating, release_date, path, save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at, use_locale_emulator, use_magpie)
		 VALUES (?, ?, '', '', '', 0, '', '', '', '', ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, FALSE, FALSE)`,
		"bangumi-game",
		"Bangumi Game",
		string(enums.StatusNotStarted),
		string(enums.Bangumi),
		"42",
	); err != nil {
		t.Fatalf("insert game failed: %v", err)
	}

	service := NewGameService()
	service.Init(context.Background(), db, &appconf.AppConfig{})
	service.SetBangumiService(&fakeBangumiService{pushErr: errors.New("push failed")})

	game, err := service.GetGameByID("bangumi-game")
	if err != nil {
		t.Fatalf("load game failed: %v", err)
	}
	game.Status = enums.StatusCompleted

	if err := service.UpdateGame(game); err != nil {
		t.Fatalf("expected local update to succeed even if push fails, got: %v", err)
	}

	savedGame, err := service.GetGameByID("bangumi-game")
	if err != nil {
		t.Fatalf("reload game failed: %v", err)
	}
	if savedGame.Status != enums.StatusCompleted {
		t.Fatalf("expected local status to remain updated, got %s", savedGame.Status)
	}
}
