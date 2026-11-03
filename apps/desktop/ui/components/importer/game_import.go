package importer

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// Source 是 GameImportModal 的 ImportSource。
type Source string

const (
	SourcePlaynite     Source = "playnite"
	SourcePotatoVN     Source = "potatovn"
	SourceYukiHub      Source = "yukihub"
	SourceVnite        Source = "vnite"
	SourceReinaManager Source = "reinamanager"
	SourceSteam        Source = "steam"
)

// importConfig 是 getImportConfigs 的一项，加上各来源的配色（iconColorClass、resultButtonClass）。
type importConfig struct {
	title, desc, hint, button func() string
	icon                      string // 没有 iconSrc 时的图标（steam）
	skipNoPathDefault         bool   // skipNoPathByDefault，默认 true
	selectFile                func(s api.ImportService) (string, error)
	preview                   func(s api.ImportService, path string) ([]service.PreviewGame, error)
	doImport                  func(s api.ImportService, path string, skipNoPath bool, samePath string, sel []vo.ImportSelection) (service.ImportResult, error)
	// primary 是选择文件按钮 bg-*-500 hover:bg-*-600；result 是导入与完成按钮 bg-*-600 hover:bg-*-700。
	primary, primaryHover, result, resultHover ui.Color
	iconColor                                  func(t *theme.Theme) ui.Color
}

func importConfigs(src Source) importConfig {
	fixed := func(c ui.Color) func(*theme.Theme) ui.Color { return func(*theme.Theme) ui.Color { return c } }
	switch src {
	case SourcePlaynite:
		return importConfig{
			title: locales.GameImportModalPlayniteTitle, desc: locales.GameImportModalPlayniteDesc, hint: locales.GameImportModalPlayniteHint, button: locales.GameImportModalPlayniteBtn,
			skipNoPathDefault: true,
			selectFile:        func(s api.ImportService) (string, error) { return s.SelectJSONFile() },
			preview:           func(s api.ImportService, p string) ([]service.PreviewGame, error) { return s.PreviewPlayniteImport(p) },
			doImport: func(s api.ImportService, p string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
				return s.ImportFromPlayniteWithSelection(p, skip, same, sel)
			},
			primary: theme.Purple500, primaryHover: theme.Purple600, result: theme.Purple600, resultHover: theme.Purple700, iconColor: fixed(theme.Purple500),
		}
	case SourceYukiHub:
		return importConfig{
			title: locales.GameImportModalYukihubTitle, desc: locales.GameImportModalYukihubDesc, hint: locales.GameImportModalYukihubHint, button: locales.GameImportModalYukihubBtn,
			selectFile: func(s api.ImportService) (string, error) { return s.SelectYukiHubBackup() },
			preview:    func(s api.ImportService, p string) ([]service.PreviewGame, error) { return s.PreviewYukiHubImport(p) },
			doImport: func(s api.ImportService, p string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
				return s.ImportFromYukiHubWithSelection(p, skip, same, sel)
			},
			primary: theme.Brand600, primaryHover: theme.Brand700, result: theme.Brand600, resultHover: theme.Brand700,
			iconColor: func(t *theme.Theme) ui.Color { return t.Pick(theme.Brand600, theme.Brand300) },
		}
	case SourceVnite:
		return importConfig{
			title: locales.GameImportModalVniteTitle, desc: locales.GameImportModalVniteDesc, hint: locales.GameImportModalVniteHint, button: locales.GameImportModalVniteBtn,
			skipNoPathDefault: true,
			selectFile:        func(s api.ImportService) (string, error) { return s.SelectVniteDirectory() },
			preview:           func(s api.ImportService, p string) ([]service.PreviewGame, error) { return s.PreviewVniteImport(p) },
			doImport: func(s api.ImportService, p string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
				return s.ImportFromVniteWithSelection(p, skip, same, sel)
			},
			primary: theme.Sky500, primaryHover: theme.Sky600, result: theme.Sky600, resultHover: theme.Sky700, iconColor: fixed(theme.Sky500),
		}
	case SourceReinaManager:
		return importConfig{
			title: locales.GameImportModalReinamanagerTitle, desc: locales.GameImportModalReinamanagerDesc, hint: locales.GameImportModalReinamanagerHint, button: locales.GameImportModalReinamanagerBtn,
			skipNoPathDefault: true,
			selectFile:        func(s api.ImportService) (string, error) { return s.SelectReinaManagerDatabase() },
			preview: func(s api.ImportService, p string) ([]service.PreviewGame, error) {
				return s.PreviewReinaManagerImport(p)
			},
			doImport: func(s api.ImportService, p string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
				return s.ImportFromReinaManagerWithSelection(p, skip, same, sel)
			},
			primary: theme.Rose500, primaryHover: theme.Rose600, result: theme.Rose600, resultHover: theme.Rose700, iconColor: fixed(theme.Rose500),
		}
	case SourceSteam:
		return importConfig{
			title: locales.GameImportModalSteamTitle, desc: locales.GameImportModalSteamDesc, hint: locales.GameImportModalSteamHint, button: locales.GameImportModalSteamBtn,
			icon: "steam", skipNoPathDefault: true,
			selectFile: func(api.ImportService) (string, error) { return "steam-local", nil },
			preview:    func(s api.ImportService, _ string) ([]service.PreviewGame, error) { return s.PreviewSteamLocalImport() },
			doImport: func(s api.ImportService, _ string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
				return s.ImportFromSteamLocalWithSelection(skip, same, sel)
			},
			primary: theme.Slate700, primaryHover: theme.Slate800, result: theme.Slate700, resultHover: theme.Slate800,
			iconColor: func(t *theme.Theme) ui.Color { return t.Pick(theme.Slate600, theme.Slate300) },
		}
	}
	return importConfig{
		title: locales.GameImportModalPotatovnTitle, desc: locales.GameImportModalPotatovnDesc, hint: locales.GameImportModalPotatovnHint, button: locales.GameImportModalPotatovnBtn,
		skipNoPathDefault: true,
		selectFile:        func(s api.ImportService) (string, error) { return s.SelectZipFile() },
		preview:           func(s api.ImportService, p string) ([]service.PreviewGame, error) { return s.PreviewImport(p) },
		doImport: func(s api.ImportService, p string, skip bool, same string, sel []vo.ImportSelection) (service.ImportResult, error) {
			return s.ImportFromPotatoVNWithSelection(p, skip, same, sel)
		},
		primary: theme.Neutral500, primaryHover: theme.Neutral600, result: theme.Neutral600, resultHover: theme.Neutral700, iconColor: fixed(theme.Neutral500),
	}
}

