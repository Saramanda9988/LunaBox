package gamehelper

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lunabox/backend/models"
)

func TestCheckGamePaths(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "game.exe")
	if err := os.WriteFile(exe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	games := []models.Game{
		{ID: "ok", Path: exe},
		{ID: "dir", Path: dir},
		{ID: "missing", Path: filepath.Join(dir, "renamed", "game.exe")},
		{ID: "empty", Path: "  "},
		{ID: "url", Path: "steam://rungameid/1"},
	}

	got := CheckGamePaths(context.Background(), games)
	if got.Checked != 3 {
		t.Errorf("checked %d, want 3", got.Checked)
	}
	if len(got.Missing) != 1 || got.Missing[0].GameID != "missing" {
		t.Errorf("missing %+v", got.Missing)
	}
	if len(got.Unavailable) != 0 {
		t.Errorf("unavailable %+v", got.Unavailable)
	}
}

func TestCheckGamePathsSkipsUnavailableVolume(t *testing.T) {
	var path string
	if runtime.GOOS == "windows" {
		// 找一个没有分配的盘符。
		for l := 'Z'; l >= 'D'; l-- {
			if _, err := os.Stat(string(l) + `:\`); err != nil {
				path = string(l) + `:\Games\game.exe`
				break
			}
		}
		if path == "" {
			t.Skip("no unused drive letter")
		}
	} else {
		path = "/Volumes/lunabox-test-missing-volume/game.app"
	}

	got := CheckGamePaths(context.Background(), []models.Game{{ID: "offline", Path: path}})
	if len(got.Missing) != 0 || len(got.Unavailable) != 1 || got.Unavailable[0].GameID != "offline" {
		t.Errorf("missing %+v, unavailable %+v", got.Missing, got.Unavailable)
	}
}

func TestVolumeRoot(t *testing.T) {
	tests := map[string]string{
		"/Volumes/Disk/Games/a.app": "/Volumes/Disk",
		"/media/user/Disk/a":        "/media/user/Disk",
		"/run/media/user/Disk/a":    "/run/media/user/Disk",
		"/mnt/games/a":              "/mnt/games",
		"/home/user/a":              "",
		"/mnt":                      "",
	}
	if runtime.GOOS == "windows" {
		tests = map[string]string{
			`D:\Games\a.exe`:          `D:\`,
			`\\nas\share\Games\a.exe`: `\\nas\share\`,
			`relative\a.exe`:          "",
		}
	}
	for in, want := range tests {
		if got := volumeRoot(in); got != want {
			t.Errorf("volumeRoot(%q) = %q, want %q", in, got, want)
		}
	}
}
