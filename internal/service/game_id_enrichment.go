package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"lunabox/internal/common/enums"
	"lunabox/internal/service/cloudsync"
	"lunabox/internal/service/gamehelper/idmapper"
	"lunabox/internal/utils/dbutils"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

const (
	gameIDEnrichmentStagingTable = "temp_game_id_enrichment"
	gameIDEnrichmentNewTable     = "temp_game_id_enrichment_new"
)

// GameIDEnrichmentResult summarizes one library-wide ID enrichment operation.
type GameIDEnrichmentResult struct {
	ScannedGames   int `json:"scanned_games"`
	MatchedGames   int `json:"matched_games"`
	UpdatedGames   int `json:"updated_games"`
	AddedSources   int `json:"added_sources"`
	UnmatchedGames int `json:"unmatched_games"`
	SkippedGames   int `json:"skipped_games"`
}

type GameIDEnrichmentSource struct {
	SourceType enums.SourceType `json:"source_type"`
	SourceID   string           `json:"source_id"`
}

type GameIDEnrichmentPreviewItem struct {
	GameID          string                   `json:"game_id"`
	GameName        string                   `json:"game_name"`
	DefaultSource   enums.SourceType         `json:"default_source"`
	DefaultSourceID string                   `json:"default_source_id"`
	ExistingSources []GameIDEnrichmentSource `json:"existing_sources"`
	AddedSources    []GameIDEnrichmentSource `json:"added_sources"`
	CanEnrich       bool                     `json:"can_enrich"`
	Reason          string                   `json:"reason"`
}

type GameIDEnrichmentPreview struct {
	ScannedGames    int                           `json:"scanned_games"`
	EnrichableGames int                           `json:"enrichable_games"`
	UnchangedGames  int                           `json:"unchanged_games"`
	AddedSources    int                           `json:"added_sources"`
	Items           []GameIDEnrichmentPreviewItem `json:"items"`
}

type gameIDEnrichmentCandidate struct {
	gameID        string
	gameName      string
	defaultSource enums.SourceType
	defaultID     string
	existing      map[enums.SourceType]string
}

type gameIDEnrichmentAddition struct {
	gameID   string
	source   enums.SourceType
	sourceID string
}

// EnrichLegacyGameMetadataSourceIDs fills missing Bangumi, VNDB, Steam, and Hikarinagi
// metadata sources for games whose default source is one of those providers.
func (s *GameService) EnrichLegacyGameMetadataSourceIDs() (GameIDEnrichmentResult, error) {
	result := GameIDEnrichmentResult{}
	preview, err := s.PreviewLegacyGameMetadataSourceIDs()
	if err != nil {
		return result, err
	}
	result.ScannedGames = preview.ScannedGames

	additions := make([]gameIDEnrichmentAddition, 0, preview.AddedSources)
	for _, item := range preview.Items {
		if item.Reason == "no_mapping" {
			result.UnmatchedGames++
			continue
		}
		result.MatchedGames++
		if !item.CanEnrich {
			result.SkippedGames++
			continue
		}
		for _, source := range item.AddedSources {
			additions = append(additions, gameIDEnrichmentAddition{
				gameID:   item.GameID,
				source:   source.SourceType,
				sourceID: source.SourceID,
			})
		}
	}

	if len(additions) == 0 {
		return result, nil
	}

	err = dbutils.WithDuckDBWriteLock(s.db, func() error {
		return dbutils.RetryDuckDBWriteConflict(s.ctx, func() error {
			addedSources, updatedGames, writeErr := s.insertEnrichedGameIDSources(additions)
			if writeErr != nil {
				return writeErr
			}
			result.AddedSources = addedSources
			result.UpdatedGames = updatedGames
			return nil
		})
	})
	if err != nil {
		return GameIDEnrichmentResult{}, err
	}
	return result, nil
}

