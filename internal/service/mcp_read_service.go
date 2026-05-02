package service

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"

	"lunabox/internal/appconf"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
)

const (
	defaultMCPListGamesLimit    = 20
	maxMCPListGamesLimit        = 50
	defaultMCPPlaySessionsLimit = 20
	maxMCPPlaySessionsLimit     = 100
	defaultMCPMetadataLimit     = 5
	maxMCPMetadataLimit         = 20
)

type MCPReadService struct {
	ctx             context.Context
	db              *sql.DB
	config          *appconf.AppConfig
	gameService     *GameService
	startService    interface{ StartGameWithTracking(string) (bool, error) }
	sessionService  *SessionService
	progressService *GameProgressService
	tagService      *TagService
	statsProvider   AIStatsProvider
	metadataFetcher func(name string) ([]vo.GameMetadataFromWebVO, error)
}

func NewMCPReadService() *MCPReadService {
	return &MCPReadService{}
}

func (s *MCPReadService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *MCPReadService) SetGameService(gameService *GameService) {
	s.gameService = gameService
	if gameService != nil {
		s.metadataFetcher = gameService.FetchMetadataByName
	}
}

func (s *MCPReadService) SetStartService(startService interface{ StartGameWithTracking(string) (bool, error) }) {
	s.startService = startService
}

func (s *MCPReadService) SetSessionService(sessionService *SessionService) {
	s.sessionService = sessionService
}

func (s *MCPReadService) SetGameProgressService(progressService *GameProgressService) {
	s.progressService = progressService
}

func (s *MCPReadService) SetTagService(tagService *TagService) {
	s.tagService = tagService
}

func (s *MCPReadService) SetStatsProvider(provider AIStatsProvider) {
	s.statsProvider = provider
}

func (s *MCPReadService) SetMetadataFetcher(fetcher func(name string) ([]vo.GameMetadataFromWebVO, error)) {
	s.metadataFetcher = fetcher
}

