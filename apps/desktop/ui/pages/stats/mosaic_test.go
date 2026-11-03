package stats

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/locales"
)

// writeCover 写一张 w×h、左右两半颜色不同的 PNG 封面，返回路径（封面缓存把非 http 的地址当作文件路径）。
func writeCover(t *testing.T, w, h int, left, right color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := left
			if x >= w/2 {
				c = right
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), fmt.Sprintf("cover-%d.png", w*h))
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// posterGames 写入 3 个游戏：前两个有封面（竖图、横图），第三个没有。
func posterGames(t *testing.T) func(svc *host.Backend) {
	return func(svc *host.Backend) {
		yesterday := time.Now().Add(-24 * time.Hour)
		covers := []string{
			writeCover(t, 200, 300, color.RGBA{200, 40, 40, 255}, color.RGBA{40, 40, 200, 255}),
			writeCover(t, 400, 200, color.RGBA{40, 160, 40, 255}, color.RGBA{220, 220, 40, 255}),
			"",
		}
		for i, cover := range covers {
			id := fmt.Sprintf("g%d", i+1)
			hosttest.AddGame(t, svc, models.Game{ID: id, Name: name(i + 1), Company: "Studio", CoverURL: cover})
			hosttest.AddSession(t, svc, id, yesterday, (3-i)*3600)
		}
	}
}

func TestMosaicPanel(t *testing.T) {
	h := newHarness(t, 1100, posterGames(t))
	h.click(t, locales.StatsExportTitle())
	h.frames(2) // 预览区第一帧量尺寸
	m := &h.export.mosaic
	if len(m.games) != 3 || len(m.images) != 2 || !m.ready() {
		t.Fatalf("games %d images %d ready %v", len(m.games), len(m.images), m.ready())
	}
	h.expect(t, locales.StatsTemplateExportMosaicResolution("3"), locales.StatsTemplateExportMosaicMissing("1"),
		locales.StatsTemplateExportMosaicAll("3"), locales.StatsTemplateExportMosaicClickToCrop())
	if m.title != locales.StatsTemplateExportMosaicDefaultTitle() || m.quote != locales.StatsTemplateExportMosaicDefaultQuote() {
		t.Fatalf("title %q quote %q", m.title, m.quote)
	}
	if _, ok := h.tt.Find(locales.StatsTemplateExportMosaicPreview()); !ok {
		t.Fatalf("no poster preview in %q", h.tt.Texts())
	}

	h.click(t, locales.StatsTemplateExportMosaicShuffle())
	if m.seed != 2 {
		t.Fatalf("seed %d after shuffle", m.seed)
	}
	h.click(t, locales.StatsTemplateExportMosaicShowTitles())
	if m.showTitles {
		t.Fatal("titles still shown after the switch")
	}

	// 点击海报里第一名的封面打开取景编辑器。
	r, _ := h.tt.Find(locales.StatsTemplateExportMosaicPreview())
	k := r.W / posterWidth
	first := m.scene.tiles[0]
	h.tt.ClickAt(r.X+float32(first.x+first.width/2)*k, r.Y+float32(first.y+first.height/2)*k)
	h.frames(2)
	if m.editing != first.game.GameID {
		t.Fatalf("editing %q, want %q", m.editing, first.game.GameID)
	}
	h.tt.Advance(editorFade)
	h.frames(2)
	h.expect(t, locales.StatsTemplateExportMosaicBack(), locales.StatsTemplateExportMosaicCropHint(), "1.00×")
	if h.export.mosaic.ready() {
		t.Fatal("export enabled while editing")
	}

	// 拖动平移、滚轮放大、恢复居中。
	cr, ok := h.tt.Find(locales.StatsTemplateExportMosaicCropCanvas())
	if !ok {
		t.Fatalf("no crop canvas in %q", h.tt.Texts())
	}
	cx, cy := cr.X+cr.W/2, cr.Y+cr.H/2
	h.tt.Wheel(cx, cy, 0, -400)
	h.frames(1)
	if z := m.crops[first.game.GameID].zoom; z <= 1 {
		t.Fatalf("zoom %v after wheel", z)
	}
	before := m.crops[first.game.GameID]
	h.tt.Press(cx, cy)
	h.tt.Move(cx+30, cy+30)
	h.tt.Release(cx+30, cy+30)
	h.frames(1)
	if after := m.crops[first.game.GameID]; after.x >= before.x || after.y >= before.y {
		t.Fatalf("pan: before %+v after %+v", before, after)
	}
	h.click(t, locales.StatsTemplateExportMosaicResetCrop())
	if c := m.crops[first.game.GameID]; c != defaultCrop {
		t.Fatalf("reset crop %+v", c)
	}

	// 下一张在有封面的两个游戏之间循环；Esc 关闭（淡出后）。
	h.click(t, locales.StatsTemplateExportMosaicNext())
	if m.editing == first.game.GameID || m.images[m.editing] == nil {
		t.Fatalf("next: editing %q", m.editing)
	}
	h.click(t, locales.StatsTemplateExportMosaicNext())
	if m.editing != first.game.GameID {
		t.Fatalf("next wraps to %q", m.editing)
	}
	h.tt.Key(0, ui.KeyEscape)
	h.frames(1)
	h.tt.Advance(editorFade)
	h.frames(2)
	if m.editing != "" {
		t.Fatalf("editor still open: %q", m.editing)
	}
	if !m.ready() {
		t.Fatal("not ready after closing the editor")
	}

	// 显示数量：前 6 个仍是全部 3 个；切换模板后丢掉封面。
	h.click(t, h.export.templates[0].Name)
	if m.games != nil || m.images != nil {
		t.Fatal("mosaic kept its covers after switching template")
	}
}

// TestPosterPNG 按 3 倍导出：3240×4320，第一名的封面画在它的框里，没有封面的框是占位色。
func TestPosterPNG(t *testing.T) {
	h := newHarness(t, 1100, posterGames(t))
	h.click(t, locales.StatsExportTitle())
	m := &h.export.mosaic
	var shaper posterShaper
	sc := shaper.buildPosterScene(m.selectedGames(), m.images, h.posterOptions(m.selectedGames()))
	start := time.Now()
	data, err := renderPosterPNG(sc)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rendered %d KB in %v", len(data)/1024, time.Since(start))
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != posterWidth*posterExportScale || b.Dy() != posterHeight*posterExportScale {
		t.Fatalf("size %v", b)
	}
	// DESKTOP_MYGO_PNG=<目录> 时另存导出的海报与弹窗的截图，用于目测。
	if dir := os.Getenv("DESKTOP_MYGO_PNG"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "poster.png"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		h.frames(2)
		f, err := os.Create(filepath.Join(dir, "poster-modal.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	// 第一名（竖图，左红右蓝）框的左上四分之一处应该偏红。
	first := sc.tiles[0]
	px := int((first.x + first.width*0.2) * posterExportScale)
	py := int((first.y + first.height*0.3) * posterExportScale)
	r, g, b, _ := img.At(px, py).RGBA()
	if r <= g || r <= b {
		t.Fatalf("pixel at first cover is %v,%v,%v", r>>8, g>>8, b>>8)
	}
}
