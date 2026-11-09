package appconf

import (
	"slices"
	"strings"

	enums2 "lunabox/backend/common/enums"
)

// MetadataStrategyAllSources 是别名字段特有的取值：合并所有匹配到的来源的名称与别名。
const MetadataStrategyAllSources enums2.SourceType = "all"

// MetadataStrategy 是高级组合策略：Base 是批量导入的必选数据源（以它识别游戏，没找到即视为没找到），不受 Enabled 影响；
// Enabled 打开并且有 Base 时组合策略生效：Base 是基底，Fields 指定个别字段改用哪个来源，
// 没有指定、指定的来源没找到这个游戏或这个字段为空时用 Base 的数据。
//
// Fields 的值是来源列表，为以后的多级回退留位置；目前只用第一项，最后总是回退到 Base。
type MetadataStrategy struct {
	Base    enums2.SourceType                                  `json:"base,omitempty"`
	Enabled bool                                               `json:"enabled,omitempty"`
	Fields  map[enums2.MetadataUpdateField][]enums2.SourceType `json:"fields,omitempty"`
}

// Active 报告组合策略是否生效：打开了开关，并且指定了必选数据源（基底）。
func (s MetadataStrategy) Active() bool { return s.Enabled && s.Base != "" }

// Clone 返回深拷贝。
func (s MetadataStrategy) Clone() MetadataStrategy {
	out := MetadataStrategy{Base: s.Base, Enabled: s.Enabled}
	if len(s.Fields) > 0 {
		out.Fields = make(map[enums2.MetadataUpdateField][]enums2.SourceType, len(s.Fields))
		for field, sources := range s.Fields {
			out.Fields[field] = slices.Clone(sources)
		}
	}
	return out
}

// FieldSource 是字段指定的来源；没有指定时为空。
func (s MetadataStrategy) FieldSource(field enums2.MetadataUpdateField) enums2.SourceType {
	if sources := s.Fields[field]; len(sources) > 0 {
		return sources[0]
	}
	return ""
}

// WithFieldSource 返回把字段改为 source 的副本；source 为空时去掉这个字段的指定。
func (s MetadataStrategy) WithFieldSource(field enums2.MetadataUpdateField, source enums2.SourceType) MetadataStrategy {
	out := s.Clone()
	if source == "" {
		delete(out.Fields, field)
		return out
	}
	if out.Fields == nil {
		out.Fields = map[enums2.MetadataUpdateField][]enums2.SourceType{}
	}
	out.Fields[field] = []enums2.SourceType{source}
	return out
}

// Overrides 返回以 base 为基底时真正改用其他来源的字段（按 DefaultMetadataUpdateFields 的顺序）。
// base 为空时用 s.Base。
func (s MetadataStrategy) Overrides(base enums2.SourceType) []enums2.MetadataUpdateField {
	if base == "" {
		base = s.Base
	}
	var out []enums2.MetadataUpdateField
	for _, field := range enums2.DefaultMetadataUpdateFields {
		if src := s.FieldSource(field); src != "" && src != base {
			out = append(out, field)
		}
	}
	return out
}

// Strategy 返回配置里的组合策略（副本）。
func (config *AppConfig) Strategy() MetadataStrategy {
	if config == nil || config.MetadataStrategy == nil {
		return MetadataStrategy{}
	}
	return config.MetadataStrategy.Clone()
}

// SetStrategy 换成新的组合策略（存副本，不与其他配置副本共用）。
func (config *AppConfig) SetStrategy(strategy MetadataStrategy) {
	s := strategy.Clone()
	config.MetadataStrategy = &s
}

// NormalizeMetadataStrategy 清理组合策略：去掉未知的来源与字段。旧配置没有组合策略时，
// 用批量导入的必选数据源（batch_import_preferred_source）作为基底。
func NormalizeMetadataStrategy(config *AppConfig) bool {
	if config == nil {
		return false
	}
	changed := false
	if config.MetadataStrategy == nil {
		config.MetadataStrategy = &MetadataStrategy{Base: enums2.SourceType(config.BatchImportPreferredSource)}
		changed = true
	}
	next := MetadataStrategy{Base: normalizeStrategySource(config.MetadataStrategy.Base), Enabled: config.MetadataStrategy.Enabled}
	for _, field := range enums2.DefaultMetadataUpdateFields {
		var sources []enums2.SourceType
		for _, raw := range config.MetadataStrategy.Fields[field] {
			source := normalizeStrategySource(raw)
			if source == "" && field == enums2.MetadataUpdateFieldAliases &&
				enums2.SourceType(strings.ToLower(strings.TrimSpace(string(raw)))) == MetadataStrategyAllSources {
				source = MetadataStrategyAllSources
			}
			if source != "" && !slices.Contains(sources, source) {
				sources = append(sources, source)
			}
		}
		if len(sources) > 0 {
			if next.Fields == nil {
				next.Fields = map[enums2.MetadataUpdateField][]enums2.SourceType{}
			}
			next.Fields[field] = sources
		}
	}
	if !strategyEqual(*config.MetadataStrategy, next) {
		config.MetadataStrategy = &next
		changed = true
	}
	return changed
}

func normalizeStrategySource(source enums2.SourceType) enums2.SourceType {
	v := strings.ToLower(strings.TrimSpace(string(source)))
	if _, ok := allowedMetadataSourceSet[v]; ok {
		return enums2.SourceType(v)
	}
	return ""
}

func strategyEqual(a, b MetadataStrategy) bool {
	if a.Base != b.Base || a.Enabled != b.Enabled || len(a.Fields) != len(b.Fields) {
		return false
	}
	for field, sources := range a.Fields {
		if !slices.Equal(sources, b.Fields[field]) {
			return false
		}
	}
	return true
}
