package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/info"
	"lunabox/internal/vo"
)

type GameService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewGameService() *GameService {
	return &GameService{}
}

func (s *GameService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *GameService) AddGame(game models.Game) error {
	return nil
}

func (s *GameService) DeleteGame(id string) error {
	return nil
}

func (s *GameService) GetGames() ([]models.Game, error) {
	return nil, nil
}

func (s *GameService) GetGameByID(id string) (models.Game, error) {
	return models.Game{}, nil
}

func (s *GameService) UpdateGame(game models.Game) error {
	return nil
}

func (s *GameService) FetchMetadata(req vo.MetadataRequest) (models.Game, error) {
	var game = models.Game{}
	var e error

	if game, e = fetchFromLocal(req.ID); e == nil {
		return game, nil
	}

	switch req.Source {
	case enums.Bangumi:
		bgmGetter := info.NewBangumiInfoGetter()
		game, e = bgmGetter.FetchMetadata(req.ID, s.config.BangumiAccessToken)
	case enums.VNDB:
		vndbGetter := info.NewVNDBInfoGetter()
		game, e = vndbGetter.FetchMetadata(req.ID, s.config.VNDBAccessToken)
	}
	return game, e
}

func fetchFromLocal(id string) (models.Game, error) {
	return models.Game{}, fmt.Errorf("not implemented")
}
