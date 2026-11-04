package importer

import (
	"encoding/base64"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/utils/metadata"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/gamefield"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/tagname"
	"lunabox/desktop/ui/theme"
)

// 手动填写（原版 AddGameModal 的 manual 步骤，只有名称、封面、开发商与简介）扩展成与详情页“编辑”标签页对齐的表单：
// 左侧是封面（点击选择、粘贴图片），右侧是名称、别名、封面 URL、开发商、评分、发售日期、标签、简介与元数据来源。

// manualWidth 是手动填写时弹窗的最大宽度（max-w-4xl 左右，放得下左侧封面与右侧字段）。
const manualWidth = 928

// manualCoverWidth 是左侧封面的宽度（3:4）。
const manualCoverWidth = 208

// manualForm 是手动填写的字段（名称与其他步骤共用 AddGame.name）。
type manualForm struct {
	cover, company, summary string
	coverBusy               bool // 正在保存选择或粘贴的图片

	aliases     []string
	aliasAdding bool
	aliasDraft  string

	rating      string // 输入框里的文字
	ratingValue float64

	releaseDate string // YYYY-MM-DD
	dateOpen    bool
	dateMonth   time.Time

	tags           []string
	tagInput       string
	tagSuggestions []string

	sources         []models.GameMetadataSource
	defaultSource   enums.SourceType // 为空或已删除时是第一个
	sourceDraftType enums.SourceType
	sourceDraftID   string
}

// toManual 进入手动填写：取消进行中的搜索；远程游戏在上一步填了 ID 时，把它带成第一个元数据来源。
func (a *AddGame) toManual() {
	a.cancelSearch()
	a.step = "manual"
	f := &a.form
	if !a.remote || len(f.sources) > 0 {
		return
	}
	_, source := a.sourceOptions()
	if id := strings.TrimSpace(a.manualID); id != "" && source != enums.Local {
		f.sources = []models.GameMetadataSource{{SourceType: source, SourceID: id}}
		f.defaultSource = source
	}
}

// selectCover 对应 handleSelectCoverImage：选择的图片以临时 ID 存进封面目录，添加游戏时改成游戏 ID。
func (a *AddGame) selectCover(c *ui.Context) {
	svc, gen, f := a.env.Svc.Game, a.gen, &a.form
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
				f.cover = url
			}
		}
	})
}

// pasteCover 在剪贴板里有图片时把它存为封面并返回 true（接管这次粘贴）；只有文字时返回 false，照常粘贴。
func (a *AddGame) pasteCover(c *ui.Context) bool {
	if a.env.Win == nil {
		return false // 没有窗口（测试）时读不了系统剪贴板
	}
	png := mygo.Clipboard.ReadImage()
	if len(png) == 0 {
		return false
	}
	a.saveCoverPNG(c, png)
	return true
}

// saveCoverPNG 以 data URL 交给 SaveCoverImageDataURLWithTempID（同详情页的 importCoverDataURL）。
func (a *AddGame) saveCoverPNG(c *ui.Context, png []byte) {
	svc, gen, f := a.env.Svc.Game, a.gen, &a.form
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	f.coverBusy = true
	a.env.Async(func() func() {
		url, err := svc.SaveCoverImageDataURLWithTempID(dataURL)
		return func() {
			if gen != a.gen {
				return
			}
			f.coverBusy = false
			if err != nil {
				log.Printf("add game: paste cover: %v", err)
				a.env.Toast(c, env.Error(locales.GameEditImportFromClipboardFailed()))
				return
			}
			f.cover = url
		}
	})
}

// defaultSourceOf 是保存时的默认来源：星标的来源，没有时是第一个。
func (f *manualForm) defaultSourceOf(sources []models.GameMetadataSource) (models.GameMetadataSource, bool) {
	if len(sources) == 0 {
		return models.GameMetadataSource{}, false
	}
	for _, s := range sources {
		if s.SourceType == f.defaultSource {
			return s, true
		}
	}
	return sources[0], true
}

