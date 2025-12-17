package vo

import "lunabox/internal/enums"

type AISummaryRequest struct {
	Dimension string `json:"dimension"` // week, month, year
}

type MetadataRequest struct {
	Source enums.SourceType `json:"source"` // "bangumi" or "vndb"
	ID     string           `json:"id"`
}
