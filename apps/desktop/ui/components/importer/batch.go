package importer

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// BatchImport 是 BatchImportModal 的状态：选择游戏库目录、扫描、匹配元数据并批量导入。
type BatchImport struct {
	scanSteps
	open        bool
	libraryPath string
	loading     bool
}

// NewBatchImport 创建弹窗（关闭状态）。
func NewBatchImport(e *env.Env) *BatchImport {
	return &BatchImport{scanSteps: scanSteps{env: e, flow: newFlow(e), step: "select"}}
}

// Open 打开弹窗。
func (b *BatchImport) Open() { b.open = true }

// IsOpen 报告弹窗是否打开。
func (b *BatchImport) IsOpen() bool { return b.open }

// Flow 是导入流程（测试用）。
func (b *BatchImport) Flow() *Flow { return b.flow }

// close 对应 resetAndClose。
func (b *BatchImport) close() {
	b.flow.Reset()
	b.step, b.libraryPath, b.loading, b.open = "select", "", false, false
	b.preview = previewState{}
}

// scan 对应 scanDirectory：扫描目录，结果进入预览；失败时提示并回到选择目录。
func (b *BatchImport) scan(c *ui.Context, path string) {
	b.libraryPath, b.step, b.loading = path, "scan", true
	opts, svc, gen := getScanConfig(b.env.Config()).options, b.env.Svc.Import, b.flow.gen
	b.env.Async(func() func() {
		r, err := svc.ScanLibraryDirectoryWithOptions(path, opts)
		return func() {
			if gen != b.flow.gen || !b.open {
				return
			}
			b.loading = false
			if err != nil {
				log.Printf("import: scan %s: %v", path, err)
				b.env.Toast(c, env.Error(locales.BatchImportModalToastScanFailed()))
				b.step = "select"
				return
			}
			b.flow.SetCandidates(scanToCandidates(r))
			b.step = "preview"
		}
	})
}

// selectDirectory 对应 handleSelectDirectory：从上次的目录打开选择框，记下选择的目录后扫描。
func (b *BatchImport) selectDirectory(c *ui.Context) {
	last := ""
	if cfg := b.env.Config(); cfg != nil {
		last = strings.TrimSpace(cfg.BatchImportLastDirectory)
	}
	svc := b.env.Svc.Import
	b.env.Async(func() func() {
		sel, err := svc.SelectLibraryDirectory(last)
		return func() {
			if err != nil {
				log.Printf("import: select directory: %v", err)
				b.env.Toast(c, env.Error(locales.BatchImportModalToastSelectDirFailed()))
				return
			}
			if sel.InitialDirectoryInvalid {
				b.patch(func(cfg *appconf.AppConfig) { cfg.BatchImportLastDirectory = "" })
			}
			if sel.Directory == "" || !b.open {
				return
			}
			b.patch(func(cfg *appconf.AppConfig) { cfg.BatchImportLastDirectory = sel.Directory })
			b.scan(c, sel.Directory)
		}
	})
}

func (b *BatchImport) patch(fn func(cfg *appconf.AppConfig)) {
	if err := b.env.PatchConfig(fn); err != nil {
		log.Printf("import: save batch import preferences: %v", err)
	}
}

// View 构建弹窗，要在页面最后调用。onComplete 在导入了游戏后调用（invalidateAndRefreshLibrary）。
func (b *BatchImport) View(c *ui.Context, t *theme.Theme, onComplete func()) {
	if !b.open {
		return
	}
	maxW := float32(0)
	if b.step == "select" {
		maxW = 896 // max-w-4xl
	}
	components.ImportModalContainer(c, t, "batch-import", components.ImportModalProps{
		Open: true, Title: locales.BatchImportModalTitle(), Icon: "folder-multiple", IconSize: 30, IconColor: theme.Success500,
		TopOffset: env.TopbarHeight, MaxWidth: maxW, OnClose: b.close,
	}, func() {
		switch b.step {
		case "select":
			b.selectStep(c, t)
		case "scan":
			loadingStep(c, t, theme.Success500, locales.BatchImportModalScanning(), b.libraryPath)
		case "preview":
			p := b.previewProps(c, t, onComplete)
			p.leftAction, p.onLeft = "← "+locales.BatchImportModalReselect(), func() { b.step = "select" }
			b.flow.preview(c, t, &b.preview, p)
		case "result":
			if r := b.flow.Result; r != nil {
				resultStep(c, t, *r, batchResultLabels(), theme.Success600, theme.Success700, b.close)
			}
		default:
			b.commonSteps(c, t, nil)
		}
	})
	enabled, _ := b.sources()
	b.flow.manualModal(c, t, enabled)
}

