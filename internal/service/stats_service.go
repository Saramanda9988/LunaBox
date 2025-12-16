package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/vo"
	"net/http"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

// ExportStatsImage TODO:不是好做法，应该使用wails本地缓存机制缓存图片到本地，而不是现获取
func (s *StatsService) ExportStatsImage(base64Data string) error {
	// Remove header if present (e.g., "data:image/png;base64,")
	if idx := strings.Index(base64Data, ","); idx != -1 {
		base64Data = base64Data[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %w", err)
	}

	filename, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "lunabox-stats.png",
		Title:           "Save Stats Image",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PNG Images (*.png)",
				Pattern:     "*.png",
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to open save dialog: %w", err)
	}

	if filename == "" {
		return nil // User cancelled
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

func (s *StatsService) FetchImageAsBase64(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image, status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // Default fallback
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
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

func (s *StatsService) GetGlobalPeriodStats(dimension enums.Period) (vo.PeriodStats, error) {
	var stats vo.PeriodStats
	stats.Dimension = dimension

	var (
		startDateExpr string
		seriesStart   string
		seriesEnd     string
		stepInterval  string
		dateFormat    string
		truncUnit     string
	)

	switch dimension {
	case "week":
		startDateExpr = "current_date - INTERVAL 6 DAY"
		seriesStart = "current_date - INTERVAL 6 DAY"
		seriesEnd = "current_date"
		stepInterval = "INTERVAL 1 DAY"
		dateFormat = "%Y-%m-%d"
		truncUnit = "day"
	case "month":
		startDateExpr = "current_date - INTERVAL 29 DAY"
		seriesStart = "current_date - INTERVAL 29 DAY"
		seriesEnd = "current_date"
		stepInterval = "INTERVAL 1 DAY"
		dateFormat = "%Y-%m-%d"
		truncUnit = "day"
	case "year":
		startDateExpr = "date_trunc('month', current_date) - INTERVAL 11 MONTH"
		seriesStart = "date_trunc('month', current_date) - INTERVAL 11 MONTH"
		seriesEnd = "date_trunc('month', current_date)"
		stepInterval = "INTERVAL 1 MONTH"
		dateFormat = "%Y-%m"
		truncUnit = "month"
	default:
		return stats, fmt.Errorf("invalid dimension: %s", dimension)
	}

	// 1. Total Play Count & Duration
	queryTotal := fmt.Sprintf("SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= %s", startDateExpr)
	err := s.db.QueryRowContext(s.ctx, queryTotal).Scan(&stats.TotalPlayCount, &stats.TotalPlayDuration)
	if err != nil {
		return stats, err
	}

	// 2. Leaderboard (Top 5)
	stats.PlayTimeLeaderboard = make([]vo.GamePlayStats, 0)
	queryLeaderboard := fmt.Sprintf(`
		SELECT ps.game_id, g.name, g.cover_url, SUM(ps.duration) as total 
		FROM play_sessions ps 
		JOIN games g ON ps.game_id = g.id 
		WHERE ps.start_time >= %s
		GROUP BY ps.game_id, g.name, g.cover_url 
		ORDER BY total DESC 
		LIMIT 5
	`, startDateExpr)

	rows, err := s.db.QueryContext(s.ctx, queryLeaderboard)
	if err != nil {
		return stats, err
	}

	for rows.Next() {
		var item vo.GamePlayStats
		if err := rows.Scan(&item.GameID, &item.GameName, &item.CoverUrl, &item.TotalDuration); err != nil {
			rows.Close()
			return stats, err
		}
		stats.PlayTimeLeaderboard = append(stats.PlayTimeLeaderboard, item)
	}
	rows.Close()

	// 3. Timeline (Total)
	stats.Timeline = make([]vo.TimePoint, 0)
	queryTimeline := fmt.Sprintf(`
		WITH dates AS (
			SELECT generate_series AS day 
			FROM generate_series(%s, %s, %s)
		)
		SELECT 
			strftime(d.day, '%s'), 
			COALESCE(SUM(ps.duration), 0)
		FROM dates d
		LEFT JOIN play_sessions ps ON date_trunc('%s', ps.start_time) = d.day
		GROUP BY d.day
		ORDER BY d.day ASC
	`, seriesStart, seriesEnd, stepInterval, dateFormat, truncUnit)

	rows, err = s.db.QueryContext(s.ctx, queryTimeline)
	if err != nil {
		return stats, err
	}

	for rows.Next() {
		var item vo.TimePoint
		if err := rows.Scan(&item.Label, &item.Duration); err != nil {
			rows.Close()
			return stats, err
		}
		stats.Timeline = append(stats.Timeline, item)
	}
	rows.Close()

	// 4. Leaderboard Series
	stats.LeaderboardSeries = make([]vo.GameTrendSeries, 0)
	for _, game := range stats.PlayTimeLeaderboard {
		series := vo.GameTrendSeries{
			GameID:   game.GameID,
			GameName: game.GameName,
			Points:   make([]vo.TimePoint, 0),
		}

		queryGameSeries := fmt.Sprintf(`
			WITH dates AS (
				SELECT generate_series AS day 
				FROM generate_series(%s, %s, %s)
			)
			SELECT 
				strftime(d.day, '%s'), 
				COALESCE(SUM(ps.duration), 0)
			FROM dates d
			LEFT JOIN play_sessions ps ON ps.game_id = ? AND date_trunc('%s', ps.start_time) = d.day
			GROUP BY d.day
			ORDER BY d.day ASC
		`, seriesStart, seriesEnd, stepInterval, dateFormat, truncUnit)

		rows, err := s.db.QueryContext(s.ctx, queryGameSeries, game.GameID)
		if err != nil {
			return stats, err
		}

		for rows.Next() {
			var p vo.TimePoint
			if err := rows.Scan(&p.Label, &p.Duration); err != nil {
				rows.Close()
				return stats, err
			}
			series.Points = append(series.Points, p)
		}
		rows.Close()
		stats.LeaderboardSeries = append(stats.LeaderboardSeries, series)
	}

	return stats, nil
}
