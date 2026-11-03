// Package edit 对应原版 components/panel/GameEditPanel.tsx（详情页“编辑”标签页），以及详情页传给它的处理函数
// （routes/game.tsx 的 handleSelect*、元数据来源的增删改、重新搜索、从远端更新、删除游戏）和它们的弹窗
// （ConfirmModal、MetadataFieldSelectModal、MetadataSourceSearchModal）。
//
// 字段的修改通过 panel.Game.OnChange 交给页面（500ms 后自动保存）；调用了会直接改数据库的服务（换封面、
// 元数据来源、从远端更新）之后调用 OnSaved 让页面重新读取。
package edit

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/importer"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// sourceSaveDelay 是 METADATA_SOURCE_AUTO_SAVE_DELAY：来源 ID 停止输入 500ms 后自动保存。
const sourceSaveDelay = 500 * time.Millisecond

// searchMode 是 handleSearchMetadataByName 的 mode。
type searchMode int

const (
	searchSourceRelations searchMode = iota // "source-relations"：只更新来源关联
	searchUpdateMetadata                    // "update-metadata"：同时从选中的来源更新元数据
)

// pendingSave 是一条等待自动保存的来源 ID。
type pendingSave struct {
	id  string
	due time.Time
}

// State 是面板跨帧保留的状态，由详情页持有；进入另一个游戏时清零。
type State struct {
	gameID string // 状态所属的游戏；异步结果回来时游戏已切换则丢弃

	rating      string  // 评分输入框里的文字（number input 允许“8.”这样的中间状态）
	ratingValue float64 // rating 对应的评分；页面里的评分从别处改变时重新填入

	aliasAdding bool // aliasAddingGameId === game.id
	aliasDraft  string

	dateOpen  bool
	dateMonth time.Time

	downloadingCover bool

	drawerOpen      bool
	drawerWasOpen   bool
	sourceDraftType enums.SourceType
	sourceDraftID   string
	sourceEdits     map[enums.SourceType]string // sourceIDEdits
	sourceSaves     map[enums.SourceType]pendingSave
	busySource      string // "add"、"<source>:default"、"<source>:delete"
	searching       bool   // isSearchingMetadataByName

	fieldsOpen bool                        // isMetadataFieldModalOpen
	fields     []enums.MetadataUpdateField // 弹窗里勾选的字段
	lastFields []enums.MetadataUpdateField // selectedMetadataFields：上次更新用的字段
	updating   bool                        // isUpdatingFromRemote

	searchOpen bool // isMetadataSearchModalOpen
	searchMode searchMode
	results    []vo.GameMetadataFromWebVO
	applying   bool // isApplyingMetadataResult
	resultsUI  importer.SearchResults

	deleteOpen bool

	patches []func(*models.Game) // 异步选择（文件对话框）的结果，下一遍构建时合并进页面里的游戏
}

// init 在第一次构建某个游戏时设置初值。
func (s *State) init(g *models.Game) {
	if s.gameID == g.ID {
		return
	}
	*s = State{
		gameID:          g.ID,
		rating:          formatRating(g.Rating),
		ratingValue:     g.Rating,
		sourceDraftType: enums.Bangumi,
		sourceEdits:     map[enums.SourceType]string{},
		sourceSaves:     map[enums.SourceType]pendingSave{},
		lastFields:      slices.Clone(enums.DefaultMetadataUpdateFields),
	}
}

// patch 把异步得到的修改排进队列；游戏已切换时丢弃。
func (s *State) patch(gameID string, fn func(*models.Game)) {
	if s.gameID == gameID {
		s.patches = append(s.patches, fn)
	}
}

// editor 是一遍构建里的上下文（Context 作为参数传递，不存进结构体）。g 是页面里的游戏加上这一遍已经提交的修改，同一遍里的多处修改不会互相覆盖。
type editor struct {
	t    *theme.Theme
	s    *State
	p    panel.Game
	g    models.Game
	winW float32
}

