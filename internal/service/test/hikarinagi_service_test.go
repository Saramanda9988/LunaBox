package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHikarinagiServiceSyncAllGameStatuses(t *testing.T) {
	applog.SetMode(applog.ModeCLI)
	db, cleanup := setupTestDB(t)
	defer cleanup()

	insertBangumiGame(t, db, "hikarinagi-batch-playing", enums.StatusPlaying, enums.Hikarinagi, "301")
	insertBangumiGame(t, db, "hikarinagi-batch-completed", enums.StatusCompleted, enums.Hikarinagi, "302")
	insertBangumiGame(t, db, "bangumi-not-in-batch", enums.StatusPlaying, enums.Bangumi, "401")

	var requestCount int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v3/open/user/me/rates/galgames/") {
			t.Fatalf("未预期的请求路径: %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("Hikarinagi 批量同步请求缺少 User-Agent")
		}
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"data":{}}`)
	}))
	defer testServer.Close()

	disabled := false
	finalProgress := vo.RemoteStatusSyncProgress{}
	svc := service.NewHikarinagiService()
	svc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	svc.SetEventEmitter(func(name string, values ...interface{}) {
		if name == "hikarinagi:status-sync-progress" && len(values) > 0 {
			if progress, ok := values[0].(vo.RemoteStatusSyncProgress); ok {
				finalProgress = progress
			}
		}
	})
	svc.Init(context.Background(), db, &appconf.AppConfig{
		HikarinagiAccessToken:       "access-token",
		HikarinagiTokenExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
		HikarinagiStatusPushEnabled: &disabled,
	})

	result, err := svc.SyncAllGameStatuses()
	if err != nil {
		t.Fatalf("批量同步 Hikarinagi 状态失败: %v", err)
	}
	if result.Total != 2 || result.SucceededGames != 2 || result.FailedGames != 0 {
		t.Fatalf("批量同步统计异常: %+v", result)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Fatalf("期望发送 2 个 Hikarinagi 请求，实际为 %d", requestCount)
	}
	if finalProgress.Status != "done" || finalProgress.Current != 2 {
		t.Fatalf("最终 Hikarinagi 进度事件异常: %+v", finalProgress)
	}
}

func TestGameServicePushesStatusToEveryLinkedProvider(t *testing.T) {
	applog.SetMode(applog.ModeCLI)
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const gameID = "multi-provider-status"
	insertBangumiGame(t, db, gameID, enums.StatusNotStarted, enums.Bangumi, "42")
	if _, err := db.Exec(`
		INSERT INTO game_metadata_sources (game_id, source_type, source_id, cached_at, created_at, updated_at)
		VALUES
			(?, 'bangumi', '42', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(?, 'hikarinagi', '84', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, gameID, gameID); err != nil {
		t.Fatalf("insert metadata sources: %v", err)
	}

	var bangumiCalls int32
	var hikarinagiCalls int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/users/-/collections/42":
			atomic.AddInt32(&bangumiCalls, 1)
			w.WriteHeader(http.StatusNoContent)
		case "/api/v3/open/user/me/rates/galgames/84":
			atomic.AddInt32(&hikarinagiCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":true,"data":{"status":"COMPLETED"}}`)
		default:
			t.Fatalf("unexpected status endpoint: %s", r.URL.Path)
		}
	}))
	defer testServer.Close()

	config := &appconf.AppConfig{
		BangumiAccessToken:       "bangumi-token",
		HikarinagiAccessToken:    "hikarinagi-token",
		HikarinagiTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
	bangumiSvc.SetEventEmitter(func(string, ...interface{}) {})
	bangumiSvc.Init(context.Background(), db, config)

	hikarinagiSvc := service.NewHikarinagiService()
	hikarinagiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	hikarinagiSvc.SetOAuthClientID("public-client-id")
	hikarinagiSvc.SetEventEmitter(func(string, ...interface{}) {})
	hikarinagiSvc.Init(context.Background(), db, config)

	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(string, ...interface{}) {})
	gameSvc.Init(context.Background(), db, config)
	gameSvc.SetBangumiService(bangumiSvc)
	gameSvc.SetHikarinagiService(hikarinagiSvc)

	game, err := gameSvc.GetGameByID(gameID)
	if err != nil {
		t.Fatalf("get multi-provider game: %v", err)
	}
	game.Status = enums.StatusCompleted
	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("update multi-provider game status: %v", err)
	}
	if atomic.LoadInt32(&bangumiCalls) != 1 || atomic.LoadInt32(&hikarinagiCalls) != 1 {
		t.Fatalf("expected one request per provider, got bangumi=%d hikarinagi=%d", bangumiCalls, hikarinagiCalls)
	}
}

