package service

import (
	"context"
	"crypto/rand"
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
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/utils/metadata"
	"lunabox/internal/version"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	bangumiOAuthAuthorizeURL   = "https://bgm.tv/oauth/authorize"
	bangumiOAuthTokenURL       = "https://bgm.tv/oauth/access_token"
	bangumiCurrentUserURL      = "https://api.bgm.tv/v0/me"
	bangumiCollectionAPIFormat = "https://api.bgm.tv/v0/users/-/collections/%s"
	bangumiUserAgent           = "LunaBox/1.0"

	bangumiOAuthClientIDEnv     = "LUNABOX_BANGUMI_CLIENT_ID"
	bangumiOAuthClientSecretEnv = "LUNABOX_BANGUMI_CLIENT_SECRET"

	bangumiOAuthCallbackPort = 23679
	bangumiOAuthCallbackPath = "/callback"
	bangumiOAuthRedirectURI  = "http://127.0.0.1:23679/callback"

	bangumiAuthTimeout       = 5 * time.Minute
	bangumiTokenRefreshSkew  = 1 * time.Minute
	bangumiHTTPTimeout       = 30 * time.Second
	bangumiMetadataEventName = "bangumi:auth-status-changed"
)

var errBangumiUnauthorized = errors.New("bangumi unauthorized")

type bangumiAuthSession struct {
	resultChan  chan bangumiAuthResult
	server      *http.Server
	listener    net.Listener
	state       string
	redirectURI string
}

type bangumiAuthResult struct {
	Code  string
	Error string
}

type bangumiTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	UserID       int    `json:"user_id"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

type bangumiCurrentUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
	} `json:"avatar"`
}

type BangumiService struct {
	ctx          context.Context
	db           *sql.DB
	config       *appconf.AppConfig
	httpClient   *http.Client
	openURL      func(context.Context, string) error
	emitEvent    func(context.Context, string, ...interface{})
	now          func() time.Time
	clientID     string
	clientSecret string
	mu           sync.Mutex
}

func NewBangumiService() *BangumiService {
	return &BangumiService{
		httpClient:   &http.Client{Timeout: bangumiHTTPTimeout},
		emitEvent:    runtime.EventsEmit,
		now:          time.Now,
		clientID:     strings.TrimSpace(version.BangumiOAuthClientID),
		clientSecret: strings.TrimSpace(version.BangumiOAuthClientSecret),
	}
}

func (s *BangumiService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	if s.httpClient == nil {
		s.httpClient = &http.Client{Timeout: bangumiHTTPTimeout}
	}
	if s.now == nil {
		s.now = time.Now
	}
	if s.emitEvent == nil {
		s.emitEvent = runtime.EventsEmit
	}
	if s.openURL == nil {
		s.openURL = func(browserCtx context.Context, targetURL string) error {
			runtime.BrowserOpenURL(browserCtx, targetURL)
			return nil
		}
	}
}

func (s *BangumiService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

func (s *BangumiService) SetOpenURLFunc(openURL func(context.Context, string) error) {
	if openURL != nil {
		s.openURL = openURL
	}
}

func (s *BangumiService) SetNowFunc(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *BangumiService) SetOAuthClientCredentials(clientID, clientSecret string) {
	s.clientID = strings.TrimSpace(clientID)
	s.clientSecret = strings.TrimSpace(clientSecret)
}

func (s *BangumiService) SetEventEmitter(emit func(context.Context, string, ...interface{})) {
	s.emitEvent = emit
}

func (s *BangumiService) GetAuthStatus() (vo.BangumiAuthStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildAuthStatusLocked(), nil
}

func (s *BangumiService) GetProfile() (vo.BangumiProfile, error) {
	token, err := s.getValidAccessToken(s.ctx)
	if err != nil {
		return vo.BangumiProfile{}, err
	}

	user, err := s.fetchCurrentUser(s.ctx, token)
	if err == nil {
		return s.buildBangumiProfileAndCache(user), nil
	}
	if !errors.Is(err, errBangumiUnauthorized) {
		return vo.BangumiProfile{}, err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(s.ctx)
	if refreshErr != nil {
		return vo.BangumiProfile{}, refreshErr
	}

	user, err = s.fetchCurrentUser(s.ctx, refreshedToken)
	if err != nil {
		return vo.BangumiProfile{}, err
	}

	return s.buildBangumiProfileAndCache(user), nil
}

func (s *BangumiService) StartAuth() (vo.BangumiAuthStatus, error) {
	if strings.TrimSpace(s.clientID) == "" || strings.TrimSpace(s.clientSecret) == "" {
		return vo.BangumiAuthStatus{}, fmt.Errorf("Bangumi OAuth 未配置，请在构建时通过 %s 和 %s 注入", bangumiOAuthClientIDEnv, bangumiOAuthClientSecretEnv)
	}

	session, err := newBangumiAuthSession()
	if err != nil {
		return vo.BangumiAuthStatus{}, err
	}
	defer session.shutdown()

	authURL := buildBangumiAuthURL(s.clientID, session.redirectURI, session.state)
	if err := s.openURL(s.ctx, authURL); err != nil {
		return vo.BangumiAuthStatus{}, fmt.Errorf("打开 Bangumi 授权页面失败: %w", err)
	}

	timer := time.NewTimer(bangumiAuthTimeout)
	defer timer.Stop()

	select {
	case result := <-session.resultChan:
		if result.Error != "" {
			return vo.BangumiAuthStatus{}, fmt.Errorf("Bangumi 授权失败: %s", result.Error)
		}

		tokenResp, err := s.exchangeAuthorizationCode(s.ctx, result.Code, session.redirectURI, session.state)
		if err != nil {
			return vo.BangumiAuthStatus{}, err
		}
		user, err := s.fetchCurrentUser(s.ctx, tokenResp.AccessToken)
		if err != nil {
			return vo.BangumiAuthStatus{}, fmt.Errorf("获取 Bangumi 当前用户信息失败: %w", err)
		}

		s.mu.Lock()
		status, persistErr := s.persistAuthorizedStateLocked(tokenResp, user)
		s.mu.Unlock()
		if persistErr != nil {
			return vo.BangumiAuthStatus{}, persistErr
		}

		s.emitAuthStatusChanged(status)
		applog.LogInfof(s.ctx, "Bangumi OAuth authorized for user %s (%d)", user.Username, user.ID)
		return status, nil
	case <-timer.C:
		return vo.BangumiAuthStatus{}, fmt.Errorf("Bangumi 授权超时")
	case <-s.resolveContext(s.ctx).Done():
		return vo.BangumiAuthStatus{}, s.resolveContext(s.ctx).Err()
	}
}

func (s *BangumiService) Disconnect() (vo.BangumiAuthStatus, error) {
	s.mu.Lock()
	status, err := s.clearAuthorizationLocked(true, "")
	s.mu.Unlock()
	if err != nil {
		return vo.BangumiAuthStatus{}, err
	}

	s.emitAuthStatusChanged(status)
	applog.LogInfof(s.ctx, "Bangumi OAuth disconnected locally")
	return status, nil
}

func (s *BangumiService) getValidAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return "", fmt.Errorf("Bangumi 配置未初始化")
	}

	accessToken := strings.TrimSpace(s.config.BangumiAccessToken)
	refreshToken := strings.TrimSpace(s.config.BangumiRefreshToken)
	if accessToken != "" && (refreshToken == "" || !s.shouldRefreshTokenLocked()) {
		return accessToken, nil
	}
	if refreshToken == "" {
		return "", fmt.Errorf("Bangumi 未授权")
	}

	return s.refreshAccessTokenLocked(ctx)
}

func (s *BangumiService) refreshAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshAccessTokenLocked(ctx)
}

func (s *BangumiService) fetchMetadataByID(ctx context.Context, sourceID string) (metadata.MetadataResult, error) {
	getter := metadata.NewBangumiInfoGetter()

	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return metadata.MetadataResult{}, err
	}

	result, err := getter.FetchMetadata(sourceID, token)
	if err == nil || !metadata.IsBangumiUnauthorizedError(err) {
		return result, err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return metadata.MetadataResult{}, refreshErr
	}

	return getter.FetchMetadata(sourceID, refreshedToken)
}

func (s *BangumiService) fetchMetadataByName(ctx context.Context, name string) (metadata.MetadataResult, error) {
	getter := metadata.NewBangumiInfoGetter()

	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return metadata.MetadataResult{}, err
	}

	result, err := getter.FetchMetadataByName(name, token)
	if err == nil || !metadata.IsBangumiUnauthorizedError(err) {
		return result, err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return metadata.MetadataResult{}, refreshErr
	}

	return getter.FetchMetadataByName(name, refreshedToken)
}

func (s *BangumiService) syncGameStatus(ctx context.Context, game models.Game) error {
	if !s.isGameEligibleForStatusPush(game) {
		return nil
	}

	return s.upsertSubjectCollectionStatus(ctx, strings.TrimSpace(game.SourceID), game.Status)
}

func (s *BangumiService) upsertSubjectCollectionStatus(ctx context.Context, subjectID string, status enums.GameStatus) error {
	collectionType, ok := mapGameStatusToBangumiCollectionType(status)
	if !ok {
		return fmt.Errorf("不支持同步的 Bangumi 状态: %s", status)
	}

	token, err := s.getValidAccessToken(ctx)
	if err != nil {
		return err
	}

	err = s.postSubjectCollection(ctx, subjectID, token, collectionType)
	if !errors.Is(err, errBangumiUnauthorized) {
		return err
	}

	refreshedToken, refreshErr := s.refreshAccessToken(ctx)
	if refreshErr != nil {
		return refreshErr
	}

	return s.postSubjectCollection(ctx, subjectID, refreshedToken, collectionType)
}

func (s *BangumiService) isGameEligibleForStatusPush(game models.Game) bool {
	return appconf.IsBangumiStatusPushEnabled(s.config) &&
		game.SourceType == enums.Bangumi &&
		strings.TrimSpace(game.SourceID) != ""
}

func (s *BangumiService) shouldRefreshTokenLocked() bool {
	if s.config == nil {
		return false
	}

	expiresAtRaw := strings.TrimSpace(s.config.BangumiTokenExpiresAt)
	if expiresAtRaw == "" {
		return strings.TrimSpace(s.config.BangumiRefreshToken) != ""
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtRaw)
	if err != nil {
		return strings.TrimSpace(s.config.BangumiRefreshToken) != ""
	}

	return !s.now().Add(bangumiTokenRefreshSkew).Before(expiresAt)
}

func (s *BangumiService) refreshAccessTokenLocked(ctx context.Context) (string, error) {
	if s.config == nil {
		return "", fmt.Errorf("Bangumi 配置未初始化")
	}

	refreshToken := strings.TrimSpace(s.config.BangumiRefreshToken)
	if refreshToken == "" {
		if strings.TrimSpace(s.config.BangumiAccessToken) != "" {
			status, clearErr := s.clearAuthorizationLocked(false, "Bangumi 授权已失效，请重新授权")
			if clearErr == nil {
				s.emitAuthStatusChanged(status)
			}
		}
		return "", fmt.Errorf("Bangumi 未授权")
	}
	if strings.TrimSpace(s.clientID) == "" || strings.TrimSpace(s.clientSecret) == "" {
		return "", fmt.Errorf("Bangumi OAuth 未配置，请在构建时通过 %s 和 %s 注入", bangumiOAuthClientIDEnv, bangumiOAuthClientSecretEnv)
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"refresh_token": {refreshToken},
		"redirect_uri":  {bangumiOAuthRedirectURI},
	}

	req, err := http.NewRequestWithContext(s.resolveContext(ctx), http.MethodPost, bangumiOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建 Bangumi refresh 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", bangumiUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("刷新 Bangumi access token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 Bangumi refresh 响应失败: %w", err)
	}

	var tokenResp bangumiTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析 Bangumi refresh 响应失败: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest || tokenResp.Error != "" {
		message := "Bangumi 授权已失效，请重新授权"
		if tokenResp.ErrorDesc != "" {
			message = tokenResp.ErrorDesc
		} else if tokenResp.Error != "" {
			message = tokenResp.Error
		}
		status, clearErr := s.clearAuthorizationLocked(false, message)
		if clearErr == nil {
			s.emitAuthStatusChanged(status)
		}
		if tokenResp.Error != "" {
			return "", fmt.Errorf("Bangumi refresh token 无效: %s", tokenResp.ErrorDesc)
		}
		return "", fmt.Errorf("Bangumi refresh token 无效，HTTP %d", resp.StatusCode)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("Bangumi refresh 请求失败，HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("Bangumi refresh 未返回 access token")
	}

	expiresAt := s.now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	s.config.BangumiAccessToken = strings.TrimSpace(tokenResp.AccessToken)
	if strings.TrimSpace(tokenResp.RefreshToken) != "" {
		s.config.BangumiRefreshToken = strings.TrimSpace(tokenResp.RefreshToken)
	}
	s.config.BangumiTokenExpiresAt = expiresAt.Format(time.RFC3339)
	s.config.BangumiAuthError = ""
	appconf.SanitizeBangumiOAuthConfig(s.config)

	if err := appconf.SaveConfig(s.config); err != nil {
		return "", fmt.Errorf("保存 Bangumi 刷新后配置失败: %w", err)
	}

	status := s.buildAuthStatusLocked()
	s.emitAuthStatusChanged(status)
	applog.LogInfof(s.ctx, "Bangumi access token refreshed successfully")
	return s.config.BangumiAccessToken, nil
}

func (s *BangumiService) buildAuthStatusLocked() vo.BangumiAuthStatus {
	if s.config == nil {
		return vo.BangumiAuthStatus{}
	}

	accessToken := strings.TrimSpace(s.config.BangumiAccessToken)
	refreshToken := strings.TrimSpace(s.config.BangumiRefreshToken)
	authError := strings.TrimSpace(s.config.BangumiAuthError)

	return vo.BangumiAuthStatus{
		Authorized:           accessToken != "" || refreshToken != "",
		NeedsReauthorization: authError != "",
		LegacyToken:          accessToken != "" && refreshToken == "",
		UserID:               strings.TrimSpace(s.config.BangumiAuthorizedUserID),
		Username:             strings.TrimSpace(s.config.BangumiAuthorizedUsername),
		AvatarURL:            strings.TrimSpace(s.config.BangumiAuthorizedAvatarURL),
		AccessTokenExpiresAt: strings.TrimSpace(s.config.BangumiTokenExpiresAt),
		LastError:            authError,
	}
}

func (s *BangumiService) persistAuthorizedStateLocked(tokenResp *bangumiTokenResponse, user *bangumiCurrentUser) (vo.BangumiAuthStatus, error) {
	if s.config == nil {
		return vo.BangumiAuthStatus{}, fmt.Errorf("Bangumi 配置未初始化")
	}

	previousUserID := strings.TrimSpace(s.config.BangumiAuthorizedUserID)
	expiresAt := s.now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	s.config.BangumiAccessToken = strings.TrimSpace(tokenResp.AccessToken)
	s.config.BangumiRefreshToken = strings.TrimSpace(tokenResp.RefreshToken)
	s.config.BangumiTokenExpiresAt = expiresAt.Format(time.RFC3339)
	s.config.BangumiAuthorizedUserID = strconv.Itoa(user.ID)
	s.config.BangumiAuthorizedUsername = strings.TrimSpace(user.Username)
	s.config.BangumiAuthorizedAvatarURL = s.resolveCachedBangumiAvatarURL(user)
	s.config.BangumiAuthError = ""
	appconf.SanitizeBangumiOAuthConfig(s.config)

	if previousUserID != "" && previousUserID != s.config.BangumiAuthorizedUserID {
		_ = imageutils.RemoveManagedAvatar("bangumi", previousUserID)
	}

	if err := appconf.SaveConfig(s.config); err != nil {
		return vo.BangumiAuthStatus{}, fmt.Errorf("保存 Bangumi 授权配置失败: %w", err)
	}

	return s.buildAuthStatusLocked(), nil
}

func (s *BangumiService) clearAuthorizationLocked(clearIdentity bool, reason string) (vo.BangumiAuthStatus, error) {
	if s.config == nil {
		return vo.BangumiAuthStatus{}, fmt.Errorf("Bangumi 配置未初始化")
	}

	previousUserID := strings.TrimSpace(s.config.BangumiAuthorizedUserID)
	s.config.BangumiAccessToken = ""
	s.config.BangumiRefreshToken = ""
	s.config.BangumiTokenExpiresAt = ""
	s.config.BangumiAuthError = strings.TrimSpace(reason)
	if clearIdentity {
		s.config.BangumiAuthorizedUserID = ""
		s.config.BangumiAuthorizedUsername = ""
		s.config.BangumiAuthorizedAvatarURL = ""
	}
	appconf.SanitizeBangumiOAuthConfig(s.config)

	if clearIdentity && previousUserID != "" {
		_ = imageutils.RemoveManagedAvatar("bangumi", previousUserID)
	}

	if err := appconf.SaveConfig(s.config); err != nil {
		return vo.BangumiAuthStatus{}, fmt.Errorf("保存 Bangumi 配置失败: %w", err)
	}

	return s.buildAuthStatusLocked(), nil
}

func (s *BangumiService) exchangeAuthorizationCode(ctx context.Context, code, redirectURI, state string) (*bangumiTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	if state != "" {
		form.Set("state", state)
	}

	req, err := http.NewRequestWithContext(s.resolveContext(ctx), http.MethodPost, bangumiOAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建 Bangumi token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", bangumiUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Bangumi token 交换失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Bangumi token 响应失败: %w", err)
	}

	var tokenResp bangumiTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 Bangumi token 响应失败: %w", err)
	}
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("Bangumi OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Bangumi token 交换失败，HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" || strings.TrimSpace(tokenResp.RefreshToken) == "" {
		return nil, fmt.Errorf("Bangumi token 响应缺少必要字段")
	}

	return &tokenResp, nil
}

func (s *BangumiService) fetchCurrentUser(ctx context.Context, accessToken string) (*bangumiCurrentUser, error) {
	req, err := http.NewRequestWithContext(s.resolveContext(ctx), http.MethodGet, bangumiCurrentUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Bangumi 当前用户请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", bangumiUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Bangumi 当前用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Bangumi 当前用户响应失败: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: %s", errBangumiUnauthorized, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Bangumi 当前用户请求失败，HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user bangumiCurrentUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("解析 Bangumi 当前用户响应失败: %w", err)
	}
	if strings.TrimSpace(user.Username) == "" {
		return nil, fmt.Errorf("Bangumi 当前用户响应缺少 username")
	}
	return &user, nil
}

func buildBangumiProfile(user *bangumiCurrentUser) vo.BangumiProfile {
	if user == nil {
		return vo.BangumiProfile{}
	}

	return vo.BangumiProfile{
		UserID:       strconv.Itoa(user.ID),
		Username:     strings.TrimSpace(user.Username),
		Nickname:     strings.TrimSpace(user.Nickname),
		AvatarLarge:  strings.TrimSpace(user.Avatar.Large),
		AvatarMedium: strings.TrimSpace(user.Avatar.Medium),
		AvatarSmall:  strings.TrimSpace(user.Avatar.Small),
	}
}

func (s *BangumiService) buildBangumiProfileAndCache(user *bangumiCurrentUser) vo.BangumiProfile {
	profile := buildBangumiProfile(user)
	if user == nil {
		return profile
	}

	avatarURL := s.resolveCachedBangumiAvatarURL(user)
	profile.AvatarURL = avatarURL
	if avatarURL != "" {
		profile.AvatarLarge = avatarURL
		profile.AvatarMedium = avatarURL
		profile.AvatarSmall = avatarURL
	}

	s.mu.Lock()
	if s.config != nil && strings.TrimSpace(s.config.BangumiAuthorizedAvatarURL) != avatarURL {
		s.config.BangumiAuthorizedAvatarURL = avatarURL
		appconf.SanitizeBangumiOAuthConfig(s.config)
		if err := appconf.SaveConfig(s.config); err != nil {
			applog.LogWarningf(s.ctx, "failed to save cached Bangumi avatar URL: %v", err)
		}
	}
	s.mu.Unlock()

	return profile
}

func (s *BangumiService) resolveCachedBangumiAvatarURL(user *bangumiCurrentUser) string {
	if user == nil {
		return ""
	}

	userID := strconv.Itoa(user.ID)
	if userID == "" || userID == "0" {
		return ""
	}

	_, cachedURL, err := imageutils.FindManagedAvatarFile("bangumi", userID)
	if err == nil && cachedURL != "" {
		return cachedURL
	}

	sourceURL := firstNonEmptyString(
		strings.TrimSpace(user.Avatar.Large),
		strings.TrimSpace(user.Avatar.Medium),
		strings.TrimSpace(user.Avatar.Small),
	)
	if sourceURL == "" {
		return ""
	}

	localURL, err := imageutils.DownloadAndSaveAvatarImageWithClient(s.httpClient, sourceURL, "bangumi", userID)
	if err != nil {
		applog.LogWarningf(s.ctx, "failed to cache Bangumi avatar for user %s: %v", userID, err)
		if s.config != nil {
			return strings.TrimSpace(s.config.BangumiAuthorizedAvatarURL)
		}
		return ""
	}

	return localURL
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *BangumiService) postSubjectCollection(ctx context.Context, subjectID, accessToken string, collectionType int) error {
	payloadBytes, err := json.Marshal(map[string]interface{}{
		"type": collectionType,
	})
	if err != nil {
		return fmt.Errorf("编码 Bangumi 收藏请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(
		s.resolveContext(ctx),
		http.MethodPost,
		fmt.Sprintf(bangumiCollectionAPIFormat, subjectID),
		strings.NewReader(string(payloadBytes)),
	)
	if err != nil {
		return fmt.Errorf("创建 Bangumi 收藏请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", bangumiUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Bangumi 收藏接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", errBangumiUnauthorized, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bangumi 收藏接口返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func (s *BangumiService) emitAuthStatusChanged(status vo.BangumiAuthStatus) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(s.ctx, bangumiMetadataEventName, status)
}

func (s *BangumiService) resolveContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func mapGameStatusToBangumiCollectionType(status enums.GameStatus) (int, bool) {
	switch status {
	case enums.StatusNotStarted:
		return 1, true
	case enums.StatusCompleted:
		return 2, true
	case enums.StatusPlaying:
		return 3, true
	case enums.StatusOnHold:
		return 4, true
	default:
		return 0, false
	}
}

func buildBangumiAuthURL(clientID, redirectURI, state string) string {
	params := url.Values{
		"client_id":     {clientID},
		"response_type": {"code"},
		"redirect_uri":  {redirectURI},
	}
	if state != "" {
		params.Set("state", state)
	}

	return bangumiOAuthAuthorizeURL + "?" + params.Encode()
}

func newBangumiAuthSession() (*bangumiAuthSession, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", bangumiOAuthCallbackPort))
	if err != nil {
		return nil, fmt.Errorf("无法启动 Bangumi 本地回调服务: %w", err)
	}

	state, err := generateBangumiOAuthState()
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成 Bangumi OAuth state 失败: %w", err)
	}

	session := &bangumiAuthSession{
		resultChan:  make(chan bangumiAuthResult, 1),
		listener:    listener,
		state:       state,
		redirectURI: bangumiOAuthRedirectURI,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(bangumiOAuthCallbackPath, session.handleOAuthCallback)
	session.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if serveErr := session.server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			session.trySendResult(bangumiAuthResult{Error: serveErr.Error()})
		}
	}()

	return session, nil
}

func generateBangumiOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *bangumiAuthSession) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	_ = s.listener.Close()
}

func (s *bangumiAuthSession) trySendResult(result bangumiAuthResult) {
	select {
	case s.resultChan <- result:
	default:
	}
}

func (s *bangumiAuthSession) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
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

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errorMsg := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	if subtle.ConstantTimeCompare([]byte(state), []byte(s.state)) != 1 {
		s.trySendResult(bangumiAuthResult{Error: "授权状态校验失败"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>授权状态校验失败</p><p>请返回应用后重试。</p></body></html>`)
		return
	}

	if errorMsg != "" {
		errorMsg = html.EscapeString(errorMsg)
		errorDesc = html.EscapeString(errorDesc)
		s.trySendResult(bangumiAuthResult{Error: fmt.Sprintf("%s: %s", errorMsg, errorDesc)})
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>%s: %s</p><p>您可以关闭此窗口。</p></body></html>`, errorMsg, errorDesc)
		return
	}

	if code == "" {
		s.trySendResult(bangumiAuthResult{Error: "未收到授权码"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body><h1>授权失败</h1><p>未收到授权码</p><p>您可以关闭此窗口。</p></body></html>`)
		return
	}

	s.trySendResult(bangumiAuthResult{Code: code})
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权成功</title></head><body><h1>授权成功！</h1><p>您可以关闭此窗口并返回应用。</p><script>window.close();</script></body></html>`)
}

func isLoopbackRequest(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
