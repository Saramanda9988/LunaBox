package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/appruntime"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service/gamehelper"
	"lunabox/backend/service/remotestatus"
	"lunabox/backend/utils/httputils"
	"lunabox/backend/utils/imageutils"
	"lunabox/backend/utils/metadata"
	"lunabox/backend/version"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 鲲 Galgame 账号由 NextMoe 账户中心签发：OAuth 2.0 授权码 + PKCE（public client，无 secret），
// 回调只收环回地址且端口无关，所以每次授权都让内核分配临时端口。
const (
	kungalOAuthIssuer        = "https://account.nextmoe.com"
	kungalOAuthAuthorizeURL  = kungalOAuthIssuer + "/api/v1/oauth/authorize"
	kungalOAuthTokenURL      = kungalOAuthIssuer + "/api/v1/oauth/token"
	kungalOAuthUserInfoURL   = kungalOAuthIssuer + "/api/v1/oauth/userinfo"
	kungalOAuthRevokeURL     = kungalOAuthIssuer + "/api/v1/oauth/revoke"
	kungalAPIBaseURL         = "https://api.nextmoe.dev/v2"
	kungalWorkStatesURL      = kungalAPIBaseURL + "/me/work-states"
	kungalPlaytimesURL       = kungalAPIBaseURL + "/me/playtimes"
	kungalOAuthClientIDEnv   = "LUNABOX_KUNGAL_CLIENT_ID"
	kungalOAuthScopes        = "openid profile catalog:read"
	kungalOAuthCallbackPath  = "/callback"
	kungalAuthTimeout        = 5 * time.Minute
	kungalTokenRefreshSkew   = 1 * time.Minute
	kungalHTTPTimeout        = 30 * time.Second
	kungalAuthStatusEvent    = "kungal:auth-status-changed"
	kungalStatusSyncEvent    = "kungal:status-sync-progress"
	kungalStatusPushFailed   = "kungal:status-push-failed"
	kungalBatchSize          = 100
	kungalMaxPlaytimeMinutes = 60000
)

var errKungalUnauthorized = errors.New("kungal unauthorized")

type kungalAuthSession struct {
	resultChan   chan kungalAuthResult
	server       *http.Server
	listener     net.Listener
	state        string
	nonce        string
	codeVerifier string
	redirectURI  string
}

type kungalAuthResult struct {
	Code  string
	Error string
}

type kungalTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

type kungalUserInfo struct {
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type kungalProblem struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Title     string `json:"title"`
	Detail    string `json:"detail"`
	RequestID string `json:"request_id"`
}

type kungalWorkState struct {
	WorkID     string  `json:"work_id"`
	State      string  `json:"state"`
	Completion *string `json:"completion,omitempty"`
}

type kungalWorkStateList struct {
	Items []kungalWorkState `json:"items"`
}

type kungalPlaytime struct {
	WorkID  string `json:"work_id"`
	Minutes int64  `json:"minutes"`
}

type kungalMultiStatusItem struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type kungalMultiStatus struct {
	Items []kungalMultiStatusItem `json:"items"`
}

type KungalService struct {
	ctx         context.Context
	db          *sql.DB
	config      *appconf.AppConfig
	httpClient  *http.Client
	runtime     appruntime.Runtime
	openURL     func(string) error
	emitEvent   func(string, ...interface{})
	now         func() time.Time
	clientID    string
	mu          sync.Mutex
	batchSyncMu sync.Mutex
}

func NewKungalService() *KungalService {
	runtime := appruntime.Unavailable()
	return &KungalService{
		runtime:   runtime,
		openURL:   runtime.OpenURL,
		emitEvent: func(name string, data ...interface{}) { runtime.Emit(name, data...) },
		now:       time.Now,
		clientID:  strings.TrimSpace(version.KungalOAuthClientID),
	}
}

//wails:ignore
func (s *KungalService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.clientID = firstNonEmptyString(s.clientID, strings.TrimSpace(version.KungalOAuthClientID))
	if s.httpClient == nil {
		client, _, err := httputils.NewClient(httputils.ClientOptions{
			Timeout:     kungalHTTPTimeout,
			ProxyConfig: config,
		})
		if err != nil {
			applog.LogWarningf(ctx, "failed to create Kungal HTTP client with proxy config: %v", err)
			client = &http.Client{Timeout: kungalHTTPTimeout}
		}
		s.httpClient = client
	}
	if s.now == nil {
		s.now = time.Now
	}
}

//wails:ignore
func (s *KungalService) SetRuntime(runtime appruntime.Runtime) {
	if runtime == nil {
		return
	}
	s.runtime = runtime
	s.openURL = runtime.OpenURL
	s.emitEvent = func(name string, data ...interface{}) {
		runtime.Emit(name, data...)
	}
}

