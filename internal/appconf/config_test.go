package appconf

import (
	"reflect"
	"testing"
)

func TestNormalizeMetadataSourcesAcceptsOptInSources(t *testing.T) {
	got := normalizeMetadataSources([]string{"bangumi", "dlsite", "erogamescape", "DLSITE", "unknown"})
	want := []string{"bangumi", "dlsite", "erogamescape"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestNormalizeMetadataSourcesDefaultsDoNotIncludeOptInSources(t *testing.T) {
	got := normalizeMetadataSources(nil)

	for _, source := range got {
		if source == "dlsite" || source == "erogamescape" {
			t.Fatalf("opt-in source %q should not be enabled by default: %#v", source, got)
		}
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
