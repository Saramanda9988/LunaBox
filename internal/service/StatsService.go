package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/vo"
)

type StatsService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewStatsService() *StatsService {
	return &StatsService{}
}

func (s *StatsService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *StatsService) GetStats() (vo.Stats, error) {
	return vo.Stats{}, nil
}
