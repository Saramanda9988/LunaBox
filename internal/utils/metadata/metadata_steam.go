package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/version"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SteamInfoGetter 获取 Steam 商店元数据。
type SteamInfoGetter struct {
	client         *http.Client
	preferredLangs []string
	countryCode    string
	tagLimit       int
	tagCatalog     *steamTagCatalogCache
}

func NewSteamInfoGetter(options ...GetterOption) *SteamInfoGetter {
	return NewSteamInfoGetterWithLanguage("", options...)
}

func NewSteamInfoGetterWithLanguage(language string, options ...GetterOption) *SteamInfoGetter {
	langs, countryCode := buildSteamLanguagePreference(language)
	config := newGetterConfig(options)
	return &SteamInfoGetter{
		client:         config.client,
		preferredLangs: langs,
		countryCode:    countryCode,
		tagLimit:       config.tagLimit,
		tagCatalog:     sharedSteamTagCatalog,
	}
}

var _ Getter = (*SteamInfoGetter)(nil)

const (
	steamAppDetailsAPIURL  = "https://store.steampowered.com/api/appdetails"
	steamStoreSearchAPI    = "https://store.steampowered.com/api/storesearch/"
	steamAppReviewsAPIURL  = "https://store.steampowered.com/appreviews/%d"
	steamStoreBrowseAPIURL = "https://api.steampowered.com/IStoreBrowseService/GetItems/v1/"
	steamPopularTagsAPIURL = "https://api.steampowered.com/IStoreService/GetMostPopularTags/v1/"
	steamStoreAppURL       = "https://store.steampowered.com/app/%d/"
	steamAppAssetsBaseURL  = "https://cdn.akamai.steamstatic.com/steam/apps/%d"
	steamCoverProbeTimeout = 3 * time.Second
	steamTagCatalogTTL     = 6 * time.Hour
	steamMaxCommunityTags  = 20
)

var steamReleaseDateRegex = regexp.MustCompile(`(\d{4})\D+(\d{1,2})\D+(\d{1,2})`)
var sharedSteamTagCatalog = newSteamTagCatalogCache()

type steamGenre struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type steamReleaseDate struct {
	ComingSoon bool   `json:"coming_soon"`
	Date       string `json:"date"`
}

type steamMetacritic struct {
	Score int    `json:"score"`
	URL   string `json:"url"`
}

type steamAppData struct {
	SteamAppID       int              `json:"steam_appid"`
	Name             string           `json:"name"`
	HeaderImage      string           `json:"header_image"`
	ShortDescription string           `json:"short_description"`
	ReleaseDate      steamReleaseDate `json:"release_date"`
	Metacritic       steamMetacritic  `json:"metacritic"`
	Developers       []string         `json:"developers"`
	Genres           []steamGenre     `json:"genres"`
}

type steamAppDetailResult struct {
	Success bool         `json:"success"`
	Data    steamAppData `json:"data"`
}

type steamStoreSearchResp struct {
	Total int               `json:"total"`
	Items []steamSearchItem `json:"items"`
}

type steamSearchItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type steamReviewQuerySummary struct {
	TotalPositive int `json:"total_positive"`
	TotalNegative int `json:"total_negative"`
}

type steamReviewResponse struct {
	Success      int                     `json:"success"`
	QuerySummary steamReviewQuerySummary `json:"query_summary"`
}

type steamStoreItemID struct {
	AppID int `json:"appid"`
}

type steamStoreBrowseContext struct {
	Language    string `json:"language"`
	CountryCode string `json:"country_code"`
	SteamRealm  int    `json:"steam_realm"`
}

type steamStoreBrowseDataRequest struct {
	IncludeTagCount int `json:"include_tag_count"`
}

type steamStoreBrowseRequest struct {
	IDs         []steamStoreItemID          `json:"ids"`
	Context     steamStoreBrowseContext     `json:"context"`
	DataRequest steamStoreBrowseDataRequest `json:"data_request"`
}

