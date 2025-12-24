package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/vo"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 1. 上次游玩的游戏（最近一次游玩记录）
	lastPlayedQuery := `
		SELECT 
			g.id, g.name, g.cover_url, g.company, g.summary, g.path, g.source_type, g.cached_at, g.source_id, g.created_at,
			ps.start_time, ps.duration,
			COALESCE((SELECT SUM(duration) FROM play_sessions WHERE game_id = g.id), 0) as total_duration
		FROM games g
		JOIN play_sessions ps ON g.id = ps.game_id
		WHERE ps.start_time = (SELECT MAX(start_time) FROM play_sessions)
		LIMIT 1
	`
	var g models.Game
	var lastPlayedAt time.Time
	var lastPlayedDur, totalPlayedDur int

	err := s.db.QueryRow(lastPlayedQuery).Scan(
		&g.ID, &g.Name, &g.CoverURL, &g.Company, &g.Summary, &g.Path, &g.SourceType, &g.CachedAt, &g.SourceID, &g.CreatedAt,
		&lastPlayedAt, &lastPlayedDur, &totalPlayedDur,
	)
	if err == nil {
		// duration = 0 表示游戏正在运行（还未结束）
		isPlaying := lastPlayedDur == 0
		data.LastPlayed = &vo.LastPlayedGame{
			Game:           g,
			LastPlayedAt:   lastPlayedAt.Format("2006-01-02 15:04"),
			LastPlayedDur:  lastPlayedDur,
			TotalPlayedDur: totalPlayedDur,
			IsPlaying:      isPlaying,
		}
	} else if err != sql.ErrNoRows {
		runtime.LogErrorf(s.ctx, "查询上次游玩游戏失败: %v", err)
	}

	// 2. 今日游戏时长
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
