package stats

import (
	"math"
	"regexp"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/assets"
)

// 封面拼贴海报的绘制，对应原版 utils/posterCanvas.ts 的 renderPoster：预览与导出画同一个 posterScene，
// 预览按面板大小缩放，导出用 ui.Render 按 3 倍画出 3240×4320 的 PNG。

// posterFont 对应 POSTER_FONT（"Microsoft YaHei", "PingFang SC", "Noto Sans CJK SC", sans-serif）：取本系统那一个，
// 缺字时由系统回退。
var posterFont = func() string {
	switch runtime.GOOS {
	case "windows":
		return "Microsoft YaHei"
	case "darwin":
		return "PingFang SC"
	}
	return "Noto Sans CJK SC"
}()

// posterOptions 是 PosterOptions。
type posterOptions struct {
	title, quote, rangeText, summary         string
	showTitles, showCompanies, showDurations bool
	durationLabels                           map[string]string
	crops                                    map[string]posterCrop
	seed                                     int
}

// posterText 是排好的一段文字，x、y 是它左上角在海报（或封面框）坐标里的位置。
type posterText struct {
	layout *ui.TextLayout
	x, y   float32
	color  ui.Color
}

// posterTileScene 是一张封面框里要画的东西：坐标都相对封面框的左上角。
type posterTileScene struct {
	tile    posterTile
	image   *ui.Bitmap
	crop    posterCrop
	shadeY  float32 // 底部渐变的起点；没有文字时为负
	texts   []posterText
	missing posterText // 没有封面时的 ◇
}

// posterScene 是一张海报排好的全部内容。文字在主线程排版（ui.ShapeText），绘制只读它，导出时在后台画同一份。
type posterScene struct {
	tiles      []posterTile // 布局顺序，点击判断用
	draw       []posterTileScene
	background *ui.Bitmap // 第一名的封面，模糊后铺在底下
	texts      []posterText
	quote      bool
	brand      *ui.Bitmap
}

// posterShaper 排版海报文字并缓存，同一组输入反复重建场景（改标题、开关）时不重复排版。只在主线程使用。
type posterShaper struct {
	cache map[posterShapeKey]*ui.TextLayout
}

type posterShapeKey struct {
	s    string
	size float64
}

func (s *posterShaper) shape(text string, size float64) *ui.TextLayout {
	k := posterShapeKey{text, size}
	if l, ok := s.cache[k]; ok {
		return l
	}
	if s.cache == nil || len(s.cache) > 8192 {
		s.cache = map[posterShapeKey]*ui.TextLayout{}
	}
	l := ui.ShapeText(text, ui.Font{Family: posterFont, Size: float32(size)}, 0)
	s.cache[k] = l
	return l
}

func (s *posterShaper) width(text string, size float64) float64 {
	w, _ := s.shape(text, size).Size()
	return float64(w)
}

// place 对应 fillText：(x, y) 是基线上的点，align 为 0、0.5、1 时分别是左对齐、居中、右对齐。
func (s *posterShaper) place(text string, size, x, y, align float64, color ui.Color) posterText {
	l := s.shape(text, size)
	w, _ := l.Size()
	base := float32(0)
	if lines := l.Lines(); len(lines) > 0 {
		base = lines[0].Baseline
	}
	return posterText{layout: l, x: float32(x - float64(w)*align), y: float32(y) - base, color: color}
}

var posterSpaces = regexp.MustCompile(`\s+`)

// wrapped 对应 wrapped：逐字折行，最多 maxLines 行，超出时最后一行截断加“…”。
func (s *posterShaper) wrapped(value string, size, width float64, maxLines int) []string {
	var lines []string
	line := ""
	chars := []rune(strings.TrimSpace(posterSpaces.ReplaceAllString(value, " ")))
	for i, ch := range chars {
		if s.width(line+string(ch), size) > width && line != "" {
			lines = append(lines, line)
			line = ""
			if len(lines) == maxLines {
				last := []rune(lines[maxLines-1])
				for len(last) > 0 && s.width(string(last)+"…", size) > width {
					last = last[:len(last)-1]
				}
				lines[maxLines-1] = string(last) + "…"
				return lines
			}
		}
		line += string(ch)
		if i == len(chars)-1 {
			lines = append(lines, line)
		}
	}
	return lines
}

func posterColor(hex string) ui.Color { return ui.Hex(hex) }