type steamWeightedTag struct {
	TagID  int `json:"tagid"`
	Weight int `json:"weight"`
}

type steamStoreBrowseItem struct {
	AppID   int                `json:"appid"`
	Success int                `json:"success"`
	Visible bool               `json:"visible"`
	Tags    []steamWeightedTag `json:"tags"`
}

type steamStoreBrowseResponse struct {
	Response struct {
		StoreItems []steamStoreBrowseItem `json:"store_items"`
	} `json:"response"`
}

type steamPopularTag struct {
	TagID int    `json:"tagid"`
	Name  string `json:"name"`
}

type steamPopularTagsResponse struct {
	Response struct {
		Tags []steamPopularTag `json:"tags"`
	} `json:"response"`
}

type steamTagCatalogEntry struct {
	names     map[int]string
	fetchedAt time.Time
}

type steamTagCatalogCache struct {
	mu      sync.RWMutex
	entries map[string]steamTagCatalogEntry
}

func newSteamTagCatalogCache() *steamTagCatalogCache {
	return &steamTagCatalogCache{entries: make(map[string]steamTagCatalogEntry)}
}

func (c *steamTagCatalogCache) get(language string) (map[int]string, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[language]
	if !ok || time.Since(entry.fetchedAt) > steamTagCatalogTTL {
		return nil, false
	}
	return entry.names, true
}

func (c *steamTagCatalogCache) set(language string, names map[int]string) {
	if c == nil || len(names) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[language] = steamTagCatalogEntry{
		names:     names,
		fetchedAt: time.Now(),
	}
}

func (s SteamInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	appID, err := normalizeSteamAppID(id)
	if err != nil {
		return MetadataResult{}, err
	}
	return s.fetchByAppID(appID)
}

func (s SteamInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return MetadataResult{}, errors.New("steam search name is empty")
	}

	var lastErr error
	for _, lang := range s.preferredLangs {
		items, err := s.searchByName(keyword, lang)
		if err != nil {
			lastErr = err
			continue
		}
		if len(items) == 0 {
			continue
		}

		best := pickBestSteamSearchItem(items, keyword)
		if best.ID <= 0 {
			continue
		}

		result, err := s.fetchByAppIDAndLang(best.ID, lang)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return MetadataResult{}, lastErr
	}
	return MetadataResult{}, errors.New("no results found")
}

func (s SteamInfoGetter) fetchByAppID(appID int) (MetadataResult, error) {
	var lastErr error
	for _, lang := range s.preferredLangs {
		result, err := s.fetchByAppIDAndLang(appID, lang)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return MetadataResult{}, lastErr
	}

	return MetadataResult{}, errors.New("no results found")
}

