package better

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// DESKTOP_MYGO_PNG=<目录> 时把图片查看器渲染成图片，用于目测。
func TestComponentDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	th := theme.New(true, 1.5)
	img := image.NewRGBA(image.Rect(0, 0, 300, 420))
	for y := range 420 {
		for x := range 300 {
			img.Set(x, y, color.RGBA{uint8(x * 255 / 300), 120, uint8(y * 255 / 420), 255})
		}
	}
	open := true
	vt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(th.AppBg)
		ImageViewer(c, th, "viewer", ImageViewerProps{Open: &open, Title: "夏日口袋 REFLECTION BLUE", Image: ui.NewBitmap(img), OnSave: func() {}})
	}, 1000, 700)
	vt.SetScale(1.5)
	vt.SetDark(true)
	vt.Frame()
	f, err := os.Create(filepath.Join(dir, "components-viewer.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, vt.Image())
}
