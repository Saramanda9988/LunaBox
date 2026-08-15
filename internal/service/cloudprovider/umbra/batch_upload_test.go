package umbra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/applog"
	"lunabox/internal/service/cloudprovider/batchupload"
)

func TestNewProviderUsesUmbraProfileID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/profile", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.UserProfile{ID: 42, Username: "luna"},
		})
	})
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()

	provider, err := newProviderWithClient(context.Background(), newTestUmbraClient(t, apiServer.URL))
	if err != nil {
		t.Fatalf("newProviderWithClient() error = %v", err)
	}
	if got := provider.GetCloudPath("legacy-user-id", "database/latest.zip"); got != "v1/42/database/latest.zip" {
		t.Fatalf("GetCloudPath() = %q, want %q", got, "v1/42/database/latest.zip")
	}
}

func TestUploadFilesUsesUmbraSyncExchange(t *testing.T) {
	previousMode := applog.GetMode()
	applog.SetMode(applog.ModeCLI)
	defer applog.SetMode(previousMode)

	var mu sync.Mutex
	exchangeBatchSizes := make([]int, 0)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/client/sync/snapshot", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.SyncSnapshotPage{
				Records:        []umbrsdk.SyncChange{},
				ExchangeCursor: "cursor-0",
			},
		})
	})
	mux.HandleFunc("/api/v1/client/sync/exchange", func(w http.ResponseWriter, r *http.Request) {
		var request umbrsdk.SyncExchangeInput
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		exchangeBatchSizes = append(exchangeBatchSizes, len(request.Mutations))
		mu.Unlock()
		accepted := make([]umbrsdk.SyncAcceptedMutation, len(request.Mutations))
		for i, mutation := range request.Mutations {
			accepted[i] = umbrsdk.SyncAcceptedMutation{MutationID: mutation.MutationID, RecordVersion: 1}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.SyncExchangeResult{
				Accepted:  accepted,
				Conflicts: []umbrsdk.SyncConflict{},
				Rejected:  []umbrsdk.SyncRejectedMutation{},
				Changes:   []umbrsdk.SyncChange{},
			},
		})
	})
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()

	tokenStore := umbrsdk.NewMemoryTokenStore()
	if err := tokenStore.Save(context.Background(), &umbrsdk.TokenSet{
		AccessToken: "token",
		TokenType:   "bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	deviceStore := umbrsdk.NewMemoryDeviceStore()
	if err := deviceStore.Save(context.Background(), &umbrsdk.DeviceCredentials{DeviceID: "dev_test", DeviceSecret: "secret"}); err != nil {
		t.Fatal(err)
	}
	client, err := umbrsdk.New(umbrsdk.Config{
		BaseURL:     apiServer.URL,
		ClientID:    "client",
		RedirectURI: "http://127.0.0.1:1420/auth/callback",
		TokenStore:  tokenStore,
		DeviceStore: deviceStore,
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &Provider{client: client, userID: "42"}

	tempDir := t.TempDir()
	items := make([]batchupload.Item, 501)
	for i := range items {
		localPath := filepath.Join(tempDir, fmt.Sprintf("item-%03d.json", i))
		if err := os.WriteFile(localPath, []byte(fmt.Sprintf(`{"item":%d}`, i)), 0o600); err != nil {
			t.Fatal(err)
		}
		items[i] = batchupload.Item{
			CloudPath: fmt.Sprintf("v1/42/sync/library/games/item-%03d.json", i),
			LocalPath: localPath,
		}
	}

	if err := provider.UploadFiles(context.Background(), items); err != nil {
		t.Fatalf("UploadFiles() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(exchangeBatchSizes) != "[500 1]" {
		t.Fatalf("exchange batch sizes = %v, want [500 1]", exchangeBatchSizes)
	}
}

func TestUploadFilesRejectsAssetsExceedingAvailableQuota(t *testing.T) {
	previousMode := applog.GetMode()
	applog.SetMode(applog.ModeCLI)
	defer applog.SetMode(previousMode)

	presignCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/quota", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": umbrsdk.QuotaInfo{QuotaBytes: 100, UsedBytes: 95, AvailableBytes: 5},
		})
	})
	mux.HandleFunc("/api/v1/client/backups/presign-upload-batch", func(w http.ResponseWriter, r *http.Request) {
		presignCalls++
		http.Error(w, "unexpected presign", http.StatusInternalServerError)
	})
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()

	client := newTestUmbraClient(t, apiServer.URL)
	provider := &Provider{client: client, userID: "42"}
	tempDir := t.TempDir()
	items := make([]batchupload.Item, 0, 2)
	for i := range 2 {
		localPath := filepath.Join(tempDir, fmt.Sprintf("cover-%d.webp", i))
		if err := os.WriteFile(localPath, []byte("123"), 0o600); err != nil {
			t.Fatal(err)
		}
		items = append(items, batchupload.Item{
			CloudPath: fmt.Sprintf("v1/42/sync/covers/game-%d.webp", i),
			LocalPath: localPath,
		})
	}

	err := provider.UploadFiles(context.Background(), items)
	if err == nil {
		t.Fatal("UploadFiles() expected insufficient quota error")
	}
	if !strings.Contains(err.Error(), "本次需要 6 bytes，当前可用 5 bytes") {
		t.Fatalf("UploadFiles() error = %v", err)
	}
	if presignCalls != 0 {
		t.Fatalf("presign calls = %d, want 0", presignCalls)
	}
}

func TestPutPresignedFileRetriesRateLimitAndReplaysFile(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upload body: %v", err)
		}
		if string(body) != "cover-data" {
			t.Fatalf("upload body on attempt %d = %q, want cover-data", attempts, body)
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "<Error><Code>TooManyRequests</Code></Error>", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	localPath := filepath.Join(tempDir, "cover.webp")
	if err := os.WriteFile(localPath, []byte("cover-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(localPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	client := newTestUmbraClientWithHTTPClient(t, "http://example.invalid", server.Client())
	provider := &Provider{client: client, userID: "42"}
	upload := preparedUpload{
		item:        batchupload.Item{CloudPath: "v1/42/sync/covers/game.webp", LocalPath: localPath},
		contentType: "image/webp",
		fileSize:    uint64(len("cover-data")),
		file:        file,
	}
	if err := provider.putPresignedFile(context.Background(), upload, server.URL); err != nil {
		t.Fatalf("putPresignedFile() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func newTestUmbraClient(t *testing.T, baseURL string) *umbrsdk.Client {
	return newTestUmbraClientWithHTTPClient(t, baseURL, nil)
}

func newTestUmbraClientWithHTTPClient(t *testing.T, baseURL string, httpClient *http.Client) *umbrsdk.Client {
	t.Helper()
	tokenStore := umbrsdk.NewMemoryTokenStore()
	if err := tokenStore.Save(context.Background(), &umbrsdk.TokenSet{
		AccessToken: "token",
		TokenType:   "bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	deviceStore := umbrsdk.NewMemoryDeviceStore()
	if err := deviceStore.Save(context.Background(), &umbrsdk.DeviceCredentials{DeviceID: "dev_test", DeviceSecret: "secret"}); err != nil {
		t.Fatal(err)
	}
	client, err := umbrsdk.New(umbrsdk.Config{
		BaseURL:     baseURL,
		ClientID:    "client",
		RedirectURI: "http://127.0.0.1:1420/auth/callback",
		HTTPClient:  httpClient,
		TokenStore:  tokenStore,
		DeviceStore: deviceStore,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
