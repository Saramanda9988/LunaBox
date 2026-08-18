package service

import (
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
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/gamehelper"
	"lunabox/internal/utils/httputils"
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/utils/metadata"
	"lunabox/internal/version"
	"lunabox/internal/wailsruntime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hikarinagiOAuthIssuer       = "https://id.hikarinagi.org/oidc"
	hikarinagiOAuthAuthorizeURL = hikarinagiOAuthIssuer + "/auth"
	hikarinagiOAuthTokenURL     = hikarinagiOAuthIssuer + "/token"
	hikarinagiCurrentUserURL    = "https://api.hikarinagi.org/v3/user/me"
	hikarinagiGameRateURLFormat = "https://api.hikarinagi.org/v3/user/me/rates/galgames/%s"
	hikarinagiSiteURL           = "https://www.hikarinagi.org/"
	hikarinagiImageBaseURL      = "https://imagesp.yurari.moe/"

	hikarinagiOAuthClientIDEnv = "LUNABOX_HIKARINAGI_CLIENT_ID"
	hikarinagiOAuthScopes      = "openid catalog:full user:read status:write offline_access"

	hikarinagiOAuthCallbackPort = 14791
	hikarinagiOAuthCallbackPath = "/callback"
	hikarinagiOAuthRedirectURI  = "http://127.0.0.1:14791/callback"

	hikarinagiAuthTimeout       = 5 * time.Minute
	hikarinagiTokenRefreshSkew  = 1 * time.Minute
	hikarinagiHTTPTimeout       = 30 * time.Second
	hikarinagiMetadataEventName = "hikarinagi:auth-status-changed"
	hikarinagiStatusSyncEvent   = "hikarinagi:status-sync-progress"
)

var errHikarinagiUnauthorized = errors.New("hikarinagi unauthorized")

type hikarinagiAuthSession struct {
	resultChan   chan hikarinagiAuthResult
	server       *http.Server
	listener     net.Listener
	state        string
	nonce        string
	codeVerifier string
	redirectURI  string
}

type hikarinagiAuthResult struct {
	Code  string
	Error string
}

type hikarinagiTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

type hikarinagiTokenClaims struct {
	Nonce string `json:"nonce"`
}

type hikarinagiAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type hikarinagiAPIEnvelope[T any] struct {
	Success bool                `json:"success"`
	Data    T                   `json:"data"`
	Error   *hikarinagiAPIError `json:"error"`
}

type hikarinagiMediaAsset struct {
	Src string `json:"src"`
}

type hikarinagiCurrentUser struct {
	ID       int64                 `json:"id"`
	Name     string                `json:"name"`
	Nickname *string               `json:"nickname"`
	Avatar   *hikarinagiMediaAsset `json:"avatar"`
}

type hikarinagiReviewPayload struct {
	Rate                *int   `json:"rate"`
	RateContent         string `json:"rate_content"`
	IsSpoiler           bool   `json:"is_spoiler"`
	TimeToFinishMinutes int64  `json:"time_to_finish_minutes"`
}

type HikarinagiService struct {
	ctx         context.Context
	db          *sql.DB
	config      *appconf.AppConfig
	httpClient  *http.Client
	runtime     wailsruntime.Runtime
	openURL     func(string) error
	emitEvent   func(string, ...interface{})
	now         func() time.Time
	clientID    string
	mu          sync.Mutex
	batchSyncMu sync.Mutex
}

func NewHikarinagiService() *HikarinagiService {
	runtime := wailsruntime.Unavailable()
	return &HikarinagiService{
		runtime:   runtime,
		openURL:   runtime.OpenURL,
		emitEvent: func(name string, data ...interface{}) { runtime.Emit(name, data...) },
		now:       time.Now,
		clientID:  strings.TrimSpace(version.HikarinagiOAuthClientID),
	}
}

//wails:ignore
func (s *HikarinagiService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.clientID = firstNonEmptyString(s.clientID, version.HikarinagiOAuthClientID)
	if s.httpClient == nil {
		client, _, err := httputils.NewClient(httputils.ClientOptions{
			Timeout:     hikarinagiHTTPTimeout,
			ProxyConfig: config,
		})
		if err != nil {
			applog.LogWarningf(ctx, "failed to create Hikarinagi HTTP client with proxy config: %v", err)
			client = &http.Client{Timeout: hikarinagiHTTPTimeout}
		}
		s.httpClient = client
	}
	if s.now == nil {
		s.now = time.Now
	}
}