// manualSave 对应 handleManualSave：填了 ID 的来源都关联到游戏，星标的作为默认来源，没有时是本地来源；
// 标签作为用户标签保存。输入框里还没确认的别名与标签也一并保存（点保存时输入框还没来得及失去焦点）。
func (a *AddGame) manualSave(c *ui.Context, onAdded func()) {
	name := strings.TrimSpace(a.name)
	if name == "" {
		a.env.Toast(c, env.Error(locales.AddGameModalToastFillGameName()))
		return
	}
	if !a.remote && a.exe == "" {
		a.env.Toast(c, env.Error(locales.AddGameModalToastSelectExecutableRequired()))
		return
	}
	f := &a.form
	aliases := slices.Clone(f.aliases)
	if draft := strings.TrimSpace(f.aliasDraft); f.aliasAdding && draft != "" &&
		!slices.ContainsFunc(aliases, func(s string) bool { return strings.EqualFold(s, draft) }) {
		aliases = append(aliases, draft)
	}
	names := slices.Clone(f.tags)
	if draft := strings.TrimSpace(f.tagInput); draft != "" &&
		!slices.ContainsFunc(names, func(s string) bool { return strings.EqualFold(s, draft) }) {
		names = append(names, draft)
	}
	tags := make([]metadata.TagItem, 0, len(names))
	for _, n := range names {
		tags = append(tags, metadata.TagItem{Name: n, Source: "user", Weight: 1})
	}

	g := models.Game{
		Name: name, CoverURL: f.cover, Company: f.company, Summary: f.summary, Aliases: aliases,
		Rating: f.ratingValue, ReleaseDate: f.releaseDate, SourceType: enums.Local, Status: enums.StatusNotStarted,
	}
	var sources []models.GameMetadataSource
	for _, s := range f.sources {
		if id := strings.TrimSpace(s.SourceID); id != "" {
			sources = append(sources, models.GameMetadataSource{SourceType: s.SourceType, SourceID: id})
		}
	}
	if def, ok := f.defaultSourceOf(sources); ok {
		g.SourceType, g.SourceID, g.MetadataSources = def.SourceType, def.SourceID, sources
	}
	if a.remote {
		g.Status = enums.StatusWantToPlay
	} else {
		g.Path = a.exe
	}
	a.save(c, vo.GameMetadataFromWebVO{Source: g.SourceType, Game: g, Tags: tags}, onAdded)
}

