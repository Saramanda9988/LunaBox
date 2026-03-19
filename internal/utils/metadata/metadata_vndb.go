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
	"strings"
	"time"
)

// VNDBInfoGetter 获取 VNDB 信息。
type VNDBInfoGetter struct {
	client *http.Client
}

func NewVNDBInfoGetter() *VNDBInfoGetter {
	return &VNDBInfoGetter{client: newMetadataClient()}
}

var _ Getter = (*VNDBInfoGetter)(nil)

const vndbAPIURL = "https://api.vndb.org/kana/vn"

type vndbRequest struct {
	Filters []interface{} `json:"filters"`
	Fields  string        `json:"fields"`
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

type vndbQueryResult struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Image       vndbImage       `json:"image"`
	Description string          `json:"description"`
	Developers  []vndbDeveloper `json:"developers"`
	Tags        []vndbTag       `json:"tags"`
}

type vndbResponse struct {
	Results []vndbQueryResult `json:"results"`
}

func (v VNDBInfoGetter) FetchMetadata(id string, token string) (MetadataResult, error) {
	return v.queryVNDB([]interface{}{"id", "=", id})
}

func (v VNDBInfoGetter) FetchMetadataByName(name string, token string) (MetadataResult, error) {
	return v.queryVNDB([]interface{}{"search", "=", name})
}

func (v VNDBInfoGetter) queryVNDB(filters []interface{}) (MetadataResult, error) {
	reqBody := vndbRequest{
		Filters: filters,
		Fields:  "id, title, image.url, description, developers.name, tags.name, tags.rating, tags.spoiler",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return MetadataResult{}, err
	}

	req, err := http.NewRequest("POST", vndbAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return MetadataResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return MetadataResult{}, err
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return MetadataResult{}, fmt.Errorf("VNDB API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var vndbResp vndbResponse
	if err := json.NewDecoder(resp.Body).Decode(&vndbResp); err != nil {
		return MetadataResult{}, err
	}
	if len(vndbResp.Results) == 0 {
		return MetadataResult{}, errors.New("no results found")
	}

	result := vndbResp.Results[0]
	company := ""
	if len(result.Developers) > 0 {
		devs := make([]string, 0, len(result.Developers))
		for _, developer := range result.Developers {
			devs = append(devs, developer.Name)
		}
		company = strings.Join(devs, ", ")
	}

	game := models.Game{
		Name:       result.Title,
		CoverURL:   result.Image.URL,
		Company:    company,
		Summary:    result.Description,
		SourceType: enums.VNDB,
		SourceID:   result.ID,
		CachedAt:   time.Now(),
	}

	return MetadataResult{Game: game, Tags: extractVNDBTags(result.Tags)}, nil
}

// extractVNDBTags 从 VNDB tag 列表中提取符合条件的 TagItem
// 规则：rating >= 1.5，spoiler >= 2 标记为 is_spoiler，按 rating 降序取前 15 条，weight = rating/3.0
func extractVNDBTags(tags []vndbTag) []TagItem {
	// 过滤 rating < 1.5 的 tag
	var filtered []vndbTag
	for _, t := range tags {
		if t.Rating >= 1.5 {
			filtered = append(filtered, t)
		}
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

	// 取前 15 条
	if len(filtered) > 15 {
		filtered = filtered[:15]
	}

	result := make([]TagItem, 0, len(filtered))
	for _, t := range filtered {
		result = append(result, TagItem{
			Name:      t.Name,
			Source:    "vndb",
			Weight:    t.Rating / 3.0,
			IsSpoiler: t.Spoiler >= 2,
		})
	}
	return result
}
