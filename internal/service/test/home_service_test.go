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
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Use fixed times inside the current calendar day. Relative times such as
	// now.Add(-time.Hour) cross into yesterday when this test runs after midnight.
	// 1. Session for Game 1: today at 03:00. Duration 3600s.
	session1Time := startOfToday.Add(3 * time.Hour)
	_, err := db.Exec("INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
		"session-1", game1ID, session1Time, session1Time.Add(time.Hour), 3600)
	if err != nil {
		t.Fatalf("Failed to insert session 1: %v", err)
	}

	// 2. Session for Game 2: today at 02:00. Duration 1800s.
	session2Time := startOfToday.Add(2 * time.Hour)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
		"session-2", game2ID, session2Time, session2Time.Add(30*time.Minute), 1800)
	if err != nil {
		t.Fatalf("Failed to insert session 2: %v", err)
	}

	isMonday := now.Weekday() == time.Monday

	// 3. Session for Game 3: today at 01:00.
	session3Time := startOfToday.Add(time.Hour)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
		"session-3", game3ID, session3Time, session3Time.Add(20*time.Minute), 1200)
	if err != nil {
		t.Fatalf("Failed to insert session 3: %v", err)
	}

	// 4. Session for Game 4: 1 year ago.
	session4Time := now.AddDate(-1, 0, 0)
	_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
		"session-4", game4ID, session4Time, session4Time.Add(100*time.Second), 100)
	if err != nil {
		t.Fatalf("Failed to insert session 4: %v", err)
	}

	// Execute
	data, err := homeService.GetHomePageData()
	if err != nil {
		t.Fatalf("GetHomePageData failed: %v", err)
	}

	// Assertions
	if data.LastPlayed == nil {
		t.Fatal("Expected last played game, got nil")
	}
	if data.LastPlayed.Game.ID != game1ID {
		t.Errorf("Expected last played game %s, got %s", game1ID, data.LastPlayed.Game.ID)
	}
	if data.LastPlayed.TotalPlayedDur != 3600 {
		t.Errorf("Expected last played total duration %d, got %d", 3600, data.LastPlayed.TotalPlayedDur)
	}

	if len(data.RecentPlayed) != 4 {
		t.Fatalf("Expected 4 recent played games, got %d", len(data.RecentPlayed))
	}
	expectedRecent := []struct {
		gameID   string
		duration int
	}{
		{game1ID, 3600},
		{game2ID, 1800},
		{game3ID, 1200},
		{game4ID, 100},
	}
	for index, expected := range expectedRecent {
		actual := data.RecentPlayed[index]
		if actual.Game.ID != expected.gameID {
			t.Errorf("Expected recent game at index %d to be %s, got %s", index, expected.gameID, actual.Game.ID)
		}
		if actual.TotalPlayedDur != expected.duration {
			t.Errorf("Expected recent game %s total duration %d, got %d", expected.gameID, expected.duration, actual.TotalPlayedDur)
		}
	}

	// 2. Today Play Time
	expectedToday := 3600 + 1800 + 1200
	if data.TodayPlayTimeSec != expectedToday {
		t.Errorf("Expected today play time %d, got %d", expectedToday, data.TodayPlayTimeSec)
	}

	// 3. Weekly Play Time
	if !isMonday {
		// Insert a session for yesterday at 01:00.
		yesterdayTime := startOfToday.AddDate(0, 0, -1).Add(time.Hour)
		_, err = db.Exec("INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
			"session-yesterday", game1ID, yesterdayTime, yesterdayTime.Add(500*time.Second), 500)
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
		if len(data.RecentPlayed) == 0 || data.RecentPlayed[0].TotalPlayedDur != 4100 {
			t.Errorf("Expected Game 1 total duration to include yesterday session, got recent list: %#v", data.RecentPlayed)
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

func TestHomeServiceUsesNullEndTimeAsPlayingMarker(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	homeService := service.NewHomeService()
	homeService.Init(context.Background(), db, &appconf.AppConfig{})

	now := time.Now().Truncate(time.Second)
	for _, game := range []struct {
		id   string
		name string
	}{
		{id: "running-game", name: "Running Game"},
		{id: "completed-game", name: "Completed Game"},
	} {
		if _, err := db.Exec(`
			INSERT INTO games (id, name, cached_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			game.id, game.name, now, now, now,
		); err != nil {
			t.Fatalf("insert game %s: %v", game.id, err)
		}
	}

	if _, err := db.Exec(`
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		VALUES (?, ?, ?, NULL, ?, ?)`,
		"running-session",
		"running-game",
		now.Add(-10*time.Minute),
		75,
		now,
	); err != nil {
		t.Fatalf("insert running session: %v", err)
	}

	completedStart := now.Add(-20 * time.Minute)
	if _, err := db.Exec(`
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"completed-session",
		"completed-game",
		completedStart,
		completedStart.Add(time.Minute),
		0,
		now,
	); err != nil {
		t.Fatalf("insert completed session: %v", err)
	}

	data, err := homeService.GetHomePageData()
	if err != nil {
		t.Fatalf("GetHomePageData failed: %v", err)
	}

	playingByGameID := make(map[string]bool, len(data.RecentPlayed))
	for _, item := range data.RecentPlayed {
		playingByGameID[item.Game.ID] = item.IsPlaying
	}
	if !playingByGameID["running-game"] {
		t.Fatal("expected duration > 0 session with NULL end_time to be playing")
	}
	if playingByGameID["completed-game"] {
		t.Fatal("expected duration = 0 session with non-NULL end_time to be completed")
	}
}