// set 修改游戏并交给页面（updateGameState）。
func (e *editor) set(fn func(g *models.Game)) {
	fn(&e.g)
	e.p.OnChange(e.g)
}

// toast 显示提示（react-hot-toast）。
func (e *editor) toast(c *ui.Context, n env.Notice) { e.p.Env.Toast(c, n) }

// Flush 由详情页在离开页面、进入另一个游戏前调用：立即保存待保存的来源 ID（原版依赖 game.id 的 effect 卸载时补存）。
// 抽屉打开时离开页面，面板之后不再构建，到期的保存不会再执行。
func Flush(s *State, p panel.Game) {
	if p.Game == nil || p.Game.ID != s.gameID || len(s.sourceSaves) == 0 {
		return
	}
	e := &editor{s: s, p: p, g: *p.Game}
	e.flushSourceSaves()
}

// View 构建面板：glass-panel w-full min-w-0 bg-white dark:bg-brand-800 p-8 rounded-lg，内容 space-y-6。
// 抽屉与弹窗在最后构建，浮在页面之上。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Game) {
	if p.Game == nil {
		return
	}
	s.init(p.Game)
	e := &editor{t: t, s: s, p: p, g: *p.Game}
	e.winW, _ = c.Size()
	if len(s.patches) > 0 {
		patches := s.patches
		s.patches = nil
		e.set(func(g *models.Game) {
			for _, fn := range patches {
				fn(g)
			}
		})
	}
	if s.drawerWasOpen && !s.drawerOpen {
		e.flushSourceSaves() // 抽屉关上后不再构建来源输入框，先把待保存的 ID 存掉
	}
	s.drawerWasOpen = s.drawerOpen
	e.tickSourceSaves(c)

	// glass-panel：玻璃模式下换成半透明底色并加边框
	panelBg, panelBorder := t.GlassPanel(t.Pick(theme.White, theme.Brand800), ui.Transparent)
	root := ui.Column(c.Key("edit-panel")).FillWidth().MinWidth(0).Padding(32).Radius(8).Background(panelBg)
	if t.Glass {
		root.Border(t.HL, panelBorder)
	}
	root.Gap(24).Children(func() {
		e.textField(c, "name", locales.GameEditName(), e.g.Name, func(g *models.Game, v string) { g.Name = v })
		e.aliasesField(c)
		e.coverField(c)
		e.coverSourceField(c)
		e.textField(c, "company", locales.GameEditDeveloper(), e.g.Company, func(g *models.Game, v string) { g.Company = v })
		e.ratingAndReleaseDate(c)
		e.pathFields(c)
		e.savePathField(c)
		e.summaryField(c)
		e.metadataSection(c)
		e.switchCard(c, "metadata-lock", locales.GameEditMetadataLock(), locales.GameEditMetadataLockHint(),
			e.g.MetadataLocked, func(g *models.Game, v bool) { g.MetadataLocked = v })
		e.switchCard(c, "is-nsfw", locales.GameEditIsNsfw(), locales.GameEditIsNsfwHint(),
			e.g.IsNSFW, func(g *models.Game, v bool) { g.IsNSFW = v })
		e.footerButtons(c)
	})

	e.metadataDrawer(c)
	e.deleteModal(c)
	e.fieldSelectModal(c)
	e.searchModal(c)
}

// textField 是 label + BetterInput 的一行（游戏名称、开发商）。
func (e *editor) textField(c *ui.Context, key, label, value string, apply func(g *models.Game, v string)) {
	t := e.t
	ui.Column(c.Key(key)).Children(func() {
		fieldLabel(c, t, label)
		v := value
		if better.Input(c, t, "input", &v, better.InputProps{Label: label}).Changed() {
			e.set(func(g *models.Game) { apply(g, v) })
		}
	})
}