func (s *MCPReadService) ListGames(limit, offset int) (vo.MCPListGamesResponse, error) {
	limit = clampMCPListLimit(limit)
	offset = clampMCPOffset(offset)

	resp := vo.MCPListGamesResponse{
		Limit:  limit,
		Offset: offset,
		Games:  make([]vo.MCPGameCatalogEntry, 0),
	}

	if s.db == nil {
		return resp, fmt.Errorf("MCP read service database is not initialized")
	}

	if err := s.db.QueryRowContext(s.context(), `SELECT COALESCE(COUNT(*), 0) FROM games`).Scan(&resp.Total); err != nil {
		return resp, fmt.Errorf("query game total: %w", err)
	}

	rows, err := s.db.QueryContext(s.context(), `
		SELECT
			g.id,
			COALESCE(g.name, ''),
			COALESCE(g.company, ''),
			COALESCE(g.status, 'not_started'),
			COALESCE(g.source_type, ''),
			COALESCE(g.rating, 0),
			COALESCE(g.release_date, ''),
			latest.last_played_at
		FROM games g
		LEFT JOIN (
			SELECT game_id, MAX(start_time) AS last_played_at
			FROM play_sessions
			GROUP BY game_id
		) latest ON latest.game_id = g.id
		ORDER BY g.created_at DESC, g.id ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return resp, fmt.Errorf("query game catalog: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry vo.MCPGameCatalogEntry
		var lastPlayedAt sql.NullTime
		if err := rows.Scan(
			&entry.GameID,
			&entry.Name,
			&entry.Company,
			&entry.Status,
			&entry.SourceType,
			&entry.Rating,
			&entry.ReleaseDate,
			&lastPlayedAt,
		); err != nil {
			return resp, fmt.Errorf("scan game catalog row: %w", err)
		}
		if lastPlayedAt.Valid {
			lastPlayed := lastPlayedAt.Time
			entry.LastPlayedAt = &lastPlayed
		}
		resp.Games = append(resp.Games, entry)
	}
	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("iterate game catalog rows: %w", err)
	}

	resp.HasMore = offset+len(resp.Games) < resp.Total
	return resp, nil
}

func (s *MCPReadService) GetGame(gameID string) (vo.MCPGetGameResponse, error) {
	resp := vo.MCPGetGameResponse{
		SpoilerContext: BuildSpoilerContext(s.config),
	}

	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return resp, fmt.Errorf("game_id is required")
	}
	if s.gameService == nil {
		return resp, fmt.Errorf("game service is not initialized")
	}

	game, err := s.gameService.GetGameByID(gameID)
	if err != nil {
		return resp, err
	}

	detail := vo.MCPGameDetail{
		GameID:       game.ID,
		Name:         game.Name,
		CoverURL:     game.CoverURL,
		Company:      game.Company,
		Summary:      game.Summary,
		Rating:       game.Rating,
		ReleaseDate:  game.ReleaseDate,
		Status:       string(game.Status),
		SourceType:   string(game.SourceType),
		SourceID:     game.SourceID,
		LastPlayedAt: game.LastPlayedAt,
	}

	categories, err := s.getCategoryNamesByGame(gameID)
	if err != nil {
		return resp, err
	}
	detail.Categories = categories

	if s.tagService != nil {
		tags, err := s.tagService.GetTagsByGame(gameID)
		if err != nil {
			return resp, fmt.Errorf("query game tags: %w", err)
		}
		detail.Tags = mapMCPGameTags(tags)
	}

	if s.progressService != nil {
		progress, err := s.progressService.GetGameProgress(gameID)
		if err != nil {
			return resp, fmt.Errorf("query latest game progress: %w", err)
		}
		if progress != nil {
			detail.LatestProgress = &vo.MCPGameProgressSnapshot{
				Chapter:         progress.Chapter,
				Route:           progress.Route,
				ProgressNote:    progress.ProgressNote,
				SpoilerBoundary: NormalizeSpoilerLevel(progress.SpoilerBoundary),
				UpdatedAt:       progress.UpdatedAt,
			}
		}
	}

	resp.Game = detail
	return resp, nil
}

func (s *MCPReadService) StartGame(gameID string) (vo.MCPStartGameResponse, error) {
	resp := vo.MCPStartGameResponse{
		GameID: strings.TrimSpace(gameID),
	}

	if resp.GameID == "" {
		return resp, fmt.Errorf("game_id is required")
	}
	if s.startService == nil {
		return resp, fmt.Errorf("start service is not initialized")
	}
	if s.gameService == nil {
		return resp, fmt.Errorf("game service is not initialized")
	}

	game, err := s.gameService.GetGameByID(resp.GameID)
	if err != nil {
		return resp, err
	}
	resp.Name = game.Name

	started, err := s.startService.StartGameWithTracking(resp.GameID)
	if err != nil {
		return resp, fmt.Errorf("start game: %w", err)
	}

	resp.Started = started
	if started {
		resp.Message = "game launch requested"
	} else {
		resp.Message = "game launch cancelled"
	}
	return resp, nil
}

func (s *MCPReadService) GetPlaySessions(gameID string, limit, offset int) (vo.MCPPlaySessionsResponse, error) {
	limit = clampMCPPlaySessionsLimit(limit)
	offset = clampMCPOffset(offset)

	resp := vo.MCPPlaySessionsResponse{
		GameID:   strings.TrimSpace(gameID),
		Limit:    limit,
		Offset:   offset,
		Sessions: make([]vo.MCPPlaySession, 0),
	}

	if resp.GameID == "" {
		return resp, fmt.Errorf("game_id is required")
	}
	if s.db == nil {
		return resp, fmt.Errorf("MCP read service database is not initialized")
	}

	if err := s.ensureGameExists(resp.GameID); err != nil {
		return resp, err
	}

	if err := s.db.QueryRowContext(s.context(), `SELECT COALESCE(COUNT(*), 0) FROM play_sessions WHERE game_id = ?`, resp.GameID).Scan(&resp.Total); err != nil {
		return resp, fmt.Errorf("query play session total: %w", err)
	}

	rows, err := s.db.QueryContext(s.context(), `
		SELECT id, game_id, start_time, COALESCE(end_time, start_time), COALESCE(duration, 0), COALESCE(updated_at, end_time, start_time)
		FROM play_sessions
		WHERE game_id = ?
		ORDER BY start_time DESC, updated_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, resp.GameID, limit, offset)
	if err != nil {
		return resp, fmt.Errorf("query play sessions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var session vo.MCPPlaySession
		if err := rows.Scan(&session.ID, &session.GameID, &session.StartTime, &session.EndTime, &session.Duration, &session.UpdatedAt); err != nil {
			return resp, fmt.Errorf("scan play session row: %w", err)
		}
		resp.Sessions = append(resp.Sessions, session)
	}
	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("iterate play sessions: %w", err)
	}

	resp.HasMore = offset+len(resp.Sessions) < resp.Total
	return resp, nil
}

