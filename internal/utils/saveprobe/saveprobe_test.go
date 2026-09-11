package saveprobe

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRankCandidatesPrefersRelatedSaveFiles(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "slot1.sav")
	second := filepath.Join(directory, "global.dat")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("save"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	now := time.Now()
	candidates := rankCandidates([]fileActivity{
		{path: first, writes: 3, lastSeen: now},
		{path: second, writes: 2, lastSeen: now},
	}, "", now)

	if len(candidates) == 0 {
		t.Fatal("expected candidates")
	}
	if candidates[0].Kind != "directory" || candidates[0].Path != directory {
		t.Fatalf("expected directory candidate first, got %#v", candidates[0])
	}
	if candidates[0].Confidence == "low" {
		t.Fatalf("expected at least medium confidence, got %s", candidates[0].Confidence)
	}
}

func TestIncidentalActivityLosesScore(t *testing.T) {
	directory := t.TempDir()
	savePath := filepath.Join(directory, "progress.sav")
	logDirectory := filepath.Join(directory, "logs")
	if err := os.MkdirAll(logDirectory, 0o700); err != nil {
		t.Fatalf("create log directory: %v", err)
	}
	logPath := filepath.Join(logDirectory, "game.log")
	for _, path := range []string{savePath, logPath} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	now := time.Now()
	candidates := rankCandidates([]fileActivity{
		{path: savePath, writes: 1, lastSeen: now},
		{path: logPath, writes: 4, lastSeen: now},
	}, "", now)

	var saveScore int
	var logScore int
	for _, candidate := range candidates {
		if candidate.Kind != "file" {
			continue
		}
		switch candidate.Path {
		case savePath:
			saveScore = candidate.Score
		case logPath:
			logScore = candidate.Score
		}
	}
	if saveScore <= logScore {
		t.Fatalf("expected save score %d to exceed log score %d", saveScore, logScore)
	}
}

func TestRecentCreateEventProducesCandidate(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "AoKana018.sud")
	if err := os.WriteFile(path, []byte("save data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	now := time.Now()
	candidates := rankCandidates([]fileActivity{
		{path: path, creates: 1, firstSeen: now.Add(-time.Second), lastSeen: now},
	}, directory, now)

	for _, candidate := range candidates {
		if candidate.Path == path {
			return
		}
	}
	t.Fatalf("expected recent create event for %s to produce a candidate", path)
}

func TestCreateEventIgnoresUnchangedFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "settings.dat")
	if err := os.WriteFile(path, []byte("old data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	now := time.Now()
	old := now.Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("set fixture time: %v", err)
	}
	candidates := rankCandidates([]fileActivity{
		{path: path, creates: 1, firstSeen: now.Add(-time.Second), lastSeen: now},
	}, directory, now)

	for _, candidate := range candidates {
		if candidate.Path == path {
			t.Fatalf("expected unchanged create-only file to be omitted, got %#v", candidate)
		}
	}
}

func TestDirectoryChangeProducesCandidateAndCount(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "UserData", "AoKana018.sud")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create user data directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("save data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	store := newActivityStore()
	now := time.Now()
	store.recordDirectoryChange(path, 16, now)
	result := store.result(now.Add(-time.Second), now, directory)

	if result.ObservedEvents != 1 {
		t.Fatalf("expected one directory event, got %d", result.ObservedEvents)
	}
	for _, candidate := range result.Candidates {
		if candidate.Path == path {
			return
		}
	}
	t.Fatalf("expected directory change for %s to produce a candidate", path)
}
