package importer

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/utils/metadata"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	metadatapanel "lunabox/desktop-mygo/ui/components/panel/metadata"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// AddGame 是 AddGameModal 的状态：选择本地或远程游戏，按名称或 ID 获取元数据，或手动填写。
type AddGame struct {
	env  *env.Env
	open bool
	step string // type、local、results、id、remote、manual
	// remote 对应 importMode === "remote"：没有可执行文件，状态是“想玩”。
	remote    bool
	exe       string // executablePath
	name      string // gameName
	results   []vo.GameMetadataFromWebVO
	resultsUI SearchResults
	loading   bool
	manualID  string
	source    enums.SourceType // manualSource
	cover     string
	company   string
	summary   string
	gen       int
}

// NewAddGame 创建弹窗（关闭状态）。
func NewAddGame(e *env.Env) *AddGame { return &AddGame{env: e, step: "type", source: enums.Local} }

// Open 打开弹窗。
func (a *AddGame) Open() { a.open = true }

// IsOpen 报告弹窗是否打开。
func (a *AddGame) IsOpen() bool { return a.open }

// close 对应 resetAndClose。
func (a *AddGame) close() {
	src := a.source
	*a = AddGame{env: a.env, step: "type", source: src, gen: a.gen + 1}
}

// sourceOptions 对应 sourceOptions：启用的元数据来源；一个都没有时只有“无元数据来源”。
func (a *AddGame) sourceOptions() ([]better.SelectOption, enums.SourceType) {
	enabled := metadatapanel.EnabledSources(a.env.Config())
	var opts []better.SelectOption
	for _, s := range enabled {
		opts = append(opts, better.SelectOption{Label: SourceLabel(s), Value: string(s)})
	}
	if len(opts) == 0 {
		opts = []better.SelectOption{{Label: locales.FilterBarNoMetadataSource(), Value: string(enums.Local)}}
	}
	selected := a.source
	if !slices.Contains(enabled, selected) {
		selected = enums.Local
		if len(enabled) > 0 {
			selected = enabled[0]
		}
	}
	return opts, selected
}

// entryStep 是本地或远程导入的第一步。
func (a *AddGame) entryStep() string {
	if a.remote {
		return "remote"
	}
	return "local"
}

// inferName 对应 inferGameNameFromPath：.app 去掉扩展名，否则取可执行文件所在的文件夹名。
func inferName(path string) string {
	parts := strings.FieldsFunc(strings.ReplaceAll(path, "\\", "/"), func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		return ""
	}
	file := parts[len(parts)-1]
	if strings.HasSuffix(strings.ToLower(file), ".app") {
		return file[:len(file)-len(".app")]
	}
	if len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return file
}

// selectExecutable 对应 handleSelectExecutable。
func (a *AddGame) selectExecutable(c *ui.Context) {
	svc, cur, gen := a.env.Svc.Game, a.exe, a.gen
	a.env.Async(func() func() {
		path, err := svc.SelectGameExecutable(cur)
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				log.Printf("add game: select executable: %v", err)
				a.env.Toast(c, env.Error(locales.AddGameModalToastOpenSelectorFailed()))
				return
			}
			if path != "" {
				a.exe, a.name = path, inferName(path)
			}
		}
	})
}

// searchByName 对应 handleSearchByName：按名称获取各来源的元数据，进入结果。
func (a *AddGame) searchByName(c *ui.Context) {
	if a.name == "" || a.loading {
		return
	}
	a.loading = true
	svc, name, gen := a.env.Svc.Game, a.name, a.gen
	a.env.Async(func() func() {
		results, err := svc.FetchMetadataByName(name)
		return func() {
			if gen != a.gen {
				return
			}
			a.loading = false
			if err != nil {
				log.Printf("add game: fetch metadata %q: %v", name, err)
				a.env.Toast(c, env.Error(locales.AddGameModalToastFetchMetaFailed()))
				return
			}
			a.results = results
			a.resultsUI.Reset()
			a.step = "results"
		}
	})
}

