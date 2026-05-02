package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
)

type AIStatsProvider interface {
	Build(dimension enums.Period) (*AIStatsData, error)
}

// AIStatsData AI总结所需的统计数据。
type AIStatsData struct {
	Dimension         string
	StartDate         string
	EndDate           string
	DateRange         string // "YYYY-MM-DD 至 YYYY-MM-DD"
	TotalPlayCount    int
	TotalPlayDuration int
	TopGames          []GamePlayInfo
	RecentSessions    []SessionInfo
}

// GamePlayInfo 单款游戏的汇总信息（已扩展 metadata）。
type GamePlayInfo struct {
	GameID          string
	Name            string
	Company         string
	Duration        int      // 秒
	Summary         string   // 截断至 300 字
	Categories      []string // 分类标签
	Status          string   // not_started / playing / completed / on_hold
	SpoilerBoundary string   // 来自 game_progress 或全局配置
	ProgressNote    string   // 玩家备注
	Route           string   // 当前路线
}

// SessionInfo 近期 session 流水（用于作息分析）。
type SessionInfo struct {
	GameID    string
	GameName  string
	StartTime time.Time
	Duration  int
	DayOfWeek int // 0=周日
	Hour      int // 本地时间小时
}

type AIStatsBuilder struct {
	ctx       context.Context
	db        *sql.DB
	appConfig *appconf.AppConfig
}

func NewAIStatsBuilder() *AIStatsBuilder {
	return &AIStatsBuilder{}
}

func (b *AIStatsBuilder) Init(ctx context.Context, db *sql.DB, appConfig *appconf.AppConfig) {
	b.ctx = ctx
	b.db = db
	b.appConfig = appConfig
}

