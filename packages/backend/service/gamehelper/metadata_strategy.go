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

// StrategyActive 判断组合策略对批量导入是否生效：打开了刮削的开关，必选数据源（批量导入的基底）已启用，
// 并且至少有一个字段实际改用了其他来源（见 StrategyOverridesFor）。
func StrategyActive(strategy appconf.MetadataStrategy, enabled []enums.SourceType) bool {
	return strategy.Active() && slices.Contains(enabled, strategy.Base) && len(StrategyOverridesFor(strategy, strategy.Base, enabled)) > 0
}

// StrategyActiveFor 判断以 base 为基底刮削时组合策略是否生效（单个添加、按名称重新搜索时基底是用户选的来源）：
// 打开了刮削的开关，并且至少有一个字段实际改用了 base 以外的来源。
func StrategyActiveFor(strategy appconf.MetadataStrategy, base enums.SourceType, enabled []enums.SourceType) bool {
	return strategy.Enabled && base != "" && len(StrategyOverridesFor(strategy, base, enabled)) > 0
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

// RefreshChain 是更新元数据时字段 field 依次尝试的来源：字段指定的来源 → 游戏的默认来源 gameSource
// （去掉空的与重复的；必选数据源只用于批量导入，不参与更新）。别名合并所有来源时只有 gameSource（合并在 ComposeRefresh 里做）。
func RefreshChain(strategy appconf.MetadataStrategy, field enums.MetadataUpdateField, gameSource enums.SourceType) []enums.SourceType {
	var out []enums.SourceType
	add := func(src enums.SourceType) {
		if src != "" && src != appconf.MetadataStrategyAllSources && !slices.Contains(out, src) {
			out = append(out, src)
		}
	}
	add(strategy.FieldSource(field))
	add(gameSource)
	return out
}

// FieldHasValue 报告 item 里字段 field 有没有数据（组合时为空的字段会回退到下一个来源）。
func FieldHasValue(item vo.GameMetadataFromWebVO, field enums.MetadataUpdateField) bool {
	switch field {
	case enums.MetadataUpdateFieldName:
		return strings.TrimSpace(item.Game.Name) != ""
	case enums.MetadataUpdateFieldAliases:
		return len(NormalizeAliases(item.Game.Aliases)) > 0
	case enums.MetadataUpdateFieldCover:
		return strings.TrimSpace(item.Game.CoverURL) != ""
	case enums.MetadataUpdateFieldCompany:
		return strings.TrimSpace(item.Game.Company) != ""
	case enums.MetadataUpdateFieldSummary:
		return strings.TrimSpace(item.Game.Summary) != ""
	case enums.MetadataUpdateFieldReleaseDate:
		return strings.TrimSpace(item.Game.ReleaseDate) != ""
	case enums.MetadataUpdateFieldRating:
		return item.Game.Rating > 0
	case enums.MetadataUpdateFieldTags:
		return len(item.Tags) > 0
	}
	return false
}

// ComposeMetadata 按组合策略把基底结果 base 与同一个游戏在其他来源的结果 others 拼成一个结果（导入时用）。
// base.Source 就是基底；字段指定的来源不在 others 里、或者那一项为空时用 base 的数据。
// 结果的来源、ID、NSFW 与关联来源都沿用 base；others 里同一来源只用第一个。
func ComposeMetadata(strategy appconf.MetadataStrategy, base vo.GameMetadataFromWebVO, others []vo.GameMetadataFromWebVO) (vo.GameMetadataFromWebVO, FieldSources) {
	baseSource := NormalizeMetadataSourceType(base.Source)
	out, sources, _ := composeChain(base, others, strategy.FieldSource(enums.MetadataUpdateFieldAliases) == appconf.MetadataStrategyAllSources,
		func(field enums.MetadataUpdateField) []enums.SourceType {
			if want := strategy.FieldSource(field); want != "" && want != baseSource {
				return []enums.SourceType{want, baseSource}
			}
			return []enums.SourceType{baseSource}
		})
	// 导入时所有来源都为空的字段仍记为基底。
	for field, src := range sources {
		if src == "" {
			sources[field] = baseSource
		}
	}
	return out, sources
}

// ComposeRefresh 是更新元数据时的组合：base 是游戏默认来源 gameSource 的结果，每个字段按 RefreshChain 依次取
// 第一个有数据的来源（不在 others 里的来源跳过）。返回组合结果、各字段实际的来源，以及所有来源都为空的字段
// （更新时保留本地内容；别名是合并，不算在内）。
func ComposeRefresh(strategy appconf.MetadataStrategy, base vo.GameMetadataFromWebVO, others []vo.GameMetadataFromWebVO) (vo.GameMetadataFromWebVO, FieldSources, []enums.MetadataUpdateField) {
	gameSource := NormalizeMetadataSourceType(base.Source)
	return composeChain(base, others, strategy.FieldSource(enums.MetadataUpdateFieldAliases) == appconf.MetadataStrategyAllSources,
		func(field enums.MetadataUpdateField) []enums.SourceType {
			return RefreshChain(strategy, field, gameSource)
		})
}

// composeChain 按 chain 给出的来源顺序为每个字段取第一个有数据的结果；mergeAliases 时别名合并所有结果的名称与别名。
// 结果的来源、ID、NSFW 与关联来源都沿用 base；others 里与 base 同来源的、以及同一来源的第二个结果忽略。
// 字段在所有来源都为空时用 base 的值，来源记为空并列在 missing 里（别名除外）。
func composeChain(base vo.GameMetadataFromWebVO, others []vo.GameMetadataFromWebVO, mergeAliases bool,
	chain func(enums.MetadataUpdateField) []enums.SourceType) (vo.GameMetadataFromWebVO, FieldSources, []enums.MetadataUpdateField) {
	baseSource := NormalizeMetadataSourceType(base.Source)
	bySource := map[enums.SourceType]vo.GameMetadataFromWebVO{baseSource: base}
	var order []enums.SourceType
	for _, item := range others {
		src := NormalizeMetadataSourceType(item.Source)
		if src == "" {
			continue
		}
		if _, exists := bySource[src]; exists {
			continue
		}
		bySource[src] = item
		order = append(order, src)
	}

	sources := FieldSources{}
	var missing []enums.MetadataUpdateField
	pick := func(field enums.MetadataUpdateField) vo.GameMetadataFromWebVO {
		for _, src := range chain(field) {
			if item, ok := bySource[src]; ok && FieldHasValue(item, field) {
				sources[field] = src
				return item
			}
		}
		sources[field] = ""
		if field != enums.MetadataUpdateFieldAliases {
			missing = append(missing, field)
		}
		return base
	}

	out := base
	out.Source = baseSource
	out.Game.Aliases = slices.Clone(base.Game.Aliases)
	out.Game.MetadataSources = slices.Clone(base.Game.MetadataSources)
	out.Tags = slices.Clone(base.Tags)

	out.Game.Name = pick(enums.MetadataUpdateFieldName).Game.Name
	item := pick(enums.MetadataUpdateFieldCover)
	out.Game.CoverURL, out.Game.CoverSourceURL = item.Game.CoverURL, item.Game.CoverSourceURL
	out.Game.Company = pick(enums.MetadataUpdateFieldCompany).Game.Company
	out.Game.Summary = pick(enums.MetadataUpdateFieldSummary).Game.Summary
	out.Game.ReleaseDate = pick(enums.MetadataUpdateFieldReleaseDate).Game.ReleaseDate
	out.Game.Rating = pick(enums.MetadataUpdateFieldRating).Game.Rating
	tagSource := baseSource
	item = pick(enums.MetadataUpdateFieldTags)
	if src := sources[enums.MetadataUpdateFieldTags]; src != "" {
		tagSource = src
	}
	out.Tags = withTagSource(item.Tags, tagSource)

	// 别名：合并全部来源时收集每个来源的名称与别名；否则取第一个有别名的来源。名称换成了其他来源的名称时，
	// 基底的名称留作别名，按原名仍然搜得到。
	var aliases []string
	if mergeAliases {
		aliases = append([]string{base.Game.Name}, base.Game.Aliases...)
		for _, s := range order {
			aliases = append(aliases, bySource[s].Game.Name)
			aliases = append(aliases, bySource[s].Game.Aliases...)
		}
		sources[enums.MetadataUpdateFieldAliases] = appconf.MetadataStrategyAllSources
	} else {
		aliases = slices.Clone(pick(enums.MetadataUpdateFieldAliases).Game.Aliases)
	}
	if strings.TrimSpace(out.Game.Name) != strings.TrimSpace(base.Game.Name) {
		aliases = append(aliases, base.Game.Name)
	}
	name := strings.ToLower(strings.TrimSpace(out.Game.Name))
	aliases = slices.DeleteFunc(NormalizeAliases(aliases), func(a string) bool { return strings.ToLower(a) == name })
	out.Game.Aliases = aliases

	return out, sources, missing
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