func (s SteamInfoGetter) fetchByAppIDAndLang(appID int, lang string) (MetadataResult, error) {
	params := url.Values{}
	params.Add("appids", strconv.Itoa(appID))
	params.Add("l", lang)
	params.Add("cc", s.countryCode)

	reqURL := fmt.Sprintf("%s?%s", steamAppDetailsAPIURL, params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return MetadataResult{}, err
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := doLimitedMetadataRequest(s.client, req, enums.Steam)
	if err != nil {
		return MetadataResult{}, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return MetadataResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return MetadataResult{}, fmt.Errorf("steam appdetails API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var payload map[string]steamAppDetailResult
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return MetadataResult{}, err
	}

	key := strconv.Itoa(appID)
	data, ok := payload[key]
	if !ok || !data.Success {
		return MetadataResult{}, fmt.Errorf("steam appdetails API returned no data for app id: %d", appID)
	}

	if strings.TrimSpace(data.Data.Name) == "" {
		return MetadataResult{}, fmt.Errorf("steam appdetails API returned empty game name for app id: %d", appID)
	}

	rating := 0.0
	if data.Data.Metacritic.Score > 0 {
		rating = float64(data.Data.Metacritic.Score) / 10.0
	}
	// Metacritic 为空时，回退到 Steam 评测正负比评分
	if rating <= 0 {
		if reviewRating, reviewErr := s.fetchReviewRating(appID); reviewErr == nil {
			rating = reviewRating
		}
	}
	rating = normalizeTenPointRating(rating)
	coverURL := s.resolveSteamCoverURL(appID, lang, data.Data.HeaderImage)
	tags, err := s.fetchCommunityTags(appID, lang)
	if err != nil || len(tags) == 0 {
		tags = extractSteamGenreTags(data.Data.Genres, s.tagLimit)
	}

	game := models.Game{
		Name:           strings.TrimSpace(data.Data.Name),
		CoverURL:       coverURL,
		CoverSourceURL: coverURL,
		Company:        strings.Join(data.Data.Developers, ", "),
		Summary:        strings.TrimSpace(data.Data.ShortDescription),
		Rating:         rating,
		ReleaseDate:    normalizeSteamReleaseDate(data.Data.ReleaseDate.Date),
		SourceType:     enums.Steam,
		SourceID:       key,
		CachedAt:       time.Now(),
	}

	return MetadataResult{
		Game: game,
		Tags: tags,
	}, nil
}

func (s SteamInfoGetter) resolveSteamCoverURL(appID int, lang string, headerImage string) string {
	ctx, cancel := context.WithTimeout(context.Background(), steamCoverProbeTimeout)
	defer cancel()

	for _, candidate := range buildSteamPortraitCoverURLs(appID, lang) {
		if steamCoverURLAvailable(ctx, s.client, candidate) {
			return candidate
		}
	}
	return strings.TrimSpace(headerImage)
}

func buildSteamPortraitCoverURLs(appID int, lang string) []string {
	if appID <= 0 {
		return nil
	}

	baseURL := fmt.Sprintf(steamAppAssetsBaseURL, appID)
	result := make([]string, 0, 3)
	add := func(fileName string) {
		if fileName == "" {
			return
		}
		candidate := baseURL + "/" + fileName
		for _, existing := range result {
			if existing == candidate {
				return
			}
		}
		result = append(result, candidate)
	}

	language := normalizeSteamAssetLanguage(lang)
	if language != "" {
		add("library_600x900_" + language + ".jpg")
	}
	add("library_600x900.jpg")
	if language != "english" {
		add("library_600x900_english.jpg")
	}
	return result
}

func normalizeSteamAssetLanguage(lang string) string {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	switch normalized {
	case "english", "schinese", "tchinese", "japanese", "koreana", "russian":
		return normalized
	default:
		return ""
	}
}

func steamCoverURLAvailable(ctx context.Context, client *http.Client, candidate string) bool {
	if client == nil || strings.TrimSpace(candidate) == "" {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, candidate, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	return contentType == "" || strings.HasPrefix(contentType, "image/")
}

func (s SteamInfoGetter) searchByName(keyword string, lang string) ([]steamSearchItem, error) {
	params := url.Values{}
	params.Add("term", keyword)
	params.Add("l", lang)
	params.Add("cc", s.countryCode)

	reqURL := fmt.Sprintf("%s?%s", steamStoreSearchAPI, params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := doLimitedMetadataRequest(s.client, req, enums.Steam)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam storesearch API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp steamStoreSearchResp
	if err := json.Unmarshal(bodyBytes, &searchResp); err != nil {
		return nil, err
	}

	return searchResp.Items, nil
}

func pickBestSteamSearchItem(items []steamSearchItem, query string) steamSearchItem {
	if len(items) == 0 {
		return steamSearchItem{}
	}

	normalizedQuery := normalizeSteamSearchText(query)
	best := items[0]
	bestScore := -1

	for _, item := range items {
		score := 0
		name := normalizeSteamSearchText(item.Name)

		if normalizedQuery != "" && name == normalizedQuery {
			score += 100
		}
		if normalizedQuery != "" && strings.HasPrefix(name, normalizedQuery) {
			score += 40
		}
		if normalizedQuery != "" && strings.Contains(name, normalizedQuery) {
			score += 20
		}

		if score > bestScore {
			bestScore = score
			best = item
		}
	}

	return best
}

func (s SteamInfoGetter) fetchCommunityTags(appID int, lang string) ([]TagItem, error) {
	if s.tagLimit == 0 {
		return nil, nil
	}

	apiTags, apiErr := s.fetchCommunityTagsFromAPI(appID, lang)
	if len(apiTags) > 0 {
		return apiTags, nil
	}

	pageTags, pageErr := s.fetchCommunityTagsFromStorePage(appID, lang)
	if len(pageTags) > 0 {
		return pageTags, nil
	}

	switch {
	case apiErr != nil && pageErr != nil:
		return nil, fmt.Errorf("steam community tag requests failed: API: %v; store page: %w", apiErr, pageErr)
	case apiErr != nil:
		return nil, apiErr
	default:
		return nil, pageErr
	}
}

func (s SteamInfoGetter) fetchCommunityTagsFromAPI(appID int, lang string) ([]TagItem, error) {
	weightedTags, err := s.fetchCommunityTagWeights(appID, lang)
	if err != nil {
		return nil, err
	}
	if len(weightedTags) == 0 {
		return nil, nil
	}

	tagNames, err := s.fetchCommunityTagCatalog(lang)
	if err != nil {
		return nil, err
	}

	return buildSteamWeightedTagItems(weightedTags, tagNames, s.tagLimit)
}

func (s SteamInfoGetter) fetchCommunityTagWeights(appID int, lang string) ([]steamWeightedTag, error) {
	includeTagCount := s.tagLimit
	if includeTagCount < 0 || includeTagCount > steamMaxCommunityTags {
		includeTagCount = steamMaxCommunityTags
	}

	input := steamStoreBrowseRequest{
		IDs: []steamStoreItemID{{AppID: appID}},
		Context: steamStoreBrowseContext{
			Language:    lang,
			CountryCode: s.countryCode,
			SteamRealm:  1,
		},
		DataRequest: steamStoreBrowseDataRequest{
			IncludeTagCount: includeTagCount,
		},
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("input_json", string(inputJSON))
	reqURL := steamStoreBrowseAPIURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := doLimitedMetadataRequest(s.client, req, enums.Steam)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam store browse API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var payload steamStoreBrowseResponse
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, err
	}
	if len(payload.Response.StoreItems) == 0 {
		return nil, fmt.Errorf("steam store browse API returned no item for app id: %d", appID)
	}

	item := payload.Response.StoreItems[0]
	if item.Success != 1 || item.AppID != appID {
		return nil, fmt.Errorf("steam store browse API returned unsuccessful item for app id: %d", appID)
	}
	return item.Tags, nil
}

func (s SteamInfoGetter) fetchCommunityTagCatalog(lang string) (map[int]string, error) {
	if names, ok := s.tagCatalog.get(lang); ok {
		return names, nil
	}

	params := url.Values{}
	params.Set("language", lang)
	reqURL := steamPopularTagsAPIURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := doLimitedMetadataRequest(s.client, req, enums.Steam)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam popular tags API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var payload steamPopularTagsResponse
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, err
	}

	names := make(map[int]string, len(payload.Response.Tags))
	for _, tag := range payload.Response.Tags {
		name := strings.TrimSpace(tag.Name)
		if tag.TagID > 0 && name != "" {
			names[tag.TagID] = name
		}
	}
	if len(names) == 0 {
		return nil, errors.New("steam popular tags API returned an empty tag catalog")
	}

	s.tagCatalog.set(lang, names)
	return names, nil
}

func (s SteamInfoGetter) fetchCommunityTagsFromStorePage(appID int, lang string) ([]TagItem, error) {
	params := url.Values{}
	params.Add("l", lang)
	params.Add("cc", s.countryCode)

	reqURL := fmt.Sprintf(steamStoreAppURL, appID) + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Cookie", "birthtime=568022401; lastagecheckage=1-January-1988; wants_mature_content=1")

	resp, err := doLimitedMetadataRequest(s.client, req, enums.Steam)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam store page returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	seen := make(map[string]struct{})
	doc.Find(".glance_tags.popular_tags a.app_tag").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		name := strings.TrimSpace(selection.Text())
		if name == "" {
			return true
		}

		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
		names = append(names, name)
		return !hasReachedTagLimit(len(names), s.tagLimit)
	})

	return buildSteamTagItems(names, s.tagLimit), nil
}

func buildSteamWeightedTagItems(weightedTags []steamWeightedTag, names map[int]string, limit int) ([]TagItem, error) {
	if limit == 0 || len(weightedTags) == 0 {
		return nil, nil
	}

	maxWeight := 0
	for _, tag := range weightedTags {
		if tag.Weight > maxWeight {
			maxWeight = tag.Weight
		}
	}

	result := make([]TagItem, 0, tagItemsCapacity(len(weightedTags), limit))
	for _, tag := range weightedTags {
		name := strings.TrimSpace(names[tag.TagID])
		if name == "" {
			return nil, fmt.Errorf("steam tag catalog does not contain tag id: %d", tag.TagID)
		}

		weight := 1.0
		if maxWeight > 0 && tag.Weight > 0 {
			weight = float64(tag.Weight) / float64(maxWeight)
		}
		result = append(result, TagItem{
			Name:      name,
			Source:    "steam",
			Weight:    weight,
			IsSpoiler: false,
		})
		if hasReachedTagLimit(len(result), limit) {
			break
		}
	}
	return result, nil
}

func extractSteamGenreTags(genres []steamGenre, limit int) []TagItem {
	names := make([]string, 0, tagItemsCapacity(len(genres), limit))
	for _, genre := range genres {
		name := strings.TrimSpace(genre.Description)
		if name != "" {
			names = append(names, name)
		}
	}
	return buildSteamTagItems(names, limit)
}

func buildSteamTagItems(names []string, limit int) []TagItem {
	if limit == 0 || len(names) == 0 {
		return nil
	}

	result := make([]TagItem, 0, tagItemsCapacity(len(names), limit))
	seen := make(map[string]struct{}, len(names))
	total := float64(len(names))
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		weight := 1.0 - float64(len(result))/total
		if weight < 0.3 {
			weight = 0.3
		}
		result = append(result, TagItem{
			Name:      name,
			Source:    "steam",
			Weight:    weight,
			IsSpoiler: false,
		})
		if hasReachedTagLimit(len(result), limit) {
			break
		}
	}
	return result
}

