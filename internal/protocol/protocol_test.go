package protocol

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func installURLForTest(extra map[string]string) string {
	values := url.Values{}
	values.Set("url", "https://example.com/game.zip")
	values.Set("file_name", "game.zip")
	values.Set("archive_format", "zip")
	values.Set("size", "1")
	values.Set("checksum_algo", "sha256")
	values.Set("checksum", strings.Repeat("a", 64))
	values.Set("expires_at", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	for key, value := range extra {
		values.Set(key, value)
	}
	return (&url.URL{Scheme: Scheme, Host: ActionInstall, RawQuery: values.Encode()}).String()
}

func TestParseInstallURLStripTopLevelDefaultsFalse(t *testing.T) {
	req, err := ParseInstallURL(installURLForTest(nil))
	if err != nil {
		t.Fatalf("parse install URL: %v", err)
	}
	if req.StripTopLevel {
		t.Fatal("strip_top_level should default to false")
	}
}

func TestParseInstallURLStripTopLevelParsesBool(t *testing.T) {
	req, err := ParseInstallURL(installURLForTest(map[string]string{"strip_top_level": "true"}))
	if err != nil {
		t.Fatalf("parse install URL: %v", err)
	}
	if !req.StripTopLevel {
		t.Fatal("strip_top_level=true should enable top-level stripping")
	}
}

func TestParseInstallURLStripTopLevelRejectsInvalidBool(t *testing.T) {
	if _, err := ParseInstallURL(installURLForTest(map[string]string{"strip_top_level": "maybe"})); err == nil {
		t.Fatal("invalid strip_top_level should fail")
	}
}