// GameImport 是 GameImportModal 的状态：选择导出文件，预览，选择要导入的条目后导入。
type GameImport struct {
	env      *env.Env
	open     bool
	source   Source
	step     string // select、preview、importing、result
	path     string
	games    []service.PreviewGame
	selected map[string]bool // selectedPreviewKeys
	result   *service.ImportResult
	loading  bool
	skip     bool   // skipNoPath
	samePath string // samePathAction：skip、merge_sessions、merge
	gen      int
}

// NewGameImport 创建弹窗（关闭状态）。
func NewGameImport(e *env.Env) *GameImport {
	return &GameImport{env: e, step: "select", skip: true, samePath: "skip"}
}

// Open 打开某个来源的导入（setImportSource）。
func (g *GameImport) Open(src Source) { g.source, g.open = src, true }

// IsOpen 报告弹窗是否打开。
func (g *GameImport) IsOpen() bool { return g.open }

// close 对应 resetAndClose。
func (g *GameImport) close() {
	g.gen++
	g.open, g.step, g.path, g.games, g.selected, g.result, g.loading = false, "select", "", nil, nil, nil, false
	g.skip, g.samePath = true, "skip"
}

// previewKey 对应 previewGameKey。
func previewKey(game service.PreviewGame, i int) string {
	return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", game.SourceType, game.SourceID, game.Path, game.Name, i)
}

// actionable 对应 isPreviewGameActionable。
func actionable(game service.PreviewGame, skipNoPath bool, samePath string) bool {
	if game.ConflictType == "same_path" {
		return samePath != "skip"
	}
	if game.Exists {
		return false
	}
	return !skipNoPath || game.HasPath
}

func (g *GameImport) isActionable(game service.PreviewGame) bool {
	return actionable(game, g.skip, g.samePath)
}

