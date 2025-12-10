package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

type CategoryService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewCategoryService() *CategoryService {
	return &CategoryService{}
}

func (s *CategoryService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *CategoryService) GetCategories() ([]models.Category, error) {
	return nil, nil
}

func (s *CategoryService) AddCategory(name string) error {
	return nil
}

func (s *CategoryService) AddGameToCategory(gameID, categoryID string) error {
	return nil
}
