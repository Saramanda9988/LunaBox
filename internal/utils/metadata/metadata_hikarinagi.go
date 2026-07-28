package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/version"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hikarinagiAPIBaseURL = "https://www.hikarinagi.org/api/v3/open"
	hikarinagiTokenURL   = "https://id.hikarinagi.org/oidc/token"
	hikarinagiScope      = "catalog:read"
)

type HikarinagiInfoGetter struct {
	client   *http.Client
	tagLimit int
}

func NewHikarinagiInfoGetter(options ...GetterOption) *HikarinagiInfoGetter {
	config := newGetterConfig(options)
	return &HikarinagiInfoGetter{
		client:   config.client,
		tagLimit: config.tagLimit,
	}
}

var _ Getter = (*HikarinagiInfoGetter)(nil)

var hikarinagiTokenCache struct {
	clientID     string
	clientSecret string
	token        string
	expiresAt    time.Time
	mu           sync.Mutex
}

type hikarinagiTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type hikarinagiEnvelope[T any] struct {
	Success   bool            `json:"success"`
	Data      T               `json:"data"`
	Message   string          `json:"message"`
	Error     json.RawMessage `json:"error"`
	RequestID string          `json:"request_id"`
}

type hikarinagiCover struct {
	URL      string `json:"url"`
	Width    *int   `json:"width"`
	Height   *int   `json:"height"`
	Sexual   int    `json:"sexual"`
	Violence int    `json:"violence"`
	Votes    int    `json:"votes"`
}

type hikarinagiTag struct {
	Name  string `json:"name"`
	Likes int    `json:"likes"`
}

type hikarinagiGame struct {
	ID          int64             `json:"id"`
	OriginTitle string            `json:"origin_title"`
	TransTitle  *string           `json:"trans_title"`
	Covers      []hikarinagiCover `json:"covers"`
	ReleaseDate *string           `json:"release_date"`
	OriginIntro *string           `json:"origin_intro"`
	TransIntro  *string           `json:"trans_intro"`
	NSFW        bool              `json:"nsfw"`
	Tags        []hikarinagiTag   `json:"tags"`
}

type hikarinagiSearchHit struct {
	Type      string           `json:"type"`
	ID        int64            `json:"id"`
	Title     string           `json:"title"`
	Subtitle  *string          `json:"subtitle"`
	Developer *string          `json:"developer"`
	Cover     *hikarinagiCover `json:"cover"`
}

type hikarinagiSearchData struct {
	Items []hikarinagiSearchHit `json:"items"`
}

func NormalizeHikarinagiID(id string) (string, bool) {
	normalized := strings.TrimSpace(id)
	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || parsed <= 0 {
		return "", false
	}
	return strconv.FormatInt(parsed, 10), true
}

func (h HikarinagiInfoGetter) FetchMetadata(id string, _ string) (MetadataResult, error) {
	normalizedID, ok := NormalizeHikarinagiID(id)
	if !ok {
		return MetadataResult{}, fmt.Errorf("invalid Hikarinagi ID format: %s", id)
	}

	bodyBytes, err := h.doAuthorizedGet(fmt.Sprintf("%s/galgames/%s", hikarinagiAPIBaseURL, url.PathEscape(normalizedID)))
	if err != nil {
		return MetadataResult{}, err
	}

	var envelope hikarinagiEnvelope[hikarinagiGame]
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return MetadataResult{}, fmt.Errorf("decode Hikarinagi detail response: %w", err)
	}
	if !envelope.Success {
		return MetadataResult{}, hikarinagiEnvelopeError("Hikarinagi detail API", envelope.Message, envelope.Error, envelope.RequestID)
	}
	if envelope.Data.ID <= 0 {
		return MetadataResult{}, errors.New("Hikarinagi API returned no game data")
	}

	return h.convertToMetadataResult(envelope.Data), nil
}

