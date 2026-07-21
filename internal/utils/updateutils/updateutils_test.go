package updateutils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestNormalizeManagedPath(t *testing.T) {
	t.Parallel()

	valid, err := NormalizeManagedPath(`7z\7z.dll`)
	if err != nil {
		t.Fatalf("expected managed path to be valid: %v", err)
	}
	if valid != "7z/7z.dll" {
		t.Fatalf("unexpected normalized path: %s", valid)
	}

	for _, value := range []string{"../LunaBox.exe", "/LunaBox.exe", `C:\LunaBox.exe`, "data/lunabox.db", "7z/../../lunabox.db"} {
		if _, err := NormalizeManagedPath(value); err == nil {
			t.Errorf("expected path %q to be rejected", value)
		}
	}
}

func TestPrepareZstdPatchAndFullFallback(t *testing.T) {
	t.Parallel()

	oldBytes := bytes.Repeat([]byte("old LunaBox executable block\n"), 4096)
	newBytes := append([]byte("new header\n"), oldBytes...)
	newBytes = append(newBytes, []byte("new trailer\n")...)

	appDir := t.TempDir()
	workDir := t.TempDir()
	sourcePath := filepath.Join(appDir, "duckdb.dll")
	if err := os.WriteFile(sourcePath, oldBytes, 0755); err != nil {
		t.Fatal(err)
	}

	patchPath := filepath.Join(workDir, "LunaBox.exe.zsdiff")
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderDictRaw(0, oldBytes), zstd.WithEncoderCRC(true))
	if err != nil {
		t.Fatal(err)
	}
	patchBytes := encoder.EncodeAll(newBytes, nil)
	encoder.Close()
	if err := os.WriteFile(patchPath, patchBytes, 0600); err != nil {
		t.Fatal(err)
	}

	patchTask := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindPatch,
		ArtifactPath:   patchPath,
		ArtifactSize:   int64(len(patchBytes)),
		ArtifactSHA256: hashBytes(patchBytes),
		Compression:    ArtifactCompressionZstd,
		SourceSHA256:   hashBytes(oldBytes),
		TargetSHA256:   hashBytes(newBytes),
		TargetSize:     int64(len(newBytes)),
	})
	if err := Prepare(patchTask); err != nil {
		t.Fatalf("prepare patch: %v", err)
	}
	assertFileBytes(t, localPath(stagingDir(patchTask), "duckdb.dll"), newBytes)

	fullPath := filepath.Join(workDir, "LunaBox.exe.zst")
	fullEncoder, err := zstd.NewWriter(nil, zstd.WithEncoderCRC(true))
	if err != nil {
		t.Fatal(err)
	}
	fullBytes := fullEncoder.EncodeAll(newBytes, nil)
	fullEncoder.Close()
	if err := os.WriteFile(fullPath, fullBytes, 0600); err != nil {
		t.Fatal(err)
	}

	fullTask := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindFull,
		ArtifactPath:   fullPath,
		ArtifactSize:   int64(len(fullBytes)),
		ArtifactSHA256: hashBytes(fullBytes),
		Compression:    ArtifactCompressionZstd,
		TargetSHA256:   hashBytes(newBytes),
		TargetSize:     int64(len(newBytes)),
	})
	if err := Prepare(fullTask); err != nil {
		t.Fatalf("prepare full fallback: %v", err)
	}
	assertFileBytes(t, localPath(stagingDir(fullTask), "duckdb.dll"), newBytes)
}

func TestPrepareRejectsWrongPatchSource(t *testing.T) {
	t.Parallel()

	appDir := t.TempDir()
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, "duckdb.dll"), []byte("installed"), 0755); err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(workDir, "patch.zsdiff")
	if err := os.WriteFile(patchPath, []byte("not reached"), 0600); err != nil {
		t.Fatal(err)
	}
	task := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindPatch,
		ArtifactPath:   patchPath,
		ArtifactSize:   11,
		ArtifactSHA256: hashBytes([]byte("not reached")),
		Compression:    ArtifactCompressionZstd,
		SourceSHA256:   strings.Repeat("0", 64),
		TargetSHA256:   strings.Repeat("1", 64),
		TargetSize:     1,
	})
	if err := Prepare(task); err == nil || !strings.Contains(err.Error(), "verify patch source") {
		t.Fatalf("expected source hash failure, got %v", err)
	}
}

func TestPreparedMarkerRejectsTaskChangesBeforeCommit(t *testing.T) {
	t.Parallel()

	appDir := t.TempDir()
	workDir := t.TempDir()
	newBytes := []byte("new duckdb")
	artifactPath := filepath.Join(workDir, "duckdb.full")
	if err := os.WriteFile(artifactPath, newBytes, 0600); err != nil {
		t.Fatal(err)
	}
	task := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindFull,
		ArtifactPath:   artifactPath,
		ArtifactSize:   int64(len(newBytes)),
		ArtifactSHA256: hashBytes(newBytes),
		Compression:    ArtifactCompressionNone,
		TargetSHA256:   hashBytes(newBytes),
		TargetSize:     int64(len(newBytes)),
	})
	if err := Prepare(task); err != nil {
		t.Fatal(err)
	}

	task.RestartArgs = []string{"--unexpected"}
	err := Commit(task)
	if err == nil || !strings.Contains(err.Error(), "task was modified") {
		t.Fatalf("expected modified task rejection, got %v", err)
	}
	if ShouldRestartAfterCommit(err) {
		t.Fatal("a pre-exit validation failure must not restart LunaBox")
	}
}

