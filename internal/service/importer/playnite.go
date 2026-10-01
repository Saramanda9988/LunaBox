package importer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/playnite"
	"lunabox/internal/service/gamehelper"
	"lunabox/internal/utils/imageutils"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type PlayniteImporter struct {
	deps Dependencies
}

type playniteImportData struct {
	games   []playnite.PlayniteGame
	archive *zip.ReadCloser
	covers  map[string]*zip.File
}

func (data playniteImportData) Close() {
	if data.archive != nil {
		_ = data.archive.Close()
	}
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

	data, err := p.readGames(jsonPath)
	if err != nil {
		return result, err
	}
	defer data.Close()

	existingGames, existingNames, existingPaths, err := p.deps.existingGames("ImportFromPlaynite")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(data.games))
	for _, pg := range data.games {
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
		gameForConversion := pg
		if data.archive != nil {
			gameForConversion.CoverURL = ""
		}
		game := p.convertToGameWithCover(gameForConversion, existingGameID, action != ImportActionMergeSessions)
		if data.archive != nil && action != ImportActionMergeSessions {
			game.CoverURL = p.importPackagedCover(pg.CoverURL, game.ID, data.covers)
		}
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
	data, err := p.readGames(jsonPath)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	existingGames, _, _, err := p.deps.existingGames("PreviewPlayniteImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(data.games))
	for _, pg := range data.games {
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

func (p *PlayniteImporter) readGames(importPath string) (playniteImportData, error) {
	var data playniteImportData
	var jsonData []byte
	var err error
	if strings.EqualFold(filepath.Ext(importPath), ".zip") {
		data.archive, err = zip.OpenReader(importPath)
		if err != nil {
			return data, fmt.Errorf("无法打开 Playnite ZIP 文件: %w", err)
		}
		data.covers = make(map[string]*zip.File)
		var jsonFile *zip.File
		for _, file := range data.archive.File {
			if file.Name == "games.json" {
				jsonFile = file
			} else if isPlayniteCoverEntry(file.Name) && !file.FileInfo().IsDir() {
				data.covers[file.Name] = file
			}
		}
		if jsonFile == nil {
			data.Close()
			return playniteImportData{}, fmt.Errorf("Playnite ZIP 文件中缺少 games.json")
		}
		reader, openErr := jsonFile.Open()
		if openErr != nil {
			data.Close()
			return playniteImportData{}, fmt.Errorf("无法读取 games.json: %w", openErr)
		}
		jsonData, err = io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			data.Close()
			return playniteImportData{}, fmt.Errorf("无法读取 games.json: %w", err)
		}
	} else {
		jsonData, err = os.ReadFile(importPath)
		if err != nil {
			applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to read JSON file: %v", err)
			return data, fmt.Errorf("无法读取 JSON 文件: %w", err)
		}
	}

	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	if err := json.Unmarshal(jsonData, &data.games); err != nil {
		data.Close()
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to unmarshal JSON: %v", err)
		return playniteImportData{}, fmt.Errorf("解析 Playnite 游戏数据失败: %w", err)
	}
	return data, nil
}

func isPlayniteCoverEntry(name string) bool {
	if !strings.HasPrefix(name, "covers/") || path.Clean(name) != name || strings.ContainsAny(name, "\\\x00") {
		return false
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".avif", ".bmp":
		return true
	default:
		return false
	}
}

func (p *PlayniteImporter) importPackagedCover(coverURL, gameID string, covers map[string]*zip.File) string {
	if coverURL == "" || gamehelper.IsDownloadableCoverURL(coverURL) {
		return coverURL
	}
	if !isPlayniteCoverEntry(coverURL) {
		applog.LogWarningf(p.deps.Ctx, "PlayniteImporter: invalid packaged cover path: %s", coverURL)
		return ""
	}
	file := covers[coverURL]
	if file == nil {
		applog.LogWarningf(p.deps.Ctx, "PlayniteImporter: packaged cover missing: %s", coverURL)
		return ""
	}
	source, err := file.Open()
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to open packaged cover %s: %v", coverURL, err)
		return ""
	}
	defer source.Close()
	tempFile, err := os.CreateTemp("", "playnite_cover_*"+path.Ext(coverURL))
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to create temporary cover: %v", err)
		return ""
	}
	defer os.Remove(tempFile.Name())
	copied, copyErr := io.Copy(tempFile, io.LimitReader(source, (64<<20)+1))
	closeErr := tempFile.Close()
	if copyErr != nil || closeErr != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to extract packaged cover %s: %v %v", coverURL, copyErr, closeErr)
		return ""
	}
	if copied > 64<<20 {
		applog.LogWarningf(p.deps.Ctx, "PlayniteImporter: packaged cover exceeds 64 MiB: %s", coverURL)
		return ""
	}
	savedPath, err := imageutils.SaveCoverImage(tempFile.Name(), gameID)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PlayniteImporter: failed to save packaged cover %s: %v", coverURL, err)
		return ""
	}
	return savedPath
}

func (p *PlayniteImporter) convertToGame(pg playnite.PlayniteGame, gameID string) models.Game {
	return p.convertToGameWithCover(pg, gameID, true)
}

func (p *PlayniteImporter) convertToGameWithCover(pg playnite.PlayniteGame, gameID string, importCover bool) models.Game {
	if gameID == "" {
		gameID = pg.ID
	}
	game := models.Game{
		ID:                 gameID,
		Name:               pg.Name,
		Company:            pg.Company,
		Summary:            pg.Summary,
		Rating:             pg.Rating,
		ReleaseDate:        pg.ReleaseDate,
		Path:               pg.Path,
		GameDirectory:      strings.TrimSpace(pg.GameDirectory),
		ProcessName:        strings.TrimSpace(pg.ProcessName),
		Status:             stringToGameStatus(pg.Status),
		SourceType:         stringToSourceType(pg.SourceType),
		SourceID:           pg.SourceID,
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
	case string(enums.StatusNotStarted):
		return enums.StatusNotStarted
	case string(enums.StatusWantToPlay):
		return enums.StatusWantToPlay
	case string(enums.StatusPlaying):
		return enums.StatusPlaying
	case string(enums.StatusCompleted):
		return enums.StatusCompleted
	case string(enums.StatusOnHold):
		return enums.StatusOnHold
	case string(enums.StatusDropped):
		return enums.StatusDropped
	default:
		return enums.StatusNotStarted
	}
}
