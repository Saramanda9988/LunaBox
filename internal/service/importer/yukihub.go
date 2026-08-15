package importer

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const yukiHubMaxJSONSize = 64 << 20

var (
	yukiHubRatingPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)
	yukiHubTagSeparator  = regexp.MustCompile(`\s{2,}|[,，;；\r\n]+`)
)

type YukiHubImporter struct {
	deps Dependencies
}

type yukiHubBackup struct {
	App           string                 `json:"app"`
	Schema        int                    `json:"schema"`
	CreatedAt     int64                  `json:"created_at"`
	Settings      yukiHubBackupSettings  `json:"settings"`
	Games         []yukiHubGame          `json:"games"`
	PlaySessions  []yukiHubPlaySession   `json:"play_sessions"`
	MetadataCache []yukiHubMetadataCache `json:"metadata_cache"`
}

type yukiHubBackupSettings struct {
	MetadataSource string `json:"metadata_source"`
}

type yukiHubGame struct {
	LocalID       int64  `json:"local_id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Description   string `json:"description"`
	Tags          string `json:"tags"`
	PlayStatus    string `json:"play_status"`
	TotalPlayTime int64  `json:"total_play_time"`
	LastPlayedAt  int64  `json:"last_played_at"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type yukiHubPlaySession struct {
	SessionUUID string `json:"session_uuid"`
	GameLocalID int64  `json:"game_local_id"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
	Duration    int64  `json:"duration"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type yukiHubMetadataCache struct {
	GameLocalID int64  `json:"game_local_id"`
	Source      string `json:"source"`
	SourceID    string `json:"source_id"`
	JSON        string `json:"json"`
	UpdatedAt   int64  `json:"updated_at"`
}

type yukiHubMetadata struct {
	ID                    string   `json:"id"`
	ChineseTitle          string   `json:"chineseTitle"`
	OriginalTitle         string   `json:"originalTitle"`
	RomanTitle            string   `json:"romanTitle"`
	CoverURL              string   `json:"coverUrl"`
	Description           string   `json:"description"`
	TranslatedDescription string   `json:"translatedDescription"`
	Released              string   `json:"released"`
	Developer             string   `json:"developer"`
	TagsText              string   `json:"tagsText"`
	RatingText            string   `json:"ratingText"`
	CoverSexual           int      `json:"coverSexual"`
	ScreenshotURLs        []string `json:"screenshotUrls"`
}

type parsedYukiHubMetadata struct {
	cache    yukiHubMetadataCache
	data     yukiHubMetadata
	source   enums.SourceType
	sourceID string
}

func NewYukiHubImporter(deps Dependencies) *YukiHubImporter {
	return &YukiHubImporter{deps: deps}
}

func (y *YukiHubImporter) Preview(backupPath string) ([]PreviewGame, error) {
	backup, err := loadYukiHubBackup(backupPath)
	if err != nil {
		applog.LogErrorf(y.deps.Ctx, "PreviewYukiHubImport: failed to load backup: %v", err)
		return nil, err
	}

	existingGames, _, _, err := y.deps.existingGames("PreviewYukiHubImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)
	existingByName := make(map[string]models.Game, len(existingGames))
	for _, game := range existingGames {
		if key := strings.ToLower(strings.TrimSpace(game.Name)); key != "" {
			existingByName[key] = game
		}
	}

	metadataByGame := indexYukiHubMetadata(backup.MetadataCache)
	previews := make([]PreviewGame, 0, len(backup.Games))
	for _, sourceGame := range backup.Games {
		name := strings.TrimSpace(sourceGame.Title)
		if name == "" {
			continue
		}
		primary := pickYukiHubMetadata(metadataByGame[sourceGame.LocalID], backup.Settings.MetadataSource)
		sourceType, sourceID := yukiHubIdentity(primary)
		conflict := previewConflict(existingIndex, name, "", string(sourceType), sourceID)
		if conflict.Type == ConflictTypeNone {
			if existing, ok := existingByName[strings.ToLower(name)]; ok {
				conflict = existingGameConflict{Type: ConflictTypeNameAndPath, Game: existing}
			}
		}

		previews = append(previews, PreviewGame{
			Name:         name,
			Developer:    strings.TrimSpace(primary.data.Developer),
			SourceType:   string(sourceType),
			SourceID:     sourceID,
			Exists:       conflict.Type != ConflictTypeNone,
			ConflictType: conflict.Type,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      yukiHubTimeOrNow(sourceGame.CreatedAt),
			HasPath:      false,
		})
	}

	return previews, nil
}

func (y *YukiHubImporter) Import(backupPath string, skipNoPath bool, samePathAction string) (ImportResult, error) {
	return y.ImportSelected(backupPath, skipNoPath, samePathAction, nil)
}

func (y *YukiHubImporter) ImportSelected(backupPath string, skipNoPath bool, _ string, selections []vo.ImportSelection) (ImportResult, error) {
	result := newImportResult()
	selectionFilter := newImportSelectionFilter(selections)

	backup, err := loadYukiHubBackup(backupPath)
	if err != nil {
		applog.LogErrorf(y.deps.Ctx, "ImportFromYukiHub: failed to load backup: %v", err)
		return result, err
	}

	existingGames, existingNames, existingPaths, err := y.deps.existingGames("ImportFromYukiHub")
	if err != nil {
		return result, err
	}

	metadataByGame := indexYukiHubMetadata(backup.MetadataCache)
	sessionsByGame := indexYukiHubSessions(backup.PlaySessions)
	items := make([]ImportItem, 0, len(backup.Games))
	for _, sourceGame := range backup.Games {
		gameName := strings.TrimSpace(sourceGame.Title)
		if gameName == "" {
			continue
		}

		metadataItems := metadataByGame[sourceGame.LocalID]
		primary := pickYukiHubMetadata(metadataItems, backup.Settings.MetadataSource)
		sourceType, sourceID := yukiHubIdentity(primary)
		if !selectionFilter.includes(gameName, "", string(sourceType), sourceID) {
			continue
		}
		if skipNoPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}
		if skipExistingGame(y.deps.Ctx, "ImportFromYukiHub", &result, existingGames, existingNames, existingPaths, gameName, "") {
			continue
		}

		game, sessions, tags := convertYukiHubGame(sourceGame, metadataItems, primary, sessionsByGame[sourceGame.LocalID], backup.CreatedAt)
		items = append(items, ImportItem{
			Source: vo.GameMetadataFromWebVO{
				Source: game.SourceType,
				Game:   game,
				Tags:   tagsFromNames(tags),
			},
			Sessions:    sessions,
			DisplayName: gameName,
			Action:      ImportActionCreate,
		})
		updateExistingIndexes(existingNames, existingPaths, game, gameName, "")
		existingGames = append(existingGames, game)
	}

	batchResult, err := addImportedItems(y.deps, items)
	if err != nil {
		applog.LogErrorf(y.deps.Ctx, "ImportFromYukiHub: failed to batch add games: %v", err)
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

func loadYukiHubBackup(backupPath string) (*yukiHubBackup, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取 YukiHub 备份文件: %w", err)
	}
	defer file.Close()

	buffered := bufio.NewReader(file)
	header, err := buffered.Peek(2)
	if err != nil {
		return nil, fmt.Errorf("YukiHub 备份文件内容为空或已损坏: %w", err)
	}

	var reader io.Reader = buffered
	if header[0] == 0x1f && header[1] == 0x8b {
		gzipReader, gzipErr := gzip.NewReader(buffered)
		if gzipErr != nil {
			return nil, fmt.Errorf("解压 YukiHub 备份文件失败: %w", gzipErr)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	data, err := io.ReadAll(io.LimitReader(reader, yukiHubMaxJSONSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取 YukiHub 备份内容失败: %w", err)
	}
	if len(data) > yukiHubMaxJSONSize {
		return nil, fmt.Errorf("YukiHub 备份解压后超过 %d MiB", yukiHubMaxJSONSize>>20)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})

	var backup yukiHubBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("解析 YukiHub 备份文件失败: %w", err)
	}
	if backup.App != "YukiHub" {
		return nil, fmt.Errorf("所选文件不是有效的 YukiHub 备份")
	}
	return &backup, nil
}

func indexYukiHubMetadata(entries []yukiHubMetadataCache) map[int64][]parsedYukiHubMetadata {
	result := make(map[int64][]parsedYukiHubMetadata)
	for _, entry := range entries {
		parsed := parsedYukiHubMetadata{
			cache:    entry,
			source:   mapYukiHubSourceType(entry.Source),
			sourceID: strings.TrimSpace(entry.SourceID),
		}
		_ = json.Unmarshal([]byte(entry.JSON), &parsed.data)
		if parsed.sourceID == "" {
			parsed.sourceID = strings.TrimSpace(parsed.data.ID)
		}
		result[entry.GameLocalID] = append(result[entry.GameLocalID], parsed)
	}
	return result
}

func indexYukiHubSessions(entries []yukiHubPlaySession) map[int64][]yukiHubPlaySession {
	result := make(map[int64][]yukiHubPlaySession)
	for _, entry := range entries {
		result[entry.GameLocalID] = append(result[entry.GameLocalID], entry)
	}
	return result
}

func pickYukiHubMetadata(items []parsedYukiHubMetadata, preferredSource string) parsedYukiHubMetadata {
	preferred := mapYukiHubSourceType(preferredSource)
	if preferred != enums.Local {
		for _, item := range items {
			if item.source == preferred && item.sourceID != "" {
				return item
			}
		}
	}
	for _, sourceType := range []enums.SourceType{enums.VNDB, enums.Bangumi, enums.Ymgal, enums.Hikarinagi} {
		for _, item := range items {
			if item.source == sourceType && item.sourceID != "" {
				return item
			}
		}
	}
	if len(items) > 0 {
		return items[0]
	}
	return parsedYukiHubMetadata{source: enums.Local}
}

func yukiHubIdentity(metadata parsedYukiHubMetadata) (enums.SourceType, string) {
	if metadata.source == "" || metadata.sourceID == "" {
		return enums.Local, ""
	}
	return metadata.source, metadata.sourceID
}

func mapYukiHubSourceType(source string) enums.SourceType {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case string(enums.VNDB):
		return enums.VNDB
	case string(enums.Bangumi), "bangumi_mirror":
		return enums.Bangumi
	case string(enums.Ymgal):
		return enums.Ymgal
	case string(enums.Hikarinagi):
		return enums.Hikarinagi
	default:
		return enums.Local
	}
}

func convertYukiHubGame(
	source yukiHubGame,
	metadataItems []parsedYukiHubMetadata,
	primary parsedYukiHubMetadata,
	sourceSessions []yukiHubPlaySession,
	backupCreatedAt int64,
) (models.Game, []models.PlaySession, []string) {
	gameID := uuid.New().String()
	createdAt := yukiHubTime(source.CreatedAt)
	if createdAt.IsZero() {
		createdAt = yukiHubTime(backupCreatedAt)
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := yukiHubTime(source.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	sourceType, sourceID := yukiHubIdentity(primary)
	coverURL := strings.TrimSpace(primary.data.CoverURL)
	if !strings.HasPrefix(coverURL, "https://") && !strings.HasPrefix(coverURL, "http://") {
		coverURL = ""
	}
	game := models.Game{
		ID:              gameID,
		Name:            strings.TrimSpace(source.Title),
		Aliases:         yukiHubAliases(source.Title, source.OriginalTitle, primary.data.ChineseTitle, primary.data.OriginalTitle, primary.data.RomanTitle),
		CoverURL:        coverURL,
		CoverSourceURL:  coverURL,
		Company:         strings.TrimSpace(primary.data.Developer),
		Summary:         firstYukiHubString(source.Description, primary.data.TranslatedDescription, primary.data.Description),
		Rating:          parseYukiHubRating(primary.data.RatingText),
		ReleaseDate:     strings.TrimSpace(primary.data.Released),
		Status:          mapYukiHubGameStatus(source.PlayStatus),
		SourceType:      sourceType,
		SourceID:        sourceID,
		MetadataSources: collectYukiHubMetadataSources(metadataItems, updatedAt),
		CachedAt:        updatedAt,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}

	tags := parseYukiHubTags(source.Tags)
	for _, item := range metadataItems {
		tags = append(tags, parseYukiHubTags(item.data.TagsText)...)
	}
	tags = uniqueYukiHubStrings(tags...)
	sessions := convertYukiHubSessions(gameID, source, sourceSessions, createdAt, updatedAt)
	return game, sessions, tags
}

func collectYukiHubMetadataSources(items []parsedYukiHubMetadata, fallbackTime time.Time) []models.GameMetadataSource {
	bySource := make(map[enums.SourceType]models.GameMetadataSource)
	for _, item := range items {
		if item.source == enums.Local || item.sourceID == "" {
			continue
		}
		cachedAt := yukiHubTime(item.cache.UpdatedAt)
		if cachedAt.IsZero() {
			cachedAt = fallbackTime
		}
		bySource[item.source] = models.GameMetadataSource{
			SourceType: item.source,
			SourceID:   item.sourceID,
			CachedAt:   cachedAt,
			UpdatedAt:  cachedAt,
		}
	}
	result := make([]models.GameMetadataSource, 0, len(bySource))
	for _, item := range bySource {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceType < result[j].SourceType })
	return result
}

func convertYukiHubSessions(gameID string, game yukiHubGame, entries []yukiHubPlaySession, createdAt time.Time, updatedAt time.Time) []models.PlaySession {
	sessions := make([]models.PlaySession, 0, len(entries)+1)
	recordedDuration := 0
	var earliestStart time.Time
	for index, entry := range entries {
		startTime := yukiHubTime(entry.StartTime)
		endTime := yukiHubTime(entry.EndTime)
		durationMillis := entry.Duration
		if durationMillis <= 0 && !startTime.IsZero() && !endTime.IsZero() {
			durationMillis = endTime.Sub(startTime).Milliseconds()
		}
		if durationMillis <= 0 {
			continue
		}
		durationSeconds := int(durationMillis / 1000)
		if durationSeconds == 0 {
			durationSeconds = 1
		}
		if startTime.IsZero() && !endTime.IsZero() {
			startTime = endTime.Add(-time.Duration(durationSeconds) * time.Second)
		}
		if startTime.IsZero() {
			continue
		}
		if endTime.IsZero() || !endTime.After(startTime) {
			endTime = startTime.Add(time.Duration(durationSeconds) * time.Second)
		}
		sessionUpdatedAt := yukiHubTime(entry.UpdatedAt)
		if sessionUpdatedAt.IsZero() {
			sessionUpdatedAt = endTime
		}
		sessionID := strings.TrimSpace(entry.SessionUUID)
		if _, err := uuid.Parse(sessionID); err != nil {
			sessionID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("yukihub:%d:%d:%d:%d", game.LocalID, entry.StartTime, entry.EndTime, index))).String()
		}
		sessions = append(sessions, models.PlaySession{
			ID:        sessionID,
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  durationSeconds,
			UpdatedAt: sessionUpdatedAt,
		})
		recordedDuration += durationSeconds
		if earliestStart.IsZero() || startTime.Before(earliestStart) {
			earliestStart = startTime
		}
	}

	totalDuration := int(game.TotalPlayTime / 1000)
	if totalDuration > recordedDuration {
		remainder := totalDuration - recordedDuration
		endTime := earliestStart
		if endTime.IsZero() {
			endTime = yukiHubTime(game.LastPlayedAt)
		}
		if endTime.IsZero() {
			endTime = createdAt
		}
		sessions = append(sessions, models.PlaySession{
			ID:        uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("yukihub:%d:aggregate", game.LocalID))).String(),
			GameID:    gameID,
			StartTime: endTime.Add(-time.Duration(remainder) * time.Second),
			EndTime:   endTime,
			Duration:  remainder,
			UpdatedAt: updatedAt,
		})
	}
	return sessions
}