func (h HikarinagiInfoGetter) FetchMetadataByName(name string, _ string) (MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return MetadataResult{}, errors.New("Hikarinagi search keyword is empty")
	}

	params := url.Values{}
	params.Set("q", keyword)
	params.Add("types", "galgame")
	params.Set("page", "1")
	params.Set("page_size", "1")
	bodyBytes, err := h.doAuthorizedGet(fmt.Sprintf("%s/search?%s", hikarinagiAPIBaseURL, params.Encode()))
	if err != nil {
		return MetadataResult{}, err
	}

	var envelope hikarinagiEnvelope[hikarinagiSearchData]
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return MetadataResult{}, fmt.Errorf("decode Hikarinagi search response: %w", err)
	}
	if !envelope.Success {
		return MetadataResult{}, hikarinagiEnvelopeError("Hikarinagi search API", envelope.Message, envelope.Error, envelope.RequestID)
	}
	if len(envelope.Data.Items) == 0 {
		return MetadataResult{}, errors.New("no results found")
	}

	hit := envelope.Data.Items[0]
	if hit.Type != "galgame" || hit.ID <= 0 {
		return MetadataResult{}, errors.New("no results found")
	}
	result, err := h.FetchMetadata(strconv.FormatInt(hit.ID, 10), "")
	if err != nil {
		return MetadataResult{}, err
	}
	if hit.Developer != nil {
		result.Game.Company = strings.TrimSpace(*hit.Developer)
	}
	if result.Game.CoverURL == "" && hit.Cover != nil {
		result.Game.CoverURL = strings.TrimSpace(hit.Cover.URL)
	}
	return result, nil
}

func (h HikarinagiInfoGetter) getAccessToken() (string, error) {
	clientID := strings.TrimSpace(version.HikarinagiOAuthClientID)
	clientSecret := strings.TrimSpace(version.HikarinagiOAuthClientSecret)
	if clientID == "" || clientSecret == "" {
		return "", errors.New("Hikarinagi API requires injected OAuth client credentials")
	}

	hikarinagiTokenCache.mu.Lock()
	defer hikarinagiTokenCache.mu.Unlock()

	now := time.Now().UTC()
	if hikarinagiTokenCache.clientID == clientID &&
		hikarinagiTokenCache.clientSecret == clientSecret &&
		hikarinagiTokenCache.token != "" &&
		now.Before(hikarinagiTokenCache.expiresAt) {
		return hikarinagiTokenCache.token, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", hikarinagiScope)
	req, err := http.NewRequest(http.MethodPost, hikarinagiTokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create Hikarinagi token request: %w", err)
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Hikarinagi access token: %w", err)
	}
	defer closeResponseBody(resp.Body)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read Hikarinagi token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Hikarinagi token API returned status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var tokenResponse hikarinagiTokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResponse); err != nil {
		return "", fmt.Errorf("decode Hikarinagi token response: %w", err)
	}
	token := strings.TrimSpace(tokenResponse.AccessToken)
	if token == "" {
		return "", errors.New("Hikarinagi token API returned an empty access token")
	}
	expiresIn := time.Duration(tokenResponse.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = time.Hour
	}
	refreshBefore := time.Minute
	if expiresIn <= refreshBefore {
		refreshBefore = 0
	}

	hikarinagiTokenCache.clientID = clientID
	hikarinagiTokenCache.clientSecret = clientSecret
	hikarinagiTokenCache.token = token
	hikarinagiTokenCache.expiresAt = now.Add(expiresIn - refreshBefore)
	return token, nil
}

func (h HikarinagiInfoGetter) invalidateAccessToken() {
	hikarinagiTokenCache.mu.Lock()
	defer hikarinagiTokenCache.mu.Unlock()
	hikarinagiTokenCache.token = ""
	hikarinagiTokenCache.expiresAt = time.Time{}
}

