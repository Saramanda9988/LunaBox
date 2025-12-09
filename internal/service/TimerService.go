package service

import (
	"context"
	"database/sql"
	"lunabox/internal/models"
)

type TimerService struct {
	ctx context.Context
	db  *sql.DB
}

func NewTimerService() *TimerService {
	return &TimerService{}
}

func (s *TimerService) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

func (s *TimerService) ReportPlaySession(session models.PlaySession) error {
	return nil
}
