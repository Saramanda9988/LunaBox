package service

import (
	"context"
	"database/sql"
	"lunabox/internal/models"
	"lunabox/internal/vo"
)

type HomeService struct {
	ctx context.Context
	db  *sql.DB
}

func NewHomeService() *HomeService {
	return &HomeService{}
}

func (s *HomeService) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

func (s *HomeService) GetHomePageData() (vo.HomePageData, error) {
	return vo.HomePageData{}, nil
}

func (s *HomeService) GetOrCreateCurrentUser() (models.User, error) {
	return models.User{}, nil
}
