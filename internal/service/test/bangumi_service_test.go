package test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/service"
	"lunabox/internal/utils/imageutils"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func newBangumiHTTPClient(t *testing.T, serverURL string) *http.Client {
	t.Helper()

	baseURL, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("解析测试服务地址失败: %v", err)
	}

	return &http.Client{
		Transport: rewriteHostTransport{
			base:          baseURL,
			baseTransport: http.DefaultTransport,
		},
		Timeout: 30 * time.Second,
	}
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func insertBangumiGame(
	t *testing.T,
	db *sql.DB,
	id string,
	status enums.GameStatus,
	sourceType enums.SourceType,
	sourceID string,
) {
	t.Helper()

	_, err := db.Exec(
		`INSERT INTO games (id, name, cover_url, company, summary, rating, release_date, path, save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at, use_locale_emulator, use_magpie)
		 VALUES (?, ?, '', '', '', 0, '', '', '', '', ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, FALSE, FALSE)`,
		id,
		"Bangumi Game "+id,
		string(status),
		string(sourceType),
		sourceID,
	)
	if err != nil {
		t.Fatalf("插入测试游戏失败: %v", err)
	}
}

func TestBangumiService_StartAuthRejectsInvalidState(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	svc := service.NewBangumiService()
	svc.SetOAuthClientCredentials("client-id", "client-secret")
	svc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	svc.SetOpenURLFunc(func(ctx context.Context, _ string) error {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"http://127.0.0.1:23679/callback?code=test-code&state=wrong-state",
			nil,
		)
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	})
	svc.Init(context.Background(), nil, &appconf.AppConfig{})

	_, err := svc.StartAuth()
	if err == nil || !strings.Contains(err.Error(), "状态") {
		t.Fatalf("期望状态校验错误，实际为: %v", err)
	}
}