// buildPosterScene 排好一张海报：games 是要画的游戏（已按时长排好），images 是加载到的封面。
func (s *posterShaper) buildPosterScene(games []vo.PosterGame, images map[string]*ui.Bitmap, o posterOptions) *posterScene {
	sc := &posterScene{tiles: buildPosterLayout(games, o.seed), brand: assets.AppIcon(true)}
	if len(games) > 0 {
		sc.background = images[games[0].GameID]
	}
	title := ""
	if lines := s.wrapped(o.title, 40, 952, 1); len(lines) > 0 {
		title = lines[0]
	}
	muted := posterColor("#a4a3aa")
	sc.texts = append(sc.texts,
		s.place(title, 40, 64, 147, 0, posterColor("#f1efef")),
		s.place(o.rangeText, 19, 64, 190, 0, muted),
		s.place(o.summary, 19, 1016, 190, 1, muted),
	)

	order := make([]posterTile, len(sc.tiles))
	copy(order, sc.tiles)
	// 按抬起程度从低到高画，前两名盖在别的封面上（Array.sort 是稳定排序）。
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && order[j-1].lift > order[j].lift; j-- {
			order[j-1], order[j] = order[j], order[j-1]
		}
	}
	for _, t := range order {
		ts := posterTileScene{tile: t, image: images[t.game.GameID], crop: o.crops[t.game.GameID], shadeY: -1}
		w, h := t.width, t.height
		if ts.image == nil {
			ts.missing = s.place("◇", math.Min(24, w*0.1), w/2, h/2, 0.5, posterColor("#9b9ba6")) // i18n:keep：占位符号
		}
		pad := math.Max(5, math.Min(20, w*0.065))
		size := math.Max(8, math.Min(35, math.Min(w*0.105, h*0.14)))
		companySize := math.Max(6, size*0.58)
		var titleLines []string
		if o.showTitles {
			maxLines := 2
			if h > w*1.1 {
				maxLines = 3
			}
			titleLines = s.wrapped(t.game.GameName, size, w-pad*2, maxLines)
		}
		companyHeight, durationHeight := 0.0, 0.0
		if o.showCompanies && t.game.Company != "" {
			companyHeight = companySize * 1.6
		}
		if o.showDurations {
			durationHeight = companySize * 1.6
		}
		details := float64(len(titleLines))*size*1.25 + companyHeight + durationHeight
		if details > 0 {
			ts.shadeY = float32(math.Max(0, h-details-pad*3))
			for i, line := range titleLines {
				y := h - pad - companyHeight - durationHeight - float64(len(titleLines)-1-i)*size*1.25
				ts.texts = append(ts.texts, s.place(line, size, pad, y, 0, posterColor("#ffffff")))
			}
			if companyHeight > 0 {
				company := ""
				if lines := s.wrapped(t.game.Company, companySize, w-pad*2, 1); len(lines) > 0 {
					company = lines[0]
				}
				ts.texts = append(ts.texts, s.place(company, companySize, pad, h-pad-durationHeight, 0, posterColor("#d5d4dc")))
			}
		}
		if o.showDurations {
			ts.texts = append(ts.texts, s.place(o.durationLabels[t.game.GameID], companySize, pad, h-pad, 0, posterColor("#dedce4")))
		}
		sc.draw = append(sc.draw, ts)
	}

	if strings.TrimSpace(o.quote) != "" {
		sc.quote = true
		for i, line := range s.wrapped(o.quote, 22, 850, 2) {
			sc.texts = append(sc.texts, s.place(line, 22, 540, 1290+float64(i)*32, 0.5, posterColor("#c9c6ce")))
		}
	}
	sc.texts = append(sc.texts, s.place("LunaBox", 19, 1016, 1394, 1, posterColor("#d8d6dc")))
	return sc
}

// posterView 把海报画成 w×h 的元素（宽高比 3:4）。第一名的封面模糊铺底（原版 ctx.filter = blur(65px)、
// globalAlpha 0.09），用元素的 Blur 与 Opacity 画在单独一层；其余内容在上面一层按海报坐标绘制。
func posterView(c *ui.Context, sc *posterScene, w, h float32) ui.Element {
	k := w / posterWidth
	box := ui.Box(c).Size(w, h).Clip().Background(posterColor("#09090b"))
	box.Children(func() {
		if bg := sc.background; bg != nil {
			ui.Box(c.Key("background")).Absolute().Top(0).Left(0).Size(w, h).Blur(65 * k).Opacity(0.09).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Transform(ui.Scaling(k, k).Then(ui.Translation(r.X, r.Y)), 0, 0, func() {
					drawPosterCover(p, bg, ui.Rect{X: -120, Y: 40, W: 1320, H: 1220}, posterCrop{})
				})
			})
		}
		ui.Box(c.Key("content")).Absolute().Top(0).Left(0).Size(w, h).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Transform(ui.Scaling(k, k).Then(ui.Translation(r.X, r.Y)), 0, 0, func() { paintPoster(p, sc) })
		})
	})
	return box
}

