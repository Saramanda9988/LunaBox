package importer

import (
	"log"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// DragDrop 是 DragDropImportModal 的状态：处理拖进窗口的文件与文件夹，之后与批量导入一样预览、匹配与导入。
type DragDrop struct {
	scanSteps
	open  bool
	paths []string
}

// NewDragDrop 创建弹窗（关闭状态）。
func NewDragDrop(e *env.Env) *DragDrop {
	return &DragDrop{scanSteps: scanSteps{env: e, flow: newFlow(e), step: "processing"}}
}

// Active 对应 isDropImportActiveRef：弹窗打开期间不接受新的拖放，也不显示拖放提示。
func (d *DragDrop) Active() bool { return d.open }

// Flow 是导入流程（测试用）。
func (d *DragDrop) Flow() *Flow { return d.flow }

// Drop 对应 files-dropped 事件：打开弹窗并处理拖进来的路径（ProcessDroppedPathsWithOptions）。
// 没有可导入的游戏或处理失败时提示并关闭。c 是当前帧的 Context（结果在之后的帧里返回）。
func (d *DragDrop) Drop(c *ui.Context, paths []string) {
	if len(paths) == 0 || d.open {
		return
	}
	d.open, d.paths, d.step = true, paths, "processing"
	opts, svc, gen := getScanConfig(d.env.Config()).options, d.env.Svc.Import, d.flow.gen
	d.env.Async(func() func() {
		r, err := svc.ProcessDroppedPathsWithOptions(paths, opts)
		return func() {
			if gen != d.flow.gen || !d.open {
				return
			}
			switch {
			case err != nil:
				log.Printf("import: process dropped paths: %v", err)
				d.env.Toast(c, env.Error(locales.DragDropImportModalToastProcessFailed()))
				d.close()
			case len(r.Candidates) == 0 && len(r.SkippedCandidates) == 0:
				d.env.Toast(c, env.Error(locales.DragDropImportModalToastNoValidGames()))
				d.close()
			default:
				d.flow.SetCandidates(scanToCandidates(r))
				d.step = "preview"
			}
		}
	})
}

// close 对应 resetAndClose 与 handleCloseDragDropModal。
func (d *DragDrop) close() {
	d.flow.Reset()
	d.open, d.paths, d.step = false, nil, "processing"
	d.preview = previewState{}
}

// View 构建弹窗，要在页面最后调用。onComplete 在导入了游戏后调用（handleImportComplete）。
func (d *DragDrop) View(c *ui.Context, t *theme.Theme, onComplete func()) {
	if !d.open {
		return
	}
	components.ImportModalContainer(c, t, "drag-drop-import", components.ImportModalProps{
		Open: true, Title: locales.BatchImportModalTitle(), Icon: "folder-multiple", IconSize: 30, IconColor: theme.Success500,
		TopOffset: env.TopbarHeight, OnClose: d.close,
	}, func() {
		switch d.step {
		case "processing":
			loadingStep(c, t, theme.Success500, locales.DragDropImportModalProcessing()+"...", locales.DragDropImportModalFileCount(strconv.Itoa(len(d.paths))))
		case "preview":
			p := d.previewProps(c, t, onComplete)
			p.leftAction, p.onLeft = locales.CommonCancel(), d.close
			d.flow.preview(c, t, &d.preview, p)
		case "result":
			if r := d.flow.Result; r != nil {
				resultStep(c, t, *r, batchResultLabels(), theme.Success600, theme.Success700, d.close)
			}
		default:
			d.commonSteps(c, t, func() {
				d.flow.Stop()
				d.step = "preview"
			})
		}
	})
	enabled, _ := d.sources()
	d.flow.manualModal(c, t, enabled)
}

// DropOverlay 是根布局拖着文件经过窗口时的提示：absolute inset-0 z-50 bg-primary-500/20 backdrop-blur-sm，
// 中间 rounded-2xl border-2 border-dashed border-primary-500 bg-white/90 p-8 shadow-2xl dark:bg-brand-800/90 的卡片，
// 上下跳动的 i-mdi-folder-upload text-6xl，text-xl font-bold 的标题与 mt-1 text-sm 的说明。不接收指针。
// top 是顶栏的高度（遮罩只盖住顶栏以下）。
func DropOverlay(c *ui.Context, t *theme.Theme, top float32) {
	ui.Column(c.Key("drop-overlay")).Absolute().Top(top).Left(0).Right(0).Bottom(0).Center().PassThrough().
		Material(glass.Blur{Radius: 4}).Children(func() {
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(theme.Primary500.Alpha(0.2))
		ui.Column(c).Gap(16).Padding(32).Radius(16).AlignItems(ui.Center).Border(2, theme.Primary500).BorderStyle(ui.BorderDashed).
			Background(t.Pick(theme.White.Alpha(0.9), theme.Brand800.Alpha(0.9))).Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
			icon := components.Icon(c, "folder-upload", 60, theme.Primary500)
			// animate-bounce：每秒一次，开始与结束时上移 25%，中间落回原位（下落 ease-in，上升 ease-out）
			phase := icon.Loop("bounce", time.Second, ui.Linear)
			var y float32
			if phase < 0.5 {
				x := phase * 2
				y = -15 * (1 - x*x*x)
			} else {
				x := 1 - (phase-0.5)*2
				y = -15 * (1 - x*x*x)
			}
			icon.Translate(0, y)
			ui.Column(c).AlignItems(ui.Center).Children(func() {
				components.Text(c, locales.RootDragDropDropToImport(), theme.TextXL, theme.WeightBold, t.TextStrong)
				components.TextMargin(c, locales.RootDragDropDropHint(), theme.TextSM, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
			})
		})
	})
}
