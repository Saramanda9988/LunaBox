package downloadutils

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

type transferProxyConfig struct {
	url string
}

func (c transferProxyConfig) NetworkProxyConfig() (string, string) {
	return proxyutils.ProxyModeManual, c.url
}

func TestDownloaderUsesConfiguredProxyAndApplicationUserAgent(t *testing.T) {
	requests := 0
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		if got := req.URL.String(); got != "http://download-test.invalid/archive.zip" {
			t.Fatalf("proxied URL = %q", got)
		}
		if got := req.Header.Get("User-Agent"); got != version.UserAgent() {
			t.Fatalf("User-Agent = %q", got)
		}
		w.Header().Set("Content-Length", "12")
		_, _ = w.Write([]byte("archive-data"))
	}))
	defer proxyServer.Close()

	downloader, _, err := NewDownloader(TransferConfig{
		ProxyConfig: transferProxyConfig{url: proxyServer.URL},
	})
	if err != nil {
		t.Fatalf("NewDownloader failed: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "archive.zip")
	if err := downloader.Download(context.Background(), TransferRequest{
		URL:             "http://download-test.invalid/archive.zip",
		DestinationPath: destination,
	}); err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if got := string(data); got != "archive-data" {
		t.Fatalf("downloaded data = %q", got)
	}
	if requests == 0 {
		t.Fatal("proxy server received no requests")
	}
}
