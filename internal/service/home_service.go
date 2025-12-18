package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/vo"
	"time"
)

type HomeService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewHomeService() *HomeService {
	return &HomeService{}
}

func (s *HomeService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *HomeService) GetHomePageData() (vo.HomePageData, error) {
	var data vo.HomePageData
	data.RecentGames = []models.Game{}
	data.RecentlyAdded = []models.Game{}

	// 1. 上一次游玩的游戏（前三个）
	recentGamesQuery := `
		SELECT g.id, g.name, g.cover_url, g.company, g.summary, g.path, g.source_type, g.cached_at, g.source_id, g.created_at
		FROM games g
		JOIN (
			SELECT game_id, MAX(start_time) as last_played
			FROM play_sessions
			GROUP BY game_id
			ORDER BY last_played DESC
			LIMIT 3
		) ps ON g.id = ps.game_id
		ORDER BY ps.last_played DESC
	`
	rows, err := s.db.Query(recentGamesQuery)
	if err != nil {
		return data, err
	}
	defer rows.Close()

	for rows.Next() {
		var g models.Game
		err := rows.Scan(
			&g.ID, &g.Name, &g.CoverURL, &g.Company, &g.Summary, &g.Path, &g.SourceType, &g.CachedAt, &g.SourceID, &g.CreatedAt,
		)
		if err != nil {
			return data, err
		}
		data.RecentGames = append(data.RecentGames, g)
	}

	// 2. 今日游戏时长
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	queryToday := `SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= ?`
	err = s.db.QueryRow(queryToday, startOfDay).Scan(&data.TodayPlayTimeSec)
	if err != nil {
		return data, err
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
		return data, err
	}

	return data, nil
}

func (s *HomeService) GetOrCreateCurrentUser() (models.User, error) {
	return models.User{}, nil
}
