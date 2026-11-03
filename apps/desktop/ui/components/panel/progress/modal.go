package progress

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

var (
	neutral200 = ui.Hex("#e2e8f0")
	neutral900 = ui.Hex("#0f172a")
)

// modalState 是 AddGameProgressModal 的状态。
type modalState struct {
	open    bool
	loading bool
	saving  bool
	gen     int

	chapter, route, note, boundary string
}

// openModal 打开弹窗并读取最近一条记录：章节、路线与剧透范围沿用，备注留空（useEffect 的 load）。
func (s *State) openModal(p panel.Game) {
	m := &s.modal
	m.open, m.loading, m.saving = true, true, false
	m.gen = nextGen()
	gen, id, e := m.gen, p.Game.ID, p.Env
	e.Async(func() func() {
		latest, err := e.Svc.Progress.GetGameProgress(id)
		return func() {
			if gen != m.gen || !m.open {
				return
			}
			m.loading = false
			if err != nil {
				return
			}
			if latest != nil && latest.GameID != "" {
				m.chapter, m.route, m.note = latest.Chapter, latest.Route, ""
				m.boundary = latest.SpoilerBoundary
				if m.boundary == "" {
					m.boundary = "none"
				}
			} else {
				m.chapter, m.route, m.note, m.boundary = "", "", "", "none"
			}
		}
	})
}

// save 对应 handleSave：追加一条记录，成功后提示、刷新列表并关闭。
func (s *State) save(c *ui.Context, p panel.Game) {
	m := &s.modal
	if m.saving || m.loading {
		return
	}
	m.saving = true
	e, gen := p.Env, m.gen
	gp := models.GameProgress{GameID: p.Game.ID, Chapter: m.chapter, Route: m.route, ProgressNote: m.note, SpoilerBoundary: m.boundary}
	now := c.Now()
	e.Async(func() func() {
		_, err := e.Svc.Progress.UpsertGameProgress(gp)
		return func() {
			if gen != m.gen {
				return
			}
			m.saving = false
			if err != nil {
				s.toasts = append(s.toasts, func() env.Notice { return env.Error(locales.GameProgressToastSaveFailed()) })
				return
			}
			s.toasts = append(s.toasts, func() env.Notice { return env.Success(locales.GameProgressToastSaved()) })
			s.load(e, now)
			m.open = false
		}
	})
}

// modalView 对应 AddGameProgressModal：absolute inset-0 z-50 bg-black/50 backdrop-blur-sm 的遮罩，
// 面板 relative bg-white dark:bg-brand-800 rounded-lg shadow-xl w-full max-w-2xl mx-4 p-6（没有边框）。
// 原版没有 Esc 与点击遮罩关闭，只有右上角的关闭按钮。
func (s *State) modalView(c *ui.Context, t *theme.Theme, p panel.Game) {
	m := &s.modal
	components.Modal(c, t, "progress-modal", components.ModalProps{Open: m.open, TopOffset: panel.TopOffset, MaxWidth: 672, Label: locales.GameProgressNewRecord()},
		func(box ui.Element) {
			box.Radius(8).Padding(24).Background(t.Pick(theme.White, theme.Brand800)).
				Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1)).
				Children(func() {
					s.modalHeader(c, t)
					s.modalForm(c, t, p)
				})
		})
}

// modalHeader：flex justify-between items-center mb-4；左侧 gap-2 的 i-mdi-playlist-plus text-xl
// text-brand-600 dark:text-brand-400 与 h2 text-lg font-semibold，右侧 i-mdi-close text-xl 的关闭按钮
// （text-brand-500 hover:text-brand-700 dark:text-brand-400 dark:hover:text-white）。
func (s *State) modalHeader(c *ui.Context, t *theme.Theme) {
	ui.Row(c).Margin(0, 0, 16, 0).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			components.Icon(c, "playlist-plus", 20, t.Pick(theme.Brand600, theme.Brand400))
			components.Text(c, locales.GameProgressNewRecord(), textLG, theme.WeightSemibold, t.TextStrong)
		})
		btn := ui.ButtonBase(c.Key("close")).FocusRing(false).Label(locales.CommonClose())
		color := t.Pick(theme.Brand500, theme.Brand400)
		if btn.Hovered() {
			color = t.Pick(theme.Brand700, theme.White)
		}
		color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
		btn.OnClick(func() { s.modal.open = false }).Children(func() {
			components.Icon(c, "close", 20, color)
		})
	})
}