// applyImportFields 对应 applyImportFields：远程游戏没有路径、状态是“想玩”。
func (a *AddGame) applyImportFields(g *models.Game) {
	if a.remote {
		g.Path, g.Status = "", enums.StatusWantToPlay
		return
	}
	g.Path = a.exe
	if g.Status == "" {
		g.Status = enums.StatusNotStarted
	}
}

// saveFromMetadata 对应 saveGameFromWebMetadata：选中的元数据作为默认来源，associated 里各来源的 ID 都记下。
func (a *AddGame) saveFromMetadata(c *ui.Context, meta vo.GameMetadataFromWebVO, associated []vo.GameMetadataFromWebVO, onAdded func()) {
	g := meta.Game
	g.SourceType = meta.Source
	g.MetadataSources = nil
	for _, item := range associated {
		id := strings.TrimSpace(item.Game.SourceID)
		if id == "" || item.Source == enums.Local || (item.Game.Name == "" && item.Game.SourceID == "") {
			continue
		}
		g.MetadataSources = append(g.MetadataSources, models.GameMetadataSource{SourceType: item.Source, SourceID: id})
	}
	a.applyImportFields(&g)
	a.save(c, vo.GameMetadataFromWebVO{Source: meta.Source, Game: g, Tags: meta.Tags}, onAdded)
}

// save 是 AddGameFromWebMetadata：成功后刷新游戏库并关闭弹窗。
func (a *AddGame) save(c *ui.Context, meta vo.GameMetadataFromWebVO, onAdded func()) {
	a.loading = true
	svc, gen := a.env.Svc.Game, a.gen
	a.env.Async(func() func() {
		err := svc.AddGameFromWebMetadata(meta)
		return func() {
			if gen != a.gen {
				return
			}
			a.loading = false
			if err != nil {
				log.Printf("add game: save %q: %v", meta.Game.Name, err)
				a.env.Toast(c, env.Error(locales.AddGameModalToastSaveGameFailed()))
				return
			}
			if onAdded != nil {
				onAdded()
			}
			a.close()
		}
	})
}

// selectResult 对应 handleSelectMetadataResult：每个来源只能留一个结果，否则提示先移除重复的。
func (a *AddGame) selectResult(c *ui.Context, selected vo.GameMetadataFromWebVO, onAdded func()) {
	seen := map[enums.SourceType]bool{}
	for _, item := range a.results {
		if seen[item.Source] {
			a.env.Toast(c, env.Error(locales.AddGameModalToastDuplicateSource(SourceLabel(item.Source))))
			return
		}
		seen[item.Source] = true
	}
	a.saveFromMetadata(c, selected, a.results, onAdded)
}

// searchByID 对应 handleSearchById：按来源与 ID 获取元数据后直接保存。
func (a *AddGame) searchByID(c *ui.Context, source enums.SourceType, onAdded func()) {
	if a.manualID == "" || source == enums.Local || a.loading {
		return
	}
	a.loading = true
	svc, id, gen := a.env.Svc.Game, a.manualID, a.gen
	a.env.Async(func() func() {
		meta, err := svc.FetchMetadataFromWeb(vo.MetadataRequest{Source: source, ID: id})
		return func() {
			if gen != a.gen {
				return
			}
			a.loading = false
			if err != nil {
				log.Printf("add game: fetch %s/%s: %v", source, id, err)
				a.env.Toast(c, env.Error(locales.AddGameModalToastFetchMetaByIdFailed()))
				return
			}
			if meta.Game.Name != "" || meta.Game.SourceID != "" {
				a.saveFromMetadata(c, meta, []vo.GameMetadataFromWebVO{meta}, onAdded)
			}
		}
	})
}