// selectFile 对应 handleSelectFile：选择文件后预览，默认选中可以导入的条目。
func (g *GameImport) selectFile(c *ui.Context) {
	cfg, svc, gen := importConfigs(g.source), g.env.Svc.Import, g.gen
	g.env.Async(func() func() {
		path, err := cfg.selectFile(svc)
		return func() {
			if gen != g.gen {
				return
			}
			if err != nil {
				log.Printf("import: select file: %v", err)
				g.env.Toast(c, env.Error(locales.GameImportModalToastSelectFileFailed()))
				return
			}
			if path == "" {
				return
			}
			g.path, g.skip, g.loading = path, cfg.skipNoPathDefault, true
			g.env.Async(func() func() {
				games, err := cfg.preview(svc, path)
				return func() {
					if gen != g.gen {
						return
					}
					g.loading = false
					if err != nil {
						log.Printf("import: preview %s: %v", path, err)
						g.env.Toast(c, env.Error(locales.GameImportModalToastPreviewFailed()))
						return
					}
					g.games = games
					g.selected = map[string]bool{}
					for i, game := range games {
						if actionable(game, g.skip, g.samePath) {
							g.selected[previewKey(game, i)] = true
						}
					}
					g.step = "preview"
				}
			})
		}
	})
}

// doImport 对应 handleImport：导入选中且可以导入的条目。
func (g *GameImport) doImport(c *ui.Context, onComplete func()) {
	if g.path == "" {
		return
	}
	var sel []vo.ImportSelection
	for i, game := range g.games {
		if g.selected[previewKey(game, i)] && g.isActionable(game) {
			sel = append(sel, vo.ImportSelection{Name: game.Name, Path: game.Path, SourceType: enums.SourceType(game.SourceType), SourceID: game.SourceID})
		}
	}
	g.step, g.loading = "importing", true
	cfg, svc, gen := importConfigs(g.source), g.env.Svc.Import, g.gen
	path, skip, same := g.path, g.skip, g.samePath
	g.env.Async(func() func() {
		r, err := cfg.doImport(svc, path, skip, same, sel)
		return func() {
			if gen != g.gen {
				return
			}
			g.loading = false
			if err != nil {
				log.Printf("import: %s: %v", g.source, err)
				g.env.Toast(c, env.Error(locales.GameImportModalToastImportFailed()))
				g.step = "preview"
				return
			}
			g.result, g.step = &r, "result"
			if r.Success > 0 {
				g.env.Toast(c, env.Success(locales.GameImportModalToastImportSuccess(strconv.Itoa(r.Success))))
				if onComplete != nil {
					onComplete()
				}
			}
		}
	})
}

// View 构建弹窗（w-full max-w-3xl 的 ImportModalContainer 样式），要在页面最后调用。
func (g *GameImport) View(c *ui.Context, t *theme.Theme, onComplete func()) {
	if !g.open {
		return
	}
	cfg := importConfigs(g.source)
	props := components.ImportModalProps{
		Open: true, Title: cfg.title(), TopOffset: env.TopbarHeight, MaxWidth: 768, OnClose: g.close,
		Icon: cfg.icon, IconSize: 30, IconColor: cfg.iconColor(t),
	}
	if img := assets.Importer(string(g.source)); img != nil {
		props.Image = img
	}
	components.ImportModalContainer(c, t, "game-import", props, func() {
		switch g.step {
		case "select":
			g.selectStep(c, t, cfg)
		case "preview":
			g.previewStep(c, t, cfg, onComplete)
		case "importing":
			loadingStep(c, t, cfg.iconColor(t), locales.GameImportModalImporting(), locales.GameImportModalImportHint())
		case "result":
			if g.result != nil {
				resultStep(c, t, *g.result, resultLabels{
					success: locales.GameImportModalResultSuccess(), skipped: locales.GameImportModalResultSkipped(),
					failed: locales.GameImportModalResultFailed(), skippedGames: locales.GameImportModalSkippedGames() + ":",
					failedGames: locales.GameImportModalFailedGames() + ":", complete: locales.CommonComplete(),
				}, cfg.result, cfg.resultHover, g.close)
			}
		}
	})
}

