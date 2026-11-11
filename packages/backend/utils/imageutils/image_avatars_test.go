package imageutils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveManagedAvatarRefreshPolicy(t *testing.T) {
	const provider, userID = "avatartest", "42"
	t.Cleanup(func() { _ = RemoveManagedAvatar(provider, userID) })
	_ = RemoveManagedAvatar(provider, userID)

	var downloads int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&downloads, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("avatar-v" + string(rune('0'+atomic.LoadInt32(&downloads)))))
	}))
	defer server.Close()
	src := server.URL + "/avatar.png"

	first, err := ResolveManagedAvatar(server.Client(), src, provider, userID, false)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if !strings.HasPrefix(first, "/local/avatars/avatartest_42.png?v=") {
		t.Fatalf("unexpected avatar url %q", first)
	}

	// 缓存未过期：不下载，地址不变。
	again, err := ResolveManagedAvatar(server.Client(), src, provider, userID, false)
	if err != nil || again != first || atomic.LoadInt32(&downloads) != 1 {
		t.Fatalf("fresh cache should be reused: url=%q err=%v downloads=%d", again, err, downloads)
	}

	// 过期：重新下载，地址的版本号变化。
	path, _, _ := FindManagedAvatarFile(provider, userID)
	old := time.Now().Add(-AvatarRefreshInterval - time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	_, staleURL, _ := FindManagedAvatarFile(provider, userID)
	refreshed, err := ResolveManagedAvatar(server.Client(), src, provider, userID, false)
	if err != nil || atomic.LoadInt32(&downloads) != 2 || refreshed == staleURL {
		t.Fatalf("stale cache should be refreshed: url=%q stale=%q err=%v downloads=%d", refreshed, staleURL, err, downloads)
	}

	// force（重新登录）：即使缓存新鲜也重新下载。
	forced, err := ResolveManagedAvatar(server.Client(), src, provider, userID, true)
	if err != nil || atomic.LoadInt32(&downloads) != 3 || forced == refreshed {
		t.Fatalf("force should re-download: url=%q prev=%q err=%v downloads=%d", forced, refreshed, err, downloads)
	}

	// 下载失败：保留旧头像并返回它的地址。
	fail.Store(true)
	kept, err := ResolveManagedAvatar(server.Client(), src, provider, userID, true)
	if err == nil {
		t.Fatalf("expected download error")
	}
	if kept != forced {
		t.Fatalf("failed refresh should keep previous avatar: got %q want %q", kept, forced)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != "avatar-v3" {
		t.Fatalf("previous avatar file should be intact: %q %v", data, readErr)
	}
}
