package game

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// guideModal 是 GameGuideDocumentModal 的状态。
type guideModal struct {
	loading bool // isLoadingGameGuideDocuments：按钮显示旋转图标
	open    bool
	docs    []vo.GameGuideDocument
	opening string // 正在打开的文档（relative_path）
	scroll  components.ScrollBar
}

// openGuides 对应 handleOpenGameGuideDocuments：没有文档时提示，只有一个时直接打开，多个时让用户选。
func (p *Page) openGuides(c *ui.Context) {
	m := &p.guide
	if p.game == nil || m.loading {
		return
	}
	m.loading = true
	id, gen := p.game.ID, p.gen
	p.env.Async(func() func() {
		docs, err := p.env.Svc.Game.FindGameGuideDocuments(id)
		if err == nil && len(docs) == 1 {
			err = p.env.Svc.Game.OpenGameGuideDocument(id, docs[0].RelativePath)
		}
		return func() {
			if gen != p.gen {
				return
			}
			m.loading = false
			switch {
			case err != nil:
				p.env.Toast(c, env.Error(locales.GameEditGuideDocumentsFailed()))
			case len(docs) == 0:
				p.env.Toast(c, env.Error(locales.GameEditGuideDocumentsEmpty()))
			case len(docs) > 1:
				m.docs, m.open, m.scroll = docs, true, components.ScrollBar{}
			}
		}
	})
}

// openGuide 对应 handleOpenGameGuideDocument：打开成功后关闭弹窗。
func (p *Page) openGuide(c *ui.Context, doc vo.GameGuideDocument) {
	m := &p.guide
	m.opening = doc.RelativePath
	id := p.game.ID
	p.env.Async(func() func() {
		err := p.env.Svc.Game.OpenGameGuideDocument(id, doc.RelativePath)
		return func() {
			m.opening = ""
			if err != nil {
				p.env.Toast(c, env.Error(locales.GameEditOpenGuideFailed()))
				return
			}
			m.open = false
		}
	})
}

// guideView 对应 GameGuideDocumentModal：w-full max-w-lg rounded-xl p-6 的面板，标题与“找到 N 个可打开的文档”，
// 下面 max-h-80 rounded-lg border bg-brand-50 的文档列表（divide-y），右下角取消。
func (p *Page) guideView(c *ui.Context, t *theme.Theme) {
	m := &p.guide
	busy := m.opening != ""
	components.Modal(c, t, "guide-modal", components.ModalProps{Open: m.open, TopOffset: env.TopbarHeight, MaxWidth: 512},
		func(panel ui.Element) {
			components.ModalPanel(t, panel).Children(func() {
				ui.Column(c).Margin(0, 0, 20, 0).Children(func() {
					components.TextMargin(c, locales.GameEditGuideDocumentsTitle(), theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 4, 0)
					components.Text(c, locales.GameEditGuideDocumentsDescription(strconv.Itoa(len(m.docs))), theme.FontStep{Size: 14, Line: 22.75},
						theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
				})
				components.ScrollArea(c, t, "guide-list", &m.scroll, func(w ui.Element) {
					w.MaxHeight(320).Radius(8).Border(t.HL, t.Pick(theme.Brand200, theme.Brand600)).
						Background(t.Pick(theme.Brand50, theme.Brand900.Alpha(0.4)))
				}, func(sc ui.Element) {
					sc.Children(func() {
						for i, doc := range m.docs {
							if i > 0 {
								ui.Box(c).Height(t.HL).Background(t.Pick(theme.Brand200, theme.Brand700))
							}
							guideRow(c.Key(doc.RelativePath), t, doc, m.opening == doc.RelativePath, busy, func() { p.openGuide(c, doc) })
						}
					})
				})
				ui.Row(c).Margin(24, 0, 0, 0).Justify(ui.End).Children(func() {
					cancel := components.ModalTextButton(c, t, "cancel", locales.CommonCancel())
					if busy {
						components.Disable(cancel, 0.5)
					} else {
						cancel.OnClick(func() { m.open = false })
					}
				})
			})
		})
}

// guideRow：flex w-full items-center gap-3 px-4 py-3 hover:bg-brand-100，左侧 i-mdi-file-document-outline text-primary-600，
// 文件名 text-sm font-medium 与相对路径 text-xs，右侧 open-in-new（打开中为旋转图标）；disabled:opacity-60。
func guideRow(c *ui.Context, t *theme.Theme, doc vo.GameGuideDocument, opening, busy bool, onOpen func()) {
	row := ui.ButtonBase(c).FillWidth().Padding(12, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(doc.Name)
	if busy {
		components.Disable(row, 0.6).Cursor(ui.CursorDefault)
	} else if row.Hovered() {
		row.Background(t.Pick(theme.Brand100, theme.Brand800))
	}
	primary := t.Pick(ui.Hex("#6d5dd3"), ui.Hex("#a78bfa"))
	row.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(onOpen).Children(func() {
		components.Icon(c, "file-document-outline", 20, primary)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, doc.Name, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100)).SingleLine()
			components.TextMargin(c, doc.RelativePath, theme.TextXS, theme.WeightNormal, t.TextMuted, 2, 0, 0, 0).SingleLine()
		})
		if opening {
			components.Spinner(c, 18, primary).Label(locales.GameEditOpeningGuideDocument())
		} else {
			components.Icon(c, "open-in-new", 18, theme.Brand400)
		}
	})
}
