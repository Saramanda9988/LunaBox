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

type ErogameScapeInfoGetter struct {
	client  *http.Client
	baseURL string
}

func NewErogameScapeInfoGetter() *ErogameScapeInfoGetter {
	return newErogameScapeInfoGetterWithBaseURL(erogamescapeBaseURL)
}

func newErogameScapeInfoGetterWithBaseURL(baseURL string) *ErogameScapeInfoGetter {
	return &ErogameScapeInfoGetter{
		client:  newMetadataClient(),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
	}
}

var _ Getter = (*ErogameScapeInfoGetter)(nil)

const erogamescapeBaseURL = "https://erogamescape.org/~ap2/ero/toukei_kaiseki"

var erogamescapeGameQueryRegex = regexp.MustCompile(`(?i)(?:\?|&|#|/)game=(\d+)`)

type erogamescapeSearchItem struct {
	ID   string
	Name string
}

func (e ErogameScapeInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	normalizedID, ok := NormalizeErogameScapeID(id)
	if !ok {
		return MetadataResult{}, fmt.Errorf("invalid ErogameScape ID format: %s", id)
	}

	doc, err := e.fetchDocument("/game.php", url.Values{"game": {normalizedID}})
	if err != nil {
		return MetadataResult{}, err
	}
	return parseErogameScapeMetadataDocument(doc, normalizedID)
}

func (e ErogameScapeInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return MetadataResult{}, errors.New("erogamescape search name is empty")
	}

	items, err := e.searchByName(keyword)
	if err != nil {
		return MetadataResult{}, err
	}
	if len(items) == 0 {
		return MetadataResult{}, errors.New("no results found")
	}

	best := pickBestErogameScapeSearchItem(items, keyword)
	if best.ID == "" {
		return MetadataResult{}, errors.New("no results found")
	}

	return e.FetchMetadata(best.ID, "")
}

func NormalizeErogameScapeID(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}

	if match := erogamescapeGameQueryRegex.FindStringSubmatch(value); len(match) == 2 {
		return strings.TrimLeft(match[1], "0"), strings.TrimLeft(match[1], "0") != ""
	}

	if parsedURL, err := url.Parse(value); err == nil {
		if gameID := parsedURL.Query().Get("game"); gameID != "" {
			return normalizeNumericID(gameID)
		}
	}

	return normalizeNumericID(value)
}

func normalizeNumericID(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	normalized := strings.TrimLeft(trimmed, "0")
	if normalized == "" {
		return "", false
	}
	return normalized, true
}

func (e ErogameScapeInfoGetter) searchByName(keyword string) ([]erogamescapeSearchItem, error) {
	doc, err := e.fetchDocument("/kensaku.php", url.Values{
		"category":      {"game"},
		"word_category": {"name"},
		"mode":          {"normal"},
		"word":          {keyword},
	})
	if err != nil {
		return nil, err
	}

	return parseErogameScapeSearchDocument(doc), nil
}

func (e ErogameScapeInfoGetter) fetchDocument(path string, params url.Values) (*goquery.Document, error) {
	baseURL := e.resolvedBaseURL()
	reqURL := baseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ja,en;q=0.8")
	req.Header.Set("Referer", baseURL)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erogamescape returned status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (e ErogameScapeInfoGetter) resolvedBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(e.baseURL), "/")
	if baseURL == "" {
		return erogamescapeBaseURL
	}
	return baseURL
}

func parseErogameScapeSearchDocument(doc *goquery.Document) []erogamescapeSearchItem {
	items := make([]erogamescapeSearchItem, 0)
	positionMap := map[string]int{
		"name":        0,
		"brand":       1,
		"releaseDate": 2,
	}

	doc.Find("#result tr").Each(func(index int, row *goquery.Selection) {
		if index == 0 {
			row.Find("th").Each(func(column int, cell *goquery.Selection) {
				switch cleanMetadataText(cell.Text()) {
				case "ゲーム名":
					positionMap["name"] = column
				case "ブランド名":
					positionMap["brand"] = column
				case "発売日":
					positionMap["releaseDate"] = column
				}
			})
			return
		}

		nameCell := row.Find("td").Eq(positionMap["name"])
		nameLink := nameCell.Find("a").First()
		name := cleanMetadataText(nameLink.Text() + nameCell.Find("span").First().Text())
		id, ok := NormalizeErogameScapeID(nameLink.AttrOr("href", ""))
		if !ok || name == "" {
			return
		}
		items = append(items, erogamescapeSearchItem{ID: id, Name: name})
	})

	return items
}

