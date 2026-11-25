// Package hooks 定义插件事件与钩子总线接口。
// 后端业务 service 只依赖本包，不依赖 plugin 包；未加载插件时注入 Nop。
package hooks

import (
	"context"
	"encoding/json"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/backend/utils/metadata"
)

// 事件名常量（apiVersion 1）。
const (
	EventAppReady             = "app/ready"
	EventGameAdded            = "game/added"
	EventGameUpdated          = "game/updated"
	EventGameDeleted          = "game/deleted"
	EventGameBeforeLaunch     = "game/before-launch"
	EventGameLaunched         = "game/launched"
	EventGameRunning          = "game/running"
	EventSessionEnded         = "session/ended"
	EventMetadataBeforeSearch = "metadata/before-search"
	EventMetadataAfterFetch   = "metadata/after-fetch"

	// 启动拦截器（Intercept）：可以修改启动选项或启动计划，也可以取消启动。
	EventLaunchPrepare = "launch/prepare"
	EventLaunchPlan    = "launch/plan"
)

// GameDeletedPayload 是 game/deleted 事件的负载。
type GameDeletedPayload struct {
	ID string `json:"id"`
}

// LaunchOptions 是 launch/prepare 拦截器看到的启动选项；nil 表示沿用游戏自身的配置。
type LaunchOptions struct {
	UseLocaleEmulator *bool   `json:"use_locale_emulator"`
	UseMagpie         *bool   `json:"use_magpie"`
	RunAsAdmin        *bool   `json:"run_as_admin"`
	WineRunner        *string `json:"wine_runner"`
	WineArgs          *string `json:"wine_args"`
	WinePrefix        *string `json:"wine_prefix"`
	UseSteam          *bool   `json:"use_steam"`
	UseCompatibility  *bool   `json:"use_compatibility"`
}

// LaunchPlan 是 launch/plan 拦截器可以修改的启动计划字段（进程检测相关字段不开放）。
type LaunchPlan struct {
	File       string            `json:"file"`
	Args       []string          `json:"args"`
	Dir        string            `json:"dir"`
	Env        map[string]string `json:"env"` // 追加的环境变量；Windows 经 ShellExecuteEx 启动，不生效
	RunAsAdmin bool              `json:"run_as_admin"`
}

// InterceptOutcome 是拦截器合并结果里的取消标记。
type InterceptOutcome struct {
	Cancel bool   `json:"cancel,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// LaunchPreparePayload 是 launch/prepare 拦截器的载荷（game 只读）。
type LaunchPreparePayload struct {
	Game    models.Game   `json:"game"`
	Options LaunchOptions `json:"options"`
	InterceptOutcome
}

// LaunchPlanPayload 是 launch/plan 拦截器的载荷（game 只读）。
type LaunchPlanPayload struct {
	Game models.Game `json:"game"`
	Plan LaunchPlan  `json:"plan"`
	InterceptOutcome
}

// GameBeforeLaunchPayload 是 game/before-launch 事件的负载：拦截器都已执行，进程即将启动。
type GameBeforeLaunchPayload struct {
	Game models.Game `json:"game"`
	Plan LaunchPlan  `json:"plan"`
}

// GameLaunchedPayload 是 game/launched 事件的负载。
type GameLaunchedPayload struct {
	Game      models.Game `json:"game"`
	SessionID string      `json:"sessionId"`
}

// GameRunningPayload 是 game/running 事件的负载：识别到实际运行的游戏进程（或进程交接到新进程）。
type GameRunningPayload struct {
	Game        models.Game `json:"game"`
	SessionID   string      `json:"sessionId"`
	PID         uint32      `json:"pid"`
	ProcessName string      `json:"processName"`
	Reason      string      `json:"reason"`
}

// SessionEndedPayload 是 session/ended 事件的负载。
type SessionEndedPayload struct {
	GameID    string `json:"gameId"`
	SessionID string `json:"sessionId"`
	Duration  int    `json:"duration"`
}

// MetadataBeforeSearchPayload 是 metadata/before-search waterfall 钩子的入参和出参。
type MetadataBeforeSearchPayload struct {
	Name   string           `json:"name"`
	Source enums.SourceType `json:"source"`
}

// MetadataAfterFetchPayload 是 metadata/after-fetch waterfall 钩子的入参。
type MetadataAfterFetchPayload struct {
	Source enums.SourceType        `json:"source"`
	Result metadata.MetadataResult `json:"result"`
}

// Bus 是插件事件与钩子派发总线。
// 实现必须满足：无监听者时 HasListeners 返回 false，避免热路径上做 JSON 序列化。
type Bus interface {
	// HasListeners 报告指定事件当前是否有至少一个监听者。
	HasListeners(event string) bool
	// Emit 异步依次投递给监听者，不等返回值，不阻塞调用方。
	Emit(event string, payload any)
	// Waterfall 按顺序调用监听者，每个监听者收到上一个的 JSON 返回值（返回 undefined 表示不改）。
	// 出错或超时跳过该监听者，沿用上一个值。返回最终的 JSON 字节。
	Waterfall(ctx context.Context, event string, initial any) (json.RawMessage, error)
	// Bail 按顺序调用监听者，第一个返回非 undefined 的结果生效；出错或超时视为 undefined。
	// matched 为 false 表示没有监听者给出有效返回值。
	Bail(ctx context.Context, event string, payload any) (result json.RawMessage, matched bool, err error)
	// Intercept 按顺序调用拦截器：返回 undefined 表示不改，返回对象则深合并进当前载荷（数组整体替换）；
	// 合并后 cancel 为 true 时立即停止。每个拦截器按注册时声明的 timeout 单独计时（默认 3 秒），
	// 出错或超时跳过该拦截器。返回最终载荷的 JSON；没有拦截器时返回 nil。
	Intercept(ctx context.Context, event string, payload any) (json.RawMessage, error)
}

// Nop 是无插件时使用的空实现，全部调用零开销返回。
var Nop Bus = nopBus{}

type nopBus struct{}

func (nopBus) HasListeners(string) bool                                        { return false }
func (nopBus) Emit(string, any)                                                {}
func (nopBus) Waterfall(context.Context, string, any) (json.RawMessage, error) { return nil, nil }
func (nopBus) Bail(context.Context, string, any) (json.RawMessage, bool, error) {
	return nil, false, nil
}
func (nopBus) Intercept(context.Context, string, any) (json.RawMessage, error) { return nil, nil }

// SafeBus 把 nil Bus 规范为 Nop，方便 service 直接使用。
func SafeBus(b Bus) Bus {
	if b == nil {
		return Nop
	}
	return b
}
