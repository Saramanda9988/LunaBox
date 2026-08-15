package service

import (
	"context"
	"database/sql"
	"errors"
	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/utils/downloadutils"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

func openDownloadServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := db.Exec(`CREATE TABLE download_tasks (
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
	)`); err != nil {
		t.Fatalf("create download_tasks table: %v", err)
	}

	return db
}

func TestDownloadServiceEmitGameImported(t *testing.T) {
	downloadService := NewDownloadService()
	downloadService.ctx = context.Background()

	var emittedName string
	var emittedPayload map[string]string
	downloadService.emitEvent = func(name string, data ...interface{}) {
		emittedName = name
		if len(data) != 1 {
			t.Fatalf("event payload count: got %d, want 1", len(data))
		}
		var ok bool
		emittedPayload, ok = data[0].(map[string]string)
		if !ok {
			t.Fatalf("event payload type: got %T", data[0])
		}
	}

	downloadService.emitGameImported("task-1")

	if emittedName != downloadGameImportedEvent {
		t.Fatalf("event name: got %q, want %q", emittedName, downloadGameImportedEvent)
	}
	if emittedPayload["task_id"] != "task-1" {
		t.Fatalf("task_id: got %q, want %q", emittedPayload["task_id"], "task-1")
	}
}

func TestImportWithPrefetchedMetadataDoesNotWaitForPendingResult(t *testing.T) {
	downloadService := NewDownloadService()
	downloadService.ctx = context.Background()
	downloadService.emitEvent = func(string, ...interface{}) {}
	task := &DownloadTask{ID: "task-1", Request: vo.InstallRequest{Title: "Test Game"}}
	results := make(chan downloadMetadataResult, 1)
	imports := make(chan *vo.GameMetadataFromWebVO, 2)

	if err := downloadService.importWithPrefetchedMetadata(
		task,
		results,
		func(metadata *vo.GameMetadataFromWebVO) error {
			imports <- metadata
			return nil
		},
	); err != nil {
		t.Fatalf("initial import: %v", err)
	}

	select {
	case metadata := <-imports:
		if metadata != nil {
			t.Fatalf("initial import should not wait for metadata, got %#v", metadata)
		}
	case <-time.After(time.Second):
		t.Fatal("initial import was blocked by pending metadata")
	}

	fetchedMetadata := &vo.GameMetadataFromWebVO{}
	results <- downloadMetadataResult{metadata: fetchedMetadata}
	close(results)

	select {
	case metadata := <-imports:
		if metadata != fetchedMetadata {
			t.Fatalf("metadata update mismatch: got %#v, want %#v", metadata, fetchedMetadata)
		}
	case <-time.After(time.Second):
		t.Fatal("prefetched metadata was not applied")
	}
}

func TestImportWithPrefetchedMetadataReportsReadyFetchFailure(t *testing.T) {
	downloadService := NewDownloadService()
	downloadService.ctx = context.Background()
	task := &DownloadTask{ID: "task-1", Request: vo.InstallRequest{Title: "Test Game"}}
	fetchErr := errors.New("metadata unavailable")
	results := make(chan downloadMetadataResult, 1)
	results <- downloadMetadataResult{err: fetchErr}
	close(results)

	var emittedName string
	downloadService.emitEvent = func(name string, _ ...interface{}) {
		emittedName = name
	}
	importCalls := 0
	if err := downloadService.importWithPrefetchedMetadata(
		task,
		results,
		func(metadata *vo.GameMetadataFromWebVO) error {
			importCalls++
			if metadata != nil {
				t.Fatalf("failed metadata fetch should fall back to base import")
			}
			return nil
		},
	); err != nil {
		t.Fatalf("base import: %v", err)
	}

	if importCalls != 1 {
		t.Fatalf("base import calls: got %d, want 1", importCalls)
	}
	if emittedName != downloadMetadataFailedEvent {
		t.Fatalf("event name: got %q, want %q", emittedName, downloadMetadataFailedEvent)
	}
}

func TestDownloadServiceLoadTasksPreservesFinishedTaskTimestamps(t *testing.T) {
	db := openDownloadServiceTestDB(t)
	historicalUpdatedAt := time.Date(2025, time.December, 8, 9, 30, 0, 0, time.UTC)

	if _, err := db.Exec(`
		INSERT INTO download_tasks (
			id, request_json, status, progress, downloaded, total, error, file_path, created_at, updated_at
		) VALUES (?, '{}', 'done', 100, 10, 10, '', '', NULL, ?)
	`, "legacy-task", historicalUpdatedAt); err != nil {
		t.Fatalf("insert legacy task: %v", err)
	}

	downloadService := NewDownloadService()
	downloadService.Init(context.Background(), db, &appconf.AppConfig{})

	var createdAt sql.NullTime
	var updatedAt time.Time
	if err := db.QueryRow(`
		SELECT created_at, updated_at
		FROM download_tasks
		WHERE id = 'legacy-task'
	`).Scan(&createdAt, &updatedAt); err != nil {
		t.Fatalf("query legacy task timestamps: %v", err)
	}

	if createdAt.Valid {
		t.Fatalf("legacy task creation time should remain unknown, got %s", createdAt.Time)
	}
	if !updatedAt.Equal(historicalUpdatedAt) {
		t.Fatalf("loading changed updated_at: got %s, want %s", updatedAt, historicalUpdatedAt)
	}

	tasks := downloadService.GetDownloadTasks()
	if len(tasks) != 1 || tasks[0].CreatedAt != nil {
		t.Fatalf("legacy task should not receive a fabricated creation time: %#v", tasks)
	}
}