//wails:ignore
func (s *HikarinagiService) SetRuntime(runtime wailsruntime.Runtime) {
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
func (s *HikarinagiService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

//wails:ignore
func (s *HikarinagiService) SetOpenURLFunc(openURL func(string) error) {
	if openURL != nil {
		s.openURL = openURL
	}
}

//wails:ignore
func (s *HikarinagiService) SetNowFunc(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

//wails:ignore
func (s *HikarinagiService) SetOAuthClientID(clientID string) {
	s.clientID = strings.TrimSpace(clientID)
}

//wails:ignore
func (s *HikarinagiService) SetEventEmitter(emit func(string, ...interface{})) {
	s.emitEvent = emit
}

func (s *HikarinagiService) GetAuthStatus() (vo.HikarinagiAuthStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildAuthStatusLocked(), nil
}

func (s *HikarinagiService) GetProfile() (vo.HikarinagiProfile, error) {
	token, err := s.getValidAccessToken(s.ctx)
	if err != nil {
		return vo.HikarinagiProfile{}, err
	}

	user, err := s.fetchCurrentUser(s.ctx, token)
	if err == nil {
		return s.buildProfileAndCache(user), nil
	}
	if !errors.Is(err, errHikarinagiUnauthorized) {
		return vo.HikarinagiProfile{}, err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(s.ctx)
	if refreshErr != nil {
		return vo.HikarinagiProfile{}, refreshErr
	}
	user, err = s.fetchCurrentUser(s.ctx, refreshedToken)
	if err != nil {
		return vo.HikarinagiProfile{}, err
	}
	return s.buildProfileAndCache(user), nil
}

func (s *HikarinagiService) StartAuth() (vo.HikarinagiAuthStatus, error) {
	if strings.TrimSpace(s.clientID) == "" {
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("Hikarinagi OAuth 未配置，请在构建时通过 %s 注入 public client ID", hikarinagiOAuthClientIDEnv)
	}

	session, err := newHikarinagiAuthSession()
	if err != nil {
		return vo.HikarinagiAuthStatus{}, err
	}
	defer session.shutdown()

	authURL := buildHikarinagiAuthURL(s.clientID, session)
	if err := s.openURL(authURL); err != nil {
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("打开 Hikarinagi 授权页面失败: %w", err)
	}

	timer := time.NewTimer(hikarinagiAuthTimeout)
	defer timer.Stop()

	select {
	case result := <-session.resultChan:
		if result.Error != "" {
			return vo.HikarinagiAuthStatus{}, fmt.Errorf("Hikarinagi 授权失败: %s", result.Error)
		}

		tokenResp, err := s.exchangeAuthorizationCode(
			s.ctx,
			result.Code,
			session.redirectURI,
			session.codeVerifier,
			session.nonce,
		)
		if err != nil {
			return vo.HikarinagiAuthStatus{}, err
		}
		user, err := s.fetchCurrentUser(s.ctx, tokenResp.AccessToken)
		if err != nil {
			return vo.HikarinagiAuthStatus{}, fmt.Errorf("获取 Hikarinagi 当前用户信息失败: %w", err)
		}

		s.mu.Lock()
		status, persistErr := s.persistAuthorizedStateLocked(tokenResp, user)
		s.mu.Unlock()
		if persistErr != nil {
			return vo.HikarinagiAuthStatus{}, persistErr
		}

		s.emitAuthStatusChanged(status)
		applog.LogInfof(s.ctx, "Hikarinagi OAuth authorized for user %s (%d)", user.Name, user.ID)
		return status, nil
	case <-timer.C:
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("Hikarinagi 授权超时")
	case <-s.resolveContext(s.ctx).Done():
		return vo.HikarinagiAuthStatus{}, s.resolveContext(s.ctx).Err()
	}
}

func (s *HikarinagiService) Disconnect() (vo.HikarinagiAuthStatus, error) {
	s.mu.Lock()
	status, err := s.clearAuthorizationLocked(true, "")
	s.mu.Unlock()
	if err != nil {
		return vo.HikarinagiAuthStatus{}, err
	}

	s.emitAuthStatusChanged(status)
	applog.LogInfof(s.ctx, "Hikarinagi OAuth disconnected locally")
	return status, nil
}

func (s *HikarinagiService) getValidAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return "", fmt.Errorf("Hikarinagi 配置未初始化")
	}

	accessToken := strings.TrimSpace(s.config.HikarinagiAccessToken)
	refreshToken := strings.TrimSpace(s.config.HikarinagiRefreshToken)
	if accessToken != "" && !s.shouldRefreshTokenLocked() {
		return accessToken, nil
	}
	if refreshToken == "" {
		return "", fmt.Errorf("Hikarinagi 未授权")
	}

	return s.refreshAccessTokenLocked(ctx)
}

func (s *HikarinagiService) refreshAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshAccessTokenLocked(ctx)
}

func (s *HikarinagiService) fetchMetadataByID(ctx context.Context, sourceID string) (metadata.MetadataResult, error) {
	getter := metadata.NewHikarinagiInfoGetter(gamehelper.MetadataGetterOptions(s.config)...)
	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return getter.FetchMetadata(sourceID, "")
	}

	result, err := getter.FetchMetadata(sourceID, token)
	if err == nil || !metadata.IsHikarinagiUnauthorizedError(err) {
		return result, err
	}
	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return metadata.MetadataResult{}, refreshErr
	}
	return getter.FetchMetadata(sourceID, refreshedToken)
}

