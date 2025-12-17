package vo

import "lunabox/internal/enums"

type AISummaryRequest struct {
	Dimension string `json:"dimension"` // week, month, year
}

type MetadataRequest struct {
	Source enums.SourceType `json:"source"` // "bangumi" or "vndb"
	ID     string           `json:"id"`
}

// ChatCompletionRequest OpenAI兼容的API请求/响应结构
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
