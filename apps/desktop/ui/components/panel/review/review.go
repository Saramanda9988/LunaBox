// Package review 对应原版 components/panel/GameReviewPanel.tsx（详情页“评价”标签页）：
// 十分制的星级评分、评价内容与剧透标记，修改后自动保存；已授权且游戏关联了 Bangumi / Hikarinagi 时可以同步评价。
package review

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/panel"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// contentMaxLength 对应 REVIEW_CONTENT_MAX_LENGTH（textarea 的 maxLength，按 UTF-16 码元计）。
const contentMaxLength = 300

// skeletonDelay 是加载 300ms 后才显示骨架屏。
const skeletonDelay = 300 * time.Millisecond

// autoSaveDelay 是最后一次修改后多久自动保存（原版 800ms 的 setTimeout）；测试里调成 0。
var autoSaveDelay = 800 * time.Millisecond

var (
	text2XLTight  = theme.FontStep{Size: 24, Line: 30} // text-2xl leading-tight
	textSMLeading = theme.FontStep{Size: 14, Line: 24} // text-sm leading-6
	amber300      = ui.Hex("#fcd34d")
	amber400      = ui.Hex("#fbbf24")
	neutral300    = ui.Hex("#cbd5e1")
	success400    = ui.Hex("#34d399")
	success600    = ui.Hex("#059669")
	defaultBorder = ui.Hex("#e5e7eb") // border-brand-250 不在色板里，回落到预设的 --un-default-border-color
)

// monoFont 是 font-mono 在 Windows 上实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

type saveStatus int

const (
	statusIdle saveStatus = iota
	statusPending
	statusSaving
	statusSaved
	statusError
)

// State 是面板跨帧保留的状态，由详情页持有；进入另一个游戏时清零。
type State struct {
	gameID    string // 已加载的游戏；与当前游戏不同时重新加载
	gen       int
	loading   bool
	loadStart time.Time

	rating  int // 0 是未评分（null）
	content string
	shown   string // 上一帧的 content，用来像 maxLength 那样只截掉超出的输入
	spoiler bool

	status    saveStatus
	inFlight  bool
	syncing   enums.SourceType // 正在同步的平台，空为没有
	revision  int              // editRevision
	timerFrom time.Time        // 自动保存计时的起点：最后一次修改，或上一次保存 / 同步结束（为零时在下一次构建取 c.Now()）

	auth   map[enums.SourceType]bool
	toasts []env.Notice // 后台结果里待显示的提示
}

var seq atomic.Int64

// nextGen 返回全局递增的请求编号：State 清零后编号也不会与旧请求重复。
func nextGen() int { return int(seq.Add(1)) }

// View 构建面板：section glass-card relative rounded-lg bg-white p-6 dark:bg-brand-800；
// 读取期间内容 invisible（保留占位），300ms 后盖上 animate-pulse 的骨架屏。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Game) {
	if p.Game == nil {
		return
	}
	if s.gameID != p.Game.ID {
		s.open(p, c.Now())
	}
	for _, msg := range s.toasts {
		p.Env.Toast(c, msg)
	}
	s.toasts = nil
	s.scheduleAutoSave(c, p)

	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), ui.Transparent)
	root := ui.Column(c.Key("review-panel")).Padding(24).Radius(8).Background(bg)
	if t.Glass {
		root.Border(t.HL, border)
	}
	root.Children(func() {
		body := ui.Column(c.Key("review-body"))
		if s.loading {
			body.Invisible()
		}
		body.Children(func() {
			s.header(c, t, p)
			s.editor(c, t)
			s.footer(c, t, p)
		})
		if s.loading {
			if wait := skeletonDelay - c.Now().Sub(s.loadStart); wait > 0 {
				c.After(wait)
			} else {
				skeleton(c, t)
			}
		}
	})
}

// open 对应读取评价的 useEffect：同时读取评价与 Bangumi、Hikarinagi 的授权状态（Promise.allSettled）。
func (s *State) open(p panel.Game, now time.Time) {
	*s = State{gameID: p.Game.ID, gen: nextGen(), loading: true, loadStart: now}
	e, gen, id := p.Env, s.gen, p.Game.ID
	e.Async(func() func() {
		rv, err := e.Svc.Review.GetGameReview(id)
		bangumi, bangumiErr := e.Svc.Bangumi.GetAuthStatus()
		hikarinagi, hikarinagiErr := e.Svc.Hikarinagi.GetAuthStatus()
		return func() {
			if gen != s.gen {
				return
			}
			switch {
			case err != nil:
				s.toasts = append(s.toasts, env.Error(locales.GameReviewToastLoadFailed(err.Error())))
			case rv != nil:
				s.rating = 0
				if rv.Rating != nil {
					s.rating = *rv.Rating
				}
				s.content, s.shown, s.spoiler = rv.Content, rv.Content, rv.IsSpoiler
				s.status = statusSaved
			}
			s.auth = map[enums.SourceType]bool{
				enums.Bangumi:    bangumiErr == nil && bangumi.Authorized && !bangumi.NeedsReauthorization,
				enums.Hikarinagi: hikarinagiErr == nil && hikarinagi.Authorized && !hikarinagi.NeedsReauthorization,
			}
			s.loading = false
			s.timerFrom = time.Time{}
		}
	})
}