// selectCover 对应 handleSelectCoverImage。
func (a *AddGame) selectCover(c *ui.Context) {
	svc, gen := a.env.Svc.Game, a.gen
	a.env.Async(func() func() {
		url, err := svc.SelectCoverImageWithTempID()
		return func() {
			if gen != a.gen {
				return
			}
			if err != nil {
				log.Printf("add game: select cover: %v", err)
				a.env.Toast(c, env.Error(locales.AddGameModalToastSelectCoverFailed()))
				return
			}
			if url != "" {
				a.cover = url
			}
		}
	})
}

// manualSave 对应 handleManualSave：远程游戏填了 ID 时记下这个来源，否则是本地来源。
func (a *AddGame) manualSave(c *ui.Context, source enums.SourceType, onAdded func()) {
	if a.name == "" {
		a.env.Toast(c, env.Error(locales.AddGameModalToastFillGameName()))
		return
	}
	if !a.remote && a.exe == "" {
		a.env.Toast(c, env.Error(locales.AddGameModalToastSelectExecutableRequired()))
		return
	}
	id := ""
	if a.remote && source != enums.Local {
		id = strings.TrimSpace(a.manualID)
	}
	src := enums.Local
	if id != "" {
		src = source
	}
	g := models.Game{Name: a.name, CoverURL: a.cover, Company: a.company, Summary: a.summary, SourceType: src, SourceID: id, Status: enums.StatusNotStarted}
	if id != "" {
		g.MetadataSources = []models.GameMetadataSource{{SourceType: src, SourceID: id}}
	}
	if a.remote {
		g.Status = enums.StatusWantToPlay
	} else {
		g.Path = a.exe
	}
	a.save(c, vo.GameMetadataFromWebVO{Source: src, Game: g, Tags: []metadata.TagItem{}}, onAdded)
}

// View 对应 AddGameModal：w-full max-w-2xl rounded-xl bg-white p-6 shadow-2xl，text-4xl 的标题与关闭按钮
// （结果步骤 mb-2，其余 mb-6）。onAdded 在保存后调用（onGameAdded）。要在页面最后构建。
func (a *AddGame) View(c *ui.Context, t *theme.Theme, onAdded func()) {
	components.Modal(c, t, "add-game", components.ModalProps{Open: a.open, TopOffset: env.TopbarHeight, MaxWidth: 672}, func(pnl ui.Element) {
		pnl.Radius(12).Padding(24).Background(t.PanelBg).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
			mb := float32(24)
			if a.step == "results" {
				mb = 8
			}
			ui.Row(c.Key("header")).Margin(0, 0, mb, 0).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				components.Text(c, locales.LibraryAddGame(), theme.Text4XL, theme.WeightBold, t.TextStrong)
				btn := ui.ButtonBase(c.Key("close")).Size(32, 32).Radius(8).Center().Shrink(0).FocusRing(false).Label(locales.CommonCancel())
				color := t.Pick(theme.Brand500, theme.Brand400)
				if btn.Hovered() {
					color = t.Pick(theme.Brand100, theme.Brand700) // 按钮本身是 mask 图标，hover:bg 盖过了文字颜色
				}
				btn.OnClick(a.close).Children(func() { components.Icon(c, "close", 32, color) })
			})
			opts, source := a.sourceOptions()
			switch a.step {
			case "type":
				a.typeStep(c, t)
			case "local":
				a.localStep(c, t)
			case "remote":
				a.remoteStep(c, t, opts, source, onAdded)
			case "results":
				a.resultsStep(c, t, onAdded)
			case "id":
				a.idStep(c, t, opts, source, onAdded)
			case "manual":
				a.manualStep(c, t, source, onAdded)
			}
		})
	})
}