func buildSteamLanguagePreference(language string) ([]string, string) {
	normalized := strings.ToLower(strings.TrimSpace(language))
	normalized = strings.ReplaceAll(normalized, "_", "-")

	langs := make([]string, 0, 4)
	add := func(lang string) {
		if lang == "" {
			return
		}
		for _, existing := range langs {
			if existing == lang {
				return
			}
		}
		langs = append(langs, lang)
	}

	countryCode := "US"

	switch {
	case normalized == "", normalized == "en", strings.HasPrefix(normalized, "en-"):
		add("english")
	case normalized == "zh", strings.HasPrefix(normalized, "zh-cn"), strings.HasPrefix(normalized, "zh-hans"):
		add("schinese")
		add("tchinese")
		countryCode = "CN"
	case strings.HasPrefix(normalized, "zh-tw"), strings.HasPrefix(normalized, "zh-hk"), strings.HasPrefix(normalized, "zh-hant"):
		add("tchinese")
		add("schinese")
		countryCode = "TW"
	case normalized == "ja", strings.HasPrefix(normalized, "ja-"):
		add("japanese")
		countryCode = "JP"
	case normalized == "ko", strings.HasPrefix(normalized, "ko-"):
		add("koreana")
		countryCode = "KR"
	case normalized == "ru", strings.HasPrefix(normalized, "ru-"):
		add("russian")
		countryCode = "RU"
	default:
		add("english")
	}

	add("english")
	return langs, countryCode
}

