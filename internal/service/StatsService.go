package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/vo"
)

type StatsService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewStatsService() *StatsService {
	return &StatsService{}
}

func (s *StatsService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *StatsService) GetGameStats(gameID string) (vo.GameDetailStats, error) {
	var stats vo.GameDetailStats

	// 1. Total Play Time
	err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE game_id = ?", gameID).Scan(&stats.TotalPlayTime)
	if err != nil {
		return stats, err
	}

	// 2. Today Play Time
	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE game_id = ? AND start_time >= current_date", gameID).Scan(&stats.TodayPlayTime)
	if err != nil {
		return stats, err
	}

	// 3. Recent Play History (Last 7 days)
	// Use DuckDB generate_series to create the date range and left join to ensure all days are present
	query := `
		WITH dates AS (
			SELECT generate_series AS day 
			FROM generate_series(current_date - INTERVAL 6 DAY, current_date, INTERVAL 1 DAY)
		)
		SELECT 
			strftime(d.day, '%Y-%m-%d'), 
			COALESCE(SUM(ps.duration), 0)
		FROM dates d
		LEFT JOIN play_sessions ps ON ps.game_id = ? AND ps.start_time::DATE = d.day
		GROUP BY d.day
		ORDER BY d.day ASC
	`

	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(COUNT(*), 0) FROM play_sessions WHERE game_id = ?", gameID).Scan(&stats.TotalPlayCount)
	if err != nil {
		return vo.GameDetailStats{}, err
	}

	rows, err := s.db.QueryContext(s.ctx, query, gameID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	stats.RecentPlayHistory = make([]vo.DailyPlayTime, 0, 7)
	for rows.Next() {
		var item vo.DailyPlayTime
		if err := rows.Scan(&item.Date, &item.Duration); err != nil {
			return stats, err
		}
		stats.RecentPlayHistory = append(stats.RecentPlayHistory, item)
	}

	return stats, nil
}

func (s *StatsService) GetGlobalStats() (vo.GlobalStats, error) {
	var stats vo.GlobalStats

	// 1. Total Play Time
	err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(SUM(duration), 0) FROM play_sessions").Scan(&stats.TotalPlayTime)
	if err != nil {
		return stats, err
	}

	// 2. Weekly Play Time (Last 7 days)
	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= current_date - INTERVAL 6 DAY").Scan(&stats.WeeklyPlayTime)
	if err != nil {
		return stats, err
	}

	// 3. Play Time Leaderboard (Top 10)
	stats.PlayTimeLeaderboard = make([]vo.GamePlayStats, 0)
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT ps.game_id, g.name, SUM(ps.duration) as total 
		FROM play_sessions ps 
		JOIN games g ON ps.game_id = g.id 
		GROUP BY ps.game_id, g.name 
		ORDER BY total DESC 
		LIMIT 10
	`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var item vo.GamePlayStats
		if err := rows.Scan(&item.GameID, &item.GameName, &item.TotalDuration); err != nil {
			continue
		}
		stats.PlayTimeLeaderboard = append(stats.PlayTimeLeaderboard, item)
	}

	// 4. Most Played Game (by count)
	err = s.db.QueryRowContext(s.ctx, `
		SELECT ps.game_id, g.name, COUNT(*) as cnt 
		FROM play_sessions ps 
		JOIN games g ON ps.game_id = g.id 
		GROUP BY ps.game_id, g.name 
		ORDER BY cnt DESC 
		LIMIT 1
	`).Scan(&stats.MostPlayedGame.GameID, &stats.MostPlayedGame.GameName, &stats.MostPlayedGame.PlayCount)

	if err == sql.ErrNoRows {
		stats.MostPlayedGame = vo.GamePlayCount{}
	} else if err != nil {
		return stats, err
	}

	return stats, nil
}