// typeStep：space-y-4，text-sm 的说明，grid gap-4 sm:grid-cols-2 的本地、远程两张卡片。
func (a *AddGame) typeStep(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("type-step")).Gap(16).Children(func() {
		components.Text(c, locales.AddGameModalChooseImportType(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
		ui.Row(c).Gap(16).Children(func() {
			a.typeCard(c, t, "local", 0, locales.AddGameModalLocalGame(), locales.AddGameModalLocalGameHint(), func() {
				a.remote, a.manualID, a.results, a.step = false, "", nil, "local"
			})
			a.typeCard(c, t, "remote", 1, locales.AddGameModalRemoteGame(), locales.AddGameModalRemoteGameHint(), func() {
				a.remote, a.exe, a.manualID, a.results, a.step = true, "", "", nil, "remote"
			})
		})
	})
}

// typeCard：relative min-h-56 overflow-hidden rounded-xl border p-5 text-left shadow-sm，悬停上移 2px、shadow-md。
// 右下角 h-[92%] w-[78%] object-contain object-bottom opacity-65 的插画，上面盖一层从左到右变淡的渐变
// （from-white/80 via-white/42 to-transparent，深色 brand-900/72、/36），文字贴底：text-lg font-semibold 的标题与
// mt-2 text-xs leading-5 的说明。
func (a *AddGame) typeCard(c *ui.Context, t *theme.Theme, key string, art int, title, hint string, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Column().Grow(1).Basis(0).MinWidth(0).MinHeight(224).Padding(20).Radius(12).Clip().
		AlignItems(ui.Stretch).Justify(ui.End).FocusRing(false).Label(title).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700))
	hovered := btn.Hovered()
	lift := btn.AnimateWith("lift", theme.Bool(hovered), 150*time.Millisecond, theme.EaseTailwind)
	btn.Translate(0, -2*lift)
	if hovered {
		btn.Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
	} else {
		btn.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	}
	from, via, to := theme.White.Alpha(0.8), theme.White.Alpha(0.42), theme.White.Alpha(0)
	if t.Dark {
		from, via, to = theme.Brand900.Alpha(0.72), theme.Brand900.Alpha(0.36), theme.Brand900.Alpha(0)
	}
	btn.OnClick(onClick).Children(func() {
		// object-contain object-bottom：按比例缩进右下角 78% × 92% 的框里，在框里水平居中、贴着底边
		img := assets.Luna(art)
		boxW, boxH := btn.Bounds().W*0.78, btn.Bounds().H*0.92
		if iw, ih := img.Size(); iw > 0 && ih > 0 && boxW > 0 {
			s := min(boxW/float32(iw), boxH/float32(ih))
			w, h := float32(iw)*s, float32(ih)*s
			ui.Image(c.Key("art"), img).Absolute().Bottom(0).Right((boxW-w)/2).Size(w, h).Opacity(0.65).PassThrough()
		}
		ui.Box(c.Key("fade")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
			LinearGradient(ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{{Color: from, At: 0}, {Color: via, At: 0.5}, {Color: to, At: 1}}})
		// 文字是 relative 的，画在插画与渐变之上：MyGo 里绝对定位的元素画在流内元素之上，所以文字也绝对定位，贴着 p-5 的底边。
		ui.Column(c.Key("text")).Absolute().Left(20).Right(20).Bottom(20).Children(func() {
			components.Text(c, title, theme.TextLG, theme.WeightSemibold, t.TextStrong)
			components.TextMargin(c, hint, theme.FontStep{Size: 12, Line: 20}, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand200), 8, 0, 0, 0)
		})
	})
}

