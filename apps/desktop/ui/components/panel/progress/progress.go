// Package progress 对应原版 components/panel/GameProgressPanel.tsx（详情页“游玩进度”标签页），
// 以及它的 components/modal/AddGameProgressModal.tsx 与 components/skeleton/GameProgressSkeleton.tsx。
package progress

import (
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

var (
	textLG       = theme.FontStep{Size: 18, Line: 28}    // text-lg
	textRelaxed  = theme.FontStep{Size: 14, Line: 22.75} // text-sm leading-relaxed
	trackingWide = float32(12 * 0.025)                   // text-xs tracking-wide
)

// skeletonDelay 对应原版加载 300ms 后才显示骨架屏。
const skeletonDelay = 300 * time.Millisecond

// spoilerOptions 对应 SPOILER_OPTIONS（gameProgress.spoilerBoundaryOpts）。
func spoilerOptions() []better.SelectOption {
	return []better.SelectOption{
		{Value: "none", Label: locales.GameProgressSpoilerBoundaryOptsNone()},
		{Value: "chapter_end", Label: locales.GameProgressSpoilerBoundaryOptsChapterEnd()},
		{Value: "route_end", Label: locales.GameProgressSpoilerBoundaryOptsRouteEnd()},
		{Value: "full", Label: locales.GameProgressSpoilerBoundaryOptsFull()},
	}
}

// spoilerHint 对应 spoilerHints（gameProgress.spoilerBoundaryHints）；没有对应的提示时为空（原版的 undefined）。
func spoilerHint(v string) string {
	switch v {
	case "none":
		return locales.GameProgressSpoilerBoundaryHintsNone()
	case "chapter_end":
		return locales.GameProgressSpoilerBoundaryHintsChapterEnd()
	case "route_end":
		return locales.GameProgressSpoilerBoundaryHintsRouteEnd()
	case "full":
		return locales.GameProgressSpoilerBoundaryHintsFull()
	}
	return ""
}

func spoilerLabel(v string) string {
	for _, o := range spoilerOptions() {
		if o.Value == v {
			return o.Label
		}
	}
	return v
}

// State 是面板跨帧保留的状态，由详情页持有；进入另一个游戏时清零。
type State struct {
	gameID    string // 已加载的游戏；与当前游戏不同时重新加载
	gen       int
	loading   bool
	loadStart time.Time
	history   []models.GameProgress

	modal  modalState
	toasts []func() env.Notice // 后台结果里待显示的提示（存函数，不存翻译后的文字）
}

var seq atomic.Int64

// nextGen 返回全局递增的请求编号：State 清零后编号也不会与旧请求重复。
func nextGen() int { return int(seq.Add(1)) }

// View 构建面板：glass-card bg-white dark:bg-brand-800 p-6 rounded-lg min-h-[22rem]，
// 标题与“新增记录”按钮，下面 mt-4 flex-1 min-h-[14rem] 的记录列表 / 骨架屏 / 空状态，最后是新增记录弹窗。
// 卡片只有 min-height，内层 h-full 与 flex-1 都撑不开，所以列表区就是 224 高，卡片底部留出空白（与原版相同）。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Game) {
	if p.Game == nil {
		return
	}
	if s.gameID != p.Game.ID {
		*s = State{gameID: p.Game.ID}
		s.load(p.Env, c.Now())
	}
	for _, msg := range s.toasts {
		p.Env.Toast(c, msg())
	}
	s.toasts = nil

	loc := timefmt.Location(p.Env.Config())
	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), ui.Transparent) // glass-card
	card := ui.Column(c.Key("progress-panel")).Padding(24).Radius(8).MinHeight(352).Background(bg)
	if t.Glass {
		card.Border(t.HL, border)
	}
	card.Children(func() {
		s.header(c, t, p, loc)
		ui.Column(c).Margin(16, 0, 0, 0).MinHeight(224).Children(func() {
			switch {
			case len(s.history) > 0:
				ui.Column(c).Gap(12).Children(func() {
					for _, gp := range s.history {
						progressCard(c.Key(gp.ID), t, gp, loc)
					}
				})
			case s.loading:
				if wait := skeletonDelay - c.Now().Sub(s.loadStart); wait > 0 {
					c.After(wait)
					ui.Box(c).MinHeight(224)
				} else {
					skeleton(c, t)
				}
			default:
				emptyState(c, t)
			}
		})
	})

	s.modalView(c, t, p)
}

// load 对应 loadHistory。
func (s *State) load(e *env.Env, now time.Time) {
	s.gen = nextGen()
	s.loading, s.loadStart = true, now
	gen, id := s.gen, s.gameID
	e.Async(func() func() {
		history, err := e.Svc.Progress.ListGameProgresses(id)
		return func() {
			if gen != s.gen {
				return
			}
			s.loading = false
			if err == nil {
				s.history = history
			}
		}
	})
}

