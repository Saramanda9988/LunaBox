package metadata

import (
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

	"github.com/PuerkitoBio/goquery"
)

type DLsiteInfoGetter struct {
	client   *http.Client
	tagLimit int
}

func NewDLsiteInfoGetter(options ...GetterOption) *DLsiteInfoGetter {
	config := newGetterConfig(options)
	return &DLsiteInfoGetter{
		client:   config.client,
		tagLimit: config.tagLimit,
	}
}

var _ Getter = (*DLsiteInfoGetter)(nil)

const (
	dlsiteManiaxWorkURL = "https://www.dlsite.com/maniax/work/=/product_id/%s.html"
	dlsiteProWorkURL    = "https://www.dlsite.com/pro/work/=/product_id/%s.html"
	dlsiteSearchURL     = "https://www.dlsite.com/maniax/fsr/=/language/jp/keyword/%s/"
)

var dlsiteIDRegex = regexp.MustCompile(`(?i)\b(RJ|RE|VJ)\d{4,}\b`)

type dlsiteSearchItem struct {
	ID   string
	Name string
}

func (d DLsiteInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	normalizedID, ok := NormalizeDLsiteID(id)
	if !ok {
		return MetadataResult{}, fmt.Errorf("invalid DLsite ID format: %s", id)
	}

	doc, err := d.fetchDocument(buildDLsiteWorkURL(normalizedID))
	if err != nil {
		return MetadataResult{}, err
	}
	return parseDLsiteMetadataDocumentWithTagLimit(doc, normalizedID, d.tagLimit)
}

func (d DLsiteInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return MetadataResult{}, errors.New("dlsite search name is empty")
	}

	items, err := d.searchByName(keyword)
	if err != nil {
		return MetadataResult{}, err
	}
	if len(items) == 0 {
		return MetadataResult{}, errors.New("no results found")
	}

	best := pickBestDLsiteSearchItem(items, keyword)
	if best.ID == "" {
		return MetadataResult{}, errors.New("no results found")
	}

	return d.FetchMetadata(best.ID, "")
}

func NormalizeDLsiteID(raw string) (string, bool) {
	match := dlsiteIDRegex.FindString(strings.TrimSpace(raw))
	if match == "" {
		return "", false
	}
	return strings.ToUpper(match), true
}

func buildDLsiteWorkURL(id string) string {
	if strings.HasPrefix(id, "VJ") {
		return fmt.Sprintf(dlsiteProWorkURL, id)
	}
	return fmt.Sprintf(dlsiteManiaxWorkURL, id)
}

func (d DLsiteInfoGetter) searchByName(keyword string) ([]dlsiteSearchItem, error) {
	escaped := strings.ReplaceAll(url.PathEscape(keyword), "%20", "+")
	doc, err := d.fetchDocument(fmt.Sprintf(dlsiteSearchURL, escaped))
	if err != nil {
		return nil, err
	}

	items := make([]dlsiteSearchItem, 0)
	doc.Find(".search_result_img_box_inner").Each(func(_ int, selection *goquery.Selection) {
		id := strings.TrimSpace(selection.AttrOr("data-list_item_product_id", ""))
		if id == "" {
			href := selection.Find("a.work_thumb_inner").First().AttrOr("href", "")
			if normalizedID, ok := NormalizeDLsiteID(href); ok {
				id = normalizedID
			}
		}
		normalizedID, ok := NormalizeDLsiteID(id)
		if !ok {
			return
		}

		nameLink := selection.Find(".work_name a").First()
		name := strings.TrimSpace(nameLink.AttrOr("title", ""))
		if name == "" {
			name = cleanMetadataText(nameLink.Text())
		}
		if name == "" {
			return
		}

		items = append(items, dlsiteSearchItem{ID: normalizedID, Name: name})
	})

	return items, nil
}

func (d DLsiteInfoGetter) fetchDocument(reqURL string) (*goquery.Document, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja,en;q=0.8")
	req.Header.Set("Cookie", "adultchecked=1; locale=ja")

	resp, err := doLimitedMetadataRequest(d.client, req, MetadataSourceDLsite)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dlsite returned status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func parseDLsiteMetadataDocument(doc *goquery.Document, sourceID string) (MetadataResult, error) {
	return parseDLsiteMetadataDocumentWithTagLimit(doc, sourceID, defaultMetadataTagLimit)
}

func parseDLsiteMetadataDocumentWithTagLimit(doc *goquery.Document, sourceID string, tagLimit int) (MetadataResult, error) {
	title := cleanMetadataText(doc.Find("#work_name").First().Text())
	if title == "" {
		return MetadataResult{}, fmt.Errorf("dlsite page returned empty game name for id: %s", sourceID)
	}

	game := models.Game{
		Name:        title,
		CoverURL:    extractDLsiteCoverURL(doc),
		Company:     cleanMetadataText(doc.Find(".maker_name a").First().Text()),
		Summary:     cleanMetadataText(doc.Find(`[itemprop="description"]`).First().Text()),
		ReleaseDate: extractDLsiteReleaseDate(doc),
		SourceType:  enums.DLsite,
		SourceID:    sourceID,
		CachedAt:    time.Now(),
	}

	return MetadataResult{Game: game, Tags: extractDLsiteTags(doc, tagLimit)}, nil
}

func extractDLsiteReleaseDate(doc *goquery.Document) string {
	var releaseDate string
	doc.Find("th").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		label := cleanMetadataText(selection.Text())
		if strings.Contains(label, "販売日") || strings.Contains(label, "発売日") || strings.Contains(strings.ToLower(label), "release") {
			releaseDate = normalizeDLsiteDate(selection.NextFiltered("td").Text())
			return false
		}
		return true
	})
	return releaseDate
}

