package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
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
		applog.LogErrorf(s.ctx, "failed to decode base64 data: %v", err)
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
		applog.LogErrorf(s.ctx, "failed to open save dialog: %v", err)
		return fmt.Errorf("failed to open save dialog: %w", err)
	}

	if filename == "" {
		return nil // User cancelled
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		applog.LogErrorf(s.ctx, "failed to save file: %v", err)
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

func (s *StatsService) FetchImageAsBase64(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to fetch image: %v", err)
		return "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		applog.LogErrorf(s.ctx, "failed to fetch image, status code: %d", resp.StatusCode)
		return "", fmt.Errorf("failed to fetch image, status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to read image body: %v", err)
		return "", fmt.Errorf("failed to read image body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // Default fallback
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

func (s *StatsService) GetGameStats(req vo.GameStatsRequest) (vo.GameDetailStats, error) {
	var stats vo.GameDetailStats
	stats.Dimension = string(req.Dimension)

	var (
		startDate    string
		endDate      string
		dateFormat   string
		stepInterval string
	)

	// 如果用户提供了自定义日期范围，使用用户的日期
	if req.StartDate != "" && req.EndDate != "" {
		startDate = req.StartDate
		endDate = req.EndDate
	} else {
		// 使用默认范围
		switch req.Dimension {
		case enums.Week:
			// 周：最近7天
			startDate = "current_date - INTERVAL 6 DAY"
			endDate = "current_date"
		case enums.Month:
			// 月：最近30天
			startDate = "current_date - INTERVAL 29 DAY"
			endDate = "current_date"
		case enums.All:
			// 所有记录：从第一条记录到现在
			startDate = "(SELECT MIN(start_time::DATE) FROM play_sessions WHERE game_id = ?)"
			endDate = "current_date"
		default:
			return stats, fmt.Errorf("invalid dimension: %s", req.Dimension)
		}
	}

	// 所有维度都按日聚合
	dateFormat = "%Y-%m-%d"
	stepInterval = "INTERVAL 1 DAY"

	// 构建日期表达式
	var startDateExpr, endDateExpr, seriesStart, seriesEnd string
	if req.StartDate != "" && req.EndDate != "" {
		startDateExpr = fmt.Sprintf("'%s'::DATE", startDate)
		endDateExpr = fmt.Sprintf("'%s'::DATE", endDate)
		seriesStart = fmt.Sprintf("'%s'::DATE", startDate)
		seriesEnd = fmt.Sprintf("'%s'::DATE", endDate)
		stats.StartDate = startDate
		stats.EndDate = endDate
	} else {
		startDateExpr = startDate
		endDateExpr = endDate
		seriesStart = startDate
		seriesEnd = endDate
		// 获取实际日期范围用于显示
		var actualStart, actualEnd string
		if req.Dimension == enums.All {
			err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(MIN(start_time::DATE), current_date), current_date FROM play_sessions WHERE game_id = ?", req.GameID).Scan(&actualStart, &actualEnd)
			if err == nil {
				stats.StartDate = actualStart
				stats.EndDate = actualEnd
			}
		} else {
			err := s.db.QueryRowContext(s.ctx, fmt.Sprintf("SELECT %s, %s", startDateExpr, endDateExpr)).Scan(&actualStart, &actualEnd)
			if err == nil {
				stats.StartDate = actualStart
				stats.EndDate = actualEnd
			}
		}
	}

	// 1. Total Play Count & Duration (in selected period)
	queryTotal := fmt.Sprintf("SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(duration), 0) FROM play_sessions WHERE game_id = ? AND start_time >= %s AND start_time <= %s + INTERVAL 1 DAY", startDateExpr, endDateExpr)
	if req.Dimension == enums.All && req.StartDate == "" {
		// For 'all' dimension without custom dates, we need special handling
		err := s.db.QueryRowContext(s.ctx, queryTotal, req.GameID, req.GameID).Scan(&stats.TotalPlayCount, &stats.TotalPlayTime)
		if err != nil {
			applog.LogErrorf(s.ctx, "failed to get total play count and duration: %v", err)
			return stats, err
		}
	} else {
		err := s.db.QueryRowContext(s.ctx, queryTotal, req.GameID).Scan(&stats.TotalPlayCount, &stats.TotalPlayTime)
		if err != nil {
			applog.LogErrorf(s.ctx, "failed to get total play count and duration: %v", err)
			return stats, err
		}
	}

	// 2. Today Play Time (always show today regardless of period)
	err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(SUM(duration), 0) FROM play_sessions WHERE game_id = ? AND start_time >= current_date", req.GameID).Scan(&stats.TodayPlayTime)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get today play time: %v", err)
		return stats, err
	}

	// 3. Play History Timeline
	queryTimeline := fmt.Sprintf(`
		WITH dates AS (
			SELECT generate_series AS day 
			FROM generate_series(%s, %s, %s)
		)
		SELECT 
			strftime(d.day, '%s'), 
			COALESCE(SUM(ps.duration), 0)
		FROM dates d
		LEFT JOIN play_sessions ps ON ps.game_id = ? AND ps.start_time::DATE = d.day
		GROUP BY d.day
		ORDER BY d.day ASC
	`, seriesStart, seriesEnd, stepInterval, dateFormat)

	var rows *sql.Rows
	if req.Dimension == enums.All && req.StartDate == "" {
		rows, err = s.db.QueryContext(s.ctx, queryTimeline, req.GameID, req.GameID)
	} else {
		rows, err = s.db.QueryContext(s.ctx, queryTimeline, req.GameID)
	}
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to query play history: %v", err)
		return stats, err
	}
	defer rows.Close()

	stats.RecentPlayHistory = make([]vo.DailyPlayTime, 0)
	for rows.Next() {
		var item vo.DailyPlayTime
		if err := rows.Scan(&item.Date, &item.Duration); err != nil {
			applog.LogErrorf(s.ctx, "failed to scan play history: %v", err)
			return stats, err
		}
		stats.RecentPlayHistory = append(stats.RecentPlayHistory, item)
	}

	return stats, nil
}

