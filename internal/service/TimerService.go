package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
)

type TimerService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewTimerService() *TimerService {
	return &TimerService{}
}

func (s *TimerService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *TimerService) ReportPlaySession(session models.PlaySession) error {
	return nil
}
