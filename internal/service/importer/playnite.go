package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/playnite"
	"lunabox/internal/utils/imageutils"
	"os"
	"strings"
	"time"
)

type PlayniteImporter struct {
	deps Dependencies
}

func NewPlayniteImporter(deps Dependencies) *PlayniteImporter {
	return &PlayniteImporter{deps: deps}
}

func (p *PlayniteImporter) Import(jsonPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)

	playniteGames, err := p.readGames(jsonPath)
	if err != nil {
		return result, err
	}

	existingGames, existingNames, existingPaths, err := p.deps.existingGames("ImportFromPlaynite")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(playniteGames))
	for _, pg := range playniteGames {
		if skipNoPath && pg.Path == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, pg.Name+" (无路径)")
			continue
		}

		action := ImportActionCreate
		existingGameID := ""
		if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, pg.Name, pg.Path); exists {
			if conflict.Type != ConflictTypeSamePath || samePathAction != SamePathActionMerge {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, pg.Name+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, pg.Name+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			existingGameID = conflict.Game.ID
		}
		game := p.convertToGame(pg, existingGameID)
		if action == ImportActionUpdateExisting {
			game.Path = pg.Path
		}

		source := vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
		}
		items = append(items, ImportItem{
			Source:         source,
			DisplayName:    pg.Name,
			Path:           pg.Path,
			Action:         action,
			ExistingGameID: existingGameID,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, pg.Name, pg.Path)
		}
	}

	batchResult, err := addImportedItems(p.deps, items)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPlaynite: failed to batch add games: %v", err)
		return result, err
	}
	result.Success += batchResult.Success
	result.Skipped += batchResult.Skipped
	result.Failed += batchResult.Failed
	result.SessionsImported += batchResult.SessionsImported
	result.SkippedNames = append(result.SkippedNames, batchResult.SkippedNames...)
	result.FailedNames = append(result.FailedNames, batchResult.FailedNames...)

	return result, nil
}

func (p *PlayniteImporter) Preview(jsonPath string) ([]PreviewGame, error) {
	playniteGames, err := p.readGames(jsonPath)
	if err != nil {
		return nil, err
	}

	existingGames, _, _, err := p.deps.existingGames("PreviewPlayniteImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(playniteGames))
	for _, pg := range playniteGames {
		conflict := previewConflict(existingIndex, pg.Name, pg.Path, pg.SourceType, pg.SourceID)
		previews = append(previews, PreviewGame{
			Name:         pg.Name,
			Developer:    pg.Company,
			SourceType:   pg.SourceType,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      pg.CreatedAt,
			HasPath:      pg.Path != "",
		})
	}

	return previews, nil
}

func (p *PlayniteImporter) readGames(jsonPath string) ([]playnite.PlayniteGame, error) {
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to read JSON file: %v", err)
		return nil, fmt.Errorf("无法读取 JSON 文件: %w", err)
	}

	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	var playniteGames []playnite.PlayniteGame
	if err := json.Unmarshal(jsonData, &playniteGames); err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to unmarshal JSON: %v", err)
		return nil, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}
	return playniteGames, nil
}

func (p *PlayniteImporter) convertToGame(pg playnite.PlayniteGame, gameID string) models.Game {
	if gameID == "" {
		gameID = pg.ID
	}
	game := models.Game{
		ID:          gameID,
		Name:        pg.Name,
		Company:     pg.Company,
		Summary:     pg.Summary,
		Rating:      pg.Rating,
		ReleaseDate: pg.ReleaseDate,
		Path:        pg.Path,
		SourceType:  stringToSourceType(pg.SourceType),
		SourceID:    pg.SourceID,
		CreatedAt:   pg.CreatedAt,
		CachedAt:    time.Now(),
	}

	if pg.SavePath != nil {
		game.SavePath = *pg.SavePath
	}

	if pg.CoverURL != "" {
		savedPath, err := imageutils.SaveCoverImage(pg.CoverURL, game.ID)
		if err == nil {
			game.CoverURL = savedPath
		} else {
			applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to save cover image for game %s: %v", game.Name, err)
			game.CoverURL = pg.CoverURL
		}
	}

	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	return game
}

func stringToSourceType(sourceType string) enums.SourceType {
	switch strings.ToLower(sourceType) {
	case "bangumi":
		return enums.Bangumi
	case "vndb":
		return enums.VNDB
	case "ymgal":
		return enums.Ymgal
	case "steam":
		return enums.Steam
	default:
		return enums.Local
	}
}