func (s *StatsService) GetGlobalPeriodStats(req vo.PeriodStatsRequest) (vo.PeriodStats, error) {
	var stats vo.PeriodStats
	stats.Dimension = req.Dimension

	var (
		startDate    string
		endDate      string
		dateFormat   string
		stepInterval string
	)

	// 如果用户提供了自定义日期范围，使用用户的日期
	if req.StartDate != "" && req.EndDate != "" {
		startDate = req.StartDate
		endDate = req.EndDate
	} else {
		// 使用默认范围
		switch req.Dimension {
		case enums.Day:
			// 日维度：默认最近7天（保留兼容）
			startDate = "current_date - INTERVAL 6 DAY"
			endDate = "current_date"
		case enums.Week:
			// 周：最近7天
			startDate = "current_date - INTERVAL 6 DAY"
			endDate = "current_date"
		case enums.Month:
			// 月：最近30天
			startDate = "current_date - INTERVAL 29 DAY"
			endDate = "current_date"
		default:
			return stats, fmt.Errorf("invalid dimension: %s", req.Dimension)
		}
	}

	// 所有维度都按日聚合
	dateFormat = "%Y-%m-%d"
	stepInterval = "INTERVAL 1 DAY"

	// 构建日期表达式
	var startDateExpr, endDateExpr, seriesStart, seriesEnd string
	if req.StartDate != "" && req.EndDate != "" {
		startDateExpr = fmt.Sprintf("'%s'::DATE", startDate)
		endDateExpr = fmt.Sprintf("'%s'::DATE", endDate)
		seriesStart = fmt.Sprintf("'%s'::DATE", startDate)
		seriesEnd = fmt.Sprintf("'%s'::DATE", endDate)
		stats.StartDate = startDate
		stats.EndDate = endDate
	} else {
		startDateExpr = startDate
		endDateExpr = endDate
		seriesStart = startDate
		seriesEnd = endDate
	}

	// 总游玩次数和时长
	queryTotal := fmt.Sprintf("SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= %s AND start_time <= %s + INTERVAL 1 DAY", startDateExpr, endDateExpr)
	err := s.db.QueryRowContext(s.ctx, queryTotal).Scan(&stats.TotalPlayCount, &stats.TotalPlayDuration)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get total play count and duration: %v", err)
		return stats, err
	}

	// 查询本期间内游玩过的游戏数量、已通关游戏数量和库中所有游戏数量
	queryGameStats := fmt.Sprintf(`
		SELECT 
			COUNT(DISTINCT ps.game_id),
			COUNT(DISTINCT CASE WHEN g.status = 'completed' THEN g.id END)
		FROM play_sessions ps
		JOIN games g ON ps.game_id = g.id
		WHERE ps.start_time >= %s AND ps.start_time <= %s + INTERVAL 1 DAY
	`, startDateExpr, endDateExpr)
	err = s.db.QueryRowContext(s.ctx, queryGameStats).Scan(&stats.TotalGamesCount, &stats.CompletedGamesCount)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get game stats: %v", err)
		return stats, err
	}

	// 查询库中所有游戏，一次查询获取数量和已通关数量
	queryLibraryGames := "SELECT status FROM games"
	rows, err := s.db.QueryContext(s.ctx, queryLibraryGames)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get library games: %v", err)
		return stats, err
	}
	defer rows.Close()

	completedCount := 0
	totalCount := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			applog.LogErrorf(s.ctx, "failed to scan game status: %v", err)
			return stats, err
		}
		totalCount++
		if status == "completed" {
			completedCount++
		}
	}
	stats.LibraryGamesCount = totalCount
	stats.AllCompletedGamesCount = completedCount

	// 查询所有session数量和总时长
	queryAllSessions := "SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(duration), 0) FROM play_sessions"
	err = s.db.QueryRowContext(s.ctx, queryAllSessions).Scan(&stats.AllSessionsCount, &stats.AllSessionsDuration)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get all sessions stats: %v", err)
		return stats, err
	}
	stats.PlayTimeLeaderboard = make([]vo.GamePlayStats, 0)
	queryLeaderboard := fmt.Sprintf(`
		SELECT ps.game_id, g.name, COALESCE(g.cover_url, '') as cover_url, SUM(ps.duration) as total 
		FROM play_sessions ps 
		JOIN games g ON ps.game_id = g.id 
		WHERE ps.start_time >= %s AND ps.start_time <= %s + INTERVAL 1 DAY
		GROUP BY ps.game_id, g.name, g.cover_url 
		ORDER BY total DESC 
		LIMIT 5
	`, startDateExpr, endDateExpr)

	// 构建Leaderboard
	rows, err = s.db.QueryContext(s.ctx, queryLeaderboard)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to query leaderboard: %v", err)
		return stats, err
	}

	for rows.Next() {
		var item vo.GamePlayStats
		if err := rows.Scan(&item.GameID, &item.GameName, &item.CoverUrl, &item.TotalDuration); err != nil {
			applog.LogErrorf(s.ctx, "failed to scan leaderboard: %v", err)
			rows.Close()
			return stats, err
		}
		stats.PlayTimeLeaderboard = append(stats.PlayTimeLeaderboard, item)
	}
	rows.Close()

	// 3. Timeline (Total)
	// 注意：使用 ps.start_time::DATE 将 TIMESTAMPTZ 转换为本地日期进行匹配
	// 这样可以正确地按用户本地时区的日期进行聚合
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
		LEFT JOIN play_sessions ps ON ps.start_time::DATE = d.day
		GROUP BY d.day
		ORDER BY d.day ASC
	`, seriesStart, seriesEnd, stepInterval, dateFormat)

	rows, err = s.db.QueryContext(s.ctx, queryTimeline)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to query timeline: %v", err)
		return stats, err
	}

	for rows.Next() {
		var item vo.TimePoint
		if err := rows.Scan(&item.Label, &item.Duration); err != nil {
			applog.LogErrorf(s.ctx, "failed to scan timeline: %v", err)
			rows.Close()
			return stats, err
		}
		stats.Timeline = append(stats.Timeline, item)
	}
	rows.Close()

	// 4. Leaderboard Series
	// 使用 ps.start_time::DATE 进行本地时区日期匹配
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
			LEFT JOIN play_sessions ps ON ps.game_id = ? AND ps.start_time::DATE = d.day
			GROUP BY d.day
			ORDER BY d.day ASC
		`, seriesStart, seriesEnd, stepInterval, dateFormat)

		rows, err := s.db.QueryContext(s.ctx, queryGameSeries, game.GameID)
		if err != nil {
			applog.LogErrorf(s.ctx, "failed to query leaderboard series for game %s: %v", game.GameID, err)
			return stats, err
		}

		for rows.Next() {
			var p vo.TimePoint
			if err := rows.Scan(&p.Label, &p.Duration); err != nil {
				applog.LogErrorf(s.ctx, "failed to scan leaderboard series for game %s: %v", game.GameID, err)
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
