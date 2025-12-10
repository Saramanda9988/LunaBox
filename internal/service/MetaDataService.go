package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/vo"
)

type MetaDataService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewMetaDataService() *MetaDataService {
	return &MetaDataService{}
}

func (s *MetaDataService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *MetaDataService) FetchMetadata(req vo.MetadataRequest) (models.Game, error) {
	var game = models.Game{}

	var e error

	switch req.Source {
	case enums.Bangumi:
		game, e = fetchFromBangumi(req.ID)
	case enums.VNDB:
		game, e = fetchFromVNDB(req.ID)
	case enums.Local:
		game, e = fetchFromLocal(req.ID)
	}
	return game, e
}

func fetchFromBangumi(id string) (models.Game, error) {
	return models.Game{}, nil
}

func fetchFromVNDB(id string) (models.Game, error) {
	return models.Game{}, nil
}

func fetchFromLocal(id string) (models.Game, error) {
	return models.Game{}, nil
}
