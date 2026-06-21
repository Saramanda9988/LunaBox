package metadata

import (
	"io"
	"lunabox/internal/models"
	"lunabox/internal/utils/proxyutils"
	"math"
	"net/http"
	"time"

	"github.com/labstack/gommon/log"
)

// TagItem 表示从数据源拉取的单个 tag
type TagItem struct {
	Name      string
	Source    string  // 'bangumi' | 'vndb' | 'ymgal' | 'steam'
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

// BatchGetter 可选实现：数据源支持按 ID 批量拉取详情时使用。
type BatchGetter interface {
	FetchMetadataBatch(ids []string, token string) (map[string]MetadataResult, error)
}

const metadataUserAgent = "Saramanda9988/LunaBox/1.8.0 (desktop) (https://github.com/Saramanda9988/LunaBox)"
const metadataHTTPTimeout = 10 * time.Second
const defaultMetadataTagLimit = 10

type getterConfig struct {
	client      *http.Client
	tagLimit    int
	hasTagLimit bool
}

type GetterOption func(*getterConfig)

func WithHTTPClient(client *http.Client) GetterOption {
	return func(config *getterConfig) {
		if client != nil {
			config.client = client
		}
	}
}

func WithProxy(mode string, manualURL string) GetterOption {
	return func(config *getterConfig) {
		client, _, err := proxyutils.NewHTTPClient(metadataHTTPTimeout, mode, manualURL)
		if err != nil {
			log.Warnf("failed to create metadata HTTP client with proxy: %v", err)
			return
		}
		config.client = client
	}
}

func WithProxyConfig(proxyConfig proxyutils.ProxyConfigProvider) GetterOption {
	return func(config *getterConfig) {
		client, _, err := proxyutils.NewHTTPClientFromConfig(metadataHTTPTimeout, proxyConfig)
		if err != nil {
			log.Warnf("failed to create metadata HTTP client with proxy config: %v", err)
			return
		}
		config.client = client
	}
}

func WithTagLimit(limit int) GetterOption {
	return func(config *getterConfig) {
		if limit < -1 {
			limit = -1
		}
		config.tagLimit = limit
		config.hasTagLimit = true
	}
}

func newMetadataClient() *http.Client {
	client, _, err := proxyutils.NewSystemHTTPClient(metadataHTTPTimeout)
	if err != nil {
		log.Warnf("failed to create metadata HTTP client with system proxy: %v", err)
		return &http.Client{Timeout: metadataHTTPTimeout}
	}
	return client
}

func newGetterConfig(options []GetterOption) getterConfig {
	config := getterConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.client == nil {
		config.client = newMetadataClient()
	}
	if !config.hasTagLimit {
		config.tagLimit = defaultMetadataTagLimit
	}
	return config
}

func tagItemsCapacity(total int, limit int) int {
	if limit == 0 {
		return 0
	}
	if limit > 0 && limit < total {
		return limit
	}
	return total
}

func hasReachedTagLimit(count int, limit int) bool {
	return limit > 0 && count >= limit
}

func closeResponseBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		log.Warnf("Error closing response body: %v", err)
	}
}

func normalizeTenPointRating(raw float64) float64 {
	if raw <= 0 || math.IsNaN(raw) || math.IsInf(raw, 0) {
		return 0
	}

	score := raw
	// 某些来源可能返回 100 分制
	if score > 10 && score <= 100 {
		score = score / 10
	}

	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}

	// 保留 2 位小数
	return math.Round(score*100) / 100
}