func normalizeSteamSearchText(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(
		"-", " ",
		"_", " ",
		":", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"'", " ",
		"\"", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func normalizeSteamAppID(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("steam app id is empty")
	}

	lower := strings.ToLower(value)
	lower = strings.TrimPrefix(lower, "steam://rungameid/")
	lower = strings.TrimPrefix(lower, "https://store.steampowered.com/app/")
	lower = strings.TrimPrefix(lower, "http://store.steampowered.com/app/")
	lower = strings.TrimPrefix(lower, "app/")

	digits := extractLeadingDigits(lower)
	if digits == "" {
		return 0, fmt.Errorf("invalid Steam app id format: %s", raw)
	}

	appID, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("invalid Steam app id: %w", err)
	}
	if appID <= 0 {
		return 0, fmt.Errorf("invalid Steam app id value: %d", appID)
	}
	return appID, nil
}

func extractLeadingDigits(value string) string {
	start := -1
	for i := 0; i < len(value); i++ {
		if value[i] >= '0' && value[i] <= '9' {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}

	end := start
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	return value[start:end]
}

func (s SteamInfoGetter) fetchReviewRating(appID int) (float64, error) {
	params := url.Values{}
	params.Add("json", "1")
	params.Add("language", "all")
	params.Add("purchase_type", "all")
	params.Add("num_per_page", "0")
	params.Add("filter", "summary")

	reqURL := fmt.Sprintf("%s?%s", fmt.Sprintf(steamAppReviewsAPIURL, appID), params.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := doLimitedMetadataRequest(s.client, req, enums.Steam)
	if err != nil {
		return 0, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("steam appreviews API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var reviewResp steamReviewResponse
	if err := json.Unmarshal(bodyBytes, &reviewResp); err != nil {
		return 0, err
	}
	if reviewResp.Success != 1 {
		return 0, fmt.Errorf("steam appreviews API returned unsuccessful payload for app id: %d", appID)
	}

	total := reviewResp.QuerySummary.TotalPositive + reviewResp.QuerySummary.TotalNegative
	if total <= 0 {
		return 0, nil
	}

	positiveRatio := float64(reviewResp.QuerySummary.TotalPositive) / float64(total)
	return normalizeTenPointRating(positiveRatio * 10.0), nil
}

func normalizeSteamReleaseDate(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	// 优先处理类似 "2025 年 7 月 18 日" / "2025年7月18日" / "2025/7/18"
	if m := steamReleaseDateRegex.FindStringSubmatch(text); len(m) == 4 {
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		if normalized, ok := buildISODate(year, month, day); ok {
			return normalized
		}
	}

	replaced := strings.NewReplacer(
		"年", "-",
		"月", "-",
		"日", "",
		".", "-",
		"/", "-",
		"，", ",",
	).Replace(text)
	replaced = strings.Join(strings.Fields(replaced), " ")

	layouts := []string{
		"2006-1-2",
		"2006-01-02",
		"2 Jan, 2006",
		"Jan 2, 2006",
		"2 Jan 2006",
		"Jan 2 2006",
		"2 January, 2006",
		"January 2, 2006",
		"2 January 2006",
		"January 2 2006",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, replaced); err == nil {
			return parsed.Format("2006-01-02")
		}
	}

	// 解析失败时保留原始值，避免丢数据
	return text
}

func buildISODate(year int, month int, day int) (string, bool) {
	if year < 1900 || year > 3000 || month < 1 || month > 12 || day < 1 || day > 31 {
		return "", false
	}
	dt := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if dt.Year() != year || int(dt.Month()) != month || dt.Day() != day {
		return "", false
	}
	return dt.Format("2006-01-02"), true
}