// drawPosterCover 对应 cover：按取景把封面铺满 dst。
func drawPosterCover(p *ui.Painter, img *ui.Bitmap, dst ui.Rect, crop posterCrop) {
	iw, ih := img.Size()
	if iw == 0 || ih == 0 {
		return
	}
	x, y, w, h := posterCoverSource(float64(iw), float64(ih), float64(dst.W), float64(dst.H), crop)
	p.ImageRegion(img, ui.Rect{X: float32(x), Y: float32(y), W: float32(w), H: float32(h)}, dst, ui.ImageOptions{})
}

func drawPosterText(p *ui.Painter, t posterText, ox, oy float32) {
	p.TextLayout(t.layout, ox+t.x, oy+t.y, t.color)
}

// paintPoster 按海报坐标（1080×1440）画除模糊底图以外的全部内容。
func paintPoster(p *ui.Painter, sc *posterScene) {
	full := ui.Rect{W: posterWidth, H: posterHeight}
	// createRadialGradient(540, 640, 90, 540, 640, 820)
	p.FillRadialGradient(full, ui.RadialGradient{
		Center: ui.Point{X: 540.0 / posterWidth, Y: 640.0 / posterHeight},
		Radius: 820,
		Stops:  []ui.GradientStop{{Color: posterColor("#24212555"), At: 90.0 / 820}, {Color: posterColor("#000000b0"), At: 1}},
	}, 0)
	for _, t := range sc.texts[:3] {
		drawPosterText(p, t, 0, 0)
	}

	for _, ts := range sc.draw {
		t := ts.tile
		r := ui.Rect{X: float32(t.x), Y: float32(t.y - t.lift*7), W: float32(t.width), H: float32(t.height)}
		grow := float32(1 + t.lift*0.045)
		p.Transform(ui.Scaling(grow, grow).Then(ui.Rotation(float32(t.rotation))), r.X+r.W/2, r.Y+r.H/2, func() {
			if t.lift > 0 {
				p.Shadow(r, 0, 0, 12, 18, 0, posterColor("#000000df"))
			}
			p.Fill(r, posterColor("#25252b"), 0)
			if ts.image != nil {
				drawPosterCover(p, ts.image, r, ts.crop)
			} else if ts.missing.layout != nil {
				drawPosterText(p, ts.missing, r.X, r.Y)
			}
			alpha := 0.04
			if t.lift == 0 {
				distance := math.Min(1, math.Hypot((t.x+t.width/2-540)/570, (t.y+t.height/2-700)/570))
				alpha = 0.07 + distance*0.38
			}
			p.Fill(r, ui.RGBA(0, 0, 0, float32(alpha)), 0)
			if ts.shadeY >= 0 {
				shade := ui.Rect{X: r.X, Y: r.Y + ts.shadeY, W: r.W, H: r.H - ts.shadeY}
				p.FillGradient(shade, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
					{Color: posterColor("#00000000"), At: 0}, {Color: posterColor("#00000087"), At: 0.5}, {Color: posterColor("#000000dc"), At: 1},
				}}, 0)
			}
			for _, text := range ts.texts {
				drawPosterText(p, text, r.X, r.Y)
			}
			if t.lift > 0 {
				p.Stroke(ui.Rect{X: r.X + 0.5, Y: r.Y + 0.5, W: r.W - 1, H: r.H - 1}, posterColor("#dedee34d"), 0, 1)
			}
		})
	}

	if sc.quote {
		line := posterColor("#ffffff26")
		p.Line(425, 1240, 515, 1240, 1, line)
		p.Line(565, 1240, 655, 1240, 1, line)
		p.Fill(ui.Rect{X: 533, Y: 1233, W: 14, H: 14}, posterColor("#e4e1e8"), 7)
		p.Fill(ui.Rect{X: 537, Y: 1231, W: 12, H: 12}, posterColor("#0d0d10"), 6)
	}
	for _, t := range sc.texts[3:] {
		drawPosterText(p, t, 0, 0)
	}
	if sc.brand != nil {
		p.ImageRegion(sc.brand, ui.Rect{}, ui.Rect{X: 893, Y: 1365, W: 38, H: 38}, ui.ImageOptions{})
	}
}