func (s *HikarinagiService) fetchMetadataByName(ctx context.Context, name string) (metadata.MetadataResult, error) {
	results, err := s.fetchMetadataCandidatesByName(ctx, name)
	if err != nil {
		return metadata.MetadataResult{}, err
	}
	return results[0], nil
}

func (s *HikarinagiService) fetchMetadataCandidatesByName(ctx context.Context, name string) ([]metadata.MetadataResult, error) {
	getter := metadata.NewHikarinagiInfoGetter(gamehelper.MetadataGetterOptions(s.config)...)
	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return getter.FetchMetadataCandidatesByName(name, "")
	}

	result, err := getter.FetchMetadataCandidatesByName(name, token)
	if err == nil || !metadata.IsHikarinagiUnauthorizedError(err) {
		return result, err
	}
	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return nil, refreshErr
	}
	return getter.FetchMetadataCandidatesByName(name, refreshedToken)
}

func (s *HikarinagiService) syncGameStatus(ctx context.Context, game models.Game) error {
	if !s.isGameEligibleForStatusPush(game) {
		return nil
	}
	return s.upsertGameStatus(ctx, strings.TrimSpace(game.SourceID), game.Status)
}

func (s *HikarinagiService) SyncAllGameStatuses() (vo.RemoteStatusSyncProgress, error) {
	s.batchSyncMu.Lock()
	defer s.batchSyncMu.Unlock()

	ctx := s.resolveContext(nil)
	games, err := loadGamesForRemoteStatusSync(ctx, s.db, enums.Hikarinagi)
	progress := vo.RemoteStatusSyncProgress{
		Provider:        string(enums.Hikarinagi),
		Status:          "started",
		Total:           len(games),
		FailedGameNames: make([]string, 0),
	}
	if err != nil {
		return s.failStatusSync(progress, err)
	}
	s.emitStatusSyncProgress(progress)

	if len(games) == 0 {
		progress.Status = "done"
		s.emitStatusSyncProgress(progress)
		return progress, nil
	}
	if _, err := s.getValidAccessToken(ctx); err != nil {
		return s.failStatusSync(progress, err)
	}

	for index, game := range games {
		progress.Status = "running"
		progress.GameName = game.Name
		s.emitStatusSyncProgress(progress)

		if err := s.upsertGameStatus(ctx, strings.TrimSpace(game.SourceID), game.Status); err != nil {
			progress.FailedGames++
			progress.FailedGameNames = append(progress.FailedGameNames, game.Name)
			progress.LastError = err.Error()
			applog.LogWarningf(s.ctx, "failed to sync Hikarinagi status for game %s (%s): %v", game.Name, game.ID, err)
		} else {
			progress.SucceededGames++
		}
		progress.Current = index + 1
		s.emitStatusSyncProgress(progress)

		if index+1 < len(games) {
			if err := waitForRemoteStatusSync(ctx); err != nil {
				return s.failStatusSync(progress, err)
			}
		}
	}

	progress.Status = "done"
	progress.GameName = ""
	s.emitStatusSyncProgress(progress)
	return progress, nil
}