// selectStep：text-center py-8 的说明（i-mdi-application-import text-6xl），下面是 w-full py-4 的选择文件按钮。
func (g *GameImport) selectStep(c *ui.Context, t *theme.Theme, cfg importConfig) {
	ui.Column(c.Key("select-step")).Gap(24).Children(func() {
		ui.Column(c).Padding(32, 0).AlignItems(ui.Center).Children(func() {
			components.Icon(c, "application-import", 60, theme.Brand400).Margin(0, 0, 16, 0)
			components.TextMargin(c, cfg.desc(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300), 0, 0, 8, 0)
			components.Text(c, cfg.hint(), theme.TextSM, theme.WeightNormal, t.TextFaint)
		})
		label := cfg.button()
		if g.loading {
			label = locales.CommonLoading()
		}
		btn := ui.ButtonBase(c.Key("select-file")).FillWidth().Padding(16, 0).Radius(8).Center().Gap(8).FocusRing(false).Label(label)
		bg := cfg.primary
		if btn.Hovered() && !g.loading {
			bg = cfg.primaryHover
		}
		btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
		if g.loading {
			components.Disable(btn, 0.5)
		} else {
			btn.OnClick(func() { g.selectFile(c) })
		}
		btn.Children(func() {
			if g.loading {
				components.Spinner(c, 20, theme.White)
			} else {
				components.Icon(c, "file-find", 20, theme.White)
			}
			components.Text(c, label, theme.TextBase, theme.WeightNormal, theme.White)
		})
	})
}

// previewStep（space-y-4）：统计卡片、同路径游戏的处理方式、跳过没有路径的游戏、预览表格，底部的重新选择与导入。
func (g *GameImport) previewStep(c *ui.Context, t *theme.Theme, cfg importConfig, onComplete func()) {
	merge := g.samePath != "skip"
	var newCount, updateCount, existing, noPath, samePath int
	for i, game := range g.games {
		sel := g.selected[previewKey(game, i)]
		if !game.Exists && g.isActionable(game) && sel {
			newCount++
		}
		if merge && game.ConflictType == "same_path" && sel {
			updateCount++
		}
		if game.Exists && game.ConflictType != "same_path" {
			existing++
		}
		if !game.HasPath && !game.Exists {
			noPath++
		}
		if game.ConflictType == "same_path" {
			samePath++
		}
	}
	importable := newCount + updateCount
	ui.Column(c.Key("preview-step")).Gap(16).Children(func() {
		cards := []statCard{
			{value: newCount, labelStr: locales.GameImportModalWillImport(), bg: t.Pick(theme.Success50, theme.Success900.Alpha(0.2)),
				valueFg: t.Pick(theme.Success600, theme.Success400), labelFg: t.Pick(theme.Success700, theme.Success300)},
			{value: existing, labelStr: locales.GameImportModalExists(), bg: t.Pick(theme.Yellow50, theme.Yellow900.Alpha(0.2)),
				valueFg: t.Pick(theme.Yellow600, theme.Yellow400), labelFg: t.Pick(theme.Yellow700, theme.Yellow300)},
		}
		if noPath > 0 {
			cards = append(cards, statCard{value: noPath, labelStr: locales.GameImportModalNoPath(), bg: t.Pick(theme.Orange50, theme.Orange900.Alpha(0.2)),
				valueFg: t.Pick(theme.Orange600, theme.Orange400), labelFg: t.Pick(theme.Orange700, theme.Orange300)})
		}
		if samePath > 0 {
			cards = append(cards, statCard{value: samePath, labelStr: locales.GameImportModalSamePath(), bg: t.Pick(theme.Sky50, theme.Sky900.Alpha(0.2)),
				valueFg: t.Pick(theme.Sky600, theme.Sky400), labelFg: t.Pick(theme.Sky700, theme.Sky300)})
		}
		statCards(c, cards)
		if samePath > 0 {
			g.samePathChoice(c, t, samePath)
		}
		if noPath > 0 {
			g.skipNoPathOption(c, t, noPath)
		}
		g.previewTable(c, t, merge)
		ui.Row(c.Key("actions")).Justify(ui.SpaceBetween).Children(func() {
			outlineButton(c, t, "reselect", "← "+locales.GameImportModalReselect(), false, 20, 10).OnClick(func() { g.step = "select" })
			btn := solidButton(c, "import", locales.GameImportModalImportCount(strconv.Itoa(importable)), cfg.result, cfg.resultHover, importable == 0, false, 20, 10)
			if importable > 0 {
				btn.OnClick(func() { g.doImport(c, onComplete) })
			}
		})
	})
}