// PreviewLegacyGameMetadataSourceIDs calculates the same changes as enrichment
// without modifying the user's library.
func (s *GameService) PreviewLegacyGameMetadataSourceIDs() (GameIDEnrichmentPreview, error) {
	preview := GameIDEnrichmentPreview{Items: []GameIDEnrichmentPreviewItem{}}
	candidates, err := s.listGameIDEnrichmentCandidates()
	if err != nil {
		return preview, err
	}
	if len(candidates) == 0 {
		return preview, nil
	}
	mapper, err := s.getGameIDMapper()
	if err != nil {
		return preview, fmt.Errorf("加载游戏 ID 映射库失败: %w", err)
	}
	if mapper == nil {
		return preview, fmt.Errorf("游戏 ID 映射库未初始化")
	}
	preview.ScannedGames = len(candidates)
	preview.Items = make([]GameIDEnrichmentPreviewItem, 0, len(candidates))
	for _, candidate := range candidates {
		item := GameIDEnrichmentPreviewItem{
			GameID:          candidate.gameID,
			GameName:        candidate.gameName,
			DefaultSource:   candidate.defaultSource,
			DefaultSourceID: candidate.defaultID,
			ExistingSources: orderedGameIDEnrichmentSources(candidate.existing),
			AddedSources:    []GameIDEnrichmentSource{},
		}

		mapping, found := mapper.Resolve(candidate.defaultSource, candidate.defaultID)
		if !found {
			item.Reason = "no_mapping"
			preview.UnchangedGames++
			preview.Items = append(preview.Items, item)
			continue
		}

		item.AddedSources = missingGameIDSources(candidate, mapping)
		if len(item.AddedSources) == 0 {
			if len(candidate.existing) >= 3 {
				item.Reason = "already_complete"
			} else {
				item.Reason = "no_available_ids"
			}
			preview.UnchangedGames++
		} else {
			item.CanEnrich = true
			item.Reason = "can_enrich"
			preview.EnrichableGames++
			preview.AddedSources += len(item.AddedSources)
		}
		preview.Items = append(preview.Items, item)
	}
	return preview, nil
}