//wails:ignore
func (s *KungalService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

//wails:ignore
func (s *KungalService) SetOpenURLFunc(openURL func(string) error) {
	if openURL != nil {
		s.openURL = openURL
	}
}

//wails:ignore
func (s *KungalService) SetOAuthClientID(clientID string) {
	s.clientID = strings.TrimSpace(clientID)
}

func (s *KungalService) GetAuthStatus() (vo.KungalAuthStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildAuthStatusLocked(), nil
}

func (s *KungalService) GetProfile() (vo.KungalProfile, error) {
	var user *kungalUserInfo
	err := s.withAccessToken(s.ctx, func(token string) error {
		var fetchErr error
		user, fetchErr = s.fetchUserInfo(s.ctx, token)
		return fetchErr
	})
	if err != nil {
		return vo.KungalProfile{}, err
	}

	avatarURL := s.resolveCachedAvatarURL(user, false)
	s.mu.Lock()
	if s.config != nil && (s.config.KungalAuthorizedAvatarURL != avatarURL || s.config.KungalAuthorizedUsername != user.Name) {
		s.config.KungalAuthorizedAvatarURL = avatarURL
		s.config.KungalAuthorizedUsername = user.Name
		appconf.SanitizeKungalOAuthConfig(s.config)
		if err := appconf.SaveConfig(s.config); err != nil {
			applog.LogWarningf(s.ctx, "failed to save cached Kungal profile: %v", err)
		}
	}
	s.mu.Unlock()

	return vo.KungalProfile{UserID: user.Sub, Username: user.Name, AvatarURL: avatarURL}, nil
}

func (s *KungalService) StartAuth() (vo.KungalAuthStatus, error) {
	return s.StartAuthContext(s.ctx)
}

// StartAuthContext 同 StartAuth，ctx 取消时停止等待授权回调并返回 ctx 的错误。
func (s *KungalService) StartAuthContext(ctx context.Context) (vo.KungalAuthStatus, error) {
	ctx = s.resolveContext(ctx)
	if strings.TrimSpace(s.clientID) == "" {
		return vo.KungalAuthStatus{}, fmt.Errorf("鲲 Galgame OAuth 未配置，请在构建时通过 %s 注入 public client ID", kungalOAuthClientIDEnv)
	}

	session, err := newKungalAuthSession()
	if err != nil {
		return vo.KungalAuthStatus{}, err
	}
	defer session.shutdown()

	// 必须用系统浏览器打开授权页，不能用内嵌 WebView。
	if err := s.openURL(buildKungalAuthURL(s.clientID, session)); err != nil {
		return vo.KungalAuthStatus{}, fmt.Errorf("打开鲲 Galgame 授权页面失败: %w", err)
	}

	timer := time.NewTimer(kungalAuthTimeout)
	defer timer.Stop()

	select {
	case result := <-session.resultChan:
		if result.Error != "" {
			return vo.KungalAuthStatus{}, fmt.Errorf("鲲 Galgame 授权失败: %s", result.Error)
		}
		tokenResp, err := s.exchangeAuthorizationCode(ctx, result.Code, session)
		if err != nil {
			return vo.KungalAuthStatus{}, err
		}
		user, err := s.fetchUserInfo(ctx, tokenResp.AccessToken)
		if err != nil {
			return vo.KungalAuthStatus{}, fmt.Errorf("获取鲲 Galgame 用户信息失败: %w", err)
		}

		s.mu.Lock()
		status, persistErr := s.persistAuthorizedStateLocked(tokenResp, user)
		s.mu.Unlock()
		if persistErr != nil {
			return vo.KungalAuthStatus{}, persistErr
		}

		s.emitAuthStatusChanged(status)
		applog.LogInfof(s.ctx, "Kungal OAuth authorized for user %s (%s)", user.Name, user.Sub)
		return status, nil
	case <-timer.C:
		return vo.KungalAuthStatus{}, fmt.Errorf("鲲 Galgame 授权超时")
	case <-ctx.Done():
		return vo.KungalAuthStatus{}, ctx.Err()
	}
}

func (s *KungalService) Disconnect() (vo.KungalAuthStatus, error) {
	s.mu.Lock()
	refreshToken := ""
	if s.config != nil {
		refreshToken = strings.TrimSpace(s.config.KungalRefreshToken)
	}
	status, err := s.clearAuthorizationLocked(true, "")
	s.mu.Unlock()
	if err != nil {
		return vo.KungalAuthStatus{}, err
	}

	if refreshToken != "" {
		go s.revokeToken(refreshToken)
	}
	s.emitAuthStatusChanged(status)
	applog.LogInfof(s.ctx, "Kungal OAuth disconnected locally")
	return status, nil
}

// SyncAllGameStatuses 把本地所有关联鲲 Galgame 的游戏状态与累计时长批量上传，每批 100 部。
func (s *KungalService) SyncAllGameStatuses() (vo.RemoteStatusSyncProgress, error) {
	s.batchSyncMu.Lock()
	defer s.batchSyncMu.Unlock()

	ctx := s.resolveContext(nil)
	emit := func(progress vo.RemoteStatusSyncProgress) {
		if s.ctx != nil && s.emitEvent != nil {
			s.emitEvent(kungalStatusSyncEvent, progress)
		}
	}

	games, err := remotestatus.LoadGames(ctx, s.db, enums.Kungal)
	progress := vo.RemoteStatusSyncProgress{
		Provider:        string(enums.Kungal),
		Status:          "started",
		Total:           len(games),
		FailedGameNames: make([]string, 0),
	}
	fail := func(err error) (vo.RemoteStatusSyncProgress, error) {
		progress.Status = "failed"
		progress.LastError = err.Error()
		emit(progress)
		return progress, err
	}
	if err != nil {
		return fail(err)
	}
	emit(progress)
	if len(games) == 0 {
		progress.Status = "done"
		emit(progress)
		return progress, nil
	}
	if _, err := s.getValidAccessToken(ctx); err != nil {
		return fail(err)
	}

	for start := 0; start < len(games); start += kungalBatchSize {
		end := min(start+kungalBatchSize, len(games))
		chunk := games[start:end]
		progress.Status = "running"
		progress.GameName = chunk[0].Name
		emit(progress)

		failures := s.syncGameChunk(ctx, chunk)
		for index, game := range chunk {
			if pushErr := failures[index]; pushErr != nil {
				progress.FailedGames++
				progress.FailedGameNames = append(progress.FailedGameNames, game.Name)
				progress.LastError = pushErr.Error()
				applog.LogWarningf(ctx, "failed to sync kungal status for game %s (%s): %v", game.Name, game.ID, pushErr)
			} else {
				progress.SucceededGames++
			}
		}
		progress.Current = end
		emit(progress)
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
	}

	progress.Status = "done"
	progress.GameName = ""
	emit(progress)
	return progress, nil
}

func (s *KungalService) syncGameChunk(ctx context.Context, games []models.Game) []error {
	failures := make([]error, len(games))
	workIDs := make([]string, 0, len(games))
	for _, game := range games {
		workIDs = append(workIDs, strings.TrimSpace(game.SourceID))
	}

	// 远端已是同一状态的跳过不写：既省请求，也不会把站点上手动填的完成度清掉。
	remoteStates, err := s.listWorkStates(ctx, workIDs)
	if err != nil {
		for index := range failures {
			failures[index] = err
		}
		return failures
	}

	stateItems := make([]kungalWorkState, 0, len(games))
	stateIndexes := make([]int, 0, len(games))
	for index, game := range games {
		state, ok := mapGameStatusToKungalState(game.Status)
		if !ok {
			failures[index] = fmt.Errorf("不支持同步的鲲 Galgame 状态: %s", game.Status)
			continue
		}
		if remote, exists := remoteStates[workIDs[index]]; exists && remote.State == state {
			continue
		}
		stateItems = append(stateItems, kungalWorkState{WorkID: workIDs[index], State: state})
		stateIndexes = append(stateIndexes, index)
	}
	if len(stateItems) > 0 {
		itemErrors, err := s.postBatch(ctx, kungalWorkStatesURL, stateItems, len(stateItems))
		for position, index := range stateIndexes {
			if err != nil {
				failures[index] = err
			} else {
				failures[index] = itemErrors[position]
			}
		}
	}

	minutesByGame, err := s.loadPlaytimeMinutes(ctx, games)
	if err != nil {
		applog.LogWarningf(ctx, "failed to load playtime for kungal sync: %v", err)
		return failures
	}
	playtimeItems := make([]kungalPlaytime, 0, len(games))
	playtimeIndexes := make([]int, 0, len(games))
	for index, game := range games {
		if failures[index] != nil {
			continue
		}
		if minutes := minutesByGame[game.ID]; minutes > 0 {
			playtimeItems = append(playtimeItems, kungalPlaytime{WorkID: workIDs[index], Minutes: minutes})
			playtimeIndexes = append(playtimeIndexes, index)
		}
	}
	if len(playtimeItems) > 0 {
		itemErrors, err := s.postBatch(ctx, kungalPlaytimesURL, playtimeItems, len(playtimeItems))
		for position, index := range playtimeIndexes {
			if err != nil {
				failures[index] = err
			} else {
				failures[index] = itemErrors[position]
			}
		}
	}
	return failures
}

func (s *KungalService) fetchMetadataByID(ctx context.Context, sourceID string) (metadata.MetadataResult, error) {
	getter := metadata.NewKungalInfoGetter(gamehelper.MetadataGetterOptionsContext(ctx, s.config)...)
	var result metadata.MetadataResult
	err := s.withOptionalAccessToken(ctx, func(token string) error {
		var fetchErr error
		result, fetchErr = getter.FetchMetadata(sourceID, token)
		return fetchErr
	})
	return result, err
}

func (s *KungalService) fetchMetadataByName(ctx context.Context, name string) (metadata.MetadataResult, error) {
	results, err := s.fetchMetadataCandidatesByName(ctx, name)
	if err != nil {
		return metadata.MetadataResult{}, err
	}
	return results[0], nil
}

func (s *KungalService) fetchMetadataCandidatesByName(ctx context.Context, name string) ([]metadata.MetadataResult, error) {
	getter := metadata.NewKungalInfoGetter(gamehelper.MetadataGetterOptionsContext(ctx, s.config)...)
	var results []metadata.MetadataResult
	err := s.withOptionalAccessToken(ctx, func(token string) error {
		var fetchErr error
		results, fetchErr = getter.FetchMetadataCandidatesByName(name, token)
		return fetchErr
	})
	return results, err
}

func (s *KungalService) isGameEligibleForStatusPush(game models.Game) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	authStatus := s.buildAuthStatusLocked()
	return authStatus.Authorized &&
		!authStatus.NeedsReauthorization &&
		appconf.IsKungalStatusPushEnabled(s.config) &&
		game.SourceType == enums.Kungal &&
		strings.TrimSpace(game.SourceID) != ""
}

