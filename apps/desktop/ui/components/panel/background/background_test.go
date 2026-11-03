package background

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/appruntime"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// fakeRuntime 让 SelectAndCropBackgroundImage 的文件对话框直接返回 file（或 err），不弹真实的对话框。
type fakeRuntime struct {
	appruntime.Runtime
	file string
	err  error
}

func (r *fakeRuntime) OpenFile(appruntime.OpenDialogOptions) (string, error) { return r.file, r.err }

type harness struct {
	svc     *host.Backend
	env     *env.Env
	rt      *fakeRuntime
	cfg     appconf.AppConfig
	state   State
	tt      *ui.Tester
	changes int
}

// newHarness 在真实服务上打开面板：Change 与 PatchLive 改本地的配置，外面套一层设置分区的 p-5 space-y-4。
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t), rt: &fakeRuntime{Runtime: appruntime.Unavailable()}}
	h.svc.SetRuntime(h.rt)
	h.cfg = *h.svc.Config
	h.cfg.WindowWidth, h.cfg.WindowHeight = 1600, 900
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 1400)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(2)
	return h
}

func (h *harness) props() panel.Settings {
	change := func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg); h.changes++ }
	return panel.Settings{Env: h.env, Config: &h.cfg, Change: change, PatchLive: change, Refresh: func() {}}
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Padding(20).Gap(16).Background(t.PanelBg).Children(func() {
		View(c, t, &h.state, h.props())
	})
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) find(t *testing.T, s string) ui.Rect {
	t.Helper()
	r, ok := h.tt.Find(s)
	if !ok {
		t.Fatalf("no %q in %q", s, h.tt.Texts())
	}
	return r
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

// solidPNG 写一张单色 PNG，返回路径。
func solidPNG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	path := filepath.Join(t.TempDir(), "pick.png")
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

func TestRendersSections(t *testing.T) {
	h := newHarness(t)
	h.expect(t,
		locales.SettingsAppearanceBackgroundSection(), locales.SettingsAppearanceEnableBg(), locales.SettingsAppearanceEnableBgHint(),
		locales.SettingsAppearanceBgImage(), locales.SettingsAppearanceBgImageHint(),
		locales.SettingsAppearanceBlurLabel(), "10px", locales.SettingsAppearanceBlurSharp(), locales.SettingsAppearanceBlurBlurry(),
		locales.SettingsAppearanceOpacityLabel(), "85%", locales.SettingsAppearanceOpacityHint(),
		locales.SettingsAppearanceHomeSection(), locales.SettingsAppearanceHideGameCover(), locales.SettingsAppearanceHideGameHeroCover(),
		locales.SettingsAppearanceHomeGameCarousel(), locales.SettingsAppearanceHomeGameCarouselInterval(),
		locales.SettingsAppearanceHomeGameCarouselIntervalHint("5"), locales.SettingsAppearanceHomeGameCarouselIntervalUnit(),
		locales.SettingsAppearanceLibrarySection(), locales.SettingsAppearanceBlurNsfwCovers(),
		locales.SettingsAppearanceGameCardLayout(), locales.SettingsAppearanceGameCardLayoutPortrait(),
		locales.SettingsAppearanceGameCardLayoutHint(),
	)
	if _, ok := h.tt.Find(locales.SettingsAppearanceClearBtn()); ok {
		t.Error("no clear button without a background image")
	}
}

func TestBackgroundSwitches(t *testing.T) {
	h := newHarness(t)
	// 没有背景图时“启用背景”禁用，依赖它的两个开关也禁用。
	h.click(t, locales.SettingsAppearanceEnableBg())
	h.click(t, locales.SettingsAppearanceHideGameCover())
	if h.cfg.BackgroundEnabled || h.cfg.BackgroundHideGameCover || h.changes != 0 {
		t.Fatalf("disabled switches changed the config: %+v (%d changes)", h.cfg, h.changes)
	}

	h.cfg.BackgroundImage, h.cfg.BackgroundIsLight, h.cfg.Theme = "/local/backgrounds/custom_bg_1.png", false, "light"
	h.frames(2)
	h.click(t, locales.SettingsAppearanceEnableBg())
	if !h.cfg.BackgroundEnabled || h.cfg.Theme != "dark" {
		t.Errorf("enabling a dark background: enabled %v theme %q", h.cfg.BackgroundEnabled, h.cfg.Theme)
	}
	h.click(t, locales.SettingsAppearanceHideGameCover())
	h.click(t, locales.SettingsAppearanceHideGameHeroCover())
	if !h.cfg.BackgroundHideGameCover || !h.cfg.BackgroundHideGameHeroCover {
		t.Errorf("hide cover switches: %v %v", h.cfg.BackgroundHideGameCover, h.cfg.BackgroundHideGameHeroCover)
	}
	// 关闭不改主题。
	h.click(t, locales.SettingsAppearanceEnableBg())
	if h.cfg.BackgroundEnabled || h.cfg.Theme != "dark" {
		t.Errorf("disabling: enabled %v theme %q", h.cfg.BackgroundEnabled, h.cfg.Theme)
	}
	h.cfg.BackgroundIsLight = true
	h.click(t, locales.SettingsAppearanceEnableBg())
	if h.cfg.Theme != "light" {
		t.Errorf("enabling a light background: theme %q", h.cfg.Theme)
	}

	// 清除背景图同时关闭背景。
	h.click(t, locales.SettingsAppearanceClearBtn())
	if h.cfg.BackgroundImage != "" || h.cfg.BackgroundEnabled {
		t.Errorf("clear: image %q enabled %v", h.cfg.BackgroundImage, h.cfg.BackgroundEnabled)
	}

	h.click(t, locales.SettingsAppearanceBlurNsfwCovers())
	if h.cfg.BlurNSFWGameCovers {
		t.Error("blur NSFW covers switch")
	}
}

func TestCarouselInterval(t *testing.T) {
	h := newHarness(t)
	// 轮播间隔为 0 时按前端的默认值 7 显示，↑ 从 7 加一。
	h.cfg.HomeGameCarouselIntervalSec = 0
	h.frames(1)
	unit := h.find(t, locales.SettingsAppearanceHomeGameCarouselIntervalUnit())
	h.tt.ClickAt(unit.X-8-50, unit.Y+unit.H/2)
	h.frames(1)
	h.tt.Key(0, ui.KeyUp)
	h.frames(2)
	if h.cfg.HomeGameCarouselIntervalSec != 8 {
		t.Errorf("arrow up from the default: %d", h.cfg.HomeGameCarouselIntervalSec)
	}
	// 输入框在单位左边（gap-2，w-25）。
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type("12")
	h.frames(2)
	if h.cfg.HomeGameCarouselIntervalSec != 12 {
		t.Errorf("typed interval: %d", h.cfg.HomeGameCarouselIntervalSec)
	}
	h.tt.Key(0, ui.KeyUp)
	h.frames(2)
	if h.cfg.HomeGameCarouselIntervalSec != 13 {
		t.Errorf("arrow up: %d", h.cfg.HomeGameCarouselIntervalSec)
	}

	h.click(t, locales.SettingsAppearanceHomeGameCarousel())
	if h.cfg.HomeGameCarouselEnabled {
		t.Fatal("carousel switch did not turn it off")
	}
	// 关闭轮播后间隔禁用。
	h.tt.ClickAt(unit.X-8-50, unit.Y+unit.H/2)
	h.frames(1)
	h.tt.Key(0, ui.KeyUp)
	h.frames(2)
	if h.cfg.HomeGameCarouselIntervalSec != 13 {
		t.Errorf("disabled interval changed to %d", h.cfg.HomeGameCarouselIntervalSec)
	}
}

func TestGameCardLayoutSelect(t *testing.T) {
	h := newHarness(t)
	h.click(t, locales.SettingsAppearanceGameCardLayoutPortrait())
	h.click(t, locales.SettingsAppearanceGameCardLayoutLandscape())
	if h.cfg.GameCardLayout != "landscape" {
		t.Errorf("layout %q", h.cfg.GameCardLayout)
	}
	// 未知的值按竖版显示。
	h.cfg.GameCardLayout = "unknown"
	h.frames(2)
	h.expect(t, locales.SettingsAppearanceGameCardLayoutPortrait())
}

// sliderAt 返回滑块上 frac 处的坐标：滑块在两端说明的上方 14px（行内滑块的行盒，见 rangeInput），高 8，
// 拇指中心在两端各缩进 8。
func (h *harness) sliderAt(t *testing.T, minLabel, maxLabel string, frac float32) (float32, float32) {
	t.Helper()
	lo, hi := h.find(t, minLabel), h.find(t, maxLabel)
	left, right := lo.X, hi.X+hi.W
	return left + 8 + (right-left-16)*frac, lo.Y - 14 - 4
}

func TestSliders(t *testing.T) {
	h := newHarness(t)
	x, y := h.sliderAt(t, locales.SettingsAppearanceBlurSharp(), locales.SettingsAppearanceBlurBlurry(), 0.5)
	h.tt.ClickAt(x, y)
	h.frames(2)
	if h.cfg.BackgroundBlur != 10 || h.changes != 0 {
		t.Fatalf("disabled blur slider changed the blur to %d", h.cfg.BackgroundBlur)
	}

	h.cfg.BackgroundImage = "/local/backgrounds/custom_bg_1.png"
	h.frames(2)
	h.tt.ClickAt(x, y)
	h.frames(2)
	if h.cfg.BackgroundBlur != 15 {
		t.Errorf("blur after clicking the middle: %d", h.cfg.BackgroundBlur)
	}
	h.expect(t, "15px")

	x, y = h.sliderAt(t, locales.SettingsAppearanceOpacityTransparent(), locales.SettingsAppearanceOpacityOpaque(), 1)
	h.tt.ClickAt(x, y)
	h.frames(2)
	if h.cfg.BackgroundOpacity != 1 {
		t.Errorf("opacity at the right end: %v", h.cfg.BackgroundOpacity)
	}
	x, _ = h.sliderAt(t, locales.SettingsAppearanceOpacityTransparent(), locales.SettingsAppearanceOpacityOpaque(), 0.5)
	h.tt.ClickAt(x, y)
	h.frames(2)
	if h.cfg.BackgroundOpacity != 0.65 {
		t.Errorf("opacity in the middle: %v, want 0.65", h.cfg.BackgroundOpacity)
	}
	h.expect(t, "65%")
}

func TestSelectAndCropBackground(t *testing.T) {
	h := newHarness(t)
	h.rt.file = solidPNG(t, 320, 180, color.NRGBA{240, 240, 230, 255})
	h.click(t, locales.SettingsAppearanceSelectBtn())
	h.expect(t, locales.NativeImageCropperTitle(), locales.NativeImageCropperAspectWindow("1600", "900"))
	if !strings.HasPrefix(h.state.selectedPath, "/local/backgrounds/temp_bg_") || h.state.image == nil {
		t.Fatalf("cropper opened with %q, image %v", h.state.selectedPath, h.state.image)
	}

	// 取消：关闭弹窗，配置不变。
	h.click(t, locales.CommonCancel())
	if h.tt.HasText(locales.NativeImageCropperTitle()) || h.changes != 0 {
		t.Fatalf("cancel: texts %q, %d changes", h.tt.Texts(), h.changes)
	}

	h.cfg.Theme = "dark"
	h.click(t, locales.SettingsAppearanceSelectBtn())
	h.click(t, locales.NativeImageCropperConfirm())
	if h.tt.HasText(locales.NativeImageCropperTitle()) {
		t.Error("cropper still open after confirming")
	}
	if !strings.HasPrefix(h.cfg.BackgroundImage, "/local/backgrounds/custom_bg_") || !h.cfg.BackgroundIsLight {
		t.Fatalf("saved image %q light %v", h.cfg.BackgroundImage, h.cfg.BackgroundIsLight)
	}
	// 裁剪后的图片是 16:9 窗口比例的 90%：288×162。
	f, err := os.Open(localFile(h.svc.DataDir, h.cfg.BackgroundImage))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != 288 || cfg.Height != 162 {
		t.Errorf("cropped image %dx%d (%v)", cfg.Width, cfg.Height, err)
	}
	if h.cfg.BackgroundEnabled || h.cfg.Theme != "dark" {
		t.Error("cropping alone does not enable the background or change the theme")
	}

	// 深色的图片记为暗色背景。
	h.rt.file = solidPNG(t, 320, 180, color.NRGBA{20, 30, 40, 255})
	h.click(t, locales.SettingsAppearanceSelectBtn())
	h.click(t, locales.NativeImageCropperConfirm())
	if h.cfg.BackgroundIsLight {
		t.Error("dark image detected as light")
	}
}

func TestSelectAndCropFailures(t *testing.T) {
	h := newHarness(t)
	h.rt.err = errors.New("dialog failed")
	h.click(t, locales.SettingsAppearanceSelectBtn())
	h.expect(t, locales.SettingsAppearanceToastSelectFailed("dialog failed"))
	if h.state.showCropper {
		t.Error("cropper opened after a failed dialog")
	}

	// 用户取消对话框：什么也不发生。
	h.rt.err = nil
	h.click(t, locales.SettingsAppearanceSelectBtn())
	if h.state.showCropper {
		t.Error("cropper opened without a selection")
	}

	// 裁剪失败：提示，弹窗保持打开。
	h.rt.file = solidPNG(t, 64, 64, color.White)
	h.click(t, locales.SettingsAppearanceSelectBtn())
	_ = os.Remove(localFile(h.svc.DataDir, h.state.selectedPath))
	h.click(t, locales.NativeImageCropperConfirm())
	if !h.state.showCropper || h.changes != 0 {
		t.Errorf("failed crop: open %v, %d changes", h.state.showCropper, h.changes)
	}
	if !strings.Contains(strings.Join(h.tt.Texts(), "\n"), strings.TrimSpace(locales.SettingsAppearanceToastCropFailed(""))) {
		t.Errorf("missing crop failure toast in %q", h.tt.Texts())
	}
}

func TestBrightness(t *testing.T) {
	solid := func(w, h int, c color.Color) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				img.Set(x, y, c)
			}
		}
		return img
	}
	if !IsLightImage(solid(400, 300, color.White)) || IsLightImage(solid(400, 300, color.Black)) {
		t.Error("white is light, black is dark")
	}
	// 平方平均：一半黑一半白是 sqrt(255²/2) ≈ 180，按亮度算是亮色（算术平均 127.5 会是暗色）。
	half := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	for y := range 200 {
		for x := range 200 {
			if x < 100 {
				half.Set(x, y, color.Black)
			} else {
				half.Set(x, y, color.White)
			}
		}
	}
	if c := averageColor(half); c[0] < 175 || c[0] > 185 || c[3] != 255 {
		t.Errorf("average of black and white %v, want about 180", c)
	}
	if !IsLightImage(half) {
		t.Error("half black, half white is light by the sqrt average")
	}
	// 0.299R + 0.587G + 0.114B：纯绿亮、纯蓝暗。
	if !IsLightImage(solid(50, 50, color.NRGBA{0, 255, 0, 255})) || IsLightImage(solid(50, 50, color.NRGBA{0, 0, 255, 255})) {
		t.Error("green is light, blue is dark")
	}
	// 透明的像素不参与平均。
	clear := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			if y < 50 {
				clear.Set(x, y, color.NRGBA{0, 0, 0, 0})
			} else {
				clear.Set(x, y, color.White)
			}
		}
	}
	if c := averageColor(clear); c[0] != 255 || !IsLightImage(clear) {
		t.Errorf("transparent pixels counted: %v", c)
	}
	// 小于 MIN_SIZE 的缩放结果改用原尺寸。
	if c := averageColor(solid(1000, 5, color.NRGBA{10, 20, 30, 255})); c != [4]float64{10, 20, 30, 255} {
		t.Errorf("thin image average %v", c)
	}
	path := solidPNG(t, 30, 20, color.NRGBA{250, 250, 250, 255})
	if light, err := DetectImageBrightness(path); err != nil || !light {
		t.Errorf("DetectImageBrightness = %v, %v", light, err)
	}
	if _, err := DetectImageBrightness(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("missing file has no error")
	}
}

func TestFileNameAndLocalFile(t *testing.T) {
	for in, want := range map[string]string{"": "", "/local/backgrounds/a.png": "a.png", `C:\x\b.jpg`: "b.jpg", "c.webp": "c.webp"} {
		if got := fileName(in); got != want {
			t.Errorf("fileName(%q) = %q, want %q", in, got, want)
		}
	}
	if got, want := localFile(`D:\data`, "/local/backgrounds/a.png"), filepath.Join(`D:\data`, "backgrounds", "a.png"); got != want {
		t.Errorf("localFile = %q, want %q", got, want)
	}
	if got := localFile(`D:\data`, `E:\pic.png`); got != `E:\pic.png` {
		t.Errorf("plain path changed to %q", got)
	}
}