// review 是要保存的评价（saveReview 的参数）。
func (s *State) review() models.GameReview {
	r := models.GameReview{GameID: s.gameID, Content: s.content, IsSpoiler: s.spoiler}
	if s.rating > 0 {
		v := s.rating
		r.Rating = &v
	}
	return r
}

// errEmptySaveResult 是后端没有返回保存结果（gameReview.toast.emptySaveResult）。
// 文字在显示时（主线程上的 Error）才按当前语言取。
type errEmptySaveResult struct{}

func (errEmptySaveResult) Error() string { return locales.GameReviewToastEmptySaveResult() }

// saveReview 对应 saveReview：保存并检查后端返回了结果。在后台线程调用。
func saveReview(p panel.Game, r models.GameReview) error {
	saved, err := p.Env.Svc.Review.SaveGameReview(r)
	if err != nil {
		return err
	}
	if saved == nil {
		return errEmptySaveResult{}
	}
	return nil
}

// markEdited 对应 markEdited：修订号加一，状态改为等待自动保存，重新计时。
func (s *State) markEdited(now time.Time) {
	s.revision++
	s.status = statusPending
	s.timerFrom = now
}

// scheduleAutoSave 对应自动保存的 useEffect：pending 且没有在读取、保存、同步时，最后一次变化 800ms 后保存。
func (s *State) scheduleAutoSave(c *ui.Context, p panel.Game) {
	if s.status != statusPending || s.loading || s.inFlight || s.syncing != "" {
		return
	}
	if s.timerFrom.IsZero() {
		s.timerFrom = c.Now() // 读取、保存、同步在两帧之间结束，从结束后的第一帧开始计时
	}
	if wait := autoSaveDelay - c.Now().Sub(s.timerFrom); wait > 0 {
		c.After(wait)
		return
	}
	rev, gen, r := s.revision, s.gen, s.review()
	s.inFlight, s.status = true, statusSaving
	p.Env.Async(func() func() {
		err := saveReview(p, r)
		return func() {
			if gen != s.gen {
				return
			}
			s.inFlight, s.timerFrom = false, time.Time{}
			switch {
			case err == nil && s.revision == rev:
				s.status = statusSaved
			case err == nil:
				s.status = statusPending
			case s.revision == rev:
				s.status = statusError
			}
		}
	})
}

// sync 对应 handleProviderSync：先保存，再把评价同步到 provider。
func (s *State) sync(p panel.Game, provider enums.SourceType) {
	rev, gen, r := s.revision, s.gen, s.review()
	s.syncing, s.status = provider, statusSaving
	e, id := p.Env, s.gameID
	fail := func(err error) {
		if s.revision == rev {
			s.status = statusError
		}
		s.toasts = append(s.toasts, env.Error(locales.GameReviewToastSyncFailed(err.Error())))
		s.syncing, s.timerFrom = "", time.Time{}
	}
	e.Async(func() func() {
		err := saveReview(p, r)
		return func() {
			if gen != s.gen {
				return
			}
			if err != nil {
				fail(err)
				return
			}
			if s.revision == rev {
				s.status = statusSaved
			} else {
				s.status = statusPending
			}
			e.Async(func() func() {
				result, err := e.Svc.Review.SyncGameReview(id, []enums.SourceType{provider})
				return func() {
					if gen != s.gen {
						return
					}
					if err != nil {
						fail(err)
						return
					}
					if result.Failed > 0 {
						msg := ""
						if len(result.Results) > 0 {
							msg = result.Results[0].Error
						}
						if msg == "" {
							msg = locales.GameReviewToastSyncPartial(strconv.Itoa(result.Succeeded), strconv.Itoa(result.Failed))
						}
						s.toasts = append(s.toasts, env.Error(msg))
					} else {
						s.toasts = append(s.toasts, env.Success(locales.GameReviewToastSynced(strconv.Itoa(result.Succeeded))))
					}
					s.syncing, s.timerFrom = "", time.Time{}
				}
			})
		}
	})
}