func mapYukiHubGameStatus(status string) enums.GameStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "playing":
		return enums.StatusPlaying
	case "completed":
		return enums.StatusCompleted
	default:
		return enums.StatusNotStarted
	}
}

func parseYukiHubRating(raw string) float64 {
	match := yukiHubRatingPattern.FindString(raw)
	if match == "" {
		return 0
	}
	rating, err := strconv.ParseFloat(match, 64)
	if err != nil || rating < 0 || rating > 10 {
		return 0
	}
	return rating
}

func parseYukiHubTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return uniqueYukiHubStrings(yukiHubTagSeparator.Split(raw, -1)...)
}

func uniqueYukiHubStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func yukiHubAliases(gameName string, values ...string) []string {
	aliases := uniqueYukiHubStrings(values...)
	result := aliases[:0]
	for _, alias := range aliases {
		if !strings.EqualFold(strings.TrimSpace(gameName), alias) {
			result = append(result, alias)
		}
	}
	return result
}

func firstYukiHubString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func yukiHubTime(milliseconds int64) time.Time {
	if milliseconds <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds)
}

func yukiHubTimeOrNow(milliseconds int64) time.Time {
	value := yukiHubTime(milliseconds)
	if value.IsZero() {
		return time.Now()
	}
	return value
}