func (s *KungalService) syncGameStatus(ctx context.Context, game models.Game) error {
	if !s.isGameEligibleForStatusPush(game) {
		return nil
	}
	state, ok := mapGameStatusToKungalState(game.Status)
	if !ok {
		return fmt.Errorf("不支持同步的鲲 Galgame 状态: %s", game.Status)
	}
	workID := strings.TrimSpace(game.SourceID)

	remoteStates, err := s.listWorkStates(ctx, []string{workID})
	if err != nil {
		return err
	}
	if remote, exists := remoteStates[workID]; exists && remote.State == state {
		return nil
	}
	return s.withAccessToken(ctx, func(token string) error {
		return s.sendJSON(ctx, http.MethodPut, kungalWorkStatesURL+"/"+url.PathEscape(workID), token, kungalWorkState{State: state}, nil)
	})
}

func (s *KungalService) syncGamePlaytime(ctx context.Context, game models.Game, minutes int64) error {
	if !s.isGameEligibleForStatusPush(game) || minutes <= 0 {
		return nil
	}
	workID := strings.TrimSpace(game.SourceID)
	payload := map[string]int64{"minutes": clampKungalPlaytimeMinutes(minutes)}
	return s.withAccessToken(ctx, func(token string) error {
		return s.sendJSON(ctx, http.MethodPut, kungalPlaytimesURL+"/"+url.PathEscape(workID), token, payload, nil)
	})
}

