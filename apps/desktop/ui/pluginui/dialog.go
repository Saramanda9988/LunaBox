package pluginui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// pendingDialog 是插件通过 ctx.ui.confirm / ctx.ui.select 请求、等待用户作答的对话框。
type pendingDialog struct {
	pluginID string
	id       string
	spec     plugin.DialogSpec
	selected string
	scroll   components.ScrollBar
}

func (m *Manager) openDialog(a plugin.UIAction) {
	if a.Dialog == nil || a.DialogID == "" {
		return
	}
	m.dialogs = append(m.dialogs, &pendingDialog{
		pluginID: a.PluginID,
		id:       a.DialogID,
		spec:     *a.Dialog,
		selected: a.Dialog.Default,
	})
	m.invalidate()
}

func (m *Manager) removeDialog(pluginID, id string) {
	for i, d := range m.dialogs {
		if d.pluginID == pluginID && d.id == id {
			m.dialogs = append(m.dialogs[:i], m.dialogs[i+1:]...)
			m.invalidate()
			return
		}
	}
}

// answerDialog 关闭对话框并把结果交还插件；result 为 nil 表示取消。
func (m *Manager) answerDialog(d *pendingDialog, result any) {
	m.removeDialog(d.pluginID, d.id)
	if m.env != nil && m.env.Svc != nil && m.env.Svc.Plugin != nil {
		m.env.Svc.Plugin.ResolveDialog(d.pluginID, d.id, result)
	}
}

func (m *Manager) invalidate() {
	if m.env != nil && m.env.Win != nil {
		m.env.Win.Invalidate()
	}
}

// DialogView 显示排在最前面的插件对话框（一次一个，作答后显示下一个）。要在页面最后构建，浮层才会盖住页面内容。
func (m *Manager) DialogView(c *ui.Context, t *theme.Theme, topOffset float32) {
	if len(m.dialogs) == 0 {
		return
	}
	d := m.dialogs[0]
	cancel := func() {
		if d.spec.Kind == "confirm" {
			m.answerDialog(d, false)
			return
		}
		m.answerDialog(d, nil)
	}
	maxW := float32(448)
	if d.spec.Kind == "select" {
		maxW = 520
	}
	key := "plugin-dialog-" + d.id
	components.Modal(c, t, key, components.ModalProps{Open: true, TopOffset: topOffset, MaxWidth: maxW, OnClose: cancel}, func(panel ui.Element) {
		components.ModalPanel(t, panel).Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				icon, bg, fg := "information", t.Pick(theme.Neutral100, ui.Hex("#0f172a").Alpha(0.3)), t.Pick(theme.Neutral600, theme.Neutral400)
				if d.spec.Danger {
					icon, bg, fg = "alert-circle", t.Pick(ui.Hex("#fee2e2"), ui.Hex("#7f1d1d").Alpha(0.3)), t.Pick(theme.Error600, theme.Error400)
				}
				ui.Box(c).Padding(8).Radius(999).Background(bg).Shrink(0).Children(func() {
					components.Icon(c, icon, 24, fg)
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.TextMargin(c, d.spec.Title, theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 8, 0)
					if d.spec.Message != "" {
						components.Text(c, d.spec.Message, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
					}
					components.TextMargin(c, m.pluginName(d.pluginID), theme.TextXS, theme.WeightNormal, t.TextFaint, 8, 0, 0, 0)
				})
			})
			if d.spec.Kind == "select" {
				m.dialogOptions(c, t, d)
			}
			ui.Row(c).Margin(32, 0, 0, 0).Gap(12).Justify(ui.End).Children(func() {
				cancelText := d.spec.CancelText
				if cancelText == "" {
					cancelText = locales.CommonCancel()
				}
				components.ModalTextButton(c, t, "cancel", cancelText).OnClick(cancel)
				confirmText := d.spec.ConfirmText
				if confirmText == "" {
					confirmText = locales.CommonConfirm()
				}
				canConfirm := d.spec.Kind != "select" || d.selected != ""
				btn := components.ModalPrimaryButton(c, t, "confirm", confirmText, d.spec.Danger)
				if !canConfirm {
					btn.Opacity(0.5)
				}
				btn.OnClick(func() {
					switch {
					case d.spec.Kind == "select" && d.selected != "":
						m.answerDialog(d, d.selected)
					case d.spec.Kind != "select":
						m.answerDialog(d, true)
					}
				})
			})
		})
	})
}

// dialogOptions 是 select 对话框的选项列表（单选）。
func (m *Manager) dialogOptions(c *ui.Context, t *theme.Theme, d *pendingDialog) {
	ui.Box(c).Margin(20, 0, 0, 0).FillWidth().Children(func() {
		components.ScrollArea(c, t, "options-scroll", &d.scroll, func(w ui.Element) { w.FillWidth().MaxHeight(360) }, func(sc ui.Element) {
			sc.Children(func() {
				ui.Column(c.Key("options")).FillWidth().Gap(8).Padding(0, t.Gutter(), 0, 0).Children(func() {
					for _, opt := range d.spec.Options {
						m.dialogOption(c, t, d, opt)
					}
				})
			})
		})
	})
}

func (m *Manager) dialogOption(c *ui.Context, t *theme.Theme, d *pendingDialog, opt plugin.DialogOption) {
	selected := opt.Value == d.selected
	row := ui.ButtonBase(c.Key("opt-"+opt.Value)).Row().Gap(12).Padding(10, 12).AlignItems(ui.Center).FillWidth().
		Radius(t.ControlRadius(8)).FocusRing(false).Cursor(ui.CursorPointer).Label(opt.Label)
	accent := t.Pick(theme.Brand700, theme.Brand200)
	border, bg := t.PanelBorder, ui.RGBA(0, 0, 0, 0)
	if selected {
		border = accent
		bg = t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))
	} else if row.Hovered() {
		bg = t.Pick(theme.Brand50, theme.Brand700.Alpha(0.25))
	}
	row.Border(t.HL, border).Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	row.OnClick(func() {
		if d.selected != opt.Value {
			d.selected = opt.Value
			m.invalidate()
		}
	})
	row.Children(func() {
		// 单选圆点
		dot := t.TextFaint
		if selected {
			dot = accent
		}
		ui.Box(c).Size(16, 16).Radius(999).Border(1.5, dot).Center().Shrink(0).Children(func() {
			if selected {
				ui.Box(c).Size(8, 8).Radius(999).Background(accent)
			}
		})
		if opt.Icon != "" {
			components.Icon(c, opt.Icon, 20, t.TextMuted)
		}
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
			components.Text(c, opt.Label, theme.TextSM, theme.WeightMedium, t.TextStrong)
			if opt.Description != "" {
				components.Text(c, opt.Description, theme.TextXS, theme.WeightNormal, t.TextMuted)
			}
		})
	})
}
