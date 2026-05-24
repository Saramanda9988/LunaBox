package service

import (
	"context"
	"database/sql"
	"fmt"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"strings"
)

const (
	defaultGameListLimit = 120
	maxGameListLimit     = 240
)

type gameListScope struct {
	joinClause  string
	whereClause string
	args        []interface{}
}

func normalizeGameListRequest(req vo.GameListRequest) vo.GameListRequest {
	if req.Limit <= 0 {
		req.Limit = defaultGameListLimit
	}
	if req.Limit > maxGameListLimit {
		req.Limit = maxGameListLimit
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	req.SearchQuery = strings.TrimSpace(req.SearchQuery)
	req.Status = strings.TrimSpace(req.Status)
	req.SortBy = normalizeGameListSortBy(req.SortBy)
	req.SortOrder = normalizeGameListSortOrder(req.SortOrder)
	req.Tags = utils.UniqueNonEmptyStrings(req.Tags)
	return req
}

func normalizeGameListSortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "name", "last_played_at", "created_at", "rating", "release_date":
		return strings.TrimSpace(sortBy)
	default:
		return "created_at"
	}
}

func normalizeGameListSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}

func gameListOrderClause(sortBy string, sortOrder string) string {
	direction := "DESC"
	if sortOrder == "asc" {
		direction = "ASC"
	}

	switch sortBy {
	case "name":
		return fmt.Sprintf("LOWER(COALESCE(g.name, '')) %s, g.created_at DESC, g.id ASC", direction)
	case "last_played_at":
		if direction == "ASC" {
			return "latest.last_played_at IS NULL ASC, latest.last_played_at ASC, g.created_at DESC, g.id ASC"
		}
		return "latest.last_played_at IS NULL ASC, latest.last_played_at DESC, g.created_at DESC, g.id ASC"
	case "rating":
		return fmt.Sprintf("COALESCE(g.rating, 0) %s, g.created_at DESC, g.id ASC", direction)
	case "release_date":
		return fmt.Sprintf("COALESCE(g.release_date, '') %s, g.created_at DESC, g.id ASC", direction)
	default:
		return fmt.Sprintf("g.created_at %s, g.id ASC", direction)
	}
}

func queryGameList(ctx context.Context, db *sql.DB, req vo.GameListRequest, scope gameListScope) (vo.GameListResponse, error) {
	req = normalizeGameListRequest(req)
	resp := vo.GameListResponse{
		Games:  make([]models.Game, 0),
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	if db == nil {
		return resp, fmt.Errorf("database is not initialized")
	}

	whereParts := make([]string, 0, 4)
	args := make([]interface{}, 0, len(scope.args)+len(req.Tags)+4)
	if strings.TrimSpace(scope.whereClause) != "" {
		whereParts = append(whereParts, scope.whereClause)
		args = append(args, scope.args...)
	}
	if req.SearchQuery != "" {
		whereParts = append(whereParts, "(LOWER(COALESCE(g.name, '')) LIKE ? OR LOWER(COALESCE(g.company, '')) LIKE ?)")
		needle := "%" + strings.ToLower(req.SearchQuery) + "%"
		args = append(args, needle, needle)
	}
	if req.Status != "" {
		whereParts = append(whereParts, "COALESCE(g.status, 'not_started') = ?")
		args = append(args, req.Status)
	}
	if len(req.Tags) > 0 {
		placeholders := utils.BuildPlaceholders(len(req.Tags))
		whereParts = append(whereParts, fmt.Sprintf(`
			g.id IN (
				SELECT game_id
				FROM game_tags
				WHERE name IN (%s)
				GROUP BY game_id
				HAVING COUNT(DISTINCT name) = ?
			)
		`, placeholders))
		for _, tag := range req.Tags {
			args = append(args, tag)
		}
		args = append(args, len(req.Tags))
	}

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = "WHERE " + strings.Join(whereParts, " AND ")
	}
	joinSQL := scope.joinClause

	countQuery := fmt.Sprintf(`
		SELECT COALESCE(COUNT(*), 0)
		FROM games g
		%s
		%s
	`, joinSQL, whereSQL)
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&resp.Total); err != nil {
		return resp, fmt.Errorf("query game list total: %w", err)
	}

	orderClause := gameListOrderClause(req.SortBy, req.SortOrder)
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, req.Limit, req.Offset)
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			g.id,
			COALESCE(g.name, '') AS name,
			COALESCE(g.cover_url, '') AS cover_url,
			COALESCE(g.company, '') AS company,
			COALESCE(g.summary, '') AS summary,
			COALESCE(g.rating, 0) AS rating,
			COALESCE(g.release_date, '') AS release_date,
			COALESCE(g.path, '') AS path,
			COALESCE(g.save_path, '') AS save_path,
			COALESCE(g.process_name, '') AS process_name,
			COALESCE(g.status, 'not_started') AS status,
			COALESCE(g.source_type, '') AS source_type,
			g.cached_at,
			COALESCE(g.source_id, '') AS source_id,
			g.created_at,
			COALESCE(g.updated_at, g.created_at, g.cached_at) AS updated_at,
			latest.last_played_at,
			COALESCE(g.use_locale_emulator, FALSE) AS use_locale_emulator,
			COALESCE(g.use_magpie, FALSE) AS use_magpie
		FROM games g
		%s
		LEFT JOIN (
			SELECT game_id, MAX(start_time) AS last_played_at
			FROM play_sessions
			GROUP BY game_id
		) latest ON latest.game_id = g.id
		%s
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, joinSQL, whereSQL, orderClause), listArgs...)
	if err != nil {
		return resp, fmt.Errorf("query game list page: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		game, scanErr := scanGameListRow(rows)
		if scanErr != nil {
			return resp, scanErr
		}
		resp.Games = append(resp.Games, game)
	}
	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("iterate game list rows: %w", err)
	}

	resp.HasMore = req.Offset+len(resp.Games) < resp.Total
	return resp, nil
}

type gameScanner interface {
	Scan(dest ...interface{}) error
}

func scanGameListRow(scanner gameScanner) (models.Game, error) {
	var game models.Game
	var sourceType string
	var status string
	var lastPlayedAt sql.NullTime
	err := scanner.Scan(
		&game.ID,
		&game.Name,
		&game.CoverURL,
		&game.Company,
		&game.Summary,
		&game.Rating,
		&game.ReleaseDate,
		&game.Path,
		&game.SavePath,
		&game.ProcessName,
		&status,
		&sourceType,
		&game.CachedAt,
		&game.SourceID,
		&game.CreatedAt,
		&game.UpdatedAt,
		&lastPlayedAt,
		&game.UseLocaleEmulator,
		&game.UseMagpie,
	)
	if err != nil {
		return game, fmt.Errorf("scan game list row: %w", err)
	}
	game.SourceType = enums2.SourceType(sourceType)
	game.Status = enums2.GameStatus(status)
	if lastPlayedAt.Valid {
		lastPlayed := lastPlayedAt.Time
		game.LastPlayedAt = &lastPlayed
	}
	return game, nil
}