// coverField 是“封面图片”：可以粘贴图片的 BetterActionInput，右侧选择图片与下载远程封面。
func (e *editor) coverField(c *ui.Context) {
	t, s := e.t, e.s
	remote := e.g.CoverSourceURL
	if isRemoteCoverURL(e.g.CoverURL) {
		remote = e.g.CoverURL
	}
	ui.Column(c.Key("cover")).Children(func() {
		fieldLabel(c, t, locales.GameEditCover())
		v := e.g.CoverURL
		in := actionInput(c, t, "input", &v, actionInputProps{
			Placeholder: locales.GameEditCoverPlaceholder(), Label: locales.GameEditCover(), OnPaste: func(text string) bool { return e.pasteCover(c, text) },
		},
			inputAction{Icon: "image-search-outline", Label: locales.GameEditSelectImage(), OnClick: func() { e.selectCoverImage(c) }},
			inputAction{Icon: "download", Label: locales.GameEditDownloadCover(), Disabled: !isRemoteCoverURL(remote) || s.downloadingCover,
				Loading: s.downloadingCover, OnClick: func() { e.downloadCover(c, remote) }},
		)
		if in.Changed() {
			e.set(func(g *models.Game) { g.CoverURL = v })
		}
		hint(c, t, locales.GameEditCoverHint(), false)
	})
}

// coverSourceField 是“远端封面源”，右侧下载远程封面。
func (e *editor) coverSourceField(c *ui.Context) {
	t, s := e.t, e.s
	remote := e.g.CoverSourceURL
	ui.Column(c.Key("cover-source")).Children(func() {
		fieldLabel(c, t, locales.GameEditCoverSource())
		v := e.g.CoverSourceURL
		in := actionInput(c, t, "input", &v, actionInputProps{Placeholder: locales.GameEditCoverSourcePlaceholder(), Label: locales.GameEditCoverSource()},
			inputAction{Icon: "download", Label: locales.GameEditDownloadCover(), Disabled: !isRemoteCoverURL(remote) || s.downloadingCover,
				Loading: s.downloadingCover, OnClick: func() { e.downloadCover(c, remote) }},
		)
		if in.Changed() {
			e.set(func(g *models.Game) { g.CoverSourceURL = v })
		}
		hint(c, t, locales.GameEditCoverSourceHint(), false)
	})
}

// ratingAndReleaseDate：grid grid-cols-1 gap-4 lg:grid-cols-2，左侧评分，右侧发售日期（断点按窗口宽度）。
func (e *editor) ratingAndReleaseDate(c *ui.Context) {
	wide := e.winW >= 1024
	var grid ui.Element
	if wide {
		grid = ui.Row(c.Key("rating-date")).Gap(16).AlignItems(ui.Start)
	} else {
		grid = ui.Column(c.Key("rating-date")).Gap(16)
	}
	grid.Children(func() {
		cell := func(key string, build func()) {
			col := ui.Column(c.Key(key)).MinWidth(0)
			if wide {
				col.Grow(1).Basis(0)
			}
			col.Children(build)
		}
		cell("rating", func() { e.ratingField(c) })
		cell("release-date", func() { e.releaseDateField(c) })
	})
}

// ratingField 对应评分：BetterInput type="number" min=0 max=10 step=0.1（flex-1），右侧 text-sm 的“/ 10”。
func (e *editor) ratingField(c *ui.Context) {
	t, s := e.t, e.s
	if e.g.Rating != s.ratingValue {
		s.rating, s.ratingValue = formatRating(e.g.Rating), e.g.Rating
	}
	fieldLabel(c, t, locales.GameEditRating())
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		prev := s.rating
		var in ui.Element
		ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			in = better.Input(c, t, "input", &s.rating, better.InputProps{Placeholder: locales.GameEditRatingPlaceholder(), Label: locales.GameEditRating()})
		})
		if in.Changed() {
			e.changeRating(c, prev)
		}
		components.Text(c, "/ 10", theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
	})
	hint(c, t, locales.GameEditRatingHint(), true)
}

