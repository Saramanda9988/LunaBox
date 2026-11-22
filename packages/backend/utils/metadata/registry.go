package metadata

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"lunabox/backend/common/enums"
)

// PluginSourcePrefix 是插件注册元数据源的来源类型前缀（plugin:<pluginId>/<id>）。
const PluginSourcePrefix = "plugin:"

// ExternalSourceMeta 描述一个由插件注册的元数据源的展示与辅助信息。
type ExternalSourceMeta struct {
	Source    enums.SourceType  `json:"source"`
	PluginID  string            `json:"plugin_id"`
	ID        string            `json:"id"`
	Names     map[string]string `json:"names"`
	IconPath  string            `json:"icon_path,omitempty"`
	PerSecond float64           `json:"per_second,omitempty"`
}

// DisplayName 按当前语言取显示名称，回退到 en-US、再回退到首个可用值或来源 ID。
func (m ExternalSourceMeta) DisplayName(lang string) string {
	if len(m.Names) == 0 {
		return string(m.Source)
	}
	if v := strings.TrimSpace(m.Names[lang]); v != "" {
		return v
	}
	// 语言前缀回退（如 zh-CN -> zh）
	if idx := strings.IndexAny(lang, "-_"); idx > 0 {
		prefix := strings.ToLower(lang[:idx])
		for k, v := range m.Names {
			if strings.HasPrefix(strings.ToLower(k), prefix) && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	if v := strings.TrimSpace(m.Names["en-US"]); v != "" {
		return v
	}
	if v := strings.TrimSpace(m.Names["en"]); v != "" {
		return v
	}
	keys := make([]string, 0, len(m.Names))
	for k := range m.Names {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if v := strings.TrimSpace(m.Names[k]); v != "" {
			return v
		}
	}
	return string(m.Source)
}

// ExternalSource 是插件元数据源在宿主侧实现的接口。
// 它同时提供 Getter / CandidateGetter / BatchGetter 能力，并带 context 支持。
type ExternalSource interface {
	Getter
	CandidateGetter
	BatchGetter
	Meta() ExternalSourceMeta
	NormalizeID(input string) (string, bool)
	Link(sourceID string) string
	FetchContext(ctx context.Context, id string, language string) (MetadataResult, error)
	SearchContext(ctx context.Context, name string, language string, limit int) ([]MetadataResult, error)
	FetchBatchContext(ctx context.Context, ids []string, language string) (map[string]MetadataResult, error)
}

// IsValidPluginSource 检查来源字符串是否符合 plugin:<pluginId>/<id> 格式。
func IsValidPluginSource(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if !strings.HasPrefix(s, PluginSourcePrefix) {
		return false
	}
	rest := strings.TrimPrefix(s, PluginSourcePrefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 || slash >= len(rest)-1 {
		return false
	}
	pluginID := rest[:slash]
	sourceID := rest[slash+1:]
	if strings.ContainsRune(sourceID, '/') {
		return false
	}
	return isValidSlug(pluginID) && isValidSlug(sourceID)
}

func isValidSlug(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isLower {
			return false
		}
		if !isLower && !isDigit && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

type externalRegistry struct {
	mu          sync.RWMutex
	sources     map[enums.SourceType]ExternalSource
	order       []enums.SourceType
	listeners   map[uint64]func()
	nextListen  uint64
}

var defaultExternalRegistry = &externalRegistry{
	sources:   make(map[enums.SourceType]ExternalSource),
	listeners: make(map[uint64]func()),
}

// RegisterExternal 注册一个插件提供的外部元数据源，返回幂等的撤销函数。
func RegisterExternal(source enums.SourceType, ext ExternalSource) (dispose func()) {
	norm := enums.SourceType(strings.ToLower(strings.TrimSpace(string(source))))
	if norm == "" || ext == nil {
		return func() {}
	}

	meta := ext.Meta()
	interval := time.Second
	if meta.PerSecond > 0 {
		interval = time.Duration(float64(time.Second) / meta.PerSecond)
		if interval < 50*time.Millisecond {
			interval = 50 * time.Millisecond
		}
	}
	RegisterRateLimitPolicy(MetadataRateLimitPolicy{
		Source:   norm,
		Interval: interval,
	})

	defaultExternalRegistry.mu.Lock()
	if _, exists := defaultExternalRegistry.sources[norm]; !exists {
		defaultExternalRegistry.order = append(defaultExternalRegistry.order, norm)
	}
	defaultExternalRegistry.sources[norm] = ext
	defaultExternalRegistry.mu.Unlock()
	defaultExternalRegistry.notify()

	var once sync.Once
	return func() {
		once.Do(func() {
			defaultExternalRegistry.mu.Lock()
			if cur, ok := defaultExternalRegistry.sources[norm]; ok && cur == ext {
				delete(defaultExternalRegistry.sources, norm)
				defaultExternalRegistry.order = slices.DeleteFunc(defaultExternalRegistry.order, func(s enums.SourceType) bool {
					return s == norm
				})
				UnregisterRateLimitPolicy(norm)
			}
			defaultExternalRegistry.mu.Unlock()
			defaultExternalRegistry.notify()
		})
	}
}

// LookupExternal 查询已注册的外部元数据源。
func LookupExternal(source enums.SourceType) (ExternalSource, bool) {
	norm := enums.SourceType(strings.ToLower(strings.TrimSpace(string(source))))
	defaultExternalRegistry.mu.RLock()
	ext, ok := defaultExternalRegistry.sources[norm]
	defaultExternalRegistry.mu.RUnlock()
	return ext, ok
}

// ExternalSources 按注册顺序返回当前所有已注册的外部元数据源。
func ExternalSources() []ExternalSource {
	defaultExternalRegistry.mu.RLock()
	out := make([]ExternalSource, 0, len(defaultExternalRegistry.order))
	for _, s := range defaultExternalRegistry.order {
		if ext, ok := defaultExternalRegistry.sources[s]; ok {
			out = append(out, ext)
		}
	}
	defaultExternalRegistry.mu.RUnlock()
	return out
}

// SubscribeExternalChanges 订阅外部元数据源注册/注销变更，返回取消订阅函数。
func SubscribeExternalChanges(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	defaultExternalRegistry.mu.Lock()
	defaultExternalRegistry.nextListen++
	id := defaultExternalRegistry.nextListen
	defaultExternalRegistry.listeners[id] = fn
	defaultExternalRegistry.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			defaultExternalRegistry.mu.Lock()
			delete(defaultExternalRegistry.listeners, id)
			defaultExternalRegistry.mu.Unlock()
		})
	}
}

func (r *externalRegistry) notify() {
	r.mu.RLock()
	cbs := make([]func(), 0, len(r.listeners))
	for _, fn := range r.listeners {
		cbs = append(cbs, fn)
	}
	r.mu.RUnlock()
	for _, fn := range cbs {
		fn()
	}
}

// AcquireExternalRateLimit 在调用插件元数据源前应用对应的限速策略。
func AcquireExternalRateLimit(ctx context.Context, source enums.SourceType) error {
	return sharedMetadataRateLimiter.Acquire(ctx, source)
}
