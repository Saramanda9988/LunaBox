//go:build linux

package archiveutils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractArchiveWithBundled7zLinux(t *testing.T) {
	if _, err := resolveBundled7zz(); err != nil {
		t.Skipf("bundled 7zz is unavailable: %v", err)
	}

	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source.zip")
	archive, err := os.Create(source)
	if err != nil {
		t.Fatalf("create test archive: %v", err)
	}

	zipWriter := zip.NewWriter(archive)
	entry, err := zipWriter.Create("中文目录/文件.txt")
	if err != nil {
		t.Fatalf("create archive entry: %v", err)
	}
	if _, err := entry.Write([]byte("lunabox")); err != nil {
		t.Fatalf("write archive entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close test archive: %v", err)
	}

	target := filepath.Join(tempDir, "output")
	extracted, err := extractArchiveWithBundled7z(source, target)
	if err != nil {
		t.Fatalf("extract archive with bundled 7zz: %v", err)
	}
	if !extracted {
		t.Fatal("extractArchiveWithBundled7z() extracted = false, want true")
	}

	content, err := os.ReadFile(filepath.Join(target, "中文目录", "文件.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "lunabox" {
		t.Fatalf("extracted content = %q, want %q", content, "lunabox")
	}
}
