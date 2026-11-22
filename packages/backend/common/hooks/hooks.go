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
	EventSessionEnded         = "session/ended"
	EventMetadataBeforeSearch = "metadata/before-search"
	EventMetadataAfterFetch   = "metadata/after-fetch"
)

// GameDeletedPayload 是 game/deleted 事件的负载。
type GameDeletedPayload struct {
	ID string `json:"id"`
}

// GameBeforeLaunchResult 是 game/before-launch bail 钩子的返回值。
type GameBeforeLaunchResult struct {
	Cancel bool   `json:"cancel"`
	Reason string `json:"reason,omitempty"`
}

// GameLaunchedPayload 是 game/launched 事件的负载。
type GameLaunchedPayload struct {
	Game      models.Game `json:"game"`
	SessionID string      `json:"sessionId"`
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

// SafeBus 把 nil Bus 规范为 Nop，方便 service 直接使用。
func SafeBus(b Bus) Bus {
	if b == nil {
		return Nop
	}
	return b
}
