package importer

import (
	"context"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/importpath"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
	"strings"
	"time"
)

// ImportResult 导入结果
type ImportResult struct {
	Success          int      `json:"success"`           // 成功导入数量
	Skipped          int      `json:"skipped"`           // 跳过数量（已存在）
	Failed           int      `json:"failed"`            // 失败数量
	FailedNames      []string `json:"failed_names"`      // 失败的游戏名称
	SkippedNames     []string `json:"skipped_names"`     // 跳过的游戏名称
	SessionsImported int      `json:"sessions_imported"` // 导入的游玩记录数量
}

// PreviewGame 预览导入的游戏信息
type PreviewGame struct {
	Name         string    `json:"name"`
	Developer    string    `json:"developer"`
	SourceType   string    `json:"source_type"`
	Exists       bool      `json:"exists"`
	ConflictType string    `json:"conflict_type"`
	ExistingID   string    `json:"existing_id"`
	ExistingName string    `json:"existing_name"`
	AddTime      time.Time `json:"add_time"`
	HasPath      bool      `json:"has_path"`
}

type Dependencies struct {
	Ctx                          context.Context
	ListGames                    func() ([]models.Game, error)
	AddGame                      func(vo.GameMetadataFromWebVO) error
	AddItems                     func([]ImportItem) (ImportResult, error)
	AddSessions                  func([]models.PlaySession) error
	AllowDuplicateMetadataImport bool
}

type ImportItem struct {
	Source         vo.GameMetadataFromWebVO
	Sessions       []models.PlaySession
	DisplayName    string
	Path           string
	Action         string
	ExistingGameID string
	CoverLoader    func(models.Game) (string, error)
}

const (
	ImportActionCreate         = "create"
	ImportActionUpdateExisting = "update_existing"

	SamePathActionSkip  = "skip"
	SamePathActionMerge = "merge"

	ConflictTypeNone        = ""
	ConflictTypeSamePath    = "same_path"
	ConflictTypePath        = "path"
	ConflictTypeSource      = "source"
	ConflictTypeNameAndPath = "name_path"
)

type existingGameConflict struct {
	Type string
	Game models.Game
}

func NormalizeSamePathAction(action string) string {
	if strings.EqualFold(strings.TrimSpace(action), SamePathActionMerge) {
		return SamePathActionMerge
	}
	return SamePathActionSkip
}

type existingPreviewIndex struct {
	byPath     map[string]models.Game
	bySource   map[string]models.Game
	byNamePath map[string]models.Game
}

func newImportResult() ImportResult {
	return ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}
}

func (d Dependencies) existingGames(logPrefix string) ([]models.Game, map[string]string, map[string]string, error) {
	if d.ListGames == nil {
		return nil, nil, nil, fmt.Errorf("缺少游戏列表依赖")
	}

	existingGames, err := d.ListGames()
	if err != nil {
		applog.LogErrorf(d.Ctx, "%s: failed to get existing games: %v", logPrefix, err)
		return nil, nil, nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}

	existingNames := make(map[string]string)
	existingPaths := make(map[string]string)
	for _, g := range existingGames {
		if g.Name != "" {
			existingNames[strings.ToLower(g.Name)] = g.ID
		}
		if g.Path != "" {
			existingPaths[normalizeImportPath(g.Path)] = g.Name
		}
	}

	return existingGames, existingNames, existingPaths, nil
}

func (d Dependencies) existingNameSet(logPrefix string) (map[string]bool, error) {
	if d.ListGames == nil {
		return nil, fmt.Errorf("缺少游戏列表依赖")
	}

	existingGames, err := d.ListGames()
	if err != nil {
		applog.LogErrorf(d.Ctx, "%s: failed to get existing games: %v", logPrefix, err)
		return nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}

	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}
	return existingNames, nil
}

func newExistingPreviewIndex(existingGames []models.Game) existingPreviewIndex {
	idx := existingPreviewIndex{
		byPath:     make(map[string]models.Game, len(existingGames)),
		bySource:   make(map[string]models.Game, len(existingGames)),
		byNamePath: make(map[string]models.Game, len(existingGames)),
	}
	for _, game := range existingGames {
		if pathKey := normalizeImportPath(game.Path); pathKey != "" {
			idx.byPath[pathKey] = game
		}
		if sourceKey := previewSourceKey(string(game.SourceType), game.SourceID); sourceKey != "" {
			idx.bySource[sourceKey] = game
		}
		if namePathKey := previewNamePathKey(game.Name, game.Path); namePathKey != "" {
			idx.byNamePath[namePathKey] = game
		}
	}
	return idx
}

func previewSourceKey(sourceType string, sourceID string) string {
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	sourceID = strings.ToLower(strings.TrimSpace(sourceID))
	if sourceType == "" || sourceID == "" {
		return ""
	}
	return sourceType + "\x00" + sourceID
}

func previewNamePathKey(gameName string, exePath string) string {
	nameKey := strings.ToLower(strings.TrimSpace(gameName))
	pathKey := normalizeImportPath(exePath)
	if nameKey == "" || pathKey == "" {
		return ""
	}
	return nameKey + "\x00" + pathKey
}

func previewExists(idx existingPreviewIndex, gameName string, exePath string, sourceType string, sourceID string) bool {
	return previewConflict(idx, gameName, exePath, sourceType, sourceID).Type != ConflictTypeNone
}