func TestGameServiceSkipsUnauthorizedProviderWithoutAffectingOtherProvider(t *testing.T) {
	applog.SetMode(applog.ModeCLI)
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const gameID = "partially-authorized-status"
	insertBangumiGame(t, db, gameID, enums.StatusNotStarted, enums.Bangumi, "42")
	if _, err := db.Exec(`
		INSERT INTO game_metadata_sources (game_id, source_type, source_id, cached_at, created_at, updated_at)
		VALUES
			(?, 'bangumi', '42', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(?, 'hikarinagi', '84', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, gameID, gameID); err != nil {
		t.Fatalf("insert metadata sources: %v", err)
	}

	var bangumiCalls int32
	var hikarinagiCalls int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/users/-/collections/42":
			atomic.AddInt32(&bangumiCalls, 1)
			w.WriteHeader(http.StatusNoContent)
		case "/api/v3/open/user/me/rates/galgames/84":
			atomic.AddInt32(&hikarinagiCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":true,"data":{"status":"COMPLETED"}}`)
		default:
			t.Fatalf("unexpected status endpoint: %s", r.URL.Path)
		}
	}))
	defer testServer.Close()

	config := &appconf.AppConfig{
		HikarinagiAccessToken:    "hikarinagi-token",
		HikarinagiTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
	bangumiSvc.SetEventEmitter(func(string, ...interface{}) {})
	bangumiSvc.Init(context.Background(), db, config)

	hikarinagiSvc := service.NewHikarinagiService()
	hikarinagiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	hikarinagiSvc.SetOAuthClientID("public-client-id")
	hikarinagiSvc.SetEventEmitter(func(string, ...interface{}) {})
	hikarinagiSvc.Init(context.Background(), db, config)

	var bangumiFailures int32
	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(name string, _ ...interface{}) {
		if name == "bangumi:status-push-failed" {
			atomic.AddInt32(&bangumiFailures, 1)
		}
	})
	gameSvc.Init(context.Background(), db, config)
	gameSvc.SetBangumiService(bangumiSvc)
	gameSvc.SetHikarinagiService(hikarinagiSvc)

	game, err := gameSvc.GetGameByID(gameID)
	if err != nil {
		t.Fatalf("get multi-provider game: %v", err)
	}
	game.Status = enums.StatusCompleted
	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("update multi-provider game status: %v", err)
	}
	if atomic.LoadInt32(&bangumiCalls) != 0 || atomic.LoadInt32(&hikarinagiCalls) != 1 {
		t.Fatalf("expected only the authorized provider to receive a request, got bangumi=%d hikarinagi=%d", bangumiCalls, hikarinagiCalls)
	}
	if atomic.LoadInt32(&bangumiFailures) != 0 {
		t.Fatalf("unauthorized provider should be skipped before status push, got %d failure events", bangumiFailures)
	}
}

