package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"net/http"
	"strings"
	"time"
)

// VNDBInfoGetter 获取 VNDB 信息。
type VNDBInfoGetter struct {
	client         *http.Client
	preferredLangs []string
	tagLimit       int
}

func NewVNDBInfoGetter(options ...GetterOption) *VNDBInfoGetter {
	config := newGetterConfig(options)
	return &VNDBInfoGetter{
		client:   config.client,
		tagLimit: config.tagLimit,
	}
}

func NewVNDBInfoGetterWithLanguage(language string, options ...GetterOption) *VNDBInfoGetter {
	config := newGetterConfig(options)
	return &VNDBInfoGetter{
		client:         config.client,
		preferredLangs: buildVNDBLanguagePreference(language),
		tagLimit:       config.tagLimit,
	}
}

var _ Getter = (*VNDBInfoGetter)(nil)
var _ BatchGetter = (*VNDBInfoGetter)(nil)

const vndbAPIURL = "https://api.vndb.org/kana/vn"
const vndbSearchSort = "searchrank"
const vndbBatchSize = 100
const vndbFields = "id, title, titles.lang, titles.title, titles.latin, titles.official, titles.main, image.url, description, rating, released, developers.name, tags.name, tags.rating, tags.spoiler"

type vndbRequest struct {
	Filters []interface{} `json:"filters"`
	Fields  string        `json:"fields"`
	Sort    string        `json:"sort,omitempty"`
	Results int           `json:"results,omitempty"`
}

type vndbImage struct {
	URL string `json:"url"`
}

type vndbDeveloper struct {
	Name string `json:"name"`
}

type vndbTag struct {
	Name    string  `json:"name"`
	Rating  float64 `json:"rating"`
	Spoiler int     `json:"spoiler"` // 0=无剧透, 1=轻微, 2=重度
}

type vndbTitle struct {
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Latin    string `json:"latin"`
	Official bool   `json:"official"`
	Main     bool   `json:"main"`
}

type vndbQueryResult struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Titles      []vndbTitle     `json:"titles"`
	Image       vndbImage       `json:"image"`
	Description string          `json:"description"`
	Rating      float64         `json:"rating"`
	Released    string          `json:"released"`
	Developers  []vndbDeveloper `json:"developers"`
	Tags        []vndbTag       `json:"tags"`
}

type vndbResponse struct {
	Results []vndbQueryResult `json:"results"`
}

func (v VNDBInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	return v.queryVNDB([]interface{}{"id", "=", id}, "")
}

func (v VNDBInfoGetter) FetchMetadataBatch(ids []string, token string) (map[string]MetadataResult, error) {
	ids = uniqueTrimmedStrings(ids)
	results := make(map[string]MetadataResult, len(ids))
	if len(ids) == 0 {
		return results, nil
	}

	for start := 0; start < len(ids); start += vndbBatchSize {
		end := start + vndbBatchSize
		if end > len(ids) {
			end = len(ids)
		}

		batchIDs := ids[start:end]
		batchResults, err := v.queryVNDBResults(buildVNDBIDBatchFilters(batchIDs), "", len(batchIDs))
		if err != nil {
			return nil, err
		}
		for _, item := range batchResults {
			id := strings.ToLower(strings.TrimSpace(item.ID))
			if id == "" {
				continue
			}
			results[id] = MetadataResult{
				Game: v.convertResultToGame(item),
				Tags: extractVNDBTags(item.Tags, v.tagLimit),
			}
		}
	}

	return results, nil
}

func (v VNDBInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	return v.queryVNDB([]interface{}{"search", "=", name}, vndbSearchSort)
}

func (v VNDBInfoGetter) queryVNDB(filters []interface{}, sort string) (MetadataResult, error) {
	results, err := v.queryVNDBResults(filters, sort, 1)
	if err != nil {
		return MetadataResult{}, err
	}
	if len(results) == 0 {
		return MetadataResult{}, errors.New("no results found")
	}

	result := results[0]
	return MetadataResult{Game: v.convertResultToGame(result), Tags: extractVNDBTags(result.Tags, v.tagLimit)}, nil
}