func TestDownloadServiceUpsertTaskStoresInitialTimestamps(t *testing.T) {
	db := openDownloadServiceTestDB(t)
	createdAt := time.Date(2026, time.July, 14, 16, 20, 0, 0, time.UTC)
	downloadService := NewDownloadService()
	downloadService.db = db

	task := &DownloadTask{
		ID:        "new-task",
		Status:    DownloadStatusPending,
		CreatedAt: &createdAt,
	}
	if err := downloadService.upsertTask(task); err != nil {
		t.Fatalf("persist new task: %v", err)
	}

	var storedCreatedAt time.Time
	var storedUpdatedAt time.Time
	if err := db.QueryRow(`
		SELECT created_at, updated_at
		FROM download_tasks
		WHERE id = 'new-task'
	`).Scan(&storedCreatedAt, &storedUpdatedAt); err != nil {
		t.Fatalf("query new task timestamps: %v", err)
	}

	if !storedCreatedAt.Equal(createdAt) {
		t.Fatalf("created_at mismatch: got %s, want %s", storedCreatedAt, createdAt)
	}
	if !storedUpdatedAt.Equal(createdAt) {
		t.Fatalf("updated_at mismatch: got %s, want %s", storedUpdatedAt, createdAt)
	}
}

func TestDeleteFailedDownloadTaskPreservesExistingExtractDirectory(t *testing.T) {
	libraryDir := t.TempDir()
	request := vo.InstallRequest{
		FileName:      "existing-game.zip",
		ArchiveFormat: "zip",
		Title:         "Existing Game",
	}
	task := &DownloadTask{
		ID:      "failed-task",
		Request: request,
		Status:  DownloadStatusError,
	}
	downloadService := NewDownloadService()
	downloadService.config = &appconf.AppConfig{GameLibraryPath: libraryDir}
	downloadService.tasks[task.ID] = task

	destPath := filepath.Join(libraryDir, request.FileName)
	extractPath := downloadutils.BuildExpectedExtractDir(destPath, request.FileName, request.ArchiveFormat, request.Title)
	sentinelPath := filepath.Join(extractPath, "keep.txt")
	if err := os.MkdirAll(extractPath, 0755); err != nil {
		t.Fatalf("create existing extract directory: %v", err)
	}
	if err := os.WriteFile(sentinelPath, []byte("user data"), 0644); err != nil {
		t.Fatalf("write existing game sentinel: %v", err)
	}

	tempPath := downloadutils.TempDownloadPath(destPath)
	partsPath := downloadutils.MultipartTempDir(destPath)
	stagingPath := downloadTaskExtractStagingPath(extractPath, task.ID)
	if err := os.WriteFile(tempPath, []byte("partial"), 0644); err != nil {
		t.Fatalf("write partial download: %v", err)
	}
	if err := os.MkdirAll(partsPath, 0755); err != nil {
		t.Fatalf("create multipart temp dir: %v", err)
	}
	if err := os.MkdirAll(stagingPath, 0755); err != nil {
		t.Fatalf("create extract staging dir: %v", err)
	}

	if err := downloadService.DeleteDownloadTask(task.ID); err != nil {
		t.Fatalf("delete failed download task: %v", err)
	}

	if data, err := os.ReadFile(sentinelPath); err != nil || string(data) != "user data" {
		t.Fatalf("existing game directory was modified or removed: data=%q err=%v", data, err)
	}
	for _, cleanedPath := range []string{tempPath, partsPath, stagingPath} {
		if _, err := os.Stat(cleanedPath); !os.IsNotExist(err) {
			t.Fatalf("task-owned artifact was not cleaned: %s err=%v", cleanedPath, err)
		}
	}
}

func TestFinalizeDownloadExtractDirKeepsExistingDirectory(t *testing.T) {
	root := t.TempDir()
	preferredPath := filepath.Join(root, "game")
	stagingPath := filepath.Join(root, "game.lunabox.extracting.task")
	if err := os.MkdirAll(preferredPath, 0755); err != nil {
		t.Fatalf("create existing destination: %v", err)
	}
	existingSentinel := filepath.Join(preferredPath, "existing.txt")
	if err := os.WriteFile(existingSentinel, []byte("existing"), 0644); err != nil {
		t.Fatalf("write existing sentinel: %v", err)
	}
	if err := os.MkdirAll(stagingPath, 0755); err != nil {
		t.Fatalf("create staging dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagingPath, "new.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("write staging sentinel: %v", err)
	}

	finalPath, err := finalizeDownloadExtractDir(stagingPath, preferredPath)
	if err != nil {
		t.Fatalf("finalize extract dir: %v", err)
	}
	wantFinalPath := preferredPath + " (2)"
	if finalPath != wantFinalPath {
		t.Fatalf("final path mismatch: got %q, want %q", finalPath, wantFinalPath)
	}
	if data, err := os.ReadFile(existingSentinel); err != nil || string(data) != "existing" {
		t.Fatalf("existing destination was modified: data=%q err=%v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(finalPath, "new.txt")); err != nil || string(data) != "new" {
		t.Fatalf("staged extraction was not finalized: data=%q err=%v", data, err)
	}
}