// samePathChoice：rounded-lg border border-sky-200 bg-sky-50 p-4，mb-3 text-sm font-medium 的标题，
// grid gap-2 sm:grid-cols-3 的三个选项（rounded-lg border px-3 py-2 text-left text-sm）。
func (g *GameImport) samePathChoice(c *ui.Context, t *theme.Theme, count int) {
	ui.Column(c.Key("same-path")).Padding(16).Radius(8).Border(t.HL, t.Pick(theme.Sky200, theme.Sky800)).
		Background(t.Pick(theme.Sky50, theme.Sky900.Alpha(0.2))).Children(func() {
		components.TextMargin(c, locales.GameImportModalSamePathActionTitle(strconv.Itoa(count)), theme.TextSM, theme.WeightMedium,
			t.Pick(theme.Sky800, theme.Sky200), 0, 0, 12, 0)
		ui.Row(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
			for _, o := range []struct {
				value, icon string
				label, hint func() string
			}{
				{"skip", "skip-next-circle", locales.GameImportModalSamePathSkip, locales.GameImportModalSamePathSkipHint},
				{"merge_sessions", "history", locales.GameImportModalSamePathMergeSessions, locales.GameImportModalSamePathMergeSessionsHint},
				{"merge", "database-sync-outline", locales.GameImportModalSamePathMerge, locales.GameImportModalSamePathMergeHint},
			} {
				on := g.samePath == o.value
				btn := ui.ButtonBase(c.Key(o.value)).Column().Grow(1).Basis(0).MinWidth(0).Padding(8, 12).Radius(8).
					AlignItems(ui.Start).Justify(ui.Start).FocusRing(false).Label(o.label())
				bg, border, fg := t.Pick(theme.White.Alpha(0.6), theme.Sky950.Alpha(0.2)), t.Pick(theme.Sky200, theme.Sky800), t.Pick(theme.Sky700, theme.Sky300)
				switch {
				case on:
					bg, border, fg = t.Pick(theme.White, theme.Sky950.Alpha(0.4)), theme.Sky500, t.Pick(theme.Sky800, theme.Sky100)
					btn.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
				case btn.Hovered():
					bg = t.Pick(theme.White, theme.Sky950.Alpha(0.2))
				}
				btn.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond)).
					OnClick(func() { g.samePath = o.value }).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						components.Icon(c, o.icon, 16, fg)
						components.Text(c, o.label(), theme.TextSM, theme.WeightMedium, fg)
					})
					components.TextMargin(c, o.hint(), theme.TextXS, theme.WeightNormal, fg.Alpha(0.8), 4, 0, 0, 0)
				})
			}
		})
	})
}

// skipNoPathOption：rounded-lg bg-orange-50 p-4 的复选框（mt-1 mr-3），标题 text-sm font-medium，说明 mt-1 text-xs 两行。
func (g *GameImport) skipNoPathOption(c *ui.Context, t *theme.Theme, count int) {
	row := ui.ButtonBase(c.Key("skip-no-path")).Padding(16).Radius(8).AlignItems(ui.Start).Justify(ui.Start).FocusRing(false).
		Background(t.Pick(theme.Orange50, theme.Orange900.Alpha(0.2))).Label(locales.GameImportModalSkipNoPath())
	// <label> 包着复选框：点哪里都切换，复选框本身不接收指针。
	row.OnClick(func() { g.skip = !g.skip }).Children(func() {
		components.Checkbox(c, "check", components.CheckboxProps{Checked: g.skip, Label: locales.GameImportModalSkipNoPath()}).
			Margin(4+3, 12+3, 3, 4).PassThrough()
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, locales.GameImportModalSkipNoPath(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Orange700, theme.Orange300))
			hint := locales.GameImportModalSkipNoPathHintCheck()
			if g.skip {
				hint = locales.GameImportModalSkipNoPathHintUncheck()
			}
			fg := t.Pick(theme.Orange600, theme.Orange400)
			components.TextMargin(c, locales.GameImportModalSkipNoPathHint1(strconv.Itoa(count)), theme.TextXS, theme.WeightNormal, fg, 4, 0, 0, 0)
			components.Text(c, hint, theme.TextXS, theme.WeightNormal, fg)
		})
	})
}

