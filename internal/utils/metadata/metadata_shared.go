package metadata

import (
	"io"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/httputils"
	"lunabox/internal/utils/proxyutils"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/gommon/log"
)

// TagItem 表示从数据源拉取的单个 tag
type TagItem struct {
	Name      string
	Source    string  // 'bangumi' | 'vndb' | 'ymgal' | 'steam' | 'hikarinagi'
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

// CandidateGetter 可选实现：数据源支持返回名称搜索中的多个同名候选时使用。
type CandidateGetter interface {
	FetchMetadataCandidatesByName(name string, token string) ([]MetadataResult, error)
}

// BatchGetter 可选实现：数据源支持按 ID 批量拉取详情时使用。
type BatchGetter interface {
	FetchMetadataBatch(ids []string, token string) (map[string]MetadataResult, error)
}

const metadataHTTPTimeout = 10 * time.Second
const defaultMetadataTagLimit = 10
const metadataSearchCandidateLimit = 5

// FetchMetadataCandidatesByName 返回数据源的同名候选；旧数据源仍以单个结果兼容。
func FetchMetadataCandidatesByName(getter Getter, name string, token string) ([]MetadataResult, error) {
	if candidateGetter, ok := getter.(CandidateGetter); ok {
		return candidateGetter.FetchMetadataCandidatesByName(name, token)
	}

	result, err := getter.FetchMetadataByName(name, token)
	if err != nil {
		return nil, err
	}
	return []MetadataResult{result}, nil
}

// exactMetadataCandidateIndexes 比较搜索结果前五项的名称及别名。
func exactMetadataCandidateIndexes(query string, candidateNames [][]string) []int {
	queryKey := normalizeMetadataSearchName(query)
	if queryKey == "" {
		return nil
	}

	limit := len(candidateNames)
	if limit > metadataSearchCandidateLimit {
		limit = metadataSearchCandidateLimit
	}
	indexes := make([]int, 0, limit)
	for index := 0; index < limit; index++ {
		for _, name := range candidateNames[index] {
			if normalizeMetadataSearchName(name) == queryKey {
				indexes = append(indexes, index)
				break
			}
		}
	}
	return indexes
}

func normalizeMetadataSearchName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(name))), " ")
}

const (
	hikarinagiBangumiImageProxyBaseURL = "https://imagesp.yurari.moe/bangumi/"
	hikarinagiVNDBImageProxyBaseURL    = "https://imagesp.yurari.moe/vndb/"
)

type getterConfig struct {
	client                *http.Client
	tagLimit              int
	hasTagLimit           bool
	bangumiCoverSource    enums2.MetadataCoverSource
	vndbCoverSource       enums2.MetadataCoverSource
	steamCoverOrientation enums2.SteamCoverOrientation
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
		client, _, err := httputils.NewClient(httputils.ClientOptions{
			Timeout:   metadataHTTPTimeout,
			ProxyMode: mode,
			ProxyURL:  manualURL,
		})
		if err != nil {
			log.Warnf("failed to create metadata HTTP client with proxy: %v", err)
			return
		}
		config.client = client
	}
}

func WithProxyConfig(proxyConfig proxyutils.ProxyConfigProvider) GetterOption {
	return func(config *getterConfig) {
		client, _, err := httputils.NewClient(httputils.ClientOptions{
			Timeout:     metadataHTTPTimeout,
			ProxyConfig: proxyConfig,
		})
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

func WithSteamCoverOrientation(orientation enums2.SteamCoverOrientation) GetterOption {
	return func(config *getterConfig) {
		config.steamCoverOrientation = orientation
	}
}

func WithBangumiCoverSource(source enums2.MetadataCoverSource) GetterOption {
	return func(config *getterConfig) {
		config.bangumiCoverSource = source
	}
}

func WithVNDBCoverSource(source enums2.MetadataCoverSource) GetterOption {
	return func(config *getterConfig) {
		config.vndbCoverSource = source
	}
}

func newMetadataClient() *http.Client {
	client, _, err := httputils.NewClient(httputils.ClientOptions{Timeout: metadataHTTPTimeout})
	if err != nil {
		log.Warnf("failed to create metadata HTTP client with system proxy: %v", err)
		return &http.Client{Timeout: metadataHTTPTimeout}
	}
	return client
}

func newGetterConfig(options []GetterOption) getterConfig {
	config := getterConfig{
		bangumiCoverSource:    enums2.MetadataCoverSourceOriginal,
		vndbCoverSource:       enums2.MetadataCoverSourceOriginal,
		steamCoverOrientation: enums2.SteamCoverOrientationPortrait,
	}
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

func resolveMetadataCoverURL(source enums2.SourceType, coverSource enums2.MetadataCoverSource, originalURL string) string {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" || coverSource != enums2.MetadataCoverSourceHikarinagi {
		return originalURL
	}

	baseURL := ""
	switch source {
	case enums2.Bangumi:
		baseURL = hikarinagiBangumiImageProxyBaseURL
	case enums2.VNDB:
		baseURL = hikarinagiVNDBImageProxyBaseURL
	default:
		return originalURL
	}
	if strings.HasPrefix(originalURL, baseURL) {
		return originalURL
	}
	return baseURL + originalURL
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

func normalizeMetadataAliases(displayName string, groups ...[]string) []string {
	aliases := make([]string, 0)
	seen := make(map[string]struct{})
	displayKey := strings.ToLower(strings.TrimSpace(displayName))
	if displayKey != "" {
		seen[displayKey] = struct{}{}
	}

	for _, group := range groups {
		for _, value := range group {
			alias := strings.TrimSpace(value)
			if alias == "" {
				continue
			}
			key := strings.ToLower(alias)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			aliases = append(aliases, alias)
		}
	}
	return aliases
}
