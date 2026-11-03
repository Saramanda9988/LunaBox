package stats

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"log"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// mosaicID 是封面拼贴在模板列表里的 ID（原版 TemplateExportModal 的 "cover_mosaic"，默认选中）。
const mosaicID = "cover_mosaic"

const (
	posterExportScale = 3    // renderPoster 的 scale：PNG 3240×4320
	posterCoverWidth  = 1000 // 封面最宽解码到这么多像素（原版按长边 1400 缩小）
	posterLoaders     = 4    // loadPosterImages 同时读取的封面数
	editorFade        = 240 * time.Millisecond
)

// mosaicState 是封面拼贴（MosaicPosterPanel 与 PosterCoverEditor）的状态。选中封面拼贴时 openMosaic 重置并加载，
// 切到别的模板或关闭弹窗时 closeMosaic 丢弃，同原版组件的挂载与卸载。
type mosaicState struct {
	title, quote                             string
	showTitles, showCompanies, showDurations bool
	limit, seed                              int
	crops                                    map[string]posterCrop

	games    []vo.PosterGame // 按时长排好的全部游戏
	images   map[string]*ui.Bitmap
	loading  bool
	progress int
	failed   bool
	gen      int // 每次加载加一，丢弃旧的结果
	cancel   context.CancelFunc

	// 预览的场景：输入不变时沿用，不重复排版。
	shaper   posterShaper
	scene    *posterScene
	sceneKey mosaicSceneKey
	imageVer int
	cropVer  int

	editing     string // editingId
	editShown   bool   // 编辑器已经画过一帧（淡入从这一帧开始）
	editClosing bool
	editCloseAt time.Time
	editFocus   bool
	back        ui.Handle
	drag        *cropDrag
}

type cropDrag struct {
	crop posterCrop
	x, y float32
}

type mosaicSceneKey struct {
	title, quote, rangeText, summary, units  string
	showTitles, showCompanies, showDurations bool
	limit, seed, imageVer, cropVer, count    int
}

// openMosaic 对应 MosaicPosterPanel 挂载：标题与配文取默认文案，开关全开，显示全部游戏，然后加载。
func (p *Page) openMosaic() {
	m := &p.export.mosaic
	p.closeMosaic()
	m.title, m.quote = locales.StatsTemplateExportMosaicDefaultTitle(), locales.StatsTemplateExportMosaicDefaultQuote()
	m.showTitles, m.showCompanies, m.showDurations = true, true, true
	m.limit, m.seed, m.crops = 0, 1, map[string]posterCrop{}
	p.loadMosaic()
}

// closeMosaic 对应卸载：停止加载，丢掉封面。
func (p *Page) closeMosaic() {
	m := &p.export.mosaic
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.gen++
	m.games, m.images, m.scene = nil, nil, nil
	m.loading, m.failed, m.progress = false, false, 0
	m.editing, m.editClosing, m.drag = "", false, nil
}

// posterDates 是统计范围的 YYYY-MM-DD（原版 startDate.slice(0, 10)）。
func posterDates(s *vo.PeriodStats) (string, string) {
	cut := func(v string) string { return v[:min(10, len(v))] }
	return cut(s.StartDate), cut(s.EndDate)
}

// loadMosaic 对应加载的 useEffect（也是“重试”）：GetPosterGames 后最多 4 个一组读取封面并报告进度。
// 读不到的封面留在排布里画占位，并提示缺几张；GetPosterGames 失败时显示错误。
func (p *Page) loadMosaic() {
	m := &p.export.mosaic
	if m.cancel != nil {
		m.cancel()
	}
	if p.stats == nil {
		return
	}
	m.gen++
	gen := m.gen
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.loading, m.failed, m.games, m.images, m.progress, m.editing = true, false, nil, nil, 0, ""
	start, end := posterDates(p.stats)
	cc := p.env.Covers
	p.env.Async(func() func() {
		list, err := p.env.Svc.Stats.GetPosterGames(start, end)
		if err != nil {
			log.Printf("template export: poster games: %v", err)
			return func() {
				if gen == m.gen {
					m.loading, m.failed = false, true
				}
			}
		}
		games := rankedPosterGames(list)
		p.post(func() {
			if gen == m.gen {
				m.games = games
			}
		})
		images := loadPosterImages(ctx, cc, games, func() {
			p.post(func() {
				if gen == m.gen {
					m.progress++
				}
			})
		})
		return func() {
			if gen != m.gen {
				return
			}
			m.loading, m.images = false, images
			m.imageVer++
		}
	})
}

