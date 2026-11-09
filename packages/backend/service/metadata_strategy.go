package service

import (
	"context"
	"slices"
	"strings"

	"lunabox/backend/appconf"
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

// composeRefreshResult 在刷新元数据时套用高级组合策略：游戏的默认来源是基底（base 是它的结果），
// 要更新的字段里改用其他来源的，按游戏已关联的 ID（没有时按 ID 对照表查）取那个来源的元数据再组合；
// 取不到的字段仍用基底的数据。返回组合后的结果与标签的来源。策略对这次要更新的字段没有影响时原样返回。
func (s *GameService) composeRefreshResult(ctx context.Context, game models.Game, base metadata.MetadataResult, fieldSet gamehelper.MetadataUpdateFieldSet) (metadata.MetadataResult, enums2.SourceType) {
	baseSource := gamehelper.NormalizeMetadataSourceType(game.SourceType)
	if s.config == nil {
		return base, ""
	}
	strategy := s.config.Strategy()
	var needed []enums2.SourceType
	for _, field := range strategy.Overrides(baseSource) {
		src := strategy.FieldSource(field)
		if !fieldSet.Has(field) || src == appconf.MetadataStrategyAllSources || slices.Contains(needed, src) {
			continue
		}
		needed = append(needed, src)
	}
	if len(needed) == 0 {
		return base, ""
	}

	known := map[enums2.SourceType]string{}
	for _, ref := range game.MetadataSources {
		if id := strings.TrimSpace(ref.SourceID); id != "" {
			known[gamehelper.NormalizeMetadataSourceType(ref.SourceType)] = id
		}
	}
	if id := strings.TrimSpace(game.SourceID); id != "" && baseSource != "" {
		known[baseSource] = id
	}
	mapped := s.mappedMetadataSourceIDs(known)

	var others []vo.GameMetadataFromWebVO
	for _, src := range needed {
		id := known[src]
		if id == "" {
			id = mapped[src]
		}
		if id == "" || !s.isMetadataSourceEnabled(src) {
			continue
		}
		result, err := s.fetchMetadataResultBySource(ctx, src, strings.ToLower(id))
		if err != nil {
			applog.LogWarningf(s.ctx, "composeRefreshResult: fetch %s/%s for %s failed: %v", src, id, game.Name, err)
			continue
		}
		if gamehelper.IsEmptyGame(result.Game) {
			continue
		}
		others = append(others, vo.GameMetadataFromWebVO{Source: src, Game: result.Game, Tags: result.Tags})
	}

	composed, sources := gamehelper.ComposeMetadata(strategy, vo.GameMetadataFromWebVO{Source: baseSource, Game: base.Game, Tags: base.Tags}, others)
	return metadata.MetadataResult{Game: composed.Game, Tags: composed.Tags}, sources[enums2.MetadataUpdateFieldTags]
}

// composeImportMatches 在批量导入匹配时套用高级组合策略：matches[0] 是基底的结果，其余是同一个游戏在其他来源的结果。
// 策略对这个基底不生效时返回 nil。
func (s *ImportService) composeImportMatches(base enums2.SourceType, matches []vo.GameMetadataFromWebVO) (*vo.GameMetadataFromWebVO, gamehelper.FieldSources) {
	if s.config == nil || len(matches) == 0 {
		return nil, nil
	}
	strategy := s.config.Strategy()
	if strategy.Base != base || len(strategy.Overrides(base)) == 0 {
		return nil, nil
	}
	composed, sources := gamehelper.ComposeMetadata(strategy, matches[0], matches[1:])
	return &composed, sources
}