// gameHasProvider 对应 gameHasProvider：元数据来源或默认来源里有该平台且 ID 非空。
func gameHasProvider(g *models.Game, provider enums.SourceType) bool {
	for _, src := range g.MetadataSources {
		if src.SourceType == provider && strings.TrimSpace(src.SourceID) != "" {
			return true
		}
	}
	return g.SourceType == provider && strings.TrimSpace(g.SourceID) != ""
}

// header：p text-xs font-semibold uppercase tracking-[0.16em] 的小标题，h3 mt-2 text-2xl font-bold leading-tight
// 的游戏名，p mt-5 text-sm 的评分提示，mt-2 flex flex-wrap items-center gap-x-3 gap-y-2 的星级、分数与清除评分。
func (s *State) header(c *ui.Context, t *theme.Theme, p panel.Game) {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	// uppercase：只影响有大小写的语言（en-US 的 BEFORE YOU GO...）
	components.Text(c, strings.ToUpper(locales.GameReviewTitle()), theme.TextXS, theme.WeightSemibold, muted).LetterSpacing(12 * 0.16)
	components.TextMargin(c, p.Game.Name, text2XLTight, theme.WeightBold, t.TextStrong, 8, 0, 0, 0)
	components.TextMargin(c, locales.GameReviewRatingPrompt(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300), 20, 0, 0, 0)
	ui.Row(c).Margin(8, 0, 0, 0).Wrap().GapX(12).GapY(8).AlignItems(ui.Center).Children(func() {
		s.stars(c, t)
		label := locales.GameReviewRatingEmpty()
		if s.rating > 0 {
			label = locales.GameReviewRatingValue(strconv.Itoa(s.rating))
		}
		components.Text(c, label, theme.TextXS, theme.WeightNormal, muted).Font(monoFont).NoWrap()
		if s.rating > 0 {
			// border-0 bg-transparent p-0 text-xs text-brand-500 underline-offset-2 hover:text-brand-800 hover:underline
			// dark:text-brand-400 dark:hover:text-brand-200
			btn := ui.ButtonBase(c.Key("clear-rating")).FocusRing(false)
			hovered := btn.Hovered()
			color := muted
			if hovered {
				color = t.Pick(theme.Brand800, theme.Brand200)
			}
			btn.OnClick(func() {
				s.rating = 0
				s.markEdited(c.Now())
			}).Children(func() {
				txt := components.Text(c, locales.GameReviewClearRating(), theme.TextXS, theme.WeightNormal, color).NoWrap()
				if hovered {
					txt.Underline()
				}
			})
		}
	})
}

// stars 对应 StarRating：flex w-fit items-center gap-2 的五颗 h-8 w-8 星星，每颗左右两半各是一个分数；
// 底层 i-mdi-star-outline（text-brand-300 dark:text-brand-500），上层按 0/50/100% 宽度裁出 i-mdi-star
// （text-amber-400 dark:text-amber-300，transition-[width] duration-150）。悬停预览，移出整组时恢复。
func (s *State) stars(c *ui.Context, t *theme.Theme) {
	group := ui.Row(c.Key("stars")).Gap(8).AlignItems(ui.Center).Shrink(0).Label(locales.GameReviewRatingLabel())
	preview := ui.Local(group, "preview", func() int { return 0 })
	// 原版在半颗星的 mouseenter 时设置预览、离开整组时清除：停在星星之间的空隙里预览不变。
	if x, _, over := group.PointerPosition(); over {
		i := int(x / 40)
		if off := x - float32(i)*40; i >= 0 && i < 5 && off < 32 {
			*preview = i*2 + 1
			if off >= 16 {
				*preview++
			}
		}
	} else {
		*preview = 0
	}
	shown := s.rating
	if *preview > 0 {
		shown = *preview
	}
	outline, fill := t.Pick(theme.Brand300, theme.Brand500), t.Pick(amber400, amber300)
	group.Children(func() {
		for i := range 5 {
			left := i*2 + 1
			pct := float32(0)
			switch {
			case shown >= left+1:
				pct = 1
			case shown == left:
				pct = 0.5
			}
			star := ui.Box(c.Key(i)).Size(32, 32).Shrink(0)
			w := star.AnimateWith("fill", 32*pct, 150*time.Millisecond, theme.EaseTailwind)
			star.Children(func() {
				components.Icon(c, "star-outline", 32, outline).Absolute().Top(0).Left(0)
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Width(w).Clip().PassThrough().Children(func() {
					components.Icon(c, "star", 32, fill).Absolute().Top(0).Left(0)
				})
				for _, v := range []int{left, left + 1} {
					half := ui.ButtonBase(c.Key(v)).Absolute().Top(0).Bottom(0).Width(16).FocusRing(false).
						Label(locales.GameReviewRatingValue(strconv.Itoa(v)))
					if v == left {
						half.Left(0)
					} else {
						half.Right(0)
					}
					half.OnClick(func() {
						s.rating = v
						s.markEdited(c.Now())
					})
				}
			})
		}
	})
}

