package metadata

import (
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
)

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
	rating = normalizeTenPointRating(rating)

	game := models.Game{
		Name:        strings.TrimSpace(data.Data.Name),
		CoverURL:    strings.TrimSpace(data.Data.HeaderImage),
		Company:     strings.Join(data.Data.Developers, ", "),
		Summary:     strings.TrimSpace(data.Data.ShortDescription),
		Rating:      rating,
		ReleaseDate: strings.TrimSpace(data.Data.ReleaseDate.Date),
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