// post 把 fn 放回主线程执行；没有窗口（测试）时直接执行。
func (p *Page) post(fn func()) {
	if p.env.Win == nil {
		fn()
		return
	}
	p.env.Win.Update(fn)
}

// loadPosterImages 对应 loadPosterImages：封面通过封面缓存读取（本地文件或网络），每读完一个（无论成败）调用 done。
func loadPosterImages(ctx context.Context, cc *covers.Cache, games []vo.PosterGame, done func()) map[string]*ui.Bitmap {
	var (
		mu     sync.Mutex
		next   atomic.Int64
		wg     sync.WaitGroup
		images = map[string]*ui.Bitmap{}
	)
	for range min(posterLoaders, len(games)) {
		wg.Go(func() {
			for ctx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= len(games) {
					return
				}
				g := games[i]
				if g.CoverURL != "" && cc != nil {
					if bmp, err := cc.LoadFull(&models.Game{ID: g.GameID, CoverURL: g.CoverURL}, posterCoverWidth); err == nil {
						mu.Lock()
						images[g.GameID] = bmp
						mu.Unlock()
					}
				}
				if ctx.Err() == nil {
					done()
				}
			}
		})
	}
	wg.Wait()
	return images
}

// selectedGames 是按“显示数量”截取的游戏。
func (m *mosaicState) selectedGames() []vo.PosterGame {
	if m.limit > 0 && m.limit < len(m.games) {
		return m.games[:m.limit]
	}
	return m.games
}

// missing 是选中的游戏里没有读到封面的个数。
func (m *mosaicState) missing() int {
	if m.images == nil {
		return 0
	}
	n := 0
	for _, g := range m.selectedGames() {
		if m.images[g.GameID] == nil {
			n++
		}
	}
	return n
}

// ready 对应 onReady(canvas)：封面加载完、有游戏、没有在调整取景时可以导出。
func (m *mosaicState) ready() bool {
	return !m.loading && !m.failed && m.images != nil && len(m.selectedGames()) > 0 && m.editing == ""
}

func (p *Page) posterOptions(selected []vo.PosterGame) posterOptions {
	m := &p.export.mosaic
	start, end := posterDates(p.stats)
	total := 0
	labels := make(map[string]string, len(selected))
	for _, g := range selected {
		total += g.TotalDuration
		labels[g.GameID] = durationCompact(g.TotalDuration)
	}
	return posterOptions{
		title: m.title, quote: m.quote,
		rangeText:  start + " — " + end,
		summary:    locales.StatsTemplateExportMosaicSummary(strconv.Itoa(len(selected)), durationCompact(total)),
		showTitles: m.showTitles, showCompanies: m.showCompanies, showDurations: m.showDurations,
		durationLabels: labels, crops: m.crops, seed: m.seed,
	}
}

// previewScene 返回预览的场景，输入变了才重新排版。
func (p *Page) previewScene() *posterScene {
	m := &p.export.mosaic
	selected := m.selectedGames()
	o := p.posterOptions(selected)
	key := mosaicSceneKey{
		title: o.title, quote: o.quote, rangeText: o.rangeText, summary: o.summary,
		units:      locales.CommonDurationHoursShort() + locales.CommonDurationMinutesShort(),
		showTitles: o.showTitles, showCompanies: o.showCompanies, showDurations: o.showDurations,
		limit: m.limit, seed: m.seed, imageVer: m.imageVer, cropVer: m.cropVer, count: len(m.games),
	}
	if m.scene == nil || key != m.sceneKey {
		m.scene, m.sceneKey = m.shaper.buildPosterScene(selected, m.images, o), key
	}
	return m.scene
}

