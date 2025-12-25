package test

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/service"
	"testing"
	"time"
)

func TestHomeService_GetHomePageData(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	homeService := service.NewHomeService()
	homeService.Init(context.Background(), db, &appconf.AppConfig{})

	// Prepare test data
	game1ID := "game-1"
	game2ID := "game-2"
	game3ID := "game-3"
	game4ID := "game-4"

	// Insert games
	games := []struct {
		id   string
		name string
	}{
		{game1ID, "Game 1"},
		{game2ID, "Game 2"},
		{game3ID, "Game 3"},
		{game4ID, "Game 4"},
	}

	for _, g := range games {
		_, err := db.Exec(`
			INSERT INTO games (id, name, cover_url, company, summary, path, source_type, cached_at, source_id, created_at) 
			VALUES (?, ?, '', 'Company', 'Summary', 'path', 'local', CURRENT_TIMESTAMP, 'src1', CURRENT_TIMESTAMP)`,
			g.id, g.name)
		if err != nil {
			t.Fatalf("Failed to insert game %s: %v", g.id, err)
		}
	}

	now := time.Now()

	// Calculate a date that is definitely earlier this week (but not today if possible, or just today)
	// For simplicity in testing "Weekly", let's just use today and yesterday.
	// If today is Monday, yesterday was last week.
	// So we need to be careful with "Weekly" logic test depending on the current day of the week.
	// However, for a unit test, we can control the input or just rely on the logic that "Today" is part of "This Week".

	// Let's construct specific dates relative to "Now" to ensure they fall into buckets.

	// 1. Session for Game 1: Today, 1 hour ago. Duration 3600s.
	session1Time := now.Add(-1 * time.Hour)
	_, err := db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, ?, ?)",
		"session-1", game1ID, session1Time, 3600)
	if err != nil {
		t.Fatalf("Failed to insert session 1: %v", err)
	}

	// 2. Session for Game 2: Today, 2 hours ago. Duration 1800s.
	session2Time := now.Add(-2 * time.Hour)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, ?, ?)",
		"session-2", game2ID, session2Time, 1800)
	if err != nil {
		t.Fatalf("Failed to insert session 2: %v", err)
	}

	// 3. Session for Game 3: Yesterday.
	isMonday := now.Weekday() == time.Monday

	// Session 3: 3 hours ago today.
	session3Time := now.Add(-3 * time.Hour)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, ?, ?)",
		"session-3", game3ID, session3Time, 1200)
	if err != nil {
		t.Fatalf("Failed to insert session 3: %v", err)
	}

	// 4. Session for Game 4: 1 year ago.
	session4Time := now.AddDate(-1, 0, 0)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, ?, ?)",
		"session-4", game4ID, session4Time, 100)
	if err != nil {
		t.Fatalf("Failed to insert session 4: %v", err)
	}

	// Execute
	data, err := homeService.GetHomePageData()
	if err != nil {
		t.Fatalf("GetHomePageData failed: %v", err)
	}

	// Assertions

	// 2. Today Play Time
	expectedToday := 3600 + 1800 + 1200
	if data.TodayPlayTimeSec != expectedToday {
		t.Errorf("Expected today play time %d, got %d", expectedToday, data.TodayPlayTimeSec)
	}

	// 3. Weekly Play Time
	if !isMonday {
		// Insert a session for yesterday
		yesterdayTime := now.AddDate(0, 0, -1)
		_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, duration) VALUES (?, ?, ?, ?)",
			"session-yesterday", game1ID, yesterdayTime, 500)
		if err != nil {
			t.Fatalf("Failed to insert yesterday session: %v", err)
		}

		// Re-fetch data
		data, err = homeService.GetHomePageData()
		if err != nil {
			t.Fatalf("GetHomePageData failed: %v", err)
		}

		// Today should remain same
		if data.TodayPlayTimeSec != expectedToday {
			t.Errorf("Expected today play time %d, got %d", expectedToday, data.TodayPlayTimeSec)
		}

		// Weekly should increase by 500
		expectedWeekly := expectedToday + 500
		if data.WeeklyPlayTimeSec != expectedWeekly {
			t.Errorf("Expected weekly play time %d, got %d", expectedWeekly, data.WeeklyPlayTimeSec)
		}
	} else {
		if data.WeeklyPlayTimeSec != expectedToday {
			t.Errorf("Expected weekly play time %d, got %d", expectedToday, data.WeeklyPlayTimeSec)
		}
	}
}
