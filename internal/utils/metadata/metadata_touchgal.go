package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/version"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TouchGalInfoGetter struct {
	client   *http.Client
	tagLimit int
}

func NewTouchGalInfoGetter(options ...GetterOption) *TouchGalInfoGetter {
	config := newGetterConfig(options)
	return &TouchGalInfoGetter{
		client:   config.client,
		tagLimit: config.tagLimit,
	}
}

var _ Getter = (*TouchGalInfoGetter)(nil)

const (
	touchGalAPIBaseURL   = "https://developer.touchgal.com/api/v1"
	touchGalSearchLimit  = "5"
	touchGalAllowNSFW    = "true"
	touchGalUniqueIDSize = 8
)

type touchGalErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type touchGalSearchResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Items []touchGalSearchItem `json:"items"`
	} `json:"data"`
	Error touchGalErrorBody `json:"error"`
}

type touchGalSearchItem struct {
	Name     string `json:"name"`
	UniqueID string `json:"uniqueId"`
}

type touchGalDetailResponse struct {
	Success bool              `json:"success"`
	Data    touchGalGameData  `json:"data"`
	Error   touchGalErrorBody `json:"error"`
}

type touchGalGameData struct {
	UniqueID     string            `json:"uniqueId"`
	Name         string            `json:"name"`
	Aliases      []string          `json:"aliases"`
	Introduction string            `json:"introduction"`
	BannerURL    string            `json:"bannerUrl"`
	Type         []string          `json:"type"`
	Platform     []string          `json:"platform"`
	Language     []string          `json:"language"`
	Tags         []string          `json:"tags"`
	PublishTime  string            `json:"publishTime"`
	ReleaseDate  string            `json:"releaseDate"`
	UpdatedAt    string            `json:"updatedAt"`
	Companies    []touchGalCompany `json:"companies"`
	Rating       touchGalRating    `json:"rating"`
	TouchGalURL  string            `json:"touchgalUrl"`
}

type touchGalCompany struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type touchGalRating struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

func NormalizeTouchGalID(id string) (string, bool) {
	normalized := strings.TrimSpace(id)
	if len(normalized) != touchGalUniqueIDSize {
		return "", false
	}
	for _, r := range normalized {
		if r >= '0' && r <= '9' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		return "", false
	}
	return normalized, true
}