// field 是弹窗里 box-border block w-full rounded-lg border border-brand-300 bg-brand-50 p-3 text-brand-900
// dark:border-brand-600 dark:bg-brand-700 dark:text-white 的输入框（text-base）。
func field(c *ui.Context, t *theme.Theme, key string, value *string, label, placeholder string) ui.Element {
	in := ui.TextInputBase(c.Key(key), value).FillWidth().Padding(12).Radius(8).
		FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
		Background(t.Pick(theme.Brand50, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).Label(label)
	if placeholder != "" {
		in.Placeholder(placeholder)
	}
	if theme.UIFont != "" {
		in.Font(theme.UIFont)
	}
	return in
}

// labeled 是 mb-2 block text-sm font-medium text-brand-900 dark:text-white 的标题加上内容。
func labeled(c *ui.Context, t *theme.Theme, key, label string, content func()) {
	ui.Column(c.Key(key)).Children(func() {
		components.TextMargin(c, label, theme.TextSM, theme.WeightMedium, t.TextStrong, 0, 0, 8, 0)
		content()
	})
}

// primary 是 rounded-lg bg-neutral-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50。
func primary(c *ui.Context, key, label string, disabled bool, onClick func()) ui.Element {
	btn := solidButton(c, key, label, theme.Neutral600, theme.Neutral700, disabled, false, 20, 10)
	if !disabled {
		btn.OnClick(onClick)
	}
	return btn
}

func (a *AddGame) searchLabel(idle string) string {
	if a.loading {
		return locales.CommonSearching()
	}
	return idle
}

// localStep（space-y-6）：选择可执行文件（w-full py-4 bg-neutral-500 的按钮）、只读的路径、游戏名，底部返回、手动添加与搜索。
func (a *AddGame) localStep(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("local-step")).Gap(24).Children(func() {
		btn := ui.ButtonBase(c.Key("select-exe")).FillWidth().Padding(16, 0).Radius(8).Center().Gap(8).FocusRing(false)
		bg := theme.Neutral500
		if btn.Hovered() {
			bg = theme.Neutral600
		}
		btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { a.selectExecutable(c) }).Children(func() {
			components.Icon(c, "file-find", 20, theme.White)
			components.Text(c, locales.AddGameModalSelectExecutable(), theme.TextBase, theme.WeightNormal, theme.White)
		})
		exe := a.exe
		field(c, t, "exe", &exe, locales.AddGameModalExecutablePlaceholder(), locales.AddGameModalExecutablePlaceholder()).ReadOnly(true)
		labeled(c, t, "name", locales.AddGameModalGameName(), func() {
			field(c, t, "name-input", &a.name, locales.AddGameModalGameName(), "")
		})
		ui.Row(c).Justify(ui.SpaceBetween).Gap(16).Children(func() {
			outlineButton(c, t, "back", locales.CommonBack(), false, 20, 10).OnClick(func() { a.step = "type" })
			ui.Row(c).Gap(16).Children(func() {
				manual := outlineButton(c, t, "manual", locales.CommonManualAdd(), a.exe == "", 20, 10)
				if a.exe != "" {
					manual.OnClick(func() { a.step = "manual" })
				}
				primary(c, "search", a.searchLabel(locales.AddGameModalSearchMeta()), a.exe == "" || a.name == "" || a.loading, func() { a.searchByName(c) })
			})
		})
	})
}

// sourceSelect 是 BetterSelect 的元数据来源；tall 时是远程步骤的 [&>button]:h-12 rounded-lg py-0。
func (a *AddGame) sourceSelect(c *ui.Context, t *theme.Theme, opts []better.SelectOption, source enums.SourceType, tall bool) {
	p := better.SelectProps{Value: string(source), Options: opts, OnChange: func(v string) { a.source = enums.SourceType(v) }}
	if tall {
		p.Style = func(btn ui.Element) { btn.Height(48).Padding(0, 40, 0, 12).AlignItems(ui.Center).Radius(8) }
	}
	better.Select(c, t, "source", p)
}

