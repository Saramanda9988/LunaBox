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

type vndbQueryResult struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Image       vndbImage       `json:"image"`
	Description string          `json:"description"`
	Developers  []vndbDeveloper `json:"developers"`
}

type vndbResponse struct {
	Results []vndbQueryResult `json:"results"`
}

func (v VNDBInfoGetter) FetchMetadata(id string, token string) (models.Game, error) {
	return v.queryVNDB([]interface{}{"id", "=", id})
}

func (v VNDBInfoGetter) FetchMetadataByName(name string, token string) (models.Game, error) {
	return v.queryVNDB([]interface{}{"search", "=", name})
}

func (v VNDBInfoGetter) queryVNDB(filters []interface{}) (models.Game, error) {
	reqBody := vndbRequest{
		Filters: filters,
		Fields:  "id, title, image.url, description, developers.name",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return models.Game{}, err
	}

	req, err := http.NewRequest("POST", vndbAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return models.Game{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}
	defer closeResponseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return models.Game{}, fmt.Errorf("VNDB API returned status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var vndbResp vndbResponse
	if err := json.NewDecoder(resp.Body).Decode(&vndbResp); err != nil {
		return models.Game{}, err
	}
	if len(vndbResp.Results) == 0 {
		return models.Game{}, errors.New("no results found")
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

	return models.Game{
		Name:       result.Title,
		CoverURL:   result.Image.URL,
		Company:    company,
		Summary:    result.Description,
		SourceType: enums.VNDB,
		SourceID:   result.ID,
		CachedAt:   time.Now(),
	}, nil
}
