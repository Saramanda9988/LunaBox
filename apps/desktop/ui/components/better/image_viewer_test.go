package better

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

func TestImageViewer(t *testing.T) {
	open, saved := true, 0
	bmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 300, 400)))
	th := theme.New(true, 1)
	tt := ui.NewTester(func(c *ui.Context) {
		ImageViewer(c, th, "viewer", ImageViewerProps{Open: &open, Title: "封面", Image: bmp, OnSave: func() { saved++ }})
	}, 1000, 800)
	tt.Frame()
	for _, s := range []string{"封面", "放大图片", "缩小图片", "恢复图片位置和缩放比例", "另存为"} {
		if _, ok := tt.Find(s); !ok {
			t.Errorf("missing %q", s)
		}
	}
	if err := tt.Click("另存为"); err != nil {
		t.Fatal(err)
	}
	if saved != 1 {
		t.Errorf("saved = %d", saved)
	}
	// 点击图片本身不关闭；点击空白关闭。
	tt.ClickAt(500, 400-56+28)
	tt.Frame()
	if !open {
		t.Fatal("clicking the image closed the viewer")
	}
	tt.ClickAt(100, 400)
	tt.Frame()
	if open {
		t.Fatal("clicking the backdrop did not close the viewer")
	}
	open = true
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if open {
		t.Fatal("Esc did not close the viewer")
	}
}
