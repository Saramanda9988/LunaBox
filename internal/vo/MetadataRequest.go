package vo

import "lunabox/internal/enums"

type MetadataRequest struct {
	Source enums.SourceType `json:"source"` // "bangumi" or "vndb"
	ID     string           `json:"id"`
}