// changeRating 对应 number input 的 onChange：空为 0，否则限制在 0–10。number input 不接受的字符不会进入输入框。
func (e *editor) changeRating(c *ui.Context, prev string) {
	s := e.s
	text := strings.TrimSpace(s.rating)
	rating := 0.0
	if text != "" {
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			if text != "-" && text != "." && !strings.HasSuffix(text, "e") {
				s.rating = prev
			}
			return
		}
		rating = min(10, max(0, v))
		if rating != v {
			s.rating = formatRating(rating)
		}
	}
	s.ratingValue = rating
	e.set(func(g *models.Game) { g.Rating = rating })
}

// formatRating 是 value={game.rating > 0 ? game.rating : ""}。
func formatRating(r float64) string {
	if r <= 0 {
		return ""
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// releaseDateField 是发售日期选择器；数据库里的日期不是 YYYY-MM-DD 时提示原始值。
func (e *editor) releaseDateField(c *ui.Context) {
	t, s := e.t, e.s
	value := formatDateInputValue(e.g.ReleaseDate)
	fieldLabel(c, t, locales.GameEditReleaseDate())
	datePicker(c, t, "picker", datePickerProps{
		Value: value, Label: locales.GameEditReleaseDate(), ClearLabel: locales.GameEditClearReleaseDate(), CanClear: e.g.ReleaseDate != "",
		Open: &s.dateOpen, Month: &s.dateMonth,
		OnChange: func(v string) { e.set(func(g *models.Game) { g.ReleaseDate = v }) },
	})
	if e.g.ReleaseDate != "" && value == "" {
		hint(c, t, locales.GameEditReleaseDateRawHint(e.g.ReleaseDate), true)
	}
}

// pathFields：space-y-3，grid gap-3 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)] 的游戏目录与可执行文件，下面是说明。
func (e *editor) pathFields(c *ui.Context) {
	t := e.t
	g := e.g
	wide := e.winW >= 1024
	ui.Column(c.Key("paths")).Gap(12).Children(func() {
		var grid ui.Element
		if wide {
			grid = ui.Row(c.Key("grid")).Gap(12).AlignItems(ui.Start)
		} else {
			grid = ui.Column(c.Key("grid")).Gap(12)
		}
		grid.Children(func() {
			dir := ui.Column(c.Key("dir")).MinWidth(0)
			exe := ui.Column(c.Key("exe")).MinWidth(0)
			if wide {
				dir.Grow(2).Basis(0)
				exe.Grow(1).Basis(0).MinWidth(288)
			}
			dir.Children(func() {
				fieldLabel(c, t, locales.GameEditGameDirectory())
				v := g.GameDirectory
				open := g.GameDirectory
				if open == "" {
					open = g.Path
				}
				in := actionInput(c, t, "input", &v, actionInputProps{Placeholder: locales.GameEditGameDirectoryPlaceholder(), Label: locales.GameEditGameDirectory()},
					inputAction{Icon: "folder-search-outline", Label: locales.GameEditSelectFolder(), OnClick: func() { e.selectGameDirectory(c) }},
					inputAction{Icon: "folder-open-outline", Label: locales.GameEditOpenInExplorer(), Disabled: open == "",
						OnClick: func() { e.openPath(c, open) }},
				)
				if in.Changed() {
					e.set(func(g *models.Game) { g.GameDirectory = v })
				}
			})
			// 原版先挂 dir 再挂 exe：两列在同一行时，可执行文件输入框的修改要以目录的当前值来解析相对路径。
			exe.Children(func() {
				fieldLabel(c, t, locales.GameEditExecutable())
				v := executableDisplayPath(e.g.Path, e.g.GameDirectory)
				path := e.g.Path
				in := actionInput(c, t, "input", &v, actionInputProps{Label: locales.GameEditExecutable()},
					inputAction{Icon: "file-search-outline", Label: locales.GameEditSelectFile(), OnClick: func() { e.selectExecutable(c) }},
					inputAction{Icon: "folder-open-outline", Label: locales.GameEditOpenInExplorer(), Disabled: path == "",
						OnClick: func() { e.openPath(c, path) }},
				)
				if in.Changed() {
					e.set(func(g *models.Game) { g.Path = resolveExecutablePath(v, g.GameDirectory) })
				}
			})
		})
		components.Text(c, locales.GameEditGameDirectoryHint(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
	})
}

// savePathField 是“存档路径”：选择目录、选择文件、在资源管理器中打开。
func (e *editor) savePathField(c *ui.Context) {
	t := e.t
	root, save := e.g.GameDirectory, e.g.SavePath
	ui.Column(c.Key("save-path")).Children(func() {
		fieldLabel(c, t, locales.GameEditSavePath())
		v := save
		in := actionInput(c, t, "input", &v, actionInputProps{Placeholder: locales.GameEditSavePathPlaceholder(), Label: locales.GameEditSavePath()},
			inputAction{Icon: "folder-search-outline", Label: locales.GameEditSelectFolder(), OnClick: func() { e.selectSaveDirectory(c, root) }},
			inputAction{Icon: "file-search-outline", Label: locales.GameEditSelectFile(), OnClick: func() { e.selectSaveFile(c, root) }},
			inputAction{Icon: "folder-open-outline", Label: locales.GameEditOpenInExplorer(), Disabled: save == "",
				OnClick: func() { e.openPath(c, save) }},
		)
		if in.Changed() {
			e.set(func(g *models.Game) { g.SavePath = v })
		}
		hint(c, t, locales.GameEditSavePathHint(), false)
	})
}

// summaryField 是“简介”：<textarea rows={6} className="glass-input w-full px-3 py-2 border rounded-md resize-none">。
func (e *editor) summaryField(c *ui.Context) {
	t := e.t
	ui.Column(c.Key("summary")).Children(func() {
		fieldLabel(c, t, locales.GameEditSummary())
		v := e.g.Summary
		if better.Textarea(c, t, "input", &v, better.TextareaProps{Label: locales.GameEditSummary(), Rows: 6}).Changed() {
			e.set(func(g *models.Game) { g.Summary = v })
		}
	})
}

// metadataSection：section.space-y-2，标题下是默认来源卡片 glass-panel flex items-center justify-between gap-4 rounded-xl border
// border-brand-200 bg-brand-50/60 p-3 dark:border-brand-700 dark:bg-brand-900/25。
func (e *editor) metadataSection(c *ui.Context) {
	t, s := e.t, e.s
	g := &e.g
	defType, defID := defaultSource(g)
	ui.Column(c.Key("metadata")).Gap(4).Children(func() {
		fieldLabel(c, t, locales.GameEditMetadataSources())
		bg, border := t.GlassPanel(t.Pick(theme.Brand50.Alpha(0.6), theme.Brand900.Alpha(0.25)), t.Pick(theme.Brand200, theme.Brand700)) // glass-panel
		ui.Row(c.Key("card")).Gap(16).Padding(12).Radius(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Border(t.HL, border).Background(bg).
			Children(func() {
				ui.Row(c).MinWidth(0).Gap(12).AlignItems(ui.Center).Children(func() {
					defaultSourceIcon(c, t, defType)
					ui.Column(c).MinWidth(0).Children(func() {
						label := "-"
						if defType != "" {
							label = sourceLabel(defType)
						}
						components.Text(c, label, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100)).NoWrap()
						id := defID
						if id == "" {
							id = "-"
						}
						muted := t.Pick(theme.Brand500, theme.Brand400)
						ui.Row(c).MinWidth(0).Margin(2, 0, 0, 0).Gap(6).AlignItems(ui.Center).Children(func() {
							components.Text(c, "ID", theme.TextXS, theme.WeightMedium, muted).NoWrap()
							components.Text(c, id, theme.TextXS, theme.WeightNormal, muted).Font(monoFont).SingleLine().MinWidth(0)
						})
					})
				})
				ui.Row(c).Gap(12).AlignItems(ui.Center).Shrink(0).Children(func() {
					if n := len(g.MetadataSources); n > 1 {
						components.Text(c, locales.GameEditMultipleSources(strconv.Itoa(n)), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
					}
					better.Button(c, t, "edit-sources", better.ButtonProps{
						Variant: better.ButtonSecondary, Size: better.ButtonSM, Icon: "pencil-outline",
						// Label 是 SingleLine，text-xs 的两个字在 150% 下会差一点点宽度而显示省略号，这里用不截断的文字。
						Children: func(color ui.Color) {
							components.Text(c, locales.CommonEdit(), theme.FontStep{Size: 12, Line: 12}, theme.WeightMedium, color).NoWrap()
						},
					}).Label(locales.CommonEdit()).OnClick(func() { s.drawerOpen = true })
				})
			})
	})
}

// monoFont 是 font-mono 在 Windows 上实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// defaultSourceIcon：flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-brand-200 text-brand-700
// dark:bg-brand-700 dark:text-brand-200。Bangumi、Hikarinagi 的方形图标 h-full w-full object-cover，其他 logo
// max-h-6 max-w-8 object-contain 的轮廓。没有图标时原版是 i-mdi-database-star-outline，但 MDI 没有这个图标，什么也不显示。
func defaultSourceIcon(c *ui.Context, t *theme.Theme, source enums.SourceType) {
	ui.Box(c.Key("source-icon")).Size(40, 40).Radius(8).Clip().Center().Shrink(0).
		Background(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		if source == "" {
			return
		}
		l := sourceCompactIcon(source)
		switch {
		case l == nil:
		case source == enums.Bangumi || source == enums.Hikarinagi:
			ui.Image(c, l.color).Size(40, 40).Fit(ui.Cover)
		default:
			logoImage(c, t, l, 24, 32)
		}
	})
}

// switchCard：flex items-center justify-between gap-4 rounded-lg border border-brand-200 bg-brand-50 p-4
// dark:border-brand-700 dark:bg-brand-700/50，左侧 label 与 text-xs 说明（space-y-2），右侧 BetterSwitch。
func (e *editor) switchCard(c *ui.Context, key, label, desc string, checked bool, apply func(g *models.Game, v bool)) {
	t := e.t
	toggle := func(v bool) { e.set(func(g *models.Game) { apply(g, v) }) }
	// data-glass:bg-white/2 data-glass:dark:bg-black/2
	bg := t.IfGlass(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.5)), theme.White.Alpha(0.02), theme.Black.Alpha(0.02))
	ui.Row(c.Key(key)).Gap(16).Padding(16).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(bg).
		Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
				// <label htmlFor>：点击文字也切换开关。
				lb := ui.ButtonBase(c.Key("label")).AlignSelf(ui.Start).Justify(ui.Start).FocusRing(false).Label(label)
				lb.OnClick(func() { toggle(!checked) }).Children(func() {
					components.Text(c, label, theme.TextSM, theme.WeightMedium, t.SectionLabel).NoWrap()
				})
				components.Text(c, desc, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
			})
			better.Switch(c, t, "switch", checked, false, toggle)
		})
}

// footerButtons：flex justify-end gap-4 pt-4，“从远端更新信息”（锁定时禁用）与“删除”。
func (e *editor) footerButtons(c *ui.Context) {
	t, s := e.t, e.s
	locked := e.g.MetadataLocked
	ui.Row(c.Key("footer")).Padding(16, 0, 0, 0).Gap(16).Justify(ui.End).Children(func() {
		better.Button(c, t, "update-remote", better.ButtonProps{
			Variant: better.ButtonPrimary, Icon: "cloud-sync", Label: locales.GameEditUpdateFromRemote(), Disabled: locked,
		}).OnClick(func() {
			// handleOpenUpdateFromRemote：打开字段选择弹窗，勾选上次用过的字段。
			if !locked {
				s.fields = slices.Clone(s.lastFields)
				s.fieldsOpen = true
			}
		})
		better.Button(c, t, "delete", better.ButtonProps{
			Variant: better.ButtonDanger, Icon: "trash-can-outline", Label: locales.CommonDelete(),
		}).OnClick(func() { s.deleteOpen = true })
	})
}
