// Package account 对应原版 components/panel/BangumiAccountSettings.tsx、HikarinagiAccountSettings.tsx、
// KungalAccountSettings.tsx（基础设置里的“账号授权”三张卡片），以及它们共用的 RemoteStatusSyncProgressModal、
// hooks/useRemoteStatusSync.ts 与 utils/*Auth.ts。
//
// 三个组件几乎相同：共用的部分写在这里与 card.go，每家不同的文字、图片、配置字段与服务调用在 providers.go 的 spec 里。
package account

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

// Provider 是账号来源，与 BasicSettingsPanel 的 AccountProvider 相同。
type Provider string

const (
	Bangumi    Provider = "bangumi"
	Hikarinagi Provider = "hikarinagi"
	Kungal     Provider = "kungal"
)

// Providers 是三张卡片的顺序。
var Providers = []Provider{Bangumi, Hikarinagi, Kungal}

// State 是三张卡片跨帧保留的状态，由设置页持有（经 basic.State）。
type State struct {
	cards  [3]cardState
	toasts []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）
}

// cardState 是一张卡片的 useState 与 useRemoteStatusSync 的状态。
type cardState struct {
	snapshot *authSnapshot // GetAuthStatus 的结果；nil 时从配置推导
	profile  *profileView  // GetProfile 的结果

	statusLoading, profileLoading bool
	authorizing, disconnecting    bool
	disconnectConfirm             bool // showDisconnectConfirm
	syncConfirm                   bool // showSyncConfirm
	gen                           int  // 最近一次 refreshStatus 的编号，丢弃过期结果

	// useRemoteStatusSync
	progress     vo.RemoteStatusSyncProgress
	syncing      bool
	progressOpen bool
	subscribed   bool // 已订阅 *:status-sync-progress（事件没有退订，只订阅一次）

	scroll components.ScrollBar // 展开内容的 overflow-y-auto
}

// CardProps 是 BasicSettingsPanel 传给每张卡片的 isExpanded、isContentVisible、onExpand。
type CardProps struct {
	Expanded       bool
	ContentVisible bool
	OnExpand       func()
}