func (s *HikarinagiService) upsertGameStatus(ctx context.Context, workID string, status enums.GameStatus) error {
	remoteStatus, ok := mapGameStatusToHikarinagiStatus(status)
	if !ok {
		return fmt.Errorf("不支持同步的 Hikarinagi 状态: %s", status)
	}

	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return err
	}
	err = s.putGameStatus(ctx, workID, token, remoteStatus)
	if !errors.Is(err, errHikarinagiUnauthorized) {
		return err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return refreshErr
	}
	return s.putGameStatus(ctx, workID, refreshedToken, remoteStatus)
}

func (s *HikarinagiService) syncGameReview(ctx context.Context, workID string, review models.GameReview, timeToFinishMinutes int64) error {
	payload := hikarinagiReviewPayload{
		Rate:                review.Rating,
		RateContent:         review.Content,
		IsSpoiler:           review.IsSpoiler,
		TimeToFinishMinutes: timeToFinishMinutes,
	}

	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return err
	}
	err = s.putGameRate(ctx, workID, token, payload, "评价")
	if !errors.Is(err, errHikarinagiUnauthorized) {
		return err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return refreshErr
	}
	return s.putGameRate(ctx, workID, refreshedToken, payload, "评价")
}

func (s *HikarinagiService) isGameEligibleForStatusPush(game models.Game) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	authStatus := s.buildAuthStatusLocked()
	return authStatus.Authorized &&
		!authStatus.NeedsReauthorization &&
		appconf.IsHikarinagiStatusPushEnabled(s.config) &&
		game.SourceType == enums.Hikarinagi &&
		strings.TrimSpace(game.SourceID) != ""
}

func (s *HikarinagiService) shouldRefreshTokenLocked() bool {
	if s.config == nil {
		return false
	}
	expiresAtRaw := strings.TrimSpace(s.config.HikarinagiTokenExpiresAt)
	if expiresAtRaw == "" {
		return strings.TrimSpace(s.config.HikarinagiRefreshToken) != ""
	}
	expiresAt, err := time.Parse(time.RFC3339, expiresAtRaw)
	if err != nil {
		return strings.TrimSpace(s.config.HikarinagiRefreshToken) != ""
	}
	return !s.now().Add(hikarinagiTokenRefreshSkew).Before(expiresAt)
}