// selectStep：py-8 text-center 的说明（i-mdi-folder-open text-6xl），下面是扫描游戏库、选择目录与扫描方式三个按钮。
func (b *BatchImport) selectStep(c *ui.Context, t *theme.Theme) {
	libraryPath := ""
	if cfg := b.env.Config(); cfg != nil {
		libraryPath = strings.TrimSpace(cfg.GameLibraryPath)
	}
	ui.Column(c.Key("select-step")).Gap(24).Children(func() {
		ui.Column(c).Padding(32, 0).AlignItems(ui.Center).Children(func() {
			components.Icon(c, "folder-open", 60, theme.Brand400).Margin(0, 0, 16, 0)
			components.TextMargin(c, locales.BatchImportModalSelectDir(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand700, theme.Brand300), 0, 0, 8, 0)
			components.Text(c, locales.BatchImportModalScanHint(), theme.TextSM, theme.WeightNormal, t.TextFaint)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			scan := better.Button(c, t, "scan-library", better.ButtonProps{
				Variant: better.ButtonPrimary, Size: better.ButtonLG, Icon: "folder-search",
				Label: locales.BatchImportModalBtnScanGameLibrary(), Disabled: b.loading || libraryPath == "",
			}).Grow(1).Basis(0)
			scan.OnClick(func() {
				if libraryPath != "" {
					b.scan(c, libraryPath)
				}
			})
			better.Button(c, t, "select-dir", better.ButtonProps{
				Size: better.ButtonLG, Icon: "folder-open-outline", Label: locales.BatchImportModalBtnSelectDir(), Disabled: b.loading,
			}).Grow(1).Basis(0).OnClick(func() { b.selectDirectory(c) })
			b.scanModeMenu(c, t)
		})
	})
}

// scanModeMenu 是扫描方式的 BetterDropdownMenu（w-[22rem]）：三种方式，footer 里调整层级模式的目录层级。
func (b *BatchImport) scanModeMenu(c *ui.Context, t *theme.Theme) {
	sc := getScanConfig(b.env.Config())
	level := strconv.Itoa(sc.hierarchyLevel())
	item := func(key, label, desc, icon string) better.MenuItem {
		it := better.MenuItem{Key: key, Label: label, Description: desc, Icon: icon, IconColor: theme.Brand400,
			OnClick: func() { b.patch(func(cfg *appconf.AppConfig) { cfg.BatchImportScanPreset = key }) }}
		if sc.preset == key {
			it.Icon, it.IconColor = "check", theme.Success500
		}
		return it
	}
	items := []better.MenuItem{
		item("scan_parent", locales.BatchImportModalScanModeScanParent(), locales.BatchImportModalScanModeScanParentHint(), "file-search-outline"),
		item("scan_library_child", locales.BatchImportModalScanModeScanLibraryChild(), locales.BatchImportModalScanModeScanLibraryChildHint(), "file-tree-outline"),
		item("hierarchy_child", locales.BatchImportModalScanModeHierarchyChild(level), locales.BatchImportModalScanModeHierarchyChildHint(level), "folder-table-outline"),
	}
	trigger := better.Button(c, t, "scan-mode", better.ButtonProps{Size: better.ButtonLG, Icon: "tune-variant", Label: locales.BatchImportModalScanModeButton()})
	better.DropdownMenu(c, t, trigger, better.MenuProps{
		Title: locales.BatchImportModalScanModeLabel(), Width: 352, Items: items,
		Footer: func() { b.depthStepper(c, t, sc, level) },
	})
}

// depthStepper 是菜单的 footer：mt-1 border-t px-3 py-2.5，左边标题与说明，右边 rounded-lg border bg-brand-50 p-0.5 的
// 减、层级、加（h-8 w-8 rounded-md，到头时 disabled:opacity-40）。
func (b *BatchImport) depthStepper(c *ui.Context, t *theme.Theme, sc scanConfig, level string) {
	setDepth := func(d int) {
		b.patch(func(cfg *appconf.AppConfig) { cfg.BatchImportHierarchyDepth = clampDepth(d) })
	}
	ui.Row(c.Key("depth")).Margin(4, 0, 0, 0).Padding(10, 12).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, locales.BatchImportModalScanModeHierarchyDepthLabel(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand200))
			components.TextMargin(c, locales.BatchImportModalScanModeHierarchyDepthHint(level), theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal, t.TextFaint, 2, 0, 0, 0)
		})
		ui.Row(c).Shrink(0).Padding(2).Radius(8).AlignItems(ui.Center).Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).
			Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.3))).Children(func() {
			stepButton(c, t, "dec", "minus", locales.BatchImportModalScanModeDecreaseDepth(), sc.hierarchyDepth == 0, func() { setDepth(sc.hierarchyDepth - 1) })
			components.Text(c, locales.BatchImportModalScanModeHierarchyDepthValue(level), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100)).
				MinWidth(48).Padding(0, 8).NoWrap()
			stepButton(c, t, "inc", "plus", locales.BatchImportModalScanModeIncreaseDepth(), sc.hierarchyDepth == maxHierarchyDepth, func() { setDepth(sc.hierarchyDepth + 1) })
		})
	})
}

func stepButton(c *ui.Context, t *theme.Theme, key, icon, label string, disabled bool, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Size(32, 32).Radius(6).Center().FocusRing(false).Label(label)
	fg := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() && !disabled {
		btn.Background(t.Pick(theme.White, theme.Brand700))
		fg = t.Pick(theme.Brand800, theme.Brand100)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.4)
	} else {
		btn.OnClick(onClick)
	}
	btn.Children(func() { components.Icon(c, icon, 18, fg) })
}
