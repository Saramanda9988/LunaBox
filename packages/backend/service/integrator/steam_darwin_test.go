//go:build darwin

package integrator

import (
	"context"
	"fmt"
	"lunabox/backend/models"
	"lunabox/backend/utils/steamutils"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withDarwinSteamEnv(t *testing.T, steamRoot string, running *bool) {
	t.Helper()
	prevRoot := findDarwinSteamRootFn
	prevRunning := isDarwinSteamRunningFn
	prevCover := findManagedSteamArtworkCover
	findDarwinSteamRootFn = func() (string, error) {
		if steamRoot == "" {
			return "", fmt.Errorf("Steam is not installed")
		}
		return steamRoot, nil
	}
	isDarwinSteamRunningFn = func() bool {
		return running != nil && *running
	}
	findManagedSteamArtworkCover = func(string) (string, string, error) {
		return "", "", nil
	}
	t.Cleanup(func() {
		findDarwinSteamRootFn = prevRoot
		isDarwinSteamRunningFn = prevRunning
		findManagedSteamArtworkCover = prevCover
	})
}

func setupDarwinSteamRoot(t *testing.T, userID string) string {
	t.Helper()
	steamRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(steamRoot, "steamapps", "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(steamRoot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(steamRoot, "userdata", userID, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	loginUsers := fmt.Sprintf(`"users"
{
	"76561198000000001"
	{
		"AccountName"		"macuser"
		"MostRecent"		"1"
		"Timestamp"		"200"
	}
}`)
	if err := os.WriteFile(filepath.Join(steamRoot, "config", "loginusers.vdf"), []byte(loginUsers), 0o644); err != nil {
		t.Fatal(err)
	}
	return steamRoot
}

func TestNativeSteamLaunchIDAcceptsOnlyNativeAppID(t *testing.T) {
	tests := []struct {
		name string
		game models.Game
		want string
		ok   bool
	}{
		{name: "native", game: models.Game{SteamLaunchKind: "native", SteamLaunchID: "123456"}, want: "123456", ok: true},
		{name: "shortcut", game: models.Game{SteamLaunchKind: "shortcut", SteamLaunchID: "123456"}},
		{name: "empty kind", game: models.Game{SteamLaunchID: "123456"}},
		{name: "invalid app id", game: models.Game{SteamLaunchKind: "native", SteamLaunchID: "not-an-id"}},
		{name: "zero app id", game: models.Game{SteamLaunchKind: "native", SteamLaunchID: "0"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := nativeSteamLaunchID(test.game)
			if ok != test.ok || got != test.want {
				t.Fatalf("nativeSteamLaunchID(%+v) = %q, %v; want %q, %v", test.game, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestDarwinSteamImportExecutableHandlesAppBundleAndRegularFiles(t *testing.T) {
	dir := t.TempDir()

	appBundle := filepath.Join(dir, "MyGame.app")
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := steamImportExecutable(appBundle); !ok || got != appBundle {
		t.Fatalf("expected .app bundle to be accepted, got %q, %v", got, ok)
	}

	plainDir := filepath.Join(dir, "NotABundle")
	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := steamImportExecutable(plainDir); ok {
		t.Fatalf("expected plain directory to be rejected, got %q", got)
	}

	regularFile := filepath.Join(dir, "game_bin")
	if err := os.WriteFile(regularFile, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := steamImportExecutable(regularFile); !ok || got != regularFile {
		t.Fatalf("expected regular file to be accepted, got %q, %v", got, ok)
	}

	if _, ok := steamImportExecutable(filepath.Join(dir, "missing.app")); ok {
		t.Fatal("expected missing path to be rejected")
	}
}

func TestDarwinResolveDetectsNativeSteamGameByDirectory(t *testing.T) {
	running := false
	steamRoot := setupDarwinSteamRoot(t, "39734273")
	withDarwinSteamEnv(t, steamRoot, &running)

	installDir := filepath.Join(steamRoot, "steamapps", "common", "SampleGame")
	appBundle := filepath.Join(installDir, "SampleGame.app")
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `"AppState"
{
	"appid"		"424242"
	"installdir"		"SampleGame"
}`
	if err := os.WriteFile(filepath.Join(steamRoot, "steamapps", "appmanifest_424242.acf"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ResolveSteamTarget(context.Background(), models.Game{
		ID:   "g1",
		Name: "Sample Game",
		Path: appBundle,
	})
	if err != nil {
		t.Fatalf("ResolveSteamTarget: %v", err)
	}
	if !result.Status.Ready || result.Status.LaunchKind != "native" || result.Status.LaunchID != "424242" {
		t.Fatalf("unexpected native resolution: %+v", result.Status)
	}
}

func TestDarwinImportAndSetLaunchOptionsForAppBundle(t *testing.T) {
	running := false
	steamRoot := setupDarwinSteamRoot(t, "39734273")
	withDarwinSteamEnv(t, steamRoot, &running)

	appBundle := filepath.Join(t.TempDir(), "VisualNovel.app")
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}

	game := models.Game{
		ID:   "g1",
		Name: "Visual Novel",
		Path: appBundle,
	}

	// Before import: needs_import
	resolved, err := ResolveSteamTarget(context.Background(), game)
	if err != nil {
		t.Fatalf("ResolveSteamTarget before import: %v", err)
	}
	if resolved.Status.State != SteamLaunchStateNeedsImport || resolved.Status.UserID != "39734273" {
		t.Fatalf("unexpected status before import: %+v", resolved.Status)
	}

	// When Steam is running: steam_running
	running = true
	resolvedRunning, err := ResolveSteamTarget(context.Background(), game)
	if err != nil {
		t.Fatalf("ResolveSteamTarget while running: %v", err)
	}
	if resolvedRunning.Status.State != SteamLaunchStateSteamRunning || !resolvedRunning.Status.SteamRunning {
		t.Fatalf("unexpected status while running: %+v", resolvedRunning.Status)
	}

	// Stop Steam and import with initial launch options
	running = false
	initialOpts := "LANG=zh_CN.UTF-8 %command%"
	imported, err := ImportSteamShortcut(context.Background(), game, SteamImportOptions{
		LaunchOptions: &initialOpts,
	})
	if err != nil {
		t.Fatalf("ImportSteamShortcut: %v", err)
	}
	if !imported.Imported || !imported.Status.Ready || imported.Status.LaunchKind != "shortcut" || imported.Status.LaunchID == "" {
		t.Fatalf("unexpected import result: %+v", imported)
	}

	// Verify shortcuts.vdf is readable and contains the entry
	shortcutsPath := steamShortcutsPath(steamRoot, "39734273")
	shortcuts, err := loadSteamShortcuts(shortcutsPath)
	if err != nil {
		t.Fatalf("loadSteamShortcuts: %v", err)
	}
	appID, found := shortcuts.Find(appBundle, imported.Status.LaunchID)
	if !found || steamutils.ShortcutLongID(appID) != imported.Status.LaunchID {
		t.Fatalf("shortcut not found in %s: appID=%d found=%v", shortcutsPath, appID, found)
	}

	// Update launch options when Steam is running -> rejected
	game.SteamLaunchKind = imported.Status.LaunchKind
	game.SteamLaunchID = imported.Status.LaunchID
	game.SteamUserID = imported.Status.UserID
	running = true
	if _, err := SetSteamLaunchOptions(context.Background(), game, "--windowed", SteamSettingsOptions{}); err == nil {
		t.Fatal("expected error updating launch options while Steam is running")
	}

	// Update launch options when Steam is stopped -> succeeds with backup
	running = false
	updated, err := SetSteamLaunchOptions(context.Background(), game, "--windowed", SteamSettingsOptions{})
	if err != nil {
		t.Fatalf("SetSteamLaunchOptions: %v", err)
	}
	if !updated.Status.Ready || updated.BackupPath == "" {
		t.Fatalf("unexpected launch options update result: %+v", updated)
	}
	if _, err := os.Stat(updated.BackupPath); err != nil {
		t.Fatalf("expected backup file at %s: %v", updated.BackupPath, err)
	}
}

func TestDarwinBatchImportShortcuts(t *testing.T) {
	running := false
	steamRoot := setupDarwinSteamRoot(t, "39734273")
	withDarwinSteamEnv(t, steamRoot, &running)

	dir := t.TempDir()
	appBundle := filepath.Join(dir, "BatchGame.app")
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	binFile := filepath.Join(dir, "unix_game")
	if err := os.WriteFile(binFile, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	plainDir := filepath.Join(dir, "FolderOnly")
	if err := os.MkdirAll(plainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	batch, err := ImportSteamShortcuts(context.Background(), []models.Game{
		{ID: "g-native", Name: "Native", SteamLaunchKind: "native", SteamLaunchID: "999"},
		{ID: "g-app", Name: "Batch App", Path: appBundle},
		{ID: "g-bin", Name: "Unix Bin", Path: binFile},
		{ID: "g-dir", Name: "No Exe", Path: plainDir},
	})
	if err != nil {
		t.Fatalf("ImportSteamShortcuts: %v", err)
	}
	if len(batch.Items) != 4 {
		t.Fatalf("expected 4 batch items, got %d", len(batch.Items))
	}
	if !batch.Items[0].Result.Status.Ready || batch.Items[0].Result.Status.LaunchKind != "native" {
		t.Fatalf("item 0: %+v", batch.Items[0])
	}
	if !batch.Items[1].Result.Imported || !batch.Items[1].Result.Status.Ready || batch.Items[1].Result.Status.LaunchKind != "shortcut" {
		t.Fatalf("item 1: %+v", batch.Items[1])
	}
	if !batch.Items[2].Result.Imported || !batch.Items[2].Result.Status.Ready || batch.Items[2].Result.Status.LaunchKind != "shortcut" {
		t.Fatalf("item 2: %+v", batch.Items[2])
	}
	if batch.Items[3].Result.Status.State != SteamLaunchStateExecutableRequired {
		t.Fatalf("item 3: %+v", batch.Items[3])
	}
}

func TestDarwinActiveSteamUserIDUsesRegistryAutoLogin(t *testing.T) {
	steamRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(steamRoot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	registryVDF := `"Registry"
{
	"HKCU"
	{
		"Software"
		{
			"Valve"
			{
				"Steam"
				{
					"AutoLoginUser"		"alice"
				}
			}
		}
	}
}`
	if err := os.WriteFile(filepath.Join(steamRoot, "registry.vdf"), []byte(registryVDF), 0o644); err != nil {
		t.Fatal(err)
	}
	loginUsers := `"users"
{
	"76561198000000001"
	{
		"AccountName"		"alice"
		"Timestamp"		"100"
	}
	"76561198000000002"
	{
		"AccountName"		"bob"
		"Timestamp"		"200"
	}
}`
	if err := os.WriteFile(filepath.Join(steamRoot, "config", "loginusers.vdf"), []byte(loginUsers), 0o644); err != nil {
		t.Fatal(err)
	}

	userID, err := activeSteamUserID(steamRoot)
	if err != nil || userID != "39734273" {
		t.Fatalf("activeSteamUserID() = %q, %v; want 39734273, nil", userID, err)
	}
}

func TestDarwinMostRecentlyModifiedSteamUserID(t *testing.T) {
	steamRoot := t.TempDir()
	for _, entry := range []struct {
		id string
		ts time.Time
	}{
		{"111", time.Unix(100, 0)},
		{"222", time.Unix(200, 0)},
	} {
		cfgDir := filepath.Join(steamRoot, "userdata", entry.id, "config")
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		cfgPath := filepath.Join(cfgDir, "localconfig.vdf")
		if err := os.WriteFile(cfgPath, []byte(`"UserLocalConfigStore" {}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(cfgPath, entry.ts, entry.ts); err != nil {
			t.Fatal(err)
		}
	}

	got, found := mostRecentlyModifiedSteamUserID(steamRoot, []string{"111", "222"})
	if !found || got != "222" {
		t.Fatalf("mostRecentlyModifiedSteamUserID() = %q, %v; want 222, true", got, found)
	}
}
