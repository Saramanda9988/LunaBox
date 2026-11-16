package service

import (
	"context"
	"database/sql"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"

	_ "lunabox/backend/utils/dbutils/duckdbdriver"
)

func setupStatsServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	applog.SetMode(applog.ModeCLI)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	queries := []string{
		`CREATE TABLE play_sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			start_time TIMESTAMP,
			end_time TIMESTAMP,
			duration INTEGER
		)`,
		`CREATE TABLE games (
			id TEXT PRIMARY KEY,
			name TEXT,
			cover_url TEXT,
			is_nsfw BOOLEAN,
			status TEXT,
			created_at TIMESTAMP
		)`,
		`CREATE TABLE game_tags (
			game_id TEXT,
			name TEXT,
			is_spoiler BOOLEAN
		)`,
		`INSERT INTO games VALUES ('game-1', 'Game 1', '', FALSE, 'completed', '2015-01-01 00:00:00')`,
		// 起始日期之前、同月内的记录不应计入趋势
		`INSERT INTO play_sessions VALUES ('s0', 'game-1', '2015-01-10 20:00:00', '2015-01-10 21:00:00', 3600)`,
		`INSERT INTO play_sessions VALUES ('s1', 'game-1', '2015-01-20 20:00:00', '2015-01-20 20:30:00', 1800)`,
		`INSERT INTO play_sessions VALUES ('s2', 'game-1', '2026-10-01 20:00:00', '2026-10-01 20:10:00', 600)`,
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("init test schema: %v", err)
		}
	}

	return db
}

func newTestStatsService(t *testing.T) *StatsService {
	t.Helper()

	statsService := NewStatsService()
	statsService.Init(context.Background(), setupStatsServiceTestDB(t), &appconf.AppConfig{})
	return statsService
}

func TestGetGlobalPeriodStatsAggregatesLongRangeByMonth(t *testing.T) {
	statsService := newTestStatsService(t)

	stats, err := statsService.GetGlobalPeriodStats(vo.PeriodStatsRequest{
		Dimension: enums.Week,
		StartDate: "2015-01-15",
		EndDate:   "2026-10-11",
	})
	if err != nil {
		t.Fatalf("get global period stats: %v", err)
	}

	// 2015-01 ~ 2026-10 共 142 个月
	if len(stats.Timeline) != 142 {
		t.Fatalf("expected 142 monthly timeline points, got %d", len(stats.Timeline))
	}
	first := stats.Timeline[0]
	if first.Label != "2015-01" || first.Duration != 1800 {
		t.Fatalf("unexpected first point: %+v", first)
	}
	last := stats.Timeline[len(stats.Timeline)-1]
	if last.Label != "2026-10" || last.Duration != 600 {
		t.Fatalf("unexpected last point: %+v", last)
	}

	if len(stats.LeaderboardSeries) != 1 {
		t.Fatalf("expected 1 leaderboard series, got %d", len(stats.LeaderboardSeries))
	}
	points := stats.LeaderboardSeries[0].Points
	if len(points) != 142 || points[0].Duration != 1800 {
		t.Fatalf("unexpected leaderboard series: len=%d first=%+v", len(points), points[0])
	}
}

func TestGetGlobalPeriodStatsKeepsShortRangeDaily(t *testing.T) {
	statsService := newTestStatsService(t)

	stats, err := statsService.GetGlobalPeriodStats(vo.PeriodStatsRequest{
		Dimension: enums.Week,
		StartDate: "2015-01-15",
		EndDate:   "2015-01-21",
	})
	if err != nil {
		t.Fatalf("get global period stats: %v", err)
	}

	if len(stats.Timeline) != 7 {
		t.Fatalf("expected 7 daily timeline points, got %d", len(stats.Timeline))
	}
	if stats.Timeline[0].Label != "2015-01-15" {
		t.Fatalf("unexpected first label: %s", stats.Timeline[0].Label)
	}
}

// 同一游戏从多个数据源拿到同名标签（含只差大小写的）时，标签时长不能按来源数重复累加，也不能超过期间总时长。
func TestGetGlobalPeriodStatsTagDistributionDedupesSameTagAcrossSources(t *testing.T) {
	statsService := newTestStatsService(t)
	for _, query := range []string{
		`INSERT INTO game_tags VALUES ('game-1', '汉化', FALSE)`, // kungal
		`INSERT INTO game_tags VALUES ('game-1', '汉化', FALSE)`, // hikarinagi
		`INSERT INTO game_tags VALUES ('game-1', '纯爱', FALSE)`,
		`INSERT INTO game_tags VALUES ('game-1', 'ADV', FALSE)`, // vndb
		`INSERT INTO game_tags VALUES ('game-2', 'ADV', FALSE)`, // 没有游玩记录，只让 ADV 成为用得多的写法
		`INSERT INTO game_tags VALUES ('game-1', 'adv', FALSE)`, // 其他来源，只差大小写
	} {
		if _, err := statsService.db.Exec(query); err != nil {
			t.Fatalf("insert tag: %v", err)
		}
	}

	stats, err := statsService.GetGlobalPeriodStats(vo.PeriodStatsRequest{
		Dimension: enums.All,
	})
	if err != nil {
		t.Fatalf("get global period stats: %v", err)
	}

	if len(stats.TagDistribution) != 3 {
		t.Fatalf("expected 3 tags, got %+v", stats.TagDistribution)
	}
	for _, tag := range stats.TagDistribution {
		if tag.Name == "adv" {
			t.Fatalf("expected the more common spelling ADV, got %+v", stats.TagDistribution)
		}
		if tag.TotalDuration != stats.TotalPlayDuration {
			t.Fatalf("tag %q duration %d, want period total %d", tag.Name, tag.TotalDuration, stats.TotalPlayDuration)
		}
		if tag.GameCount != 1 {
			t.Fatalf("tag %q game count %d, want 1", tag.Name, tag.GameCount)
		}
	}
}

func TestGetGlobalPeriodStatsAllDimensionAggregatesByMonth(t *testing.T) {
	statsService := newTestStatsService(t)

	stats, err := statsService.GetGlobalPeriodStats(vo.PeriodStatsRequest{
		Dimension: enums.All,
	})
	if err != nil {
		t.Fatalf("get global period stats: %v", err)
	}

	if len(stats.Timeline) == 0 || stats.Timeline[0].Label != "2015-01" {
		t.Fatalf("expected monthly timeline starting at 2015-01, got %+v", stats.Timeline)
	}
	if stats.Timeline[0].Duration != 5400 {
		t.Fatalf("expected first month duration 5400, got %d", stats.Timeline[0].Duration)
	}
}