// editor 是 mt-6 的评价输入框与 mt-3 的剧透开关。
func (s *State) editor(c *ui.Context, t *theme.Theme) {
	ui.Column(c).Margin(24, 0, 0, 0).Children(func() {
		s.textarea(c, t)
		// mt-3 flex sm:flex-row sm:items-center sm:justify-between gap-3 rounded-lg bg-brand-50 px-3 py-2.5 dark:bg-brand-750
		ui.Row(c).Margin(12, 0, 0, 0).Padding(10, 12).Gap(12).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Background(t.Pick(theme.Brand50, theme.Brand750)).Children(func() {
			toggle := func(v bool) {
				s.spoiler = v
				s.markEdited(c.Now())
			}
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				// <label htmlFor>：点击标签切换开关。
				lbl := ui.ButtonBase(c.Key("spoiler-label")).AlignSelf(ui.Start).FocusRing(false)
				lbl.OnClick(func() { toggle(!s.spoiler) }).Children(func() {
					components.Text(c, locales.GameReviewSpoilerLabel(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100))
				})
				components.Text(c, locales.GameReviewSpoilerHint(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
			})
			better.Switch(c, t, "spoiler", s.spoiler, false, toggle)
		})
	})
}

// textarea：min-h-60 w-full rounded-xl border border-brand-250 bg-brand-50 px-5 py-4 pb-10 text-sm leading-6
// text-brand-900 dark:border-brand-700 dark:bg-brand-750 dark:text-white，聚焦时 border-neutral-500
// ring-2 ring-neutral-500/20（暗色 neutral-300）；右下角 absolute bottom-4 right-4 font-mono text-xs 的字数。
func (s *State) textarea(c *ui.Context, t *theme.Theme) {
	ui.Box(c.Key("content-box")).Children(func() {
		ta := ui.TextAreaBase(c.Key("content"), &s.content).FillWidth().Height(240).Padding(16, 20, 40, 20).Radius(12).
			FontSize(textSMLeading.Size).FixedLineHeight(textSMLeading.Line).TextColor(t.Pick(theme.Brand900, theme.White)).
			Background(t.Pick(theme.Brand50, theme.Brand750)).Label(locales.GameReviewContentLabel()).
			Placeholder(locales.GameReviewContentPlaceholder()).PlaceholderColor(t.Pick(theme.Brand400, theme.Brand500))
		if theme.UIFont != "" {
			ta.Font(theme.UIFont)
		}
		border := t.Pick(defaultBorder, theme.Brand700)
		if ta.Focused() {
			focus := t.Pick(theme.Neutral500, neutral300)
			border = focus
			ta.Shadow(0, 0, 0, 2, focus.Alpha(0.2))
		}
		ta.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
		if s.content != s.shown { // 只有输入框会改 content（读取时 shown 一起更新）
			s.content = clampEdit(s.shown, s.content, contentMaxLength)
			s.shown = s.content
			s.markEdited(c.Now())
		}
		components.Text(c, fmt.Sprintf("%d/%d", utf16Len(s.content), contentMaxLength), theme.TextXS, theme.WeightNormal,
			t.Pick(theme.Brand400, theme.Brand500)).Font(monoFont).NoWrap().Absolute().Bottom(16).Right(16).PassThrough()
	})
}

// footer：mt-6 flex sm:flex-row sm:items-center sm:justify-between gap-3 border-t border-brand-200 pt-5
// dark:border-brand-700；左侧自动保存状态（flex items-center gap-2 text-xs，图标 text-base），
// 右侧 flex gap-3 的两个同步按钮（BetterButton variant=primary icon=i-mdi-cloud-upload-outline）。
func (s *State) footer(c *ui.Context, t *theme.Theme, p panel.Game) {
	ui.Row(c).Margin(24, 0, 0, 0).Padding(20, 0, 0, 0).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		s.autoSaveLabel(c, t)
		ui.Row(c).Gap(12).Shrink(0).Children(func() {
			for _, prov := range []struct {
				source enums.SourceType
				name   string
			}{{enums.Bangumi, "Bangumi"}, {enums.Hikarinagi, "Hikarinagi"}} {
				can := s.auth[prov.source] && gameHasProvider(p.Game, prov.source)
				disabled := s.status == statusSaving || s.syncing != "" || !can || strings.TrimSpace(s.content) == ""
				better.Button(c, t, "sync-"+string(prov.source), better.ButtonProps{
					Variant:  better.ButtonPrimary,
					Icon:     "cloud-upload-outline",
					Loading:  s.syncing == prov.source,
					Disabled: disabled,
					Label:    locales.GameReviewSyncTo(prov.name),
				}).OnClick(func() { s.sync(p, prov.source) })
			}
		})
	})
}

