package info

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

	"github.com/labstack/gommon/log"
)

type VNDBInfoGetter struct {
	client  *http.Client
	timeout time.Duration
}

func NewVNDBInfoGetter() *VNDBInfoGetter {
	return &VNDBInfoGetter{
		client:  &http.Client{},
		timeout: 10 * time.Second,
	}
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

func (V VNDBInfoGetter) FetchMetadata(id string, token string) (models.Game, error) {
	filters := []interface{}{"id", "=", id}
	return V.queryVNDB(filters)
}

func (V VNDBInfoGetter) FetchMetadataByName(name string, token string) (models.Game, error) {
	filters := []interface{}{"search", "=", name}
	return V.queryVNDB(filters)
}

func (V VNDBInfoGetter) queryVNDB(filters []interface{}) (models.Game, error) {
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

	resp, err := V.client.Do(req)
	if err != nil {
		return models.Game{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("Error closing response body: %v", err)
		}
	}(resp.Body)

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

	var company string
	if len(result.Developers) > 0 {
		var devs []string
		for _, d := range result.Developers {
			devs = append(devs, d.Name)
		}
		company = strings.Join(devs, ", ")
	}

	var coverURL string
	if result.Image.URL != "" {
		coverURL = result.Image.URL
	}

	game := models.Game{
		Name:       result.Title,
		CoverURL:   coverURL,
		Company:    company,
		Summary:    result.Description,
		SourceType: enums.VNDB,
		SourceID:   result.ID,
		CachedAt:   time.Now(),
	}

	return game, nil
}
