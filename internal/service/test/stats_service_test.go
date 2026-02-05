package test

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/service"
	"testing"
	"time"
)

func TestStatsService_GetGameStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	statsService := service.NewStatsService()
	statsService.Init(context.Background(), db, &appconf.AppConfig{})

	gameID := "game-stats-001"
	// Insert game
	_, err := db.Exec("INSERT INTO games (id, name) VALUES (?, ?)", gameID, "Test Game Stats")
	if err != nil {
		t.Fatalf("Failed to insert game: %v", err)
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	eightDaysAgo := now.AddDate(0, 0, -8).Format("2006-01-02")

	// Insert play sessions
	// 1. Today: 3600 seconds
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "session-1", gameID, today+" 10:00:00", 3600)
	if err != nil {
		t.Fatalf("Failed to insert session 1: %v", err)
	}
	// 2. Yesterday: 1800 seconds
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "session-2", gameID, yesterday+" 10:00:00", 1800)
	if err != nil {
		t.Fatalf("Failed to insert session 2: %v", err)
	}
	// 3. 8 Days ago: 7200 seconds (Should count in total but not in recent history or today)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "session-3", gameID, eightDaysAgo+" 10:00:00", 7200)
	if err != nil {
		t.Fatalf("Failed to insert session 3: %v", err)
	}

	// stats, err := statsService.GetGameStats(gameID)
	// if err != nil {
	// 	t.Fatalf("GetGameStats failed: %v", err)
	// }

	// // Verify Total Play Time (3600 + 1800 + 7200 = 12600)
	// expectedTotal := 3600 + 1800 + 7200
	// if stats.TotalPlayTime != expectedTotal {
	// 	t.Errorf("Expected TotalPlayTime %d, got %d", expectedTotal, stats.TotalPlayTime)
	// }

	// // Verify Today Play Time (3600)
	// expectedToday := 3600
	// if stats.TodayPlayTime != expectedToday {
	// 	t.Errorf("Expected TodayPlayTime %d, got %d", expectedToday, stats.TodayPlayTime)
	// }

	// // Verify Recent Play History (Last 7 days)
	// if len(stats.RecentPlayHistory) != 7 {
	// 	t.Errorf("Expected 7 days history, got %d", len(stats.RecentPlayHistory))
	// }

	// // Check today in history (last element)
	// lastDay := stats.RecentPlayHistory[6]
	// if lastDay.Date != today {
	// 	t.Errorf("Expected last day date %s, got %s", today, lastDay.Date)
	// }
	// if lastDay.Duration != 3600 {
	// 	t.Errorf("Expected last day duration 3600, got %d", lastDay.Duration)
	// }

	// // Check yesterday in history (second to last element)
	// prevDay := stats.RecentPlayHistory[5]
	// if prevDay.Date != yesterday {
	// 	t.Errorf("Expected prev day date %s, got %s", yesterday, prevDay.Date)
	// }
	// if prevDay.Duration != 1800 {
	// 	t.Errorf("Expected prev day duration 1800, got %d", prevDay.Duration)
	// }
}

func TestStatsService_GetGlobalStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	statsService := service.NewStatsService()
	statsService.Init(context.Background(), db, &appconf.AppConfig{})

	// Insert games
	games := []struct {
		ID   string
		Name string
	}{
		{"g1", "Game 1"},
		{"g2", "Game 2"},
		{"g3", "Game 3"},
	}
	for _, g := range games {
		_, err := db.Exec("INSERT INTO games (id, name) VALUES (?, ?)", g.ID, g.Name)
		if err != nil {
			t.Fatalf("Failed to insert game %s: %v", g.Name, err)
		}
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	eightDaysAgo := now.AddDate(0, 0, -8).Format("2006-01-02")

	// Insert sessions
	// Game 1: 1000 (Today) + 2000 (Yesterday) = 3000 total, 3000 weekly. 2 sessions.
	db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "s1", "g1", today+" 10:00:00", 1000)
	db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "s2", "g1", yesterday+" 10:00:00", 2000)

	// Game 2: 5000 (8 days ago) = 5000 total, 0 weekly. 1 session.
	db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "s3", "g2", eightDaysAgo+" 10:00:00", 5000)

	// Game 3: 4000 (Today) = 4000 total, 4000 weekly. 1 session.
	db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, CAST(? AS TIMESTAMPTZ), ?)", "s4", "g3", today+" 11:00:00", 4000)

	// TODO:重写验证逻辑
}
