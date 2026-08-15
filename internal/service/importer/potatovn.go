package importer

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/potatovn"
	"lunabox/internal/service/gamehelper"
	"lunabox/internal/utils/archiveutils"
	"lunabox/internal/utils/imageutils"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	return p.ImportSelected(zipPath, skipNoPath, samePathAction, nil)
}

func (p *PotatoVNImporter) ImportSelected(zipPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

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
	if err := p.applyGalgameSourcePathsFromFile(galgames, filepath.Join(tempDir, "data.galgameSources.json")); err != nil {
		return result, err
	}

	existingGames, existingNames, existingPaths, err := p.deps.existingGames("ImportFromPotatoVN")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(galgames))
	for _, galgame := range galgames {
		gameName := galgame.GetDisplayName()
		importPath := potatoVNImportPath(galgame)
		identitySource, sourceID := pickPotatoVNIdentity(galgame)
		sourceType := string(identitySource)
		if !selectionFilter.includes(gameName, importPath, sourceType, sourceID) {
			continue
		}
		hasPath := importPath != ""

		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		action := ImportActionCreate
		existingGameID := ""
		if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, gameName, importPath); exists {
			if conflict.Type != ConflictTypeSamePath || !IsSamePathMergeAction(samePathAction) {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			if samePathAction == SamePathActionMergeSessions {
				action = ImportActionMergeSessions
			}
			existingGameID = conflict.Game.ID
		}
		game, sessions := p.convertToGameWithCover(galgame, tempDir, existingGameID, action != ImportActionMergeSessions)
		if TargetsExistingGame(action) {
			game.Path = importPath
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
			Path:           importPath,
			Action:         action,
			ExistingGameID: existingGameID,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, gameName, importPath)
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
		importPath := potatoVNImportPath(galgame)
		identitySource, sourceID := pickPotatoVNIdentity(galgame)
		sourceType := string(identitySource)
		conflict := previewConflict(existingIndex, name, importPath, sourceType, sourceID)
		previews = append(previews, PreviewGame{
			Name:         name,
			Developer:    galgame.Developer.Value,
			SourceType:   sourceType,
			SourceID:     sourceID,
			Path:         importPath,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      galgame.AddTime.ToTime(),
			HasPath:      importPath != "",
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

	galgamesFile := findPotatoVNZipFile(zipReader.File, "data.galgames.json")
	if galgamesFile == nil {
		applog.LogWarningf(p.deps.Ctx, "PreviewImport: data.galgames.json not found in ZIP: %s", zipPath)
		return nil, fmt.Errorf("无法读取 data.galgames.json: 文件不存在")
	}

	data, err := readPotatoVNZipFile(galgamesFile)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to read data.galgames.json: %v", err)
		return nil, fmt.Errorf("读取 data.galgames.json 失败: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(data, &galgames); err != nil {
		applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to unmarshal data.galgames.json: %v", err)
		return nil, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	sourcesFile := findPotatoVNZipFile(zipReader.File, "data.galgameSources.json")
	if sourcesFile == nil {
		return galgames, nil
	}
	sourcesData, err := readPotatoVNZipFile(sourcesFile)
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to read data.galgameSources.json: %v", err)
		return nil, fmt.Errorf("读取 data.galgameSources.json 失败: %w", err)
	}
	if err := applyPotatoVNGalgameSourcePaths(galgames, sourcesData); err != nil {
		applog.LogErrorf(p.deps.Ctx, "PreviewImport: failed to unmarshal data.galgameSources.json: %v", err)
		return nil, err
	}
	return galgames, nil
}

func (p *PotatoVNImporter) applyGalgameSourcePathsFromFile(galgames []potatovn.Galgame, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to read data.galgameSources.json: %v", err)
		return fmt.Errorf("无法读取 data.galgameSources.json: %w", err)
	}
	if err := applyPotatoVNGalgameSourcePaths(galgames, data); err != nil {
		applog.LogErrorf(p.deps.Ctx, "ImportFromPotatoVN: failed to unmarshal data.galgameSources.json: %v", err)
		return err
	}
	return nil
}

func applyPotatoVNGalgameSourcePaths(galgames []potatovn.Galgame, data []byte) error {
	var sources []potatovn.GalgameSource
	if err := json.Unmarshal(data, &sources); err != nil {
		return fmt.Errorf("解析 data.galgameSources.json 失败: %w", err)
	}

	sourcePaths := make(map[string]string)
	for _, source := range sources {
		for _, entry := range source.Galgames {
			gameUUID := strings.ToLower(strings.TrimSpace(entry.Galgame))
			gameDirectory := strings.TrimSpace(entry.Path)
			if gameUUID == "" || gameDirectory == "" {
				continue
			}
			if _, exists := sourcePaths[gameUUID]; !exists {
				sourcePaths[gameUUID] = gameDirectory
			}
		}
	}

	for i := range galgames {
		if strings.TrimSpace(galgames[i].Path) != "" {
			continue
		}
		galgames[i].Path = sourcePaths[strings.ToLower(strings.TrimSpace(galgames[i].Uuid))]
	}
	return nil
}

func findPotatoVNZipFile(files []*zip.File, name string) *zip.File {
	for _, file := range files {
		if filepath.Base(filepath.ToSlash(file.Name)) == name {
			return file
		}
	}
	return nil
}

func readPotatoVNZipFile(file *zip.File) ([]byte, error) {
	srcFile, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()
	return io.ReadAll(srcFile)
}

func (p *PotatoVNImporter) convertToGame(galgame potatovn.Galgame, tempDir string, gameID string) (models.Game, []models.PlaySession) {
	return p.convertToGameWithCover(galgame, tempDir, gameID, true)
}

func (p *PotatoVNImporter) convertToGameWithCover(galgame potatovn.Galgame, tempDir string, gameID string, importCover bool) (models.Game, []models.PlaySession) {
	if gameID == "" {
		gameID = uuid.New().String()
	}
	sourceType, sourceID := pickPotatoVNIdentity(galgame)
	game := models.Game{
		ID:                gameID,
		Name:              galgame.GetDisplayName(),
		Company:           galgame.Developer.Value,
		Summary:           galgame.Description.Value,
		Rating:            galgame.Rating.Value,
		ReleaseDate:       formatPotatoVNDate(galgame.ReleaseDate.Value),
		Path:              potatoVNImportPath(galgame),
		GameDirectory:     strings.TrimSpace(galgame.Path),
		SavePath:          galgame.GetSavePath(),
		ProcessName:       galgame.GetProcessName(),
		SourceType:        sourceType,
		MetadataSources:   collectPotatoVNMetadataSources(galgame),
		SourceID:          sourceID,
		CreatedAt:         galgame.AddTime.ToTime(),
		CachedAt:          time.Now(),
		UseLocaleEmulator: galgame.RunInLocaleEmulator,
		UseMagpie:         galgame.EnableMagpie,
	}
	if game.GameDirectory == "" {
		game.GameDirectory = gamehelper.DefaultGameDirectory(game.Path)
	}

	if importCover && galgame.ImagePath.Value != "" && galgame.ImagePath.Value != potatovn.DefaultImagePath {
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

func collectPotatoVNMetadataSources(galgame potatovn.Galgame) []models.GameMetadataSource {
	bySource := make(map[enums.SourceType]string)
	for _, rssType := range potatoVNIdentityPriority {
		if sourceID := galgame.IDForRssType(rssType); sourceID != "" {
			bySource[mapPotatoVNRssTypeToSourceType(rssType)] = sourceID
		}
	}
	for _, entry := range potatoVNMixedKeyPriority {
		if sourceID := galgame.MixedIDs()[entry.key]; sourceID != "" {
			if _, exists := bySource[entry.source]; !exists {
				bySource[entry.source] = sourceID
			}
		}
	}
	items := make([]models.GameMetadataSource, 0, len(bySource))
	for sourceType, sourceID := range bySource {
		if sourceType == enums.Local || strings.TrimSpace(sourceID) == "" {
			continue
		}
		items = append(items, models.GameMetadataSource{SourceType: sourceType, SourceID: strings.TrimSpace(sourceID)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SourceType < items[j].SourceType })
	return items
}

func potatoVNImportPath(galgame potatovn.Galgame) string {
	if exePath := galgame.GetExePath(); exePath != "" {
		return exePath
	}
	return strings.TrimSpace(galgame.Path)
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

// potatoVNIdentityPriority Mixed 源挑选单源身份的优先级，与 reinaIdentityPriority 保持一致
var potatoVNIdentityPriority = []potatovn.RssType{
	potatovn.RssTypeBangumi,
	potatovn.RssTypeVndb,
	potatovn.RssTypeYmgal,
	potatovn.RssTypeSteam,
}

var potatoVNMixedKeyPriority = []struct {
	key    string
	source enums.SourceType
}{
	{"bgm", enums.Bangumi},
	{"vndb", enums.VNDB},
	{"ymgal", enums.Ymgal},
	{"steam", enums.Steam},
}

// pickPotatoVNIdentity 为游戏挑选单一数据源身份。
// PotatoVN 的 Mixed 源（RssType=2）没有单一 ID，其槽位存的是 "bgm:X,vndb:Y,..." 复合串，
// 需按优先级从各单源槽位挑选；旧版导出可能只填复合串，此时解析复合串兜底。
func pickPotatoVNIdentity(galgame potatovn.Galgame) (enums.SourceType, string) {
	if sourceType := mapPotatoVNRssTypeToSourceType(galgame.RssType); sourceType != enums.Local {
		if id := galgame.IDForRssType(galgame.RssType); id != "" {
			return sourceType, id
		}
	}
	for _, rssType := range potatoVNIdentityPriority {
		if id := galgame.IDForRssType(rssType); id != "" {
			return mapPotatoVNRssTypeToSourceType(rssType), id
		}
	}
	mixedIDs := galgame.MixedIDs()
	for _, entry := range potatoVNMixedKeyPriority {
		if id := mixedIDs[entry.key]; id != "" {
			return entry.source, id
		}
	}
	return enums.Local, ""
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