func (s *HikarinagiService) refreshAccessTokenLocked(ctx context.Context) (string, error) {
	if s.config == nil {
		return "", fmt.Errorf("Hikarinagi 配置未初始化")
	}
	refreshToken := strings.TrimSpace(s.config.HikarinagiRefreshToken)
	if refreshToken == "" {
		return "", fmt.Errorf("Hikarinagi 未授权")
	}
	if strings.TrimSpace(s.clientID) == "" {
		return "", fmt.Errorf("Hikarinagi OAuth 未配置，请在构建时通过 %s 注入 public client ID", hikarinagiOAuthClientIDEnv)
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {s.clientID},
		"refresh_token": {refreshToken},
	}
	tokenResp, statusCode, body, err := s.requestToken(ctx, form)
	if err != nil {
		return "", fmt.Errorf("刷新 Hikarinagi access token 失败: %w", err)
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest || tokenResp.Error != "" {
		message := firstNonEmptyString(tokenResp.ErrorDesc, tokenResp.Error, "Hikarinagi 授权已失效，请重新授权")
		status, clearErr := s.clearAuthorizationLocked(false, message)
		if clearErr == nil {
			s.emitAuthStatusChanged(status)
		}
		return "", fmt.Errorf("Hikarinagi refresh token 无效: %s", message)
	}
	if statusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("Hikarinagi refresh 请求失败，HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" || strings.TrimSpace(tokenResp.RefreshToken) == "" {
		return "", fmt.Errorf("Hikarinagi refresh 响应缺少轮换后的令牌")
	}

	expiresAt := s.now().Add(tokenExpiryDuration(tokenResp.ExpiresIn))
	s.config.HikarinagiAccessToken = strings.TrimSpace(tokenResp.AccessToken)
	s.config.HikarinagiRefreshToken = strings.TrimSpace(tokenResp.RefreshToken)
	s.config.HikarinagiTokenExpiresAt = expiresAt.Format(time.RFC3339)
	s.config.HikarinagiAuthError = ""
	appconf.SanitizeHikarinagiOAuthConfig(s.config)
	if err := appconf.SaveConfig(s.config); err != nil {
		return "", fmt.Errorf("保存 Hikarinagi 刷新后配置失败: %w", err)
	}

	status := s.buildAuthStatusLocked()
	s.emitAuthStatusChanged(status)
	applog.LogInfof(s.ctx, "Hikarinagi access token refreshed successfully")
	return s.config.HikarinagiAccessToken, nil
}

func (s *HikarinagiService) buildAuthStatusLocked() vo.HikarinagiAuthStatus {
	if s.config == nil {
		return vo.HikarinagiAuthStatus{}
	}
	accessToken := strings.TrimSpace(s.config.HikarinagiAccessToken)
	refreshToken := strings.TrimSpace(s.config.HikarinagiRefreshToken)
	authError := strings.TrimSpace(s.config.HikarinagiAuthError)
	return vo.HikarinagiAuthStatus{
		Authorized:           accessToken != "" || refreshToken != "",
		NeedsReauthorization: authError != "",
		UserID:               strings.TrimSpace(s.config.HikarinagiAuthorizedUserID),
		Username:             strings.TrimSpace(s.config.HikarinagiAuthorizedUsername),
		AvatarURL:            strings.TrimSpace(s.config.HikarinagiAuthorizedAvatarURL),
		AccessTokenExpiresAt: strings.TrimSpace(s.config.HikarinagiTokenExpiresAt),
		LastError:            authError,
	}
}

func (s *HikarinagiService) persistAuthorizedStateLocked(tokenResp *hikarinagiTokenResponse, user *hikarinagiCurrentUser) (vo.HikarinagiAuthStatus, error) {
	if s.config == nil {
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("Hikarinagi 配置未初始化")
	}
	previousUserID := strings.TrimSpace(s.config.HikarinagiAuthorizedUserID)
	s.config.HikarinagiAccessToken = strings.TrimSpace(tokenResp.AccessToken)
	s.config.HikarinagiRefreshToken = strings.TrimSpace(tokenResp.RefreshToken)
	s.config.HikarinagiTokenExpiresAt = s.now().Add(tokenExpiryDuration(tokenResp.ExpiresIn)).Format(time.RFC3339)
	s.config.HikarinagiAuthorizedUserID = strconv.FormatInt(user.ID, 10)
	s.config.HikarinagiAuthorizedUsername = strings.TrimSpace(user.Name)
	s.config.HikarinagiAuthorizedAvatarURL = s.resolveCachedAvatarURL(user)
	s.config.HikarinagiAuthError = ""
	appconf.SanitizeHikarinagiOAuthConfig(s.config)

	if previousUserID != "" && previousUserID != s.config.HikarinagiAuthorizedUserID {
		_ = imageutils.RemoveManagedAvatar("hikarinagi", previousUserID)
	}
	if err := appconf.SaveConfig(s.config); err != nil {
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("保存 Hikarinagi 授权配置失败: %w", err)
	}
	return s.buildAuthStatusLocked(), nil
}

func (s *HikarinagiService) clearAuthorizationLocked(clearIdentity bool, reason string) (vo.HikarinagiAuthStatus, error) {
	if s.config == nil {
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("Hikarinagi 配置未初始化")
	}
	previousUserID := strings.TrimSpace(s.config.HikarinagiAuthorizedUserID)
	s.config.HikarinagiAccessToken = ""
	s.config.HikarinagiRefreshToken = ""
	s.config.HikarinagiTokenExpiresAt = ""
	s.config.HikarinagiAuthError = strings.TrimSpace(reason)
	if clearIdentity {
		s.config.HikarinagiAuthorizedUserID = ""
		s.config.HikarinagiAuthorizedUsername = ""
		s.config.HikarinagiAuthorizedAvatarURL = ""
	}
	appconf.SanitizeHikarinagiOAuthConfig(s.config)
	if clearIdentity && previousUserID != "" {
		_ = imageutils.RemoveManagedAvatar("hikarinagi", previousUserID)
	}
	if err := appconf.SaveConfig(s.config); err != nil {
		return vo.HikarinagiAuthStatus{}, fmt.Errorf("保存 Hikarinagi 配置失败: %w", err)
	}
	return s.buildAuthStatusLocked(), nil
}

func (s *HikarinagiService) exchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier, nonce string) (*hikarinagiTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {s.clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}
	tokenResp, statusCode, body, err := s.requestToken(ctx, form)
	if err != nil {
		return nil, fmt.Errorf("Hikarinagi token 交换失败: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("Hikarinagi OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	if statusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Hikarinagi token 交换失败，HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" || strings.TrimSpace(tokenResp.RefreshToken) == "" {
		return nil, fmt.Errorf("Hikarinagi token 响应缺少必要字段")
	}
	if err := validateHikarinagiIDTokenNonce(tokenResp.IDToken, nonce); err != nil {
		return nil, err
	}
	return tokenResp, nil
}

func (s *HikarinagiService) requestToken(ctx context.Context, form url.Values) (*hikarinagiTokenResponse, int, []byte, error) {
	req, err := http.NewRequestWithContext(s.resolveContext(ctx), http.MethodPost, hikarinagiOAuthTokenURL, strings.NewReader(form.Encode()))
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
	var tokenResp hikarinagiTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, resp.StatusCode, body, fmt.Errorf("解析令牌响应失败: %w", err)
	}
	return &tokenResp, resp.StatusCode, body, nil
}

func (s *HikarinagiService) fetchCurrentUser(ctx context.Context, accessToken string) (*hikarinagiCurrentUser, error) {
	req, err := http.NewRequestWithContext(s.resolveContext(ctx), http.MethodGet, hikarinagiCurrentUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Hikarinagi 当前用户请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")
	resp, err := s.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Hikarinagi 当前用户信息失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Hikarinagi 当前用户响应失败: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: %s", errHikarinagiUnauthorized, hikarinagiErrorMessage(body))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Hikarinagi 当前用户请求失败，HTTP %d: %s", resp.StatusCode, hikarinagiErrorMessage(body))
	}
	var envelope hikarinagiAPIEnvelope[hikarinagiCurrentUser]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("解析 Hikarinagi 当前用户响应失败: %w", err)
	}
	if !envelope.Success {
		return nil, fmt.Errorf("Hikarinagi 当前用户接口返回失败: %s", hikarinagiEnvelopeErrorMessage(envelope.Error))
	}
	if envelope.Data.ID <= 0 || strings.TrimSpace(envelope.Data.Name) == "" {
		return nil, fmt.Errorf("Hikarinagi 当前用户响应缺少必要字段")
	}
	return &envelope.Data, nil
}

