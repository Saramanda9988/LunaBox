package umbra

import (
	"testing"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
)

func TestAddressRoundTrip(t *testing.T) {
	tests := []string{
		"saves/550e8400-e29b-41d4-a716-446655440000/2026-07-10T12-30-45.zip",
		"saves/550e8400-e29b-41d4-a716-446655440000/latest.zip",
		"database/lunabox_2026-07-10T12-30-45.zip",
		"database/latest.zip",
		"sync/covers/550e8400-e29b-41d4-a716-446655440000.webp",
	}

	for _, want := range tests {
		t.Run(want, func(t *testing.T) {
			address, err := addressForSubPath(want)
			if err != nil {
				t.Fatalf("addressForSubPath() error = %v", err)
			}
			got, ok := subPathForRecord(umbrsdk.BackupRecord{
				Category: string(address.Category),
				Subject:  address.Subject,
				Version:  address.Version,
			})
			if !ok {
				t.Fatal("subPathForRecord() did not recognize mapped address")
			}
			if got != want {
				t.Fatalf("round trip = %q, want %q", got, want)
			}
		})
	}
}

func TestSyncKeyRoundTrip(t *testing.T) {
	tests := []string{
		"sync/library/manifest.json",
		"sync/library/game_progresses/f.json",
	}
	for _, want := range tests {
		t.Run(want, func(t *testing.T) {
			key, ok, err := syncKeyForSubPath(want)
			if err != nil {
				t.Fatalf("syncKeyForSubPath() error = %v", err)
			}
			if !ok {
				t.Fatal("syncKeyForSubPath() did not recognize sync path")
			}
			got, ok := subPathForSyncKey(key)
			if !ok || got != want {
				t.Fatalf("round trip = %q, %v; want %q", got, ok, want)
			}
		})
	}
}

func TestListQueryForGame(t *testing.T) {
	query, err := listQueryForSubPath("saves/550e8400-e29b-41d4-a716-446655440000/")
	if err != nil {
		t.Fatalf("listQueryForSubPath() error = %v", err)
	}
	if query.filter.Category != umbrsdk.CategoryGame || query.filter.Subject == "" {
		t.Fatalf("unexpected game list filter: %#v", query.filter)
	}
	if query.prefix != "saves/550e8400-e29b-41d4-a716-446655440000/" {
		t.Fatalf("prefix = %q", query.prefix)
	}
}

func TestAddressRejectsUnsupportedPath(t *testing.T) {
	if _, err := addressForSubPath("other/file.bin"); err == nil {
		t.Fatal("addressForSubPath() expected an error")
	}
}

func TestProviderCloudPathUsesServerAccountScope(t *testing.T) {
	provider := &Provider{userID: "42"}
	cloudPath := provider.GetCloudPath("legacy-user-id", "database/latest.zip")
	if cloudPath != "v1/42/database/latest.zip" {
		t.Fatalf("GetCloudPath() = %q, want %q", cloudPath, "v1/42/database/latest.zip")
	}

	subPath, err := provider.subPath(cloudPath)
	if err != nil {
		t.Fatalf("subPath() error = %v", err)
	}
	if subPath != "database/latest.zip" {
		t.Fatalf("subPath() = %q, want %q", subPath, "database/latest.zip")
	}
}
