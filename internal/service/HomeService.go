package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/vo"
)

type HomeService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewHomeService() *HomeService {
	return &HomeService{}
}

func (s *HomeService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *HomeService) GetHomePageData() (vo.HomePageData, error) {
	return vo.HomePageData{}, nil
}

func (s *HomeService) GetOrCreateCurrentUser() (models.User, error) {
	return models.User{}, nil
}
