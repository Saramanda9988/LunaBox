package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func openGameLibraryConfigTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE games (
			id TEXT PRIMARY KEY,
			name TEXT,
			path TEXT,
			game_directory TEXT,
			save_path TEXT,
			launch_mode TEXT,
			steam_launch_kind TEXT,
			updated_at TIMESTAMPTZ
		);
		CREATE TABLE download_tasks (
			id TEXT PRIMARY KEY,
			request_json TEXT,
			status TEXT,
			progress DOUBLE,
			downloaded BIGINT,
			total BIGINT,
			error TEXT,
			file_path TEXT,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		);
	`); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	return db
}

func TestPreviewAndApplyGameLibraryPathChange(t *testing.T) {
	db := openGameLibraryConfigTestDB(t)
	libraryParent := t.TempDir()
	oldLibrary := filepath.Join(libraryParent, "old-library")
	newLibrary := filepath.Join(libraryParent, "new-library")
	oldExecutable := filepath.Join(oldLibrary, "Game A", "bin", "game.exe")
	newExecutable := filepath.Join(newLibrary, "Game A", "bin", "game.exe")
	oldGameDirectory := filepath.Join(oldLibrary, "Game A")
	newGameDirectory := filepath.Join(newLibrary, "Game A")
	oldDownloadPath := filepath.Join(oldLibrary, "Game A.zip")
	newDownloadPath := filepath.Join(newLibrary, "Game A.zip")
	externalSavePath := filepath.Join(t.TempDir(), "saves", "Game A")

	if err := os.MkdirAll(filepath.Dir(newExecutable), 0o755); err != nil {
		t.Fatalf("create new game directory: %v", err)
	}
	if err := os.WriteFile(newExecutable, []byte("game"), 0o644); err != nil {
		t.Fatalf("create new executable: %v", err)
	}
	if err := os.WriteFile(newDownloadPath, []byte("archive"), 0o644); err != nil {
		t.Fatalf("create new download file: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO games (id, name, path, game_directory, save_path, launch_mode, steam_launch_kind)
		VALUES (?, ?, ?, ?, ?, 'normal', '')
	`, "game-1", "Game A", oldExecutable, oldGameDirectory, externalSavePath); err != nil {
		t.Fatalf("insert game: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO games (id, name, path, game_directory, save_path, launch_mode, steam_launch_kind)
		VALUES (?, ?, ?, ?, '', 'normal', '')
	`, "game-2", "Sibling", filepath.Join(oldLibrary+"-extra", "game.exe"), oldLibrary+"-extra"); err != nil {
		t.Fatalf("insert sibling game: %v", err)
	}
	requestJSON, _ := json.Marshal(vo.InstallRequest{Title: "Game A"})
	if _, err := db.Exec(`
		INSERT INTO download_tasks (id, request_json, status, progress, downloaded, total, error, file_path)
		VALUES (?, ?, 'done', 100, 10, 10, '', ?)
	`, "task-1", string(requestJSON), oldDownloadPath); err != nil {
		t.Fatalf("insert download task: %v", err)
	}

	config := &appconf.AppConfig{Theme: "light", Language: "zh-CN", GameLibraryPath: oldLibrary}
	downloadService := NewDownloadService()
	downloadService.Init(context.Background(), db, config)
	configService := NewConfigService()
	configService.Init(context.Background(), db, config)
	configService.SetDownloadService(downloadService)
	configService.SetConfigSaverForTest(func(*appconf.AppConfig) error { return nil })

	preview, err := configService.PreviewGameLibraryPathChange(newLibrary)
	if err != nil {
		t.Fatalf("preview library change: %v", err)
	}
	if preview.AffectedGameCount != 1 || preview.AffectedDownloadTaskCount != 1 {
		t.Fatalf("unexpected affected counts: games=%d tasks=%d", preview.AffectedGameCount, preview.AffectedDownloadTaskCount)
	}
	if len(preview.Changes) != 3 || preview.MissingTargetCount != 0 {
		t.Fatalf("unexpected change summary: changes=%d missing=%d", len(preview.Changes), preview.MissingTargetCount)
	}

	result, err := configService.ApplyGameLibraryPathChange(newLibrary, true)
	if err != nil {
		t.Fatalf("apply library change: %v", err)
	}
	if result.UpdatedGameCount != 1 || result.UpdatedDownloadTaskCount != 1 {
		t.Fatalf("unexpected updated counts: games=%d tasks=%d", result.UpdatedGameCount, result.UpdatedDownloadTaskCount)
	}
	if config.GameLibraryPath != newLibrary {
		t.Fatalf("config library path: got %q want %q", config.GameLibraryPath, newLibrary)
	}

	var gotPath, gotDirectory, gotSavePath string
	if err := db.QueryRow(`SELECT path, game_directory, save_path FROM games WHERE id = 'game-1'`).Scan(&gotPath, &gotDirectory, &gotSavePath); err != nil {
		t.Fatalf("query updated game: %v", err)
	}
	if gotPath != newExecutable || gotDirectory != newGameDirectory || gotSavePath != externalSavePath {
		t.Fatalf("unexpected updated game values: path=%q directory=%q save=%q", gotPath, gotDirectory, gotSavePath)
	}

	var gotTaskPath string
	if err := db.QueryRow(`SELECT file_path FROM download_tasks WHERE id = 'task-1'`).Scan(&gotTaskPath); err != nil {
		t.Fatalf("query updated task: %v", err)
	}
	if gotTaskPath != newDownloadPath {
		t.Fatalf("download task path: got %q want %q", gotTaskPath, newDownloadPath)
	}
	tasks := downloadService.GetDownloadTasks()
	if len(tasks) != 1 || tasks[0].FilePath != newDownloadPath {
		t.Fatalf("in-memory task was not updated: %+v", tasks)
	}
}

func TestApplyGameLibraryPathChangeRejectsPausedDownload(t *testing.T) {
	db := openGameLibraryConfigTestDB(t)
	oldLibrary := filepath.Join(t.TempDir(), "old-library")
	newLibrary := filepath.Join(t.TempDir(), "new-library")
	requestJSON, _ := json.Marshal(vo.InstallRequest{Title: "Paused Game"})
	if _, err := db.Exec(`
		INSERT INTO download_tasks (id, request_json, status, progress, downloaded, total, error, file_path)
		VALUES ('task-paused', ?, 'paused', 50, 5, 10, '', '')
	`, string(requestJSON)); err != nil {
		t.Fatalf("insert paused task: %v", err)
	}

	config := &appconf.AppConfig{Theme: "light", Language: "zh-CN", GameLibraryPath: oldLibrary}
	downloadService := NewDownloadService()
	downloadService.Init(context.Background(), db, config)
	configService := NewConfigService()
	configService.Init(context.Background(), db, config)
	configService.SetDownloadService(downloadService)
	configService.SetConfigSaverForTest(func(*appconf.AppConfig) error { return nil })

	preview, err := configService.PreviewGameLibraryPathChange(newLibrary)
	if err != nil {
		t.Fatalf("preview library change: %v", err)
	}
	if preview.BlockingDownloadTaskCount != 1 {
		t.Fatalf("blocking task count: got %d want 1", preview.BlockingDownloadTaskCount)
	}
	_, err = configService.ApplyGameLibraryPathChange(newLibrary, true)
	if err == nil || !strings.Contains(err.Error(), "下载任务") {
		t.Fatalf("expected paused task error, got %v", err)
	}
	if config.GameLibraryPath != oldLibrary {
		t.Fatalf("config changed despite blocker: %q", config.GameLibraryPath)
	}

	result, err := configService.ApplyGameLibraryPathChange(newLibrary, false)
	if err != nil {
		t.Fatalf("change library without syncing paths: %v", err)
	}
	if result.NewConfiguredPath != newLibrary {
		t.Fatalf("new configured path: got %q want %q", result.NewConfiguredPath, newLibrary)
	}
	if config.GameLibraryPath != newLibrary {
		t.Fatalf("config was not changed when path sync was skipped: %q", config.GameLibraryPath)
	}
	if configService.pendingGameLibrarySource != "" {
		t.Fatalf("empty game scan should not retain pending source: %q", configService.pendingGameLibrarySource)
	}
}

func TestRefreshFindsSkippedGameLibraryPathChanges(t *testing.T) {
	db := openGameLibraryConfigTestDB(t)
	libraryParent := t.TempDir()
	oldLibrary := filepath.Join(libraryParent, "old-library")
	newLibrary := filepath.Join(libraryParent, "new-library")
	relativeGameDirectory := filepath.Join("Publisher", "Game A")
	oldGameDirectory := filepath.Join(oldLibrary, relativeGameDirectory)
	newGameDirectory := filepath.Join(newLibrary, relativeGameDirectory)
	oldExecutable := filepath.Join(oldGameDirectory, "bin", "game.exe")
	newExecutable := filepath.Join(newGameDirectory, "bin", "game.exe")

	if _, err := db.Exec(`
		INSERT INTO games (id, name, path, game_directory, save_path, launch_mode, steam_launch_kind)
		VALUES ('game-skipped', 'Game A', ?, ?, '', 'normal', '')
	`, oldExecutable, oldGameDirectory); err != nil {
		t.Fatalf("insert game: %v", err)
	}

	config := &appconf.AppConfig{Theme: "light", Language: "zh-CN", GameLibraryPath: oldLibrary}
	configService := NewConfigService()
	configService.Init(context.Background(), db, config)
	configService.SetConfigSaverForTest(func(*appconf.AppConfig) error { return nil })

	if _, err := configService.ApplyGameLibraryPathChange(newLibrary, false); err != nil {
		t.Fatalf("skip path updates: %v", err)
	}

	refreshedConfigService := NewConfigService()
	refreshedConfigService.Init(context.Background(), db, config)
	refreshedConfigService.SetConfigSaverForTest(func(*appconf.AppConfig) error { return nil })
	preview, err := refreshedConfigService.PreviewGameLibraryPathChange(newLibrary)
	if err != nil {
		t.Fatalf("refresh skipped path updates: %v", err)
	}
	if preview.AffectedGameCount != 1 || len(preview.Changes) != 2 {
		t.Fatalf("unexpected refreshed preview: games=%d changes=%d", preview.AffectedGameCount, len(preview.Changes))
	}
	if preview.MissingTargetCount != 2 {
		t.Fatalf("missing target count: got %d want 2", preview.MissingTargetCount)
	}
	if !sameLibraryPath(preview.OldLibraryPath, oldLibrary) {
		t.Fatalf("inferred source library: got %q want %q", preview.OldLibraryPath, oldLibrary)
	}

	result, err := refreshedConfigService.ApplyGameLibraryPathChange(newLibrary, true)
	if err != nil {
		t.Fatalf("apply refreshed path updates: %v", err)
	}
	if result.UpdatedGameCount != 1 {
		t.Fatalf("updated game count: got %d want 1", result.UpdatedGameCount)
	}

	var gotPath, gotDirectory string
	if err := db.QueryRow(`SELECT path, game_directory FROM games WHERE id = 'game-skipped'`).Scan(&gotPath, &gotDirectory); err != nil {
		t.Fatalf("query refreshed game: %v", err)
	}
	if gotPath != newExecutable || gotDirectory != newGameDirectory {
		t.Fatalf("unexpected refreshed game values: path=%q directory=%q", gotPath, gotDirectory)
	}
}

func TestDiscoverGameSourceLibrariesPrefersDominantSibling(t *testing.T) {
	libraryParent := t.TempDir()
	oldLibrary := filepath.Join(libraryParent, "old-library")
	newLibrary := filepath.Join(libraryParent, "new-library")
	externalLibrary := filepath.Join(libraryParent, "external-library")
	records := []libraryGameRecord{
		{id: "game-1", gameDirectory: filepath.Join(oldLibrary, "Game A")},
		{id: "game-2", gameDirectory: filepath.Join(oldLibrary, "Game B")},
		{id: "external", gameDirectory: filepath.Join(externalLibrary, "Game C")},
	}

	discovered := discoverGameSourceLibraries(records, newLibrary)
	if len(discovered) != 2 {
		t.Fatalf("discovered games: got %d want 2", len(discovered))
	}
	for _, gameID := range []string{"game-1", "game-2"} {
		if !sameLibraryPath(discovered[gameID], oldLibrary) {
			t.Fatalf("source library for %s: got %q want %q", gameID, discovered[gameID], oldLibrary)
		}
	}
	if _, found := discovered["external"]; found {
		t.Fatal("external game should not be selected")
	}
}

func TestRebaseLibraryPathUsesDirectoryBoundary(t *testing.T) {
	oldLibrary := filepath.Join(t.TempDir(), "Games")
	newLibrary := filepath.Join(t.TempDir(), "NewGames")
	inside := filepath.Join(oldLibrary, "A", "game.exe")
	sibling := filepath.Join(oldLibrary+"2", "A", "game.exe")

	if got, ok := rebaseLibraryPath(inside, oldLibrary, newLibrary); !ok || got != filepath.Join(newLibrary, "A", "game.exe") {
		t.Fatalf("inside path rebase: got %q ok=%v", got, ok)
	}
	if got, ok := rebaseLibraryPath(sibling, oldLibrary, newLibrary); ok || got != sibling {
		t.Fatalf("sibling path should stay unchanged: got %q ok=%v", got, ok)
	}
}
