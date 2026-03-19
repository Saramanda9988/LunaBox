package metadata

import (
	"io"
	"lunabox/internal/models"
	"net/http"
	"time"

	"github.com/labstack/gommon/log"
)

// TagItem 表示从数据源拉取的单个 tag
type TagItem struct {
	Name      string
	Source    string  // 'bangumi' | 'vndb'
	Weight    float64 // 归一化权重
	IsSpoiler bool
}

// MetadataResult 包含游戏元数据和 tag 列表
type MetadataResult struct {
	Game models.Game
	Tags []TagItem
}

// Getter 获取元数据。
type Getter interface {
	FetchMetadata(id string, token string) (MetadataResult, error)
	FetchMetadataByName(name string, token string) (MetadataResult, error)
}

const metadataUserAgent = "Saramanda9988/LunaBox/1.5.3 (desktop) (https://github.com/Saramanda9988/LunaBox)"

func newMetadataClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func closeResponseBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		log.Warnf("Error closing response body: %v", err)
	}
}
