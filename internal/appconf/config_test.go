package appconf

import (
	enums2 "lunabox/internal/common/enums"
	"reflect"
	"testing"
)

func TestNormalizeMetadataSourcesAcceptsOptInSources(t *testing.T) {
	got := normalizeMetadataSources([]string{"bangumi", "dlsite", "touchgal", "hikarinagi", "erogamescape", "DLSITE", "unknown"})
	want := []string{"bangumi", "dlsite", "touchgal", "hikarinagi", "erogamescape"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestNormalizeMetadataSourcesUsesExpectedDefaults(t *testing.T) {
	got := normalizeMetadataSources(nil)
	want := []string{"bangumi", "vndb", "hikarinagi", "steam"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestNormalizeMetadataCoverSourcesDefaultsToHikarinagi(t *testing.T) {
	config := &AppConfig{
		BangumiCoverSource: "unknown",
	}

	NormalizeMetadataCoverSources(config)

	if config.BangumiCoverSource != enums2.MetadataCoverSourceHikarinagi {
		t.Fatalf("expected Bangumi cover source to default to Hikarinagi, got %q", config.BangumiCoverSource)
	}
	if config.VNDBCoverSource != enums2.MetadataCoverSourceHikarinagi {
		t.Fatalf("expected VNDB cover source to default to Hikarinagi, got %q", config.VNDBCoverSource)
	}
}

func TestNormalizeMetadataCoverSourcesKeepsOriginal(t *testing.T) {
	config := &AppConfig{
		BangumiCoverSource: enums2.MetadataCoverSourceOriginal,
		VNDBCoverSource:    enums2.MetadataCoverSourceOriginal,
	}

	NormalizeMetadataCoverSources(config)

	if config.BangumiCoverSource != enums2.MetadataCoverSourceOriginal || config.VNDBCoverSource != enums2.MetadataCoverSourceOriginal {
		t.Fatalf("expected original cover sources to remain unchanged: %+v", config)
	}
}

func TestNormalizeProxySettingsKeepsNetworkProxyURLAsGlobalURL(t *testing.T) {
	config := &AppConfig{
		NetworkProxyMode: "manual",
		NetworkProxyURL:  " 127.0.0.1:7890 ",
	}

	if !NormalizeProxySettings(config) {
		t.Fatal("expected proxy normalization to report changes")
	}
	if config.NetworkProxyURL != "127.0.0.1:7890" {
		t.Fatalf("expected global proxy URL to be trimmed, got %q", config.NetworkProxyURL)
	}
	if config.NetworkProxyMode != "manual" {
		t.Fatalf("unexpected proxy mode: %q", config.NetworkProxyMode)
	}
}

func TestNetworkProxyConfigReturnsGlobalProxy(t *testing.T) {
	config := &AppConfig{
		NetworkProxyMode: "manual",
		NetworkProxyURL:  "http://127.0.0.1:7890",
	}

	mode, proxyURL := config.NetworkProxyConfig()
	if mode != "manual" || proxyURL != config.NetworkProxyURL {
		t.Fatalf("unexpected network proxy config: mode=%q url=%q", mode, proxyURL)
	}
}

func TestSanitizeUmbraConfigPreservesConfiguredBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "stage service",
			baseURL: "https://stage.umbrae.cc",
			want:    "https://stage.umbrae.cc",
		},
		{
			name:    "custom service with surrounding whitespace",
			baseURL: " https://umbra.example.com/// ",
			want:    "https://umbra.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AppConfig{
				UmbraBaseURL:       tt.baseURL,
				UmbraAuthenticated: true,
			}

			SanitizeUmbraConfig(config)
			if config.UmbraBaseURL != tt.want {
				t.Fatalf("expected Umbra base URL %q, got %q", tt.want, config.UmbraBaseURL)
			}
			if !config.UmbraAuthenticated {
				t.Fatal("sanitizing a configured Umbra base URL cleared authentication")
			}
		})
	}
}

func TestNormalizeScrapedTagLimitAllowsZeroAndUnlimited(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "too negative becomes unlimited", limit: -2, want: -1},
		{name: "unlimited stays negative one", limit: -1, want: -1},
		{name: "zero disables scraped tags", limit: 0, want: 0},
		{name: "positive limit is kept", limit: 10, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeScrapedTagLimit(tt.limit); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestNormalizeProcessDetectionTimeoutSec(t *testing.T) {
	tests := []struct {
		name       string
		timeoutSec int
		want       int
	}{
		{name: "missing value uses default", timeoutSec: 0, want: DefaultProcessDetectionTimeoutSec},
		{name: "value below minimum is clamped", timeoutSec: 30, want: MinProcessDetectionTimeoutSec},
		{name: "valid value is kept", timeoutSec: 180, want: 180},
		{name: "value above maximum is clamped", timeoutSec: 900, want: MaxProcessDetectionTimeoutSec},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeProcessDetectionTimeoutSec(tt.timeoutSec); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestMigrateLegacyCompatibilityConfigMovesCrossOverFields(t *testing.T) {
	config := &AppConfig{
		WineRunnerPath: "/Applications/CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine",
		WinePrefix:     "Legacy Bottle",
	}

	if !MigrateLegacyCompatibilityConfig(config) {
		t.Fatal("expected legacy CrossOver config to be migrated")
	}
	if config.WineRunnerPath != "" || config.WinePrefix != "" {
		t.Fatalf("legacy shared fields were not cleared: path=%q prefix=%q", config.WineRunnerPath, config.WinePrefix)
	}
	if config.CrossOverRunnerPath != "/Applications/CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine" || config.CrossOverBottle != "Legacy Bottle" {
		t.Fatalf("unexpected migrated CrossOver config: path=%q bottle=%q", config.CrossOverRunnerPath, config.CrossOverBottle)
	}
}

func TestMigrateLegacyCompatibilityConfigKeepsWineFields(t *testing.T) {
	config := &AppConfig{
		WineRunnerPath: "/opt/homebrew/bin/wine",
		WinePrefix:     "/Users/test/.wine",
	}

	if MigrateLegacyCompatibilityConfig(config) {
		t.Fatal("plain Wine config should not be migrated")
	}
	if config.WineRunnerPath != "/opt/homebrew/bin/wine" || config.WinePrefix != "/Users/test/.wine" {
		t.Fatalf("Wine config changed unexpectedly: %+v", config)
	}
}
