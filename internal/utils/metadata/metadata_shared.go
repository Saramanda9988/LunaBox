package metadata

import (
	"io"
	"lunabox/internal/models"
	"net/http"
	"time"

	"github.com/labstack/gommon/log"
)

// Getter 获取元数据。
type Getter interface {
	FetchMetadata(id string, token string) (models.Game, error)
	FetchMetadataByName(name string, token string) (models.Game, error)
}

const metadataUserAgent = "Saramanda9988/LunaBox/1.5.2 (desktop) (https://github.com/Saramanda9988/LunaBox)"

func newMetadataClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func closeResponseBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		log.Warnf("Error closing response body: %v", err)
	}
}
