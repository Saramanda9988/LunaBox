//go:build windows

package integrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectSteamLoginUser(t *testing.T) {
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
	users := parseSteamLoginUsers(data)

	tests := []struct {
		name          string
		autoLoginUser string
		want          string
	}{
		{name: "most recent user takes priority over stale auto login user", autoLoginUser: "OLDER", want: "39734274"},
		{name: "most recent user", want: "39734274"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := selectSteamLoginUser(users, test.autoLoginUser)
			if !found || got != test.want {
				t.Fatalf("selectSteamLoginUser() = %q, %v; want %q, true", got, found, test.want)
			}
		})
	}
}

func TestSelectSteamLoginUserUsesAutoLoginUserWhenMostRecentIsMissing(t *testing.T) {
	data := []byte(`"users"
{
	"76561198000000001"
	{
		"AccountName"		"older"
		"Timestamp"		"100"
	}
	"76561198000000002"
	{
		"AccountName"		"current"
		"Timestamp"		"200"
	}
}`)

	got, found := selectSteamLoginUser(parseSteamLoginUsers(data), "OLDER")
	if !found || got != "39734273" {
		t.Fatalf("selectSteamLoginUser() = %q, %v; want %q, true", got, found, "39734273")
	}
}

func TestSelectSteamLoginUserUsesTimestampWhenMostRecentIsMissing(t *testing.T) {
	data := []byte(`"users"
{
	"76561198000000001"
	{
		"AccountName"		"older"
		"Timestamp"		"100"
	}
	"76561198000000002"
	{
		"AccountName"		"newer"
		"Timestamp"		"200"
	}
}`)

	got, found := selectSteamLoginUser(parseSteamLoginUsers(data), "")
	if !found || got != "39734274" {
		t.Fatalf("selectSteamLoginUser() = %q, %v; want %q, true", got, found, "39734274")
	}
}

func TestMostRecentlyModifiedSteamUserID(t *testing.T) {
	steamRoot := t.TempDir()
	olderPath := createSteamLocalConfig(t, steamRoot, "111")
	newerPath := createSteamLocalConfig(t, steamRoot, "222")
	oldTime := time.Unix(100, 0)
	newTime := time.Unix(200, 0)
	if err := os.Chtimes(olderPath, oldTime, oldTime); err != nil {
		t.Fatalf("set older local config time: %v", err)
	}
	if err := os.Chtimes(newerPath, newTime, newTime); err != nil {
		t.Fatalf("set newer local config time: %v", err)
	}

	got, found := mostRecentlyModifiedSteamUserID(steamRoot, []string{"111", "222"})
	if !found || got != "222" {
		t.Fatalf("mostRecentlyModifiedSteamUserID() = %q, %v; want %q, true", got, found, "222")
	}
}

func TestMostRecentlyModifiedSteamUserIDRejectsTiedTimes(t *testing.T) {
	steamRoot := t.TempDir()
	firstPath := createSteamLocalConfig(t, steamRoot, "111")
	secondPath := createSteamLocalConfig(t, steamRoot, "222")
	modifiedAt := time.Unix(100, 0)
	for _, path := range []string{firstPath, secondPath} {
		if err := os.Chtimes(path, modifiedAt, modifiedAt); err != nil {
			t.Fatalf("set local config time: %v", err)
		}
	}

	got, found := mostRecentlyModifiedSteamUserID(steamRoot, []string{"111", "222"})
	if found || got != "" {
		t.Fatalf("mostRecentlyModifiedSteamUserID() = %q, %v; want empty result", got, found)
	}
}

func createSteamLocalConfig(t *testing.T, steamRoot string, userID string) string {
	t.Helper()
	configDirectory := filepath.Join(steamRoot, "userdata", userID, "config")
	if err := os.MkdirAll(configDirectory, 0o755); err != nil {
		t.Fatalf("create Steam user config directory: %v", err)
	}
	path := filepath.Join(configDirectory, "localconfig.vdf")
	if err := os.WriteFile(path, []byte(`"UserLocalConfigStore" {}`), 0o644); err != nil {
		t.Fatalf("create Steam local config: %v", err)
	}
	return path
}
