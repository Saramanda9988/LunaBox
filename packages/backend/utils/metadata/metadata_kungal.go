package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/backend/version"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 鲲 Galgame 的目录数据由 NextMoe 开放 API v2 提供，作品 id 与 www.kungal.com 的 galgame id 一致。
const (
	kungalAPIBaseURL = "https://api.nextmoe.dev/v2"
	kungalIncludes   = "titles,intros,companies,ratings,tags"
)

var ErrKungalUnauthorized = errors.New("kungal unauthorized")

var kungalWorkURLPattern = regexp.MustCompile(`(?i)kungal\.com/(?:[a-z-]+/)?galgame/(\d+)`)

func IsKungalUnauthorizedError(err error) bool {
	return errors.Is(err, ErrKungalUnauthorized)
}

type KungalInfoGetter struct {
	client   *http.Client
	ctx      context.Context
	tagLimit int
}

func NewKungalInfoGetter(options ...GetterOption) *KungalInfoGetter {
	config := newGetterConfig(options)
	return &KungalInfoGetter{
		client:   config.client,
		ctx:      config.ctx,
		tagLimit: config.tagLimit,
	}
}

var _ Getter = (*KungalInfoGetter)(nil)
var _ CandidateGetter = (*KungalInfoGetter)(nil)

type kungalLocalizedTitle struct {
	Value string `json:"value"`
}

type kungalTitle struct {
	Lang  string  `json:"lang"`
	Title string  `json:"title"`
	Latin *string `json:"latin"`
}

type kungalIntro struct {
	Lang      string `json:"lang"`
	Value     string `json:"value"`
	IsMachine bool   `json:"is_machine"`
}

type kungalCompany struct {
	DisplayName     string                          `json:"display_name"`
	Localized       map[string]kungalLocalizedTitle `json:"localized"`
	AttributionRole string                          `json:"attribution_role"`
}

type kungalRating struct {
	Source    string   `json:"source"`
	Score     *float64 `json:"score"`
	VoteCount int      `json:"vote_count"`
}

type kungalTag struct {
	DisplayName string `json:"display_name"`
	Tier        string `json:"tier"`
	Spoiler     string `json:"spoiler"`
}

type kungalCover struct {
	URL string `json:"url"`
}

type kungalWork struct {
	ID            string                          `json:"id"`
	DisplayName   string                          `json:"display_name"`
	Latin         *string                         `json:"latin"`
	Localized     map[string]kungalLocalizedTitle `json:"localized"`
	Olang         string                          `json:"olang"`
	ContentRating string                          `json:"content_rating"`
	ReleaseDate   *string                         `json:"release_date"`
	Cover         *kungalCover                    `json:"cover"`
	Titles        []kungalTitle                   `json:"titles"`
	Intros        []kungalIntro                   `json:"intros"`
	Companies     []kungalCompany                 `json:"companies"`
	Ratings       []kungalRating                  `json:"ratings"`
	Tags          []kungalTag                     `json:"tags"`
}

type kungalWorkList struct {
	Items []kungalWork `json:"items"`
}

type kungalProblem struct {
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	Title     string `json:"title"`
	CurrentID string `json:"current_id"`
	RequestID string `json:"request_id"`
}

// NormalizeKungalID 接受纯数字 id 或 www.kungal.com 的游戏页链接。
func NormalizeKungalID(id string) (string, bool) {
	normalized := strings.TrimSpace(id)
	if match := kungalWorkURLPattern.FindStringSubmatch(normalized); len(match) == 2 {
		normalized = match[1]
	}
	parsed, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || parsed <= 0 {
		return "", false
	}
	return strconv.FormatInt(parsed, 10), true
}

