package service

import (
	"context"
	"database/sql"
	"lunabox/internal/vo"
)

type AiService struct {
	ctx context.Context
	db  *sql.DB
}

func NewAiService() *AiService {
	return &AiService{}
}

func (s *AiService) Init(ctx context.Context, db *sql.DB) {
	s.ctx = ctx
	s.db = db
}

func (s *AiService) AISummarize(req vo.AISummaryRequest) (string, error) {
	return "", nil
}
