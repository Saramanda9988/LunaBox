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
	"lunabox/internal/service/gamehelper"
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
	return p.ImportSelected(jsonPath, skipNoPath, samePathAction, nil)
}

func (p *PlayniteImporter) ImportSelected(jsonPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

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
		if !selectionFilter.includes(pg.Name, pg.Path, pg.SourceType, pg.SourceID) {
			continue
		}
		if skipNoPath && pg.Path == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, pg.Name+" (无路径)")
			continue
		}

		action := ImportActionCreate
		existingGameID := ""
		if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, pg.Name, pg.Path); exists {
			if conflict.Type != ConflictTypeSamePath || !IsSamePathMergeAction(samePathAction) {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, pg.Name+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, pg.Name+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			if samePathAction == SamePathActionMergeSessions {
				action = ImportActionMergeSessions
			}
			existingGameID = conflict.Game.ID
		}
		game := p.convertToGameWithCover(pg, existingGameID, action != ImportActionMergeSessions)
		if TargetsExistingGame(action) {
			game.Path = pg.Path
		}

		source := vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
			Tags:   tagsFromNames(pg.Tags),
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
			SourceID:     pg.SourceID,
			Path:         pg.Path,
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
	return p.convertToGameWithCover(pg, gameID, true)
}

func (p *PlayniteImporter) convertToGameWithCover(pg playnite.PlayniteGame, gameID string, importCover bool) models.Game {
	if gameID == "" {
		gameID = pg.ID
	}
	game := models.Game{
		ID:              gameID,
		Name:            pg.Name,
		Company:         pg.Company,
		Summary:         pg.Summary,
		Rating:          pg.Rating,
		ReleaseDate:     pg.ReleaseDate,
		Path:            pg.Path,
		GameDirectory:   strings.TrimSpace(pg.GameDirectory),
		ProcessName:     strings.TrimSpace(pg.ProcessName),
		Status:          stringToGameStatus(pg.Status),
		SourceType:      stringToSourceType(pg.SourceType),
		SourceID:        pg.SourceID,
		LaunchMode:         enums.NormalizeLaunchMode(enums.LaunchMode(pg.LaunchMode)),
		SteamLaunchID:      strings.TrimSpace(pg.SteamLaunchID),
		SteamLaunchKind:    strings.TrimSpace(pg.SteamLaunchKind),
		SteamLaunchOptions: strings.TrimSpace(pg.SteamLaunchOptions),
		CreatedAt:          pg.CreatedAt,
		CachedAt:           time.Now(),
	}
	if game.SteamLaunchID != "" {
		game.LaunchMode = enums.LaunchModeSteam
	}
	if game.GameDirectory == "" {
		game.GameDirectory = gamehelper.DefaultGameDirectory(game.Path)
	}
	game.CoverSourceURL = strings.TrimSpace(pg.CoverSourceURL)
	if game.CoverSourceURL == "" && gamehelper.IsDownloadableCoverURL(pg.CoverURL) {
		game.CoverSourceURL = strings.TrimSpace(pg.CoverURL)
	}

	if pg.SavePath != nil {
		game.SavePath = *pg.SavePath
	}

	if importCover && pg.CoverURL != "" {
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

func stringToGameStatus(status string) enums.GameStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(enums.StatusWantToPlay):
		return enums.StatusWantToPlay
	case string(enums.StatusPlaying):
		return enums.StatusPlaying
	case string(enums.StatusCompleted):
		return enums.StatusCompleted
	case string(enums.StatusOnHold):
		return enums.StatusOnHold
	default:
		return enums.StatusNotStarted
	}
}
