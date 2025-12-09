package service

import (
	"context"
	"database/sql"
	"lunabox/internal/models"
)

type GameService struct {
	ctx context.Context
	db  *sql.DB
}

func NewGameService() *GameService {
	return &GameService{}
}

func (service *GameService) Init(ctx context.Context, db *sql.DB) {
	service.ctx = ctx
	service.db = db
}

func (service *GameService) AddGame(game models.Game) error {
	return nil
}

func (service *GameService) GetGames() ([]models.Game, error) {
	return nil, nil
}

func (service *GameService) GetGameByID(id string) (models.Game, error) {
	return models.Game{}, nil
}

func (service *GameService) UpdateGame(game models.Game) error {
	return nil
}

func (service *GameService) GetCategories() ([]models.Category, error) {
	return nil, nil
}

func (service *GameService) AddCategory(name string) error {
	return nil
}

func (service *GameService) AddGameToCategory(gameID, categoryID string) error {
	return nil
}
