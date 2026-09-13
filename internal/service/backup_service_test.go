package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/vo"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/utils/archiveutils"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

type retentionTestProvider struct {
	keys    []string
	deleted []string
}

func (p *retentionTestProvider) UploadFile(context.Context, string, string) error { return nil }
func (p *retentionTestProvider) DownloadFile(context.Context, string, string) error {
	return nil
}
func (p *retentionTestProvider) ListObjects(_ context.Context, prefix string) ([]string, error) {
	items := make([]string, 0, len(p.keys))
	for _, key := range p.keys {
		if strings.HasPrefix(key, prefix) {
			items = append(items, key)
		}
	}
	return items, nil
}
func (p *retentionTestProvider) DeleteObject(_ context.Context, key string) error {
	p.deleted = append(p.deleted, key)
	return nil
}
func (p *retentionTestProvider) TestConnection(context.Context) error { return nil }
func (p *retentionTestProvider) EnsureDir(context.Context, string) error {
	return nil
}
func (p *retentionTestProvider) GetCloudPath(userID, subPath string) string {
	return filepath.ToSlash(filepath.Join("v1", userID, subPath))
}

var _ cloudprovider.CloudStorageProvider = (*retentionTestProvider)(nil)

func TestNextScheduledDBBackup(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 9, 8, 10, 30, 0, 0, location)

	t.Run("interval waits from last backup", func(t *testing.T) {
		config := &appconf.AppConfig{
			ScheduledDBBackupMode:            appconf.ScheduledDBBackupModeInterval,
			ScheduledDBBackupIntervalMinutes: 60,
		}
		got := nextScheduledDBBackup(now, config, now.Add(-20*time.Minute))
		want := now.Add(40 * time.Minute)
		if !got.Equal(want) {
			t.Fatalf("next backup = %v, want %v", got, want)
		}
	})

	t.Run("missed daily time runs now", func(t *testing.T) {
		config := &appconf.AppConfig{
			ScheduledDBBackupMode: appconf.ScheduledDBBackupModeDaily,
			ScheduledDBBackupTime: "09:00",
		}
		got := nextScheduledDBBackup(now, config, now.AddDate(0, 0, -1))
		if !got.Equal(now) {
			t.Fatalf("next backup = %v, want %v", got, now)
		}
	})

	t.Run("completed daily backup waits until tomorrow", func(t *testing.T) {
		config := &appconf.AppConfig{
			ScheduledDBBackupMode: appconf.ScheduledDBBackupModeDaily,
			ScheduledDBBackupTime: "09:00",
		}
		got := nextScheduledDBBackup(now, config, now.Add(-time.Hour))
		want := time.Date(2026, 9, 9, 9, 0, 0, 0, location)
		if !got.Equal(want) {
			t.Fatalf("next backup = %v, want %v", got, want)
		}
	})
}

func TestShouldRestoreCloudGameBackup(t *testing.T) {
	localTime := time.Date(2026, 9, 8, 10, 0, 0, 0, time.Local)
	latest := vo.CloudBackupItem{
		Key:       "v1/user/saves/game/2026-09-08T11-00-00.zip",
		CreatedAt: localTime.Add(time.Hour),
	}

	if !shouldRestoreCloudGameBackup(latest, latest.CreatedAt, localTime, "") {
		t.Fatal("expected a newer cloud backup to be restored")
	}
	if shouldRestoreCloudGameBackup(latest, latest.CreatedAt, latest.CreatedAt.Add(time.Minute), "") {
		t.Fatal("expected a newer local save to be kept")
	}
	if shouldRestoreCloudGameBackup(latest, latest.CreatedAt, time.Time{}, latest.Key) {
		t.Fatal("expected the last synchronized cloud backup to be skipped")
	}
}

func TestCloudGameBackupSourceTime(t *testing.T) {
	createdAt := time.Date(2026, 9, 8, 13, 0, 0, 0, time.Local)
	sourceModifiedAt := createdAt.Add(-2 * time.Hour)
	fileName := cloudGameBackupFileName(createdAt, sourceModifiedAt)

	gotCreatedAt, gotSourceModifiedAt, hasSourceTime, ok := cloudBackupTimesFromName(fileName)
	if !ok || !hasSourceTime {
		t.Fatalf("cloud backup timestamps were not parsed: %q", fileName)
	}
	if !gotCreatedAt.Equal(createdAt) || !gotSourceModifiedAt.Equal(sourceModifiedAt) {
		t.Fatalf("parsed times = (%v, %v), want (%v, %v)", gotCreatedAt, gotSourceModifiedAt, createdAt, sourceModifiedAt)
	}

	legacyCreatedAt, legacySourceModifiedAt, hasSourceTime, ok := cloudBackupTimesFromName("2026-09-08T13-00-00.zip")
	if !ok || hasSourceTime || !legacyCreatedAt.Equal(createdAt) || !legacySourceModifiedAt.Equal(createdAt) {
		t.Fatalf("legacy backup timestamps = (%v, %v, %t, %t)", legacyCreatedAt, legacySourceModifiedAt, hasSourceTime, ok)
	}
}

func TestNewestCloudGameBackupUsesSourceTime(t *testing.T) {
	base := time.Date(2026, 9, 8, 10, 0, 0, 0, time.Local)
	newerProgress := vo.CloudBackupItem{
		Key:       "newer-progress",
		Name:      cloudGameBackupFileName(base.Add(time.Hour), base.Add(30*time.Minute)),
		CreatedAt: base.Add(time.Hour),
	}
	staleReupload := vo.CloudBackupItem{
		Key:       "stale-reupload",
		Name:      cloudGameBackupFileName(base.Add(2*time.Hour), base),
		CreatedAt: base.Add(2 * time.Hour),
	}

	got, gotSourceModifiedAt := newestCloudGameBackupBySourceTime([]vo.CloudBackupItem{staleReupload, newerProgress})
	if got.Key != newerProgress.Key {
		t.Fatalf("selected backup = %q, want %q", got.Key, newerProgress.Key)
	}
	if !gotSourceModifiedAt.Equal(base.Add(30 * time.Minute)) {
		t.Fatalf("selected source time = %v, want %v", gotSourceModifiedAt, base.Add(30*time.Minute))
	}
}

