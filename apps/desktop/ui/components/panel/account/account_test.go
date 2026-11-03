package account

import (
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// roundTrip 是假的 HTTP 传输：测试不访问网络。
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

// failing 对所有请求返回 500。
var failing = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
	return respond(http.StatusInternalServerError, "offline"), nil
})}

type harness struct {
	svc       *host.Backend
	env       *env.Env
	cfg       appconf.AppConfig // 设置页的草稿
	state     State
	expanded  Provider
	hidden    bool // isContentVisible 为 false
	expands   []Provider
	changes   int
	refreshes int
	tt        *ui.Tester
	announced []string
}

// newHarness 在真实服务上打开三张卡片（grid-cols-1，宽 1280：lg 断点，卡片高 160）。setup 在第一帧之前修改
// 服务的配置（授权状态），services 替换服务的 HTTP 客户端等；之后草稿从服务的配置复制。
func newHarness(t *testing.T, expanded Provider, setup func(cfg *appconf.AppConfig), services ...func(svc *host.Backend)) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t), expanded: expanded}
	for _, s := range []interface{ SetHTTPClient(*http.Client) }{h.svc.Bangumi, h.svc.Hikarinagi, h.svc.Kungal} {
		s.SetHTTPClient(failing)
	}
	noBrowser := func(string) error { return errors.New("no browser in tests") }
	h.svc.Bangumi.SetOpenURLFunc(noBrowser)
	h.svc.Hikarinagi.SetOpenURLFunc(noBrowser)
	h.svc.Kungal.SetOpenURLFunc(noBrowser)
	for _, fn := range services {
		fn(h.svc)
	}
	if setup != nil {
		setup(h.svc.Config)
	}
	h.cfg = *h.svc.Config
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1280, 900)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) settings() panel.Settings {
	return panel.Settings{
		Env:    h.env,
		Config: &h.cfg,
		Change: func(fn func(cfg *appconf.AppConfig)) {
			fn(&h.cfg)
			h.changes++
		},
		PatchLive: func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg) },
		Refresh: func() {
			h.refreshes++
			h.cfg = *h.svc.Config
		},
	}
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	p := h.settings()
	ui.Column(c).Fill().Padding(20).Gap(12).Background(t.PanelBg).Children(func() {
		for _, pr := range Providers {
			ui.Box(c.Key(string(pr))).FillWidth().Children(func() {
				Card(c, t, &h.state, p, pr, CardProps{
					Expanded:       h.expanded == pr,
					ContentVisible: !h.hidden,
					OnExpand:       func() { h.expands = append(h.expands, pr) },
				})
			})
		}
	})
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

