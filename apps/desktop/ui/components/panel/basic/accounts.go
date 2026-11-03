package basic

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/account"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// handleAccountExpand 的时长（ACCOUNT_CONTENT_FADE_MS、ACCOUNT_CARD_RESIZE_MS、ACCOUNT_CARD_RESIZE_BUFFER_MS），
// accountFrame 是原版用的 16ms 计时器。
const (
	accountContentFade      = 100 * time.Millisecond
	accountCardResize       = 180 * time.Millisecond // account-choice-transition：grid-template-columns 180ms ease
	accountCardResizeBuffer = 32 * time.Millisecond
	accountFrame            = 16 * time.Millisecond
)

// smBreakpoint 是 sm: 的 min-width: 640px（按窗口宽度，同原版的媒体查询）。
const smBreakpoint = 640

// accountPhase 是 handleAccountExpand 的阶段。
type accountPhase int

const (
	accountIdle      accountPhase = iota
	accountFadingOut              // 内容淡出（100ms），之后切换展开的卡片
	accountResizing               // 网格列宽过渡，之后内容淡入
)

// accountSection 对应 <section className="space-y-2">：标题与 account-choice-transition 网格
// （grid grid-cols-1 items-stretch gap-3，sm: 起三列 minmax(0,1fr)，展开的一列 6fr、其余 2fr）。
func (s *State) accountSection(c *ui.Context, t *theme.Theme, p panel.Settings) {
	s.tickAccounts(c)
	winW, _ := c.Size()
	wide := winW >= smBreakpoint
	reduce := c.Preferences().ReduceMotion
	label := locales.SettingsBasicAccountAuthorizationSectionLabel()

	ui.Column(c.Key("accounts")).FillWidth().Gap(8).Children(func() {
		panel.GroupTitle(c, t, label)
		var grid ui.Element
		if wide {
			grid = ui.Row(c.Key("grid-sm")).AlignItems(ui.Stretch)
		} else {
			grid = ui.Column(c.Key("grid"))
		}
		grid.FillWidth().Gap(12).Role(ui.RoleGroup).Label(label)
		// minmax(0, Nfr)：fr 的值随 grid-template-columns 的过渡插值（180ms ease）。
		var fr [3]float32
		if wide {
			target := accountColumns(s.expanded)
			for i, provider := range account.Providers {
				fr[i] = grid.AnimateWith("col-"+string(provider), target[i], accountCardResize, theme.EaseCSS)
			}
		}
		grid.Children(func() {
			for i, provider := range account.Providers {
				cell := ui.Column(c.Key(string(provider))).AlignItems(ui.Stretch)
				if wide {
					cell.Grow(fr[i]).Basis(0).MinWidth(0)
				} else {
					cell.FillWidth()
				}
				cell.Children(func() {
					account.Card(c, t, &s.Accounts, p, provider, account.CardProps{
						Expanded:       s.expanded == provider,
						ContentVisible: !s.contentHidden,
						OnExpand:       func() { s.expandAccount(provider, reduce, wide) },
					})
				})
			}
		})
	})
}

// accountColumns 是 accountGridColumns 里三列的 fr。
func accountColumns(expanded account.Provider) [3]float32 {
	switch expanded {
	case account.Bangumi:
		return [3]float32{6, 2, 2}
	case account.Hikarinagi:
		return [3]float32{2, 6, 2}
	case account.Kungal:
		return [3]float32{2, 2, 6}
	}
	return [3]float32{1, 1, 1}
}

// expandAccount 对应 handleAccountExpand：已展开或正在切换时忽略；系统要求减少动画时直接切换；否则先让卡片内容淡出，
// 100ms 后切换展开的卡片，等网格列宽过渡结束后再淡入（见 tickAccounts）。gridAnim 是点击时窗口是否 ≥ 640。
func (s *State) expandAccount(provider account.Provider, reduceMotion, gridAnim bool) {
	if provider == s.expanded || s.contentHidden {
		return
	}
	if reduceMotion {
		s.expanded = provider
		return
	}
	s.gridAnim = gridAnim
	s.contentHidden = true
	s.phase, s.next, s.phaseAt = accountFadingOut, provider, time.Time{}
}

// ResetAccounts 对应重新挂载 BasicSettingsPanel（进入设置页）：expandedAccount 回到 null，三张卡片等宽，
// 进行中的切换也作废。分区收起时原版不卸载面板（CollapsibleSection 的 hasOpened），展开的卡片保留。
func (s *State) ResetAccounts() {
	s.expanded, s.contentHidden = "", false
	s.phase, s.next, s.phaseAt = accountIdle, "", time.Time{}
}

// tickAccounts 用帧时钟推进 handleAccountExpand 的两个计时器。
//
// 原版淡入有两条路：切换后 setTimeout(180 + 32ms)，以及网格的 transitionend（grid-template-columns 的 180ms 过渡结束）
// 之后 setTimeout(16ms)，取先到的一个，所以有网格过渡时是切换后 196ms；没有过渡（窗口窄于 640）时是 16ms。
func (s *State) tickAccounts(c *ui.Context) {
	if s.phase == accountIdle {
		return
	}
	now := c.Now()
	if s.phaseAt.IsZero() {
		s.phaseAt = now
	}
	if s.phase == accountFadingOut {
		if wait := accountContentFade - now.Sub(s.phaseAt); wait > 0 {
			c.After(wait)
			return
		}
		s.expanded = s.next
		s.phase, s.phaseAt = accountResizing, now
	}
	delay := accountFrame
	if s.gridAnim {
		delay = min(accountCardResize+accountCardResizeBuffer, accountCardResize+accountFrame)
	}
	if wait := delay - now.Sub(s.phaseAt); wait > 0 {
		c.After(wait)
		return
	}
	s.contentHidden = false
	s.phase, s.next = accountIdle, ""
}
