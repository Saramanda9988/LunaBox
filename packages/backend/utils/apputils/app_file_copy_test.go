package apputils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyDirWithProgressCountsAllBytes(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"a.png":         strings.Repeat("a", 70*1024),
		"nested/b.webp": "bb",
		"nested/c.jpg":  "",
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

	dst := filepath.Join(t.TempDir(), "covers")
	var copied int64
	if err := CopyDirWithProgress(src, dst, func(n int64) { copied += n }); err != nil {
		t.Fatal(err)
	}
	if copied != want {
		t.Fatalf("copied %d bytes, want %d", copied, want)
	}
	for name, content := range files {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(name)))
		if err != nil || string(got) != content {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
}