func TestBangumiService_RefreshExpiredTokenAndPushMappedStatus(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	cases := []struct {
		name         string
		initial      enums.GameStatus
		status       enums.GameStatus
		expectedType int
	}{
		{name: "not started", initial: enums.StatusPlaying, status: enums.StatusNotStarted, expectedType: 1},
		{name: "playing", initial: enums.StatusNotStarted, status: enums.StatusPlaying, expectedType: 3},
		{name: "completed", initial: enums.StatusNotStarted, status: enums.StatusCompleted, expectedType: 2},
		{name: "on hold", initial: enums.StatusNotStarted, status: enums.StatusOnHold, expectedType: 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			insertBangumiGame(t, db, fmt.Sprintf("bangumi-%d", tc.expectedType), tc.initial, enums.Bangumi, "42")

			var gotCollectionType string
			var tokenRefreshCalls int32

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/oauth/access_token":
					if err := r.ParseForm(); err != nil {
						t.Fatalf("解析 refresh 表单失败: %v", err)
					}
					if r.Form.Get("grant_type") != "refresh_token" {
						t.Fatalf("期望 refresh_token grant，实际为 %q", r.Form.Get("grant_type"))
					}
					if r.Form.Get("refresh_token") != "refresh-old" {
						t.Fatalf("期望旧 refresh token，实际为 %q", r.Form.Get("refresh_token"))
					}
					atomic.AddInt32(&tokenRefreshCalls, 1)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600,"token_type":"Bearer"}`)
				case "/v0/users/-/collections/42":
					if got := r.Header.Get("Authorization"); got != "Bearer access-new" {
						t.Fatalf("期望刷新后的 access token，实际为 %q", got)
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("读取收藏请求体失败: %v", err)
					}
					if !strings.Contains(string(body), fmt.Sprintf(`"type":%d`, tc.expectedType)) {
						t.Fatalf("期望收藏 type 为 %q，实际请求体 %s", tc.expectedType, string(body))
					}
					gotCollectionType = fmt.Sprintf("%d", tc.expectedType)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Fatalf("未预期的请求路径: %s", r.URL.Path)
				}
			}))
			defer testServer.Close()

			config := &appconf.AppConfig{
				BangumiAccessToken:    "access-old",
				BangumiRefreshToken:   "refresh-old",
				BangumiTokenExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
			}

			bangumiSvc := service.NewBangumiService()
			bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
			bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
			bangumiSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
			bangumiSvc.Init(context.Background(), nil, config)

			gameSvc := service.NewGameService()
			gameSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
			gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
			gameSvc.SetBangumiService(bangumiSvc)

			game, err := gameSvc.GetGameByID(fmt.Sprintf("bangumi-%d", tc.expectedType))
			if err != nil {
				t.Fatalf("读取测试游戏失败: %v", err)
			}
			game.Status = tc.status

			if err := gameSvc.UpdateGame(game); err != nil {
				t.Fatalf("更新游戏状态失败: %v", err)
			}

			if gotCollectionType != fmt.Sprintf("%d", tc.expectedType) {
				t.Fatalf("期望推送的收藏状态为 %d，实际为 %q", tc.expectedType, gotCollectionType)
			}
			if atomic.LoadInt32(&tokenRefreshCalls) != 1 {
				t.Fatalf("期望触发 1 次 token refresh，实际为 %d", tokenRefreshCalls)
			}
			if config.BangumiAccessToken != "access-new" || config.BangumiRefreshToken != "refresh-new" {
				t.Fatalf("期望配置中的 token 被刷新，实际 access=%q refresh=%q", config.BangumiAccessToken, config.BangumiRefreshToken)
			}
		})
	}
}

func TestBangumiService_GetProfileReturnsNicknameAndAvatar(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	var requestCount int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0/me":
			if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
				t.Fatalf("期望 access token 为 %q，实际为 %q", "access-token", got)
			}
			atomic.AddInt32(&requestCount, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":1,"username":"sai","nickname":"Sai","avatar":{"large":"https://lain.bgm.tv/pic/user/l/000/00/00/1.jpg","medium":"https://lain.bgm.tv/pic/user/m/000/00/00/1.jpg","small":"https://lain.bgm.tv/pic/user/s/000/00/00/1.jpg"}}`)
		case "/pic/user/l/000/00/00/1.jpg":
			atomic.AddInt32(&requestCount, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-image"))
		default:
			t.Fatalf("未预期的请求路径: %s", r.URL.Path)
		}
	}))
	defer testServer.Close()
	defer func() {
		_ = imageutils.RemoveManagedAvatar("bangumi", "1")
	}()

	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
	bangumiSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	bangumiSvc.Init(context.Background(), nil, &appconf.AppConfig{
		BangumiAccessToken: "access-token",
	})

	profile, err := bangumiSvc.GetProfile()
	if err != nil {
		t.Fatalf("获取 Bangumi profile 失败: %v", err)
	}
	if profile.UserID != "1" || profile.Username != "sai" || profile.Nickname != "Sai" {
		t.Fatalf("返回的 profile 信息不符合预期: %+v", profile)
	}
	if profile.AvatarURL == "" || profile.AvatarLarge == "" || profile.AvatarMedium == "" || profile.AvatarSmall == "" {
		t.Fatalf("期望返回完整头像地址，实际为 %+v", profile)
	}
	if !strings.HasPrefix(profile.AvatarURL, "/local/avatars/bangumi_1.") {
		t.Fatalf("期望返回本地缓存头像地址，实际为 %q", profile.AvatarURL)
	}
	avatarPath, _, err := imageutils.FindManagedAvatarFile("bangumi", "1")
	if err != nil {
		t.Fatalf("查找缓存头像失败: %v", err)
	}
	if avatarPath == "" {
		t.Fatalf("期望能找到本地缓存头像文件")
	}
	if _, statErr := os.Stat(avatarPath); statErr != nil {
		t.Fatalf("期望缓存头像文件存在，实际错误: %v", statErr)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Fatalf("期望请求 /v0/me 和头像图片各一次，实际为 %d", requestCount)
	}
}

