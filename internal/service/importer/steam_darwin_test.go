//go:build darwin

package importer

import (
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func TestFindSteamInstallPathInHome(t *testing.T) {
	homeDir := t.TempDir()
	expected := filepath.Join(homeDir, "Library", "Application Support", "Steam")
	if err := os.MkdirAll(filepath.Join(expected, "steamapps"), 0o755); err != nil {
		t.Fatalf("create Steam library: %v", err)
	}

	actual, err := findSteamInstallPathInHome(homeDir)
	if err != nil {
		t.Fatalf("find Steam install path: %v", err)
	}
	if actual != expected {
		t.Fatalf("expected Steam path %q, got %q", expected, actual)
	}
}

func TestFindSteamInstallPathInHomeRequiresSteamApps(t *testing.T) {
	if _, err := findSteamInstallPathInHome(t.TempDir()); err == nil {
		t.Fatal("expected missing steamapps directory to be rejected")
	}
}

func TestIsImportableSteamGameRequiresInstalledNumericAppID(t *testing.T) {
	installDir := t.TempDir()
	base := SteamLocalGame{
		AppID:      "123456",
		Name:       "Native Steam Game",
		InstallDir: installDir,
		StateFlags: steamFullyInstalledFlag,
	}
	if !isImportableSteamGame(base) {
		t.Fatal("expected fully installed Steam game with numeric AppID to be importable")
	}

	invalidAppID := base
	invalidAppID.AppID = "shortcut"
	if isImportableSteamGame(invalidAppID) {
		t.Fatal("expected non-numeric AppID to be rejected")
	}

	incomplete := base
	incomplete.StateFlags = 2
	if isImportableSteamGame(incomplete) {
		t.Fatal("expected incomplete Steam game to be rejected")
	}
}

func TestSteamPreviewTreatsSyncedGameWithoutPathAsImportable(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	steamPath := filepath.Join(homeDir, "Library", "Application Support", "Steam")
	installDir := filepath.Join(steamPath, "steamapps", "common", "Native Steam Game")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("create Steam game directory: %v", err)
	}
	manifest := []byte(`"AppState"
{
	"appid" "123456"
	"name" "Native Steam Game"
	"installdir" "Native Steam Game"
	"StateFlags" "4"
}`)
	manifestPath := filepath.Join(steamPath, "steamapps", "appmanifest_123456.acf")
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		t.Fatalf("write Steam manifest: %v", err)
	}

	previewForPath := func(existingPath string) PreviewGame {
		t.Helper()
		steamImporter := NewSteamImporter(Dependencies{
			ListGames: func() ([]models.Game, error) {
				return []models.Game{{
					ID:         "synced-game",
					Name:       "Synced Steam Game",
					Path:       existingPath,
					SourceType: enums.Steam,
					SourceID:   "123456",
				}}, nil
			},
		})
		previews, err := steamImporter.Preview()
		if err != nil {
			t.Fatalf("Preview() error = %v", err)
		}
		if len(previews) != 1 {
			t.Fatalf("preview count = %d, want 1", len(previews))
		}
		return previews[0]
	}

	missingPath := previewForPath("")
	if missingPath.Exists || missingPath.ConflictType != ConflictTypeNone || missingPath.ExistingID != "synced-game" {
		t.Fatalf("missing-path synced game should be an ordinary import candidate: %+v", missingPath)
	}

	existingPath := previewForPath(filepath.Join(t.TempDir(), "other-install"))
	if !existingPath.Exists || existingPath.ConflictType != ConflictTypeSource {
		t.Fatalf("game with a local path should remain a source conflict: %+v", existingPath)
	}
}
