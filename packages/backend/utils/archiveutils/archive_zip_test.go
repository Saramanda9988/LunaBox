package archiveutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnzipForRestoreWithProgressReportsAllBytes(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"lunabox.db":       strings.Repeat("d", 100*1024),
		"covers/a.png":     strings.Repeat("a", 3000),
		"covers/sub/b.png": "b",
	}
	var want int64
	for name, content := range files {
		path := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		want += int64(len(content))
	}
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := ZipDirectory(src, archive); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	var calls int
	var lastDone, lastTotal int64
	err := UnzipForRestoreWithProgress(archive, dest, func(done, total int64) {
		if done < lastDone {
			t.Errorf("progress went backwards: %d after %d", done, lastDone)
		}
		calls++
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatal(err)
	}
	if lastTotal != want || lastDone != want {
		t.Fatalf("last progress = %d/%d, want %d/%d", lastDone, lastTotal, want, want)
	}
	if calls < 2 {
		t.Fatalf("progress called %d times, want an initial report and at least one write", calls)
	}
	got, err := os.ReadFile(filepath.Join(dest, "covers", "a.png"))
	if err != nil || string(got) != files["covers/a.png"] {
		t.Fatalf("extracted covers/a.png = %q, %v", got, err)
	}
}
