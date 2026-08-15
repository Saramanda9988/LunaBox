//go:build linux

package integrator

import (
	"context"
	"lunabox/internal/models"
	"lunabox/internal/utils/steamutils"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSteamRootLinuxUsesEnvCandidate(t *testing.T) {
	steamRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(steamRoot, "steamapps"), 0o755); err != nil {
		t.Fatalf("create steamapps: %v", err)
	}
	t.Setenv("STEAM_DIR", steamRoot)

	got, err := findSteamRoot()
	if err != nil {
		t.Fatalf("findSteamRoot() returned error: %v", err)
	}
	if got != steamRoot {
		t.Fatalf("findSteamRoot() = %q, want %q", got, steamRoot)
	}
}

func TestLinuxResolveSteamTargetFindsNativeGame(t *testing.T) {
	steamRoot := t.TempDir()
	installDir := filepath.Join(steamRoot, "steamapps", "common", "Native Game")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("create native game dir: %v", err)
	}
	protonPrefix := filepath.Join(steamRoot, "steamapps", "compatdata", "123456", "pfx")
	if err := os.MkdirAll(protonPrefix, 0o755); err != nil {
		t.Fatalf("create proton prefix: %v", err)
	}
	manifestPath := filepath.Join(steamRoot, "steamapps", "appmanifest_123456.acf")
	if err := os.WriteFile(manifestPath, []byte(`"AppState"
{
	"appid"		"123456"
	"name"		"Native Game"
	"installdir"		"Native Game"
	"StateFlags"		"4"
}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Setenv("STEAM_DIR", steamRoot)

	result, err := resolveSteamPlatformTarget(context.Background(), models.Game{
		Path:          installDir,
		GameDirectory: installDir,
	})
	if err != nil {
		t.Fatalf("resolveSteamPlatformTarget() returned error: %v", err)
	}
	if !result.Status.Ready || result.Status.State != SteamLaunchStateReady {
		t.Fatalf("expected ready native status, got %+v", result.Status)
	}
	if result.Status.LaunchID != "123456" || result.Status.LaunchKind != "native" {
		t.Fatalf("unexpected launch identity: %+v", result.Status)
	}
	if result.Status.ProtonPrefix != protonPrefix {
		t.Fatalf("expected proton prefix %q, got %q", protonPrefix, result.Status.ProtonPrefix)
	}
}

func TestLinuxFindSteamProtonPrefixUsesShortcutCompatdataID(t *testing.T) {
	steamRoot := t.TempDir()
	appID := uint32(0x81234567)
	appIDs := steamShortcutCompatdataIDs(appID)
	protonPrefix := filepath.Join(steamRoot, "steamapps", "compatdata", appIDs[0], "pfx")
	if err := os.MkdirAll(protonPrefix, 0o755); err != nil {
		t.Fatalf("create shortcut proton prefix: %v", err)
	}

	got := findSteamProtonPrefix(steamRoot, appIDs...)
	if got != protonPrefix {
		t.Fatalf("expected shortcut proton prefix %q, got %q", protonPrefix, got)
	}
}

func TestLinuxFindSteamProtonPrefixUsesShortcutLongCompatdataID(t *testing.T) {
	steamRoot := t.TempDir()
	appID := uint32(0x81234567)
	appIDs := steamShortcutCompatdataIDs(appID)
	protonPrefix := filepath.Join(steamRoot, "steamapps", "compatdata", appIDs[1], "pfx")
	if err := os.MkdirAll(protonPrefix, 0o755); err != nil {
		t.Fatalf("create shortcut long proton prefix: %v", err)
	}

	got := findSteamProtonPrefix(steamRoot, appIDs...)
	if got != protonPrefix {
		t.Fatalf("expected shortcut long proton prefix %q, got %q", protonPrefix, got)
	}
}

func TestLinuxSteamCompatibilityShortcutAppIDUsesShortID(t *testing.T) {
	appID := uint32(0x81234567)
	got := steamCompatibilityLaunchAppID("shortcut", steamutils.ShortcutLongID(appID))
	if got != "2166572391" {
		t.Fatalf("expected shortcut app ID %q, got %q", "2166572391", got)
	}
}

func TestLinuxSteamCompatibilityInfoIncludesProtonPrefix(t *testing.T) {
	steamRoot := t.TempDir()
	protonPrefix := filepath.Join(steamRoot, "steamapps", "compatdata", "123456", "pfx")
	if err := os.MkdirAll(protonPrefix, 0o755); err != nil {
		t.Fatalf("create proton prefix: %v", err)
	}
	t.Setenv("STEAM_DIR", steamRoot)

	info, err := getSteamPlatformCompatibilityInfo(context.Background(), models.Game{
		SteamLaunchID:   "123456",
		SteamLaunchKind: "native",
	})
	if err != nil {
		t.Fatalf("get compatibility info: %v", err)
	}
	if info.ProtonPrefix != protonPrefix {
		t.Fatalf("expected proton prefix %q, got %q", protonPrefix, info.ProtonPrefix)
	}
}

func TestLinuxUpdateSteamCompatibilityToolAddsAndRemovesMapping(t *testing.T) {
	steamRoot := t.TempDir()
	configDir := filepath.Join(steamRoot, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.vdf")
	if err := os.WriteFile(configPath, []byte(`"InstallConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"SomeOtherConfig"		"1"
			}
		}
	}
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := updateSteamCompatibilityTool(steamRoot, "123456", "GE-Proton9-20"); err != nil {
		t.Fatalf("update compatibility tool: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"CompatToolMapping"`) ||
		!strings.Contains(content, `"123456"`) ||
		!strings.Contains(content, `"name"`+"\t\t"+`"GE-Proton9-20"`) {
		t.Fatalf("compatibility mapping was not written:\n%s", content)
	}
	mapping, err := readSteamCompatibilityMapping(steamRoot)
	if err != nil {
		t.Fatalf("read mapping: %v", err)
	}
	if mapping["123456"] != "GE-Proton9-20" {
		t.Fatalf("expected mapping to GE-Proton9-20, got %q", mapping["123456"])
	}

	if err := updateSteamCompatibilityTool(steamRoot, "123456", ""); err != nil {
		t.Fatalf("clear compatibility tool: %v", err)
	}
	mapping, err = readSteamCompatibilityMapping(steamRoot)
	if err != nil {
		t.Fatalf("read mapping after clear: %v", err)
	}
	if mapping["123456"] != "" {
		t.Fatalf("expected mapping to be cleared, got %q", mapping["123456"])
	}
}

func TestLinuxSteamCompatibilityToolsReadsCustomTool(t *testing.T) {
	steamRoot := t.TempDir()
	toolDir := filepath.Join(steamRoot, "compatibilitytools.d", "GE-Proton9-20")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("create compatibility tool dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolDir, "compatibilitytool.vdf"), []byte(`"compatibilitytools"
{
	"compat_tools"
	{
		"GE-Proton9-20" // Internal name of this tool
		{
			"display_name" "GE-Proton9-20"
		}
	}
}`), 0o644); err != nil {
		t.Fatalf("write compatibilitytool.vdf: %v", err)
	}

	tools := steamCompatibilityTools(steamRoot)
	for _, tool := range tools {
		if tool.Name == "GE-Proton9-20" && tool.DisplayName == "GE-Proton9-20" && tool.Path == toolDir {
			return
		}
	}
	t.Fatalf("custom compatibility tool was not detected: %+v", tools)
}

func TestLinuxSelectSteamLoginUser(t *testing.T) {
	data := []byte(`"users"
{
	"76561198000000001"
	{
		"AccountName"		"older"
		"MostRecent"		"0"
		"Timestamp"		"100"
	}
	"76561198000000002"
	{
		"AccountName"		"current"
		"MostRecent"		"1"
		"Timestamp"		"200"
	}
}`)

	got, found := selectSteamLoginUser(parseSteamLoginUsers(data), "")
	if !found || got != "39734274" {
		t.Fatalf("selectSteamLoginUser() = %q, %v; want %q, true", got, found, "39734274")
	}
}
