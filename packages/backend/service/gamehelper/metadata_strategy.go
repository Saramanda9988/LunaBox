package gamehelper

import (
	"slices"
	"strings"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/utils/metadata"
)

// FieldSources 记下组合结果里每个字段实际来自哪个来源。只给界面展示用，不入库。
type FieldSources map[enums.MetadataUpdateField]enums.SourceType

// StrategyActive 判断组合策略对新刮削的游戏是否生效：基底已启用，并且至少有一个字段实际改用了其他来源
// （见 StrategyOverridesFor）。
func StrategyActive(strategy appconf.MetadataStrategy, enabled []enums.SourceType) bool {
	return strategy.Base != "" && slices.Contains(enabled, strategy.Base) && len(StrategyOverridesFor(strategy, strategy.Base, enabled)) > 0
}

// StrategyOverridesFor 返回以 base 为基底时实际改用其他来源的字段：指定的来源已启用，或者别名合并所有来源。
// 指定了未启用的来源的字段会用基底的数据，不算在内。
func StrategyOverridesFor(strategy appconf.MetadataStrategy, base enums.SourceType, enabled []enums.SourceType) []enums.MetadataUpdateField {
	var out []enums.MetadataUpdateField
	for _, field := range strategy.Overrides(base) {
		if src := strategy.FieldSource(field); src == appconf.MetadataStrategyAllSources || slices.Contains(enabled, src) {
			out = append(out, field)
		}
	}
	return out
}

// StrategySources 返回以 base 为基底时，组合需要用到的其他来源（不含 base，按字段顺序去重）。
func StrategySources(strategy appconf.MetadataStrategy, base enums.SourceType) []enums.SourceType {
	var out []enums.SourceType
	for _, field := range strategy.Overrides(base) {
		src := strategy.FieldSource(field)
		if src == appconf.MetadataStrategyAllSources || slices.Contains(out, src) {
			continue
		}
		out = append(out, src)
	}
	return out
}

// ComposeMetadata 按组合策略把基底结果 base 与同一个游戏在其他来源的结果 others 拼成一个结果。
// base.Source 就是基底；字段指定的来源不在 others 里、或者那一项为空时用 base 的数据。
// 结果的来源、ID、NSFW 与关联来源都沿用 base；others 里同一来源只用第一个。
func ComposeMetadata(strategy appconf.MetadataStrategy, base vo.GameMetadataFromWebVO, others []vo.GameMetadataFromWebVO) (vo.GameMetadataFromWebVO, FieldSources) {
	baseSource := NormalizeMetadataSourceType(base.Source)
	bySource := make(map[enums.SourceType]vo.GameMetadataFromWebVO, len(others))
	var order []enums.SourceType
	for _, item := range others {
		src := NormalizeMetadataSourceType(item.Source)
		if src == "" || src == baseSource {
			continue
		}
		if _, exists := bySource[src]; exists {
			continue
		}
		bySource[src] = item
		order = append(order, src)
	}

	pick := func(field enums.MetadataUpdateField, has func(vo.GameMetadataFromWebVO) bool) (vo.GameMetadataFromWebVO, enums.SourceType) {
		if want := strategy.FieldSource(field); want != "" && want != baseSource {
			if item, ok := bySource[want]; ok && has(item) {
				return item, want
			}
		}
		return base, baseSource
	}
	nonEmpty := func(get func(vo.GameMetadataFromWebVO) string) func(vo.GameMetadataFromWebVO) bool {
		return func(item vo.GameMetadataFromWebVO) bool { return strings.TrimSpace(get(item)) != "" }
	}

	out := base
	out.Source = baseSource
	out.Game.Aliases = slices.Clone(base.Game.Aliases)
	out.Game.MetadataSources = slices.Clone(base.Game.MetadataSources)
	out.Tags = slices.Clone(base.Tags)
	sources := FieldSources{}

	item, src := pick(enums.MetadataUpdateFieldName, nonEmpty(func(v vo.GameMetadataFromWebVO) string { return v.Game.Name }))
	out.Game.Name, sources[enums.MetadataUpdateFieldName] = item.Game.Name, src

	item, src = pick(enums.MetadataUpdateFieldCover, nonEmpty(func(v vo.GameMetadataFromWebVO) string { return v.Game.CoverURL }))
	out.Game.CoverURL, out.Game.CoverSourceURL, sources[enums.MetadataUpdateFieldCover] = item.Game.CoverURL, item.Game.CoverSourceURL, src

	item, src = pick(enums.MetadataUpdateFieldCompany, nonEmpty(func(v vo.GameMetadataFromWebVO) string { return v.Game.Company }))
	out.Game.Company, sources[enums.MetadataUpdateFieldCompany] = item.Game.Company, src

	item, src = pick(enums.MetadataUpdateFieldSummary, nonEmpty(func(v vo.GameMetadataFromWebVO) string { return v.Game.Summary }))
	out.Game.Summary, sources[enums.MetadataUpdateFieldSummary] = item.Game.Summary, src

	item, src = pick(enums.MetadataUpdateFieldReleaseDate, nonEmpty(func(v vo.GameMetadataFromWebVO) string { return v.Game.ReleaseDate }))
	out.Game.ReleaseDate, sources[enums.MetadataUpdateFieldReleaseDate] = item.Game.ReleaseDate, src

	item, src = pick(enums.MetadataUpdateFieldRating, func(v vo.GameMetadataFromWebVO) bool { return v.Game.Rating > 0 })
	out.Game.Rating, sources[enums.MetadataUpdateFieldRating] = item.Game.Rating, src

	item, src = pick(enums.MetadataUpdateFieldTags, func(v vo.GameMetadataFromWebVO) bool { return len(v.Tags) > 0 })
	out.Tags, sources[enums.MetadataUpdateFieldTags] = withTagSource(item.Tags, src), src

	// 别名：合并全部来源时收集每个来源的名称与别名；否则取指定来源的别名。名称换成了其他来源的名称时，
	// 基底的名称留作别名，按原名仍然搜得到。
	var aliases []string
	if strategy.FieldSource(enums.MetadataUpdateFieldAliases) == appconf.MetadataStrategyAllSources {
		aliases = append([]string{base.Game.Name}, base.Game.Aliases...)
		for _, s := range order {
			aliases = append(aliases, bySource[s].Game.Name)
			aliases = append(aliases, bySource[s].Game.Aliases...)
		}
		sources[enums.MetadataUpdateFieldAliases] = appconf.MetadataStrategyAllSources
	} else {
		item, src = pick(enums.MetadataUpdateFieldAliases, func(v vo.GameMetadataFromWebVO) bool { return len(NormalizeAliases(v.Game.Aliases)) > 0 })
		aliases, sources[enums.MetadataUpdateFieldAliases] = slices.Clone(item.Game.Aliases), src
	}
	if strings.TrimSpace(out.Game.Name) != strings.TrimSpace(base.Game.Name) {
		aliases = append(aliases, base.Game.Name)
	}
	name := strings.ToLower(strings.TrimSpace(out.Game.Name))
	aliases = slices.DeleteFunc(NormalizeAliases(aliases), func(a string) bool { return strings.ToLower(a) == name })
	out.Game.Aliases = aliases

	return out, sources
}

// withTagSource 返回标签的副本，没有来源的标签记为 source（入库时按标签自己的来源写入）。
func withTagSource(tags []metadata.TagItem, source enums.SourceType) []metadata.TagItem {
	out := slices.Clone(tags)
	for i := range out {
		if strings.TrimSpace(out[i].Source) == "" {
			out[i].Source = string(source)
		}
	}
	return out
}