func (s *HikarinagiService) buildProfileAndCache(user *hikarinagiCurrentUser) vo.HikarinagiProfile {
	if user == nil {
		return vo.HikarinagiProfile{}
	}
	avatarURL := s.resolveCachedAvatarURL(user)
	profile := vo.HikarinagiProfile{
		UserID:    strconv.FormatInt(user.ID, 10),
		Username:  strings.TrimSpace(user.Name),
		AvatarURL: avatarURL,
	}
	if user.Nickname != nil {
		profile.Nickname = strings.TrimSpace(*user.Nickname)
	}

	s.mu.Lock()
	if s.config != nil && strings.TrimSpace(s.config.HikarinagiAuthorizedAvatarURL) != avatarURL {
		s.config.HikarinagiAuthorizedAvatarURL = avatarURL
		appconf.SanitizeHikarinagiOAuthConfig(s.config)
		if err := appconf.SaveConfig(s.config); err != nil {
			applog.LogWarningf(s.ctx, "failed to save cached Hikarinagi avatar URL: %v", err)
		}
	}
	s.mu.Unlock()
	return profile
}

func (s *HikarinagiService) resolveCachedAvatarURL(user *hikarinagiCurrentUser) string {
	if user == nil || user.ID <= 0 {
		return ""
	}
	userID := strconv.FormatInt(user.ID, 10)
	_, cachedURL, err := imageutils.FindManagedAvatarFile("hikarinagi", userID)
	if err == nil && cachedURL != "" {
		return cachedURL
	}
	if user.Avatar == nil {
		return ""
	}
	sourceURL := resolveHikarinagiAssetURL(user.Avatar.Src)
	if sourceURL == "" {
		return ""
	}
	localURL, err := imageutils.DownloadAndSaveAvatarImageWithClient(s.httpClient, sourceURL, "hikarinagi", userID)
	if err != nil {
		applog.LogWarningf(s.ctx, "failed to cache Hikarinagi avatar for user %s: %v", userID, err)
		if s.config != nil {
			return strings.TrimSpace(s.config.HikarinagiAuthorizedAvatarURL)
		}
		return ""
	}
	return localURL
}

