// Package background 对应原版 components/panel/BackgroundSettingsPanel.tsx（设置页“外观”分区）：自定义背景图
// （选择后在 ImageCropperModal 里裁剪）、模糊度、不透明度，首页与游戏库的外观选项。
package background

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// 与 BackgroundSettingsPanel.tsx 相同的常量（原版前端的取值，与后端 appconf 的默认值不同）。
const (
	defaultCarouselIntervalSec = 7 // DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC
	minCarouselIntervalSec     = 5 // MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC
	defaultWindowWidth         = 1134
	defaultWindowHeight        = 750
)

// gameCardLayouts 是 GAME_CARD_LAYOUTS。
var gameCardLayouts = []string{"portrait", "landscape"}

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	selectedPath string                // selectedImagePath：SelectAndCropBackgroundImage 返回的 /local/backgrounds/temp_bg_*
	showCropper  bool                  // showCropper
	image        *components.CropImage // 裁剪弹窗里的图片（原版 <img src={imagePath}> 的加载）
	gen          int                   // 每次打开裁剪弹窗加一，丢弃过期的加载与裁剪结果
	toasts       []env.Notice          // 后台结果里待显示的提示（apply 拿不到 Context）
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// View 构建面板的内容（CollapsibleSection 的 space-y-4 列里的各项）：原版根元素 space-y-4 下的三个 section，最后是裁剪弹窗。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil
	cfg := p.Config

	// section.space-y-4：背景图
	ui.Column(c.Key("background-section")).FillWidth().Gap(16).Children(func() {
		panel.GroupTitle(c, t, locales.SettingsAppearanceBackgroundSection())
		better.SettingSwitchRow(c, t, "background_enabled", better.SettingSwitchRowProps{
			Label:    locales.SettingsAppearanceEnableBg(),
			Hint:     locales.SettingsAppearanceEnableBgHint(),
			Checked:  cfg.BackgroundEnabled,
			Disabled: cfg.BackgroundImage == "",
			OnChange: func(checked bool) {
				p.Change(func(cfg *appconf.AppConfig) {
					cfg.BackgroundEnabled = checked
					// 原版：checked && background_is_light !== undefined 时按背景的明暗切换主题。
					if checked {
						if cfg.BackgroundIsLight {
							cfg.Theme = "light"
						} else {
							cfg.Theme = "dark"
						}
					}
				})
			},
		})
		panel.Field(c, t, "background_image", locales.SettingsAppearanceBgImage(), locales.SettingsAppearanceBgImageHint(), func() {
			s.imageInput(c, t, p)
		})
		blur := cfg.BackgroundBlur
		sliderField(c, t, "background_blur", sliderProps{
			label: locales.SettingsAppearanceBlurLabel(), value: strconv.Itoa(blur) + "px",
			current: float64(blur), lo: 0, hi: 30, step: 1, disabled: cfg.BackgroundImage == "",
			minLabel: locales.SettingsAppearanceBlurSharp(), maxLabel: locales.SettingsAppearanceBlurBlurry(),
			onChange: func(v float64) {
				n := int(math.Round(v))
				p.Change(func(cfg *appconf.AppConfig) { cfg.BackgroundBlur = n })
			},
		})
		opacity := cfg.BackgroundOpacity
		sliderField(c, t, "background_opacity", sliderProps{
			label: locales.SettingsAppearanceOpacityLabel(), value: strconv.Itoa(int(jsRound(opacity*100))) + "%",
			current: opacity, lo: 0.3, hi: 1, step: 0.05, disabled: cfg.BackgroundImage == "",
			minLabel: locales.SettingsAppearanceOpacityTransparent(), maxLabel: locales.SettingsAppearanceOpacityOpaque(),
			hint: locales.SettingsAppearanceOpacityHint(),
			onChange: func(v float64) {
				v = math.Round(v*100) / 100 // parseFloat(e.target.value)：输入框的值是按 step 取整后的小数
				p.Change(func(cfg *appconf.AppConfig) { cfg.BackgroundOpacity = v })
			},
		})
	})

	// section.space-y-4 border-t border-brand-200 pt-4 dark:border-brand-700：首页
	panel.Divided(c, t, "home-appearance-section", 16, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			panel.GroupTitle(c, t, locales.SettingsAppearanceHomeSection())
			better.SettingSwitchRow(c, t, "background_hide_game_cover", better.SettingSwitchRowProps{
				Label:    locales.SettingsAppearanceHideGameCover(),
				Hint:     locales.SettingsAppearanceHideGameCoverHint(),
				Checked:  cfg.BackgroundHideGameCover,
				Disabled: !cfg.BackgroundEnabled,
				OnChange: func(v bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.BackgroundHideGameCover = v }) },
			})
			better.SettingSwitchRow(c, t, "background_hide_game_hero_cover", better.SettingSwitchRowProps{
				Label:    locales.SettingsAppearanceHideGameHeroCover(),
				Hint:     locales.SettingsAppearanceHideGameHeroCoverHint(),
				Checked:  cfg.BackgroundHideGameHeroCover,
				Disabled: !cfg.BackgroundEnabled,
				OnChange: func(v bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.BackgroundHideGameHeroCover = v }) },
			})
			better.SettingSwitchRow(c, t, "home_game_carousel_enabled", better.SettingSwitchRowProps{
				Label:    locales.SettingsAppearanceHomeGameCarousel(),
				Hint:     locales.SettingsAppearanceHomeGameCarouselHint(),
				Checked:  cfg.HomeGameCarouselEnabled,
				OnChange: func(v bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.HomeGameCarouselEnabled = v }) },
			})
			carouselInterval(c, t, p)
		})
	})

	// section.space-y-4 border-t pt-4：游戏库
	panel.Divided(c, t, "library-appearance-section", 16, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			panel.GroupTitle(c, t, locales.SettingsAppearanceLibrarySection())
			better.SettingSwitchRow(c, t, "blur_nsfw_game_covers", better.SettingSwitchRowProps{
				Label:    locales.SettingsAppearanceBlurNsfwCovers(),
				Hint:     locales.SettingsAppearanceBlurNsfwCoversHint(),
				Checked:  cfg.BlurNSFWGameCovers,
				OnChange: func(v bool) { p.Change(func(cfg *appconf.AppConfig) { cfg.BlurNSFWGameCovers = v }) },
			})
			layout := "portrait"
			if cfg.GameCardLayout == "landscape" {
				layout = "landscape"
			}
			panel.Field(c, t, "game_card_layout", locales.SettingsAppearanceGameCardLayout(), locales.SettingsAppearanceGameCardLayoutHint(), func() {
				options := make([]better.SelectOption, 0, len(gameCardLayouts))
				for _, l := range gameCardLayouts {
					options = append(options, better.SelectOption{Value: l, Label: locales.Key("settings.appearance.gameCardLayout_" + l)})
				}
				better.Select(c, t, "select", better.SelectProps{
					Value:    layout,
					Options:  options,
					OnChange: func(v string) { p.Change(func(cfg *appconf.AppConfig) { cfg.GameCardLayout = v }) },
				})
			})
		})
	})

	// 裁剪弹窗（原版放在 Fragment 的最前面，由 ModalPortal 盖在页面之上；MyGo 的浮层要最后构建）。
	ww, wh := cfg.WindowWidth, cfg.WindowHeight
	if ww == 0 {
		ww = defaultWindowWidth
	}
	if wh == 0 {
		wh = defaultWindowHeight
	}
	components.ImageCropperModal(c, t, "background-cropper", components.ImageCropperProps{
		Open:         s.showCropper && s.selectedPath != "",
		Image:        s.image,
		WindowWidth:  ww,
		WindowHeight: wh,
		TopOffset:    panel.TopOffset,
		OnConfirm:    func(r components.CropRect) { s.confirmCrop(p, r) },
		OnCancel:     s.closeCropper,
	})
}

