package service

import (
	"context"
	"database/sql"
	"errors"
	"lunabox/internal/appconf"
	"os"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

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
