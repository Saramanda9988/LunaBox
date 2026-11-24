package pluginui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/api"
	"lunabox/desktop/ui/env"
)

func makeTestPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func waitForImageSource(t *testing.T, mgr *Manager, src string) ui.ImageSource {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := mgr.LoadImageSource(src); got != nil {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for image source %q", src)
	return nil
}

func TestImageDiskCacheReusesBitmapAndSVGAccessAcrossManagers(t *testing.T) {
	pngBytes := makeTestPNG(t, 64, 48, color.RGBA{R: 120, G: 180, B: 240, A: 255})
	svgBytes := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20" viewBox="0 0 40 20"><rect width="40" height="20" fill="#ff0000"/></svg>`)

	var pngHits atomic.Int32
	var svgHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/banner.png":
			pngHits.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		case "/logo.svg":
			svgHits.Add(1)
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write(svgBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	bannerURL := srv.URL + "/banner.png"
	logoURL := srv.URL + "/logo.svg"

	// 第一次加载：走网络并写入磁盘缓存
	mgr1 := New(&env.Env{MainWidth: 1200}, HostCallbacks{})
	mgr1.enableImageDiskCache(cacheDir)

	bmp1, ok := waitForImageSource(t, mgr1, bannerURL).(*ui.Bitmap)
	if !ok || bmp1 == nil {
		t.Fatalf("expected *ui.Bitmap from first load")
	}
	if w, h := bmp1.Size(); w != 64 || h != 48 {
		t.Fatalf("unexpected bitmap size %dx%d, want 64x48", w, h)
	}
	svg1, ok := waitForImageSource(t, mgr1, logoURL).(*ui.SVG)
	if !ok || svg1 == nil {
		t.Fatalf("expected *ui.SVG from first load")
	}

	// 等待异步落盘完成（saveToDisk 在 invalidateWindows 之后执行）
	disk := openImageDiskCacheAt(cacheDir)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, bHit := disk.loadBitmap(bannerURL)
		_, sHit := disk.loadBytes(logoURL)
		if bHit && sHit {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pngHits.Load() != 1 || svgHits.Load() != 1 {
		t.Fatalf("expected 1 fetch each before second manager, got png=%d svg=%d", pngHits.Load(), svgHits.Load())
	}

	// 模拟应用重启后的新 Manager：同一磁盘目录应直接命中，不再发起网络请求
	mgr2 := New(&env.Env{MainWidth: 1200}, HostCallbacks{})
	mgr2.enableImageDiskCache(cacheDir)

	bmp2, ok := waitForImageSource(t, mgr2, bannerURL).(*ui.Bitmap)
	if !ok || bmp2 == nil {
		t.Fatalf("expected *ui.Bitmap from disk cache")
	}
	svg2, ok := waitForImageSource(t, mgr2, logoURL).(*ui.SVG)
	if !ok || svg2 == nil {
		t.Fatalf("expected *ui.SVG from disk cache")
	}
	if pngHits.Load() != 1 || svgHits.Load() != 1 {
		t.Fatalf("expected disk cache hit with 0 extra HTTP requests, got png=%d svg=%d", pngHits.Load(), svgHits.Load())
	}
}

func TestResolveLocalPathAndLoadLocalCover(t *testing.T) {
	dataDir := t.TempDir()
	coverDir := filepath.Join(dataDir, "covers")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	coverFile := filepath.Join(coverDir, "demo.png")
	if err := os.WriteFile(coverFile, makeTestPNG(t, 32, 32, color.RGBA{R: 50, G: 200, B: 80, A: 255}), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := New(&env.Env{
		MainWidth: 1200,
		Svc:       &api.Services{DataDir: dataDir},
	}, HostCallbacks{})

	if got, isLocal := mgr.resolveLocalPath("/local/covers/demo.png?v=1"); !isLocal || got != coverFile {
		t.Fatalf("resolveLocalPath(/local/covers/demo.png?v=1) = (%q, %v), want (%q, true)", got, isLocal, coverFile)
	}
	if got, isLocal := mgr.resolveLocalPath("/local/../secret.png"); !isLocal || got != "" {
		t.Fatalf("expected traversal path to be rejected, got (%q, %v)", got, isLocal)
	}

	bmp, ok := waitForImageSource(t, mgr, "/local/covers/demo.png").(*ui.Bitmap)
	if !ok || bmp == nil {
		t.Fatalf("expected /local/ cover to load into *ui.Bitmap")
	}
	if w, h := bmp.Size(); w != 32 || h != 32 {
		t.Fatalf("unexpected local cover size %dx%d, want 32x32", w, h)
	}
}
