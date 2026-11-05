package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lunabox/backend/utils/archiveutils"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should not exist (err = %v)", path, err)
	}
}

// newRestoreTestDirs 准备现有数据：数据库与它的 .wal、covers/old.png、设置文件。
func newRestoreTestDirs(t *testing.T) restoreDirs {
	t.Helper()
	root := t.TempDir()
	dirs := restoreDirs{dataDir: filepath.Join(root, "data"), configDir: filepath.Join(root, "config")}
	writeTestFile(t, filepath.Join(dirs.dataDir, "lunabox.db"), "old-db")
	writeTestFile(t, filepath.Join(dirs.dataDir, "lunabox.db.wal"), "old-wal")
	writeTestFile(t, filepath.Join(dirs.dataDir, "covers", "old.png"), "old-cover")
	writeTestFile(t, filepath.Join(dirs.configDir, "appconf.json"), `{"theme":"old"}`)
	return dirs
}

func assertOriginalData(t *testing.T, dirs restoreDirs) {
	t.Helper()
	assertFile(t, filepath.Join(dirs.dataDir, "lunabox.db"), "old-db")
	assertFile(t, filepath.Join(dirs.dataDir, "lunabox.db.wal"), "old-wal")
	assertFile(t, filepath.Join(dirs.dataDir, "covers", "old.png"), "old-cover")
	assertMissing(t, filepath.Join(dirs.dataDir, "covers", "new.png"))
	assertMissing(t, filepath.Join(dirs.dataDir, "backgrounds"))
	assertFile(t, filepath.Join(dirs.configDir, "appconf.json"), `{"theme":"old"}`)
}

// stageNewData 在暂存目录里准备新数据，返回全部替换项。
func stageNewData(t *testing.T, dirs restoreDirs) []restoreSwapItem {
	t.Helper()
	staging, err := dirs.prepareStaging()
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staging, "lunabox.db"), "new-db")
	writeTestFile(t, filepath.Join(staging, "covers", "new.png"), "new-cover")
	writeTestFile(t, filepath.Join(staging, "backgrounds", "bg.png"), "new-bg")
	writeTestFile(t, filepath.Join(staging, "appconf.json"), `{"theme":"new"}`)
	items := dirs.databaseItems(filepath.Join(staging, "lunabox.db"))
	items = append(items,
		dirs.dataItem("covers", filepath.Join(staging, "covers")),
		dirs.dataItem("backgrounds", filepath.Join(staging, "backgrounds")))
	cfg, err := dirs.configItem(filepath.Join(staging, "appconf.json"))
	if err != nil {
		t.Fatal(err)
	}
	return append(items, cfg)
}

func TestCommitRestoreSwapReplacesEverything(t *testing.T) {
	dirs := newRestoreTestDirs(t)
	if err := commitRestoreSwap(dirs.journal(), stageNewData(t, dirs)); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dirs.dataDir, "lunabox.db"), "new-db")
	assertMissing(t, filepath.Join(dirs.dataDir, "lunabox.db.wal"))
	assertFile(t, filepath.Join(dirs.dataDir, "covers", "new.png"), "new-cover")
	assertMissing(t, filepath.Join(dirs.dataDir, "covers", "old.png"))
	assertFile(t, filepath.Join(dirs.dataDir, "backgrounds", "bg.png"), "new-bg")
	assertFile(t, filepath.Join(dirs.configDir, "appconf.json"), `{"theme":"new"}`)
	assertMissing(t, dirs.journal())
	assertMissing(t, filepath.Join(dirs.rollback(), "lunabox.db"))
	assertMissing(t, filepath.Join(dirs.configDir, restoreConfigBackupName))
}

func TestCommitRestoreSwapRollsBackWhenAStepFails(t *testing.T) {
	dirs := newRestoreTestDirs(t)
	items := stageNewData(t, dirs)
	// 最后一项的暂存文件不存在：前面的都已换上去，这一项失败后要全部换回去。
	items = append(items, dirs.dataItem("backups", filepath.Join(dirs.staging(), "missing")))
	err := commitRestoreSwap(dirs.journal(), items)
	if err == nil {
		t.Fatal("commit succeeded with a missing staged item")
	}
	var rb *restoreRollbackError
	if errors.As(err, &rb) {
		t.Fatalf("rollback failed: %v", err)
	}
	assertOriginalData(t, dirs)
	assertMissing(t, dirs.journal())
}

