package metadata

import (
	"bytes"
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

type BangumiInfoGetter struct {
	client *http.Client
}

func NewBangumiInfoGetter() *BangumiInfoGetter {
	return &BangumiInfoGetter{client: newMetadataClient()}
}

var _ Getter = (*BangumiInfoGetter)(nil)

const bangumiIDQueryAPIURL = "https://api.bgm.tv/v0/subjects"

type bangumiImages struct {
	Large  string `json:"large"`
	Common string `json:"common"`
	Medium string `json:"medium"`
	Small  string `json:"small"`
	Grid   string `json:"grid"`
}

type bangumiInfoboxItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type bangumiRating struct {
	Rank  int            `json:"rank"`
	Total int            `json:"total"`
	Count map[string]int `json:"count"`
	Score float64        `json:"score"`
}

type bangumiCollection struct {
	Wish    int `json:"wish"`
	Collect int `json:"collect"`
	Doing   int `json:"doing"`
	OnHold  int `json:"on_hold"`
	Dropped int `json:"dropped"`
}

type bangumiTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type bangumiResponse struct {
	ID            int                  `json:"id"`
	Type          int                  `json:"type"`
	Name          string               `json:"name"`
	NameCN        string               `json:"name_cn"`
	Summary       string               `json:"summary"`
	Series        bool                 `json:"series"`
	NSFW          bool                 `json:"nsfw"`
	Locked        bool                 `json:"locked"`
	Date          string               `json:"date"`
	Platform      string               `json:"platform"`
	Images        bangumiImages        `json:"images"`
	Infobox       []bangumiInfoboxItem `json:"infobox"`
	Volumes       int                  `json:"volumes"`
	Eps           int                  `json:"eps"`
	TotalEpisodes int                  `json:"total_episodes"`
	Rating        bangumiRating        `json:"rating"`
	Collection    bangumiCollection    `json:"collection"`
	MetaTags      []string             `json:"meta_tags"`
	Tags          []bangumiTag         `json:"tags"`
}

func (b BangumiInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	if token == "" {
		return MetadataResult{}, errors.New("bangumi API requires Bearer token")
	}

	reqURL := fmt.Sprintf("%s/%s", bangumiIDQueryAPIURL, id)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return MetadataResult{}, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", metadataUserAgent)

	resp, err := b.client.Do(req)
	if err != nil {
		return MetadataResult{}, err
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return MetadataResult{}, fmt.Errorf("bangumi API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var bangumiResp bangumiResponse
	if err := json.NewDecoder(resp.Body).Decode(&bangumiResp); err != nil {
		return MetadataResult{}, err
	}

	if bangumiResp.Type != 4 { // 4 代表游戏
		return MetadataResult{}, errors.New("the provided ID does not correspond to a game")
	}

	// 从 infobox 中提取开发商信息
	company := b.extractCompanyFromInfobox(bangumiResp.Infobox)

	// 使用中文名，如果没有则使用原名
	name := bangumiResp.NameCN
	if name == "" {
		name = bangumiResp.Name
	}

	// 选择最佳的封面图片 (优先使用 large，然后是 common)
	coverURL := bangumiResp.Images.Large
	if coverURL == "" {
		coverURL = bangumiResp.Images.Common
	}

	game := models.Game{
		Name:       name,
		CoverURL:   coverURL,
		Company:    company,
		Summary:    bangumiResp.Summary,
		SourceType: enums.Bangumi,
		SourceID:   id,
		CachedAt:   time.Now(),
	}

	return MetadataResult{Game: game, Tags: extractBangumiTags(bangumiResp.Tags)}, nil
}

func (b BangumiInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	if token == "" {
		return MetadataResult{}, errors.New("bangumi API requires Bearer token")
	}

	searchURL := "https://api.bgm.tv/v0/search/subjects"

	params := url.Values{}
	params.Add("limit", "1")
	params.Add("offset", "0")
	fullURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())

	reqBody := map[string]interface{}{
		"keyword": name,
		"sort":    "rank",
		"filter": map[string]interface{}{
			"type": []int{4},
			"nsfw": true,
		},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return MetadataResult{}, err
	}

	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return MetadataResult{}, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return MetadataResult{}, err
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return MetadataResult{}, fmt.Errorf("bangumi search API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp struct {
		Data []bangumiResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return MetadataResult{}, err
	}

	if len(searchResp.Data) == 0 {
		return MetadataResult{}, errors.New("no results found")
	}

	bangumiResp := searchResp.Data[0]

	if bangumiResp.Type != 4 { // 4 代表游戏
		return MetadataResult{}, errors.New("the provided ID does not correspond to a game")
	}

	// 从 infobox 中提取开发商信息
	company := b.extractCompanyFromInfobox(bangumiResp.Infobox)

	// 使用中文名，如果没有则使用原名
	gameName := bangumiResp.NameCN
	if gameName == "" {
		gameName = bangumiResp.Name
	}

	// 选择最佳的封面图片 (优先使用 large，然后是 common)
	coverURL := bangumiResp.Images.Large
	if coverURL == "" {
		coverURL = bangumiResp.Images.Common
	}

	game := models.Game{
		Name:       gameName,
		CoverURL:   coverURL,
		Company:    company,
		Summary:    bangumiResp.Summary,
		SourceType: enums.Bangumi,
		SourceID:   strconv.Itoa(bangumiResp.ID),
		CachedAt:   time.Now(),
	}

	return MetadataResult{Game: game, Tags: extractBangumiTags(bangumiResp.Tags)}, nil
}

// extractBangumiTags 从 Bangumi tag 列表中提取符合条件的 TagItem
// 规则：count >= 5，按 count 降序取前 15 条，weight = count/max(count)
func extractBangumiTags(tags []bangumiTag) []TagItem {
	// 过滤 count < 5 的 tag
	var filtered []bangumiTag
	for _, t := range tags {
		if t.Count >= 5 {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	// 按 count 降序排序
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[j].Count > filtered[i].Count {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	// 取前 15 条
	if len(filtered) > 15 {
		filtered = filtered[:15]
	}

	maxCount := filtered[0].Count
	result := make([]TagItem, 0, len(filtered))
	for _, t := range filtered {
		weight := 1.0
		if maxCount > 0 {
			weight = float64(t.Count) / float64(maxCount)
		}
		result = append(result, TagItem{
			Name:      t.Name,
			Source:    "bangumi",
			Weight:    weight,
			IsSpoiler: false,
		})
	}
	return result
}

// extractCompanyFromInfobox 从 infobox 中提取开发商信息
func (b BangumiInfoGetter) extractCompanyFromInfobox(infobox []bangumiInfoboxItem) string {
	for _, item := range infobox {
		// 查找开发商相关的字段
		if strings.Contains(item.Key, "开发商") || strings.Contains(item.Key, "开发") {
			switch v := item.Value.(type) {
			case string:
				return v
			case []interface{}:
				// 如果是数组，尝试提取第一个值
				if len(v) > 0 {
					if str, ok := v[0].(string); ok {
						return str
					}
					// 处理可能的对象格式 {"v": "value"}
					if obj, ok := v[0].(map[string]interface{}); ok {
						if val, exists := obj["v"]; exists {
							if str, ok := val.(string); ok {
								return str
							}
						}
					}
				}
			}
		}
	}
	return ""
}
