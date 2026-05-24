package umbra

import (
	"testing"

	umbraSDK "github.com/Umbrae-Labs/umbra-core/sdk/umbra-go"
)

func TestGetCloudPathUsesUmbraCategories(t *testing.T) {
	provider := &UmbraProvider{}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "db folder", in: "database/", want: "db"},
		{name: "db file", in: "database/lunabox_2026-05-17T21-20-00.zip", want: "db/lunabox_2026-05-17T21-20-00.zip"},
		{name: "game folder", in: "saves/game-1/", want: "game/game-1"},
		{name: "game file", in: "saves/game-1/latest.zip", want: "game/game-1/latest.zip"},
		{name: "library folder", in: "sync/library", want: "asset/lunabox_sync_library"},
		{name: "library snapshot", in: "sync/library/latest.json", want: "asset/lunabox_sync_library/latest.json"},
		{name: "cover file", in: "sync/covers/game-1.jpg", want: "asset/lunabox_sync_covers/game-1__jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := provider.GetCloudPath("", tt.in); got != tt.want {
				t.Fatalf("GetCloudPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParsePrefixAcceptsUmbraCategoryPrefixes(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		wantPath     string
		wantCategory umbraSDK.BackupCategory
		wantSubject  string
	}{
		{name: "db", prefix: "db", wantPath: "db/", wantCategory: umbraSDK.CategoryDB},
		{name: "db slash", prefix: "db/", wantPath: "db/", wantCategory: umbraSDK.CategoryDB},
		{name: "legacy db", prefix: "database", wantPath: "db/", wantCategory: umbraSDK.CategoryDB},
		{name: "game", prefix: "game/game-1", wantPath: "game/game-1/", wantCategory: umbraSDK.CategoryGame, wantSubject: "game-1"},
		{name: "legacy game", prefix: "saves/game-1", wantPath: "game/game-1/", wantCategory: umbraSDK.CategoryGame, wantSubject: "game-1"},
		{name: "asset", prefix: "asset/lunabox_sync_library", wantPath: "asset/lunabox_sync_library/", wantCategory: umbraSDK.CategoryAsset, wantSubject: "lunabox_sync_library"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrefix(tt.prefix)
			if err != nil {
				t.Fatalf("parsePrefix(%q) returned error: %v", tt.prefix, err)
			}
			if got.PathPrefix != tt.wantPath || got.Category != tt.wantCategory || got.Subject != tt.wantSubject {
				t.Fatalf("parsePrefix(%q) = %+v, want path=%q category=%q subject=%q", tt.prefix, got, tt.wantPath, tt.wantCategory, tt.wantSubject)
			}
		})
	}
}

func TestParseCloudPathAcceptsUmbraCategoryPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want umbraSDK.BackupAddress
	}{
		{name: "db", path: "db/lunabox_2026-05-17T21-20-00.zip", want: umbraSDK.DBBackup("lunabox_2026-05-17T21-20-00")},
		{name: "legacy db", path: "database/lunabox_2026-05-17T21-20-00.zip", want: umbraSDK.DBBackup("lunabox_2026-05-17T21-20-00")},
		{name: "game", path: "game/game-1/latest.zip", want: umbraSDK.GameBackup("game-1", "latest")},
		{name: "legacy game", path: "saves/game-1/latest.zip", want: umbraSDK.GameBackup("game-1", "latest")},
		{name: "library snapshot", path: "asset/lunabox_sync_library/latest.json", want: umbraSDK.AssetBackup("lunabox_sync_library", "latest")},
		{name: "cover", path: "asset/lunabox_sync_covers/game-1__jpg", want: umbraSDK.AssetBackup("lunabox_sync_covers", "game-1__jpg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCloudPath(tt.path)
			if err != nil {
				t.Fatalf("parseCloudPath(%q) returned error: %v", tt.path, err)
			}
			if got != tt.want {
				t.Fatalf("parseCloudPath(%q) = %+v, want %+v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRecordToPathUsesUmbraCategoryPaths(t *testing.T) {
	tests := []struct {
		name   string
		record umbraSDK.BackupRecord
		want   string
	}{
		{name: "db", record: umbraSDK.BackupRecord{Category: string(umbraSDK.CategoryDB), Version: "v1"}, want: "db/v1.zip"},
		{name: "game", record: umbraSDK.BackupRecord{Category: string(umbraSDK.CategoryGame), Subject: "game-1", Version: "v1"}, want: "game/game-1/v1.zip"},
		{name: "full", record: umbraSDK.BackupRecord{Category: string(umbraSDK.CategoryFull), Version: "v1"}, want: "full/v1.zip"},
		{name: "library snapshot", record: umbraSDK.BackupRecord{Category: string(umbraSDK.CategoryAsset), Subject: subjectSyncLibrary, Version: "latest"}, want: "asset/lunabox_sync_library/latest.json"},
		{name: "cover", record: umbraSDK.BackupRecord{Category: string(umbraSDK.CategoryAsset), Subject: subjectSyncCovers, Version: "game-1__jpg"}, want: "asset/lunabox_sync_covers/game-1__jpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := recordToPath(tt.record)
			if !ok {
				t.Fatalf("recordToPath(%+v) returned !ok", tt.record)
			}
			if got != tt.want {
				t.Fatalf("recordToPath(%+v) = %q, want %q", tt.record, got, tt.want)
			}
		})
	}
}