func (s *KungalService) listWorkStates(ctx context.Context, workIDs []string) (map[string]kungalWorkState, error) {
	query := url.Values{"work_ids": {strings.Join(workIDs, ",")}}
	var list kungalWorkStateList
	err := s.withAccessToken(ctx, func(token string) error {
		return s.sendJSON(ctx, http.MethodGet, kungalWorkStatesURL+"?"+query.Encode(), token, nil, &list)
	})
	if err != nil {
		return nil, err
	}
	states := make(map[string]kungalWorkState, len(list.Items))
	for _, item := range list.Items {
		states[strings.TrimSpace(item.WorkID)] = item
	}
	return states, nil
}

// postBatch 调用 207 Multi-Status 批量写接口；整体不是事务，按项读 status。
func (s *KungalService) postBatch(ctx context.Context, endpoint string, items any, count int) ([]error, error) {
	var response kungalMultiStatus
	err := s.withAccessToken(ctx, func(token string) error {
		return s.sendJSON(ctx, http.MethodPost, endpoint, token, map[string]any{"items": items}, &response)
	})
	if err != nil {
		return nil, err
	}
	itemErrors := make([]error, count)
	for index := range itemErrors {
		if index >= len(response.Items) {
			itemErrors[index] = fmt.Errorf("鲲 Galgame 批量响应缺少第 %d 项", index+1)
			continue
		}
		item := response.Items[index]
		if item.Status >= http.StatusBadRequest {
			itemErrors[index] = fmt.Errorf("HTTP %d %s", item.Status, firstNonEmptyString(strings.TrimSpace(item.Code+": "+item.Detail), item.Title))
		}
	}
	return itemErrors, nil
}