func extractDLsiteCoverURL(doc *goquery.Document) string {
	var fallback string
	var cover string
	doc.Find("img, source").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		candidates := []string{
			selection.AttrOr("data-src", ""),
			firstSrcSetURL(selection.AttrOr("srcset", "")),
			selection.AttrOr("src", ""),
		}
		for _, candidate := range candidates {
			normalized := normalizeSourceURL(candidate, "https://www.dlsite.com")
			if normalized == "" {
				continue
			}
			if fallback == "" && strings.Contains(normalized, "_img_smp") {
				fallback = normalized
			}
			if strings.Contains(normalized, "_img_main") {
				cover = normalized
				return false
			}
		}
		return true
	})
	if cover != "" {
		return cover
	}
	return fallback
}

func extractDLsiteTags(doc *goquery.Document, limit int) []TagItem {
	labels := []string{"ジャンル", "作品形式", "販売形式", "年齢指定"}
	names := make([]string, 0, 16)

	doc.Find(".main_genre a").Each(func(_ int, selection *goquery.Selection) {
		names = append(names, cleanMetadataText(selection.Text()))
	})
	doc.Find("th").Each(func(_ int, selection *goquery.Selection) {
		label := cleanMetadataText(selection.Text())
		for _, target := range labels {
			if strings.Contains(label, target) {
				selection.NextFiltered("td").Find("a").Each(func(_ int, link *goquery.Selection) {
					names = append(names, cleanMetadataText(link.Text()))
				})
				return
			}
		}
	})

	return buildTagItems(names, "dlsite", limit)
}

func normalizeDLsiteDate(raw string) string {
	return normalizeJapaneseDate(raw)
}

func pickBestDLsiteSearchItem(items []dlsiteSearchItem, query string) dlsiteSearchItem {
	if len(items) == 0 {
		return dlsiteSearchItem{}
	}

	normalizedQuery := normalizeSteamSearchText(query)
	best := items[0]
	bestScore := -1

	for _, item := range items {
		name := normalizeSteamSearchText(item.Name)
		score := 0
		if normalizedQuery != "" && name == normalizedQuery {
			score += 100
		}
		if normalizedQuery != "" && strings.Contains(name, normalizedQuery) {
			score += 20
		}
		if score > bestScore {
			best = item
			bestScore = score
		}
	}

	return best
}

func firstSrcSetURL(srcset string) string {
	first := strings.TrimSpace(strings.Split(srcset, ",")[0])
	if first == "" {
		return ""
	}
	return strings.Fields(first)[0]
}

func normalizeSourceURL(raw string, baseURL string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return strings.TrimRight(baseURL, "/") + value
	}
	return value
}

func cleanMetadataText(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func normalizeJapaneseDate(raw string) string {
	text := cleanMetadataText(raw)
	if text == "" {
		return ""
	}

	replaced := strings.NewReplacer(
		"年", "-",
		"月", "-",
		"日", "",
		".", "-",
		"/", "-",
	).Replace(text)
	replaced = strings.TrimSpace(replaced)

	parts := strings.Split(replaced, "-")
	if len(parts) >= 3 {
		year, yErr := strconv.Atoi(strings.TrimSpace(parts[0]))
		month, mErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		day, dErr := strconv.Atoi(strings.TrimSpace(parts[2]))
		if yErr == nil && mErr == nil && dErr == nil {
			if normalized, ok := buildISODate(year, month, day); ok {
				return normalized
			}
		}
	}

	for _, layout := range []string{"2006-01-02", "2006-1-2"} {
		if parsed, err := time.Parse(layout, replaced); err == nil {
			return parsed.Format("2006-01-02")
		}
	}

	return text
}

func buildTagItems(names []string, source string, limit int) []TagItem {
	if limit == 0 {
		return nil
	}

	result := make([]TagItem, 0, tagItemsCapacity(len(names), limit))
	seen := make(map[string]struct{}, len(names))
	for _, raw := range names {
		name := cleanMetadataText(raw)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		weight := 1.0 - float64(len(result))*0.03
		if weight < 0.5 {
			weight = 0.5
		}
		result = append(result, TagItem{
			Name:      name,
			Source:    source,
			Weight:    weight,
			IsSpoiler: false,
		})
		if hasReachedTagLimit(len(result), limit) {
			break
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
