//go:build linux

package apputils

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindExecutablesAllowsWindowsExecutablesOnLinux(t *testing.T) {
	dir := t.TempDir()
	for _, file := range []struct {
		name string
		mode os.FileMode
	}{
		{name: "Game.exe", mode: 0o644},
		{name: "patch.bat", mode: 0o644},
		{name: "native", mode: 0o755},
		{name: "notes.txt", mode: 0o644},
	} {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, []byte("test"), file.mode); err != nil {
			t.Fatal(err)
		}
	}

	executables := FindExecutables(dir, nil)
	names := make([]string, 0, len(executables))
	for _, executable := range executables {
		names = append(names, filepath.Base(executable))
	}
	slices.Sort(names)

	want := []string{"Game.exe", "native", "patch.bat"}
	if !slices.Equal(names, want) {
		t.Fatalf("FindExecutables() = %#v, want %#v", names, want)
	}

	best := SelectBestExecutable(executables, "Game")
	if filepath.Base(best) != "Game.exe" {
		t.Fatalf("expected Game.exe as best executable, got %q", best)
	}
}