func (s *GameService) listGameIDEnrichmentCandidates() ([]gameIDEnrichmentCandidate, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT
			g.id,
			COALESCE(g.name, ''),
			LOWER(TRIM(COALESCE(g.source_type, ''))),
			TRIM(COALESCE(g.source_id, '')),
			COALESCE(LOWER(TRIM(source.source_type)), ''),
			COALESCE(TRIM(source.source_id), '')
		FROM games AS g
		LEFT JOIN game_metadata_sources AS source ON source.game_id = g.id
		WHERE LOWER(TRIM(COALESCE(g.source_type, ''))) IN ('bangumi', 'vndb', 'steam', 'hikarinagi')
		  AND TRIM(COALESCE(g.source_id, '')) <> ''
		ORDER BY LOWER(COALESCE(g.name, '')), g.id, source.source_type
	`)
	if err != nil {
		return nil, fmt.Errorf("查询待补全游戏失败: %w", err)
	}
	defer rows.Close()

	candidates := make([]gameIDEnrichmentCandidate, 0)
	indexByGameID := make(map[string]int)
	for rows.Next() {
		var gameID string
		var gameName string
		var defaultSource string
		var defaultID string
		var existingSource string
		var existingID string
		if err := rows.Scan(&gameID, &gameName, &defaultSource, &defaultID, &existingSource, &existingID); err != nil {
			return nil, fmt.Errorf("读取待补全游戏失败: %w", err)
		}

		candidateIndex, exists := indexByGameID[gameID]
		if !exists {
			candidates = append(candidates, gameIDEnrichmentCandidate{
				gameID:        gameID,
				gameName:      gameName,
				defaultSource: enums.SourceType(defaultSource),
				defaultID:     defaultID,
				existing:      make(map[enums.SourceType]string, 4),
			})
			candidateIndex = len(candidates) - 1
			indexByGameID[gameID] = candidateIndex
			candidate := &candidates[candidateIndex]
			candidate.existing[candidate.defaultSource] = candidate.defaultID
		}
		candidate := &candidates[candidateIndex]

		sourceType := enums.SourceType(existingSource)
		if existingID != "" && isGameIDEnrichmentSource(sourceType) {
			candidate.existing[sourceType] = existingID
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历待补全游戏失败: %w", err)
	}
	return candidates, nil
}

func missingGameIDSources(
	candidate gameIDEnrichmentCandidate,
	mapping idmapper.IDs,
) []GameIDEnrichmentSource {
	sources := make([]GameIDEnrichmentSource, 0, 3)
	values := []struct {
		source enums.SourceType
		id     int64
	}{
		{source: enums.Bangumi, id: mapping.BangumiID},
		{source: enums.VNDB, id: mapping.VNDBID},
		{source: enums.Steam, id: mapping.SteamID},
		{source: enums.Hikarinagi, id: mapping.HikarinagiID},
	}

	for _, value := range values {
		if value.id <= 0 {
			continue
		}
		if _, exists := candidate.existing[value.source]; exists {
			continue
		}

		sourceID := strconv.FormatInt(value.id, 10)
		if value.source == enums.VNDB {
			sourceID = "v" + sourceID
		}
		sources = append(sources, GameIDEnrichmentSource{
			SourceType: value.source,
			SourceID:   sourceID,
		})
	}
	return sources
}

func orderedGameIDEnrichmentSources(existing map[enums.SourceType]string) []GameIDEnrichmentSource {
	sources := make([]GameIDEnrichmentSource, 0, len(existing))
	for _, sourceType := range []enums.SourceType{enums.Bangumi, enums.VNDB, enums.Steam, enums.Hikarinagi} {
		if sourceID, exists := existing[sourceType]; exists {
			sources = append(sources, GameIDEnrichmentSource{SourceType: sourceType, SourceID: sourceID})
		}
	}
	return sources
}

func (s *GameService) insertEnrichedGameIDSources(additions []gameIDEnrichmentAddition) (int, int, error) {
	conn, err := s.db.Conn(s.ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("获取游戏 ID 补全数据库连接失败: %w", err)
	}
	defer conn.Close()
	defer cleanupGameIDEnrichmentStagingTables(s.ctx, conn)

	if _, err := conn.ExecContext(s.ctx, `BEGIN TRANSACTION`); err != nil {
		return 0, 0, fmt.Errorf("开始补全游戏 ID 事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(s.ctx, `ROLLBACK`)
		}
	}()

	if _, err := conn.ExecContext(s.ctx, `DROP TABLE IF EXISTS temp_game_id_enrichment`); err != nil {
		return 0, 0, fmt.Errorf("清理游戏 ID 补全临时表失败: %w", err)
	}
	if _, err := conn.ExecContext(s.ctx, `DROP TABLE IF EXISTS temp_game_id_enrichment_new`); err != nil {
		return 0, 0, fmt.Errorf("清理游戏 ID 补全结果临时表失败: %w", err)
	}
	if _, err := conn.ExecContext(s.ctx, `
		CREATE TEMP TABLE temp_game_id_enrichment (
			game_id TEXT,
			source_type TEXT,
			source_id TEXT,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)
	`); err != nil {
		return 0, 0, fmt.Errorf("创建游戏 ID 补全临时表失败: %w", err)
	}

	now := time.Now()
	if err := dbutils.AppendRows(
		s.ctx,
		conn,
		"",
		gameIDEnrichmentStagingTable,
		func(appender *duckdb.Appender) error {
			for _, addition := range additions {
				if err := appender.AppendRow(
					addition.gameID,
					string(addition.source),
					addition.sourceID,
					now,
					now,
				); err != nil {
					return fmt.Errorf("追加游戏 %s 的 %s ID 失败: %w", addition.gameID, addition.source, err)
				}
			}
			return nil
		},
	); err != nil {
		return 0, 0, err
	}

	if _, err := conn.ExecContext(s.ctx, `
		CREATE TEMP TABLE temp_game_id_enrichment_new AS
		SELECT DISTINCT staging.*
		FROM temp_game_id_enrichment AS staging
		WHERE NOT EXISTS (
			SELECT 1
			FROM game_metadata_sources AS existing
			WHERE existing.game_id = staging.game_id
			  AND existing.source_type = staging.source_type
		)
	`); err != nil {
		return 0, 0, fmt.Errorf("筛选待写入游戏 ID 失败: %w", err)
	}

	var addedSources int
	var updatedGames int
	if err := conn.QueryRowContext(s.ctx, `
		SELECT COUNT(*), COUNT(DISTINCT game_id)
		FROM temp_game_id_enrichment_new
	`).Scan(&addedSources, &updatedGames); err != nil {
		return 0, 0, fmt.Errorf("统计待写入游戏 ID 失败: %w", err)
	}

	if addedSources > 0 {
		if _, err := conn.ExecContext(s.ctx, `
			INSERT INTO game_metadata_sources (
				game_id, source_type, source_id, cached_at, created_at, updated_at
			)
			SELECT game_id, source_type, source_id, NULL, created_at, updated_at
			FROM temp_game_id_enrichment_new
		`); err != nil {
			return 0, 0, fmt.Errorf("批量写入游戏 ID 失败: %w", err)
		}

		if _, err := conn.ExecContext(s.ctx, `
			DELETE FROM sync_tombstones
			WHERE entity_type = ?
			  AND EXISTS (
				SELECT 1
				FROM temp_game_id_enrichment_new AS source
				WHERE sync_tombstones.entity_id = source.game_id || '::' || source.source_type
			  )
		`, cloudsync.EntityGameMetadataSource); err != nil {
			return 0, 0, fmt.Errorf("批量清理游戏 ID 同步删除记录失败: %w", err)
		}
	}

	if _, err := conn.ExecContext(s.ctx, `COMMIT`); err != nil {
		return 0, 0, fmt.Errorf("提交游戏 ID 补全事务失败: %w", err)
	}
	committed = true
	return addedSources, updatedGames, nil
}

func cleanupGameIDEnrichmentStagingTables(ctx context.Context, conn *sql.Conn) {
	_, _ = conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_game_id_enrichment_new`)
	_, _ = conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_game_id_enrichment`)
}

func isGameIDEnrichmentSource(source enums.SourceType) bool {
	switch enums.SourceType(strings.ToLower(strings.TrimSpace(string(source)))) {
	case enums.Bangumi, enums.VNDB, enums.Steam, enums.Hikarinagi:
		return true
	default:
		return false
	}
}