func (h HikarinagiInfoGetter) doAuthorizedGet(reqURL string) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		accessToken, err := h.getAccessToken()
		if err != nil {
			return nil, fmt.Errorf("get Hikarinagi access token: %w", err)
		}

		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create Hikarinagi API request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", version.UserAgent())

		statusCode, _, bodyBytes, err := doLimitedMetadataRequestBody(h.client, req, enums.Hikarinagi)
		if err != nil {
			return nil, err
		}
		if statusCode == http.StatusUnauthorized && attempt == 0 {
			h.invalidateAccessToken()
			continue
		}
		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("Hikarinagi API returned status: %d, body: %s", statusCode, strings.TrimSpace(string(bodyBytes)))
		}
		return bodyBytes, nil
	}

	return nil, errors.New("Hikarinagi API authorization failed after token refresh")
}

func (h HikarinagiInfoGetter) convertToMetadataResult(data hikarinagiGame) MetadataResult {
	name := strings.TrimSpace(data.OriginTitle)
	if data.TransTitle != nil && strings.TrimSpace(*data.TransTitle) != "" {
		name = strings.TrimSpace(*data.TransTitle)
	}
	summary := ""
	if data.TransIntro != nil && strings.TrimSpace(*data.TransIntro) != "" {
		summary = strings.TrimSpace(*data.TransIntro)
	} else if data.OriginIntro != nil {
		summary = strings.TrimSpace(*data.OriginIntro)
	}
	releaseDate := ""
	if data.ReleaseDate != nil {
		releaseDate = normalizeHikarinagiDate(*data.ReleaseDate)
	}

	coverURL := bestHikarinagiCoverURL(data.Covers)
	game := models.Game{
		Name:           name,
		CoverURL:       coverURL,
		CoverSourceURL: coverURL,
		Summary:        summary,
		ReleaseDate:    releaseDate,
		IsNSFW:         data.NSFW,
		SourceType:     enums.Hikarinagi,
		SourceID:       strconv.FormatInt(data.ID, 10),
		CachedAt:       time.Now(),
	}
	return MetadataResult{Game: game, Tags: extractHikarinagiTags(data.Tags, h.tagLimit)}
}

func bestHikarinagiCoverURL(covers []hikarinagiCover) string {
	bestURL := ""
	bestVotes := -1
	for _, cover := range covers {
		coverURL := strings.TrimSpace(cover.URL)
		if coverURL == "" || cover.Votes < bestVotes {
			continue
		}
		bestURL = coverURL
		bestVotes = cover.Votes
	}
	return bestURL
}

func normalizeHikarinagiDate(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Format(time.DateOnly)
	}
	return value
}

func extractHikarinagiTags(tags []hikarinagiTag, limit int) []TagItem {
	if limit == 0 {
		return nil
	}
	filtered := make([]hikarinagiTag, 0, len(tags))
	for _, tag := range tags {
		tag.Name = strings.TrimSpace(tag.Name)
		if tag.Name != "" {
			filtered = append(filtered, tag)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Likes > filtered[j].Likes
	})
	filtered = filtered[:tagItemsCapacity(len(filtered), limit)]
	maxLikes := filtered[0].Likes
	result := make([]TagItem, 0, len(filtered))
	for _, tag := range filtered {
		weight := 1.0
		if maxLikes > 0 {
			weight = float64(tag.Likes) / float64(maxLikes)
		}
		result = append(result, TagItem{
			Name:      tag.Name,
			Source:    string(enums.Hikarinagi),
			Weight:    weight,
			IsSpoiler: false,
		})
	}
	return result
}

func hikarinagiEnvelopeError(prefix string, message string, rawError json.RawMessage, requestID string) error {
	detail := strings.TrimSpace(message)
	if detail == "" && len(rawError) > 0 && string(rawError) != "null" {
		detail = strings.TrimSpace(string(rawError))
	}
	if detail == "" {
		detail = "unknown error"
	}
	if requestID != "" {
		return fmt.Errorf("%s error: %s (request_id: %s)", prefix, detail, requestID)
	}
	return fmt.Errorf("%s error: %s", prefix, detail)
}