// exportMosaic 对应 handleExport 的 cover_mosaic 分支：按 3 倍画成 PNG 交给保存对话框，导出后弹窗不关闭。
// 场景在主线程重新排版一份，后台只读它。
func (p *Page) exportMosaic() {
	x := &p.export
	m := &x.mosaic
	if !m.ready() || x.exporting {
		return
	}
	selected := m.selectedGames()
	var shaper posterShaper
	sc := shaper.buildPosterScene(selected, m.images, p.posterOptions(selected))
	gen := x.gen
	x.exporting = true
	p.env.Async(func() func() {
		data, err := renderPosterPNG(sc)
		if err == nil {
			err = p.env.Svc.Template.ExportRenderedHTML("data:image/png;base64," + base64.StdEncoding.EncodeToString(data))
		}
		return func() {
			if gen != x.gen {
				return
			}
			x.exporting = false
			if err != nil {
				log.Printf("template export: export poster: %v", err)
				msg := err.Error()
				p.toast(func() env.Notice { return env.Error(locales.StatsTemplateExportToastExportFailed(msg)) })
			}
		}
	})
}

// renderPosterPNG 用软件渲染器按 3 倍画出海报（3240×4320）。
func renderPosterPNG(sc *posterScene) ([]byte, error) {
	img := ui.Render(func(c *ui.Context) { posterView(c, sc, posterWidth, posterHeight) }, posterWidth, posterHeight, posterExportScale)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// editableTiles 是能调整取景的封面（有封面的），按布局顺序。
func (m *mosaicState) editableTiles(sc *posterScene) []posterTile {
	var out []posterTile
	for _, t := range sc.tiles {
		if m.images[t.game.GameID] != nil {
			out = append(out, t)
		}
	}
	return out
}

func (m *mosaicState) openEditor(id string) {
	m.editing, m.editShown, m.editClosing, m.editFocus, m.drag = id, false, false, true, nil
}

// stepEditor 对应 stepEditor：在能调整的封面之间循环切换。
func (m *mosaicState) stepEditor(sc *posterScene, direction int) {
	editable := m.editableTiles(sc)
	if len(editable) == 0 {
		return
	}
	index := -1
	for i, t := range editable {
		if t.game.GameID == m.editing {
			index = i
		}
	}
	m.editing = editable[((index+direction)%len(editable)+len(editable))%len(editable)].game.GameID
	m.editClosing, m.drag = false, nil
}

func (m *mosaicState) setCrop(id string, crop posterCrop) {
	m.crops[id] = crop
	m.cropVer++
}

// mosaicPanel 对应 MosaicPosterPanel：上方 grid gap-3 p-4 border-b 的表单，下方 relative flex-1 p-4 的预览。
func (p *Page) mosaicPanel(c *ui.Context, t *theme.Theme, line ui.Color) {
	m := &p.export.mosaic
	ui.Column(c.Key("mosaic")).Grow(1).Basis(0).MinHeight(0).Children(func() {
		p.mosaicFields(c, t)
		ui.Box(c).Height(t.HL).Background(line).Shrink(0)
		ui.Column(c.Key("mosaic-area")).Grow(1).Basis(0).MinHeight(0).Padding(16).Clip().Children(func() {
			muted := t.Pick(theme.Brand500, theme.Brand300)
			switch {
			case m.loading:
				ui.Row(c.Key("loading")).Grow(1).Gap(8).Center().Children(func() {
					components.Spinner(c, 20, muted)
					components.Text(c, locales.StatsTemplateExportMosaicLoading(strconv.Itoa(m.progress), strconv.Itoa(len(m.games))),
						theme.TextSM, theme.WeightNormal, muted)
				})
			case m.failed:
				// p-6 text-center text-brand-700 dark:text-brand-200；重试 mt-3 underline
				fg := t.Pick(theme.Brand700, theme.Brand200)
				ui.Column(c.Key("error")).Padding(24).AlignItems(ui.Center).Children(func() {
					components.Text(c, locales.StatsTemplateExportMosaicError(), theme.TextBase, theme.WeightNormal, fg)
					ui.ButtonBase(c.Key("retry")).Margin(12, 0, 0, 0).FocusRing(false).OnClick(p.loadMosaic).Children(func() {
						components.Text(c, locales.StatsTemplateExportMosaicRetry(), theme.TextBase, theme.WeightNormal, fg).Underline()
					})
				})
			case len(m.games) == 0:
				ui.Column(c.Key("empty")).Padding(32).AlignItems(ui.Center).Children(func() {
					components.Text(c, locales.StatsTemplateExportMosaicEmpty(), theme.TextSM, theme.WeightNormal, muted)
				})
			default:
				sc := p.previewScene()
				p.posterPreview(c, t, sc)
				if m.editing != "" {
					p.posterEditor(c, t, sc)
				}
			}
		})
	})
}

// mosaicFields 是 fieldset：海报标题、底部配文（maxLength 60、120），三个开关，显示数量与“换个排布”。
// sm 以上两列。导出中整体禁用（disabled:opacity-60）。
func (p *Page) mosaicFields(c *ui.Context, t *theme.Theme) {
	x := &p.export
	m := &x.mosaic
	winW, _ := c.Size()
	cols := 1
	if winW >= breakSM {
		cols = 2
	}
	label := t.Pick(theme.Brand600, theme.Brand300)
	grid := ui.Grid(c.Key("fields")).Columns(cols).GapX(12).GapY(12).Padding(16).Shrink(0).Background(t.Pick(theme.White, theme.Brand800))
	if x.exporting {
		components.Disable(grid, 0.6)
	}
	grid.Children(func() {
		p.mosaicInput(c, t, "title", locales.StatsTemplateExportMosaicHeading(), &m.title, 60)
		p.mosaicInput(c, t, "quote", locales.StatsTemplateExportMosaicQuote(), &m.quote, 120)
		// flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-brand-700 dark:text-brand-200
		ui.Row(c.Key("switches")).Wrap().GapX(20).GapY(8).AlignItems(ui.Center).Children(func() {
			fg := t.Pick(theme.Brand700, theme.Brand200)
			for _, sw := range []struct {
				key   string
				label string
				value *bool
			}{
				{"titles", locales.StatsTemplateExportMosaicShowTitles(), &m.showTitles},
				{"companies", locales.StatsTemplateExportMosaicShowCompanies(), &m.showCompanies},
				{"durations", locales.StatsTemplateExportMosaicShowDurations(), &m.showDurations},
			} {
				value := sw.value
				toggle := func(v bool) {
					if !x.exporting {
						*value = v
					}
				}
				ui.Row(c.Key(sw.key)).Gap(8).AlignItems(ui.Center).Children(func() {
					// <label htmlFor>：点文字也切换开关
					ui.ButtonBase(c.Key("label")).FocusRing(false).OnClick(func() { toggle(!*value) }).Children(func() {
						components.Text(c, sw.label, theme.TextXS, theme.WeightNormal, fg).NoWrap()
					})
					better.Switch(c, t, "switch", *value, x.exporting, toggle)
				})
			}
		})
		// flex items-center gap-3：label flex-1（span + select），“换个排布”按钮
		ui.Row(c.Key("count")).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Text(c, locales.StatsTemplateExportMosaicCount(), theme.TextXS, theme.WeightNormal, label).NoWrap()
				opts := []better.SelectOption{{Label: locales.StatsTemplateExportMosaicAll(strconv.Itoa(len(m.games))), Value: "0"}}
				for _, n := range []int{6, 20, 60, 120} {
					opts = append(opts, better.SelectOption{Label: locales.StatsTemplateExportMosaicTop(strconv.Itoa(n)), Value: strconv.Itoa(n)})
				}
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					better.Select(c, t, "limit", better.SelectProps{
						Value: strconv.Itoa(m.limit), Options: opts, Font: theme.TextSM, Disabled: x.exporting,
						OnChange: func(v string) { m.limit, _ = strconv.Atoi(v) },
						Style:    func(btn ui.Element) { btn.Radius(8) },
					})
				})
			})
			// px-3 py-2 rounded-lg text-xs whitespace-nowrap text-brand-700 dark:text-brand-200 bg-brand-100
			// hover:bg-brand-200 dark:bg-brand-700 dark:hover:bg-brand-600；不足 2 个游戏时 disabled（样式不变）
			btn := ui.ButtonBase(c.Key("shuffle")).Padding(8, 12).Radius(8).FocusRing(false)
			bg := t.Pick(theme.Brand100, theme.Brand700)
			if btn.Hovered() {
				bg = t.Pick(theme.Brand200, theme.Brand600)
			}
			btn.Background(bg).OnClick(func() {
				if len(m.selectedGames()) >= 2 && !x.exporting {
					m.seed++
				}
			}).Children(func() {
				components.Text(c, locales.StatsTemplateExportMosaicShuffle(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand200)).NoWrap()
			})
		})
	})
}