func previewConflict(idx existingPreviewIndex, gameName string, exePath string, sourceType string, sourceID string) existingGameConflict {
	if pathKey := normalizeImportPath(exePath); pathKey != "" {
		if game, exists := idx.byPath[pathKey]; exists {
			return existingGameConflict{Type: ConflictTypeSamePath, Game: game}
		}
		for existingPath, game := range idx.byPath {
			if importpath.Conflicts(pathKey, existingPath) {
				return existingGameConflict{Type: ConflictTypePath, Game: game}
			}
		}
	}
	if sourceKey := previewSourceKey(sourceType, sourceID); sourceKey != "" {
		if game, exists := idx.bySource[sourceKey]; exists {
			return existingGameConflict{Type: ConflictTypeSource, Game: game}
		}
	}
	if namePathKey := previewNamePathKey(gameName, exePath); namePathKey != "" {
		if game, exists := idx.byNamePath[namePathKey]; exists {
			return existingGameConflict{Type: ConflictTypeNameAndPath, Game: game}
		}
	}
	return existingGameConflict{}
}

func findExistingGameConflict(existingGames []models.Game, existingNames map[string]string, existingPaths map[string]string, gameName string, exePath string) (existingGameConflict, bool) {
	pathKey := normalizeImportPath(exePath)
	if pathKey != "" {
		for _, game := range existingGames {
			if normalizeImportPath(game.Path) == pathKey {
				return existingGameConflict{Type: ConflictTypeSamePath, Game: game}, true
			}
		}
		if existingName, exists := findExistingPathConflict(existingPaths, exePath); exists {
			return existingGameConflict{
				Type: ConflictTypePath,
				Game: models.Game{Name: existingName},
			}, true
		}
	}

	if existingID, exists := existingNames[strings.ToLower(gameName)]; exists {
		for _, game := range existingGames {
			if game.ID == existingID && normalizeImportPath(game.Path) == pathKey {
				return existingGameConflict{Type: ConflictTypeNameAndPath, Game: game}, true
			}
		}
	}

	return existingGameConflict{}, false
}

func skipExistingGame(
	ctx context.Context,
	logPrefix string,
	result *ImportResult,
	existingGames []models.Game,
	existingNames map[string]string,
	existingPaths map[string]string,
	gameName string,
	exePath string,
) bool {
	if conflict, exists := findExistingGameConflict(existingGames, existingNames, existingPaths, gameName, exePath); exists {
		result.Skipped++
		if conflict.Type == ConflictTypeNameAndPath {
			result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
			return true
		}
		result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+conflict.Game.Name+")")
		return true
	}

	if _, exists := existingNames[strings.ToLower(gameName)]; exists {
		applog.LogInfof(ctx, "%s: importing duplicate name %s with different path: %s", logPrefix, gameName, exePath)
	}

	return false
}

func addImportedGame(deps Dependencies, source vo.GameMetadataFromWebVO) error {
	if deps.AddGame == nil {
		return fmt.Errorf("缺少游戏导入依赖")
	}
	return deps.AddGame(source)
}

func addImportedItems(deps Dependencies, items []ImportItem) (ImportResult, error) {
	if deps.AddItems != nil {
		return deps.AddItems(items)
	}

	result := newImportResult()
	for _, item := range items {
		if err := addImportedGame(deps, item.Source); err != nil {
			result.Failed++
			result.FailedNames = append(result.FailedNames, item.DisplayName)
			continue
		}
		addPlaySessions(deps, "ImportItems", &result, item.DisplayName, item.Sessions)
		result.Success++
	}
	return result, nil
}

func addPlaySessions(deps Dependencies, logPrefix string, result *ImportResult, gameName string, sessions []models.PlaySession) {
	if len(sessions) == 0 || deps.AddSessions == nil {
		return
	}

	if err := deps.AddSessions(sessions); err != nil {
		applog.LogWarningf(deps.Ctx, "%s: failed to import play sessions for game %s: %v", logPrefix, gameName, err)
		return
	}

	applog.LogInfof(deps.Ctx, "%s: imported %d play sessions for game %s", logPrefix, len(sessions), gameName)
	result.SessionsImported += len(sessions)
}

func tagsFromNames(names []string) []metadata.TagItem {
	if len(names) == 0 {
		return nil
	}

	tags := make([]metadata.TagItem, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, metadata.TagItem{
			Name:      name,
			Source:    "user",
			Weight:    1.0,
			IsSpoiler: false,
		})
	}
	return tags
}

func updateExistingIndexes(existingNames map[string]string, existingPaths map[string]string, game models.Game, gameName string, exePath string) {
	existingNames[strings.ToLower(gameName)] = game.ID
	if exePath != "" {
		existingPaths[normalizeImportPath(exePath)] = gameName
	}
}

func normalizeImportPath(path string) string {
	return importpath.Normalize(path)
}

func findExistingPathConflict(existingPaths map[string]string, path string) (string, bool) {
	pathKey := normalizeImportPath(path)
	if pathKey == "" {
		return "", false
	}
	if existingName, exists := existingPaths[pathKey]; exists {
		return existingName, true
	}
	for existingPath, existingName := range existingPaths {
		if importpath.Conflicts(pathKey, existingPath) {
			return existingName, true
		}
	}
	return "", false
}