func (k KungalInfoGetter) FetchMetadata(id string, accessToken string) (MetadataResult, error) {
	normalizedID, ok := NormalizeKungalID(id)
	if !ok {
		return MetadataResult{}, fmt.Errorf("invalid Kungal ID format: %s", id)
	}

	params := url.Values{}
	params.Set("nsfw", "true")
	params.Set("include", kungalIncludes)

	// 被合并的作品返回 404 ENTITY_MERGED 并带 current_id，跟随一次即可拿到幸存条目。
	for attempt := 0; attempt < 2; attempt++ {
		reqURL := fmt.Sprintf("%s/catalog/works/%s?%s", kungalAPIBaseURL, url.PathEscape(normalizedID), params.Encode())
		statusCode, bodyBytes, err := k.doGet(reqURL, accessToken)
		if err != nil {
			return MetadataResult{}, err
		}
		if statusCode == http.StatusNotFound {
			problem := parseKungalProblem(bodyBytes)
			if problem.Code == "ENTITY_MERGED" && problem.CurrentID != "" && attempt == 0 {
				normalizedID = problem.CurrentID
				continue
			}
			return MetadataResult{}, fmt.Errorf("Kungal work %s not found", normalizedID)
		}
		if statusCode != http.StatusOK {
			return MetadataResult{}, kungalStatusError("Kungal detail API", statusCode, bodyBytes)
		}

		var work kungalWork
		if err := json.Unmarshal(bodyBytes, &work); err != nil {
			return MetadataResult{}, fmt.Errorf("decode Kungal detail response: %w", err)
		}
		if strings.TrimSpace(work.ID) == "" {
			return MetadataResult{}, errors.New("Kungal API returned no work data")
		}
		return k.convertToMetadataResult(work), nil
	}
	return MetadataResult{}, fmt.Errorf("Kungal work %s redirect loop", normalizedID)
}

func (k KungalInfoGetter) FetchMetadataByName(name string, accessToken string) (MetadataResult, error) {
	results, err := k.FetchMetadataCandidatesByName(name, accessToken)
	if err != nil {
		return MetadataResult{}, err
	}
	return results[0], nil
}

func (k KungalInfoGetter) FetchMetadataCandidatesByName(name string, accessToken string) ([]MetadataResult, error) {
	keyword := strings.TrimSpace(name)
	if keyword == "" {
		return nil, errors.New("Kungal search keyword is empty")
	}

	// works?q= 的每条命中就是完整作品对象，不需要再逐条读详情。
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("limit", strconv.Itoa(metadataSearchCandidateLimit))
	params.Set("nsfw", "true")
	params.Set("include", kungalIncludes)
	statusCode, bodyBytes, err := k.doGet(fmt.Sprintf("%s/catalog/works?%s", kungalAPIBaseURL, params.Encode()), accessToken)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, kungalStatusError("Kungal search API", statusCode, bodyBytes)
	}

	var list kungalWorkList
	if err := json.Unmarshal(bodyBytes, &list); err != nil {
		return nil, fmt.Errorf("decode Kungal search response: %w", err)
	}
	works := make([]kungalWork, 0, len(list.Items))
	candidateNames := make([][]string, 0, len(list.Items))
	for _, work := range list.Items {
		if strings.TrimSpace(work.ID) == "" {
			continue
		}
		works = append(works, work)
		candidateNames = append(candidateNames, kungalWorkNames(work))
	}
	if len(works) == 0 {
		return nil, errors.New("no results found")
	}

	indexes := exactMetadataCandidateIndexes(keyword, candidateNames)
	if len(indexes) == 0 {
		indexes = []int{0}
	}
	results := make([]MetadataResult, 0, len(indexes))
	for _, index := range indexes {
		results = append(results, k.convertToMetadataResult(works[index]))
	}
	return results, nil
}

func (k KungalInfoGetter) doGet(reqURL, accessToken string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(metadataContext(k.ctx), http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create Kungal API request: %w", err)
	}
	// 目录只读面可匿名调用；带用户令牌时按用户计配额，额度更高。
	if token := strings.TrimSpace(accessToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())

	statusCode, _, bodyBytes, err := doLimitedMetadataRequestBody(k.client, req, enums.Kungal)
	if err != nil {
		return 0, nil, err
	}
	if statusCode == http.StatusUnauthorized {
		return statusCode, bodyBytes, fmt.Errorf("%w: %s", ErrKungalUnauthorized, kungalProblemMessage(bodyBytes))
	}
	return statusCode, bodyBytes, nil
}

func (k KungalInfoGetter) convertToMetadataResult(work kungalWork) MetadataResult {
	name := firstKungalLocalized(work.Localized, "zh-Hans", "zh-Hant")
	if name == "" {
		name = strings.TrimSpace(work.DisplayName)
	}

	releaseDate := ""
	if work.ReleaseDate != nil {
		releaseDate = strings.TrimSpace(*work.ReleaseDate)
	}
	coverURL := ""
	if work.Cover != nil {
		coverURL = strings.TrimSpace(work.Cover.URL)
	}

	game := models.Game{
		Name:           name,
		Company:        kungalDeveloper(work.Companies),
		Aliases:        normalizeMetadataAliases(name, kungalWorkNames(work)),
		CoverURL:       coverURL,
		CoverSourceURL: coverURL,
		Summary:        kungalSummary(work.Intros, work.Olang),
		ReleaseDate:    releaseDate,
		Rating:         kungalRatingScore(work.Ratings),
		IsNSFW:         work.ContentRating == "r18",
		SourceType:     enums.Kungal,
		SourceID:       strings.TrimSpace(work.ID),
		CachedAt:       time.Now(),
	}
	return MetadataResult{Game: game, Tags: extractKungalTags(work.Tags, k.tagLimit)}
}