func (s *MCPReadService) SearchMetadataByName(name string, limit int) (vo.MCPMetadataSearchResponse, error) {
	resp := vo.MCPMetadataSearchResponse{
		Query:          strings.TrimSpace(name),
		Limit:          clampMCPMetadataLimit(limit),
		Results:        make([]vo.MCPMetadataCandidate, 0),
		SpoilerContext: BuildSpoilerContext(s.config),
	}

	if resp.Query == "" {
		return resp, fmt.Errorf("name is required")
	}

	fetcher := s.metadataFetcher
	if fetcher == nil && s.gameService != nil {
		fetcher = s.gameService.FetchMetadataByName
	}
	if fetcher == nil {
		return resp, fmt.Errorf("metadata fetcher is not initialized")
	}

	results, err := fetcher(resp.Query)
	if err != nil {
		return resp, fmt.Errorf("search metadata by name: %w", err)
	}

	enabled := s.enabledMetadataSources()
	dedup := make(map[string]struct{}, len(results))
	candidates := make([]vo.MCPMetadataCandidate, 0, len(results))
	for _, item := range results {
		source := strings.ToLower(string(item.Source))
		if _, ok := enabled[source]; !ok {
			continue
		}

		candidate := vo.MCPMetadataCandidate{
			Source:      source,
			SourceID:    item.Game.SourceID,
			Name:        item.Game.Name,
			CoverURL:    item.Game.CoverURL,
			Company:     item.Game.Company,
			Summary:     item.Game.Summary,
			Rating:      item.Game.Rating,
			ReleaseDate: item.Game.ReleaseDate,
			Tags:        mapMCPMetadataTags(item.Tags),
		}

		key := candidate.Source + "\x00" + candidate.SourceID
		if _, exists := dedup[key]; exists {
			continue
		}
		dedup[key] = struct{}{}
		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Rating != candidates[j].Rating {
			return candidates[i].Rating > candidates[j].Rating
		}
		if candidates[i].Name != candidates[j].Name {
			return candidates[i].Name < candidates[j].Name
		}
		if candidates[i].Source != candidates[j].Source {
			return candidates[i].Source < candidates[j].Source
		}
		return candidates[i].SourceID < candidates[j].SourceID
	})

	resp.TotalResults = len(candidates)
	if len(candidates) > resp.Limit {
		resp.Results = candidates[:resp.Limit]
	} else {
		resp.Results = candidates
	}

	return resp, nil
}

func (s *MCPReadService) GetGameStatistic(period enums2.Period) (vo.MCPGameStatisticResponse, error) {
	if period == "" {
		period = enums2.Week
	}
	if period != enums2.Week && period != enums2.Month {
		return vo.MCPGameStatisticResponse{}, fmt.Errorf("unsupported period: %s", period)
	}

	provider := s.statsProvider
	if provider == nil {
		builder := NewAIStatsBuilder()
		builder.Init(s.context(), s.db, s.config)
		provider = builder
	}

	data, err := provider.Build(period)
	if err != nil {
		return vo.MCPGameStatisticResponse{}, err
	}

	resp := vo.MCPGameStatisticResponse{
		Period:            data.Dimension,
		StartDate:         data.StartDate,
		EndDate:           data.EndDate,
		DateRange:         data.DateRange,
		TotalPlayCount:    data.TotalPlayCount,
		TotalPlayDuration: data.TotalPlayDuration,
		TopGames:          make([]vo.MCPGameStatisticTopGame, 0, len(data.TopGames)),
		RecentSessions:    make([]vo.MCPGameStatisticSession, 0, len(data.RecentSessions)),
		SpoilerContext:    BuildSpoilerContext(s.config),
	}

	for _, game := range data.TopGames {
		resp.TopGames = append(resp.TopGames, vo.MCPGameStatisticTopGame{
			GameID:          game.GameID,
			Name:            game.Name,
			Company:         game.Company,
			Duration:        game.Duration,
			Summary:         game.Summary,
			Categories:      slices.Clone(game.Categories),
			Status:          game.Status,
			SpoilerBoundary: NormalizeSpoilerLevel(game.SpoilerBoundary),
			ProgressNote:    game.ProgressNote,
			Route:           game.Route,
		})
	}

	for _, session := range data.RecentSessions {
		resp.RecentSessions = append(resp.RecentSessions, vo.MCPGameStatisticSession{
			GameID:    session.GameID,
			GameName:  session.GameName,
			StartTime: session.StartTime,
			Duration:  session.Duration,
			DayOfWeek: session.DayOfWeek,
			Hour:      session.Hour,
		})
	}

	return resp, nil
}