// autoSaveLabel 是自动保存状态：保存中 i-mdi-loading animate-spin，已保存 i-mdi-check-circle-outline
// text-success-600 dark:text-success-400，失败 i-mdi-alert-circle-outline 且整行 text-error-600 dark:text-error-400。
// 其余状态原版用的 i-mdi-content-save-clock-outline 在 MDI 里不存在，不显示图标，只留下 gap-2 的空位。
func (s *State) autoSaveLabel(c *ui.Context, t *theme.Theme) {
	color := t.Pick(theme.Brand500, theme.Brand400)
	label := locales.GameReviewAutoSaveHint()
	switch s.status {
	case statusPending:
		label = locales.GameReviewAutoSavePending()
	case statusSaving:
		label = locales.GameReviewAutoSaveSaving()
	case statusSaved:
		label = locales.GameReviewAutoSaveSaved()
	case statusError:
		label = locales.GameReviewAutoSaveFailed()
		color = t.Pick(theme.Error600, theme.Error400)
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
		switch s.status {
		case statusSaving:
			components.Spinner(c, 16, color)
		case statusSaved:
			components.Icon(c, "check-circle-outline", 16, t.Pick(success600, success400))
		case statusError:
			components.Icon(c, "alert-circle-outline", 16, color)
		default:
			ui.Box(c).Width(0)
		}
		components.Text(c, label, theme.TextXS, theme.WeightNormal, color)
	})
}

// skeleton：absolute inset-0 animate-pulse p-6 的骨架屏，盖在 invisible 的内容上。
func skeleton(c *ui.Context, t *theme.Theme) {
	strong, weak := t.Pick(theme.Brand200, theme.Brand700), t.Pick(theme.Brand100, theme.Brand750)
	sk := ui.Column(c.Key("skeleton")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(24)
	sk.Opacity(components.Pulse(sk)).Children(func() {
		block := func(top, w, h, r float32, color ui.Color) ui.Element {
			b := ui.Box(c).Margin(top, 0, 0, 0).Height(h).Radius(r).Background(color)
			if w > 0 {
				b.Width(w).AlignSelf(ui.Start)
			}
			return b
		}
		block(0, 96, 16, 4, strong)
		block(8, 0, 32, 4, strong).WidthPercent(50).AlignSelf(ui.Start)
		block(20, 112, 16, 4, weak)
		block(8, 224, 32, 4, weak)
		block(24, 0, 240, 12, weak)
		block(12, 0, 64, 8, weak)
		ui.Row(c).Margin(24, 0, 0, 0).Padding(20, 0, 0, 0).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
			ui.Box(c).Size(112, 16).Radius(4).Background(weak)
			ui.Box(c).Size(224, 40).Radius(8).Background(strong)
		})
	})
}

// utf16Len 是 JavaScript 的 string.length。
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// clampEdit 像 textarea 的 maxLength 那样处理一次编辑：新文本超过 max 时只截掉这次插入的部分。
func clampEdit(old, cur string, max int) string {
	if utf16Len(cur) <= max {
		return cur
	}
	o, n := []rune(old), []rune(cur)
	pre := 0
	for pre < len(o) && pre < len(n) && o[pre] == n[pre] {
		pre++
	}
	suf := 0
	for suf < len(o)-pre && suf < len(n)-pre && o[len(o)-1-suf] == n[len(n)-1-suf] {
		suf++
	}
	head, inserted, tail := n[:pre], n[pre:len(n)-suf], n[len(n)-suf:]
	room := max - utf16Len(string(head)) - utf16Len(string(tail))
	keep := 0
	for keep < len(inserted) && room-utf16.RuneLen(inserted[keep]) >= 0 {
		room -= utf16.RuneLen(inserted[keep])
		keep++
	}
	// 原有文本本来就超长时（读到的评价），maxLength 不截断它，只是不再接受输入。
	return string(head) + string(inserted[:keep]) + string(tail)
}
