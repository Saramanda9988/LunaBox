package launch

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/backend/utils/processutils"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// processModal 是 ProcessSelectModal 的状态。
type processModal struct {
	open      bool
	loading   bool
	gen       int
	processes []processutils.ProcessInfo
	search    string
	selected  string
	list      components.ScrollBar
}

// show 对应 isOpen 变为 true 时的 useEffect：重新加载进程列表，清空选择与搜索。
func (m *processModal) show(c *ui.Context, p panel.Game) {
	m.open, m.selected, m.search = true, "", ""
	m.list = components.ScrollBar{}
	m.load(c, p)
}

// load 对应 loadProcesses（GetRunningProcesses）。
func (m *processModal) load(c *ui.Context, p panel.Game) {
	m.loading = true
	m.gen++
	gen, e := m.gen, p.Env
	e.Async(func() func() {
		list, err := e.Svc.Game.GetRunningProcesses()
		return func() {
			if gen != m.gen {
				return
			}
			m.loading = false
			if err != nil {
				e.Toast(c, env.Error(locales.ProcessSelectModalToastLoadFailed()))
				return
			}
			m.processes = list
		}
	})
}

// confirm 对应 handleConfirm：UpdateGameProcessName 写入数据库，再交给页面（onSelected → updateGameState）。
func (m *processModal) confirm(c *ui.Context, l *Launcher, p panel.Game) {
	if m.selected == "" {
		p.Env.Toast(c, env.Error(locales.ProcessSelectModalToastSelectProcess()))
		return
	}
	name, id, e := m.selected, p.Game.ID, p.Env
	e.Async(func() func() {
		err := e.Svc.Game.UpdateGameProcessName(id, name)
		return func() {
			if err != nil {
				e.Toast(c, env.Error(locales.ProcessSelectModalToastSaveFailed()))
				return
			}
			e.Toast(c, env.Success(locales.ProcessSelectModalToastSetSuccess(name)))
			l.change(id, func(g *models.Game) { g.ProcessName = name })
			m.open = false
		}
	})
}

// view 对应 ProcessSelectModal：w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-brand-800 border。
// 标题行 flex items-start gap-4 mb-4，左侧 p-2 rounded-full bg-warning-100 text-warning-600
// dark:bg-warning-900/30 dark:text-warning-400 的 i-mdi-application-cog text-2xl；搜索框 mb-4；
// 列表 h-64 overflow-y-auto rounded-lg border bg-brand-50 dark:bg-brand-900；刷新 mt-2；按钮 mt-6 gap-3。
// 遮罩不响应点击与 Esc（原版没有）。
func (m *processModal) view(c *ui.Context, t *theme.Theme, l *Launcher, p panel.Game) {
	components.Modal(c, t, "process-select", components.ModalProps{Open: m.open, TopOffset: panel.TopOffset, MaxWidth: 512}, func(pn ui.Element) {
		components.ModalPanel(t, pn).Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Margin(0, 0, 16, 0).Children(func() {
				ui.Box(c).Padding(8).Radius(999).Shrink(0).Background(t.Pick(warning100, warning900.Alpha(0.3))).Children(func() {
					components.Icon(c, "application-cog", 24, t.Pick(warning600, warning400))
				})
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.TextMargin(c, locales.ProcessSelectModalTitle(), theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 4, 0)
					components.Text(c, locales.ProcessSelectModalDesc(),
						textSMRelax, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
				})
			})
			m.searchInput(c, t)
			m.processList(c, t)
			m.refreshButton(c, t, p)
			ui.Row(c).Margin(24, 0, 0, 0).Gap(12).Justify(ui.End).Children(func() {
				components.ModalTextButton(c, t, "process-cancel", locales.ProcessSelectModalCancelBtn()).OnClick(func() { m.open = false })
				primaryModalButton(c, t, "process-confirm", locales.ProcessSelectModalConfirmBtn(), m.selected == "").OnClick(func() { m.confirm(c, l, p) })
			})
		})
	})
}

// searchInput：relative mb-4，i-mdi-magnify 在 left-3 垂直居中（text-brand-400）；输入框 w-full pl-10 pr-4 py-2
// rounded-lg border border-brand-200 dark:border-brand-600 bg-white dark:bg-brand-700 text-brand-900 dark:text-white
// focus:ring-2 focus:ring-primary-500。
func (m *processModal) searchInput(c *ui.Context, t *theme.Theme) {
	ui.Box(c.Key("process-search")).Margin(0, 0, 16, 0).Children(func() {
		in := ui.TextInputBase(c.Key("input"), &m.search).FillWidth().Padding(8, 16, 8, 40).Radius(8).
			FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
			Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand200, theme.Brand600)).
			Placeholder(locales.ProcessSelectModalSearchPlaceholder()).Label(locales.ProcessSelectModalSearchPlaceholder())
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		if in.Focused() {
			in.Shadow(0, 0, 0, 2, theme.Primary500)
		}
		ui.Box(c).Absolute().Left(12).Top(0).Bottom(0).Center().PassThrough().Children(func() {
			components.Icon(c, "magnify", 16, theme.Brand400)
		})
	})
}