// remoteStep（space-y-6）：grid sm:grid-cols-2 的来源与 ID（h-12，右边确定），“或按名称搜索”的分隔线，名称与搜索，
// 底部返回与手动添加。
func (a *AddGame) remoteStep(c *ui.Context, t *theme.Theme, opts []better.SelectOption, source enums.SourceType, onAdded func()) {
	ui.Column(c.Key("remote-step")).Gap(24).Children(func() {
		ui.Row(c).Gap(20).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				labeled(c, t, "source", locales.AddGameModalDataSource(), func() { a.sourceSelect(c, t, opts, source, true) })
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				labeled(c, t, "id", locales.AddGameModalGameId(), func() {
					ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
						field(c, t, "id-input", &a.manualID, locales.AddGameModalGameId(), locales.AddGameModalGameIdPlaceholder()).
							Height(48).Padding(0, 12).Grow(1).Basis(0).MinWidth(0)
						primary(c, "confirm-id", a.searchLabel(locales.CommonConfirm()), a.manualID == "" || source == enums.Local || a.loading,
							func() { a.searchByID(c, source, onAdded) }).Height(48).Shrink(0)
					})
				})
			})
		})
		// flex items-center gap-3：h-px flex-1 bg-brand-200 的两条线，中间 text-xs 的说明
		ui.Row(c.Key("or")).Gap(12).AlignItems(ui.Center).Children(func() {
			line := t.Pick(theme.Brand200, theme.Brand700)
			ui.Box(c).Height(1).Grow(1).Background(line)
			components.Text(c, locales.AddGameModalOrSearchByName(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
			ui.Box(c).Height(1).Grow(1).Background(line)
		})
		labeled(c, t, "name", locales.AddGameModalGameName(), func() {
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() { // flex gap-3：按钮拉伸到输入框的高度
				field(c, t, "name-input", &a.name, locales.AddGameModalGameName(), "").Grow(1).Basis(0).MinWidth(0)
				primary(c, "search", a.searchLabel(locales.AddGameModalSearchMeta()), a.name == "" || a.loading, func() { a.searchByName(c) })
			})
		})
		ui.Row(c).Justify(ui.SpaceBetween).Gap(16).Children(func() {
			outlineButton(c, t, "back", locales.CommonBack(), false, 20, 10).OnClick(func() { a.step = "type" })
			outlineButton(c, t, "manual", locales.CommonManualAdd(), false, 20, 10).OnClick(func() { a.step = "manual" })
		})
	})
}

// resultsStep 是 MetadataSearchResultsStep：底部 border-t pt-4 的“← 返回”与“都不是？手动填写 / 按 ID 搜索”。
func (a *AddGame) resultsStep(c *ui.Context, t *theme.Theme, onAdded func()) {
	a.resultsUI.View(c, t, SearchResultsProps{
		Results:  a.results,
		Covers:   a.env.Covers,
		Disabled: a.loading,
		OnSelect: func(r vo.GameMetadataFromWebVO) { a.selectResult(c, r, onAdded) },
		OnRemove: func(i int) { a.results = slices.Delete(slices.Clone(a.results), i, i+1) },
		Footer: func() {
			ui.Row(c.Key("footer")).Padding(16, 0, 0, 0).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
				linkButton(c, t, "back", "←"+locales.AddGameModalGoBack(), false, func() { a.step = a.entryStep() })
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					components.Text(c, locales.AddGameModalNoneOfAbove(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
					linkButton(c, t, "fill", locales.AddGameModalFillManually(), false, func() { a.step = "manual" })
					linkButton(c, t, "by-id", locales.AddGameModalSearchById(), true, func() {
						if a.remote {
							a.step = "remote"
						} else {
							a.step = "id"
						}
					})
				})
			})
		},
	})
}

// linkButton 是 text-sm text-brand-500 hover:text-brand-700 的文字按钮（accent 时 text-neutral-600 hover:text-neutral-800）。
func linkButton(c *ui.Context, t *theme.Theme, key, label string, accent bool, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).FocusRing(false).Label(label)
	fg, hover := t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.Brand700, theme.Brand200)
	if accent {
		fg, hover = t.Pick(theme.Neutral600, theme.Neutral400), t.Pick(theme.Neutral800, theme.Neutral300)
	}
	if btn.Hovered() {
		fg = hover
	}
	btn.OnClick(onClick).Children(func() { components.Text(c, label, theme.TextSM, theme.WeightNormal, fg).NoWrap() })
}