func (s *MCPReadService) ensureGameExists(gameID string) error {
	if s.gameService != nil {
		_, err := s.gameService.GetGameByID(gameID)
		return err
	}

	if s.db == nil {
		return fmt.Errorf("MCP read service database is not initialized")
	}

	var exists bool
	if err := s.db.QueryRowContext(s.context(), `SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)`, gameID).Scan(&exists); err != nil {
		return fmt.Errorf("check game existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("game not found with id: %s", gameID)
	}
	return nil
}

func (s *MCPReadService) getCategoryNamesByGame(gameID string) ([]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("MCP read service database is not initialized")
	}

	rows, err := s.db.QueryContext(s.context(), `
		SELECT COALESCE(c.name, '')
		FROM categories c
		INNER JOIN game_categories gc ON c.id = gc.category_id
		WHERE gc.game_id = ?
		ORDER BY c.created_at, c.id
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("query game categories: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan game category row: %w", err)
		}
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game categories: %w", err)
	}
	return names, nil
}

func (s *MCPReadService) enabledMetadataSources() map[string]struct{} {
	defaultSources := []string{
		string(enums2.Bangumi),
		string(enums2.VNDB),
		string(enums2.Ymgal),
		string(enums2.Steam),
	}

	result := make(map[string]struct{}, len(defaultSources))
	sources := defaultSources
	if s.config != nil && len(s.config.MetadataSources) > 0 {
		sources = s.config.MetadataSources
	}

	for _, source := range sources {
		normalized := strings.ToLower(strings.TrimSpace(source))
		switch normalized {
		case string(enums2.Bangumi), string(enums2.VNDB), string(enums2.Ymgal), string(enums2.Steam):
			result[normalized] = struct{}{}
		}
	}

	if len(result) == 0 {
		for _, source := range defaultSources {
			result[source] = struct{}{}
		}
	}

	return result
}

func (s *MCPReadService) context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func clampMCPListLimit(limit int) int {
	if limit <= 0 {
		return defaultMCPListGamesLimit
	}
	if limit > maxMCPListGamesLimit {
		return maxMCPListGamesLimit
	}
	return limit
}

func clampMCPPlaySessionsLimit(limit int) int {
	if limit <= 0 {
		return defaultMCPPlaySessionsLimit
	}
	if limit > maxMCPPlaySessionsLimit {
		return maxMCPPlaySessionsLimit
	}
	return limit
}

func clampMCPMetadataLimit(limit int) int {
	if limit <= 0 {
		return defaultMCPMetadataLimit
	}
	if limit > maxMCPMetadataLimit {
		return maxMCPMetadataLimit
	}
	return limit
}

func clampMCPOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func mapMCPGameTags(tags []models.GameTag) []vo.MCPGameTag {
	if len(tags) == 0 {
		return nil
	}

	result := make([]vo.MCPGameTag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, vo.MCPGameTag{
			Name:      tag.Name,
			Source:    tag.Source,
			Weight:    tag.Weight,
			IsSpoiler: tag.IsSpoiler,
		})
	}
	return result
}

func mapMCPMetadataTags(tags []metadata.TagItem) []vo.MCPGameTag {
	if len(tags) == 0 {
		return nil
	}

	result := make([]vo.MCPGameTag, 0, len(tags))
	for _, tag := range tags {
		result = append(result, vo.MCPGameTag{
			Name:      tag.Name,
			Source:    tag.Source,
			Weight:    tag.Weight,
			IsSpoiler: tag.IsSpoiler,
		})
	}
	return result
}