func (v VNDBInfoGetter) queryVNDBResults(filters []interface{}, sort string, resultsLimit int) ([]vndbQueryResult, error) {
	reqBody := vndbRequest{
		Filters: filters,
		Fields:  vndbFields,
		Sort:    sort,
		Results: resultsLimit,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", vndbAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doLimitedMetadataRequest(v.client, req, MetadataSourceVNDB)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("VNDB API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var vndbResp vndbResponse
	if err := json.NewDecoder(resp.Body).Decode(&vndbResp); err != nil {
		return nil, err
	}
	return vndbResp.Results, nil
}

func (v VNDBInfoGetter) convertResultToGame(result vndbQueryResult) models.Game {
	displayName := pickVNDBDisplayTitle(result, v.preferredLangs)
	company := ""
	if len(result.Developers) > 0 {
		devs := make([]string, 0, len(result.Developers))
		for _, developer := range result.Developers {
			devs = append(devs, developer.Name)
		}
		company = strings.Join(devs, ", ")
	}

	game := models.Game{
		Name:        displayName,
		CoverURL:    result.Image.URL,
		Company:     company,
		Summary:     result.Description,
		Rating:      normalizeTenPointRating(result.Rating),
		ReleaseDate: strings.TrimSpace(result.Released),
		SourceType:  enums.VNDB,
		SourceID:    result.ID,
		CachedAt:    time.Now(),
	}
	return game
}

func buildVNDBIDBatchFilters(ids []string) []interface{} {
	if len(ids) == 1 {
		return []interface{}{"id", "=", ids[0]}
	}

	filters := make([]interface{}, 0, len(ids)+1)
	filters = append(filters, "or")
	for _, id := range ids {
		filters = append(filters, []interface{}{"id", "=", id})
	}
	return filters
}

func uniqueTrimmedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func pickVNDBDisplayTitle(result vndbQueryResult, preferredLangs []string) string {
	if len(preferredLangs) == 0 {
		return strings.TrimSpace(result.Title)
	}

	for _, lang := range preferredLangs {
		if title := pickVNDBTitleByLang(result.Titles, lang); title != "" {
			return title
		}
	}
	if title := pickVNDBBestTitle(result.Titles); title != "" {
		return title
	}
	return strings.TrimSpace(result.Title)
}

func pickVNDBTitleByLang(titles []vndbTitle, lang string) string {
	target := normalizeVNDBLang(lang)
	if target == "" {
		return ""
	}

	bestScore := -1
	bestTitle := ""
	for _, t := range titles {
		if normalizeVNDBLang(t.Lang) != target {
			continue
		}
		title := firstNonEmpty(strings.TrimSpace(t.Title), strings.TrimSpace(t.Latin))
		if title == "" {
			continue
		}
		score := 0
		if t.Main {
			score += 2
		}
		if t.Official {
			score++
		}
		if score > bestScore {
			bestScore = score
			bestTitle = title
		}
	}
	return bestTitle
}

func pickVNDBBestTitle(titles []vndbTitle) string {
	bestScore := -1
	bestTitle := ""
	for _, t := range titles {
		title := firstNonEmpty(strings.TrimSpace(t.Title), strings.TrimSpace(t.Latin))
		if title == "" {
			continue
		}
		score := 0
		if t.Main {
			score += 2
		}
		if t.Official {
			score++
		}
		if score > bestScore {
			bestScore = score
			bestTitle = title
		}
	}
	return bestTitle
}

func buildVNDBLanguagePreference(language string) []string {
	normalized := normalizeVNDBLang(language)
	if normalized == "" {
		return nil
	}

	prefs := make([]string, 0, 6)
	add := func(lang string) {
		n := normalizeVNDBLang(lang)
		if n == "" {
			return
		}
		for _, existing := range prefs {
			if existing == n {
				return
			}
		}
		prefs = append(prefs, n)
	}

	base := normalized
	if idx := strings.Index(base, "-"); idx > 0 {
		base = base[:idx]
	}

	switch base {
	case "zh":
		if strings.Contains(normalized, "hant") || strings.HasSuffix(normalized, "-tw") || strings.HasSuffix(normalized, "-hk") || strings.HasSuffix(normalized, "-mo") {
			add("zh-hant")
			add("zh-hans")
		} else {
			add("zh-hans")
			add("zh-hant")
		}
		add("zh")
	default:
		add(normalized)
		add(base)
	}

	add("ja")
	add("en")
	return prefs
}

func normalizeVNDBLang(lang string) string {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	return normalized
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// extractVNDBTags 从 VNDB tag 列表中提取 TagItem。
// 规则：保留全部非空 tag，spoiler >= 2 标记为 is_spoiler，按 rating 降序，weight = rating/3.0。
func extractVNDBTags(tags []vndbTag, limit int) []TagItem {
	if limit == 0 {
		return nil
	}

	var filtered []vndbTag
	for _, t := range tags {
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		filtered = append(filtered, t)
	}
	if len(filtered) == 0 {
		return nil
	}

	// 按 rating 降序排序
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[j].Rating > filtered[i].Rating {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	filtered = filtered[:tagItemsCapacity(len(filtered), limit)]

	result := make([]TagItem, 0, len(filtered))
	for _, t := range filtered {
		result = append(result, TagItem{
			Name:      t.Name,
			Source:    "vndb",
			Weight:    t.Rating / 3.0,
			IsSpoiler: t.Spoiler >= 2,
		})
		if hasReachedTagLimit(len(result), limit) {
			break
		}
	}
	return result
}
