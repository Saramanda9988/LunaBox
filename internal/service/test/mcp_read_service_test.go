package test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service"
	"lunabox/internal/utils/metadata"
)

func TestMCPReadServiceListGamesBounded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	baseTime := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	insertTestGameRecord(t, db, "game-1", "Game 1", baseTime.Add(-3*time.Hour))
	insertTestGameRecord(t, db, "game-2", "Game 2", baseTime.Add(-2*time.Hour))
	insertTestGameRecord(t, db, "game-3", "Game 3", baseTime.Add(-1*time.Hour))

	readService := newTestMCPReadService(t, db, &appconf.AppConfig{})
	resp, err := readService.ListGames(2, 0)
	if err != nil {
		t.Fatalf("ListGames failed: %v", err)
	}

	if resp.Total != 3 {
		t.Fatalf("expected total=3, got %d", resp.Total)
	}
	if len(resp.Games) != 2 {
		t.Fatalf("expected 2 games, got %d", len(resp.Games))
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
	if resp.Games[0].GameID != "game-3" || resp.Games[1].GameID != "game-2" {
		t.Fatalf("unexpected game order: %#v", resp.Games)
	}
}

func TestMCPReadServiceGetGameNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	readService := newTestMCPReadService(t, db, &appconf.AppConfig{})
	if _, err := readService.GetGame("missing-game"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestMCPReadServiceGetGameNormalizesSpoilerContext(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	insertTestGameRecord(t, db, "game-1", "Game 1", now)
	if _, err := db.Exec(`INSERT INTO categories (id, name, emoji, is_system, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"cat-1", "推理", "", false, now, now); err != nil {
		t.Fatalf("insert category failed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO game_categories (game_id, category_id, updated_at) VALUES (?, ?, ?)`,
		"game-1", "cat-1", now); err != nil {
		t.Fatalf("insert category relation failed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO game_progress (id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"progress-1", "game-1", "chapter 3", "route a", "midpoint", "chapter_end", now); err != nil {
		t.Fatalf("insert game progress failed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"tag-1", "game-1", "mystery", "user", 1.0, false, now, now); err != nil {
		t.Fatalf("insert game tag failed: %v", err)
	}

	readService := newTestMCPReadService(t, db, &appconf.AppConfig{})
	resp, err := readService.GetGame("game-1")
	if err != nil {
		t.Fatalf("GetGame failed: %v", err)
	}

	if resp.SpoilerContext.GlobalLevel != "none" {
		t.Fatalf("expected spoiler level none, got %s", resp.SpoilerContext.GlobalLevel)
	}
	if resp.Game.LatestProgress == nil {
		t.Fatal("expected latest progress snapshot")
	}
	if len(resp.Game.Categories) != 1 || resp.Game.Categories[0] != "推理" {
		t.Fatalf("unexpected categories: %#v", resp.Game.Categories)
	}
	if len(resp.Game.Tags) != 1 || resp.Game.Tags[0].Name != "mystery" {
		t.Fatalf("unexpected tags: %#v", resp.Game.Tags)
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response failed: %v", err)
	}
	if bytes.Contains(payload, []byte(`"path"`)) {
		t.Fatal("response should not expose local launch path")
	}
	if bytes.Contains(payload, []byte(`"process_name"`)) {
		t.Fatal("response should not expose process name")
	}
}

func TestMCPReadServiceGetPlaySessionsBoundedDescending(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	baseTime := time.Date(2026, 5, 2, 20, 0, 0, 0, time.UTC)
	insertTestGameRecord(t, db, "game-1", "Game 1", baseTime)
	insertTestSessionRecord(t, db, "session-1", "game-1", baseTime.Add(-3*time.Hour), 1800)
	insertTestSessionRecord(t, db, "session-2", "game-1", baseTime.Add(-2*time.Hour), 1800)
	insertTestSessionRecord(t, db, "session-3", "game-1", baseTime.Add(-1*time.Hour), 1800)

	readService := newTestMCPReadService(t, db, &appconf.AppConfig{})
	resp, err := readService.GetPlaySessions("game-1", 2, 0)
	if err != nil {
		t.Fatalf("GetPlaySessions failed: %v", err)
	}

	if resp.Total != 3 {
		t.Fatalf("expected total=3, got %d", resp.Total)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(resp.Sessions))
	}
	if !resp.HasMore {
		t.Fatal("expected has_more=true")
	}
	if resp.Sessions[0].ID != "session-3" || resp.Sessions[1].ID != "session-2" {
		t.Fatalf("unexpected session order: %#v", resp.Sessions)
	}
}

func TestMCPReadServiceStartGameUsesStartService(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Date(2026, 5, 2, 20, 0, 0, 0, time.UTC)
	insertTestGameRecord(t, db, "game-1", "Game 1", now)

	readService := newTestMCPReadService(t, db, &appconf.AppConfig{})
	starter := &stubMCPGameStarter{
		started: true,
	}
	readService.SetStartService(starter)

	resp, err := readService.StartGame("game-1")
	if err != nil {
		t.Fatalf("StartGame failed: %v", err)
	}

	if starter.calls != 1 {
		t.Fatalf("expected start service to be called once, got %d", starter.calls)
	}
	if starter.lastGameID != "game-1" {
		t.Fatalf("expected game_id game-1, got %s", starter.lastGameID)
	}
	if !resp.Started {
		t.Fatal("expected started=true")
	}
	if resp.GameID != "game-1" || resp.Name != "Game 1" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestMCPReadServiceSearchMetadataByNameFiltersDisabledSources(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	config := &appconf.AppConfig{
		MetadataSources: []string{string(enums.Steam)},
	}
	readService := newTestMCPReadService(t, db, config)
	readService.SetMetadataFetcher(func(name string) ([]vo.GameMetadataFromWebVO, error) {
		return []vo.GameMetadataFromWebVO{
			{
				Source: enums.Bangumi,
				Game: models.Game{
					Name:     "Bangumi Result",
					SourceID: "bgm-1",
				},
			},
			{
				Source: enums.Steam,
				Game: models.Game{
					Name:     "Steam Result",
					SourceID: "steam-1",
				},
				Tags: []metadata.TagItem{
					{Name: "story-rich", Source: "steam", Weight: 0.8},
				},
			},
		}, nil
	})

	resp, err := readService.SearchMetadataByName("test", 10)
	if err != nil {
		t.Fatalf("SearchMetadataByName failed: %v", err)
	}

	if resp.SpoilerContext.GlobalLevel != "none" {
		t.Fatalf("expected spoiler level none, got %s", resp.SpoilerContext.GlobalLevel)
	}
	if resp.TotalResults != 1 {
		t.Fatalf("expected 1 enabled result, got %d", resp.TotalResults)
	}
	if len(resp.Results) != 1 || resp.Results[0].Source != string(enums.Steam) {
		t.Fatalf("unexpected metadata results: %#v", resp.Results)
	}
}

func TestMCPReadServiceGetGameStatisticUsesStatsProviderOnly(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	readService := newTestMCPReadService(t, db, &appconf.AppConfig{})
	provider := &stubAIStatsProvider{
		data: &service.AIStatsData{
			Dimension:         "week",
			StartDate:         "2026-04-26",
			EndDate:           "2026-05-02",
			DateRange:         "2026-04-26 至 2026-05-02",
			TotalPlayCount:    4,
			TotalPlayDuration: 7200,
			TopGames: []service.GamePlayInfo{
				{
					GameID:          "game-1",
					Name:            "Game 1",
					Company:         "Studio",
					Duration:        3600,
					Summary:         "Summary",
					Categories:      []string{"悬疑"},
					Status:          "playing",
					SpoilerBoundary: "chapter_end",
					ProgressNote:    "chapter 3",
					Route:           "route a",
				},
			},
			RecentSessions: []service.SessionInfo{
				{
					GameID:    "game-1",
					GameName:  "Game 1",
					StartTime: time.Date(2026, 5, 2, 22, 0, 0, 0, time.UTC),
					Duration:  1800,
					DayOfWeek: 5,
					Hour:      22,
				},
			},
		},
	}
	readService.SetStatsProvider(provider)

	resp, err := readService.GetGameStatistic(enums.Week)
	if err != nil {
		t.Fatalf("GetGameStatistic failed: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("expected stats provider to be called once, got %d", provider.calls)
	}
	if provider.lastPeriod != enums.Week {
		t.Fatalf("expected period week, got %s", provider.lastPeriod)
	}
	if resp.SpoilerContext.GlobalLevel != "none" {
		t.Fatalf("expected spoiler level none, got %s", resp.SpoilerContext.GlobalLevel)
	}
	if len(resp.TopGames) != 1 || resp.TopGames[0].GameID != "game-1" {
		t.Fatalf("unexpected top games: %#v", resp.TopGames)
	}
	if len(resp.RecentSessions) != 1 || resp.RecentSessions[0].GameID != "game-1" {
		t.Fatalf("unexpected recent sessions: %#v", resp.RecentSessions)
	}
}

type stubAIStatsProvider struct {
	calls      int
	lastPeriod enums.Period
	data       *service.AIStatsData
	err        error
}

type stubMCPGameStarter struct {
	calls      int
	lastGameID string
	started    bool
	err        error
}

func (s *stubAIStatsProvider) Build(period enums.Period) (*service.AIStatsData, error) {
	s.calls++
	s.lastPeriod = period
	return s.data, s.err
}

func (s *stubMCPGameStarter) StartGameWithTracking(gameID string) (bool, error) {
	s.calls++
	s.lastGameID = gameID
	return s.started, s.err
}

func newTestMCPReadService(t *testing.T, db *sql.DB, config *appconf.AppConfig) *service.MCPReadService {
	t.Helper()

	ctx := context.Background()
	gameService := service.NewGameService()
	progressService := service.NewGameProgressService()
	tagService := service.NewTagService()
	readService := service.NewMCPReadService()

	gameService.Init(ctx, db, config)
	progressService.Init(ctx, db, config)
	tagService.Init(ctx, db, config)
	readService.Init(ctx, db, config)
	readService.SetGameService(gameService)
	readService.SetGameProgressService(progressService)
	readService.SetTagService(tagService)

	return readService
}

func insertTestGameRecord(t *testing.T, db *sql.DB, id, name string, createdAt time.Time) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, cover_url, company, summary, rating, release_date,
			path, save_path, process_name, status, source_type,
			cached_at, source_id, created_at, updated_at, use_locale_emulator, use_magpie
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		id,
		name,
		"https://example.com/cover.jpg",
		"Studio",
		"Summary",
		8.6,
		"2026-01-01",
		`C:\Games\`+name+`\game.exe`,
		`C:\Saves\`+name,
		"game.exe",
		string(enums.StatusPlaying),
		string(enums.Local),
		createdAt,
		"local-"+id,
		createdAt,
		createdAt,
		false,
		false,
	); err != nil {
		t.Fatalf("insert test game %s failed: %v", id, err)
	}
}

func insertTestSessionRecord(t *testing.T, db *sql.DB, id, gameID string, startTime time.Time, duration int) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		id,
		gameID,
		startTime,
		startTime.Add(time.Duration(duration)*time.Second),
		duration,
		startTime.Add(time.Duration(duration)*time.Second),
	); err != nil {
		t.Fatalf("insert test session %s failed: %v", id, err)
	}
}