// modalForm：form space-y-4，每个字段 space-y-1.5（label text-sm font-medium text-brand-700 dark:text-brand-300），
// 读取中字段换成 animate-pulse 的 h-9 / h-20 色块；底部 flex justify-end pt-6 border-t 的提交按钮
// （mt-6 被 space-y-4 覆盖，实际上边距 16）。
func (s *State) modalForm(c *ui.Context, t *theme.Theme, p panel.Game) {
	m := &s.modal
	submit := func() { s.save(c, p) }
	ui.Column(c).Gap(16).Children(func() {
		formField(c, t, locales.GameProgressChapter(), func() {
			if m.loading {
				loadingBlock(c, t, 36)
				return
			}
			if textField(c, t, "chapter", &m.chapter, locales.GameProgressChapterPlaceholder()).Submitted() {
				submit()
			}
		})
		formField(c, t, locales.GameProgressRoute(), func() {
			if m.loading {
				loadingBlock(c, t, 36)
				return
			}
			if textField(c, t, "route", &m.route, locales.GameProgressRoutePlaceholder()).Submitted() {
				submit()
			}
		})
		formField(c, t, locales.GameProgressProgressNote(), func() {
			if m.loading {
				loadingBlock(c, t, 80)
				return
			}
			better.Textarea(c, t, "note", &m.note, better.TextareaProps{
				Placeholder: locales.GameProgressProgressNotePlaceholder(), Rows: 3, Font: theme.TextSM,
			}).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
		})
		formField(c, t, locales.GameProgressSpoilerBoundary(), func() {
			if m.loading {
				loadingBlock(c, t, 36)
				return
			}
			better.Select(c, t, "boundary", better.SelectProps{
				Value: m.boundary, Options: spoilerOptions(), OnChange: func(v string) { m.boundary = v },
			})
			components.Text(c, spoilerHint(m.boundary), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		ui.Row(c).Padding(24, 0, 0, 0).Justify(ui.End).Gap(12).
			BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
			submitButton(c, t, m.saving, m.saving || m.loading).OnClick(submit)
		})
	})
}

// formField：space-y-1.5 的标签与控件。
func formField(c *ui.Context, t *theme.Theme, label string, control func()) {
	ui.Column(c).Gap(6).Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
		control()
	})
}

// loadingBlock：bg-brand-100 dark:bg-brand-700 rounded-md animate-pulse。
func loadingBlock(c *ui.Context, t *theme.Theme, h float32) {
	b := ui.Box(c).Height(h).Radius(6).Background(t.Pick(theme.Brand100, theme.Brand700))
	b.Opacity(components.Pulse(b))
}

// textField：glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm
// focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm。
func textField(c *ui.Context, t *theme.Theme, key string, value *string, placeholder string) ui.Element {
	in := ui.TextInputBase(c.Key(key), value).FillWidth().Padding(8, 12).Radius(6).
		FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).TextColor(t.TextStrong).
		Background(t.Pick(theme.White, theme.Brand700)).Placeholder(placeholder).Label(placeholder).
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	if theme.UIFont != "" {
		in.Font(theme.UIFont)
	}
	if in.Focused() {
		in.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	in.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
	return in
}

// submitButton：flex items-center gap-2 px-4 py-2 rounded-lg bg-neutral-600 text-white text-sm font-medium
// hover:bg-neutral-700 dark:bg-white dark:text-neutral-900 dark:hover:bg-neutral-200 disabled:opacity-50，
// 图标 i-mdi-playlist-plus（保存中 i-mdi-loading animate-spin）。
func submitButton(c *ui.Context, t *theme.Theme, saving, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key("submit")).Padding(8, 16).Radius(8).Gap(8).AlignItems(ui.Center).FocusRing(false)
	bg, fg := theme.Neutral600, theme.White
	hover := theme.Neutral700
	if t.Dark {
		bg, fg, hover = theme.White, neutral900, neutral200
	}
	if btn.Hovered() && !disabled {
		bg = hover
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	label := locales.GameProgressAddRecord()
	if saving {
		label = locales.CommonSaving()
	}
	btn.Children(func() {
		if saving {
			components.Spinner(c, 14, fg)
		} else {
			components.Icon(c, "playlist-plus", 14, fg)
		}
		components.Text(c, label, theme.TextSM, theme.WeightMedium, fg).NoWrap()
	})
	return btn
}
