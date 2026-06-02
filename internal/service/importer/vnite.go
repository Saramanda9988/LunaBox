package importer

import (
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/vnite"
	"lunabox/internal/utils/imageutils"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VniteImporter struct {
	deps Dependencies
}

func NewVniteImporter(deps Dependencies) *VniteImporter {
	return &VniteImporter{deps: deps}
}

func (v *VniteImporter) Preview(vniteDir string) ([]PreviewGame, error) {
	startedAt := time.Now()
	stepStartedAt := time.Now()
	data, err := vnite.LoadExportData(vniteDir)
	if err != nil {
		applog.LogErrorf(v.deps.Ctx, "PreviewVniteImport: failed to load vnite data: %v", err)
		return nil, fmt.Errorf("读取 Vnite 导出目录失败: %w", err)
	}
	applog.LogInfof(v.deps.Ctx, "PreviewVniteImport: loaded data games=%d locals=%d elapsed=%s", len(data.GameDocs), len(data.GameLocalDocs), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	existingGames, _, _, err := v.deps.existingGames("PreviewVniteImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)
	applog.LogInfof(v.deps.Ctx, "PreviewVniteImport: loaded existing games=%d elapsed=%s", len(existingGames), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	allIDs := collectVniteIDs(data)
	previews := make([]PreviewGame, 0, len(allIDs))
	for id := range allIDs {
		gameDoc, hasGame := data.GameDocs[id]
		localDoc, hasLocal := data.GameLocalDocs[id]
		if !hasGame {
			continue
		}

		name := pickVniteName(gameDoc)
		if name == "" {
			continue
		}
		exePath := ""
		if hasLocal {
			exePath = pickVniteGamePath(localDoc)
		}
		sourceType := string(mapVniteSourceType(gameDoc))

		previews = append(previews, PreviewGame{
			Name:       name,
			Developer:  pickVniteDeveloper(gameDoc),
			SourceType: sourceType,
			Exists:     previewExists(existingIndex, name, exePath, sourceType, pickVniteSourceID(gameDoc)),
			AddTime:    parseVniteTimeOrNow(gameDoc.Record.AddDate),
			HasPath:    hasLocal && exePath != "",
		})
	}

	applog.LogInfof(v.deps.Ctx, "PreviewVniteImport: built previews=%d elapsed=%s total=%s", len(previews), time.Since(stepStartedAt), time.Since(startedAt))
	return previews, nil
}

func (v *VniteImporter) Import(vniteDir string, skipNoPath bool) (ImportResult, error) {
	result := newImportResult()

	startedAt := time.Now()
	stepStartedAt := time.Now()
	data, err := vnite.LoadExportData(vniteDir)
	if err != nil {
		applog.LogErrorf(v.deps.Ctx, "ImportFromVnite: failed to load vnite data: %v", err)
		return result, fmt.Errorf("读取 Vnite 导出目录失败: %w", err)
	}
	applog.LogInfof(v.deps.Ctx, "ImportFromVnite: loaded data games=%d locals=%d elapsed=%s", len(data.GameDocs), len(data.GameLocalDocs), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	existingGames, existingNames, existingPaths, err := v.deps.existingGames("ImportFromVnite")
	if err != nil {
		return result, err
	}
	applog.LogInfof(v.deps.Ctx, "ImportFromVnite: loaded existing games=%d elapsed=%s", len(existingGames), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	allIDs := collectVniteIDs(data)
	items := make([]ImportItem, 0, len(allIDs))
	for id := range allIDs {
		gameDoc, hasGame := data.GameDocs[id]
		localDoc := data.GameLocalDocs[id]
		if !hasGame {
			continue
		}

		gameName := pickVniteName(gameDoc)
		if gameName == "" {
			continue
		}

		exePath := pickVniteGamePath(localDoc)
		hasPath := exePath != ""
		if skipExistingGame(v.deps.Ctx, "ImportFromVnite", &result, existingGames, existingNames, existingPaths, gameName, exePath) {
			continue
		}

		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		game, sessions := v.convertToGame(gameDoc, localDoc)
		source := vo.GameMetadataFromWebVO{
			Source: game.SourceType,
			Game:   game,
			Tags:   tagsFromNames(gameDoc.Metadata.Tags),
		}
		items = append(items, ImportItem{
			Source:      source,
			Sessions:    sessions,
			DisplayName: gameName,
			Path:        exePath,
			CoverLoader: v.coverLoader(vniteDir, gameDoc),
		})
		updateExistingIndexes(existingNames, existingPaths, game, gameName, exePath)
	}
	applog.LogInfof(v.deps.Ctx, "ImportFromVnite: built import items=%d skipped=%d failed=%d elapsed=%s", len(items), result.Skipped, result.Failed, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	batchResult, err := addImportedItems(v.deps, items)
	if err != nil {
		applog.LogErrorf(v.deps.Ctx, "ImportFromVnite: failed to batch add games: %v", err)
		return result, err
	}
	applog.LogInfof(v.deps.Ctx, "ImportFromVnite: committed batch success=%d skipped=%d failed=%d sessions=%d elapsed=%s", batchResult.Success, batchResult.Skipped, batchResult.Failed, batchResult.SessionsImported, time.Since(stepStartedAt))
	result.Success += batchResult.Success
	result.Skipped += batchResult.Skipped
	result.Failed += batchResult.Failed
	result.SessionsImported += batchResult.SessionsImported
	result.SkippedNames = append(result.SkippedNames, batchResult.SkippedNames...)
	result.FailedNames = append(result.FailedNames, batchResult.FailedNames...)

	applog.LogInfof(v.deps.Ctx, "ImportFromVnite: complete success=%d skipped=%d failed=%d sessions=%d total=%s", result.Success, result.Skipped, result.Failed, result.SessionsImported, time.Since(startedAt))
	return result, nil
}

func collectVniteIDs(data *vnite.ExportData) map[string]bool {
	allIDs := make(map[string]bool)
	for id := range data.GameDocs {
		allIDs[id] = true
	}
	for id := range data.GameLocalDocs {
		allIDs[id] = true
	}
	return allIDs
}

func (v *VniteImporter) convertToGame(gameDoc vnite.GameDoc, localDoc vnite.GameLocalDoc) (models.Game, []models.PlaySession) {
	gameID := uuid.New().String()
	game := models.Game{
		ID:                gameID,
		Name:              pickVniteName(gameDoc),
		Company:           pickVniteDeveloper(gameDoc),
		Summary:           gameDoc.Metadata.Description,
		ReleaseDate:       strings.TrimSpace(gameDoc.Metadata.ReleaseDate),
		Path:              pickVniteGamePath(localDoc),
		SavePath:          pickVniteSavePath(localDoc),
		ProcessName:       pickVniteProcessName(localDoc),
		SourceType:        mapVniteSourceType(gameDoc),
		SourceID:          pickVniteSourceID(gameDoc),
		CreatedAt:         parseVniteTimeOrNow(gameDoc.Record.AddDate),
		CachedAt:          time.Now(),
		UseLocaleEmulator: pickVniteUseLocaleEmulator(localDoc),
		UseMagpie:         localDoc.Launcher.UseMagpie,
	}

	sessions := parseVniteTimers(gameID, gameDoc.Record.Timers)
	return game, sessions
}

func (v *VniteImporter) applyCover(game *models.Game, vniteDir string, gameDoc vnite.GameDoc) {
	coverURL, err := v.loadCover(game.ID, game.Name, vniteDir, gameDoc)
	if err != nil {
		applog.LogWarningf(v.deps.Ctx, "ImportFromVnite: failed to save cover image for game %s: %v", game.Name, err)
		return
	}
	game.CoverURL = coverURL
}

func (v *VniteImporter) coverLoader(vniteDir string, gameDoc vnite.GameDoc) func(models.Game) (string, error) {
	return func(game models.Game) (string, error) {
		return v.loadCover(game.ID, game.Name, vniteDir, gameDoc)
	}
}

func (v *VniteImporter) loadCover(gameID string, gameName string, vniteDir string, gameDoc vnite.GameDoc) (string, error) {
	coverBytes, ext, err := vnite.LoadGameCoverBytes(vniteDir, gameDoc)
	if err != nil {
		return "", err
	}
	if len(coverBytes) == 0 {
		return "", nil
	}

	if ext == "" {
		ext = ".jpg"
	}

	tempFile, err := os.CreateTemp("", "vnite_cover_*"+ext)
	if err != nil {
		return "", err
	}
	tempFilePath := tempFile.Name()
	if _, err := tempFile.Write(coverBytes); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFilePath)
		return "", err
	}
	defer os.Remove(tempFilePath)

	savedPath, err := imageutils.SaveCoverImage(tempFilePath, gameID)
	if err != nil {
		return "", fmt.Errorf("save cover for %s: %w", gameName, err)
	}

	return savedPath, nil
}

func pickVniteName(gameDoc vnite.GameDoc) string {
	if gameDoc.Metadata.Name != "" {
		return gameDoc.Metadata.Name
	}
	return gameDoc.Metadata.OriginalName
}

func pickVniteDeveloper(gameDoc vnite.GameDoc) string {
	if len(gameDoc.Metadata.Developers) > 0 {
		return gameDoc.Metadata.Developers[0]
	}
	if len(gameDoc.Metadata.Publishers) > 0 {
		return gameDoc.Metadata.Publishers[0]
	}
	return ""
}

func pickVniteGamePath(localDoc vnite.GameLocalDoc) string {
	if strings.EqualFold(strings.TrimSpace(localDoc.Launcher.Mode), "file") {
		if path := strings.TrimSpace(localDoc.Launcher.FileConfig.Path); path != "" {
			return path
		}
	}
	if path := strings.TrimSpace(localDoc.Path.GamePath); path != "" {
		return path
	}
	return strings.TrimSpace(localDoc.Utils.MarkPath)
}

func pickVniteSavePath(localDoc vnite.GameLocalDoc) string {
	if len(localDoc.Path.SavePaths) > 0 {
		return strings.TrimSpace(localDoc.Path.SavePaths[0])
	}
	return ""
}

func pickVniteProcessName(localDoc vnite.GameLocalDoc) string {
	if !strings.EqualFold(strings.TrimSpace(localDoc.Launcher.Mode), "file") {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(localDoc.Launcher.FileConfig.MonitorMode), "process") {
		return ""
	}
	return strings.TrimSpace(localDoc.Launcher.FileConfig.MonitorPath)
}

func pickVniteUseLocaleEmulator(localDoc vnite.GameLocalDoc) bool {
	return localDoc.Launcher.UseLocaleEmulator || localDoc.Launcher.RunInLocaleEmulator
}

func pickVniteSourceID(gameDoc vnite.GameDoc) string {
	if gameDoc.Metadata.VNDBID != "" {
		return gameDoc.Metadata.VNDBID
	}
	if gameDoc.Metadata.YmgalID != "" {
		return gameDoc.Metadata.YmgalID
	}
	if gameDoc.Metadata.BangumiID != "" {
		return gameDoc.Metadata.BangumiID
	}
	if gameDoc.Metadata.SteamID != "" {
		return gameDoc.Metadata.SteamID
	}
	return ""
}

func mapVniteSourceType(gameDoc vnite.GameDoc) enums.SourceType {
	if gameDoc.Metadata.VNDBID != "" {
		return enums.VNDB
	}
	if gameDoc.Metadata.YmgalID != "" {
		return enums.Ymgal
	}
	if gameDoc.Metadata.BangumiID != "" {
		return enums.Bangumi
	}
	if gameDoc.Metadata.SteamID != "" {
		return enums.Steam
	}
	return enums.Local
}

func parseVniteTimeOrNow(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}

	return time.Now()
}

func parseVniteTimers(gameID string, timers []vnite.GameTimer) []models.PlaySession {
	sessions := make([]models.PlaySession, 0, len(timers))
	for _, timer := range timers {
		if timer.Start == "" || timer.End == "" {
			continue
		}

		startTime := parseVniteTimeOrNow(timer.Start)
		endTime := parseVniteTimeOrNow(timer.End)
		duration := int(endTime.Sub(startTime).Seconds())
		if duration <= 0 {
			continue
		}

		sessions = append(sessions, models.PlaySession{
			ID:        uuid.New().String(),
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		})
	}

	return sessions
}
