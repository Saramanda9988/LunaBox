package webdav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
)

type testProxyConfig struct {
	mode string
	url  string
}

func (c testProxyConfig) NetworkProxyConfig() (string, string) {
	return c.mode, c.url
}

func TestDownloadFileUsesRestyRetryAndApplicationHeaders(t *testing.T) {
	attempts := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempts++
		if req.URL.Host != "webdav-test.invalid" {
			t.Fatalf("proxied URL = %q", req.URL.String())
		}
		if got := req.Header.Get("User-Agent"); got != version.UserAgent() {
			t.Fatalf("User-Agent = %q", got)
		}
		username, password, ok := req.BasicAuth()
		if !ok || username != "luna" || password != "secret" {
			t.Fatalf("BasicAuth = %q, %q, %v", username, password, ok)
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("backup-data"))
	}))
	defer proxyServer.Close()

	provider, err := NewProvider(Config{
		URL:      "http://webdav-test.invalid/root",
		Username: "luna",
		Password: "secret",
		ProxyConfig: testProxyConfig{
			mode: proxyutils.ProxyModeManual,
			url:  proxyServer.URL,
		},
	})
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "backup.zip")
	if err := provider.DownloadFile(context.Background(), "backup.zip", destination); err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if got := string(data); got != "backup-data" {
		t.Fatalf("downloaded data = %q", got)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