// clickConfirm 点击确认弹窗的确认按钮（它在“取消”右边 gap-3 处；文字可能与卡片上按钮的无障碍名称相同）。
func (h *harness) clickConfirm(t *testing.T) {
	t.Helper()
	r, ok := h.tt.Find(locales.CommonCancel())
	if !ok {
		t.Fatalf("no confirm modal; texts %q", h.tt.Texts())
	}
	h.tt.ClickAt(r.X+r.W+12+16, r.Y+r.H/2)
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) expectNot(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if h.tt.HasText(s) {
			t.Errorf("unexpected %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) toasts() []string {
	h.announced = append(h.announced, h.tt.Announcements()...)
	return h.announced
}

func (h *harness) expectToast(t *testing.T, msg string) {
	t.Helper()
	if !slices.ContainsFunc(h.toasts(), func(s string) bool { return strings.Contains(s, msg) }) {
		t.Errorf("no toast %q in %q", msg, h.announced)
	}
}

func authorizeBangumi(cfg *appconf.AppConfig) {
	cfg.BangumiAccessToken = "token"
	cfg.BangumiAuthorizedUserID = "42"
	cfg.BangumiAuthorizedUsername = "luna"
}

func TestCollapsedUnauthorized(t *testing.T) {
	h := newHarness(t, "", nil)
	h.expect(t, "Bangumi", "Hikarinagi", locales.SettingsBasicKungalProviderName(),
		locales.SettingsBasicBangumiAuthUnauthorized(), locales.SettingsBasicHikarinagiAuthUnauthorized(),
		locales.SettingsBasicKungalAuthUnauthorized())
	h.expectNot(t, locales.SettingsBasicBangumiAuthorize())
	for i, cs := range h.state.cards {
		if cs.snapshot == nil || cs.snapshot.authorized || cs.statusLoading {
			t.Errorf("card %d: status not loaded on mount: %+v", i, cs.snapshot)
		}
	}

	h.click(t, "Hikarinagi")
	if !slices.Equal(h.expands, []Provider{Hikarinagi}) {
		t.Fatalf("onExpand calls %v", h.expands)
	}

	// 内容隐藏（isContentVisible=false，pointer-events-none）时点击不展开。
	h.hidden = true
	h.frames(3)
	h.click(t, "Bangumi")
	if len(h.expands) != 1 {
		t.Errorf("hidden card expanded: %v", h.expands)
	}
}

func TestExpandedUnauthorizedAuthorizeFails(t *testing.T) {
	h := newHarness(t, Bangumi, nil)
	h.expect(t, locales.SettingsBasicBangumiAuthHint(), locales.SettingsBasicBangumiAuthorize(),
		locales.SettingsBasicHikarinagiAuthUnauthorized(), locales.SettingsBasicKungalAuthUnauthorized())
	h.expectNot(t, locales.SettingsBasicBangumiStatusPushLabel())

	// 测试构建没有注入 Bangumi OAuth 凭据：StartAuth 直接失败，不打开浏览器。
	h.click(t, locales.SettingsBasicBangumiAuthorize())
	h.frames(2)
	h.expectToast(t, locales.SettingsBasicBangumiAuthActionFailed("Bangumi OAuth"))
	cs := h.state.cards[0]
	if cs.authorizing || cs.statusLoading {
		t.Errorf("still loading: authorizing %v status %v", cs.authorizing, cs.statusLoading)
	}
	if h.refreshes != 0 {
		t.Errorf("config refreshed after failed authorization")
	}
}

func TestAuthorizedProfileAndStatusPush(t *testing.T) {
	h := newHarness(t, Bangumi, func(cfg *appconf.AppConfig) {
		authorizeBangumi(cfg)
	}, func(svc *host.Backend) {
		// GetProfile 从假的 /v0/me 读到昵称。
		svc.Bangumi.SetHTTPClient(&http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "Bearer token" {
				return respond(http.StatusUnauthorized, "no token"), nil
			}
			return respond(http.StatusOK, `{"id":42,"username":"luna","nickname":"Luna Nick","avatar":{}}`), nil
		})})
	})
	h.expect(t, "Luna Nick", "@luna", locales.SettingsBasicBangumiStatusPushLabel(), locales.SettingsBasicBangumiStatusPushHint(),
		"L") // 没有头像时显示首字母
	h.expectNot(t, locales.SettingsBasicBangumiAuthorize(), locales.SettingsBasicBangumiAuthHint())
	for _, label := range []string{locales.SettingsBasicBangumiSyncAllLabel(), locales.SettingsBasicBangumiDisconnect()} {
		if _, ok := h.tt.Find(label); !ok {
			t.Errorf("missing button %q", label)
		}
	}

	// 开关改草稿（onChange）：未设置时视为开启，点击后关闭。
	if v := h.cfg.BangumiStatusPushEnabled; v != nil && !*v {
		t.Fatalf("status push initially off")
	}
	r, ok := h.tt.Find(locales.SettingsBasicBangumiStatusPushLabel())
	if !ok {
		t.Fatal("no status push row")
	}
	// 开关在行的右端（卡片 p-3 之内），与 label、说明两行垂直居中。
	h.tt.ClickAt(1280-20-12-22, r.Y+r.H+2)
	h.frames(2)
	if h.changes != 1 || h.cfg.BangumiStatusPushEnabled == nil || *h.cfg.BangumiStatusPushEnabled {
		t.Errorf("status push not turned off: changes %d value %v", h.changes, h.cfg.BangumiStatusPushEnabled)
	}
}

func TestNeedsReauthorization(t *testing.T) {
	h := newHarness(t, Kungal, func(cfg *appconf.AppConfig) {
		cfg.BangumiAuthError = "expired"
		cfg.KungalAuthorizedUsername = "kun"
		cfg.KungalAuthError = "refresh token revoked"
	})
	// 收起的 Bangumi 卡片说明“需要重新授权”；展开的鲲 Galgame 卡片有标签、重新连接说明、上次错误与“重新授权”。
	h.expect(t, locales.SettingsBasicBangumiAuthNeedsReauth(), locales.SettingsBasicKungalAuthNeedsReauth(),
		locales.SettingsBasicKungalAuthReconnectHint(), locales.SettingsBasicKungalReauthorize(),
		locales.SettingsBasicKungalAuthLastErrorLabel()+": refresh token revoked", locales.SettingsBasicKungalProviderName())
	h.expectNot(t, locales.SettingsBasicKungalStatusPushLabel(), "@kun")
}

