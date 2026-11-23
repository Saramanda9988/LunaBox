package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 描述树校验上限（见 docs/plugin-system.md §9.2）。
const (
	MaxUITreeDepth      = 64
	MaxUITreeNodes      = 2000
	MaxUITreeStringSize = 64 * 1024 // 64 KiB
)

// UINode 是插件描述树的单个节点，字段名与 §9.2 协议一一对应。
type UINode struct {
	Type     string            `json:"t"`
	Key      string            `json:"k,omitempty"`
	Props    map[string]any    `json:"p,omitempty"`
	Events   map[string]string `json:"on,omitempty"`
	Children []*UINode         `json:"c,omitempty"`
	Version  int               `json:"v,omitempty"`
}

// UITreeUpdate 是插件线程产出的完整描述树更新，经校验后投递到界面线程缓存。
type UITreeUpdate struct {
	InstanceID string  `json:"instance_id"`
	PluginID   string  `json:"plugin_id"`
	Version    uint64  `json:"version"`
	Tree       *UINode `json:"tree,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// UIEvent 是界面线程投递给插件线程的用户交互事件。
type UIEvent struct {
	InstanceID string `json:"instance_id"`
	PluginID   string `json:"plugin_id"`
	HandlerID  string `json:"handler_id"`
	Version    uint64 `json:"version"`
	Payload    any    `json:"payload,omitempty"`
}

// AllowedComponentTypes 是 apiVersion 1 白名单组件集合（见 §9.6）。
var AllowedComponentTypes = map[string]struct{}{
	"Column":      {},
	"Row":         {},
	"Box":         {},
	"ScrollArea":  {},
	"Grid":        {},
	"Text":        {},
	"Markdown":    {},
	"CodeBlock":   {},
	"Button":      {},
	"IconButton":  {},
	"Input":       {},
	"Textarea":    {},
	"Switch":      {},
	"Checkbox":    {},
	"Select":      {},
	"NumberInput": {},
	"Tabs":        {},
	"Badge":       {},
	"Card":        {},
	"Collapsible": {},
	"Progress":    {},
	"Image":       {},
	"Avatar":      {},
	"Empty":       {},
	"Skeleton":    {},
	"Tooltip":     {},
	"Table":       {},
	"Divider":     {},
	"Spacer":      {},
	"Icon":        {},
	"HeroBanner":  {},
	"NativeSlot":  {},
}

// DecodeAndValidateUITree 在插件线程解码并校验描述树（不阻塞界面线程）。
// 允许未知组件类型通过解码（由渲染器按开发/正式模式处理），但严格强制深度、节点数与字符串长度限制。
func DecodeAndValidateUITree(raw []byte) (*UINode, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "undefined" {
		return nil, nil
	}
	var root UINode
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("描述树 JSON 解码失败: %w", err)
	}
	nodeCount := 0
	if err := validateUINode(&root, 1, &nodeCount); err != nil {
		return nil, err
	}
	return &root, nil
}

func validateUINode(node *UINode, depth int, count *int) error {
	if node == nil {
		return nil
	}
	if depth > MaxUITreeDepth {
		return fmt.Errorf("描述树深度超过限制 (%d)", MaxUITreeDepth)
	}
	*count++
	if *count > MaxUITreeNodes {
		return fmt.Errorf("描述树节点数量超过限制 (%d)", MaxUITreeNodes)
	}
	if strings.TrimSpace(node.Type) == "" {
		return fmt.Errorf("描述树节点缺少类型 t")
	}
	if len(node.Key) > MaxUITreeStringSize {
		return fmt.Errorf("节点 key 超过长度限制")
	}
	if err := validatePropValue(node.Props, 0); err != nil {
		return err
	}
	for k, v := range node.Events {
		if len(k) > MaxUITreeStringSize || len(v) > MaxUITreeStringSize {
			return fmt.Errorf("节点事件名或处理器 ID 超过长度限制")
		}
	}
	for _, child := range node.Children {
		if err := validateUINode(child, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func validatePropValue(val any, depth int) error {
	if val == nil {
		return nil
	}
	if depth > MaxUITreeDepth {
		return fmt.Errorf("节点属性嵌套深度超过限制 (%d)", MaxUITreeDepth)
	}
	switch v := val.(type) {
	case string:
		if len(v) > MaxUITreeStringSize {
			return fmt.Errorf("描述树字符串长度超过 64 KiB 限制 (%d bytes)", len(v))
		}
	case map[string]any:
		for k, item := range v {
			if len(k) > MaxUITreeStringSize {
				return fmt.Errorf("描述树属性键长度超过限制")
			}
			if err := validatePropValue(item, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := validatePropValue(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// ConfigFieldOption 是 enum 类型配置字段的选项。
type ConfigFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ConfigFieldSchema 描述 ctx.config.define(schema) 的单个配置项。
type ConfigFieldSchema struct {
	Key         string              `json:"key"`
	Type        string              `json:"type"` // string | boolean | number | enum | secret
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	Placeholder string              `json:"placeholder,omitempty"`
	Default     any                 `json:"default,omitempty"`
	Options     []ConfigFieldOption `json:"options,omitempty"`
	Min         *float64            `json:"min,omitempty"`
	Max         *float64            `json:"max,omitempty"`
	Step        *float64            `json:"step,omitempty"`
}

// ConfigSchema 是插件声明的设置项 schema，用于在设置页自动生成分区。
type ConfigSchema struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Icon        string              `json:"icon,omitempty"`
	Fields      []ConfigFieldSchema `json:"fields"`
}