// manualStep：说明，可以滚动的两栏（左侧封面，右侧字段），右下角返回与保存。
func (a *AddGame) manualStep(c *ui.Context, t *theme.Theme, onAdded func()) {
	_, winH := c.Size()
	// 窗口减去顶栏、弹窗外边距与内边距、标题、说明与底部按钮，至少留 240。
	bodyH := max(240, winH-env.TopbarHeight-300)
	ui.Column(c.Key("manual-step")).Gap(16).Children(func() {
		info := locales.AddGameModalManualFillInfo()
		if a.remote {
			info = locales.AddGameModalRemoteManualFillInfo()
		}
		components.Text(c, info, theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
		// 滚动区伸进面板右侧的内边距，滚动条落在那里，字段的右边与底部按钮对齐。
		ui.Scroll(c.Key("manual-body")).MaxHeight(bodyH).Margin(0, -14, 0, 0).Children(func() {
			// 四周 2px 给输入框的 focus ring 留出位置，右侧再留出滚动条的位置
			ui.Row(c).Padding(2, 14, 2, 2).Gap(24).AlignItems(ui.Start).Children(func() {
				ui.Column(c.Key("cover-col")).Width(manualCoverWidth).Shrink(0).Gap(8).Children(func() { a.coverColumn(c, t) })
				ui.Column(c.Key("fields")).Grow(1).Basis(0).MinWidth(0).Gap(16).Children(func() { a.manualFields(c, t) })
			})
		})
		ui.Row(c).Justify(ui.End).Gap(16).Children(func() {
			outlineButton(c, t, "back", locales.CommonBack(), false, 20, 10).OnClick(func() { a.step = a.entryStep() })
			label := locales.CommonSave()
			if a.loading {
				label = locales.CommonSaving()
			}
			primary(c, "save", label, strings.TrimSpace(a.name) == "" || a.loading, func() { a.manualSave(c, onAdded) })
		})
	})
}

// coverColumn 是左侧的封面：aspect-[3/4] 的按钮（点击选择图片，获得焦点时 Ctrl+V 粘贴图片）。URL 输入框在右侧的字段里。
func (a *AddGame) coverColumn(c *ui.Context, t *theme.Theme) {
	f := &a.form
	btn := ui.ButtonBase(c.Key("cover-box")).FillWidth().AspectRatio(3.0 / 4).Radius(12).Clip().Column().Center().Gap(10).
		Padding(12).FocusRing(false).Label(locales.AddGameModalCoverImage())
	hovered := btn.Hovered()
	border := t.Pick(theme.Brand300, theme.Brand600)
	if hovered || btn.Focused() {
		border = t.Pick(theme.Brand500, theme.Brand400)
	}
	btn.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if f.cover == "" {
		btn.BorderStyle(ui.BorderDashed)
		bg := t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))
		if hovered {
			bg = t.Pick(theme.Brand100, theme.Brand700.Alpha(0.7))
		}
		btn.Background(bg)
	}
	btn.OnClick(func() { a.selectCover(c) }).OnShortcut(ui.Cmd, ui.KeyV, func() { a.pasteCover(c) })
	btn.Children(func() {
		muted := t.Pick(theme.Brand500, theme.Brand400)
		if f.cover == "" {
			components.Icon(c, "image-search-outline", 32, muted)
			components.Text(c, locales.NativeAddGameCoverEmpty(), theme.TextXS, theme.WeightNormal, muted).TextAlign(ui.Center)
		} else {
			ui.Box(c.Key("image")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Children(func() {
				coverImage(c, a.env.Covers, models.Game{CoverURL: f.cover}, 30)
			})
			if hovered { // 悬停时盖一层，提示可以更换
				ui.Column(c.Key("replace")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Center().Gap(8).
					Background(theme.Black.Alpha(0.45)).Children(func() {
					components.Icon(c, "image-move", 28, theme.White)
					components.Text(c, locales.NativeAddGameCoverEmpty(), theme.TextXS, theme.WeightNormal, theme.White).TextAlign(ui.Center)
				})
			}
		}
		if f.coverBusy {
			ui.Column(c.Key("busy")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Center().
				Background(theme.Black.Alpha(0.45)).Children(func() { components.Spinner(c, 24, theme.White) })
		}
	})
}

// manualFields 是右侧的字段，与详情页“编辑”标签页的同名字段相同。
func (a *AddGame) manualFields(c *ui.Context, t *theme.Theme) {
	f := &a.form
	field := func(key, label string, build func()) {
		ui.Column(c.Key(key)).Children(func() {
			gamefield.Label(c, t, label)
			build()
		})
	}
	field("name", locales.AddGameModalGameName(), func() {
		better.Input(c, t, "input", &a.name, better.InputProps{Label: locales.AddGameModalGameName()})
	})
	field("aliases", locales.GameEditAliases(), func() {
		gamefield.Aliases(c, t, "aliases", gamefield.AliasesProps{
			Aliases: f.aliases, Adding: &f.aliasAdding, Draft: &f.aliasDraft,
			OnChange: func(aliases []string) { f.aliases = aliases },
		})
		gamefield.Hint(c, t, locales.GameEditAliasHint(), false)
	})
	// 封面 URL：可以直接粘贴图片（同详情页的封面输入框），选择本地图片点左侧的封面。
	field("cover", locales.GameEditCover(), func() {
		better.Input(c, t, "input", &f.cover, better.InputProps{Placeholder: locales.GameEditCoverPlaceholder(), Label: locales.GameEditCover()}).
			OnPaste(func(string) bool { return a.pasteCover(c) })
		gamefield.Hint(c, t, locales.GameEditCoverHint(), true)
	})
	field("company", locales.AddGameModalDeveloper(), func() {
		better.Input(c, t, "input", &f.company, better.InputProps{Label: locales.AddGameModalDeveloper()})
	})
	// 评分与发售日期并排（grid-cols-2 gap-4）
	ui.Row(c.Key("rating-date")).Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Column(c.Key("rating")).Grow(1).Basis(0).MinWidth(0).Children(func() {
			gamefield.Label(c, t, locales.GameEditRating())
			gamefield.Rating(c, t, &f.rating, func(r float64) { f.ratingValue = r })
		})
		ui.Column(c.Key("release-date")).Grow(1).Basis(0).MinWidth(0).Children(func() {
			gamefield.Label(c, t, locales.GameEditReleaseDate())
			gamefield.DatePicker(c, t, "picker", gamefield.DatePickerProps{
				Value: f.releaseDate, Label: locales.GameEditReleaseDate(), ClearLabel: locales.GameEditClearReleaseDate(),
				CanClear: f.releaseDate != "", Open: &f.dateOpen, Month: &f.dateMonth,
				OnChange: func(v string) { f.releaseDate = v },
			})
		})
	})
	field("tags", locales.NativeTags(), func() {
		gamefield.Tags(c, t, "tags", gamefield.TagsProps{
			Tags: f.tags, Input: &f.tagInput, Suggestions: f.tagSuggestions, Translate: tagname.Enabled(a.env.Config()),
			OnInput: func(q string) {
				a.env.SearchTags(q, f.tags, func(out []string) {
					if q == f.tagInput {
						f.tagSuggestions = out
					}
				})
			},
			OnChange: func(tags []string) { f.tags = tags },
		})
		gamefield.Hint(c, t, locales.NativeAddGameTagsHint(), true)
	})
	field("summary", locales.AddGameModalSummary(), func() {
		better.Textarea(c, t, "input", &f.summary, better.TextareaProps{Label: locales.AddGameModalSummary(), Rows: 4})
	})
	a.sourcesField(c, t)
}

