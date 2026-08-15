package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"strings"
	"time"

	"github.com/google/uuid"
)

type GameFilterPresetService struct {
	ctx context.Context
	db  *sql.DB
}

func NewGameFilterPresetService() *GameFilterPresetService {
	return &GameFilterPresetService{}
}

//wails:ignore
func (s *GameFilterPresetService) Init(ctx context.Context, db *sql.DB, _ *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
}

func (s *GameFilterPresetService) ListGameFilterPresets() ([]models.GameFilterPreset, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT id, name, tags, exclude_tags, status, exclude_status, created_at, updated_at
		FROM game_filter_presets
		ORDER BY created_at ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("读取游戏筛选预设失败: %w", err)
	}
	defer rows.Close()

	presets := make([]models.GameFilterPreset, 0)
	for rows.Next() {
		preset, err := scanGameFilterPreset(rows)
		if err != nil {
			return nil, err
		}
		presets = append(presets, preset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取游戏筛选预设失败: %w", err)
	}
	return presets, nil
}

func (s *GameFilterPresetService) CreateGameFilterPreset(req vo.SaveGameFilterPresetRequest) (models.GameFilterPreset, error) {
	normalized, err := normalizeGameFilterPresetRequest(req)
	if err != nil {
		return models.GameFilterPreset{}, err
	}

	tagsJSON, err := json.Marshal(normalized.Tags)
	if err != nil {
		return models.GameFilterPreset{}, fmt.Errorf("编码筛选预设标签失败: %w", err)
	}

	now := time.Now()
	preset := models.GameFilterPreset{
		ID:            uuid.New().String(),
		Name:          normalized.Name,
		Tags:          normalized.Tags,
		ExcludeTags:   normalized.ExcludeTags,
		Status:        normalized.Status,
		ExcludeStatus: normalized.ExcludeStatus,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if _, err := s.db.ExecContext(s.ctx, `
		INSERT INTO game_filter_presets (
			id, name, tags, exclude_tags, status, exclude_status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, preset.ID, preset.Name, string(tagsJSON), preset.ExcludeTags, preset.Status, preset.ExcludeStatus, preset.CreatedAt, preset.UpdatedAt); err != nil {
		return models.GameFilterPreset{}, fmt.Errorf("创建游戏筛选预设失败: %w", err)
	}
	return preset, nil
}

func (s *GameFilterPresetService) UpdateGameFilterPreset(id string, req vo.SaveGameFilterPresetRequest) (models.GameFilterPreset, error) {
	id = strings.TrimSpace(id)
	existing, err := s.getGameFilterPreset(id)
	if err != nil {
		return models.GameFilterPreset{}, err
	}

	normalized, err := normalizeGameFilterPresetRequest(req)
	if err != nil {
		return models.GameFilterPreset{}, err
	}
	tagsJSON, err := json.Marshal(normalized.Tags)
	if err != nil {
		return models.GameFilterPreset{}, fmt.Errorf("编码筛选预设标签失败: %w", err)
	}

	now := time.Now()
	if _, err := s.db.ExecContext(s.ctx, `
		UPDATE game_filter_presets
		SET name = ?, tags = ?, exclude_tags = ?, status = ?, exclude_status = ?, updated_at = ?
		WHERE id = ?
	`, normalized.Name, string(tagsJSON), normalized.ExcludeTags, normalized.Status, normalized.ExcludeStatus, now, id); err != nil {
		return models.GameFilterPreset{}, fmt.Errorf("修改游戏筛选预设失败: %w", err)
	}

	return models.GameFilterPreset{
		ID:            id,
		Name:          normalized.Name,
		Tags:          normalized.Tags,
		ExcludeTags:   normalized.ExcludeTags,
		Status:        normalized.Status,
		ExcludeStatus: normalized.ExcludeStatus,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     now,
	}, nil
}

func (s *GameFilterPresetService) DeleteGameFilterPreset(id string) error {
	id = strings.TrimSpace(id)
	if _, err := s.getGameFilterPreset(id); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(s.ctx, `
		DELETE FROM game_filter_presets
		WHERE id = ?
	`, id); err != nil {
		return fmt.Errorf("删除游戏筛选预设失败: %w", err)
	}
	return nil
}

func (s *GameFilterPresetService) getGameFilterPreset(id string) (models.GameFilterPreset, error) {
	if id == "" {
		return models.GameFilterPreset{}, fmt.Errorf("筛选预设 ID 不能为空")
	}

	row := s.db.QueryRowContext(s.ctx, `
		SELECT id, name, tags, exclude_tags, status, exclude_status, created_at, updated_at
		FROM game_filter_presets
		WHERE id = ?
	`, id)
	preset, err := scanGameFilterPreset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.GameFilterPreset{}, fmt.Errorf("筛选预设不存在")
	}
	if err != nil {
		return models.GameFilterPreset{}, err
	}
	return preset, nil
}

type gameFilterPresetScanner interface {
	Scan(dest ...any) error
}

func scanGameFilterPreset(scanner gameFilterPresetScanner) (models.GameFilterPreset, error) {
	var preset models.GameFilterPreset
	var tagsJSON string
	if err := scanner.Scan(
		&preset.ID,
		&preset.Name,
		&tagsJSON,
		&preset.ExcludeTags,
		&preset.Status,
		&preset.ExcludeStatus,
		&preset.CreatedAt,
		&preset.UpdatedAt,
	); err != nil {
		return models.GameFilterPreset{}, fmt.Errorf("读取筛选预设记录失败: %w", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &preset.Tags); err != nil {
		return models.GameFilterPreset{}, fmt.Errorf("解析筛选预设标签失败: %w", err)
	}
	if preset.Tags == nil {
		preset.Tags = []string{}
	}
	return preset, nil
}

func normalizeGameFilterPresetRequest(req vo.SaveGameFilterPresetRequest) (vo.SaveGameFilterPresetRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return req, fmt.Errorf("筛选预设名称不能为空")
	}

	trimmedTags := make([]string, 0, len(req.Tags))
	for _, tag := range req.Tags {
		trimmedTags = append(trimmedTags, strings.TrimSpace(tag))
	}
	req.Tags = utils.UniqueNonEmptyStrings(trimmedTags)
	if len(req.Tags) == 0 {
		req.ExcludeTags = false
	}

	if !isValidGameFilterPresetStatus(req.Status) {
		return req, fmt.Errorf("筛选预设包含无效的游戏状态")
	}
	if req.Status == "" {
		req.ExcludeStatus = false
	}
	if len(req.Tags) == 0 && req.Status == "" {
		return req, fmt.Errorf("筛选预设至少需要一个标签或游戏状态")
	}
	return req, nil
}

func isValidGameFilterPresetStatus(status enums.GameStatus) bool {
	if status == "" {
		return true
	}
	for _, item := range enums.AllGameStatuses {
		if item.Value == status {
			return true
		}
	}
	return false
}