func TestRollbackFailurePreventsRestart(t *testing.T) {
	t.Parallel()

	err := &unsafeRestartCommitError{err: os.ErrPermission}
	if ShouldRestartAfterCommit(err) {
		t.Fatal("an incomplete rollback must not restart LunaBox")
	}
}

func TestRollbackRecoversReplacementBeforeFinalJournalWrite(t *testing.T) {
	t.Parallel()

	appDir := t.TempDir()
	workDir := t.TempDir()
	task := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindFull,
		ArtifactPath:   filepath.Join(workDir, "unused"),
		ArtifactSize:   1,
		ArtifactSHA256: strings.Repeat("1", 64),
		TargetSHA256:   strings.Repeat("2", 64),
		TargetSize:     1,
	})
	targetPath := filepath.Join(appDir, "duckdb.dll")
	backupPath := filepath.Join(appDir, ".duckdb.dll.test-transaction.bak")
	if err := os.WriteFile(targetPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	journal := &transactionJournal{
		TransactionID: task.TransactionID,
		Status:        "applying",
		Entries: []transactionJournalEntry{{
			Path:          "duckdb.dll",
			SwapPath:      filepath.Join(appDir, ".duckdb.dll.test-transaction.new"),
			BackupPath:    backupPath,
			TargetExisted: true,
			Attempted:     true,
			Applied:       false,
		}},
	}
	if err := rollbackTransaction(task, journal); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, targetPath, []byte("old"))
}

func TestRollbackRemovesNewFileWhenRenameConsumedSwap(t *testing.T) {
	t.Parallel()

	appDir := t.TempDir()
	workDir := t.TempDir()
	task := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindFull,
		ArtifactPath:   filepath.Join(workDir, "unused"),
		ArtifactSize:   1,
		ArtifactSHA256: strings.Repeat("1", 64),
		TargetSHA256:   strings.Repeat("2", 64),
		TargetSize:     1,
	})
	targetPath := filepath.Join(appDir, "duckdb.dll")
	if err := os.WriteFile(targetPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}
	journal := &transactionJournal{
		TransactionID: task.TransactionID,
		Status:        "applying",
		Entries: []transactionJournalEntry{{
			Path:          "duckdb.dll",
			SwapPath:      filepath.Join(appDir, ".duckdb.dll.test-transaction.new"),
			BackupPath:    filepath.Join(appDir, ".duckdb.dll.test-transaction.bak"),
			TargetExisted: false,
			Attempted:     true,
		}},
	}
	if err := rollbackTransaction(task, journal); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("new target should be removed, stat error: %v", err)
	}
}

func TestPrepareAndCommitFullUpdate(t *testing.T) {
	t.Parallel()

	appDir := t.TempDir()
	workDir := t.TempDir()
	oldBytes := []byte("old duckdb")
	newBytes := []byte("new duckdb")
	targetPath := filepath.Join(appDir, "duckdb.dll")
	if err := os.WriteFile(targetPath, oldBytes, 0755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(workDir, "duckdb.full")
	if err := os.WriteFile(artifactPath, newBytes, 0600); err != nil {
		t.Fatal(err)
	}
	task := testTask(appDir, workDir, TaskFile{
		Path:           "duckdb.dll",
		Kind:           TaskFileKindFull,
		ArtifactPath:   artifactPath,
		ArtifactSize:   int64(len(newBytes)),
		ArtifactSHA256: hashBytes(newBytes),
		Compression:    ArtifactCompressionNone,
		TargetSHA256:   hashBytes(newBytes),
		TargetSize:     int64(len(newBytes)),
	})
	if err := Prepare(task); err != nil {
		t.Fatal(err)
	}
	if err := Commit(task); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, targetPath, newBytes)
	if _, err := os.Stat(filepath.Join(workDir, "transaction.json")); !os.IsNotExist(err) {
		t.Fatalf("transaction journal should be cleaned up, stat error: %v", err)
	}
	if _, err := os.Stat(stagingDir(task)); !os.IsNotExist(err) {
		t.Fatalf("prepared files should be cleaned up, stat error: %v", err)
	}
}

func testTask(appDir string, workDir string, file TaskFile) *Task {
	return &Task{
		SchemaVersion: TaskSchemaVersion,
		TransactionID: "test-transaction",
		TargetVersion: "9.9.9",
		BuildMode:     "portable",
		AppDir:        appDir,
		WorkDir:       workDir,
		WaitPID:       999999,
		RestartPath:   "LunaBox.exe",
		Files:         []TaskFile{file},
	}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertFileBytes(t *testing.T, filePath string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("file contents differ: got %d bytes, want %d", len(actual), len(expected))
	}
}
