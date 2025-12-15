package vo

import "lunabox/internal/enums"

type AISummaryRequest struct {
	ChatIDs []string `json:"chat_ids"`
}

type MetadataRequest struct {
	Source enums.SourceType `json:"source"` // "bangumi" or "vndb"
	ID     string           `json:"id"`
}
