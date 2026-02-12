package utils

import (
	"bytes"
	"encoding/json"
	"errors"
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

	"github.com/labstack/gommon/log"
)

// Getter 获取元数据
type Getter interface {
	FetchMetadata(id string, token string) (models.Game, error)

	FetchMetadataByName(name string, token string) (models.Game, error)
}

type BangumiInfoGetter struct {
	client  *http.Client
	timeout time.Duration
}

func NewBangumiInfoGetter() *BangumiInfoGetter {
	return &BangumiInfoGetter{
		client:  &http.Client{},
		timeout: 10 * time.Second,
	}
}

var _ Getter = (*BangumiInfoGetter)(nil)

const bangumiIdQueryAPIURL = "https://api.bgm.tv/v0/subjects"

type bangumiImages struct {
	Large  string `json:"large"`
	Common string `json:"common"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Grid   string `json:"grid"`
}

type bangumiInfoboxItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type bangumiRating struct {
	Rank  int            `json:"rank"`
	Total int            `json:"total"`
	Count map[string]int `json:"count"`
	Score float64        `json:"score"`
}

type bangumiCollection struct {
	Wish    int `json:"wish"`
	Collect int `json:"collect"`
	Doing   int `json:"doing"`
	OnHold  int `json:"on_hold"`
	Dropped int `json:"dropped"`
}

type bangumiTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type bangumiResponse struct {
	ID            int                  `json:"id"`
	Type          int                  `json:"type"`
	Name          string               `json:"name"`
	NameCN        string               `json:"name_cn"`
	Summary       string               `json:"summary"`
	Series        bool                 `json:"series"`
	NSFW          bool                 `json:"nsfw"`
	Locked        bool                 `json:"locked"`
	Date          string               `json:"date"`
	Platform      string               `json:"platform"`
	Images        bangumiImages        `json:"images"`
	Infobox       []bangumiInfoboxItem `json:"infobox"`
	Volumes       int                  `json:"volumes"`
	Eps           int                  `json:"eps"`
	TotalEpisodes int                  `json:"total_episodes"`
	Rating        bangumiRating        `json:"rating"`
	Collection    bangumiCollection    `json:"collection"`
	MetaTags      []string             `json:"meta_tags"`
	Tags          []bangumiTag         `json:"tags"`
}

func (b BangumiInfoGetter) FetchMetadata(id string, token string) (models.Game, error) {
	if token == "" {
		return models.Game{}, errors.New("bangumi API requires Bearer token")
	}

	url := fmt.Sprintf("%s/%s", bangumiIdQueryAPIURL, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return models.Game{}, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Saramanda9988/LunaBox/1.4.0 (desktop) (https://github.com/Saramanda9988/LunaBox)")

	resp, err := b.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return models.Game{}, fmt.Errorf("bangumi API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var bangumiResp bangumiResponse
	if err := json.NewDecoder(resp.Body).Decode(&bangumiResp); err != nil {
		return models.Game{}, err
	}

	if bangumiResp.Type != 4 { // 4 代表游戏
		return models.Game{}, errors.New("the provided ID does not correspond to a game")
	}

	// 从 infobox 中提取开发商信息
	company := b.extractCompanyFromInfobox(bangumiResp.Infobox)

	// 使用中文名，如果没有则使用原名
	name := bangumiResp.NameCN
	if name == "" {
		name = bangumiResp.Name
	}

	// 选择最佳的封面图片 (优先使用 large，然后是 common)
	coverURL := bangumiResp.Images.Large
	if coverURL == "" {
		coverURL = bangumiResp.Images.Common
	}

	game := models.Game{
		Name:       name,
		CoverURL:   coverURL,
		Company:    company,
		Summary:    bangumiResp.Summary,
		SourceType: enums.Bangumi,
		SourceID:   id,
		CachedAt:   time.Now(),
	}

	return game, nil
}

func (b BangumiInfoGetter) FetchMetadataByName(name string, token string) (models.Game, error) {
	if token == "" {
		return models.Game{}, errors.New("bangumi API requires Bearer token")
	}

	searchURL := "https://api.bgm.tv/v0/search/subjects"

	params := url.Values{}
	params.Add("limit", "1")
	params.Add("offset", "0")
	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())

	reqBody := map[string]interface{}{
		"keyword": name,
		"sort":    "rank",
		"filter": map[string]interface{}{
			"type": []int{4},
			"nsfw": true,
		},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return models.Game{}, err
	}

	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return models.Game{}, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Saramanda9988/LunaBox/1.4.0 (desktop) (https://github.com/Saramanda9988/LunaBox)")
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return models.Game{}, fmt.Errorf("bangumi search API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp struct {
		Data []bangumiResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return models.Game{}, err
	}

	if len(searchResp.Data) == 0 {
		return models.Game{}, errors.New("no results found")
	}

	bangumiResp := searchResp.Data[0]

	if bangumiResp.Type != 4 { // 4 代表游戏
		return models.Game{}, errors.New("the provided ID does not correspond to a game")
	}

	// 从 infobox 中提取开发商信息
	company := b.extractCompanyFromInfobox(bangumiResp.Infobox)

	// 使用中文名，如果没有则使用原名
	gameName := bangumiResp.NameCN
	if gameName == "" {
		gameName = bangumiResp.Name
	}

	// 选择最佳的封面图片 (优先使用 large，然后是 common)
	coverURL := bangumiResp.Images.Large
	if coverURL == "" {
		coverURL = bangumiResp.Images.Common
	}

	game := models.Game{
		Name:       gameName,
		CoverURL:   coverURL,
		Company:    company,
		Summary:    bangumiResp.Summary,
		SourceType: enums.Bangumi,
		SourceID:   strconv.Itoa(bangumiResp.ID),
		CachedAt:   time.Now(),
	}

	return game, nil
}

// extractCompanyFromInfobox 从 infobox 中提取开发商信息
func (b BangumiInfoGetter) extractCompanyFromInfobox(infobox []bangumiInfoboxItem) string {
	for _, item := range infobox {
		// 查找开发商相关的字段
		if strings.Contains(item.Key, "开发商") || strings.Contains(item.Key, "开发") {
			switch v := item.Value.(type) {
			case string:
				return v
			case []interface{}:
				// 如果是数组，尝试提取第一个值
				if len(v) > 0 {
					if str, ok := v[0].(string); ok {
						return str
					}
					// 处理可能的对象格式 {"v": "value"}
					if obj, ok := v[0].(map[string]interface{}); ok {
						if val, exists := obj["v"]; exists {
							if str, ok := val.(string); ok {
								return str
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// VNDBInfoGetter 获取 VNDB 信息
type VNDBInfoGetter struct {
	client  *http.Client
	timeout time.Duration
}

func NewVNDBInfoGetter() *VNDBInfoGetter {
	return &VNDBInfoGetter{
		client:  &http.Client{},
		timeout: 10 * time.Second,
	}
}

var _ Getter = (*VNDBInfoGetter)(nil)

const vndbAPIURL = "https://api.vndb.org/kana/vn"

type vndbRequest struct {
	Filters []interface{} `json:"filters"`
	Fields  string        `json:"fields"`
}

type vndbImage struct {
	URL string `json:"url"`
}

type vndbDeveloper struct {
	Name string `json:"name"`
}

type vndbQueryResult struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Image       vndbImage       `json:"image"`
	Description string          `json:"description"`
	Developers  []vndbDeveloper `json:"developers"`
}

type vndbResponse struct {
	Results []vndbQueryResult `json:"results"`
}

func (V VNDBInfoGetter) FetchMetadata(id string, token string) (models.Game, error) {
	filters := []interface{}{"id", "=", id}
	return V.queryVNDB(filters)
}

func (V VNDBInfoGetter) FetchMetadataByName(name string, token string) (models.Game, error) {
	filters := []interface{}{"search", "=", name}
	return V.queryVNDB(filters)
}

func (V VNDBInfoGetter) queryVNDB(filters []interface{}) (models.Game, error) {
	reqBody := vndbRequest{
		Filters: filters,
		Fields:  "id, title, image.url, description, developers.name",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return models.Game{}, err
	}

	req, err := http.NewRequest("POST", vndbAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return models.Game{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := V.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return models.Game{}, fmt.Errorf("VNDB API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var vndbResp vndbResponse
	if err := json.NewDecoder(resp.Body).Decode(&vndbResp); err != nil {
		return models.Game{}, err
	}

	if len(vndbResp.Results) == 0 {
		return models.Game{}, errors.New("no results found")
	}

	result := vndbResp.Results[0]

	var company string
	if len(result.Developers) > 0 {
		var devs []string
		for _, d := range result.Developers {
			devs = append(devs, d.Name)
		}
		company = strings.Join(devs, ", ")
	}

	var coverURL string
	if result.Image.URL != "" {
		coverURL = result.Image.URL
	}

	game := models.Game{
		Name:       result.Title,
		CoverURL:   coverURL,
		Company:    company,
		Summary:    result.Description,
		SourceType: enums.VNDB,
		SourceID:   result.ID,
		CachedAt:   time.Now(),
	}

	return game, nil
}

// YmgalInfoGetter 获取月幕Galgame信息
type YmgalInfoGetter struct {
	client  *http.Client
	timeout time.Duration
}

func NewYmgalInfoGetter() *YmgalInfoGetter {
	return &YmgalInfoGetter{
		client:  &http.Client{},
		timeout: 10 * time.Second,
	}
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
	Gid          int64  `json:"gid"`
	Name         string `json:"name"`
	ChineseName  string `json:"chineseName"`
	Introduction string `json:"introduction"`
	MainImg      string `json:"mainImg"`
	ReleaseDate  string `json:"releaseDate"`
	DeveloperID  int64  `json:"developerId"`
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

	req.Header.Set("User-Agent", "Saramanda9988/LunaBox/1.4.0 (desktop) (https://github.com/Saramanda9988/LunaBox)")

	resp, err := y.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ymgal token API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp ymgalTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	ymgalTokenCache.token = tokenResp.AccessToken
	// 提前 60 秒过期，以防万一
	ymgalTokenCache.expiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn)*time.Second - 60*time.Second)

	return ymgalTokenCache.token, nil
}

func (y YmgalInfoGetter) invalidateToken() {
	ymgalTokenCache.mu.Lock()
	defer ymgalTokenCache.mu.Unlock()
	ymgalTokenCache.token = ""
	ymgalTokenCache.expiresAt = time.Time{}
}

func (y YmgalInfoGetter) FetchMetadata(id string, token string) (models.Game, error) {
	accessToken, err := y.getAccessToken()
	if err != nil {
		return models.Game{}, fmt.Errorf("failed to get access token: %w", err)
	}

	reqURL := fmt.Sprintf("%s?gid=%s", ymgalAPIURL, id)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return models.Game{}, err
	}

	req.Header.Set("User-Agent", "Saramanda9988/LunaBox/1.4.0 (desktop) (https://github.com/Saramanda9988/LunaBox)")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("version", "1")
	req.Header.Set("Accept", "application/json;charset=utf-8")

	resp, err := y.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}

	// Handle 401 Unauthorized - Retry once
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		y.invalidateToken()

		accessToken, err = y.getAccessToken()
		if err != nil {
			return models.Game{}, fmt.Errorf("failed to refresh access token: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = y.client.Do(req)
		if err != nil {
			return models.Game{}, err
		}
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.Game{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return models.Game{}, fmt.Errorf("ymgal API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var ymgalResp ymgalResponse
	if err := json.Unmarshal(bodyBytes, &ymgalResp); err != nil {
		return models.Game{}, err
	}

	if ymgalResp.Success != nil && !*ymgalResp.Success {
		return models.Game{}, fmt.Errorf("ymgal API error: %s (code: %d)", ymgalResp.Msg, ymgalResp.Code)
	}

	if ymgalResp.Data == nil || ymgalResp.Data.Game == nil {
		return models.Game{}, fmt.Errorf("ymgal API returned no game data, body: %s", string(bodyBytes))
	}

	return y.convertToModel(ymgalResp.Data.Game)
}

func (y YmgalInfoGetter) FetchMetadataByName(name string, token string) (models.Game, error) {
	accessToken, err := y.getAccessToken()
	if err != nil {
		return models.Game{}, fmt.Errorf("failed to get access token: %w", err)
	}

	searchURL := fmt.Sprintf("%s/search-game", ymgalAPIURL)
	params := url.Values{}
	params.Add("mode", "accurate")
	params.Add("keyword", name)
	params.Add("similarity", "70")
	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return models.Game{}, err
	}

	req.Header.Set("User-Agent", "Saramanda9988/LunaBox/1.4.0 (desktop) (https://github.com/Saramanda9988/LunaBox)")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("version", "1")
	req.Header.Set("Accept", "application/json;charset=utf-8")
	resp, err := y.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}

	// Handle 401 Unauthorized - Retry once
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		y.invalidateToken()

		accessToken, err = y.getAccessToken()
		if err != nil {
			return models.Game{}, fmt.Errorf("failed to refresh access token: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err = y.client.Do(req)
		if err != nil {
			return models.Game{}, err
		}
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.Game{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return models.Game{}, fmt.Errorf("ymgal search API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var ymgalResp ymgalResponse
	if err := json.Unmarshal(bodyBytes, &ymgalResp); err != nil {
		return models.Game{}, err
	}

	if ymgalResp.Success != nil && !*ymgalResp.Success {
		return models.Game{}, fmt.Errorf("ymgal API error: %s (code: %d)", ymgalResp.Msg, ymgalResp.Code)
	}

	if ymgalResp.Data == nil || ymgalResp.Data.Game == nil {
		return models.Game{}, fmt.Errorf("ymgal API returned no game data, body: %s", string(bodyBytes))
	}

	return y.convertToModel(ymgalResp.Data.Game)
}

func (y YmgalInfoGetter) convertToModel(g *ymgalGame) (models.Game, error) {
	name := g.ChineseName
	if name == "" {
		name = g.Name
	}

	game := models.Game{
		Name:       name,
		CoverURL:   g.MainImg,
		Company:    "", // Ymgal API response doesn't directly provide company name in the game object
		Summary:    g.Introduction,
		SourceType: enums.Ymgal,
		SourceID:   strconv.FormatInt(g.Gid, 10),
		CachedAt:   time.Now(),
	}
	return game, nil
}