// idStep（space-y-6）：来源与 ID，右下角返回与确定。
func (a *AddGame) idStep(c *ui.Context, t *theme.Theme, opts []better.SelectOption, source enums.SourceType, onAdded func()) {
	ui.Column(c.Key("id-step")).Gap(24).Children(func() {
		labeled(c, t, "source", locales.AddGameModalDataSource(), func() { a.sourceSelect(c, t, opts, source, false) })
		labeled(c, t, "id", locales.AddGameModalGameId(), func() {
			field(c, t, "id-input", &a.manualID, locales.AddGameModalGameId(), locales.AddGameModalGameIdPlaceholder())
		})
		ui.Row(c).Justify(ui.End).Gap(16).Children(func() {
			outlineButton(c, t, "back", locales.CommonBack(), false, 20, 10).OnClick(func() { a.step = "results" })
			primary(c, "confirm", a.searchLabel(locales.CommonConfirm()), a.manualID == "" || source == enums.Local || a.loading,
				func() { a.searchByID(c, source, onAdded) })
		})
	})
}

// manualStep（space-y-4）：说明，名称、封面（输入框加“选择”按钮与 mt-1 text-xs 的提示）、开发商、简介（rows=3），
// 右下角返回与保存。
func (a *AddGame) manualStep(c *ui.Context, t *theme.Theme, source enums.SourceType, onAdded func()) {
	ui.Column(c.Key("manual-step")).Gap(16).Children(func() {
		info := locales.AddGameModalManualFillInfo()
		if a.remote {
			info = locales.AddGameModalRemoteManualFillInfo()
		}
		components.Text(c, info, theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
		labeled(c, t, "name", locales.AddGameModalGameName(), func() {
			field(c, t, "name-input", &a.name, locales.AddGameModalGameName(), "")
		})
		labeled(c, t, "cover", locales.AddGameModalCoverImage(), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
				field(c, t, "cover-input", &a.cover, locales.AddGameModalCoverImage(), locales.AddGameModalCoverPlaceholder()).Grow(1).Basis(0).MinWidth(0)
				// rounded-lg bg-brand-100 px-4 py-2 text-brand-700 hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-300 dark:hover:bg-brand-600
				btn := ui.ButtonBase(c.Key("select-cover")).Padding(8, 16).Radius(8).Center().FocusRing(false)
				bg := t.Pick(theme.Brand100, theme.Brand700)
				if btn.Hovered() {
					bg = t.Pick(theme.Brand200, theme.Brand600)
				}
				btn.Background(bg).OnClick(func() { a.selectCover(c) }).Children(func() {
					components.Text(c, locales.CommonSelect(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand300)).NoWrap()
				})
			})
			components.TextMargin(c, locales.AddGameModalCoverHint(), theme.TextXS, theme.WeightNormal, theme.Brand500, 4, 0, 0, 0)
		})
		labeled(c, t, "company", locales.AddGameModalDeveloper(), func() {
			field(c, t, "company-input", &a.company, locales.AddGameModalDeveloper(), "")
		})
		labeled(c, t, "summary", locales.AddGameModalSummary(), func() {
			better.Textarea(c, t, "summary-input", &a.summary, better.TextareaProps{Label: locales.AddGameModalSummary(), Rows: 3}).
				Padding(12).Radius(8).Background(t.Pick(theme.Brand50, theme.Brand700))
		})
		ui.Row(c).Justify(ui.End).Gap(16).Margin(8, 0, 0, 0).Children(func() {
			outlineButton(c, t, "back", locales.CommonBack(), false, 20, 10).OnClick(func() { a.step = a.entryStep() })
			label := locales.CommonSave()
			if a.loading {
				label = locales.CommonSaving()
			}
			primary(c, "save", label, a.name == "" || a.loading, func() { a.manualSave(c, source, onAdded) })
		})
	})
}
