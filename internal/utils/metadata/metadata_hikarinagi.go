package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"lunabox/internal/version"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
)

const (
	hikarinagiAPIBaseURL = "https://www.hikarinagi.org/api/v3/open"
	hikarinagiPageAPIURL = "https://www.hikarinagi.org/api/pages"
	hikarinagiTokenURL   = "https://id.hikarinagi.org/oidc/token"
	hikarinagiScope      = "catalog:read"
)

var ErrHikarinagiUnauthorized = errors.New("hikarinagi unauthorized")

func IsHikarinagiUnauthorizedError(err error) bool {
	return errors.Is(err, ErrHikarinagiUnauthorized)
}

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
	Aliases     []string          `json:"aliases"`
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

type hikarinagiProducer struct {
	Name string `json:"name"`
}

type hikarinagiProducerRelation struct {
	Role     string             `json:"role"`
	Producer hikarinagiProducer `json:"producer"`
}

type hikarinagiRateStats struct {
	Average *float64 `json:"average"`
}

type hikarinagiPageData struct {
	Producers []hikarinagiProducerRelation `json:"producers"`
	RateStats hikarinagiRateStats          `json:"rate_stats"`
}

type hikarinagiPageMetadata struct {
	Company string
	Rating  float64
}

func NormalizeHikarinagiID(id string) (string, bool) {
	normalized := strings.TrimSpace(id)
	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || parsed <= 0 {
		return "", false
	}
	return strconv.FormatInt(parsed, 10), true
}

func (h HikarinagiInfoGetter) FetchMetadata(id string, accessToken string) (MetadataResult, error) {
	normalizedID, ok := NormalizeHikarinagiID(id)
	if !ok {
		return MetadataResult{}, fmt.Errorf("invalid Hikarinagi ID format: %s", id)
	}

	bodyBytes, err := h.doAuthorizedGet(
		fmt.Sprintf("%s/galgames/%s", hikarinagiAPIBaseURL, url.PathEscape(normalizedID)),
		accessToken,
	)
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

	result := h.convertToMetadataResult(envelope.Data)
	pageMetadata, err := h.fetchPageMetadata(normalizedID)
	if err != nil {
		log.Warnf("failed to fetch Hikarinagi page metadata for game %s: %v", normalizedID, err)
	} else {
		result.Game.Company = pageMetadata.Company
		result.Game.Rating = pageMetadata.Rating
	}
	return result, nil
}

func (h HikarinagiInfoGetter) FetchMetadataByName(name string, accessToken string) (MetadataResult, error) {
	results, err := h.FetchMetadataCandidatesByName(name, accessToken)
	if err != nil {
		return MetadataResult{}, err
	}
	return results[0], nil
}

