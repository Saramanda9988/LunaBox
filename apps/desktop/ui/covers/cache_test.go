package covers

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
)

// TestDecodeSharesInflight：同一地址同时要两种尺寸（首页选中游戏的卡片封面与大图）时只读取、解码一次，
// 两边拿到同一张原图；解码完以后再要则重新读取（原图不留）。
func TestDecodeSharesInflight(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(100 * time.Millisecond) // 让两次请求重叠
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	cc := New()
	imgs := make([]image.Image, 2)
	var wg sync.WaitGroup
	for i := range imgs {
		wg.Go(func() {
			if err := cc.decode(srv.URL, func(img image.Image, _ string) { imgs[i] = img }); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := hits.Load(); n != 1 {
		t.Fatalf("fetched %d times, want 1", n)
	}
	if imgs[0] == nil || imgs[0] != imgs[1] {
		t.Fatalf("decoded images not shared: %p %p", imgs[0], imgs[1])
	}
	if err := cc.decode(srv.URL, func(image.Image, string) {}); err != nil || hits.Load() != 2 {
		t.Errorf("later decode: err %v, fetched %d times", err, hits.Load())
	}
}

// 每张 16MB，7 张超出 budget（96MB）一张。
func addCovers(cc *Cache, n int) []*models.Game {
	games := make([]*models.Game, n)
	for i := range games {
		games[i] = &models.Game{CoverURL: fmt.Sprintf("/local/covers/%d.jpg", i)}
		cc.finish(result{key: games[i].CoverURL, bitmap: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2048, 2048)))})
	}
	return games
}

func TestEndFrameKeepsCoversUsedThisFrame(t *testing.T) {
	cc := New()
	games := addCovers(cc, 7)
	cc.EndFrame()
	if n, _ := cc.Cached(); n != 7 {
		t.Fatalf("cached %d covers, want all 7: they were added this frame", n)
	}
	for _, g := range games {
		if cc.Get(g) == nil {
			t.Fatalf("%s evicted", g.CoverURL)
		}
	}
}

func TestEndFrameEvictsLeastRecentlyUsed(t *testing.T) {
	cc := New()
	games := addCovers(cc, 7)
	cc.EndFrame()
	cc.Get(games[0]) // 最早加入，但这一帧用过
	cc.EndFrame()
	if n, bytes := cc.Cached(); n != 6 || bytes > budget {
		t.Fatalf("cached %d covers (%d bytes), want 6 within budget", n, bytes)
	}
	if cc.Get(games[0]) == nil {
		t.Fatal("cover used last frame was evicted")
	}
	if cc.Get(games[1]) != nil {
		t.Fatal("least recently used cover was kept")
	}
}

func TestForgetReleasesBytes(t *testing.T) {
	cc := New()
	games := addCovers(cc, 2)
	cc.Forget(games[0])
	if n, bytes := cc.Cached(); n != 1 || bytes != 4*2048*2048 {
		t.Fatalf("cached %d covers (%d bytes) after Forget", n, bytes)
	}
}