// sourcesField 是“元数据来源”：每个来源一行（名称、ID、设为默认、删除），下面是选择来源、填 ID 后添加的一行。
func (a *AddGame) sourcesField(c *ui.Context, t *theme.Theme) {
	f := &a.form
	def, _ := f.defaultSourceOf(f.sources)
	var options []better.SelectOption
	for _, s := range gamefield.MetadataSourceTypes {
		if !slices.ContainsFunc(f.sources, func(m models.GameMetadataSource) bool { return m.SourceType == s }) {
			options = append(options, better.SelectOption{Label: SourceLabel(s), Value: string(s)})
		}
	}
	if len(options) > 0 && !slices.ContainsFunc(options, func(o better.SelectOption) bool { return o.Value == string(f.sourceDraftType) }) {
		f.sourceDraftType = enums.SourceType(options[0].Value)
	}
	ui.Column(c.Key("sources")).Gap(8).Children(func() {
		gamefield.Label(c, t, locales.GameEditMetadataSources())
		for i := range f.sources {
			src := f.sources[i].SourceType
			ui.Row(c.Key(string(src))).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(144).Shrink(0).Children(func() { // 与下面添加行的来源下拉框同宽，ID 输入框对齐
					components.Text(c, SourceLabel(src), theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100)).SingleLine()
				})
				ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					better.Input(c, t, "id", &f.sources[i].SourceID, better.InputProps{
						Placeholder: locales.GameEditSourceIdPlaceholder(), Label: SourceLabel(src) + " ID",
					})
				})
				isDefault := src == def.SourceType
				star := better.ButtonProps{Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "star-outline"}
				label := locales.GameEditSetDefaultSource()
				if isDefault {
					// className="!text-warning-500 dark:!text-warning-400"
					warning := t.Pick(ui.Hex("#f59e0b"), ui.Hex("#fbbf24"))
					star.Icon, label = "", locales.GameEditDefaultSource()
					star.Children = func(ui.Color) { components.Icon(c, "star", 16, warning) }
				}
				btn := better.Button(c, t, "default", star).Label(label)
				if star.Children != nil {
					btn.Width(32).Padding(0)
				}
				btn.OnClick(func() { f.defaultSource = src })
				better.Button(c, t, "delete", better.ButtonProps{Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "delete-outline"}).
					Label(locales.GameEditDeleteSource()).OnClick(func() {
					f.sources = slices.DeleteFunc(slices.Clone(f.sources), func(m models.GameMetadataSource) bool { return m.SourceType == src })
				})
			})
		}
		if len(options) > 0 {
			ui.Row(c.Key("add-source")).Gap(8).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Width(144).Shrink(0).Children(func() {
					better.Select(c, t, "source-type", better.SelectProps{
						Value: string(f.sourceDraftType), Options: options,
						OnChange: func(v string) { f.sourceDraftType = enums.SourceType(v) },
					})
				})
				ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					better.Input(c, t, "source-id", &f.sourceDraftID, better.InputProps{Placeholder: locales.GameEditSourceIdPlaceholder()})
				})
				id := strings.TrimSpace(f.sourceDraftID)
				better.Button(c, t, "add", better.ButtonProps{
					Variant: better.ButtonSecondary, Icon: "plus", Label: locales.GameEditAddSource(), Disabled: id == "",
					OnClick: func() {
						f.sources = append(slices.Clone(f.sources), models.GameMetadataSource{SourceType: f.sourceDraftType, SourceID: id})
						f.sourceDraftID = ""
					},
				})
			})
		}
		gamefield.Hint(c, t, locales.NativeAddGameSourcesHint(), true)
	})
}