func TestDisconnectConfirm(t *testing.T) {
	h := newHarness(t, Hikarinagi, func(cfg *appconf.AppConfig) {
		cfg.HikarinagiAccessToken = "token"
		cfg.HikarinagiAuthorizedUserID = "7"
		cfg.HikarinagiAuthorizedUsername = "hika"
	})
	// GetProfile 失败（假的 500）时显示授权状态里的用户名。
	h.expect(t, "hika", "@hika", locales.SettingsBasicHikarinagiStatusPushLabel())

	// 内容淡出期间（pointer-events-none）按钮不响应。
	h.hidden = true
	h.frames(2)
	h.click(t, locales.SettingsBasicHikarinagiDisconnect())
	h.expectNot(t, locales.SettingsBasicHikarinagiDisconnectConfirmTitle())
	h.hidden = false
	h.frames(2)

	h.click(t, locales.SettingsBasicHikarinagiDisconnect())
	h.expect(t, locales.SettingsBasicHikarinagiDisconnectConfirmTitle(), locales.SettingsBasicHikarinagiDisconnectConfirmMsg())
	h.click(t, locales.CommonCancel())
	h.expectNot(t, locales.SettingsBasicHikarinagiDisconnectConfirmTitle())
	if h.svc.Config.HikarinagiAccessToken == "" {
		t.Fatal("cancel disconnected")
	}

	h.click(t, locales.SettingsBasicHikarinagiDisconnect())
	h.clickConfirm(t)
	h.frames(2)
	if h.svc.Config.HikarinagiAccessToken != "" || h.cfg.HikarinagiAuthorizedUsername != "" {
		t.Errorf("not disconnected: service %q draft %q", h.svc.Config.HikarinagiAccessToken, h.cfg.HikarinagiAuthorizedUsername)
	}
	if h.refreshes != 1 {
		t.Errorf("config refreshed %d times, want 1", h.refreshes)
	}
	h.expectToast(t, locales.SettingsBasicHikarinagiDisconnectSuccess())
	h.expect(t, locales.SettingsBasicHikarinagiAuthorize(), locales.SettingsBasicHikarinagiAuthHint())
}

func TestSyncAllWithoutGames(t *testing.T) {
	h := newHarness(t, Bangumi, authorizeBangumi)
	h.click(t, locales.SettingsBasicBangumiSyncAllLabel())
	h.expect(t, locales.SettingsBasicBangumiSyncAllConfirmTitle(), locales.SettingsBasicBangumiSyncAllConfirmMsg())
	h.click(t, locales.SettingsBasicRemoteStatusSyncStart())
	// 没有关联 Bangumi 的游戏：同步立即完成。
	h.expect(t, locales.SettingsBasicRemoteStatusSyncDoneTitle("Bangumi"), "0 /0",
		locales.SettingsBasicRemoteStatusSyncEmpty("Bangumi"), locales.SettingsBasicRemoteStatusSyncSucceeded(), locales.CommonComplete())
	h.click(t, locales.CommonComplete())
	h.expectNot(t, locales.SettingsBasicRemoteStatusSyncDoneTitle("Bangumi"))
}

func TestProgressModalFromEvents(t *testing.T) {
	h := newHarness(t, Kungal, func(cfg *appconf.AppConfig) { cfg.KungalAccessToken = "token" })
	name := locales.SettingsBasicKungalProviderName()
	cs := &h.state.cards[2]
	cs.syncing = true
	h.svc.Events.Emit("kungal:status-sync-progress", vo.RemoteStatusSyncProgress{
		Provider: "kungal", Status: "running", Current: 3, Total: 10, GameName: "千恋＊万花", SucceededGames: 2, FailedGames: 0,
	})
	h.frames(2)
	h.expect(t, locales.SettingsBasicRemoteStatusSyncRunningTitle(name), "3 /10", "千恋＊万花", "2")
	h.expectNot(t, locales.CommonComplete())

	// 进行中不能关闭；失败事件结束同步，显示失败标题、错误与“完成”。
	cs.closeProgress()
	if !cs.progressOpen {
		t.Fatal("progress closed while syncing")
	}
	h.svc.Events.Emit("kungal:status-sync-progress", vo.RemoteStatusSyncProgress{
		Provider: "kungal", Status: "failed", Current: 4, Total: 10, GameName: "サクラノ詩", SucceededGames: 3, FailedGames: 1,
		LastError: "HTTP 503",
	})
	h.frames(2)
	if cs.syncing {
		t.Error("still syncing after failed event")
	}
	h.expect(t, locales.SettingsBasicRemoteStatusSyncFailedTitle(name), "4 /10", "HTTP 503", locales.CommonComplete())
	h.click(t, locales.CommonComplete())
	h.expectNot(t, locales.SettingsBasicRemoteStatusSyncFailedTitle(name))

	// 其他来源的事件不影响这张卡片。
	h.svc.Events.Emit("bangumi:status-sync-progress", vo.RemoteStatusSyncProgress{Status: "done"})
	h.frames(2)
	if cs.progressOpen || !h.state.cards[0].progressOpen {
		t.Errorf("event routed to the wrong card: kungal %v bangumi %v", cs.progressOpen, h.state.cards[0].progressOpen)
	}
}

func TestNarrowLayout(t *testing.T) {
	h := newHarness(t, Bangumi, authorizeBangumi)
	// 窄于 sm 时卡片随内容变高（min-h-[132px]），头部是纵向排列。
	h.tt.SetSize(600, 900)
	h.frames(3)
	h.expect(t, "@luna", locales.SettingsBasicBangumiStatusPushLabel())
	label, ok1 := h.tt.Find("@luna")
	push, ok2 := h.tt.Find(locales.SettingsBasicBangumiStatusPushLabel())
	if !ok1 || !ok2 || push.Y <= label.Y+16 {
		t.Errorf("unexpected layout: identity %v push %v", label, push)
	}
}
