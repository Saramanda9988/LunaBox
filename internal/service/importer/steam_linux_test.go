//go:build linux

package importer

import (
	"os"
	"path/filepath"
	"testing"

	"lunabox/internal/common/enums"
)

func TestFindSteamInstallPathLinuxUsesEnvCandidate(t *testing.T) {
	root := t.TempDir()
	steamPath := filepath.Join(root, "custom-steam")
	if err := os.MkdirAll(filepath.Join(steamPath, "steamapps"), 0755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}

	t.Setenv("STEAM_DIR", steamPath)
	t.Setenv("STEAM_HOME", "")
	t.Setenv("STEAM_ROOT", "")
	t.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "missing-xdg"))
	t.Setenv("HOME", filepath.Join(root, "missing-home"))

	got, err := findSteamInstallPath()
	if err != nil {
		t.Fatalf("findSteamInstallPath returned error: %v", err)
	}
	if got != steamPath {
		t.Fatalf("expected %q, got %q", steamPath, got)
	}
}

func TestFindSteamInstallPathLinuxUsesXDGDataHome(t *testing.T) {
	root := t.TempDir()
	xdgDataHome := filepath.Join(root, "xdg-data")
	steamPath := filepath.Join(xdgDataHome, "Steam")
	if err := os.MkdirAll(filepath.Join(steamPath, "steamapps"), 0755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}

	t.Setenv("STEAM_DIR", "")
	t.Setenv("STEAM_HOME", "")
	t.Setenv("STEAM_ROOT", "")
	t.Setenv("STEAM_COMPAT_CLIENT_INSTALL_PATH", "")
	t.Setenv("XDG_DATA_HOME", xdgDataHome)
	t.Setenv("HOME", filepath.Join(root, "missing-home"))

	got, err := findSteamInstallPath()
	if err != nil {
		t.Fatalf("findSteamInstallPath returned error: %v", err)
	}
	if got != steamPath {
		t.Fatalf("expected %q, got %q", steamPath, got)
	}
}

func TestNormalizeSteamLinuxInstallPathAcceptsSteamappsDirectory(t *testing.T) {
	root := t.TempDir()
	steamapps := filepath.Join(root, "Steam", "steamapps")
	if err := os.MkdirAll(steamapps, 0755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}

	got, ok := normalizeSteamLinuxInstallPath(steamapps)
	if !ok {
		t.Fatal("expected steamapps path to be accepted")
	}
	if got != filepath.Dir(steamapps) {
		t.Fatalf("expected %q, got %q", filepath.Dir(steamapps), got)
	}
}

func TestReadSteamManifestDetectsProtonPrefix(t *testing.T) {
	libraryPath := t.TempDir()
	installDir := filepath.Join(libraryPath, "steamapps", "common", "Native Game")
	protonPrefix := filepath.Join(libraryPath, "steamapps", "compatdata", "123456", "pfx")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("create install dir: %v", err)
	}
	if err := os.MkdirAll(protonPrefix, 0o755); err != nil {
		t.Fatalf("create proton prefix: %v", err)
	}

	manifestPath := filepath.Join(libraryPath, "steamapps", "appmanifest_123456.acf")
	if err := os.WriteFile(manifestPath, []byte(`"AppState"
{
	"appid"		"123456"
	"name"		"Native Game"
	"installdir"		"Native Game"
	"StateFlags"		"4"
}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	game, err := readSteamManifest(libraryPath, manifestPath)
	if err != nil {
		t.Fatalf("read steam manifest: %v", err)
	}
	if game.ProtonPrefix != protonPrefix {
		t.Fatalf("expected proton prefix %q, got %q", protonPrefix, game.ProtonPrefix)
	}
}

func TestDefaultSteamImportedLaunchModeLinuxUsesSteamLaunch(t *testing.T) {
	if got := defaultSteamImportedLaunchMode(); got != enums.LaunchModeSteam {
		t.Fatalf("expected Steam launch mode on Linux, got %q", got)
	}
}

func TestIsImportableSteamGameRejectsRuntimeAndCompatibilityTools(t *testing.T) {
	installDir := t.TempDir()
	makeGame := func(appID string, name string) SteamLocalGame {
		return SteamLocalGame{
			AppID:      appID,
			Name:       name,
			InstallDir: installDir,
			StateFlags: steamFullyInstalledFlag,
		}
	}

	rejected := []SteamLocalGame{
		makeGame("228980", "Steamworks Common Redistributables"),
		makeGame("2805730", "Some Renamed Proton Tool"),
		makeGame("123456", "Steam Linux Runtime 3.0 (sniper)"),
		makeGame("123457", "Proton 9.0"),
		makeGame("123458", "Proton Experimental"),
		makeGame("123459", "Proton BattlEye Runtime"),
	}
	for _, game := range rejected {
		if isImportableSteamGame(game) {
			t.Fatalf("expected %s/%s to be rejected", game.AppID, game.Name)
		}
	}

	if !isImportableSteamGame(makeGame("123460", "Proton Bus Simulator")) {
		t.Fatal("expected ordinary game name containing Proton to remain importable")
	}
}