func (s *KungalService) loadPlaytimeMinutes(ctx context.Context, games []models.Game) (map[string]int64, error) {
	minutes := make(map[string]int64, len(games))
	if s.db == nil || len(games) == 0 {
		return minutes, nil
	}
	placeholders := make([]string, 0, len(games))
	args := make([]any, 0, len(games))
	for _, game := range games {
		placeholders = append(placeholders, "?")
		args = append(args, game.ID)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT game_id, COALESCE(SUM(CASE WHEN duration > 0 THEN duration ELSE 0 END), 0)
		FROM play_sessions
		WHERE game_id IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY game_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var gameID string
		var seconds int64
		if err := rows.Scan(&gameID, &seconds); err != nil {
			return nil, err
		}
		minutes[gameID] = clampKungalPlaytimeMinutes(seconds / 60)
	}
	return minutes, rows.Err()
}

// withAccessToken 带用户令牌执行请求，遇到 401 时刷新令牌并重试一次。
func (s *KungalService) withAccessToken(ctx context.Context, call func(token string) error) error {
	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return err
	}
	err = call(token)
	if !errors.Is(err, errKungalUnauthorized) && !metadata.IsKungalUnauthorizedError(err) {
		return err
	}
	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return refreshErr
	}
	return call(refreshedToken)
}

// withOptionalAccessToken 用于目录只读面：已登录时按用户计配额，未登录则匿名调用。
func (s *KungalService) withOptionalAccessToken(ctx context.Context, call func(token string) error) error {
	// 未登录或授权已失效时降级为匿名读取，不影响元数据刮削。
	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return call("")
	}
	err = call(token)
	if !metadata.IsKungalUnauthorizedError(err) {
		return err
	}
	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return call("")
	}
	return call(refreshedToken)
}

func (s *KungalService) getValidAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return "", fmt.Errorf("鲲 Galgame 配置未初始化")
	}
	accessToken := strings.TrimSpace(s.config.KungalAccessToken)
	if accessToken != "" && !s.shouldRefreshTokenLocked() {
		return accessToken, nil
	}
	if strings.TrimSpace(s.config.KungalRefreshToken) == "" {
		return "", fmt.Errorf("鲲 Galgame 未授权")
	}
	return s.refreshAccessTokenLocked(ctx)
}

func (s *KungalService) refreshAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshAccessTokenLocked(ctx)
}

func (s *KungalService) shouldRefreshTokenLocked() bool {
	if s.config == nil {
		return false
	}
	hasRefreshToken := strings.TrimSpace(s.config.KungalRefreshToken) != ""
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(s.config.KungalTokenExpiresAt))
	if err != nil {
		return hasRefreshToken
	}
	return !s.now().Add(kungalTokenRefreshSkew).Before(expiresAt)
}

