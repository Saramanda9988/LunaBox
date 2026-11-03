package stats

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"lunabox/backend/common/vo"
)

// TestPosterLayoutMatchesFrontend 拿原版 posterLayout.ts 算出的排布（testdata/poster_layout.mjs 生成）逐项对照：
// 固定排布、并列时长、领奖台并列与 90 个游戏的长尾，种子取奇偶两种。
func TestPosterLayoutMatchesFrontend(t *testing.T) {
	data, err := os.ReadFile("testdata/poster_layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string          `json:"name"`
		Seed  int             `json:"seed"`
		Input []vo.PosterGame `json:"input"`
		Tiles []struct {
			ID                                  string
			X, Y, Width, Height, Lift, Rotation float64
		} `json:"tiles"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	for _, tc := range cases {
		got := buildPosterLayout(tc.Input, tc.Seed)
		if len(got) != len(tc.Tiles) {
			t.Errorf("%s: %d tiles, want %d", tc.Name, len(got), len(tc.Tiles))
			continue
		}
		for i, w := range tc.Tiles {
			g := got[i]
			if g.game.GameID != w.ID || !near(g.x, w.X) || !near(g.y, w.Y) || !near(g.width, w.Width) || !near(g.height, w.Height) ||
				!near(g.lift, w.Lift) || !near(g.rotation, w.Rotation) {
				t.Errorf("%s tile %d: got %s %.3f,%.3f %.3fx%.3f lift %v rot %v; want %s %.3f,%.3f %.3fx%.3f lift %v rot %v",
					tc.Name, i, g.game.GameID, g.x, g.y, g.width, g.height, g.lift, g.rotation,
					w.ID, w.X, w.Y, w.Width, w.Height, w.Lift, w.Rotation)
				break
			}
		}
	}
}

func TestRankedPosterGames(t *testing.T) {
	got := rankedPosterGames([]vo.PosterGame{
		{GameID: "b", TotalDuration: 10}, {GameID: "a", TotalDuration: 10}, {GameID: "z", TotalDuration: 0},
		{GameID: "c", TotalDuration: 5}, {GameID: "c", TotalDuration: 30},
	})
	var ids []string
	for _, g := range got {
		ids = append(ids, g.GameID)
	}
	if want := []string{"c", "a", "b"}; len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("ranked %v, want %v", ids, want)
	}
}

func TestPosterCrop(t *testing.T) {
	// 1000×500 的横图铺满 100×100 的框：只取中间 500×500；放大 2 倍后取中间 250×250。
	x, y, w, h := posterCoverSource(1000, 500, 100, 100, posterCrop{})
	if x != 250 || y != 0 || w != 500 || h != 500 {
		t.Fatalf("cover source %v %v %v %v", x, y, w, h)
	}
	z := zoomPosterCrop(defaultCrop, 2, 1000, 500, 100, 100)
	if x, y, w, h = posterCoverSource(1000, 500, 100, 100, z); x != 375 || y != 125 || w != 250 || h != 250 {
		t.Fatalf("zoomed source %v %v %v %v (crop %+v)", x, y, w, h, z)
	}
	// 向右拖到头：取景移到最左边；纵向铺满时不能拖。
	p := panPosterCrop(defaultCrop, 1000, 50, 1000, 500, 100, 100)
	if p.x != 0 || p.y != 0.5 {
		t.Fatalf("pan %+v", p)
	}
	if n := normalizeCrop(posterCrop{x: 2, y: -1, zoom: 9}); n != (posterCrop{1, 0, 3}) {
		t.Fatalf("normalize %+v", n)
	}
}

func TestHitPosterTile(t *testing.T) {
	tiles := buildPosterLayout([]vo.PosterGame{{GameID: "a", TotalDuration: 30}, {GameID: "b", TotalDuration: 20}}, 1)
	if hit, ok := hitPosterTile(tiles, 380, 640); !ok || hit.game.GameID != "a" {
		t.Fatalf("hit %v %v", hit.game.GameID, ok)
	}
	if _, ok := hitPosterTile(tiles, 20, 20); ok {
		t.Fatal("hit outside every tile")
	}
}
