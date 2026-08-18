package test

import (
	"context"
	"encoding/json"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/service"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestGameReviewServiceSaveAndSync(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const gameID = "review-game"
	insertBangumiGame(t, db, gameID, enums.StatusCompleted, enums.Bangumi, "42")
	if _, err := db.Exec(`
		INSERT INTO game_metadata_sources (game_id, source_type, source_id, cached_at, created_at, updated_at)
		VALUES
			(?, 'bangumi', '42', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
			(?, 'hikarinagi', '84', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, gameID, gameID); err != nil {
		t.Fatalf("insert review metadata sources: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		VALUES
			('review-session-1', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 3600, CURRENT_TIMESTAMP),
			('review-session-2', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 125, CURRENT_TIMESTAMP)
	`, gameID, gameID); err != nil {
		t.Fatalf("insert review play sessions: %v", err)
	}

	requestBodies := make(map[string]map[string]any)
	var requestMu sync.Mutex
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read review request: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode review request: %v", err)
		}
		requestMu.Lock()
		requestBodies[r.URL.Path] = payload
		requestMu.Unlock()

		switch r.URL.Path {
		case "/v0/users/-/collections/42":
			w.WriteHeader(http.StatusNoContent)
		case "/v3/user/me/rates/galgames/84":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":true,"data":{}}`)
		default:
			t.Fatalf("unexpected review endpoint: %s", r.URL.Path)
		}
	}))
	defer testServer.Close()

	config := &appconf.AppConfig{
		BangumiAccessToken:       "bangumi-token",
		BangumiTokenExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
		HikarinagiAccessToken:    "hikarinagi-token",
		HikarinagiTokenExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	bangumiSvc := service.NewBangumiService()
	bangumiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	bangumiSvc.Init(context.Background(), db, config)
	hikarinagiSvc := service.NewHikarinagiService()
	hikarinagiSvc.SetHTTPClient(newBangumiHTTPClient(t, testServer.URL))
	hikarinagiSvc.Init(context.Background(), db, config)

	reviewSvc := service.NewGameReviewService()
	reviewSvc.Init(context.Background(), db, config)
	reviewSvc.SetBangumiService(bangumiSvc)
	reviewSvc.SetHikarinagiService(hikarinagiSvc)

	rating := 9
	saved, err := reviewSvc.SaveGameReview(models.GameReview{
		GameID:    gameID,
		Rating:    &rating,
		Content:   "值得体验",
		IsSpoiler: true,
	})
	if err != nil {
		t.Fatalf("save game review: %v", err)
	}
	if saved == nil || saved.Rating == nil || *saved.Rating != 9 || saved.Content != "值得体验" || !saved.IsSpoiler {
		t.Fatalf("unexpected saved review: %+v", saved)
	}

	result, err := reviewSvc.SyncGameReview(gameID, []enums.SourceType{enums.Bangumi, enums.Hikarinagi})
	if err != nil {
		t.Fatalf("sync game review: %v", err)
	}
	if result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("unexpected review sync result: %+v", result)
	}

	requestMu.Lock()
	defer requestMu.Unlock()
	bangumiBody := requestBodies["/v0/users/-/collections/42"]
	if bangumiBody["rate"] != float64(9) || bangumiBody["comment"] != "值得体验" {
		t.Fatalf("unexpected Bangumi review payload: %+v", bangumiBody)
	}
	hikarinagiBody := requestBodies["/v3/user/me/rates/galgames/84"]
	if hikarinagiBody["rate"] != float64(9) || hikarinagiBody["rate_content"] != "值得体验" || hikarinagiBody["is_spoiler"] != true || hikarinagiBody["time_to_finish_minutes"] != float64(62) {
		t.Fatalf("unexpected Hikarinagi review payload: %+v", hikarinagiBody)
	}
}

func TestGameReviewServiceRejectsInvalidRating(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	insertBangumiGame(t, db, "invalid-review", enums.StatusNotStarted, enums.Local, "")

	svc := service.NewGameReviewService()
	svc.Init(context.Background(), db, &appconf.AppConfig{})
	rating := 11
	if _, err := svc.SaveGameReview(models.GameReview{GameID: "invalid-review", Rating: &rating}); err == nil {
		t.Fatal("expected invalid rating error")
	}
}