func TestLatestGameLaunchTime(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE play_sessions (game_id TEXT, start_time TIMESTAMPTZ)`); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 8, 10, 0, 0, 0, time.Local)
	if _, err := db.Exec(`INSERT INTO play_sessions (game_id, start_time) VALUES (?, ?), (?, ?), (?, ?)`, "game-1", base, "game-1", base.Add(time.Hour), "game-2", base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	backupService := NewBackupService()
	backupService.Init(context.Background(), db, &appconf.AppConfig{})
	got, err := backupService.latestGameLaunchTime("game-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(base.Add(time.Hour)) {
		t.Fatalf("latest launch time = %v, want %v", got, base.Add(time.Hour))
	}

	missing, err := backupService.latestGameLaunchTime("missing-game")
	if err != nil {
		t.Fatal(err)
	}
	if !missing.IsZero() {
		t.Fatalf("missing game launch time = %v, want zero time", missing)
	}
}

func TestRestoreArchivePreservesSourceModificationTime(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source", "save.dat")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("save"), 0644); err != nil {
		t.Fatal(err)
	}
	modifiedAt := time.Date(2026, 9, 8, 10, 0, 0, 0, time.Local)
	if err := os.Chtimes(sourcePath, modifiedAt, modifiedAt); err != nil {
		t.Fatal(err)
	}

	backupPath := filepath.Join(tempDir, "save.zip")
	if _, err := archiveutils.ZipFileOrDirectory(filepath.Dir(sourcePath), backupPath); err != nil {
		t.Fatal(err)
	}
	backupSourceModifiedAt, err := latestZipEntryModTime(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !backupSourceModifiedAt.Equal(modifiedAt) {
		t.Fatalf("backup source modification time = %v, want %v", backupSourceModifiedAt, modifiedAt)
	}
	restoredDir := filepath.Join(tempDir, "restored")
	if err := archiveutils.UnzipForRestore(backupPath, restoredDir); err != nil {
		t.Fatal(err)
	}

	restoredPath := filepath.Join(restoredDir, filepath.Base(sourcePath))
	restoredInfo, err := os.Stat(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if !restoredInfo.ModTime().Equal(modifiedAt) {
		t.Fatalf("restored modification time = %v, want %v", restoredInfo.ModTime(), modifiedAt)
	}
}

func TestCleanupOldCloudDBBackupsUsesDatabaseRetention(t *testing.T) {
	provider := &retentionTestProvider{}
	for day := 1; day <= 4; day++ {
		provider.keys = append(provider.keys, fmt.Sprintf("v1/user/database/lunabox_2026-09-0%dT03-00-00.zip", day))
	}
	provider.keys = append(provider.keys, "v1/user/database/latest.zip")

	backupService := NewBackupService()
	backupService.Init(context.Background(), nil, &appconf.AppConfig{
		BackupUserID:           "user",
		CloudDBBackupRetention: 2,
		CloudBackupRetention:   4,
	})
	backupService.SetCloudProviderFactoryForTest(func() (cloudprovider.CloudStorageProvider, error) {
		return provider, nil
	})

	backupService.cleanupOldCloudDBBackups()
	if len(provider.deleted) != 2 {
		t.Fatalf("deleted %d backups, want 2", len(provider.deleted))
	}
	if !strings.Contains(provider.deleted[0], "2026-09-02") || !strings.Contains(provider.deleted[1], "2026-09-01") {
		t.Fatalf("unexpected deleted backups: %v", provider.deleted)
	}
}

func TestRemoveDBBackupFilesRemovesExpiredBackups(t *testing.T) {
	tempDir := t.TempDir()
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = filepath.Join(tempDir, fmt.Sprintf("backup-%d.zip", i))
		if err := os.WriteFile(paths[i], []byte("backup"), 0644); err != nil {
			t.Fatalf("create backup %d: %v", i, err)
		}
	}

	err := removeDBBackupFiles([]vo.DBBackupInfo{
		{Path: paths[1], Name: "backup-1.zip"},
		{Path: paths[2], Name: "backup-2.zip"},
	})
	if err != nil {
		t.Fatalf("removeDBBackupFiles() error = %v", err)
	}

	if _, err := os.Stat(paths[0]); err != nil {
		t.Fatalf("newer backup should remain: %v", err)
	}
	for _, path := range paths[1:] {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired backup %s still exists or cannot be checked: %v", path, err)
		}
	}
}

func TestCreateDBBackupForShutdownUsesIndependentContext(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE backup_test (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create test table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO backup_test VALUES (1, 'LunaBox')`); err != nil {
		t.Fatalf("insert test row: %v", err)
	}

	appCtx, cancel := context.WithCancel(context.Background())
	cancel()

	backupService := NewBackupService()
	backupService.Init(appCtx, db, &appconf.AppConfig{LocalDBBackupRetention: 5})

	if _, err := backupService.CreateDBBackup(); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateDBBackup() error = %v, want context.Canceled", err)
	}

	backup, err := backupService.CreateDBBackupForShutdown()
	if err != nil {
		t.Fatalf("CreateDBBackupForShutdown() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(backup.Path) })

	info, err := os.Stat(backup.Path)
	if err != nil {
		t.Fatalf("stat shutdown backup: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("shutdown backup is empty")
	}
}