// processList：h-64 overflow-y-auto rounded-lg border border-brand-200 dark:border-brand-600 bg-brand-50 dark:bg-brand-900，
// 加载中与空列表居中显示；每行 w-full flex justify-between px-4 py-3（divide-y），左侧 i-mdi-application text-lg 与
// font-mono text-sm 的进程名，右侧 text-xs 的“PID:”。选中 bg-primary-100 dark:bg-primary-900/30，
// 悬停 hover:bg-brand-100 dark:hover:bg-brand-800。
func (m *processModal) processList(c *ui.Context, t *theme.Theme) {
	query := strings.ToLower(m.search)
	var filtered []processutils.ProcessInfo
	for _, pr := range m.processes {
		if strings.Contains(strings.ToLower(pr.Name), query) {
			filtered = append(filtered, pr)
		}
	}
	wrap := func(w ui.Element) {
		w.FillWidth().Height(256).Radius(8).Border(t.HL, t.Pick(theme.Brand200, theme.Brand600)).
			Background(t.Pick(theme.Brand50, theme.Brand900))
	}
	if m.loading || len(filtered) == 0 {
		box := ui.Row(c.Key("process-list")).Center()
		wrap(box)
		box.Children(func() {
			if m.loading {
				components.Spinner(c, 24, theme.Primary500)
				components.TextMargin(c, locales.ProcessSelectModalLoading(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400), 0, 0, 0, 8)
				return
			}
			msg := locales.ProcessSelectModalNoProcesses()
			if m.search != "" {
				msg = locales.ProcessSelectModalNoMatch()
			}
			components.Text(c, msg, theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		return
	}
	gutter := float32(0)
	if m.list.Overflowing() {
		gutter = t.Gutter()
	}
	components.ScrollArea(c, t, "process-list", &m.list, wrap, func(sc ui.Element) {
		sc.Padding(0, gutter, 0, 0).Children(func() {
			ui.Column(c).Children(func() {
				for i, pr := range filtered {
					m.processRow(c.Key(fmt.Sprintf("%s-%d", pr.Name, pr.PID)), t, pr, i > 0)
				}
			})
		})
	})
}

func (m *processModal) processRow(c *ui.Context, t *theme.Theme, pr processutils.ProcessInfo, divider bool) {
	selected := m.selected == pr.Name
	btn := ui.ButtonBase(c).FillWidth().Padding(12, 16).AlignItems(ui.Center).Justify(ui.SpaceBetween).FocusRing(false).Label(pr.Name)
	if divider {
		btn.BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700))
	}
	switch {
	case selected:
		btn.Background(t.Pick(primary100, primary900.Alpha(0.3)))
	case btn.Hovered():
		btn.Background(t.Pick(theme.Brand100, theme.Brand800))
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	iconColor, nameColor, weight := t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.Brand700, theme.Brand300), theme.WeightNormal
	if selected {
		iconColor, nameColor, weight = t.Pick(primary600, primary400), t.Pick(primary700, primary300), theme.WeightMedium
	}
	btn.OnClick(func() { m.selected = pr.Name }).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).MinWidth(0).Children(func() {
			components.Icon(c, "application", 18, iconColor)
			monoText(c, pr.Name, theme.TextSM, weight, nameColor).SingleLine()
		})
		components.Text(c, fmt.Sprintf("PID:%d", pr.PID), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand400, theme.Brand500)).NoWrap()
	})
}

// refreshButton：mt-2 flex justify-start，flex items-center gap-1 text-sm text-brand-600 hover:text-primary-600
// dark:text-brand-400 dark:hover:text-primary-400 disabled:opacity-50，加载时图标旋转。
func (m *processModal) refreshButton(c *ui.Context, t *theme.Theme, p panel.Game) {
	ui.Row(c).Margin(8, 0, 0, 0).Justify(ui.Start).Children(func() {
		btn := ui.ButtonBase(c.Key("process-refresh")).Gap(4).AlignItems(ui.Center).FocusRing(false)
		color := t.Pick(theme.Brand600, theme.Brand400)
		if btn.Hovered() && !m.loading {
			color = t.Pick(primary600, primary400)
		}
		color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
		if m.loading {
			components.Disable(btn, 0.5)
		}
		btn.OnClick(func() { m.load(c, p) }).Children(func() {
			ic := components.Icon(c, "refresh", 14, color)
			if m.loading {
				ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
			}
			components.Text(c, locales.ProcessSelectModalRefreshBtn(), theme.TextSM, theme.WeightNormal, color).NoWrap()
		})
	})
}
