package service

import (
	"context"
	"database/sql"
	"lunabox/internal/models"
	"lunabox/internal/vo"
)

type MetaDataService struct {
	ctx context.Context
	db  *sql.DB
}

func NewMetaDataService() *MetaDataService {
	return &MetaDataService{}
}

func (s *MetaDataService) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

func (s *MetaDataService) FetchMetadata(req vo.MetadataRequest) (models.Game, error) {
	return models.Game{}, nil
}