func parseErogameScapeMetadataDocument(doc *goquery.Document, sourceID string) (MetadataResult, error) {
	title := cleanMetadataText(doc.Find("div#soft-title > span.bold").First().Text())
	if title == "" {
		title = cleanMetadataText(doc.Find("#soft-title .bold").First().Text())
	}
	if title == "" {
		return MetadataResult{}, fmt.Errorf("erogamescape page returned empty game name for id: %s", sourceID)
	}

	game := models.Game{
		Name:        title,
		CoverURL:    normalizeSourceURL(doc.Find("div#main_image img").First().AttrOr("src", ""), erogamescapeBaseURL),
		Company:     cleanMetadataText(doc.Find("tr#brand > td").First().Text()),
		ReleaseDate: normalizeErogameScapeDate(doc.Find("tr#sellday > td").First().Text()),
		Rating:      extractErogameScapeRating(doc),
		SourceType:  enums.ErogameScape,
		SourceID:    sourceID,
		CachedAt:    time.Now(),
	}

	return MetadataResult{Game: game, Tags: extractErogameScapeTags(doc)}, nil
}

func extractErogameScapeRating(doc *goquery.Document) float64 {
	selectors := []string{
		"tr#median > td",
		"tr#average > td",
		"#median",
		"#average",
	}
	for _, selector := range selectors {
		if rating := parseErogameScapeRating(doc.Find(selector).First().Text()); rating > 0 {
			return rating
		}
	}

	var rating float64
	doc.Find("tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		label := cleanMetadataText(row.Find("th").First().Text())
		if strings.Contains(label, "中央値") || strings.Contains(label, "平均値") {
			rating = parseErogameScapeRating(row.Find("td").First().Text())
			return rating <= 0
		}
		return true
	})
	return rating
}

func parseErogameScapeRating(raw string) float64 {
	text := cleanMetadataText(raw)
	if text == "" {
		return 0
	}
	match := regexp.MustCompile(`\d+(?:\.\d+)?`).FindString(text)
	if match == "" {
		return 0
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return normalizeTenPointRating(value)
}

func normalizeErogameScapeDate(raw string) string {
	return normalizeJapaneseDate(raw)
}

func extractErogameScapeTags(doc *goquery.Document) []TagItem {
	names := make([]string, 0, 24)
	if nsfwCell := cleanMetadataText(doc.Find("tr#erogame > td").First().Text()); nsfwCell != "" {
		for _, token := range []string{"18禁", "非18禁", "抜きゲー", "非抜きゲー", "和姦もの", "陵辱もの", "どちらともいえない"} {
			if strings.Contains(nsfwCell, token) {
				names = append(names, token)
			}
		}
	}

	allowedHeaders := map[string]struct{}{
		"公式ジャンル":   {},
		"ジャンル":     {},
		"タグ":       {},
		"シチュエーション": {},
		"エロシーン":    {},
	}
	doc.Find("table#att_pov_table tr").Each(func(_ int, row *goquery.Selection) {
		header := cleanMetadataText(row.Find("th").First().Text())
		if _, ok := allowedHeaders[header]; !ok {
			return
		}
		row.Find("td a").Each(func(_ int, link *goquery.Selection) {
			names = append(names, cleanMetadataText(link.Text()))
		})
	})

	return buildTagItems(names, "erogamescape", 20)
}

func pickBestErogameScapeSearchItem(items []erogamescapeSearchItem, query string) erogamescapeSearchItem {
	if len(items) == 0 {
		return erogamescapeSearchItem{}
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
