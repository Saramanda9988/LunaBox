package service

import (
	"testing"

	"lunabox/internal/version"
)

func TestGetUpdateURLs(t *testing.T) {
	previousServiceURL := version.UpdateServiceURL
	t.Cleanup(func() {
		version.UpdateServiceURL = previousServiceURL
	})

	service := NewUpdateService()
	version.UpdateServiceURL = "https://updates.example.com/"
	urls := service.getUpdateURLs("")
	if len(urls) != len(defaultUpdateURLs)+1 {
		t.Fatalf("getUpdateURLs() returned %d URLs, want %d", len(urls), len(defaultUpdateURLs)+1)
	}
	if urls[0] != "https://updates.example.com/version.json" {
		t.Fatalf("getUpdateURLs()[0] = %q", urls[0])
	}

	customURL := "https://mirror.example.com/version.json"
	customURLs := service.getUpdateURLs(customURL)
	if len(customURLs) != 1 || customURLs[0] != customURL {
		t.Fatalf("getUpdateURLs(custom) = %q", customURLs)
	}
}

func TestBuildOfficialUpdateManifestURL(t *testing.T) {
	tests := []struct {
		name           string
		serviceURL     string
		releaseVersion string
		want           string
		wantErr        bool
	}{
		{
			name:           "stable version",
			serviceURL:     "https://updates.example.com/",
			releaseVersion: "2.0.0",
			want:           "https://updates.example.com/v1/releases/2.0.0/manifest",
		},
		{
			name:           "version prefix and prerelease",
			serviceURL:     " https://updates.example.com/base/ ",
			releaseVersion: "v2.0.0-test.4",
			want:           "https://updates.example.com/base/v1/releases/2.0.0-test.4/manifest",
		},
		{
			name:           "empty service url",
			serviceURL:     "",
			releaseVersion: "2.0.0",
			want:           "",
		},
		{
			name:           "invalid version",
			serviceURL:     "https://updates.example.com",
			releaseVersion: "latest",
			wantErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildOfficialUpdateManifestURL(test.serviceURL, test.releaseVersion)
			if test.wantErr {
				if err == nil {
					t.Fatal("buildOfficialUpdateManifestURL() expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildOfficialUpdateManifestURL() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("buildOfficialUpdateManifestURL() = %q, want %q", got, test.want)
			}
		})
	}
}