func (b *AIStatsBuilder) Build(dimension enums.Period) (*AIStatsData, error) {
	if b.db == nil {
		return nil, fmt.Errorf("AI stats builder database is not initialized")
	}

	if b.ctx == nil {
		b.ctx = context.Background()
	}

	switch dimension {
	case enums.Day, enums.Week, enums.Month:
	default:
		dimension = enums.Week
	}

	data := &AIStatsData{
		Dimension: string(dimension),
	}

	loc := time.UTC
	if b.appConfig != nil {
		if tz := b.appConfig.TimeZone; tz != "" {
			if resolved, err := time.LoadLocation(tz); err == nil && resolved != nil {
				loc = resolved
			}
		}
	}
	now := time.Now().In(loc)

	var startDateExpr string
	endDateExpr := "current_date"
	var startDate time.Time
	switch dimension {
	case enums.Day, enums.Week:
		startDateExpr = "current_date - INTERVAL 6 DAY"
		startDate = now.AddDate(0, 0, -6)
	case enums.Month:
		startDateExpr = "current_date - INTERVAL 29 DAY"
		startDate = now.AddDate(0, 0, -29)
	default:
		startDateExpr = "current_date - INTERVAL 6 DAY"
		startDate = now.AddDate(0, 0, -6)
	}

	data.StartDate = startDate.Format("2006-01-02")
	data.EndDate = now.Format("2006-01-02")
	data.DateRange = fmt.Sprintf("%s 至 %s", data.StartDate, data.EndDate)

	queryTotal := fmt.Sprintf(
		"SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= %s AND start_time <= %s + INTERVAL 1 DAY",
		startDateExpr,
		endDateExpr,
	)
	if err := b.db.QueryRowContext(b.ctx, queryTotal).Scan(&data.TotalPlayCount, &data.TotalPlayDuration); err != nil {
		return nil, fmt.Errorf("query total AI stats: %w", err)
	}

	globalSpoiler := "none"
	if b.appConfig != nil {
		globalSpoiler = NormalizeSpoilerLevel(b.appConfig.AISpoilerLevel)
	}

	queryLeaderboard := fmt.Sprintf(`
		SELECT
			g.id,
			COALESCE(g.name, '') AS name,
			COALESCE(g.company, '') AS company,
			COALESCE(SUM(ps.duration), 0) AS total_duration,
			COALESCE(LEFT(g.summary, 300), '') AS summary,
			COALESCE(g.status, 'not_started') AS status,
			COALESCE(gp.spoiler_boundary, ?) AS spoiler_boundary,
			COALESCE(gp.progress_note, '') AS progress_note,
			COALESCE(gp.route, '') AS route
		FROM play_sessions ps
		JOIN games g ON ps.game_id = g.id
		LEFT JOIN (
			SELECT game_id, spoiler_boundary, progress_note, route
			FROM (
				SELECT
					game_id,
					spoiler_boundary,
					progress_note,
					route,
					ROW_NUMBER() OVER (
						PARTITION BY game_id
						ORDER BY updated_at DESC, id DESC
					) AS rn
				FROM game_progress
			) latest_progress
			WHERE rn = 1
		) gp ON g.id = gp.game_id
		WHERE ps.start_time >= %s AND ps.start_time <= %s + INTERVAL 1 DAY
		GROUP BY g.id, g.name, g.company, g.summary, g.status,
		         gp.spoiler_boundary, gp.progress_note, gp.route
		ORDER BY total_duration DESC, g.id ASC
		LIMIT 5
	`, startDateExpr, endDateExpr)

	rows, err := b.db.QueryContext(b.ctx, queryLeaderboard, globalSpoiler)
	if err != nil {
		return nil, fmt.Errorf("query AI leaderboard: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var info GamePlayInfo
		if err := rows.Scan(
			&info.GameID,
			&info.Name,
			&info.Company,
			&info.Duration,
			&info.Summary,
			&info.Status,
			&info.SpoilerBoundary,
			&info.ProgressNote,
			&info.Route,
		); err != nil {
			return nil, fmt.Errorf("scan AI leaderboard row: %w", err)
		}
		data.TopGames = append(data.TopGames, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI leaderboard rows: %w", err)
	}

	for i, game := range data.TopGames {
		catRows, err := b.db.QueryContext(b.ctx, `
			SELECT COALESCE(c.name, '')
			FROM game_categories gc
			JOIN categories c ON gc.category_id = c.id
			WHERE gc.game_id = ?
			ORDER BY c.created_at, c.id
		`, game.GameID)
		if err != nil {
			continue
		}

		var cats []string
		for catRows.Next() {
			var cat string
			if err := catRows.Scan(&cat); err == nil && cat != "" {
				cats = append(cats, cat)
			}
		}
		catRows.Close()
		data.TopGames[i].Categories = cats
	}

	contextLimit := 20
	if b.appConfig != nil && b.appConfig.AIContextWindowSize > 0 {
		contextLimit = b.appConfig.AIContextWindowSize
	}

	tz := "UTC"
	if b.appConfig != nil && b.appConfig.TimeZone != "" {
		tz = b.appConfig.TimeZone
	}

	sessionQuery := fmt.Sprintf(`
		SELECT
			ps.game_id,
			COALESCE(g.name, '') AS game_name,
			ps.start_time,
			COALESCE(ps.duration, 0) AS duration,
			dayofweek(timezone(?, ps.start_time)) AS dow,
			hour(timezone(?, ps.start_time)) AS hr
		FROM play_sessions ps
		JOIN games g ON ps.game_id = g.id
		WHERE ps.start_time >= %s AND ps.start_time <= %s + INTERVAL 1 DAY
		ORDER BY ps.start_time DESC, ps.id DESC
		LIMIT ?
	`, startDateExpr, endDateExpr)
	sessRows, err := b.db.QueryContext(b.ctx, sessionQuery, tz, tz, contextLimit)
	if err != nil {
		return data, nil
	}
	defer sessRows.Close()

	for sessRows.Next() {
		var si SessionInfo
		if err := sessRows.Scan(&si.GameID, &si.GameName, &si.StartTime, &si.Duration, &si.DayOfWeek, &si.Hour); err == nil {
			data.RecentSessions = append(data.RecentSessions, si)
		}
	}

	return data, nil
}