func kungalWorkNames(work kungalWork) []string {
	names := []string{work.DisplayName}
	if work.Latin != nil {
		names = append(names, *work.Latin)
	}
	for _, title := range work.Localized {
		names = append(names, title.Value)
	}
	for _, title := range work.Titles {
		names = append(names, title.Title)
		if title.Latin != nil {
			names = append(names, *title.Latin)
		}
	}
	return names
}

func firstKungalLocalized(localized map[string]kungalLocalizedTitle, langs ...string) string {
	for _, lang := range langs {
		if value := strings.TrimSpace(localized[lang].Value); value != "" {
			return value
		}
	}
	return ""
}

func kungalSummary(intros []kungalIntro, olang string) string {
	langs := []string{"zh-Hans", "zh-Hant", olang, "ja", "en"}
	// 人工简介优先，机器翻译只作兜底。
	for _, allowMachine := range []bool{false, true} {
		for _, lang := range langs {
			for _, intro := range intros {
				value := strings.TrimSpace(intro.Value)
				if value != "" && intro.Lang == lang && (allowMachine || !intro.IsMachine) {
					return value
				}
			}
		}
	}
	for _, intro := range intros {
		if value := strings.TrimSpace(intro.Value); value != "" {
			return value
		}
	}
	return ""
}

func kungalDeveloper(companies []kungalCompany) string {
	for _, role := range []string{"developer", "circle", "brand", "publisher"} {
		for _, company := range companies {
			if company.AttributionRole != role {
				continue
			}
			if name := firstKungalLocalized(company.Localized, "zh-Hans", "zh-Hant"); name != "" {
				return name
			}
			if name := strings.TrimSpace(company.DisplayName); name != "" {
				return name
			}
		}
	}
	if len(companies) > 0 {
		return strings.TrimSpace(companies[0].DisplayName)
	}
	return ""
}

// kungalRatingScore 按来源优先级取一个十分制评分；各源量纲不同，不做跨源平均。
func kungalRatingScore(ratings []kungalRating) float64 {
	for _, source := range []string{"nextmoe", "vndb", "bangumi", "erogamescape"} {
		for _, rating := range ratings {
			if rating.Source == source && rating.Score != nil && rating.VoteCount > 0 {
				if score := normalizeTenPointRating(*rating.Score); score > 0 {
					return score
				}
			}
		}
	}
	return 0
}

func extractKungalTags(tags []kungalTag, limit int) []TagItem {
	if limit == 0 {
		return nil
	}
	result := make([]TagItem, 0, tagItemsCapacity(len(tags), limit))
	seen := make(map[string]struct{}, len(tags))
	// 先收 core，再收 longtail；hidden 不展示。
	for _, tier := range []string{"core", "longtail"} {
		weight := 1.0
		if tier == "longtail" {
			weight = 0.5
		}
		for _, tag := range tags {
			name := strings.TrimSpace(tag.DisplayName)
			if tag.Tier != tier || name == "" {
				continue
			}
			key := strings.ToLower(name)
			if _, exists := seen[key]; exists {
				continue
			}
			if hasReachedTagLimit(len(result), limit) {
				return result
			}
			seen[key] = struct{}{}
			result = append(result, TagItem{
				Name:      name,
				Source:    string(enums.Kungal),
				Weight:    weight,
				IsSpoiler: tag.Spoiler != "" && tag.Spoiler != "none",
			})
		}
	}
	return result
}

func parseKungalProblem(body []byte) kungalProblem {
	var problem kungalProblem
	_ = json.Unmarshal(body, &problem)
	return problem
}

func kungalProblemMessage(body []byte) string {
	problem := parseKungalProblem(body)
	message := firstNonEmpty(problem.Detail, problem.Title)
	if problem.Code != "" {
		message = strings.TrimSpace(problem.Code + ": " + message)
	}
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if problem.RequestID != "" {
		message += " (request_id: " + problem.RequestID + ")"
	}
	return message
}

func kungalStatusError(prefix string, statusCode int, body []byte) error {
	return fmt.Errorf("%s returned status: %d, %s", prefix, statusCode, kungalProblemMessage(body))
}