// previewTable 是 max-h-[300px] 的 BetterDataTable：可选择的行，名称（没有路径时 mt-1 text-xs text-orange-500 的提示）、
// 开发商与状态胶囊（新增、已存在、将更新、跳过）。不能导入的行 opacity-45，没有选中的 opacity-65。
func (g *GameImport) previewTable(c *ui.Context, t *theme.Theme, merge bool) {
	type row struct {
		game  service.PreviewGame
		index int
	}
	rows := make([]row, len(g.games))
	for i, game := range g.games {
		rows[i] = row{game, i}
	}
	pill := func(icon, label string, bg, fg ui.Color) {
		// inline-flex min-w-[4.5rem] gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium leading-none
		ui.Row(c.Key("pill")).MinWidth(72).Padding(6, 12).Gap(6).Radius(999).Center().Background(bg).Children(func() {
			components.Icon(c, icon, 14, fg)
			components.Text(c, label, theme.FontStep{Size: 12, Line: 12}, theme.WeightMedium, fg).NoWrap()
		})
	}
	better.DataTable(c, t, "preview", better.DataTableProps[row]{
		Rows:      rows,
		MaxHeight: 300,
		RowKey:    func(r row, _ int) string { return previewKey(r.game, r.index) },
		Empty: func() {
			components.Text(c, locales.GameImportModalNoGameData(), theme.TextSM, theme.WeightNormal, t.TextFaint)
		},
		RowOpacity: func(r row, _ int) float32 {
			switch {
			case !g.isActionable(r.game):
				return 0.45
			case !g.selected[previewKey(r.game, r.index)]:
				return 0.65
			}
			return 1
		},
		Selection: &better.TableSelection[row]{
			Selected:   func(r row, _ int) bool { return g.selected[previewKey(r.game, r.index)] },
			Selectable: func(r row, _ int) bool { return g.isActionable(r.game) },
			OnToggleRow: func(r row, _ int, on bool) {
				if !g.isActionable(r.game) {
					return
				}
				if g.selected == nil {
					g.selected = map[string]bool{}
				}
				if on {
					g.selected[previewKey(r.game, r.index)] = true
				} else {
					delete(g.selected, previewKey(r.game, r.index))
				}
			},
			OnToggleAll: func(on bool) {
				g.selected = map[string]bool{}
				if !on {
					return
				}
				for i, game := range g.games {
					if g.isActionable(game) {
						g.selected[previewKey(game, i)] = true
					}
				}
			},
			AllLabel: locales.CommonSelectAll(),
			RowLabel: func(r row, _ int) string { return r.game.Name },
		},
		Columns: []better.DataTableColumn[row]{
			{Key: "name", Header: locales.GameImportModalGameName(), Width: 0.46, Cell: func(r row, _ int) {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					components.Text(c, r.game.Name, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine()
					if !r.game.HasPath {
						components.TextMargin(c, locales.GameImportModalNoPathLabel(), theme.TextXS, theme.WeightNormal, theme.Orange500, 4, 0, 0, 0)
					}
				})
			}},
			{Key: "developer", Header: locales.GameImportModalDeveloper(), Width: 0.30, Cell: func(r row, _ int) {
				dev := r.game.Developer
				if dev == "" {
					dev = "-"
				}
				components.Text(c, dev, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).SingleLine().Grow(1).MinWidth(0)
			}},
			{Key: "status", Header: locales.CommonStatus(), FixedWidth: 144, Center: true, Cell: func(r row, _ int) {
				game := r.game
				switch {
				case game.Exists && game.ConflictType == "same_path" && merge:
					pill("database-sync-outline", locales.GameImportModalStatusUpdate(), t.Pick(theme.Sky100, theme.Sky900.Alpha(0.3)), t.Pick(theme.Sky700, theme.Sky400))
				case game.Exists:
					pill("check-circle", locales.GameImportModalExists(), t.Pick(theme.Yellow100, theme.Yellow900.Alpha(0.3)), t.Pick(theme.Yellow700, theme.Yellow400))
				case !game.HasPath && g.skip:
					pill("close-circle", locales.GameImportModalStatusSkipped(), t.Pick(theme.Orange100, theme.Orange900.Alpha(0.3)), t.Pick(theme.Orange700, theme.Orange400))
				default:
					pill("plus-circle", locales.GameImportModalNewAdd(), t.Pick(theme.Success100, theme.Success900.Alpha(0.3)), t.Pick(theme.Success700, theme.Success400))
				}
			}},
		},
	})
}
