package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SteamInfoGetter 获取 Steam 商店元数据。
type SteamInfoGetter struct {
	client         *http.Client
	preferredLangs []string
	countryCode    string
}

func NewSteamInfoGetter() *SteamInfoGetter {
	return NewSteamInfoGetterWithLanguage("")
}

func NewSteamInfoGetterWithLanguage(language string) *SteamInfoGetter {
	langs, countryCode := buildSteamLanguagePreference(language)
	return &SteamInfoGetter{
		client:         newMetadataClient(),
		preferredLangs: langs,
		countryCode:    countryCode,
	}
}

var _ Getter = (*SteamInfoGetter)(nil)

const (
	steamAppDetailsAPIURL = "https://store.steampowered.com/api/appdetails"
	steamStoreSearchAPI   = "https://store.steampowered.com/api/storesearch/"
	steamAppReviewsAPIURL = "https://store.steampowered.com/appreviews/%d"
)

var steamReleaseDateRegex = regexp.MustCompile(`(\d{4})\D+(\d{1,2})\D+(\d{1,2})`)

type steamGenre struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type steamCategory struct {
	ID          int    `json:"id"`
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
	Categories       []steamCategory  `json:"categories"`
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
	req.Header.Set("User-Agent", metadataUserAgent)

	resp, err := s.client.Do(req)
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

	game := models.Game{
		Name:        strings.TrimSpace(data.Data.Name),
		CoverURL:    strings.TrimSpace(data.Data.HeaderImage),
		Company:     strings.Join(data.Data.Developers, ", "),
		Summary:     strings.TrimSpace(data.Data.ShortDescription),
		Rating:      rating,
		ReleaseDate: normalizeSteamReleaseDate(data.Data.ReleaseDate.Date),
		SourceType:  enums.Steam,
		SourceID:    key,
		CachedAt:    time.Now(),
	}

	return MetadataResult{
		Game: game,
		Tags: extractSteamTags(data.Data.Genres, data.Data.Categories),
	}, nil
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
	req.Header.Set("User-Agent", metadataUserAgent)

	resp, err := s.client.Do(req)
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

func extractSteamTags(genres []steamGenre, categories []steamCategory) []TagItem {
	if len(genres) == 0 && len(categories) == 0 {
		return nil
	}

	result := make([]TagItem, 0, len(genres)+len(categories))
	seen := make(map[string]struct{}, len(genres)+len(categories))

	total := float64(len(genres))
	if total <= 0 {
		total = 1
	}

	for i, g := range genres {
		name := strings.TrimSpace(g.Description)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		weight := 1.0 - float64(i)/total
		if weight < 0.3 {
			weight = 0.3
		}

		result = append(result, TagItem{
			Name:      name,
			Source:    "steam",
			Weight:    weight,
			IsSpoiler: false,
		})
		if len(result) >= 15 {
			break
		}
	}

	if len(result) >= 15 {
		return result
	}

	// 某些 Steam 条目没有 genres，使用 categories 作为兜底标签来源。
	categoryWeight := 0.6
	for _, c := range categories {
		name := strings.TrimSpace(c.Description)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		result = append(result, TagItem{
			Name:      name,
			Source:    "steam",
			Weight:    categoryWeight,
			IsSpoiler: false,
		})
		if len(result) >= 15 {
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
	req.Header.Set("User-Agent", metadataUserAgent)

	resp, err := s.client.Do(req)
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
