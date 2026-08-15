package importer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/models/reinamanager"
	"lunabox/internal/service/gamehelper"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type ReinaManagerImporter struct {
	deps Dependencies
}

var (
	reinaBasicFieldPriority = []string{"bgm", "vndb", "dlsite", "erogamescape", "ymgal", "kun"}
	reinaSummaryPriority    = []string{"ymgal", "bgm", "kun", "vndb", "dlsite", "erogamescape"}
	reinaDeveloperPriority  = []string{"vndb", "erogamescape", "kun", "dlsite", "ymgal", "bgm"}
	reinaCoverPriority      = []string{"bgm", "vndb", "erogamescape", "dlsite", "kun", "ymgal"}
	reinaRatingPriority     = []string{"bgm", "erogamescape", "vndb", "dlsite", "ymgal", "kun"}
	reinaTagPriority        = []string{"bgm", "dlsite", "erogamescape", "vndb", "kun", "ymgal"}
	reinaIdentityPriority   = []string{"bgm", "vndb", "dlsite", "erogamescape", "ymgal"}
)

func NewReinaManagerImporter(deps Dependencies) *ReinaManagerImporter {
	return &ReinaManagerImporter{deps: deps}
}

func (r *ReinaManagerImporter) Preview(dbPath string) ([]PreviewGame, error) {
	data, err := loadReinaManagerData(dbPath)
	if err != nil {
		applog.LogErrorf(r.deps.Ctx, "PreviewReinaManagerImport: failed to load database: %v", err)
		return nil, err
	}

	existingGames, _, _, err := r.deps.existingGames("PreviewReinaManagerImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(data.Games))
	for _, sourceGame := range data.Games {
		game, _ := convertReinaManagerGame(sourceGame)
		if game.Name == "" {
			continue
		}
		conflict := previewConflict(existingIndex, game.Name, game.Path, string(game.SourceType), game.SourceID)
		previews = append(previews, PreviewGame{
			Name:         game.Name,
			Developer:    game.Company,
			SourceType:   string(game.SourceType),
			SourceID:     game.SourceID,
			Path:         game.Path,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      game.CreatedAt,
			HasPath:      game.Path != "",
		})
	}
	return previews, nil
}

func (r *ReinaManagerImporter) Import(dbPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	return r.ImportSelected(dbPath, skipNoPath, samePathAction, nil)
}

func (r *ReinaManagerImporter) ImportSelected(dbPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

	data, err := loadReinaManagerData(dbPath)
	if err != nil {
		applog.LogErrorf(r.deps.Ctx, "ImportFromReinaManager: failed to load database: %v", err)
		return result, err
	}

	existingGames, existingNames, existingPaths, err := r.deps.existingGames("ImportFromReinaManager")
	if err != nil {
		return result, err
	}

	items := make([]ImportItem, 0, len(data.Games))
	for _, sourceGame := range data.Games {
		game, sessions := convertReinaManagerGame(sourceGame)
		if game.Name == "" {
			result.Failed++
			result.FailedNames = append(result.FailedNames, fmt.Sprintf("ReinaManager #%d (缺少名称)", sourceGame.ID))
			continue
		}
		if !selectionFilter.includes(game.Name, game.Path, string(game.SourceType), game.SourceID) {
			continue
		}
		if skipNoPath && game.Path == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, game.Name+" (无路径)")
			continue
		}

		action := ImportActionCreate
		existingGameID := ""
		if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, game.Name, game.Path); exists {
			if conflict.Type != ConflictTypeSamePath || !IsSamePathMergeAction(samePathAction) {
				result.Skipped++
				if conflict.Type == ConflictTypeNameAndPath {
					result.SkippedNames = append(result.SkippedNames, game.Name+" (已存在)")
				} else {
					result.SkippedNames = append(result.SkippedNames, game.Name+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			}
			action = ImportActionUpdateExisting
			if samePathAction == SamePathActionMergeSessions {
				action = ImportActionMergeSessions
			}
			existingGameID = conflict.Game.ID
			game.ID = conflict.Game.ID
			game.Path = conflict.Game.Path
			for i := range sessions {
				sessions[i].GameID = conflict.Game.ID
			}
		}

		items = append(items, ImportItem{
			Source: vo.GameMetadataFromWebVO{
				Source: game.SourceType,
				Game:   game,
				Tags:   tagsFromNames(collectReinaManagerTags(sourceGame)),
			},
			Sessions:       sessions,
			DisplayName:    game.Name,
			Path:           game.Path,
			Action:         action,
			ExistingGameID: existingGameID,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, game.Name, game.Path)
		}
	}

	batchResult, err := addImportedItems(r.deps, items)
	if err != nil {
		applog.LogErrorf(r.deps.Ctx, "ImportFromReinaManager: failed to batch add games: %v", err)
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

func loadReinaManagerData(dbPath string) (reinamanager.Data, error) {
	var result reinamanager.Data
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return result, fmt.Errorf("ReinaManager 数据库路径为空")
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		return result, fmt.Errorf("读取 ReinaManager 数据库失败: %w", err)
	}
	if info.IsDir() {
		return result, fmt.Errorf("ReinaManager 数据库路径不能是目录")
	}

	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return result, fmt.Errorf("解析 ReinaManager 数据库路径失败: %w", err)
	}
	uriPath := filepath.ToSlash(absPath)
	if filepath.VolumeName(absPath) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := url.URL{Scheme: "file", Path: uriPath}
	query := dsn.Query()
	query.Set("mode", "ro")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("sqlite3", dsn.String())
	if err != nil {
		return result, fmt.Errorf("初始化 ReinaManager 数据库读取器失败: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return result, fmt.Errorf("以只读方式打开 ReinaManager SQLite 数据库失败: %w", err)
	}

	games, err := readReinaManagerGames(db)
	if err != nil {
		return result, err
	}
	if err := readReinaManagerSources(db, games); err != nil {
		return result, err
	}
	if err := readReinaManagerSessions(db, games); err != nil {
		return result, err
	}

	result.Games = make([]reinamanager.Game, 0, len(games))
	for _, game := range games {
		result.Games = append(result.Games, *game)
	}
	sort.Slice(result.Games, func(i, j int) bool {
		return result.Games[i].ID < result.Games[j].ID
	})
	return result, nil
}

func readReinaManagerGames(db *sql.DB) (map[int64]*reinamanager.Game, error) {
	rows, err := db.Query(`
		SELECT id, id_type, date, localpath, executable, savepath, clear,
		       le_launch, magpie, custom_data, created_at, updated_at
		FROM games
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("读取 ReinaManager games 表失败: %w", err)
	}
	defer rows.Close()

	games := make(map[int64]*reinamanager.Game)
	for rows.Next() {
		var (
			game                                          reinamanager.Game
			idType, date, localPath, executable, savePath sql.NullString
			clear, leLaunch, magpie, createdAt, updatedAt sql.NullInt64
			customJSON                                    sql.NullString
		)
		if err := rows.Scan(
			&game.ID, &idType, &date, &localPath, &executable, &savePath, &clear,
			&leLaunch, &magpie, &customJSON, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("解析 ReinaManager 游戏记录失败: %w", err)
		}
		game.IDType = idType.String
		game.Date = date.String
		game.LocalPath = localPath.String
		game.Executable = executable.String
		game.SavePath = savePath.String
		game.Clear = clear.Int64
		game.UseLocaleEmulator = leLaunch.Int64 != 0
		game.UseMagpie = magpie.Int64 != 0
		game.CreatedAt = createdAt.Int64
		game.UpdatedAt = updatedAt.Int64
		game.Sources = make(map[string]reinamanager.Source)
		if customJSON.Valid && strings.TrimSpace(customJSON.String) != "" {
			if err := json.Unmarshal([]byte(customJSON.String), &game.Custom); err != nil {
				return nil, fmt.Errorf("解析 ReinaManager 游戏 %d 的 custom_data 失败: %w", game.ID, err)
			}
		}
		games[game.ID] = &game
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 ReinaManager 游戏记录失败: %w", err)
	}
	return games, nil
}

func readReinaManagerSources(db *sql.DB, games map[int64]*reinamanager.Game) error {
	rows, err := db.Query(`
		SELECT game_id, source, external_id, data
		FROM game_sources
		ORDER BY game_id, source
	`)
	if err != nil {
		return fmt.Errorf("读取 ReinaManager game_sources 表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var gameID int64
		var sourceName, externalID, dataJSON sql.NullString
		if err := rows.Scan(&gameID, &sourceName, &externalID, &dataJSON); err != nil {
			return fmt.Errorf("解析 ReinaManager 数据源记录失败: %w", err)
		}
		game, ok := games[gameID]
		if !ok {
			continue
		}
		source := reinamanager.Source{
			Source:     strings.ToLower(strings.TrimSpace(sourceName.String)),
			ExternalID: strings.TrimSpace(externalID.String),
		}
		if source.Source == "" {
			continue
		}
		if dataJSON.Valid && strings.TrimSpace(dataJSON.String) != "" {
			if err := json.Unmarshal([]byte(dataJSON.String), &source.Data); err != nil {
				return fmt.Errorf("解析 ReinaManager 游戏 %d 的 %s 数据源失败: %w", gameID, source.Source, err)
			}
		}
		game.Sources[source.Source] = source
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历 ReinaManager 数据源记录失败: %w", err)
	}
	return nil
}

func readReinaManagerSessions(db *sql.DB, games map[int64]*reinamanager.Game) error {
	rows, err := db.Query(`
		SELECT game_id, start_time, end_time, duration
		FROM game_sessions
		WHERE duration > 0
		ORDER BY game_id, start_time
	`)
	if err != nil {
		return fmt.Errorf("读取 ReinaManager game_sessions 表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var gameID int64
		var session reinamanager.Session
		if err := rows.Scan(&gameID, &session.StartTime, &session.EndTime, &session.Duration); err != nil {
			return fmt.Errorf("解析 ReinaManager 游玩记录失败: %w", err)
		}
		if game, ok := games[gameID]; ok {
			game.Sessions = append(game.Sessions, session)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历 ReinaManager 游玩记录失败: %w", err)
	}
	return nil
}

func convertReinaManagerGame(source reinamanager.Game) (models.Game, []models.PlaySession) {
	now := time.Now()
	gameID := uuid.New().String()
	sourceType, sourceID := pickReinaManagerIdentity(source)
	coverURL := pickReinaManagerCover(source)
	game := models.Game{
		ID:                gameID,
		Name:              pickReinaManagerName(source),
		CoverURL:          coverURL,
		Company:           pickReinaManagerString(source, source.Custom.Developer, reinaDeveloperPriority, func(data reinamanager.Metadata) string { return data.Developer }),
		Summary:           pickReinaManagerString(source, source.Custom.Summary, reinaSummaryPriority, func(data reinamanager.Metadata) string { return data.Summary }),
		Rating:            pickReinaManagerRating(source),
		ReleaseDate:       pickReinaManagerReleaseDate(source),
		Path:              joinReinaManagerLaunchPath(source.LocalPath, source.Executable),
		GameDirectory:     strings.TrimSpace(source.LocalPath),
		SavePath:          strings.TrimSpace(source.SavePath),
		Status:            mapReinaManagerStatus(source.Clear),
		SourceType:        sourceType,
		MetadataSources:   collectReinaManagerMetadataSources(source, now),
		SourceID:          sourceID,
		CreatedAt:         timeFromUnixOr(source.CreatedAt, now),
		CachedAt:          now,
		UpdatedAt:         timeFromUnixOr(source.UpdatedAt, now),
		UseLocaleEmulator: source.UseLocaleEmulator,
		UseMagpie:         source.UseMagpie,
		IsNSFW:            pickReinaManagerNSFW(source),
	}
	if gamehelper.IsDownloadableCoverURL(coverURL) {
		game.CoverSourceURL = coverURL
	}

	sessions := make([]models.PlaySession, 0, len(source.Sessions))
	for _, sourceSession := range source.Sessions {
		if sourceSession.StartTime <= 0 || sourceSession.EndTime <= sourceSession.StartTime || sourceSession.Duration <= 0 {
			continue
		}
		endTime := time.Unix(sourceSession.EndTime, 0)
		sessions = append(sessions, models.PlaySession{
			ID:        uuid.New().String(),
			GameID:    gameID,
			StartTime: time.Unix(sourceSession.StartTime, 0),
			EndTime:   endTime,
			Duration:  int(sourceSession.Duration * 60),
			UpdatedAt: endTime,
		})
	}
	return game, sessions
}

func collectReinaManagerMetadataSources(game reinamanager.Game, timestamp time.Time) []models.GameMetadataSource {
	items := make([]models.GameMetadataSource, 0, len(game.Sources))
	seen := make(map[enums.SourceType]struct{}, len(game.Sources))
	for sourceName, source := range game.Sources {
		sourceType := mapReinaManagerSource(sourceName)
		sourceID := strings.TrimSpace(source.ExternalID)
		if sourceType == enums.Local || sourceID == "" {
			continue
		}
		if _, exists := seen[sourceType]; exists {
			continue
		}
		seen[sourceType] = struct{}{}
		items = append(items, models.GameMetadataSource{
			SourceType: sourceType,
			SourceID:   sourceID,
			CachedAt:   timestamp,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SourceType < items[j].SourceType })
	return items
}

func pickReinaManagerIdentity(game reinamanager.Game) (enums.SourceType, string) {
	idType := strings.ToLower(strings.TrimSpace(game.IDType))
	if idType != "mixed" {
		if source, ok := game.Sources[idType]; ok {
			if mapped := mapReinaManagerSource(idType); mapped != enums.Local && source.ExternalID != "" {
				return mapped, source.ExternalID
			}
		}
	}
	for _, sourceName := range reinaIdentityPriority {
		if source, ok := game.Sources[sourceName]; ok && source.ExternalID != "" {
			return mapReinaManagerSource(sourceName), source.ExternalID
		}
	}
	return enums.Local, ""
}

func mapReinaManagerSource(source string) enums.SourceType {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "bgm", "bangumi":
		return enums.Bangumi
	case "vndb":
		return enums.VNDB
	case "ymgal":
		return enums.Ymgal
	case "dlsite":
		return enums.DLsite
	case "erogamescape":
		return enums.ErogameScape
	default:
		return enums.Local
	}
}

func pickReinaManagerName(game reinamanager.Game) string {
	if name := strings.TrimSpace(game.Custom.Name); name != "" {
		return name
	}
	for _, sourceName := range reinaBasicFieldPriority {
		if source, ok := game.Sources[sourceName]; ok {
			if name := firstNonEmpty(source.Data.NameCN, source.Data.Name); name != "" {
				return name
			}
		}
	}
	return ""
}

func pickReinaManagerCover(game reinamanager.Game) string {
	if image := usableReinaManagerCover(game.Custom.Image); image != "" {
		return image
	}
	if sourceName := strings.ToLower(strings.TrimSpace(game.Custom.CoverSource)); sourceName != "" {
		if source, ok := game.Sources[sourceName]; ok {
			if image := usableReinaManagerCover(source.Data.Image); image != "" {
				return image
			}
		}
	}
	for _, sourceName := range reinaCoverPriority {
		if source, ok := game.Sources[sourceName]; ok {
			if image := usableReinaManagerCover(source.Data.Image); image != "" {
				return image
			}
		}
	}
	return ""
}

func usableReinaManagerCover(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return raw
}

func pickReinaManagerString(game reinamanager.Game, custom string, priority []string, field func(reinamanager.Metadata) string) string {
	if value := strings.TrimSpace(custom); value != "" {
		return value
	}
	for _, sourceName := range priority {
		if source, ok := game.Sources[sourceName]; ok {
			if value := strings.TrimSpace(field(source.Data)); value != "" {
				return value
			}
		}
	}
	return ""
}

func pickReinaManagerRating(game reinamanager.Game) float64 {
	if game.Custom.UserRating != nil {
		return *game.Custom.UserRating
	}
	for _, sourceName := range reinaRatingPriority {
		if source, ok := game.Sources[sourceName]; ok && source.Data.Score != nil {
			return *source.Data.Score
		}
	}
	return 0
}

func pickReinaManagerReleaseDate(game reinamanager.Game) string {
	if releaseDate := strings.TrimSpace(game.Date); releaseDate != "" {
		return releaseDate
	}
	return pickReinaManagerString(game, "", reinaBasicFieldPriority, func(data reinamanager.Metadata) string { return data.ReleaseDate })
}

func pickReinaManagerNSFW(game reinamanager.Game) bool {
	if game.Custom.NSFW != nil {
		return *game.Custom.NSFW
	}
	for _, sourceName := range reinaBasicFieldPriority {
		if source, ok := game.Sources[sourceName]; ok && source.Data.NSFW != nil {
			return *source.Data.NSFW
		}
	}
	return false
}

func collectReinaManagerTags(game reinamanager.Game) []string {
	result := append([]string(nil), game.Custom.Tags...)
	for _, sourceName := range reinaTagPriority {
		if source, ok := game.Sources[sourceName]; ok {
			result = append(result, source.Data.Tags...)
		}
	}
	return result
}

func mapReinaManagerStatus(clear int64) enums.GameStatus {
	switch clear {
	case 2:
		return enums.StatusCompleted
	case 3:
		return enums.StatusPlaying
	case 4, 5:
		return enums.StatusOnHold
	default:
		return enums.StatusWantToPlay
	}
}

func joinReinaManagerLaunchPath(localPath string, executable string) string {
	localPath = strings.TrimSpace(localPath)
	executable = strings.TrimSpace(executable)
	switch {
	case localPath == "":
		return executable
	case executable == "":
		return localPath
	case filepath.IsAbs(executable):
		return executable
	default:
		return filepath.Join(localPath, executable)
	}
}

func timeFromUnixOr(timestamp int64, fallback time.Time) time.Time {
	if timestamp <= 0 {
		return fallback
	}
	return time.Unix(timestamp, 0)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