func TestHikarinagiServiceRefreshesRotatingTokenAndPushesMappedStatus(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	cases := []struct {
		name           string
		initial        enums.GameStatus
		status         enums.GameStatus
		expectedStatus string
	}{
		{name: "not started", initial: enums.StatusPlaying, status: enums.StatusNotStarted, expectedStatus: "PLAN"},
		{name: "want to play", initial: enums.StatusPlaying, status: enums.StatusWantToPlay, expectedStatus: "PLAN"},
		{name: "playing", initial: enums.StatusNotStarted, status: enums.StatusPlaying, expectedStatus: "GOING"},
		{name: "completed", initial: enums.StatusNotStarted, status: enums.StatusCompleted, expectedStatus: "COMPLETED"},
		{name: "on hold", initial: enums.StatusNotStarted, status: enums.StatusOnHold, expectedStatus: "ON_HOLD"},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			gameID := fmt.Sprintf("hikarinagi-%d", index)
			insertBangumiGame(t, db, gameID, tc.initial, enums.Hikarinagi, "42")

			var tokenRefreshCalls int32
			var statusCalls int32
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/oidc/token":
					if err := r.ParseForm(); err != nil {
						t.Fatalf("解析 refresh 表单失败: %v", err)
					}
					if r.Form.Get("grant_type") != "refresh_token" {
						t.Fatalf("期望 refresh_token grant，实际为 %q", r.Form.Get("grant_type"))
					}
					if r.Form.Get("client_id") != "public-client-id" {
						t.Fatalf("public client 未携带预期的 client_id: %q", r.Form.Get("client_id"))
					}
					if r.Form.Get("refresh_token") != "refresh-old" {
						t.Fatalf("期望旧 refresh token，实际为 %q", r.Form.Get("refresh_token"))
					}
					atomic.AddInt32(&tokenRefreshCalls, 1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600,"token_type":"Bearer"}`)
				case "/api/v3/open/user/me/rates/galgames/42":
					if got := r.Header.Get("Authorization"); got != "Bearer access-new" {
						t.Fatalf("期望刷新后的 access token，实际为 %q", got)
					}
					var payload struct {
						Status string `json:"status"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatalf("解析状态请求失败: %v", err)
					}
					if payload.Status != tc.expectedStatus {
						t.Fatalf("期望远端状态 %q，实际为 %q", tc.expectedStatus, payload.Status)
					}
					atomic.AddInt32(&statusCalls, 1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"success":true,"data":{"status":"`+tc.expectedStatus+`"}}`)
				default:
					t.Fatalf("未预期的请求地址: %s", r.URL.Path)
				}
			}))
			defer testServer.Close()

			config := &appconf.AppConfig{
				HikarinagiAccessToken:    "access-old",
				HikarinagiRefreshToken:   "refresh-old",
				HikarinagiTokenExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
			}
			hikarinagiSvc := service.NewHikarinagiService()
			hikarinagiSvc.SetOAuthClientID("public-client-id")
			hikarinagiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
			hikarinagiSvc.SetEventEmitter(func(string, ...interface{}) {})
			hikarinagiSvc.Init(context.Background(), nil, config)

			gameSvc := service.NewGameService()
			gameSvc.SetEventEmitter(func(string, ...interface{}) {})
			gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
			gameSvc.SetHikarinagiService(hikarinagiSvc)

			game, err := gameSvc.GetGameByID(gameID)
			if err != nil {
				t.Fatalf("读取测试游戏失败: %v", err)
			}
			game.Status = tc.status
			if err := gameSvc.UpdateGame(game); err != nil {
				t.Fatalf("更新游戏状态失败: %v", err)
			}

			if atomic.LoadInt32(&tokenRefreshCalls) != 1 || atomic.LoadInt32(&statusCalls) != 1 {
				t.Fatalf("期望刷新与状态请求各一次，实际 refresh=%d status=%d", tokenRefreshCalls, statusCalls)
			}
			if config.HikarinagiAccessToken != "access-new" || config.HikarinagiRefreshToken != "refresh-new" {
				t.Fatalf("轮换令牌保存失败: access=%q refresh=%q", config.HikarinagiAccessToken, config.HikarinagiRefreshToken)
			}
		})
	}
}

func TestGameServiceSkipsHikarinagiPushWhenDisabled(t *testing.T) {
	applog.SetMode(applog.ModeCLI)
	db, cleanup := setupTestDB(t)
	defer cleanup()
	insertBangumiGame(t, db, "hikarinagi-disabled", enums.StatusNotStarted, enums.Hikarinagi, "42")

	var requestCount int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	disabled := false
	hikarinagiSvc := service.NewHikarinagiService()
	hikarinagiSvc.SetOAuthClientID("public-client-id")
	hikarinagiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	hikarinagiSvc.SetEventEmitter(func(string, ...interface{}) {})
	hikarinagiSvc.Init(context.Background(), nil, &appconf.AppConfig{
		HikarinagiAccessToken:       "access-token",
		HikarinagiStatusPushEnabled: &disabled,
	})

	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(string, ...interface{}) {})
	gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
	gameSvc.SetHikarinagiService(hikarinagiSvc)
	game, err := gameSvc.GetGameByID("hikarinagi-disabled")
	if err != nil {
		t.Fatalf("读取测试游戏失败: %v", err)
	}
	game.Status = enums.StatusCompleted
	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("关闭 Hikarinagi 状态同步后，本地更新失败: %v", err)
	}
	if atomic.LoadInt32(&requestCount) != 0 {
		t.Fatalf("关闭状态同步后仍发生请求，次数为 %d", requestCount)
	}
}

func TestHikarinagiServiceReportsRemoteFailureWithoutRollingBackLocalStatus(t *testing.T) {
	applog.SetMode(applog.ModeCLI)
	db, cleanup := setupTestDB(t)
	defer cleanup()
	insertBangumiGame(t, db, "hikarinagi-failure", enums.StatusNotStarted, enums.Hikarinagi, "42")

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"success":false,"error":{"code":"AUTH_FORBIDDEN","message":"missing scope"}}`)
	}))
	defer testServer.Close()

	hikarinagiSvc := service.NewHikarinagiService()
	hikarinagiSvc.SetOAuthClientID("public-client-id")
	hikarinagiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	hikarinagiSvc.SetEventEmitter(func(string, ...interface{}) {})
	hikarinagiSvc.Init(context.Background(), nil, &appconf.AppConfig{HikarinagiAccessToken: "access-token"})

	var failureMessage string
	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(name string, values ...interface{}) {
		if name == "hikarinagi:status-push-failed" && len(values) > 0 {
			failureMessage = fmt.Sprint(values[0])
		}
	})
	gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
	gameSvc.SetHikarinagiService(hikarinagiSvc)
	game, err := gameSvc.GetGameByID("hikarinagi-failure")
	if err != nil {
		t.Fatalf("读取测试游戏失败: %v", err)
	}
	game.Status = enums.StatusCompleted
	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("远端失败影响了本地状态保存: %v", err)
	}
	saved, err := gameSvc.GetGameByID("hikarinagi-failure")
	if err != nil {
		t.Fatalf("重新读取游戏失败: %v", err)
	}
	if saved.Status != enums.StatusCompleted {
		t.Fatalf("本地状态发生回退: %s", saved.Status)
	}
	if !strings.Contains(failureMessage, "AUTH_FORBIDDEN") {
		t.Fatalf("失败事件缺少服务端错误信息: %q", failureMessage)
	}
}