func (s *HikarinagiService) putGameStatus(ctx context.Context, workID, accessToken, status string) error {
	return s.putGameRate(ctx, workID, accessToken, map[string]string{"status": status}, "状态")
}

func (s *HikarinagiService) putGameRate(ctx context.Context, workID, accessToken string, value any, operation string) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("编码 Hikarinagi%s请求失败: %w", operation, err)
	}
	req, err := http.NewRequestWithContext(
		s.resolveContext(ctx),
		http.MethodPut,
		fmt.Sprintf(hikarinagiGameRateURLFormat, url.PathEscape(workID)),
		strings.NewReader(string(payload)),
	)
	if err != nil {
		return fmt.Errorf("创建 Hikarinagi%s请求失败: %w", operation, err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.doRequest(req)
	if err != nil {
		return fmt.Errorf("请求 Hikarinagi%s接口失败: %w", operation, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取 Hikarinagi%s响应失败: %w", operation, err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: %s", errHikarinagiUnauthorized, hikarinagiErrorMessage(body))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("Hikarinagi%s接口返回 HTTP %d: %s", operation, resp.StatusCode, hikarinagiErrorMessage(body))
	}
	var envelope hikarinagiAPIEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("解析 Hikarinagi%s响应失败: %w", operation, err)
	}
	if !envelope.Success {
		return fmt.Errorf("Hikarinagi%s接口返回失败: %s", operation, hikarinagiEnvelopeErrorMessage(envelope.Error))
	}
	return nil
}

func (s *HikarinagiService) doRequest(req *http.Request) (*http.Response, error) {
	return httputils.DoWithRetry(req.Context(), s.httpClient, req, httputils.RetryPolicy{
		MaxRetries:    1,
		FallbackDelay: time.Second,
		MaxDelay:      30 * time.Second,
	})
}

func (s *HikarinagiService) emitAuthStatusChanged(status vo.HikarinagiAuthStatus) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(hikarinagiMetadataEventName, status)
}

func (s *HikarinagiService) emitStatusSyncProgress(progress vo.RemoteStatusSyncProgress) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(hikarinagiStatusSyncEvent, progress)
}

func (s *HikarinagiService) failStatusSync(
	progress vo.RemoteStatusSyncProgress,
	err error,
) (vo.RemoteStatusSyncProgress, error) {
	progress.Status = "failed"
	progress.LastError = err.Error()
	s.emitStatusSyncProgress(progress)
	return progress, err
}

func (s *HikarinagiService) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func mapGameStatusToHikarinagiStatus(status enums.GameStatus) (string, bool) {
	switch status {
	case enums.StatusNotStarted, enums.StatusWantToPlay:
		return "PLAN", true
	case enums.StatusPlaying:
		return "GOING", true
	case enums.StatusCompleted:
		return "COMPLETED", true
	case enums.StatusOnHold:
		return "ON_HOLD", true
	default:
		return "", false
	}
}

func tokenExpiryDuration(expiresIn int) time.Duration {
	if expiresIn <= 0 {
		return time.Hour
	}
	return time.Duration(expiresIn) * time.Second
}

func buildHikarinagiAuthURL(clientID string, session *hikarinagiAuthSession) string {
	challengeBytes := sha256.Sum256([]byte(session.codeVerifier))
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {session.redirectURI},
		"scope":                 {hikarinagiOAuthScopes},
		"prompt":                {"consent"},
		"state":                 {session.state},
		"nonce":                 {session.nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challengeBytes[:])},
		"code_challenge_method": {"S256"},
	}
	return hikarinagiOAuthAuthorizeURL + "?" + params.Encode()
}