// Card 构建一张账号卡片（BangumiAccountSettings 等），填满 BasicSettingsPanel 网格里的这一格（宽度由网格决定），
// 之后是这张卡片的确认弹窗与同步进度弹窗（浮层）。
func Card(c *ui.Context, t *theme.Theme, s *State, p panel.Settings, provider Provider, props CardProps) {
	sp := specFor(provider)
	cs := &s.cards[sp.index()]
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil
	s.subscribe(p, sp, cs)
	s.card(c, t, p, sp, cs, props)
	s.modals(c, t, p, sp, cs)
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// subscribe 对应 useRemoteStatusSync 里的 onWailsEvent：服务发出的同步进度打开进度弹窗，完成或失败时结束同步状态。
func (s *State) subscribe(p panel.Settings, sp *spec, cs *cardState) {
	if cs.subscribed || p.Env == nil || p.Env.Svc == nil || p.Env.Svc.Events == nil {
		return
	}
	cs.subscribed = true
	p.Env.Svc.Events.On(sp.event, func(data any) {
		var next vo.RemoteStatusSyncProgress
		switch v := data.(type) {
		case vo.RemoteStatusSyncProgress:
			next = v
		case *vo.RemoteStatusSyncProgress:
			if v == nil {
				return
			}
			next = *v
		default:
			return
		}
		cs.progress = next
		cs.progressOpen = true
		if next.Status == "done" || next.Status == "failed" {
			cs.syncing = false
		}
	})
}

// refreshStatus 对应 refreshStatus 与 refreshProfile：读取授权状态，已授权且不需要重新授权时再读取资料。
// done 在两者都完成后执行（原版 await refreshStatus() 之后的步骤）。
func (s *State) refreshStatus(p panel.Settings, sp *spec, cs *cardState, done func()) {
	finish := func() {
		if done != nil {
			done()
		}
	}
	if p.Env == nil || p.Env.Svc == nil {
		finish()
		return
	}
	cs.gen++
	gen, e := cs.gen, p.Env
	cs.statusLoading = true
	e.Async(func() func() {
		snapshot, err := sp.status(e.Svc)
		return func() {
			if gen != cs.gen {
				finish()
				return
			}
			if err != nil {
				cs.snapshot, cs.profile, cs.statusLoading = nil, nil, false
				finish()
				return
			}
			cs.snapshot = &snapshot
			if !snapshot.authorized || snapshot.needsReauth {
				cs.profile, cs.statusLoading = nil, false
				finish()
				return
			}
			cs.profileLoading = true
			e.Async(func() func() {
				profile, err := sp.profile(e.Svc)
				return func() {
					if gen != cs.gen {
						finish()
						return
					}
					cs.profile = nil
					if err == nil {
						cs.profile = &profile
					}
					cs.profileLoading, cs.statusLoading = false, false
					finish()
				}
			})
		}
	})
}

// authorize 对应 handleAuthorize：StartAuth 打开浏览器并等待回调（会阻塞，在后台执行），成功后重新读取配置与授权状态。
func (s *State) authorize(p panel.Settings, sp *spec, cs *cardState) {
	if cs.authorizing || p.Env == nil || p.Env.Svc == nil {
		return
	}
	cs.authorizing = true
	e := p.Env
	e.Async(func() func() {
		err := sp.start(e.Svc)
		return func() {
			if err != nil {
				s.toast(env.Error(sp.text.actionFailed(err.Error())))
				s.refreshStatus(p, sp, cs, func() { cs.authorizing = false })
				return
			}
			p.Refresh()
			s.refreshStatus(p, sp, cs, func() {
				s.toast(env.Success(sp.text.authSuccess()))
				cs.authorizing = false
			})
		}
	})
}

// disconnect 对应 handleDisconnect。
func (s *State) disconnect(p panel.Settings, sp *spec, cs *cardState) {
	if cs.disconnecting || p.Env == nil || p.Env.Svc == nil {
		return
	}
	cs.disconnecting = true
	e := p.Env
	e.Async(func() func() {
		err := sp.disconnect(e.Svc)
		return func() {
			if err != nil {
				s.toast(env.Error(sp.text.actionFailed(err.Error())))
				cs.disconnecting = false
				return
			}
			p.Refresh()
			s.refreshStatus(p, sp, cs, func() {
				s.toast(env.Success(sp.text.disconnectSuccess()))
				cs.disconnecting = false
			})
		}
	})
}

// startSync 对应 useRemoteStatusSync 的 startSync：打开进度弹窗，后台执行 SyncAllGameStatuses；
// 过程中的进度由事件更新，结束时用返回值，出错时在当前进度上标记失败。
func (s *State) startSync(p panel.Settings, sp *spec, cs *cardState) {
	if cs.syncing || p.Env == nil || p.Env.Svc == nil {
		return
	}
	cs.progress = initialProgress(sp.name())
	cs.progressOpen = true
	cs.syncing = true
	e := p.Env
	e.Async(func() func() {
		result, err := sp.sync(e.Svc)
		return func() {
			if err != nil {
				cs.progress.Status = "failed"
				cs.progress.LastError = err.Error()
			} else {
				cs.progress = result
			}
			cs.syncing = false
		}
	})
}

// initialProgress 对应 createInitialProgress。
func initialProgress(provider string) vo.RemoteStatusSyncProgress {
	return vo.RemoteStatusSyncProgress{Provider: provider, Status: "started", FailedGameNames: []string{}}
}

// closeProgress 对应 closeProgress：同步进行中不能关闭。
func (cs *cardState) closeProgress() {
	if !cs.syncing {
		cs.progressOpen = false
	}
}