// imageInput 对应背景图的 BetterActionInput：只读，显示文件名（text-sm text-brand-600 dark:text-brand-400），右侧“选择”与
// 已有背景图时的“清除”。
func (s *State) imageInput(c *ui.Context, t *theme.Theme, p panel.Settings) {
	name := fileName(p.Config.BackgroundImage)
	actions := []better.InputAction{{
		Icon:    "image-search-outline",
		Label:   locales.SettingsAppearanceSelectBtn(),
		OnClick: func() { s.selectImage(p) },
	}}
	if p.Config.BackgroundImage != "" {
		actions = append(actions, better.InputAction{
			Icon:  "close",
			Label: locales.SettingsAppearanceClearBtn(),
			// handleClearImage
			OnClick: func() {
				p.Change(func(cfg *appconf.AppConfig) {
					cfg.BackgroundImage = ""
					cfg.BackgroundEnabled = false
				})
			},
		})
	}
	in := better.ActionInput(c, t, "input", &name, better.InputProps{
		Placeholder: locales.SettingsAppearanceNoImageSelected(),
		Label:       locales.SettingsAppearanceBgImage(),
	}, actions...)
	in.ReadOnly(true).FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.Pick(theme.Brand600, theme.Brand400))
}

// fileName 对应 getFileName：路径的最后一段。
func fileName(path string) string {
	if path == "" {
		return ""
	}
	return path[strings.LastIndexAny(path, `/\`)+1:]
}

// selectImage 对应 handleSelectImage：打开文件对话框（会阻塞，放到后台），选中后打开裁剪弹窗。
func (s *State) selectImage(p panel.Settings) {
	e := p.Env
	e.Async(func() func() {
		path, err := e.Svc.AppConfig.SelectAndCropBackgroundImage()
		return func() {
			if err != nil {
				s.toast(env.Error(locales.SettingsAppearanceToastSelectFailed(err.Error())))
				return
			}
			if path != "" {
				s.openCropper(e, path)
			}
		}
	})
}

// openCropper 是 setSelectedImagePath + setShowCropper(true)，并在后台解码图片（原版由 <img> 加载）。
// 解码失败时裁剪区保持空白，同原版图片加载失败。
func (s *State) openCropper(e *env.Env, path string) {
	s.gen++
	s.selectedPath, s.showCropper, s.image = path, true, nil
	gen, file := s.gen, localFile(e.Svc.DataDir, path)
	e.Async(func() func() {
		img, err := components.LoadCropImage(file)
		return func() {
			if gen == s.gen && err == nil {
				s.image = img
			}
		}
	})
}

// closeCropper 对应 handleCropCancel。
func (s *State) closeCropper() {
	s.gen++
	s.showCropper, s.selectedPath, s.image = false, "", nil
}

// confirmCrop 对应 handleCropConfirm：保存裁剪后的图片、检测明暗（文件读写与解码放到后台），写进配置后关闭弹窗；
// 失败时提示并保持弹窗打开。
func (s *State) confirmCrop(p panel.Settings, r components.CropRect) {
	e, gen, src := p.Env, s.gen, s.selectedPath
	e.Async(func() func() {
		local, err := e.Svc.AppConfig.SaveCroppedBackgroundImage(src, r.X, r.Y, r.Width, r.Height)
		isLight := false
		if err == nil && local != "" {
			isLight, err = DetectImageBrightness(localFile(e.Svc.DataDir, local))
		}
		return func() {
			if err != nil {
				s.toast(env.Error(locales.SettingsAppearanceToastCropFailed(err.Error())))
				return
			}
			if local != "" {
				p.Change(func(cfg *appconf.AppConfig) {
					cfg.BackgroundImage = local
					cfg.BackgroundIsLight = isLight
				})
			}
			if gen == s.gen {
				s.closeCropper()
			}
		}
	})
}

// carouselInterval 对应轮播间隔：flex items-center justify-between gap-4，左侧 flex-1 space-y-2 的标签与提示，
// 右侧 BetterNumberInput（min 5、step 1、size sm、单位“秒”），关闭轮播时禁用。
func carouselInterval(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg := p.Config
	value := cfg.HomeGameCarouselIntervalSec
	if value == 0 {
		value = defaultCarouselIntervalSec
	}
	ui.Row(c.Key("home_game_carousel_interval_sec")).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			panel.FieldLabel(c, t, locales.SettingsAppearanceHomeGameCarouselInterval())
			panel.FieldHint(c, t, locales.SettingsAppearanceHomeGameCarouselIntervalHint(strconv.Itoa(minCarouselIntervalSec)))
		})
		better.NumberInput(c, t, "input", value, better.NumberInputProps{
			Min:      better.Bound(minCarouselIntervalSec),
			Step:     1,
			Unit:     locales.SettingsAppearanceHomeGameCarouselIntervalUnit(),
			Size:     better.NumberInputSM,
			Disabled: !cfg.HomeGameCarouselEnabled,
			Label:    locales.SettingsAppearanceHomeGameCarouselInterval(),
			OnChange: func(v int) { p.Change(func(cfg *appconf.AppConfig) { cfg.HomeGameCarouselIntervalSec = v }) },
		})
	})
}

type sliderProps struct {
	label, value       string // 标签与右侧的当前值（text-sm text-brand-500 dark:text-brand-400）
	current            float64
	lo, hi, step       float64
	disabled           bool
	minLabel, maxLabel string // 滑块下方两端的说明
	hint               string
	onChange           func(float64)
}

// sliderField 对应模糊度、不透明度的 space-y-2：flex items-center justify-between 的标签与当前值，滑块，
// flex justify-between text-xs text-brand-400 dark:text-brand-500 的两端说明，以及可选的提示。
func sliderField(c *ui.Context, t *theme.Theme, key string, p sliderProps) {
	ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			panel.FieldLabel(c, t, p.label)
			components.Text(c, p.value, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		rangeInput(c, t, p)
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).Children(func() {
			faint := t.Pick(theme.Brand400, theme.Brand500)
			components.Text(c, p.minLabel, theme.TextXS, theme.WeightNormal, faint)
			components.Text(c, p.maxLabel, theme.TextXS, theme.WeightNormal, faint)
		})
		if p.hint != "" {
			panel.FieldHint(c, t, p.hint)
		}
	})
}

// rangeThumb 是 Chromium 滑块的圆形拇指（accent-neutral-600）的直径。
const rangeThumb = 16

// rangeInput 对应 <BetterInput type="range" variant="unstyled" className="h-2 w-full cursor-pointer appearance-none
// rounded-lg bg-brand-200 accent-neutral-600 dark:bg-brand-700">：appearance-none 去掉了原生的轨道，只剩 8px 高的
// 圆角底色与 Chromium 的圆形拇指（accent-color 填充）。拇指中心在两端各留半个拇指的宽度。
func rangeInput(c *ui.Context, t *theme.Theme, p sliderProps) {
	v := p.current
	// 滑块是行内元素，放在 space-y-2 的块里时自成一个行盒：Chromium 量出来它与上方的标签行相隔 10px、与下方的
	// 刻度文字相隔 14px（而不是 space-y-2 的 8px），所以上下另加 2px、6px 的外边距。
	sl := ui.SliderBase(c.Key("range"), &v, p.lo, p.hi).Step(p.step).FillWidth().Height(8).PaddingX(rangeThumb/2).
		Margin(2, 0, 6, 0).Cursor(ui.CursorPointer).Label(p.label)
	if p.disabled {
		components.Disable(sl, 0.5) // 原生的禁用滑块不变淡，只有拇指变灰
	}
	if v != p.current && !p.disabled && p.onChange != nil {
		p.onChange(v)
	}
	frac := float32(0)
	if p.hi > p.lo {
		frac = float32(min(1, max(0, (v-p.lo)/(p.hi-p.lo))))
	}
	track := t.Pick(theme.Brand200, theme.Brand700)
	thumb := theme.Neutral600
	if p.disabled {
		// 禁用的原生拇指是灰色的（没有和原版截图对照过）。
		thumb = t.Pick(theme.Brand300, theme.Brand600)
	}
	sl.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(r, track, min(8, r.H/2))
		x := r.X + rangeThumb/2 + (r.W-rangeThumb)*frac
		pt.Fill(ui.Rect{X: x - rangeThumb/2, Y: r.Y + r.H/2 - rangeThumb/2, W: rangeThumb, H: rangeThumb}, thumb, rangeThumb/2)
	})
}
