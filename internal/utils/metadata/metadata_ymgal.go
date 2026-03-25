package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// YmgalInfoGetter 获取月幕 Galgame 信息。
type YmgalInfoGetter struct {
	client *http.Client
}

func NewYmgalInfoGetter() *YmgalInfoGetter {
	return &YmgalInfoGetter{client: newMetadataClient()}
}

var _ Getter = (*YmgalInfoGetter)(nil)

const (
	ymgalAPIURL       = "https://www.ymgal.games/open/archive"
	ymgalTokenURL     = "https://www.ymgal.games/oauth/token"
	ymgalClientID     = "ymgal"
	ymgalClientSecret = "luna0327"
)

var ymgalTokenCache struct {
	token     string
	expiresAt time.Time
	mu        sync.Mutex
}

type ymgalTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type ymgalGame struct {
	Gid          int64       `json:"gid"`
	Name         string      `json:"name"`
	ChineseName  string      `json:"chineseName"`
	Introduction string      `json:"introduction"`
	MainImg      string      `json:"mainImg"`
	ReleaseDate  string      `json:"releaseDate"`
	Score        interface{} `json:"score"`
	DeveloperID  int64       `json:"developerId"`
}

type ymgalResponse struct {
	Data    *ymgalData `json:"data"`
	Success *bool      `json:"success"`
	Code    int        `json:"code"`
	Msg     string     `json:"msg"`
}

type ymgalData struct {
	Game *ymgalGame `json:"game"`
}

func (y YmgalInfoGetter) getAccessToken() (string, error) {
	ymgalTokenCache.mu.Lock()
	defer ymgalTokenCache.mu.Unlock()

	if ymgalTokenCache.token != "" && time.Now().UTC().Before(ymgalTokenCache.expiresAt) {
		return ymgalTokenCache.token, nil
	}

	params := url.Values{}
	params.Add("grant_type", "client_credentials")
	params.Add("client_id", ymgalClientID)
	params.Add("client_secret", ymgalClientSecret)
	params.Add("scope", "public")

	reqURL := fmt.Sprintf("%s?%s", ymgalTokenURL, params.Encode())
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", metadataUserAgent)

	resp, err := y.client.Do(req)
	if err != nil {
		return "", err
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ymgal token API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp ymgalTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	ymgalTokenCache.token = tokenResp.AccessToken
	ymgalTokenCache.expiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn)*time.Second - 60*time.Second)
	return ymgalTokenCache.token, nil
}

func (y YmgalInfoGetter) invalidateToken() {
	ymgalTokenCache.mu.Lock()
	defer ymgalTokenCache.mu.Unlock()
	ymgalTokenCache.token = ""
	ymgalTokenCache.expiresAt = time.Time{}
}

func (y YmgalInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	accessToken, err := y.getAccessToken()
	if err != nil {
		return MetadataResult{}, fmt.Errorf("failed to get access token: %w", err)
	}

	reqURL := fmt.Sprintf("%s?gid=%s", ymgalAPIURL, id)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return MetadataResult{}, err
	}

	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("version", "1")
	req.Header.Set("Accept", "application/json;charset=utf-8")

	bodyBytes, err := y.doAuthorizedRequest(req)
	if err != nil {
		return MetadataResult{}, err
	}

	var ymgalResp ymgalResponse
	if err := json.Unmarshal(bodyBytes, &ymgalResp); err != nil {
		return MetadataResult{}, err
	}
	if ymgalResp.Success != nil && !*ymgalResp.Success {
		return MetadataResult{}, fmt.Errorf("ymgal API error: %s (code: %d)", ymgalResp.Msg, ymgalResp.Code)
	}
	if ymgalResp.Data == nil || ymgalResp.Data.Game == nil {
		return MetadataResult{}, fmt.Errorf("ymgal API returned no game data, body: %s", string(bodyBytes))
	}

	game, err := y.convertToModel(ymgalResp.Data.Game)
	if err != nil {
		return MetadataResult{}, err
	}
	return MetadataResult{Game: game}, nil
}

func (y YmgalInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	accessToken, err := y.getAccessToken()
	if err != nil {
		return MetadataResult{}, fmt.Errorf("failed to get access token: %w", err)
	}

	searchURL := fmt.Sprintf("%s/search-game", ymgalAPIURL)
	params := url.Values{}
	params.Add("mode", "accurate")
	params.Add("keyword", name)
	params.Add("similarity", "70")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", searchURL, params.Encode()), nil)
	if err != nil {
		return MetadataResult{}, err
	}
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("version", "1")
	req.Header.Set("Accept", "application/json;charset=utf-8")

	bodyBytes, err := y.doAuthorizedRequest(req)
	if err != nil {
		return MetadataResult{}, err
	}

	var ymgalResp ymgalResponse
	if err := json.Unmarshal(bodyBytes, &ymgalResp); err != nil {
		return MetadataResult{}, err
	}
	if ymgalResp.Success != nil && !*ymgalResp.Success {
		return MetadataResult{}, fmt.Errorf("ymgal API error: %s (code: %d)", ymgalResp.Msg, ymgalResp.Code)
	}
	if ymgalResp.Data == nil || ymgalResp.Data.Game == nil {
		return MetadataResult{}, fmt.Errorf("ymgal API returned no game data, body: %s", string(bodyBytes))
	}

	game, err := y.convertToModel(ymgalResp.Data.Game)
	if err != nil {
		return MetadataResult{}, err
	}
	return MetadataResult{Game: game}, nil
}

func (y YmgalInfoGetter) doAuthorizedRequest(req *http.Request) ([]byte, error) {
	resp, err := y.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		y.invalidateToken()

		accessToken, err := y.getAccessToken()
		if err != nil {
			return nil, fmt.Errorf("failed to refresh access token: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = y.client.Do(req)
		if err != nil {
			return nil, err
		}
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ymgal API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	return bodyBytes, nil
}

func (y YmgalInfoGetter) convertToModel(g *ymgalGame) (models.Game, error) {
	name := g.ChineseName
	if name == "" {
		name = g.Name
	}

	return models.Game{
		Name:        name,
		CoverURL:    g.MainImg,
		Company:     "",
		Summary:     g.Introduction,
		Rating:      normalizeTenPointRating(parseYmgalScore(g.Score)),
		ReleaseDate: strings.TrimSpace(g.ReleaseDate),
		SourceType:  enums.Ymgal,
		SourceID:    strconv.FormatInt(g.Gid, 10),
		CachedAt:    time.Now(),
	}, nil
}

func parseYmgalScore(raw interface{}) float64 {
	switch v := raw.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if score, err := v.Float64(); err == nil {
			return score
		}
	case string:
		if score, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return score
		}
	}
	return 0
}
