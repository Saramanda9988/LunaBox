package service

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestPersistSteamIdentitiesUpdatesBatchInOneStatement(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open DuckDB: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE games (
			id TEXT PRIMARY KEY,
			steam_launch_id TEXT,
			steam_launch_kind TEXT,
			steam_user_id TEXT,
			steam_launch_options TEXT,
			wine_prefix TEXT,
			launch_mode TEXT
		);
		INSERT INTO games VALUES
			('game-1', '', '', '', '', '', 'normal'),
			('game-2', '', '', '', '', '/custom/prefix', 'normal');
	`); err != nil {
		t.Fatalf("prepare games: %v", err)
	}

	integrationService := NewIntegrationService()
	integrationService.Init(context.Background(), db, nil)
	err = integrationService.persistSteamIdentities([]SteamBatchImportItemResult{
		{
			GameID: "game-1",
			Status: SteamLaunchStatus{
				LaunchID:     "111",
				LaunchKind:   "native",
				ProtonPrefix: "/steam/compatdata/111/pfx",
			},
		},
		{
			GameID: "game-2",
			Status: SteamLaunchStatus{
				LaunchID:     "222",
				LaunchKind:   "shortcut",
				UserID:       "333",
				ProtonPrefix: "/steam/compatdata/222/pfx",
			},
		},
	})
	if err != nil {
		t.Fatalf("persist Steam identities: %v", err)
	}

	tests := []struct {
		gameID     string
		launchID   string
		launchKind string
		userID     string
		winePrefix string
		launchMode string
	}{
		{gameID: "game-1", launchID: "111", launchKind: "native", winePrefix: "/steam/compatdata/111/pfx", launchMode: "steam"},
		{gameID: "game-2", launchID: "222", launchKind: "shortcut", userID: "333", winePrefix: "/custom/prefix", launchMode: "steam"},
	}
	for _, test := range tests {
		var launchID, launchKind, userID, winePrefix, launchMode string
		err := db.QueryRow(`
			SELECT steam_launch_id, steam_launch_kind, steam_user_id, wine_prefix, launch_mode
			FROM games
			WHERE id = ?
		`, test.gameID).Scan(&launchID, &launchKind, &userID, &winePrefix, &launchMode)
		if err != nil {
			t.Fatalf("query Steam identity for %s: %v", test.gameID, err)
		}
		if launchID != test.launchID ||
			launchKind != test.launchKind ||
			userID != test.userID ||
			winePrefix != test.winePrefix ||
			launchMode != test.launchMode {
			t.Fatalf(
				"unexpected Steam identity for %s: got %q %q %q %q %q, want %q %q %q %q %q",
				test.gameID,
				launchID,
				launchKind,
				userID,
				winePrefix,
				launchMode,
				test.launchID,
				test.launchKind,
				test.userID,
				test.winePrefix,
				test.launchMode,
			)
		}
	}
}

func TestPersistSteamLaunchOptionsSanitizesValue(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open DuckDB: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE games (
			id TEXT PRIMARY KEY,
			steam_launch_options TEXT
		);
		INSERT INTO games VALUES ('game-1', '');
	`); err != nil {
		t.Fatalf("prepare games: %v", err)
	}

	integrationService := NewIntegrationService()
	integrationService.Init(context.Background(), db, nil)
	if err := integrationService.persistSteamLaunchOptions("game-1", "  LANG=zh_CN.UTF-8 %command%\x00  "); err != nil {
		t.Fatalf("persist Steam launch options: %v", err)
	}

	var launchOptions string
	if err := db.QueryRow(`
		SELECT steam_launch_options
		FROM games
		WHERE id = 'game-1'
	`).Scan(&launchOptions); err != nil {
		t.Fatalf("query Steam launch options: %v", err)
	}
	if launchOptions != "LANG=zh_CN.UTF-8 %command%" {
		t.Fatalf("unexpected launch options: %q", launchOptions)
	}
}

func TestNormalizeSteamBatchGameIDsTrimsAndDeduplicates(t *testing.T) {
	got := normalizeSteamBatchGameIDs([]string{" game-1 ", "", "game-2", "game-1"})
	if len(got) != 2 || got[0] != "game-1" || got[1] != "game-2" {
		t.Fatalf("unexpected normalized game IDs: %#v", got)
	}
}
