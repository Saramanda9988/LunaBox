package importer

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/potatovn"
	"lunabox/internal/utils/archiveutils"
	"lunabox/internal/utils/imageutils"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type PotatoVNImporter struct {
	deps Dependencies
}

func NewPotatoVNImporter(deps Dependencies) *PotatoVNImporter {
	return &PotatoVNImporter{deps: deps}
}

func (p *PotatoVNImporter) Import(zipPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)

	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to open ZIP file: %v", err)
		return result, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	tempDir, err := os.MkdirTemp("", "potatovn_import_*")
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to create temp dir: %v", err)
		return result, fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := archiveutils.ExtractZip(zipReader, tempDir); err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to extract ZIP: %v", err)
		return result, fmt.Errorf("解压失败: %w", err)
	}

	galgames, err := p.readGalgames(filepath.Join(tempDir, "data.galgames.json"))
	if err != nil {
		return result, err
	}

	existingGames, existingNames, existingPaths, err := p.deps.existingGames("ImportFromPotatoVN")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(galgames))
	for _, galgame := range galgames {
		gameName := galgame.GetDisplayName()
		exePath := galgame.GetExePath()
		hasPath := exePath != ""

		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		action := ImportActionCreate
		existingGameID := ""
		if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, gameName, exePath); exists {
			if conflict.Type != ConflictTypeSamePath || samePathAction != SamePathActionMerge {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			existingGameID = conflict.Game.ID
		}
		game, sessions := p.convertToGame(galgame, tempDir, existingGameID)
		if action == ImportActionUpdateExisting {
			game.Path = exePath
			for i := range sessions {
				sessions[i].GameID = existingGameID
			}
		}

		source := vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
			Tags:   tagsFromNames(galgame.Tags.Value),
		}
		items = append(items, ImportItem{
			Source:         source,
			Sessions:       sessions,
			DisplayName:    gameName,
			Path:           exePath,
			Action:         action,
			ExistingGameID: existingGameID,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, gameName, exePath)
		}
	}

	batchResult, err := addImportedItems(p.deps, items)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to batch add games: %v", err)
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

func (p *PotatoVNImporter) Preview(zipPath string) ([]PreviewGame, error) {
	galgames, err := p.readGalgamesFromZip(zipPath)
	if err != nil {
		return nil, err
	}

	existingGames, _, _, err := p.deps.existingGames("PreviewImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(galgames))
	for _, galgame := range galgames {
		name := galgame.GetDisplayName()
		sourceType := string(mapPotatoVNRssTypeToSourceType(galgame.RssType))
		conflict := previewConflict(existingIndex, name, galgame.GetExePath(), sourceType, galgame.GetSourceID())
		previews = append(previews, PreviewGame{
			Name:         name,
			Developer:    galgame.Developer.Value,
			SourceType:   sourceType,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      galgame.AddTime.ToTime(),
			HasPath:      galgame.GetExePath() != "",
		})
	}

	return previews, nil
}

func (p *PotatoVNImporter) readGalgames(path string) ([]potatovn.Galgame, error) {
	galgamesData, err := os.ReadFile(path)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to read data.galgames.json: %v", err)
		return nil, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to unmarshal data.galgames.json: %v", err)
		return nil, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}
	return galgames, nil
}

func (p *PotatoVNImporter) readGalgamesFromZip(zipPath string) ([]potatovn.Galgame, error) {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to open ZIP file: %v", err)
		return nil, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		if file.Name != "data.galgames.json" {
			continue
		}

		srcFile, err := file.Open()
		if err != nil {
			applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to open data.galgames.json in ZIP: %v", err)
			return nil, err
		}
		defer srcFile.Close()

		data, err := io.ReadAll(srcFile)
		if err != nil {
			applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to read data.galgames.json: %v", err)
			return nil, err
		}

		var galgames []potatovn.Galgame
		if err := json.Unmarshal(data, &galgames); err != nil {
			applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to unmarshal data.galgames.json: %v", err)
			return nil, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
		}
		return galgames, nil
	}

	applog.LogWarningf(p.deps.Ctx, "PreviewImport: data.galgames.json not found in ZIP: %s", zipPath)
	return nil, fmt.Errorf("无法读取 data.galgames.json: 文件不存在")
}

func (p *PotatoVNImporter) convertToGame(galgame potatovn.Galgame, tempDir string, gameID string) (models.Game, []models.PlaySession) {
	if gameID == "" {
		gameID = uuid.New().String()
	}
	game := models.Game{
		ID:                gameID,
		Name:              galgame.GetDisplayName(),
		Company:           galgame.Developer.Value,
		Summary:           galgame.Description.Value,
		Rating:            galgame.Rating.Value,
		ReleaseDate:       formatPotatoVNDate(galgame.ReleaseDate.Value),
		Path:              galgame.GetExePath(),
		SavePath:          galgame.GetSavePath(),
		ProcessName:       galgame.GetProcessName(),
		SourceType:        mapPotatoVNRssTypeToSourceType(galgame.RssType),
		SourceID:          galgame.GetSourceID(),
		CreatedAt:         galgame.AddTime.ToTime(),
		CachedAt:          time.Now(),
		UseLocaleEmulator: galgame.RunInLocaleEmulator,
		UseMagpie:         galgame.EnableMagpie,
	}

	if galgame.ImagePath.Value != "" && galgame.ImagePath.Value != potatovn.DefaultImagePath {
		coverPath := imageutils.ResolveCoverPath(galgame.ImagePath.Value, tempDir)
		if coverPath != "" {
			savedPath, err := imageutils.SaveCoverImage(coverPath, game.ID)
			if err == nil {
				game.CoverURL = savedPath
			} else {
				applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to save cover image for game %s: %v", game.Name, err)
			}
		} else {
			applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: cover image not found for game %s, path: %s", game.Name, galgame.ImagePath.Value)
		}
	}

	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	var sessions []models.PlaySession
	if len(galgame.PlayedTime) > 0 {
		sessions = p.parsePlayedTime(gameID, galgame.PlayedTime)
	}

	return game, sessions
}

func formatPotatoVNDate(raw potatovn.FlexibleTime) string {
	releaseDate := raw.ToTime()
	if releaseDate.IsZero() {
		return ""
	}
	return releaseDate.Format("2006-01-02")
}

func mapPotatoVNRssTypeToSourceType(rssType potatovn.RssType) enums.SourceType {
	switch rssType {
	case potatovn.RssTypeBangumi:
		return enums.Bangumi
	case potatovn.RssTypeVndb:
		return enums.VNDB
	case potatovn.RssTypeYmgal:
		return enums.Ymgal
	case potatovn.RssTypeSteam:
		return enums.Steam
	default:
		return enums.Local
	}
}

func (p *PotatoVNImporter) parsePlayedTime(gameID string, playedTime map[string]int) []models.PlaySession {
	var sessions []models.PlaySession

	for dateStr, durationMinutes := range playedTime {
		if durationMinutes <= 0 {
			continue
		}

		parsedTime, err := time.Parse("2006/1/2", dateStr)
		if err != nil {
			parsedTime, err = time.Parse("2006/01/02", dateStr)
			if err != nil {
				applog.LogWarningf(p.deps.Ctx, "ImportFromPotatoVN: failed to parse date %s: %v", dateStr, err)
				continue
			}
		}

		startTime := time.Date(parsedTime.Year(), parsedTime.Month(), parsedTime.Day(), 12, 0, 0, 0, time.Local)
		durationSeconds := durationMinutes * 60
		endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

		sessions = append(sessions, models.PlaySession{
			ID:        uuid.New().String(),
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  durationSeconds,
		})
	}

	return sessions
}
