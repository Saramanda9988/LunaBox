package imageutils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"lunabox/internal/version"
)

func TestImageRequestRetriesRateLimitWithApplicationHeaders(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempts++
		if got := req.Header.Get("User-Agent"); got != version.UserAgent() {
			t.Fatalf("User-Agent = %q", got)
		}
		if got := req.Header.Get("Accept"); got == "" {
			t.Fatal("Accept header is empty")
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("image-data"))
	}))
	defer server.Close()

	client, err := newImageRestyClientWithHTTPClient(server.Client())
	if err != nil {
		t.Fatalf("create image Resty client: %v", err)
	}
	response, err := newImageRequest(context.Background(), client).Get(server.URL)
	if err != nil {
		t.Fatalf("perform image request: %v", err)
	}
	if response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode())
	}
	if got := string(response.Bytes()); got != "image-data" {
		t.Fatalf("body = %q", got)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
