package components

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// writeTestPNG 写一张 w×h 的渐变 PNG，返回路径。
func writeTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	path := filepath.Join(t.TempDir(), "bg.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

type cropperHarness struct {
	tt        *ui.Tester
	open      bool
	image     *CropImage
	confirmed []CropRect
	canceled  int
}

// newCropperHarness 在 1280×800 的窗口里打开 1600×900 的图片；窗口宽高比 16:9。
// 面板宽 896，图片最多 864×480，显示为 853.33×480（缩放 8/15）。
func newCropperHarness(t *testing.T) *cropperHarness {
	t.Helper()
	img, err := LoadCropImage(writeTestPNG(t, 1600, 900))
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != 1600 || img.Height != 900 {
		t.Fatalf("natural size %dx%d", img.Width, img.Height)
	}
	h := &cropperHarness{open: true, image: img}
	th := theme.New(true, 1.5)
	h.tt = ui.NewTester(func(c *ui.Context) {
		ImageCropperModal(c, th, "cropper", ImageCropperProps{
			Open: h.open, Image: h.image, WindowWidth: 1600, WindowHeight: 900, TopOffset: 28,
			OnConfirm: func(r CropRect) { h.confirmed = append(h.confirmed, r) },
			OnCancel:  func() { h.canceled++; h.open = false },
		})
	}, 1280, 800)
	h.tt.SetScale(1.5)
	h.frames(2)
	return h
}

func (h *cropperHarness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *cropperHarness) find(t *testing.T, label string) ui.Rect {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok {
		t.Fatalf("no %q in %q", label, h.tt.Texts())
	}
	return r
}

// drag 按下 (x, y)，分两步移动 (dx, dy) 后松开。
func (h *cropperHarness) drag(x, y, dx, dy float32) {
	h.tt.Press(x, y)
	h.frames(1)
	h.tt.Move(x+dx/2, y+dy/2)
	h.frames(1)
	h.tt.Move(x+dx, y+dy)
	h.frames(1)
	h.tt.Release(x+dx, y+dy)
	h.frames(1)
}

func (h *cropperHarness) confirm(t *testing.T) CropRect {
	t.Helper()
	n := len(h.confirmed)
	if err := h.tt.Click(locales.NativeImageCropperConfirm()); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	if len(h.confirmed) != n+1 {
		t.Fatalf("confirm emitted %d rects", len(h.confirmed)-n)
	}
	r := h.confirmed[n]
	if r.X < 0 || r.Y < 0 || r.Width < 0 || r.Height < 0 || r.X+r.Width > 1600 || r.Y+r.Height > 900 {
		t.Fatalf("rect %+v out of the 1600x900 image", r)
	}
	return r
}

func near(a, b, tol int) bool { return math.Abs(float64(a-b)) <= float64(tol) }

func TestImageCropperInitialCrop(t *testing.T) {
	h := newCropperHarness(t)
	for _, s := range []string{
		locales.NativeImageCropperTitle(), locales.NativeImageCropperAspect(),
		locales.NativeImageCropperAspectWindow("1600", "900"), locales.NativeImageCropperAspectFree(),
		"16:9", "4:3", "1:1", locales.CommonCancel(), locales.NativeImageCropperConfirm(),
	} {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
	stage := h.find(t, locales.NativeImageCropperPreview())
	if math.Abs(float64(stage.W)-853.33) > 0.5 || math.Abs(float64(stage.H)-480) > 0.5 {
		t.Errorf("image shown at %vx%v, want 853.33x480", stage.W, stage.H)
	}
	// 居中、宽 90% 的 16:9 裁剪框：原图里是 (80, 45) 起的 1440×810。
	if r := h.confirm(t); r != (CropRect{X: 80, Y: 45, Width: 1440, Height: 810}) {
		t.Errorf("initial crop %+v", r)
	}
}

func TestImageCropperMoveSelection(t *testing.T) {
	h := newCropperHarness(t)
	sel := h.find(t, locales.NativeImageCropperCropArea())
	// 向右下拖 30×15 DIP：原图里是 56.25×28.125 像素，尺寸不变。
	h.drag(sel.X+sel.W/2, sel.Y+sel.H/2, 30, 15)
	r := h.confirm(t)
	if !near(r.X, 136, 1) || !near(r.Y, 73, 1) || !near(r.Width, 1440, 1) || !near(r.Height, 810, 1) {
		t.Errorf("moved crop %+v, want about {136 73 1440 810}", r)
	}
	// 拖出图片时停在边上。
	sel = h.find(t, locales.NativeImageCropperCropArea())
	h.drag(sel.X+sel.W/2, sel.Y+sel.H/2, 500, 500)
	r = h.confirm(t)
	if r.X+r.Width != 1600 || r.Y+r.Height != 900 || !near(r.Width, 1440, 1) {
		t.Errorf("crop dragged past the corner %+v, want it at the bottom right", r)
	}
}

func TestImageCropperResizeKeepsAspect(t *testing.T) {
	h := newCropperHarness(t)
	sel := h.find(t, locales.NativeImageCropperCropArea())
	// 把右下角的手柄向左上拖 160 DIP：固定 16:9，左上角不动。
	h.drag(sel.X+sel.W, sel.Y+sel.H, -160, -40)
	r := h.confirm(t)
	if r.X != 80 || r.Y != 45 || !near(r.Width, 1440-300, 2) {
		t.Errorf("resized crop %+v, want x=80 y=45 width≈1140", r)
	}
	if got := float64(r.Width) / float64(r.Height); math.Abs(got-16.0/9) > 0.01 {
		t.Errorf("aspect %.3f, want 16:9", got)
	}
}

func TestImageCropperAspectButtons(t *testing.T) {
	h := newCropperHarness(t)
	if err := h.tt.Click("1:1"); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	sel := h.find(t, locales.NativeImageCropperCropArea())
	if math.Abs(float64(sel.W-480)) > 0.5 || math.Abs(float64(sel.H-480)) > 0.5 {
		t.Errorf("1:1 crop %vx%v, want 480x480", sel.W, sel.H)
	}
	// 同原版：换宽高比只改 crop，不调用 onComplete，确认的仍是之前的裁剪框。
	if r := h.confirm(t); r != (CropRect{X: 80, Y: 45, Width: 1440, Height: 810}) {
		t.Errorf("crop after changing the aspect without dragging %+v", r)
	}

	// 自由：按图片的宽高比放 90% 的裁剪框，拖动下边只改高度。
	if err := h.tt.Click(locales.NativeImageCropperAspectFree()); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	sel = h.find(t, locales.NativeImageCropperCropArea())
	stage := h.find(t, locales.NativeImageCropperPreview())
	if math.Abs(float64(sel.W-stage.W*0.9)) > 0.5 || math.Abs(float64(sel.H-stage.H*0.9)) > 0.5 {
		t.Errorf("free crop %vx%v, want 90%% of %vx%v", sel.W, sel.H, stage.W, stage.H)
	}
	h.drag(sel.X+sel.W/2, sel.Y+sel.H, 0, -48)
	r := h.confirm(t)
	if r.X != 80 || r.Y != 45 || r.Width != 1440 || !near(r.Height, 810-90, 1) {
		t.Errorf("crop after dragging the bottom edge %+v, want {80 45 1440 720}", r)
	}
	// 拖动右边的拖动条只改宽度。
	sel = h.find(t, locales.NativeImageCropperCropArea())
	h.drag(sel.X+sel.W, sel.Y+sel.H/4, -64, 30)
	r = h.confirm(t)
	if r.X != 80 || r.Y != 45 || !near(r.Width, 1440-120, 1) || !near(r.Height, 720, 1) {
		t.Errorf("crop after dragging the right edge %+v, want {80 45 1320 720}", r)
	}
}

func TestImageCropperDrawNewCrop(t *testing.T) {
	h := newCropperHarness(t)
	if err := h.tt.Click("1:1"); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	stage := h.find(t, locales.NativeImageCropperPreview())
	sel := h.find(t, locales.NativeImageCropperCropArea())
	// 在裁剪框左边的图片上按下，向右下拖：画一个新的正方形裁剪框。
	x, y := stage.X+(sel.X-stage.X)/2, stage.Y+40
	h.drag(x, y, 60, 90)
	r := h.confirm(t)
	if !near(r.Width, r.Height, 1) || !near(r.Width, 113, 1) {
		t.Errorf("new crop %+v, want a square about 113px wide", r)
	}
	// 只点一下：0×0 的裁剪框不显示，确认发出空的区域（原版同样交给后端，由后端报错）。
	h.tt.ClickAt(stage.X+10, stage.Y+stage.H-10)
	h.frames(1)
	if _, ok := h.tt.Find(locales.NativeImageCropperCropArea()); ok {
		t.Error("an empty crop is not shown")
	}
	if r := h.confirm(t); r.Width != 0 || r.Height != 0 {
		t.Errorf("click without dragging emitted %+v", r)
	}
}

func TestImageCropperKeyboardAndCancel(t *testing.T) {
	h := newCropperHarness(t)
	sel := h.find(t, locales.NativeImageCropperCropArea())
	h.tt.ClickAt(sel.X+sel.W/2, sel.Y+sel.H/2)
	h.frames(1)
	h.tt.Key(ui.Shift, ui.KeyLeft) // 10 DIP，原图里 18.75 像素
	h.frames(1)
	h.tt.Key(0, ui.KeyDown)
	h.frames(1)
	if r := h.confirm(t); !near(r.X, 61, 1) || !near(r.Y, 47, 1) || r.Width != 1440 {
		t.Errorf("crop after the arrow keys %+v, want about {61 47 1440 810}", r)
	}
	if err := h.tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	if h.canceled != 1 || h.tt.HasText(locales.NativeImageCropperTitle()) {
		t.Errorf("cancel: %d calls, texts %q", h.canceled, h.tt.Texts())
	}
	// 关闭按钮同样取消。
	h.open = true
	h.frames(2)
	if err := h.tt.Click(locales.NativeImageCropperClose()); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	if h.canceled != 2 {
		t.Errorf("close button: %d cancels", h.canceled)
	}
}

// 手柄可以用 Tab 聚焦（裁剪框之后依次是各个手柄），方向键按手柄的方向缩放裁剪框（onHandlerKeyDown）。
func TestImageCropperHandleKeyboard(t *testing.T) {
	h := newCropperHarness(t)
	sel := h.find(t, locales.NativeImageCropperCropArea())
	h.tt.ClickAt(sel.X+sel.W/2, sel.Y+sel.H/2)
	h.frames(1)
	// 固定宽高比时只有四角：左上、右上、右下。
	for _, label := range []string{locales.NativeImageCropperHandleNw(), locales.NativeImageCropperHandleNe(), locales.NativeImageCropperHandleSe()} {
		h.tt.Key(0, ui.KeyTab)
		h.frames(1)
		if !h.tt.Focused(label) {
			t.Fatalf("tab did not reach %q", label)
		}
	}
	// 右下角向左 10 DIP：nudgeCrop 让宽、高都减 10（768×432 → 758×422），containCrop 再按 16:9 取较短的一边
	// （422 × 16/9 = 750.2 DIP，原图里 1407 像素），左上角不动。
	h.tt.Key(ui.Shift, ui.KeyLeft)
	h.frames(1)
	r := h.confirm(t)
	if r.X != 80 || r.Y != 45 || !near(r.Width, 1407, 1) || !near(r.Height, 791, 1) {
		t.Errorf("crop after shrinking the south east handle %+v, want about {80 45 1407 791}", r)
	}

	// 自由宽高比：右边的手柄向右 1 DIP 只改宽度。
	if err := h.tt.Click(locales.NativeImageCropperAspectFree()); err != nil {
		t.Fatal(err)
	}
	h.frames(2)
	h.drag(sel.X+sel.W/2, sel.Y+sel.H/2, 0, 0) // 换了宽高比后先确定一次裁剪区域
	before := h.confirm(t)
	h.tt.ClickAt(sel.X+sel.W/2, sel.Y+sel.H/2)
	h.frames(1)
	for range 4 { // 左上、上、右上、右
		h.tt.Key(0, ui.KeyTab)
		h.frames(1)
	}
	if !h.tt.Focused(locales.NativeImageCropperHandleE()) {
		t.Fatal("tab did not reach the east handle")
	}
	h.tt.Key(ui.Cmd, ui.KeyLeft) // 100 DIP
	h.frames(1)
	after := h.confirm(t)
	if after.X != before.X || after.Y != before.Y || after.Height != before.Height || !near(before.Width-after.Width, 188, 1) {
		t.Errorf("east handle: %+v -> %+v, want the width 187.5 pixels narrower", before, after)
	}
}

func TestImageCropperWithoutImage(t *testing.T) {
	h := newCropperHarness(t)
	h.image = nil
	h.open = false
	h.frames(1)
	h.open = true
	h.frames(2)
	if _, ok := h.tt.Find(locales.NativeImageCropperPreview()); ok {
		t.Error("no image is shown while it decodes")
	}
	if err := h.tt.Click(locales.NativeImageCropperConfirm()); err != nil {
		t.Fatal(err)
	}
	h.frames(1)
	if len(h.confirmed) != 0 {
		t.Errorf("confirm without an image emitted %v", h.confirmed)
	}
}
