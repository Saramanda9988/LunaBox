package service

import (
	"context"
	"database/sql"
	"lunabox/internal/vo"
)

type StatsService struct {
	ctx context.Context
	db  *sql.DB
}

func NewStatsService() *StatsService {
	return &StatsService{}
}

func (s *StatsService) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

func (s *StatsService) GetStats() (vo.Stats, error) {
	return vo.Stats{}, nil
}