// refreshAccessTokenLocked 刷新令牌；refresh token 每次都会轮换，必须原地覆盖保存。
func (s *KungalService) refreshAccessTokenLocked(ctx context.Context) (string, error) {
	if s.config == nil {
		return "", fmt.Errorf("鲲 Galgame 配置未初始化")
	}
	refreshToken := strings.TrimSpace(s.config.KungalRefreshToken)
	if refreshToken == "" {
		return "", fmt.Errorf("鲲 Galgame 未授权")
	}
	if strings.TrimSpace(s.clientID) == "" {
		return "", fmt.Errorf("鲲 Galgame OAuth 未配置，请在构建时通过 %s 注入 public client ID", kungalOAuthClientIDEnv)
	}

	tokenResp, statusCode, body, err := s.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {s.clientID},
		"refresh_token": {refreshToken},
	})
	if err != nil {
		return "", fmt.Errorf("刷新鲲 Galgame access token 失败: %w", err)
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest || tokenResp.Error != "" {
		message := firstNonEmptyString(tokenResp.ErrorDesc, tokenResp.Error, "鲲 Galgame 授权已失效，请重新授权")
		status, clearErr := s.clearAuthorizationLocked(false, message)
		if clearErr == nil {
			s.emitAuthStatusChanged(status)
		}
		return "", fmt.Errorf("鲲 Galgame refresh token 无效: %s", message)
	}
	if statusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("鲲 Galgame refresh 请求失败，HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" || strings.TrimSpace(tokenResp.RefreshToken) == "" {
		return "", fmt.Errorf("鲲 Galgame refresh 响应缺少轮换后的令牌")
	}

	s.config.KungalAccessToken = strings.TrimSpace(tokenResp.AccessToken)
	s.config.KungalRefreshToken = strings.TrimSpace(tokenResp.RefreshToken)
	s.config.KungalTokenExpiresAt = s.now().Add(tokenExpiryDuration(tokenResp.ExpiresIn)).Format(time.RFC3339)
	s.config.KungalAuthError = ""
	appconf.SanitizeKungalOAuthConfig(s.config)
	if err := appconf.SaveConfig(s.config); err != nil {
		return "", fmt.Errorf("保存鲲 Galgame 刷新后配置失败: %w", err)
	}

	s.emitAuthStatusChanged(s.buildAuthStatusLocked())
	return s.config.KungalAccessToken, nil
}

func (s *KungalService) buildAuthStatusLocked() vo.KungalAuthStatus {
	if s.config == nil {
		return vo.KungalAuthStatus{}
	}
	authError := strings.TrimSpace(s.config.KungalAuthError)
	return vo.KungalAuthStatus{
		Authorized:           strings.TrimSpace(s.config.KungalAccessToken) != "" || strings.TrimSpace(s.config.KungalRefreshToken) != "",
		NeedsReauthorization: authError != "",
		UserID:               strings.TrimSpace(s.config.KungalAuthorizedUserID),
		Username:             strings.TrimSpace(s.config.KungalAuthorizedUsername),
		AvatarURL:            strings.TrimSpace(s.config.KungalAuthorizedAvatarURL),
		AccessTokenExpiresAt: strings.TrimSpace(s.config.KungalTokenExpiresAt),
		LastError:            authError,
	}
}

func (s *KungalService) persistAuthorizedStateLocked(tokenResp *kungalTokenResponse, user *kungalUserInfo) (vo.KungalAuthStatus, error) {
	if s.config == nil {
		return vo.KungalAuthStatus{}, fmt.Errorf("鲲 Galgame 配置未初始化")
	}
	previousUserID := strings.TrimSpace(s.config.KungalAuthorizedUserID)
	s.config.KungalAccessToken = strings.TrimSpace(tokenResp.AccessToken)
	s.config.KungalRefreshToken = strings.TrimSpace(tokenResp.RefreshToken)
	s.config.KungalTokenExpiresAt = s.now().Add(tokenExpiryDuration(tokenResp.ExpiresIn)).Format(time.RFC3339)
	s.config.KungalAuthorizedUserID = strings.TrimSpace(user.Sub)
	s.config.KungalAuthorizedUsername = strings.TrimSpace(user.Name)
	s.config.KungalAuthorizedAvatarURL = s.resolveCachedAvatarURL(user, true)
	s.config.KungalAuthError = ""
	appconf.SanitizeKungalOAuthConfig(s.config)

	if previousUserID != "" && previousUserID != s.config.KungalAuthorizedUserID {
		_ = imageutils.RemoveManagedAvatar("kungal", previousUserID)
	}
	if err := appconf.SaveConfig(s.config); err != nil {
		return vo.KungalAuthStatus{}, fmt.Errorf("保存鲲 Galgame 授权配置失败: %w", err)
	}
	return s.buildAuthStatusLocked(), nil
}

func (s *KungalService) clearAuthorizationLocked(clearIdentity bool, reason string) (vo.KungalAuthStatus, error) {
	if s.config == nil {
		return vo.KungalAuthStatus{}, fmt.Errorf("鲲 Galgame 配置未初始化")
	}
	previousUserID := strings.TrimSpace(s.config.KungalAuthorizedUserID)
	s.config.KungalAccessToken = ""
	s.config.KungalRefreshToken = ""
	s.config.KungalTokenExpiresAt = ""
	s.config.KungalAuthError = strings.TrimSpace(reason)
	if clearIdentity {
		s.config.KungalAuthorizedUserID = ""
		s.config.KungalAuthorizedUsername = ""
		s.config.KungalAuthorizedAvatarURL = ""
	}
	appconf.SanitizeKungalOAuthConfig(s.config)
	if clearIdentity && previousUserID != "" {
		_ = imageutils.RemoveManagedAvatar("kungal", previousUserID)
	}
	if err := appconf.SaveConfig(s.config); err != nil {
		return vo.KungalAuthStatus{}, fmt.Errorf("保存鲲 Galgame 配置失败: %w", err)
	}
	return s.buildAuthStatusLocked(), nil
}

func (s *KungalService) exchangeAuthorizationCode(ctx context.Context, code string, session *kungalAuthSession) (*kungalTokenResponse, error) {
	tokenResp, statusCode, body, err := s.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {s.clientID},
		"code":          {code},
		"redirect_uri":  {session.redirectURI},
		"code_verifier": {session.codeVerifier},
	})
	if err != nil {
		return nil, fmt.Errorf("鲲 Galgame token 交换失败: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("鲲 Galgame OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	if statusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("鲲 Galgame token 交换失败，HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" || strings.TrimSpace(tokenResp.RefreshToken) == "" {
		return nil, fmt.Errorf("鲲 Galgame token 响应缺少必要字段")
	}
	if strings.TrimSpace(tokenResp.IDToken) != "" {
		if err := validateKungalIDTokenNonce(tokenResp.IDToken, session.nonce); err != nil {
			return nil, err
		}
	}
	return tokenResp, nil
}

func (s *KungalService) requestToken(ctx context.Context, form url.Values) (*kungalTokenResponse, int, []byte, error) {
	req, err := http.NewRequestWithContext(s.resolveContext(ctx), http.MethodPost, kungalOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := s.doRequest(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, nil, err
	}
	var tokenResp kungalTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, resp.StatusCode, body, fmt.Errorf("解析令牌响应失败: %w", err)
	}
	return &tokenResp, resp.StatusCode, body, nil
}

func (s *KungalService) revokeToken(refreshToken string) {
	if strings.TrimSpace(s.clientID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	form := url.Values{
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
		"client_id":       {s.clientID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kungalOAuthRevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := s.httpClient.Do(req)
	if err != nil {
		applog.LogWarningf(s.ctx, "failed to revoke Kungal refresh token: %v", err)
		return
	}
	_ = resp.Body.Close()
}

func (s *KungalService) fetchUserInfo(ctx context.Context, accessToken string) (*kungalUserInfo, error) {
	var user kungalUserInfo
	if err := s.sendJSON(ctx, http.MethodGet, kungalOAuthUserInfoURL, accessToken, nil, &user); err != nil {
		return nil, err
	}
	user.Sub = strings.TrimSpace(user.Sub)
	user.Name = strings.TrimSpace(user.Name)
	if user.Sub == "" {
		return nil, fmt.Errorf("鲲 Galgame 用户信息缺少 sub")
	}
	if user.Name == "" {
		user.Name = user.Sub
	}
	return &user, nil
}

// resolveCachedAvatarURL 返回本地缓存的头像地址；force 为 true（刚授权）或缓存超过一天时重新下载。
func (s *KungalService) resolveCachedAvatarURL(user *kungalUserInfo, force bool) string {
	if user == nil || user.Sub == "" {
		return ""
	}
	sourceURL := strings.TrimSpace(user.Picture)
	localURL, err := imageutils.ResolveManagedAvatar(s.httpClient, sourceURL, "kungal", user.Sub, force)
	if err != nil {
		applog.LogWarningf(s.ctx, "failed to cache Kungal avatar for user %s: %v", user.Sub, err)
		if localURL == "" {
			return sourceURL
		}
	}
	return localURL
}

// sendJSON 发送带用户令牌的 JSON 请求；401 映射为 errKungalUnauthorized 以便调用方刷新重试。
func (s *KungalService) sendJSON(ctx context.Context, method, endpoint, accessToken string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("编码鲲 Galgame 请求失败: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(s.resolveContext(ctx), method, endpoint, body)
	if err != nil {
		return fmt.Errorf("创建鲲 Galgame 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.doRequest(req)
	if err != nil {
		return fmt.Errorf("请求鲲 Galgame 接口失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取鲲 Galgame 响应失败: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: %s", errKungalUnauthorized, kungalProblemMessage(respBody))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("鲲 Galgame 接口返回 HTTP %d: %s", resp.StatusCode, kungalProblemMessage(respBody))
	}
	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("解析鲲 Galgame 响应失败: %w", err)
		}
	}
	return nil
}

func (s *KungalService) doRequest(req *http.Request) (*http.Response, error) {
	return httputils.DoWithRetry(req.Context(), s.httpClient, req, httputils.RetryPolicy{
		MaxRetries:    1,
		FallbackDelay: time.Second,
		MaxDelay:      30 * time.Second,
	})
}

func (s *KungalService) emitAuthStatusChanged(status vo.KungalAuthStatus) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(kungalAuthStatusEvent, status)
}

func (s *KungalService) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func mapGameStatusToKungalState(status enums.GameStatus) (string, bool) {
	switch status {
	case enums.StatusNotStarted, enums.StatusWantToPlay:
		return "wish", true
	case enums.StatusPlaying:
		return "doing", true
	case enums.StatusCompleted:
		return "done", true
	case enums.StatusOnHold:
		return "on_hold", true
	case enums.StatusDropped:
		return "dropped", true
	default:
		return "", false
	}
}

func clampKungalPlaytimeMinutes(minutes int64) int64 {
	return max(0, min(minutes, kungalMaxPlaytimeMinutes))
}

func kungalProblemMessage(body []byte) string {
	var problem kungalProblem
	if json.Unmarshal(body, &problem) != nil || (problem.Code == "" && problem.Detail == "" && problem.Title == "") {
		return strings.TrimSpace(string(body))
	}
	message := firstNonEmptyString(strings.TrimSpace(problem.Detail), strings.TrimSpace(problem.Title))
	if problem.Code != "" {
		message = strings.TrimSpace(problem.Code + ": " + message)
	}
	if problem.RequestID != "" {
		message += " (request_id: " + problem.RequestID + ")"
	}
	return message
}

func buildKungalAuthURL(clientID string, session *kungalAuthSession) string {
	challenge := sha256.Sum256([]byte(session.codeVerifier))
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {session.redirectURI},
		"scope":                 {kungalOAuthScopes},
		"state":                 {session.state},
		"nonce":                 {session.nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	return kungalOAuthAuthorizeURL + "?" + params.Encode()
}

func newKungalAuthSession() (*kungalAuthSession, error) {
	// 端口 0：由内核挑选临时端口，注册的环回回调按 RFC 8252 §7.3 忽略端口。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("无法启动鲲 Galgame 本地回调服务: %w", err)
	}
	values := make([]string, 3)
	for index := range values {
		value, err := generateKungalRandomValue(32)
		if err != nil {
			_ = listener.Close()
			return nil, fmt.Errorf("生成鲲 Galgame OAuth 随机值失败: %w", err)
		}
		values[index] = value
	}

	session := &kungalAuthSession{
		resultChan:   make(chan kungalAuthResult, 1),
		listener:     listener,
		state:        values[0],
		nonce:        values[1],
		codeVerifier: values[2],
		redirectURI:  fmt.Sprintf("http://127.0.0.1:%d%s", listener.Addr().(*net.TCPAddr).Port, kungalOAuthCallbackPath),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(kungalOAuthCallbackPath, session.handleOAuthCallback)
	session.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := session.server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			session.trySendResult(kungalAuthResult{Error: serveErr.Error()})
		}
	}()
	return session, nil
}

func generateKungalRandomValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validateKungalIDTokenNonce(idToken, expectedNonce string) error {
	parts := strings.Split(strings.TrimSpace(idToken), ".")
	if len(parts) != 3 {
		return fmt.Errorf("鲲 Galgame ID token 格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("解析鲲 Galgame ID token 失败: %w", err)
	}
	var claims struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("解析鲲 Galgame ID token 声明失败: %w", err)
	}
	if claims.Nonce != "" && subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		return fmt.Errorf("鲲 Galgame ID token nonce 校验失败")
	}
	return nil
}

func (s *kungalAuthSession) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	_ = s.listener.Close()
}

func (s *kungalAuthSession) trySendResult(result kungalAuthResult) {
	select {
	case s.resultChan <- result:
	default:
	}
}

func (s *kungalAuthSession) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePage := func(status int, title, message string) {
		w.WriteHeader(status)
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>%s</title></head><body><h1>%s</h1><p>%s</p><p>您可以关闭此窗口并返回应用。</p></body></html>`,
			html.EscapeString(title), html.EscapeString(title), html.EscapeString(message))
	}
	if r.Method != http.MethodGet {
		writePage(http.StatusMethodNotAllowed, "授权失败", "请求方法无效")
		return
	}
	if !isLoopbackRequest(r.RemoteAddr) {
		writePage(http.StatusForbidden, "授权失败", "回调来源无效")
		return
	}
	query := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(s.state)) != 1 {
		s.trySendResult(kungalAuthResult{Error: "授权状态校验失败"})
		writePage(http.StatusBadRequest, "授权失败", "授权状态校验失败")
		return
	}
	if issuer := strings.TrimSpace(query.Get("iss")); issuer != "" && issuer != kungalOAuthIssuer {
		s.trySendResult(kungalAuthResult{Error: "授权签发方校验失败"})
		writePage(http.StatusBadRequest, "授权失败", "授权签发方校验失败")
		return
	}
	if oauthError := strings.TrimSpace(query.Get("error")); oauthError != "" {
		message := strings.TrimSpace(oauthError + ": " + strings.TrimSpace(query.Get("error_description")))
		s.trySendResult(kungalAuthResult{Error: message})
		writePage(http.StatusOK, "授权失败", message)
		return
	}
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		s.trySendResult(kungalAuthResult{Error: "未收到授权码"})
		writePage(http.StatusBadRequest, "授权失败", "未收到授权码")
		return
	}
	s.trySendResult(kungalAuthResult{Code: code})
	writePage(http.StatusOK, "授权成功", "鲲 Galgame 账号已连接")
}
