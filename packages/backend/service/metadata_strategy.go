package service

import (
	"context"
	"slices"
	"strings"

	"lunabox/backend/applog"
	enums2 "lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service/gamehelper"
	"lunabox/backend/utils/metadata"
)

// idMappedSources 是内置 ID 对照表覆盖的来源，按查找的先后顺序。
var idMappedSources = []enums2.SourceType{enums2.VNDB, enums2.Bangumi, enums2.Steam, enums2.Hikarinagi}

// mappedMetadataSourceIDs 用内置的 ID 对照表，从 known 里任一个来源的 ID 查出同一个游戏在其他来源的 ID
// （不含 known 里已有的来源）。查不到或对照表不可用时为空。
func (s *GameService) mappedMetadataSourceIDs(known map[enums2.SourceType]string) map[enums2.SourceType]string {
	if s == nil || len(known) == 0 {
		return nil
	}
	mapper, err := s.getGameIDMapper()
	if err != nil || mapper == nil {
		return nil
	}
	for _, source := range idMappedSources {
		id := strings.TrimSpace(known[source])
		if id == "" {
			continue
		}
		mapping, ok := mapper.Resolve(source, id)
		if !ok {
			continue
		}
		out := map[enums2.SourceType]string{}
		for _, target := range idMappedSources {
			if _, exists := known[target]; exists {
				continue
			}
			if targetID, ok := mapping.SourceID(target); ok {
				out[target] = targetID
			}
		}
		return out
	}
	return nil
}

// ResolveMetadataSourceIDs 返回刷新元数据时游戏在各来源可用的 ID：已关联的来源与默认来源的 ID，
// 再加上按内置 ID 对照表查到的其他来源的 ID（已关联的优先）。界面据此标出组合策略里哪些字段的来源没有 ID。
func (s *GameService) ResolveMetadataSourceIDs(game models.Game) map[enums2.SourceType]string {
	known := map[enums2.SourceType]string{}
	for _, ref := range game.MetadataSources {
		if id := strings.TrimSpace(ref.SourceID); id != "" {
			known[gamehelper.NormalizeMetadataSourceType(ref.SourceType)] = id
		}
	}
	if id, base := strings.TrimSpace(game.SourceID), gamehelper.NormalizeMetadataSourceType(game.SourceType); id != "" && base != "" {
		known[base] = id
	}
	for src, id := range s.mappedMetadataSourceIDs(known) {
		known[src] = id
	}
	return known
}

// refreshUsesStrategy 报告默认的更新元数据是否套用高级组合策略（设置里的“更新元数据时使用组合策略”，与刮削的开关无关）。
func (s *GameService) refreshUsesStrategy() bool {
	return s.config != nil && s.config.MetadataRefreshUseStrategy
}

// composeRefreshResult 在更新元数据时套用高级组合策略（要不要套用由调用方决定）：base 是 gameSource 的结果；
// 要更新的每个字段按“字段指定的来源 → gameSource”依次取第一个有数据的来源，
// 来源没有 ID（已关联的或者 ID 对照表查到的）、没有启用、获取失败或者这一项为空时往后回退，需要时才去获取。
// 返回组合后的结果、标签的来源，以及要更新的字段：所有来源都为空的字段去掉，保留本地内容。没有配置时原样返回。
func (s *GameService) composeRefreshResult(ctx context.Context, game models.Game, gameSource enums2.SourceType, base metadata.MetadataResult, fieldSet gamehelper.MetadataUpdateFieldSet) (metadata.MetadataResult, enums2.SourceType, gamehelper.MetadataUpdateFieldSet) {
	if s.config == nil {
		return base, "", fieldSet
	}
	strategy := s.config.Strategy()
	gameSource = gamehelper.NormalizeMetadataSourceType(gameSource)
	baseItem := vo.GameMetadataFromWebVO{Source: gameSource, Game: base.Game, Tags: base.Tags}

	var ids map[enums2.SourceType]string
	fetched := map[enums2.SourceType]*vo.GameMetadataFromWebVO{gameSource: &baseItem} // nil：取不到
	var others []vo.GameMetadataFromWebVO
	get := func(src enums2.SourceType) *vo.GameMetadataFromWebVO {
		if item, ok := fetched[src]; ok {
			return item
		}
		fetched[src] = nil
		if ids == nil {
			ids = s.ResolveMetadataSourceIDs(game)
		}
		id := ids[src]
		if id == "" || !s.isMetadataSourceEnabled(src) || ctx.Err() != nil {
			return nil
		}
		result, err := s.fetchMetadataResultBySource(ctx, src, strings.ToLower(id))
		if err != nil {
			applog.LogWarningf(s.ctx, "composeRefreshResult: fetch %s/%s for %s failed: %v", src, id, game.Name, err)
			return nil
		}
		if gamehelper.IsEmptyGame(result.Game) {
			return nil
		}
		item := vo.GameMetadataFromWebVO{Source: src, Game: result.Game, Tags: result.Tags}
		others = append(others, item)
		fetched[src] = &item
		return &item
	}
	for _, field := range enums2.DefaultMetadataUpdateFields {
		if !fieldSet.Has(field) {
			continue
		}
		for _, src := range gamehelper.RefreshChain(strategy, field, gameSource) {
			if item := get(src); item != nil && gamehelper.FieldHasValue(*item, field) {
				break
			}
		}
	}

	composed, sources, missing := gamehelper.ComposeRefresh(strategy, baseItem, others)
	next := gamehelper.MetadataUpdateFieldSet{}
	for field := range fieldSet {
		if !slices.Contains(missing, field) {
			next[field] = struct{}{}
		}
	}
	return metadata.MetadataResult{Game: composed.Game, Tags: composed.Tags}, sources[enums2.MetadataUpdateFieldTags], next
}

// composeImportMatches 在批量导入匹配时套用高级组合策略：matches[0] 是基底的结果，其余是同一个游戏在其他来源的结果。
// 策略对这个基底不生效时返回 nil。
func (s *ImportService) composeImportMatches(base enums2.SourceType, matches []vo.GameMetadataFromWebVO) (*vo.GameMetadataFromWebVO, gamehelper.FieldSources) {
	if s.config == nil || len(matches) == 0 {
		return nil, nil
	}
	strategy := s.config.Strategy()
	if !strategy.Active() || strategy.Base != base || len(strategy.Overrides(base)) == 0 {
		return nil, nil
	}
	composed, sources := gamehelper.ComposeMetadata(strategy, matches[0], matches[1:])
	return &composed, sources
}
