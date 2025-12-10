package info

import "lunabox/internal/models"

// Getter 获取元数据
type Getter interface {
	FetchMetadata(id string, token string) (models.Game, error)

	FetchMetadataByName(name string, token string) (models.Game, error)
}