func TestGameService_SkipsBangumiPushForIneligibleGames(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	cases := []struct {
		name       string
		sourceType enums.SourceType
		sourceID   string
	}{
		{name: "non bangumi source", sourceType: enums.Local, sourceID: "123"},
		{name: "missing source id", sourceType: enums.Bangumi, sourceID: "   "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			insertBangumiGame(t, db, "skip-"+strings.ReplaceAll(tc.name, " ", "-"), enums.StatusNotStarted, tc.sourceType, tc.sourceID)

			var requestCount int32
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer testServer.Close()

			bangumiSvc := service.NewBangumiService()
			bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
			bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
			bangumiSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
			bangumiSvc.Init(context.Background(), nil, &appconf.AppConfig{
				BangumiAccessToken: "access-token",
			})

			gameSvc := service.NewGameService()
			gameSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
			gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
			gameSvc.SetBangumiService(bangumiSvc)

			game, err := gameSvc.GetGameByID("skip-" + strings.ReplaceAll(tc.name, " ", "-"))
			if err != nil {
				t.Fatalf("读取测试游戏失败: %v", err)
			}
			game.Status = enums.StatusCompleted

			if err := gameSvc.UpdateGame(game); err != nil {
				t.Fatalf("非 Bangumi 可同步游戏更新不应失败: %v", err)
			}
			if atomic.LoadInt32(&requestCount) != 0 {
				t.Fatalf("期望不发生 Bangumi 请求，实际请求次数为 %d", requestCount)
			}
		})
	}
}

func TestGameService_SkipsBangumiPushWhenDisabled(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	db, cleanup := setupTestDB(t)
	defer cleanup()

	insertBangumiGame(t, db, "bangumi-push-disabled", enums.StatusNotStarted, enums.Bangumi, "42")

	var requestCount int32
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer testServer.Close()

	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
	bangumiSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	bangumiSvc.Init(context.Background(), nil, &appconf.AppConfig{
		BangumiAccessToken:       "access-token",
		BangumiStatusPushEnabled: boolPtr(false),
	})

	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
	gameSvc.SetBangumiService(bangumiSvc)

	game, err := gameSvc.GetGameByID("bangumi-push-disabled")
	if err != nil {
		t.Fatalf("读取测试游戏失败: %v", err)
	}
	game.Status = enums.StatusCompleted

	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("关闭 Bangumi 状态推送后，本地更新不应失败: %v", err)
	}
	if atomic.LoadInt32(&requestCount) != 0 {
		t.Fatalf("关闭 Bangumi 状态推送后不应发生请求，实际请求次数为 %d", requestCount)
	}
}

func TestGameService_PushFailureDoesNotRollbackLocalStatus(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	db, cleanup := setupTestDB(t)
	defer cleanup()

	insertBangumiGame(t, db, "bangumi-push-fail", enums.StatusNotStarted, enums.Bangumi, "42")

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"push failed"}`)
	}))
	defer testServer.Close()

	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
	bangumiSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	bangumiSvc.Init(context.Background(), nil, &appconf.AppConfig{
		BangumiAccessToken: "access-token",
	})

	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
	gameSvc.SetBangumiService(bangumiSvc)

	game, err := gameSvc.GetGameByID("bangumi-push-fail")
	if err != nil {
		t.Fatalf("读取测试游戏失败: %v", err)
	}
	game.Status = enums.StatusCompleted

	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("本地更新不应因 Bangumi 推送失败而失败: %v", err)
	}

	savedGame, err := gameSvc.GetGameByID("bangumi-push-fail")
	if err != nil {
		t.Fatalf("重新读取游戏失败: %v", err)
	}
	if savedGame.Status != enums.StatusCompleted {
		t.Fatalf("期望本地状态保留为 completed，实际为 %s", savedGame.Status)
	}
}

func TestGameService_AcceptsBangumi202Response(t *testing.T) {
	applog.SetMode(applog.ModeCLI)

	db, cleanup := setupTestDB(t)
	defer cleanup()

	insertBangumiGame(t, db, "bangumi-accepted", enums.StatusNotStarted, enums.Bangumi, "42")

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer testServer.Close()

	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.SetOAuthClientCredentials("client-id", "client-secret")
	bangumiSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	bangumiSvc.Init(context.Background(), nil, &appconf.AppConfig{
		BangumiAccessToken: "access-token",
	})

	gameSvc := service.NewGameService()
	gameSvc.SetEventEmitter(func(context.Context, string, ...interface{}) {})
	gameSvc.Init(context.Background(), db, &appconf.AppConfig{})
	gameSvc.SetBangumiService(bangumiSvc)

	game, err := gameSvc.GetGameByID("bangumi-accepted")
	if err != nil {
		t.Fatalf("读取测试游戏失败: %v", err)
	}
	game.Status = enums.StatusCompleted

	if err := gameSvc.UpdateGame(game); err != nil {
		t.Fatalf("Bangumi 返回 202 时不应报错: %v", err)
	}
}
