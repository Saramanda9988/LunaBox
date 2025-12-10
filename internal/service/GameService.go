package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
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
