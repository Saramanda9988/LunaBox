package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/gamehelper"
	"time"
)

const homeRecentPlayedLimit = 10

type HomeService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewHomeService() *HomeService {
	return &HomeService{}
}

//wails:ignore
func (s *HomeService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *HomeService) GetHomePageData() (vo.HomePageData, error) {
	data := vo.HomePageData{
		RecentPlayed: make([]vo.LastPlayedGame, 0),
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 1. 最近游玩的游戏列表和每个游戏的累计时长
	recentPlayedQuery := `
		WITH session_rollup AS (
			SELECT
				game_id,
				MAX(start_time) AS last_played_at,
				COALESCE(SUM(COALESCE(duration, 0)), 0) AS total_played_dur
			FROM play_sessions
			GROUP BY game_id
		),
		latest_sessions AS (
			SELECT
				game_id,
				start_time,
				COALESCE(duration, 0) AS last_played_dur,
				end_time IS NULL AS is_playing,
				ROW_NUMBER() OVER (
					PARTITION BY game_id
					ORDER BY start_time DESC, id DESC
				) AS row_num
			FROM play_sessions
		)
		SELECT
			g.id, g.name,
			COALESCE(g.aliases, '[]') as aliases,
			COALESCE(g.cover_url, '') as cover_url,
			COALESCE(g.cover_source_url, '') as cover_source_url,
			COALESCE(g.company, '') as company, 
			COALESCE(g.summary, '') as summary, 
			COALESCE(g.rating, 0) as rating,
			COALESCE(g.release_date, '') as release_date,
			COALESCE(g.path, '') as path,
			COALESCE(g.game_directory, '') as game_directory,
			COALESCE(g.save_path, '') as save_path,
			COALESCE(g.process_name, '') as process_name,
			COALESCE(g.wine_runner, '') as wine_runner,
			COALESCE(g.wine_args, '') as wine_args,
			COALESCE(g.wine_prefix, '') as wine_prefix,
			COALESCE(g.launch_mode, 'normal') as launch_mode,
			COALESCE(g.steam_launch_id, '') as steam_launch_id,
			COALESCE(g.steam_launch_kind, '') as steam_launch_kind,
			COALESCE(g.steam_user_id, '') as steam_user_id,
			COALESCE(g.steam_launch_options, '') as steam_launch_options,
			COALESCE(g.status, 'not_started') as status,
			COALESCE(g.source_type, '') as source_type, 
			g.cached_at, 
			COALESCE(g.source_id, '') as source_id, 
			g.created_at,
			COALESCE(g.updated_at, g.created_at, g.cached_at) as updated_at,
			rollup.last_played_at,
			latest.last_played_dur,
			latest.is_playing,
			rollup.total_played_dur,
			COALESCE(g.use_locale_emulator, FALSE) as use_locale_emulator,
			COALESCE(g.use_magpie, FALSE) as use_magpie,
			COALESCE(g.is_nsfw, FALSE) as is_nsfw,
			COALESCE(g.metadata_locked, FALSE) as metadata_locked
		FROM games g
		JOIN session_rollup rollup ON rollup.game_id = g.id
		JOIN latest_sessions latest ON latest.game_id = g.id AND latest.row_num = 1
		ORDER BY rollup.last_played_at DESC, g.created_at DESC, g.id ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(s.ctx, recentPlayedQuery, homeRecentPlayedLimit)
	if err != nil {
		applog.LogErrorf(s.ctx, "查询上次游玩游戏失败: %v", err)
		return data, fmt.Errorf("query recent played games: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var game models.Game
		var status string
		var sourceType string
		var launchMode string
		var aliasesJSON string
		var lastPlayedAt time.Time
		var lastPlayedDur int
		var isPlaying bool
		var totalPlayedDur int

		if err := rows.Scan(
			&game.ID,
			&game.Name,
			&aliasesJSON,
			&game.CoverURL,
			&game.CoverSourceURL,
			&game.Company,
			&game.Summary,
			&game.Rating,
			&game.ReleaseDate,
			&game.Path,
			&game.GameDirectory,
			&game.SavePath,
			&game.ProcessName,
			&game.WineRunner,
			&game.WineArgs,
			&game.WinePrefix,
			&launchMode,
			&game.SteamLaunchID,
			&game.SteamLaunchKind,
			&game.SteamUserID,
			&game.SteamLaunchOptions,
			&status,
			&sourceType,
			&game.CachedAt,
			&game.SourceID,
			&game.CreatedAt,
			&game.UpdatedAt,
			&lastPlayedAt,
			&lastPlayedDur,
			&isPlaying,
			&totalPlayedDur,
			&game.UseLocaleEmulator,
			&game.UseMagpie,
			&game.IsNSFW,
			&game.MetadataLocked,
		); err != nil {
			return data, fmt.Errorf("scan recent played game: %w", err)
		}
		game.Aliases, err = gamehelper.DecodeAliases(aliasesJSON)
		if err != nil {
			return data, fmt.Errorf("decode recent played game aliases: %w", err)
		}

		game.Status = enums2.GameStatus(status)
		game.SourceType = enums2.SourceType(sourceType)
		game.LaunchMode = enums2.NormalizeLaunchMode(enums2.LaunchMode(launchMode))
		game.LastPlayedAt = &lastPlayedAt

		recentPlayed := vo.LastPlayedGame{
			Game:           game,
			LastPlayedAt:   lastPlayedAt,
			LastPlayedDur:  lastPlayedDur,
			TotalPlayedDur: totalPlayedDur,
			IsPlaying:      isPlaying,
		}
		data.RecentPlayed = append(data.RecentPlayed, recentPlayed)
	}
	if err := rows.Err(); err != nil {
		return data, fmt.Errorf("iterate recent played games: %w", err)
	}
	if len(data.RecentPlayed) > 0 {
		data.LastPlayed = &data.RecentPlayed[0]
	}

	// 2. 今日游戏时长
	queryToday := `SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= ?`
	err = s.db.QueryRow(queryToday, startOfDay).Scan(&data.TodayPlayTimeSec)
	if err != nil {
		return data, fmt.Errorf("query today play time: %w", err)
	}

	// 3. 本周游戏时长
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysToSubtract := weekday - 1
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -daysToSubtract)

	queryWeek := `SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= ?`
	err = s.db.QueryRow(queryWeek, startOfWeek).Scan(&data.WeeklyPlayTimeSec)
	if err != nil {
		return data, fmt.Errorf("query weekly play time: %w", err)
	}

	return data, nil
}

func (s *HomeService) GetOrCreateCurrentUser() (models.User, error) {
	return models.User{}, nil
}