func (t TouchGalInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	uniqueID, ok := NormalizeTouchGalID(id)
	if !ok {
		return MetadataResult{}, fmt.Errorf("invalid TouchGAL uniqueId format: %s", id)
	}
	token = resolveTouchGalAPIToken(token)
	if token == "" {
		return MetadataResult{}, errors.New("TouchGAL API requires Bearer token")
	}

	reqURL := fmt.Sprintf("%s/games/%s?allowNsfw=%s", touchGalAPIBaseURL, url.PathEscape(uniqueID), touchGalAllowNSFW)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return MetadataResult{}, err
	}
	t.setHeaders(req, token)

	resp, err := doLimitedMetadataRequest(t.client, req, enums.TouchGal)
	if err != nil {
		return MetadataResult{}, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return MetadataResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return MetadataResult{}, fmt.Errorf("TouchGAL API returned status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var detailResp touchGalDetailResponse
	if err := json.Unmarshal(bodyBytes, &detailResp); err != nil {
		return MetadataResult{}, err
	}
	if !detailResp.Success {
		return MetadataResult{}, touchGalAPIError("TouchGAL detail API", detailResp.Error)
	}
	if strings.TrimSpace(detailResp.Data.UniqueID) == "" {
		return MetadataResult{}, fmt.Errorf("TouchGAL API returned no game data, body: %s", strings.TrimSpace(string(bodyBytes)))
	}

	return t.convertToMetadataResult(detailResp.Data), nil
}

func (t TouchGalInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	results, err := t.FetchMetadataCandidatesByName(name, token)
	if err != nil {
		return MetadataResult{}, err
	}
	return results[0], nil
}

func (t TouchGalInfoGetter) FetchMetadataCandidatesByName(name string, token string) ([]MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return nil, errors.New("TouchGAL search keyword is empty")
	}
	token = resolveTouchGalAPIToken(token)
	if token == "" {
		return nil, errors.New("TouchGAL API requires Bearer token")
	}

	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("limit", touchGalSearchLimit)
	params.Set("allowNsfw", touchGalAllowNSFW)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/games/search?%s", touchGalAPIBaseURL, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	t.setHeaders(req, token)

	resp, err := doLimitedMetadataRequest(t.client, req, enums.TouchGal)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TouchGAL search API returned status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var searchResp touchGalSearchResponse
	if err := json.Unmarshal(bodyBytes, &searchResp); err != nil {
		return nil, err
	}
	if !searchResp.Success {
		return nil, touchGalAPIError("TouchGAL search API", searchResp.Error)
	}
	if len(searchResp.Data.Items) == 0 {
		return nil, errors.New("no results found")
	}

	limit := len(searchResp.Data.Items)
	if limit > metadataSearchCandidateLimit {
		limit = metadataSearchCandidateLimit
	}
	candidateNames := make([][]string, limit)
	for index := 0; index < limit; index++ {
		candidateNames[index] = []string{searchResp.Data.Items[index].Name}
	}
	indexes := exactMetadataCandidateIndexes(keyword, candidateNames)
	if len(indexes) == 0 {
		indexes = []int{0}
	}

	results := make([]MetadataResult, 0, len(indexes))
	seenIDs := make(map[string]struct{}, len(indexes))
	var lastErr error
	for _, index := range indexes {
		id := strings.TrimSpace(searchResp.Data.Items[index].UniqueID)
		if id == "" {
			continue
		}
		if _, exists := seenIDs[id]; exists {
			continue
		}
		result, fetchErr := t.FetchMetadata(id, token)
		if fetchErr != nil {
			lastErr = fetchErr
			continue
		}
		seenIDs[id] = struct{}{}
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

func (t TouchGalInfoGetter) setHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")
}

func resolveTouchGalAPIToken(token string) string {
	if token = strings.TrimSpace(token); token != "" {
		return token
	}
	if token = strings.TrimSpace(version.TouchGalAPIToken); token != "" {
		return token
	}
	return ""
}

func (t TouchGalInfoGetter) convertToMetadataResult(data touchGalGameData) MetadataResult {
	name := strings.TrimSpace(data.Name)
	company := ""
	for _, item := range data.Companies {
		if strings.TrimSpace(item.Name) != "" {
			company = strings.TrimSpace(item.Name)
			break
		}
	}

	game := models.Game{
		Name:           name,
		Aliases:        normalizeMetadataAliases(name, data.Aliases),
		CoverURL:       strings.TrimSpace(data.BannerURL),
		CoverSourceURL: strings.TrimSpace(data.BannerURL),
		Company:        company,
		Summary:        strings.TrimSpace(data.Introduction),
		Rating:         normalizeTenPointRating(data.Rating.Average),
		ReleaseDate:    strings.TrimSpace(data.ReleaseDate),
		SourceType:     enums.TouchGal,
		SourceID:       strings.TrimSpace(data.UniqueID),
		CachedAt:       time.Now(),
	}
	return MetadataResult{Game: game, Tags: extractTouchGalTags(data.Tags, t.tagLimit)}
}

func touchGalAPIError(prefix string, apiErr touchGalErrorBody) error {
	message := strings.TrimSpace(apiErr.Message)
	if message == "" {
		message = "unknown error"
	}
	code := strings.TrimSpace(apiErr.Code)
	if code == "" {
		return fmt.Errorf("%s error: %s", prefix, message)
	}
	return fmt.Errorf("%s error %s: %s", prefix, code, message)
}

func extractTouchGalTags(tags []string, limit int) []TagItem {
	if limit == 0 {
		return nil
	}
	result := make([]TagItem, 0, tagItemsCapacity(len(tags), limit))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		name := strings.TrimSpace(tag)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, TagItem{
			Name:      name,
			Source:    string(enums.TouchGal),
			Weight:    1,
			IsSpoiler: false,
		})
		if hasReachedTagLimit(len(result), limit) {
			break
		}
	}
	return result
}