func TestRecoverInterruptedRestoreRollsBackFromJournal(t *testing.T) {
	dirs := newRestoreTestDirs(t)
	items := stageNewData(t, dirs)
	// 模拟替换到一半进程退出：日志已落盘，数据库与 .wal 已换掉，covers 只移走了现有的。
	for i := range items {
		_, err := os.Lstat(items[i].Live)
		items[i].Existed = err == nil
		if err := os.MkdirAll(filepath.Dir(items[i].Backup), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(items)
	writeTestFile(t, dirs.journal(), string(data))
	for _, item := range items[:2] {
		if err := os.Rename(item.Live, item.Backup); err != nil {
			t.Fatal(err)
		}
		if item.Staged != "" {
			if err := os.Rename(item.Staged, item.Live); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Rename(items[2].Live, items[2].Backup); err != nil {
		t.Fatal(err)
	}

	rolledBack, err := recoverInterruptedRestore(dirs)
	if err != nil || !rolledBack {
		t.Fatalf("recover = %v, %v", rolledBack, err)
	}
	assertOriginalData(t, dirs)
	assertMissing(t, dirs.journal())
	assertMissing(t, dirs.staging())
	assertMissing(t, dirs.rollback())
	assertMissing(t, filepath.Join(dirs.configDir, restoreStagedConfigName))

	// 没有日志时什么也不做。
	if rolledBack, err := recoverInterruptedRestore(dirs); err != nil || rolledBack {
		t.Fatalf("second recover = %v, %v", rolledBack, err)
	}
}

func zipTestBackup(t *testing.T, files map[string]string) string {
	t.Helper()
	src := t.TempDir()
	for name, content := range files {
		writeTestFile(t, filepath.Join(src, filepath.FromSlash(name)), content)
	}
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := archiveutils.ZipDirectory(src, archive); err != nil {
		t.Fatal(err)
	}
	return archive
}

func TestExecuteDBRestoreImportsIntoStagingThenSwaps(t *testing.T) {
	dirs := newRestoreTestDirs(t)
	// 新格式的数据库备份：database/ 是 EXPORT DATABASE 的导出，covers/ 是封面。
	src := t.TempDir()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE games (name VARCHAR); INSERT INTO games VALUES ('restored')"); err != nil {
		t.Fatal(err)
	}
	if err := exportDuckDBDatabaseSnapshot(context.Background(), db, filepath.Join(src, "database")); err != nil {
		t.Fatal(err)
	}
	db.Close()
	writeTestFile(t, filepath.Join(src, "covers", "new.png"), "new-cover")
	backup := filepath.Join(t.TempDir(), "db.zip")
	if _, err := archiveutils.ZipDirectory(src, backup); err != nil {
		t.Fatal(err)
	}

	if err := executeDBRestore(dirs, backup, nil); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(dirs.dataDir, "lunabox.db.wal"))
	assertFile(t, filepath.Join(dirs.dataDir, "covers", "new.png"), "new-cover")
	assertFile(t, filepath.Join(dirs.configDir, "appconf.json"), `{"theme":"old"}`)
	assertMissing(t, dirs.staging())

	restored, err := sql.Open("duckdb", filepath.Join(dirs.dataDir, "lunabox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var name string
	if err := restored.QueryRow("SELECT name FROM games").Scan(&name); err != nil || name != "restored" {
		t.Fatalf("restored database: name = %q, err = %v", name, err)
	}
}

func TestExecuteFullDataRestoreIsAtomic(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dirs := newRestoreTestDirs(t)
		backup := zipTestBackup(t, map[string]string{
			"lunabox.db":         "new-db",
			"covers/new.png":     "new-cover",
			"backgrounds/bg.png": "new-bg",
			"appconf.json":       `{"theme":"new"}`,
		})
		var stages []RestoreStage
		err := executeFullDataRestore(dirs, backup, func(p RestoreProgress) {
			if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
				stages = append(stages, p.Stage)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		assertFile(t, filepath.Join(dirs.dataDir, "lunabox.db"), "new-db")
		assertMissing(t, filepath.Join(dirs.dataDir, "lunabox.db.wal"))
		assertFile(t, filepath.Join(dirs.dataDir, "covers", "new.png"), "new-cover")
		assertMissing(t, filepath.Join(dirs.dataDir, "covers", "old.png"))
		assertFile(t, filepath.Join(dirs.configDir, "appconf.json"), `{"theme":"new"}`)
		assertMissing(t, dirs.staging())
		assertMissing(t, filepath.Join(dirs.configDir, restoreStagedConfigName))
		want := []RestoreStage{RestoreStageExtract, RestoreStageDatabase, RestoreStageApply}
		if len(stages) != len(want) || stages[0] != want[0] || stages[1] != want[1] || stages[2] != want[2] {
			t.Fatalf("stages = %v, want %v", stages, want)
		}
	})

	t.Run("missing database keeps data", func(t *testing.T) {
		dirs := newRestoreTestDirs(t)
		backup := zipTestBackup(t, map[string]string{"covers/new.png": "new-cover", "appconf.json": `{"theme":"new"}`})
		if err := executeFullDataRestore(dirs, backup, nil); err == nil {
			t.Fatal("restore without database succeeded")
		}
		assertOriginalData(t, dirs)
		assertMissing(t, dirs.staging())
	})

	t.Run("locked file keeps data", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("only Windows refuses to rename a directory with an open file")
		}
		dirs := newRestoreTestDirs(t)
		locked, err := os.Open(filepath.Join(dirs.dataDir, "covers", "old.png"))
		if err != nil {
			t.Fatal(err)
		}
		backup := zipTestBackup(t, map[string]string{"lunabox.db": "new-db", "covers/new.png": "new-cover"})
		err = executeFullDataRestore(dirs, backup, nil)
		locked.Close()
		if err == nil {
			t.Fatal("restore succeeded while covers was locked")
		}
		assertOriginalData(t, dirs)
		assertMissing(t, dirs.journal())
	})
}