func newHikarinagiAuthSession() (*hikarinagiAuthSession, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", hikarinagiOAuthCallbackPort))
	if err != nil {
		return nil, fmt.Errorf("无法启动 Hikarinagi 本地回调服务: %w", err)
	}
	state, err := generateHikarinagiRandomValue(32)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成 Hikarinagi OAuth state 失败: %w", err)
	}
	nonce, err := generateHikarinagiRandomValue(32)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成 Hikarinagi OAuth nonce 失败: %w", err)
	}
	codeVerifier, err := generateHikarinagiRandomValue(32)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成 Hikarinagi PKCE verifier 失败: %w", err)
	}

	session := &hikarinagiAuthSession{
		resultChan:   make(chan hikarinagiAuthResult, 1),
		listener:     listener,
		state:        state,
		nonce:        nonce,
		codeVerifier: codeVerifier,
		redirectURI:  hikarinagiOAuthRedirectURI,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(hikarinagiOAuthCallbackPath, session.handleOAuthCallback)
	session.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if serveErr := session.server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			session.trySendResult(hikarinagiAuthResult{Error: serveErr.Error()})
		}
	}()
	return session, nil
}

func generateHikarinagiRandomValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func validateHikarinagiIDTokenNonce(idToken, expectedNonce string) error {
	parts := strings.Split(strings.TrimSpace(idToken), ".")
	if len(parts) != 3 {
		return fmt.Errorf("Hikarinagi ID token 格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("解析 Hikarinagi ID token 失败: %w", err)
	}
	var claims hikarinagiTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("解析 Hikarinagi ID token 声明失败: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		return fmt.Errorf("Hikarinagi ID token nonce 校验失败")
	}
	return nil
}

func (s *hikarinagiAuthSession) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	_ = s.listener.Close()
}

func (s *hikarinagiAuthSession) trySendResult(result hikarinagiAuthResult) {
	select {
	case s.resultChan <- result:
	default:
	}
}

func (s *hikarinagiAuthSession) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>请求方法无效</p><p>您可以关闭此窗口。</p></body></html>`)
		return
	}
	if !isLoopbackRequest(r.RemoteAddr) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>回调来源无效</p><p>请返回应用后重试。</p></body></html>`)
		return
	}
	query := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(s.state)) != 1 {
		s.trySendResult(hikarinagiAuthResult{Error: "授权状态校验失败"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>授权状态校验失败</p><p>请返回应用后重试。</p></body></html>`)
		return
	}
	if issuer := strings.TrimSpace(query.Get("iss")); issuer != "" && issuer != hikarinagiOAuthIssuer {
		s.trySendResult(hikarinagiAuthResult{Error: "授权签发方校验失败"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>授权签发方校验失败</p><p>请返回应用后重试。</p></body></html>`)
		return
	}
	if oauthError := strings.TrimSpace(query.Get("error")); oauthError != "" {
		description := strings.TrimSpace(query.Get("error_description"))
		s.trySendResult(hikarinagiAuthResult{Error: strings.TrimSpace(oauthError + ": " + description)})
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>%s: %s</p><p>您可以关闭此窗口。</p></body></html>`, html.EscapeString(oauthError), html.EscapeString(description))
		return
	}
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		s.trySendResult(hikarinagiAuthResult{Error: "未收到授权码"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>未收到授权码</p><p>您可以关闭此窗口。</p></body></html>`)
		return
	}
	s.trySendResult(hikarinagiAuthResult{Code: code})
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权成功</title></head><body><h1>授权成功！</h1><p>您可以关闭此窗口并返回应用。</p><script>window.close();</script></body></html>`)
}

func resolveHikarinagiAssetURL(rawURL string) string {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, _ := url.Parse(hikarinagiImageBaseURL)
	return base.ResolveReference(parsed).String()
}

func hikarinagiErrorMessage(body []byte) string {
	var envelope hikarinagiAPIEnvelope[json.RawMessage]
	if json.Unmarshal(body, &envelope) == nil && envelope.Error != nil {
		return hikarinagiEnvelopeErrorMessage(envelope.Error)
	}
	return strings.TrimSpace(string(body))
}

func hikarinagiEnvelopeErrorMessage(apiError *hikarinagiAPIError) string {
	if apiError == nil {
		return "未知错误"
	}
	if apiError.Code != "" && apiError.Message != "" {
		return apiError.Code + ": " + apiError.Message
	}
	return firstNonEmptyString(apiError.Message, apiError.Code, "未知错误")
}
