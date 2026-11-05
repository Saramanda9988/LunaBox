package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lunabox/backend/common/vo"
)

// 批量导入的各个扫描入口都应清洗搜索名，原文件夹名保留在 FolderName。
func TestBatchScanCleansSearchName(t *testing.T) {
	const raw = "[110624][Navel] 俺たちに翼はない R -Limited Edition- (nrg)"
	const want = "俺たちに翼はない R"

	root := t.TempDir()
	gameDir := filepath.Join(root, raw)
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "game.exe"), []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &ImportService{ctx: context.Background()}
	check := func(name string, m map[string]vo.BatchImportCandidate) {
		t.Helper()
		c, ok := m[gameDir]
		if !ok {
			t.Fatalf("%s: candidate not found in %v", name, m)
		}
		if c.SearchName != want {
			t.Errorf("%s: SearchName = %q, want %q", name, c.SearchName, want)
		}
	}

	for _, mode := range []string{"parent", "depth"} {
		m := map[string]vo.BatchImportCandidate{}
		if err := s.scanDirectoryRecursive(root, root, 0, 3, nil, mode, 0, m); err != nil {
			t.Fatal(err)
		}
		check("recursive/"+mode, m)
		if m[gameDir].FolderName != raw {
			t.Errorf("recursive/%s: FolderName = %q, want original %q", mode, m[gameDir].FolderName, raw)
		}
	}

	m := map[string]vo.BatchImportCandidate{}
	if err := s.scanDirectoryByHierarchy(root, 0, m); err != nil {
		t.Fatal(err)
	}
	check("hierarchy", m)

	m = map[string]vo.BatchImportCandidate{}
	s.scanDroppedDirectoryByHierarchy(root, 1, m)
	check("dropped hierarchy", m)

	m = map[string]vo.BatchImportCandidate{}
	if err := s.scanDroppedDirectoryRecursive(gameDir, 3, nil, vo.BatchImportScanOptions{ScanNameMode: "depth"}, m); err != nil {
		t.Fatal(err)
	}
	check("dropped recursive", m)

	if got := searchNameForExecutable("game.exe", gameDir); got != want {
		t.Errorf("dropped exe: SearchName = %q, want %q", got, want)
	}
}
