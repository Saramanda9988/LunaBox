package covers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gen2brain/avif"
)

// testAVIF 用 gen2brain/avif 编码一张 w×h 的渐变图。
func testAVIF(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := avif.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newDiskTest 返回数据目录里有一张封面 covers/<name> 的 Cache，磁盘缓存在临时目录。
func newDiskTest(t *testing.T, name string, data []byte) (cc *Cache, src string) {
	t.Helper()
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "covers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "covers", name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	cc = New()
	cc.dataDir, cc.disk = dataDir, &diskCache{dir: t.TempDir()}
	return cc, "/local/covers/" + name
}

// TestDiskCacheAVIF：AVIF 封面缩小后存进磁盘缓存，下次直接读小图（不再解码）；换了封面（同一地址）不用旧的。
func TestDiskCacheAVIF(t *testing.T) {
	cc, src := newDiskTest(t, "a.avif", testAVIF(t, 800, 400))
	req := request{key: src, src: src, width: maxWidth}

	var res result
	save, err := cc.load(req, src, &res)
	if err != nil || save == nil {
		t.Fatalf("first load: save %v, err %v", save != nil, err)
	}
	save()
	if w, h := res.bitmap.Size(); w != maxWidth || h != 180 {
		t.Fatalf("decoded size %d×%d", w, h)
	}

	res = result{}
	save, err = cc.load(req, src, &res)
	if err != nil || save != nil {
		t.Fatalf("second load not from disk: save %v, err %v", save != nil, err)
	}
	if w, h := res.bitmap.Size(); w != maxWidth || h != 180 {
		t.Errorf("cached size %d×%d", w, h)
	}

	// 大图是另一份。
	large := request{key: src + largeSuffix, src: src, width: largeWidth, height: largeHeight, accent: true}
	res = result{}
	if save, _ = cc.load(large, src, &res); save == nil || !res.hasAccent {
		t.Errorf("large request used the card-sized cache: save %v accent %v", save != nil, res.hasAccent)
	}

	// 换封面：文件改了，修改时间也变了。
	path := filepath.Join(cc.dataDir, "covers", "a.avif")
	if err := os.WriteFile(path, testAVIF(t, 400, 400), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	os.Chtimes(path, later, later)
	res = result{}
	if save, err = cc.load(req, src, &res); err != nil || save == nil {
		t.Fatalf("replaced cover served from cache: save %v, err %v", save != nil, err)
	}
	if w, h := res.bitmap.Size(); w != maxWidth || h != maxWidth {
		t.Errorf("replaced cover size %d×%d", w, h)
	}
}

// TestDiskCacheSkipsOtherFormats：PNG 等解码本来就快的格式不写磁盘缓存。
func TestDiskCacheSkipsOtherFormats(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 500, 500)))
	cc, src := newDiskTest(t, "a.png", buf.Bytes())
	var res result
	save, err := cc.load(request{key: src, src: src, width: maxWidth}, src, &res)
	if err != nil || save != nil || res.bitmap == nil {
		t.Errorf("png: save %v, err %v, bitmap %v", save != nil, err, res.bitmap != nil)
	}
}

// TestDiskCachePrune：超出上限时从最久没用的删起，删到上限的 80%。
func TestDiskCachePrune(t *testing.T) {
	d := &diskCache{dir: t.TempDir()}
	now := time.Now()
	for i, name := range []string{"old", "mid", "new"} {
		path := filepath.Join(d.dir, name)
		os.WriteFile(path, make([]byte, 100), 0o644)
		at := now.Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(path, at, at)
	}
	d.prune(200) // 300 字节超出 200，删到 160 以下：删掉 old 与 mid
	for name, want := range map[string]bool{"old": false, "mid": false, "new": true} {
		if _, err := os.Stat(filepath.Join(d.dir, name)); (err == nil) != want {
			t.Errorf("%s exists = %v, want %v", name, err == nil, want)
		}
	}
}
