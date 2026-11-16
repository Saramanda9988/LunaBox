package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/models"
	"lunabox/backend/service/cloudsync"
	"lunabox/backend/utils"
	"lunabox/backend/utils/metadata"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TagService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewTagService() *TagService {
	return &TagService{}
}

//wails:ignore
func (s *TagService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

// GetTagsByGame 获取指定游戏的所有 tag
func (s *TagService) GetTagsByGame(gameID string) ([]models.GameTag, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT id, game_id, name, source, weight, is_spoiler, created_at, COALESCE(updated_at, created_at)
		FROM game_tags
		WHERE game_id = ?
		ORDER BY weight DESC
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []models.GameTag
	for rows.Next() {
		var t models.GameTag
		if err := rows.Scan(&t.ID, &t.GameID, &t.Name, &t.Source, &t.Weight, &t.IsSpoiler, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// AddUserTag 用户手动添加 tag
func (s *TagService) AddUserTag(gameID string, tagName string) error {
	if tagName == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	id := uuid.New().String()
	now := time.Now()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
		VALUES (?, ?, ?, 'user', 1.0, false, ?, ?)
		ON CONFLICT (game_id, name, source) DO UPDATE SET
			weight = EXCLUDED.weight,
			is_spoiler = EXCLUDED.is_spoiler,
			updated_at = EXCLUDED.updated_at
	`, id, gameID, tagName, now, now)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddUserTag: failed for game %s tag %s: %v", gameID, tagName, err)
		return fmt.Errorf("failed to add user tag: %w", err)
	}

	if clearErr := cloudsync.DeleteTombstone(s.ctx, s.db, cloudsync.EntityGameTag, cloudsync.TagTombstoneID(gameID, "user", tagName)); clearErr != nil {
		applog.LogWarningf(s.ctx, "AddUserTag: failed to clear tag tombstone for %s/%s: %v", gameID, tagName, clearErr)
	}
	return nil
}

// DeleteTag 删除 tag。自动刮削 tag 允许手动删除，但重新刮削后可能再次出现。
func (s *TagService) DeleteTag(tagID string) error {
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete tag tx: %w", err)
	}
	defer tx.Rollback()

	var gameID string
	var source string
	var name string
	if err := tx.QueryRowContext(s.ctx, `
		SELECT game_id, source, name
		FROM game_tags
		WHERE id = ?
	`, tagID).Scan(&gameID, &source, &name); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("tag not found")
		}
		return fmt.Errorf("failed to query tag identity: %w", err)
	}

	result, err := tx.ExecContext(s.ctx, `
		DELETE FROM game_tags WHERE id = ?
	`, tagID)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tag not found")
	}

	if err := cloudsync.UpsertTombstone(s.ctx, tx, cloudsync.EntityGameTag, cloudsync.TagTombstoneID(gameID, source, name), time.Now()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete tag tx: %w", err)
	}

	return nil
}

// DeleteTags 在一个事务里批量删除 tag（详情页的批量删除、合并显示的同名标签），不存在的 ID 忽略。
func (s *TagService) DeleteTags(tagIDs []string) error {
	ids := utils.UniqueNonEmptyStrings(tagIDs)
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete tags tx: %w", err)
	}
	defer tx.Rollback()

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	placeholders := utils.BuildPlaceholders(len(ids))
	rows, err := tx.QueryContext(s.ctx, fmt.Sprintf(`
		SELECT game_id, source, name
		FROM game_tags
		WHERE id IN (%s)
	`, placeholders), args...)
	if err != nil {
		return fmt.Errorf("failed to query tag identities: %w", err)
	}
	var identities []string
	for rows.Next() {
		var gameID, source, name string
		if err := rows.Scan(&gameID, &source, &name); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan tag identity: %w", err)
		}
		identities = append(identities, cloudsync.TagTombstoneID(gameID, source, name))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to query tag identities: %w", err)
	}
	if len(identities) == 0 {
		return nil
	}

	if _, err := tx.ExecContext(s.ctx, fmt.Sprintf(`DELETE FROM game_tags WHERE id IN (%s)`, placeholders), args...); err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}
	now := time.Now()
	for _, identity := range identities {
		if err := cloudsync.UpsertTombstone(s.ctx, tx, cloudsync.EntityGameTag, identity, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete tags tx: %w", err)
	}
	return nil
}

// SearchTagsInLibrary 搜索库中匹配的 tag 名称（用于游戏库筛选）。只差大小写的名称算同一个标签
// （筛选也不区分大小写），返回用得最多的写法。
func (s *TagService) SearchTagsInLibrary(query string) ([]string, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT name FROM (
			SELECT name, ROW_NUMBER() OVER (PARTITION BY lower(name) ORDER BY COUNT(*) DESC, name) AS rn
			FROM game_tags
			WHERE name ILIKE ?
			GROUP BY name
		)
		WHERE rn = 1
		ORDER BY name
		LIMIT 50
	`, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search tags: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// FilterExistingTagNames 返回输入中已经存在于库内的 tag 名称（不区分大小写），保留输入顺序。
func (s *TagService) FilterExistingTagNames(names []string) ([]string, error) {
	candidates := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		normalizedName := strings.TrimSpace(name)
		if normalizedName == "" {
			continue
		}
		key := strings.ToLower(normalizedName)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, normalizedName)
	}
	if len(candidates) == 0 {
		return []string{}, nil
	}

	args := make([]interface{}, 0, len(candidates))
	for _, name := range candidates {
		args = append(args, strings.ToLower(name))
	}

	rows, err := s.db.QueryContext(s.ctx, fmt.Sprintf(`
		SELECT DISTINCT lower(name) FROM game_tags
		WHERE lower(name) IN (%s)
	`, utils.BuildPlaceholders(len(candidates))), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to filter existing tag names: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]struct{}, len(candidates))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		existing[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(existing))
	for _, name := range candidates {
		if _, exists := existing[strings.ToLower(name)]; exists {
			result = append(result, name)
		}
	}
	return result, nil
}

// GetGameIDsByTag 获取包含指定 tag 的所有游戏 ID（用于游戏库筛选，不区分大小写）
func (s *TagService) GetGameIDsByTag(tagName string) ([]string, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT DISTINCT game_id FROM game_tags WHERE lower(name) = lower(?)
	`, tagName)
	if err != nil {
		return nil, fmt.Errorf("failed to get game ids by tag: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// upsertScrapedTags groups incoming tags by provider and updates each provider independently.
func (s *TagService) upsertScrapedTags(gameID string, tags []metadata.TagItem) error {
	bySource := make(map[string][]metadata.TagItem)
	for _, tag := range tags {
		source := strings.ToLower(strings.TrimSpace(tag.Source))
		if source == "" || source == "user" {
			continue
		}
		tag.Source = source
		bySource[source] = append(bySource[source], tag)
	}
	for source, sourceTags := range bySource {
		if err := s.upsertScrapedTagsForSource(gameID, source, sourceTags); err != nil {
			return err
		}
	}
	return nil
}

// upsertScrapedTagsForSource 用 tags 替换 source 这一个来源的标签，其他来源与用户标签不动。
func (s *TagService) upsertScrapedTagsForSource(gameID string, source string, tags []metadata.TagItem) error {
	return s.writeScrapedTags(gameID, source, tags, false)
}

// replaceScrapedTags 用 source 的 tags 替换游戏全部自动获取的标签（其他来源的一并删除），只保留 user 来源的标签。
// 用于设置里开启了“更新元数据时替换旧标签”的更新。
func (s *TagService) replaceScrapedTags(gameID string, source string, tags []metadata.TagItem) error {
	return s.writeScrapedTags(gameID, source, tags, true)
}

func (s *TagService) writeScrapedTags(gameID string, source string, tags []metadata.TagItem, allSources bool) error {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" || source == "user" {
		return fmt.Errorf("invalid scraped tag source: %s", source)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 要替换的范围：这一个来源；allSources 时是 user 以外的全部来源。
	scope, scopeArgs := `game_id = ? AND source = ?`, []interface{}{gameID, source}
	if allSources {
		scope, scopeArgs = `game_id = ? AND source <> 'user'`, []interface{}{gameID}
	}

	rows, err := tx.QueryContext(s.ctx, `
		SELECT game_id, source, name
		FROM game_tags
		WHERE `+scope, scopeArgs...)
	if err != nil {
		return fmt.Errorf("failed to query existing scraped tags: %w", err)
	}

	existing := make(map[string]struct{})
	for rows.Next() {
		var existingGameID string
		var existingSource string
		var existingName string
		if err := rows.Scan(&existingGameID, &existingSource, &existingName); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan existing scraped tag: %w", err)
		}
		existing[cloudsync.TagTombstoneID(existingGameID, existingSource, existingName)] = struct{}{}
	}
	rows.Close()

	if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_tags WHERE `+scope, scopeArgs...); err != nil {
		return fmt.Errorf("failed to delete old scraped tags: %w", err)
	}

	now := time.Now()
	incoming := make(map[string]struct{}, len(tags))
	// 批量插入新 tag
	for _, t := range tags {
		t.Source = source
		id := uuid.New().String()
		identity := cloudsync.TagTombstoneID(gameID, t.Source, t.Name)
		incoming[identity] = struct{}{}
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (game_id, name, source) DO UPDATE SET
				id = EXCLUDED.id,
				weight = EXCLUDED.weight,
				is_spoiler = EXCLUDED.is_spoiler,
				updated_at = EXCLUDED.updated_at
		`, id, gameID, t.Name, t.Source, t.Weight, t.IsSpoiler, now, now); err != nil {
			return fmt.Errorf("failed to insert tag %s: %w", t.Name, err)
		}
		if err := cloudsync.DeleteTombstone(s.ctx, tx, cloudsync.EntityGameTag, identity); err != nil {
			return err
		}
	}

	for identity := range existing {
		if _, keep := incoming[identity]; keep {
			continue
		}
		if err := cloudsync.UpsertTombstone(s.ctx, tx, cloudsync.EntityGameTag, identity, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