// mosaicInput 是 label space-y-1 text-xs 的标题加 fieldClass 的输入框：w-full rounded-lg border border-brand-300
// dark:border-brand-600 bg-white dark:bg-brand-900 px-3 py-2 text-sm text-brand-900 dark:text-white，聚焦 ring-2 ring-neutral-500。
func (p *Page) mosaicInput(c *ui.Context, t *theme.Theme, key, label string, value *string, maxLen int) {
	ui.Column(c.Key(key)).Gap(4).Children(func() {
		components.Text(c, label, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
		in := ui.TextInputBase(c.Key("input"), value).FillWidth().Padding(8, 12).Radius(8).Label(label).
			FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.Pick(theme.Brand900, theme.White)).
			Background(t.Pick(theme.White, theme.Brand900)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		if in.Focused() {
			in.Shadow(0, 0, 0, 2, theme.Neutral500)
		}
		if utf8.RuneCountInString(*value) > maxLen {
			*value = string([]rune(*value)[:maxLen])
		}
	})
}

// posterPreview 是预览：flex-wrap justify-between mb-3 text-xs 的分辨率与缺封面提示，h-[calc(100%-3rem)]
// min-h-[280px] 里按比例放大到最大的海报（rounded-lg shadow-xl，点击封面调整取景），mt-2 的提示行。
func (p *Page) posterPreview(c *ui.Context, t *theme.Theme, sc *posterScene) {
	x := &p.export
	m := &x.mosaic
	meta := t.Pick(theme.Brand600, theme.Brand300)
	ui.Row(c.Key("meta")).Wrap().Gap(8).Justify(ui.SpaceBetween).AlignItems(ui.Center).Margin(0, 0, 12, 0).Shrink(0).Children(func() {
		components.Text(c, locales.StatsTemplateExportMosaicResolution(strconv.Itoa(len(m.selectedGames()))), theme.TextXS, theme.WeightNormal, meta)
		if n := m.missing(); n > 0 {
			ui.ButtonBase(c.Key("missing")).FocusRing(false).OnClick(func() {
				if !x.exporting {
					p.loadMosaic()
				}
			}).Children(func() {
				components.Text(c, locales.StatsTemplateExportMosaicMissing(strconv.Itoa(n)), theme.TextXS, theme.WeightNormal, meta).Underline()
			})
		}
	})
	area := ui.Row(c.Key("canvas")).Grow(1).Basis(0).MinHeight(280).Justify(ui.Center)
	b := area.Bounds()
	area.Children(func() {
		if b.W <= 0 || b.H <= 0 {
			c.Invalidate() // 第一帧还不知道大小
			return
		}
		k := min(b.W/posterWidth, b.H/posterHeight)
		w, h := posterWidth*k, posterHeight*k
		frame := ui.Box(c.Key("poster")).Size(w, h).Radius(8).Clip().Cursor(ui.CursorPointer).Label(locales.StatsTemplateExportMosaicPreview()).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		frame.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Button != 0 || (ev.Kind != ui.InputPointerDown && ev.Kind != ui.InputPointerUp) {
				return false
			}
			if ev.Kind == ui.InputPointerUp && !x.exporting {
				if hit, ok := hitPosterTile(sc.tiles, float64(ev.X/k), float64(ev.Y/k)); ok && m.images[hit.game.GameID] != nil {
					m.openEditor(hit.game.GameID)
				}
			}
			return true
		}).Children(func() {
			posterView(c, sc, w, h)
		})
	})
	ui.Row(c.Key("crop-hint")).Margin(8, 0, 0, 0).Gap(8).Justify(ui.Center).AlignItems(ui.Center).Shrink(0).Children(func() {
		muted := t.Pick(theme.Brand500, theme.Brand300)
		components.Text(c, locales.StatsTemplateExportMosaicClickToCrop(), theme.TextXS, theme.WeightNormal, muted)
		editable := m.editableTiles(sc)
		btn := ui.ButtonBase(c.Key("crop-editor")).FocusRing(false)
		if x.exporting || len(editable) == 0 {
			btn.Disabled(true)
		}
		btn.OnClick(func() {
			if len(editable) > 0 {
				m.openEditor(editable[0].game.GameID)
			}
		}).Children(func() {
			components.Text(c, locales.StatsTemplateExportMosaicCropEditor(), theme.TextXS, theme.WeightNormal, muted).Underline()
		})
	})
}