func (h HikarinagiInfoGetter) FetchMetadataCandidatesByName(name string, accessToken string) ([]MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return nil, errors.New("Hikarinagi search keyword is empty")
	}

	params := url.Values{}
	params.Set("q", keyword)
	params.Add("types", "galgame")
	params.Set("page", "1")
	params.Set("page_size", strconv.Itoa(metadataSearchCandidateLimit))
	bodyBytes, err := h.doAuthorizedGet(fmt.Sprintf("%s/search?%s", hikarinagiAPIBaseURL, params.Encode()), accessToken)
	if err != nil {
		return nil, err
	}

	var envelope hikarinagiEnvelope[hikarinagiSearchData]
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, fmt.Errorf("decode Hikarinagi search response: %w", err)
	}
	if !envelope.Success {
		return nil, hikarinagiEnvelopeError("Hikarinagi search API", envelope.Message, envelope.Error, envelope.RequestID)
	}
	if len(envelope.Data.Items) == 0 {
		return nil, errors.New("no results found")
	}

	hits := make([]hikarinagiSearchHit, 0, metadataSearchCandidateLimit)
	candidateNames := make([][]string, 0, metadataSearchCandidateLimit)
	for _, hit := range envelope.Data.Items {
		if len(hits) >= metadataSearchCandidateLimit {
			break
		}
		if hit.Type != "galgame" || hit.ID <= 0 {
			continue
		}
		names := []string{hit.Title}
		if hit.Subtitle != nil {
			names = append(names, *hit.Subtitle)
		}
		hits = append(hits, hit)
		candidateNames = append(candidateNames, names)
	}
	if len(hits) == 0 {
		return nil, errors.New("no results found")
	}
	indexes := exactMetadataCandidateIndexes(keyword, candidateNames)
	if len(indexes) == 0 {
		indexes = []int{0}
	}

	results := make([]MetadataResult, 0, len(indexes))
	seenIDs := make(map[int64]struct{}, len(indexes))
	var lastErr error
	for _, index := range indexes {
		hit := hits[index]
		if _, exists := seenIDs[hit.ID]; exists {
			continue
		}
		result, fetchErr := h.FetchMetadata(strconv.FormatInt(hit.ID, 10), accessToken)
		if fetchErr != nil {
			lastErr = fetchErr
			continue
		}
		if result.Game.Company == "" && hit.Developer != nil {
			result.Game.Company = strings.TrimSpace(*hit.Developer)
		}
		if result.Game.CoverURL == "" && hit.Cover != nil {
			result.Game.CoverURL = strings.TrimSpace(hit.Cover.URL)
		}
		seenIDs[hit.ID] = struct{}{}
		results = append(results, result)
	}
	if len(results) > 0 {
		return results, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no results found")
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

func (h HikarinagiInfoGetter) doAuthorizedGet(reqURL, providedToken string) ([]byte, error) {
	providedToken = strings.TrimSpace(providedToken)
	if providedToken != "" {
		return h.doGetWithToken(reqURL, providedToken)
	}

	for attempt := 0; attempt < 2; attempt++ {
		accessToken, err := h.getAccessToken()
		if err != nil {
			return nil, fmt.Errorf("get Hikarinagi access token: %w", err)
		}

		bodyBytes, err := h.doGetWithToken(reqURL, accessToken)
		if errors.Is(err, ErrHikarinagiUnauthorized) && attempt == 0 {
			h.invalidateAccessToken()
			continue
		}
		if err != nil {
			return nil, err
		}
		return bodyBytes, nil
	}

	return nil, errors.New("Hikarinagi API authorization failed after token refresh")
}

func (h HikarinagiInfoGetter) doGetWithToken(reqURL, accessToken string) ([]byte, error) {
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
	if statusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("%w: %s", ErrHikarinagiUnauthorized, strings.TrimSpace(string(bodyBytes)))
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("Hikarinagi API returned status: %d, body: %s", statusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return bodyBytes, nil
}

func (h HikarinagiInfoGetter) fetchPageMetadata(id string) (hikarinagiPageMetadata, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s/galgames/%s", hikarinagiPageAPIURL, url.PathEscape(id)),
		nil,
	)
	if err != nil {
		return hikarinagiPageMetadata{}, fmt.Errorf("create Hikarinagi page data request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())

	statusCode, _, bodyBytes, err := doLimitedMetadataRequestBody(h.client, req, enums.Hikarinagi)
	if err != nil {
		return hikarinagiPageMetadata{}, fmt.Errorf("request Hikarinagi page data: %w", err)
	}
	if statusCode != http.StatusOK {
		return hikarinagiPageMetadata{}, fmt.Errorf("Hikarinagi page data API returned status: %d, body: %s", statusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var pageData hikarinagiPageData
	if err := json.Unmarshal(bodyBytes, &pageData); err != nil {
		return hikarinagiPageMetadata{}, fmt.Errorf("decode Hikarinagi page data response: %w", err)
	}
	return convertHikarinagiPageData(pageData), nil
}

func convertHikarinagiPageData(pageData hikarinagiPageData) hikarinagiPageMetadata {
	developers := make([]string, 0, len(pageData.Producers))
	for _, relation := range pageData.Producers {
		if !strings.EqualFold(strings.TrimSpace(relation.Role), "DEVELOPER") {
			continue
		}
		developers = append(developers, strings.TrimSpace(relation.Producer.Name))
	}

	rating := 0.0
	if pageData.RateStats.Average != nil {
		rating = normalizeTenPointRating(*pageData.RateStats.Average)
	}
	return hikarinagiPageMetadata{
		Company: strings.Join(utils.UniqueNonEmptyStrings(developers), " / "),
		Rating:  rating,
	}
}

func (h HikarinagiInfoGetter) convertToMetadataResult(data hikarinagiGame) MetadataResult {
	name := strings.TrimSpace(data.OriginTitle)
	if data.TransTitle != nil && strings.TrimSpace(*data.TransTitle) != "" {
		name = strings.TrimSpace(*data.TransTitle)
	}
	titleVariants := []string{data.OriginTitle}
	if data.TransTitle != nil {
		titleVariants = append(titleVariants, *data.TransTitle)
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
		Aliases:        normalizeMetadataAliases(name, titleVariants, data.Aliases),
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