// header：flex sm:flex-row sm:items-start sm:justify-between gap-3；左侧 space-y-1 的标题与最近记录时间
// （没有记录时是提示），右侧 BetterButton variant=primary icon=i-mdi-playlist-plus。
func (s *State) header(c *ui.Context, t *theme.Theme, p panel.Game, loc *time.Location) {
	ui.Row(c).Gap(12).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Gap(4).Shrink(1).MinWidth(0).Children(func() {
			components.Text(c, locales.GameProgressTitle(), textLG, theme.WeightSemibold, t.TextStrong)
			sub := locales.GameProgressHint()
			if len(s.history) > 0 {
				sub = locales.GameProgressLatestRecordedAt(timefmt.DateTime(s.history[0].UpdatedAt, loc))
			}
			components.Text(c, sub, theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		better.Button(c, t, "add-record", better.ButtonProps{
			Variant: better.ButtonPrimary, Icon: "playlist-plus", Label: locales.GameProgressAddRecord(),
		}).OnClick(func() { s.openModal(p) })
	})
}

// progressCard：article rounded-lg bg-brand-50 p-4 dark:bg-brand-700；时间行（i-mdi-circle-medium 与
// text-sm font-medium 的时间，右侧剧透范围徽标），mt-3 md:grid-cols-2 gap-3 的章节与路线，mt-3 的进度备注。
func progressCard(c *ui.Context, t *theme.Theme, gp models.GameProgress, loc *time.Location) {
	// data-glass:bg-white/1 data-glass:dark:bg-black/1
	bg := t.IfGlass(t.Pick(theme.Brand50, theme.Brand700), theme.White.Alpha(0.01), theme.Black.Alpha(0.01))
	ui.Column(c).Padding(16).Radius(8).Background(bg).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
				components.Icon(c, "circle-medium", 14, t.Pick(theme.Brand500, theme.Brand400))
				components.Text(c, timefmt.DateTime(gp.UpdatedAt, loc), theme.TextSM, theme.WeightMedium, t.TextStrong).Shrink(1)
			})
			// w-fit rounded-full bg-brand-200 px-2.5 py-1 text-xs font-medium text-brand-700 dark:bg-brand-600 dark:text-brand-100
			ui.Box(c).Padding(4, 10).Radius(999).Shrink(0).Background(t.Pick(theme.Brand200, theme.Brand600)).Children(func() {
				components.Text(c, spoilerLabel(gp.SpoilerBoundary), theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand100)).NoWrap()
			})
		})
		ui.Grid(c).Columns(2).GapX(12).GapY(12).Margin(12, 0, 0, 0).Children(func() {
			field(c, t, locales.GameProgressChapter(), orDash(gp.Chapter), theme.TextSM, t.TextStrong)
			field(c, t, locales.GameProgressRoute(), orDash(gp.Route), theme.TextSM, t.TextStrong)
		})
		note := gp.ProgressNote
		if note == "" {
			note = locales.GameProgressNoNote()
		}
		ui.Column(c).Margin(12, 0, 0, 0).Children(func() {
			field(c, t, locales.GameProgressProgressNote(), note, textRelaxed, t.Pick(theme.Brand700, theme.Brand200))
		})
	})
}

// field：space-y-1 min-w-0，标签 text-xs font-medium uppercase tracking-wide text-brand-500 dark:text-brand-400，
// 内容 break-words（备注另有 whitespace-pre-wrap leading-relaxed）。
func field(c *ui.Context, t *theme.Theme, label, value string, f theme.FontStep, color ui.Color) {
	ui.Column(c).Gap(4).MinWidth(0).Children(func() {
		components.Text(c, label, theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand500, theme.Brand400)).LetterSpacing(trackingWide)
		components.Text(c, value, f, theme.WeightNormal, color)
	})
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// emptyState：flex h-full min-h-[14rem] items-center justify-center rounded-lg border border-dashed
// border-brand-300 px-4 py-6 text-center text-sm text-brand-500 dark:border-brand-600 dark:text-brand-400。
func emptyState(c *ui.Context, t *theme.Theme) {
	ui.Column(c).MinHeight(224).Padding(24, 16).Radius(8).Center().
		Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).BorderStyle(ui.BorderDashed).Children(func() {
		components.Text(c, locales.GameProgressHistoryEmpty(), theme.TextSM, theme.WeightNormal,
			t.Pick(theme.Brand500, theme.Brand400)).TextAlign(ui.Center)
	})
}

// skeleton 对应 GameProgressSkeleton：space-y-3 animate-pulse 的两张卡片（rounded-lg bg-brand-100 p-4
// dark:bg-brand-700），色块 bg-brand-200/60 dark:bg-brand-600/60。
func skeleton(c *ui.Context, t *theme.Theme) {
	list := ui.Column(c.Key("skeleton")).Gap(12)
	list.Opacity(components.Pulse(list)).Children(func() {
		bar := t.Pick(theme.Brand200.Alpha(0.6), theme.Brand600.Alpha(0.6))
		block := func(w, h, r float32) ui.Element {
			b := ui.Box(c).Height(h).Radius(r).Background(bar)
			if w > 0 {
				b.Width(w)
			}
			return b
		}
		for range 2 {
			// data-glass:bg-white/2 data-glass:dark:bg-black/2
			bg := t.IfGlass(t.Pick(theme.Brand100, theme.Brand700), theme.White.Alpha(0.02), theme.Black.Alpha(0.02))
			ui.Column(c).Padding(16).Radius(8).Background(bg).Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					block(144, 20, 4)
					block(96, 24, 999)
				})
				ui.Grid(c).Columns(2).GapX(12).GapY(12).Margin(12, 0, 0, 0).Children(func() {
					ui.Column(c).Gap(8).Children(func() {
						block(64, 12, 4)
						block(112, 16, 4)
					})
					ui.Column(c).Gap(8).Children(func() {
						block(56, 12, 4)
						block(96, 16, 4)
					})
				})
				ui.Column(c).Gap(8).Margin(12, 0, 0, 0).Children(func() {
					block(80, 12, 4)
					block(0, 16, 4)
					block(0, 16, 4).WidthPercent(75)
				})
			})
		}
	})
}