// posterEditor 对应 PosterCoverEditor：盖住预览区的 absolute inset-0 flex-col gap-3 p-4 bg-brand-50/98
// dark:bg-brand-900/98，淡入淡出 240ms；拖动封面平移取景，滚轮或滑块缩放（1～3 倍），Esc 或“返回全景”关闭。
func (p *Page) posterEditor(c *ui.Context, t *theme.Theme, sc *posterScene) {
	x := &p.export
	m := &x.mosaic
	var tile posterTile
	found := false
	for _, tl := range sc.tiles {
		if tl.game.GameID == m.editing {
			tile, found = tl, true
		}
	}
	img := m.images[m.editing]
	if !found || img == nil {
		m.editing = ""
		return
	}
	if m.editClosing {
		if m.editCloseAt.IsZero() {
			m.editCloseAt = c.Now()
		}
		if wait := editorFade - c.Now().Sub(m.editCloseAt); wait > 0 {
			c.After(wait)
		} else {
			m.editing, m.editClosing, m.drag = "", false, nil
			c.Invalidate()
			return
		}
	}
	id := m.editing
	crop := normalizeCrop(m.crops[id])
	iw, ih := img.Size()
	closeEditor := func() {
		if !x.exporting && !m.editClosing {
			m.editClosing, m.editCloseAt = true, time.Time{}
		}
	}

	ov := ui.Column(c.Key("editor")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(16).Gap(12).
		Background(t.Pick(theme.Brand50.Alpha(0.98), theme.Brand900.Alpha(0.98))).Label(locales.StatsTemplateExportMosaicCropEditor())
	ov.Opacity(ov.AnimateWith("visible", theme.Bool(m.editShown && !m.editClosing), editorFade, theme.EaseOut))
	if !m.editShown {
		m.editShown = true // 这一帧不透明度为 0，下一帧开始淡入
		c.Invalidate()
	}
	ov.OnShortcut(0, ui.KeyEscape, closeEditor)
	ov.Children(func() {
		ui.Row(c.Key("header")).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			components.Text(c, tile.game.GameName, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand900, theme.Brand100)).
				SingleLine().Grow(1).Basis(0).MinWidth(0)
			back := editorButton(c, t, "back", locales.StatsTemplateExportMosaicBack(), theme.TextSM, 12, t.Pick(theme.Brand900, theme.Brand100)).Bind(&m.back).OnClick(closeEditor)
			if x.exporting {
				back.Disabled(true)
			}
		})
		if m.editFocus {
			m.editFocus = false
			m.back.Focus(c) // 让 Esc 落在编辑器里
		}
		components.Text(c, locales.StatsTemplateExportMosaicCropHint(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand300))

		area := ui.Row(c.Key("crop-area")).Grow(1).Basis(0).MinHeight(0).Center()
		b := area.Bounds()
		area.Children(func() {
			if b.W <= 0 || b.H <= 0 {
				c.Invalidate()
				return
			}
			k := min(b.W/float32(tile.width), b.H/float32(tile.height))
			w, h := float32(tile.width)*k, float32(tile.height)*k
			cursor := ui.CursorGrab
			if m.drag != nil {
				cursor = ui.CursorGrabbing
			}
			canvas := ui.Box(c.Key("crop-canvas")).Size(w, h).Radius(8).Clip().Cursor(cursor).Label(locales.StatsTemplateExportMosaicCropCanvas()).
				Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
			canvas.HandleInput(func(ev ui.InputEvent) bool {
				if x.exporting {
					return false
				}
				switch ev.Kind {
				case ui.InputPointerDown:
					if ev.Button != 0 {
						return false
					}
					m.drag = &cropDrag{crop: crop, x: ev.X, y: ev.Y}
				case ui.InputPointerMove:
					if m.drag == nil {
						return false
					}
					m.setCrop(id, panPosterCrop(m.drag.crop, float64(ev.X-m.drag.x), float64(ev.Y-m.drag.y), float64(iw), float64(ih), float64(w), float64(h)))
				case ui.InputPointerUp:
					m.drag = nil
				case ui.InputScroll:
					cur := normalizeCrop(m.crops[id]) // 两帧之间可能来好几次
					zoom := math.Min(3, math.Max(1, cur.zoom*math.Exp(-float64(ev.DY)*0.001)))
					m.setCrop(id, zoomPosterCrop(cur, zoom, float64(iw), float64(ih), float64(tile.width), float64(tile.height)))
				default:
					return false
				}
				return true
			}).Children(func() {
				// 画在子元素里，才被圆角裁剪
				ui.Box(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) { drawPosterCover(pt, img, r, crop) })
			})
		})

		// fieldset flex items-center flex-wrap gap-3 text-xs text-brand-700 dark:text-brand-200
		fg := t.Pick(theme.Brand700, theme.Brand200)
		ui.Row(c.Key("controls")).Wrap().Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
			step := func(direction int) func() {
				return func() {
					if !x.exporting {
						m.stepEditor(sc, direction)
					}
				}
			}
			editorButton(c, t, "previous", locales.StatsTemplateExportMosaicPrevious(), theme.TextXS, 8, fg).OnClick(step(-1))
			ui.Row(c.Key("zoom")).Grow(1).Basis(0).MinWidth(120).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Text(c, locales.StatsTemplateExportMosaicZoom(), theme.TextXS, theme.WeightNormal, fg).NoWrap()
				v := crop.zoom
				sl := ui.SliderBase(c.Key("slider"), &v, 1, 3).Step(0.01).Grow(1).Basis(0).MinWidth(0).Height(16).
					Cursor(ui.CursorPointer).Label(locales.StatsTemplateExportMosaicZoom())
				if x.exporting {
					sl.Disabled(true)
				} else if v != crop.zoom {
					m.setCrop(id, zoomPosterCrop(crop, v, float64(iw), float64(ih), float64(tile.width), float64(tile.height)))
				}
				frac := float32((v - 1) / 2)
				sl.Draw(func(pt *ui.Painter, r ui.Rect) {
					track := ui.Rect{X: r.X, Y: r.Y + r.H/2 - 2, W: r.W, H: 4}
					pt.Fill(track, t.Pick(theme.Brand200, theme.Brand700), 2)
					pt.Fill(ui.Rect{X: track.X, Y: track.Y, W: track.W * frac, H: 4}, theme.Neutral600, 2)
					cx := r.X + 8 + (r.W-16)*frac
					pt.Fill(ui.Rect{X: cx - 8, Y: r.Y + r.H/2 - 8, W: 16, H: 16}, theme.Neutral600, 8)
				})
			})
			components.Text(c, strconv.FormatFloat(crop.zoom, 'f', 2, 64)+"×", theme.TextXS, theme.WeightNormal, fg).NoWrap().Font(monoFont)
			editorButton(c, t, "reset", locales.StatsTemplateExportMosaicResetCrop(), theme.TextXS, 8, fg).OnClick(func() {
				if !x.exporting {
					m.setCrop(id, defaultCrop)
				}
			})
			editorButton(c, t, "next", locales.StatsTemplateExportMosaicNext(), theme.TextXS, 8, fg).OnClick(step(1))
		})
	})
}

// editorButton 是编辑器的按钮：py-2 rounded-lg bg-brand-200 dark:bg-brand-700，文字颜色随所在的行
// （“返回全景” px-3 text-sm，其余 px-2 text-xs）。
func editorButton(c *ui.Context, t *theme.Theme, key, label string, font theme.FontStep, padX float32, fg ui.Color) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, padX).Radius(8).FocusRing(false).Shrink(0).Background(t.Pick(theme.Brand200, theme.Brand700))
	btn.Children(func() {
		components.Text(c, label, font, theme.WeightNormal, fg).NoWrap()
	})
	return btn
}
