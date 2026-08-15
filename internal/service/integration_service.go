package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/service/integrator"
	"lunabox/internal/utils"
	"lunabox/internal/utils/apputils"
	"strings"
)

type SteamLaunchStatus struct {
	State          string `json:"state"`
	Ready          bool   `json:"ready"`
	SteamInstalled bool   `json:"steam_installed"`
	SteamRunning   bool   `json:"steam_running"`
	LaunchID       string `json:"launch_id"`
	LaunchKind     string `json:"launch_kind"`
	UserID         string `json:"user_id"`
	ProtonPrefix   string `json:"proton_prefix"`
}

type SteamImportResult struct {
	Status     SteamLaunchStatus `json:"status"`
	Imported   bool              `json:"imported"`
	BackupPath string            `json:"backup_path"`
}

type SteamBatchImportItemResult struct {
	GameID   string            `json:"game_id"`
	Status   SteamLaunchStatus `json:"status"`
	Imported bool              `json:"imported"`
	Error    string            `json:"error,omitempty"`
}

type SteamBatchImportResult struct {
	Items         []SteamBatchImportItemResult `json:"items"`
	ImportedCount int                          `json:"imported_count"`
	ExistingCount int                          `json:"existing_count"`
	FailedCount   int                          `json:"failed_count"`
	BackupPath    string                       `json:"backup_path"`
}

type SteamCompatibilityTool struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Path        string `json:"path"`
	BuiltIn     bool   `json:"built_in"`
}

type SteamCompatibilityInfo struct {
	Supported      bool                     `json:"supported"`
	SteamInstalled bool                     `json:"steam_installed"`
	SteamRoot      string                   `json:"steam_root"`
	AppID          string                   `json:"app_id"`
	ProtonPrefix   string                   `json:"proton_prefix"`
	CurrentTool    string                   `json:"current_tool"`
	DefaultTool    string                   `json:"default_tool"`
	Tools          []SteamCompatibilityTool `json:"tools"`
}

type IntegrationService struct {
	ctx         context.Context
	db          *sql.DB
	gameService *GameService
}

func NewIntegrationService() *IntegrationService {
	return &IntegrationService{}
}

//wails:ignore
func (s *IntegrationService) Init(ctx context.Context, db *sql.DB, _ *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
}

//wails:ignore
func (s *IntegrationService) SetGameService(gameService *GameService) {
	s.gameService = gameService
}

func (s *IntegrationService) GetGameSteamStatus(gameID string) (SteamLaunchStatus, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamLaunchStatus{}, err
	}
	result, err := integrator.ResolveSteamTarget(s.ctx, game)
	if err != nil {
		return SteamLaunchStatus{}, err
	}
	status := steamLaunchStatusFromIntegrator(result.Status)
	if status.Ready {
		if err := s.persistSteamIdentity(game.ID, status); err != nil {
			return SteamLaunchStatus{}, err
		}
	}
	return status, nil
}

func (s *IntegrationService) ImportGameToSteam(gameID string) (SteamImportResult, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamImportResult{}, err
	}
	result, err := integrator.ImportSteamShortcut(s.ctx, game)
	if err != nil {
		return SteamImportResult{}, err
	}
	status := steamLaunchStatusFromIntegrator(result.Status)
	if status.Ready {
		if err := s.persistSteamIdentity(game.ID, status); err != nil {
			return SteamImportResult{}, err
		}
	}
	return SteamImportResult{
		Status:     status,
		Imported:   result.Imported,
		BackupPath: result.BackupPath,
	}, nil
}

func (s *IntegrationService) SetGameSteamLaunchOptions(gameID string, launchOptions string) (SteamLaunchStatus, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamLaunchStatus{}, err
	}
	game.SteamLaunchOptions = normalizeSteamLaunchOptions(launchOptions)

	result, err := integrator.SetSteamLaunchOptions(s.ctx, game, game.SteamLaunchOptions)
	status := steamLaunchStatusFromIntegrator(result.Status)
	if err != nil {
		return status, err
	}
	if !status.Ready {
		return status, nil
	}
	if err := s.persistSteamIdentity(game.ID, status); err != nil {
		return SteamLaunchStatus{}, err
	}
	if err := s.persistSteamLaunchOptions(game.ID, game.SteamLaunchOptions); err != nil {
		return SteamLaunchStatus{}, err
	}
	return status, nil
}

func (s *IntegrationService) BatchImportGamesToSteam(gameIDs []string) (SteamBatchImportResult, error) {
	gameIDs = normalizeSteamBatchGameIDs(gameIDs)
	response := SteamBatchImportResult{
		Items: make([]SteamBatchImportItemResult, 0, len(gameIDs)),
	}
	if len(gameIDs) == 0 {
		return response, nil
	}

	gamesByID, err := s.getGames(gameIDs)
	if err != nil {
		return response, err
	}
	games := make([]models.Game, 0, len(gamesByID))
	for _, gameID := range gameIDs {
		if game, ok := gamesByID[gameID]; ok {
			games = append(games, game)
		}
	}

	batchResult, err := integrator.ImportSteamShortcuts(s.ctx, games)
	if err != nil {
		return response, err
	}
	response.BackupPath = batchResult.BackupPath

	resultsByID := make(map[string]integrator.SteamBatchItemResult, len(batchResult.Items))
	for _, item := range batchResult.Items {
		resultsByID[item.GameID] = item
	}

	readyItems := make([]SteamBatchImportItemResult, 0, len(batchResult.Items))
	for _, gameID := range gameIDs {
		game, exists := gamesByID[gameID]
		if !exists {
			response.Items = append(response.Items, SteamBatchImportItemResult{
				GameID: gameID,
				Error:  fmt.Sprintf("game not found with id: %s", gameID),
			})
			response.FailedCount++
			continue
		}

		item, exists := resultsByID[game.ID]
		if !exists {
			response.Items = append(response.Items, SteamBatchImportItemResult{
				GameID: gameID,
				Error:  "Steam batch import returned no result",
			})
			response.FailedCount++
			continue
		}

		serviceItem := SteamBatchImportItemResult{
			GameID:   gameID,
			Status:   steamLaunchStatusFromIntegrator(item.Result.Status),
			Imported: item.Result.Imported,
		}
		if item.Err != nil {
			serviceItem.Error = item.Err.Error()
		}
		response.Items = append(response.Items, serviceItem)

		if serviceItem.Error != "" || !serviceItem.Status.Ready {
			response.FailedCount++
			continue
		}
		readyItems = append(readyItems, serviceItem)
		if serviceItem.Imported {
			response.ImportedCount++
		} else {
			response.ExistingCount++
		}
	}

	if err := s.persistSteamIdentities(readyItems); err != nil {
		return SteamBatchImportResult{}, err
	}
	return response, nil
}

func (s *IntegrationService) GetGameSteamCompatibility(gameID string) (SteamCompatibilityInfo, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	info, err := integrator.GetSteamCompatibilityInfo(s.ctx, game)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	return steamCompatibilityInfoFromIntegrator(info), nil
}

func (s *IntegrationService) SetGameSteamCompatibilityTool(gameID string, toolName string) (SteamCompatibilityInfo, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	info, err := integrator.SetSteamCompatibilityTool(s.ctx, game, toolName)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	return steamCompatibilityInfoFromIntegrator(info), nil
}

func (s *IntegrationService) RestartSteamClient() error {
	return integrator.RestartSteamClient(s.ctx)
}

func (s *IntegrationService) OpenGameSteamProtonPrefix(gameID string) (string, error) {
	game, err := s.getGame(gameID)
	if err != nil {
		return "", err
	}
	info, err := integrator.GetSteamCompatibilityInfo(s.ctx, game)
	if err != nil {
		return "", err
	}
	if !info.Supported {
		return "", fmt.Errorf("Steam Proton Prefix 目录仅支持 Linux")
	}
	if !info.SteamInstalled {
		return "", fmt.Errorf("未检测到 Linux Steam 客户端")
	}
	if strings.TrimSpace(info.AppID) == "" {
		return "", fmt.Errorf("该游戏尚未关联 Steam")
	}

	prefix := strings.TrimSpace(info.ProtonPrefix)
	if prefix == "" {
		return "", fmt.Errorf("未找到该游戏的 Proton Prefix 目录，请先通过 Steam 启动一次游戏")
	}
	if err := apputils.OpenDirectory(prefix); err != nil {
		return "", fmt.Errorf("打开 Proton Prefix 目录失败: %w", err)
	}
	return prefix, nil
}

func (s *IntegrationService) getGame(gameID string) (models.Game, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return models.Game{}, fmt.Errorf("game ID is required")
	}
	if s.gameService == nil {
		return models.Game{}, fmt.Errorf("game service is not initialized")
	}
	return s.gameService.GetGameByID(gameID)
}

func (s *IntegrationService) getGames(gameIDs []string) (map[string]models.Game, error) {
	if s.db == nil {
		return nil, fmt.Errorf("Steam integration service is not initialized")
	}
	placeholders := utils.BuildPlaceholders(len(gameIDs))
	args := make([]interface{}, 0, len(gameIDs))
	for _, gameID := range gameIDs {
		args = append(args, gameID)
	}
	rows, err := s.db.QueryContext(s.ctx, fmt.Sprintf(`
		SELECT
			id,
			COALESCE(name, ''),
			COALESCE(path, ''),
			COALESCE(game_directory, ''),
			COALESCE(steam_launch_id, ''),
			COALESCE(steam_launch_kind, ''),
			COALESCE(steam_user_id, ''),
			COALESCE(steam_launch_options, '')
		FROM games
		WHERE id IN (%s)
	`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("load games for Steam batch import: %w", err)
	}
	defer rows.Close()

	games := make(map[string]models.Game, len(gameIDs))
	for rows.Next() {
		var game models.Game
		if err := rows.Scan(
			&game.ID,
			&game.Name,
			&game.Path,
			&game.GameDirectory,
			&game.SteamLaunchID,
			&game.SteamLaunchKind,
			&game.SteamUserID,
			&game.SteamLaunchOptions,
		); err != nil {
			return nil, fmt.Errorf("scan game for Steam batch import: %w", err)
		}
		games[game.ID] = game
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate games for Steam batch import: %w", err)
	}
	return games, nil
}

func (s *IntegrationService) persistSteamIdentity(gameID string, status SteamLaunchStatus) error {
	if s.db == nil {
		return fmt.Errorf("Steam integration service is not initialized")
	}
	_, err := s.db.ExecContext(s.ctx, `
		UPDATE games
		SET steam_launch_id = ?,
		    steam_launch_kind = ?,
		    steam_user_id = ?,
		    wine_prefix = CASE
		        WHEN ? <> '' AND COALESCE(wine_prefix, '') = '' THEN ?
		        ELSE wine_prefix
		    END
		WHERE id = ?
	`, status.LaunchID, status.LaunchKind, status.UserID, status.ProtonPrefix, status.ProtonPrefix, gameID)
	if err != nil {
		return fmt.Errorf("save Steam launch identity: %w", err)
	}
	return nil
}

func (s *IntegrationService) persistSteamLaunchOptions(gameID string, launchOptions string) error {
	if s.db == nil {
		return fmt.Errorf("Steam integration service is not initialized")
	}
	_, err := s.db.ExecContext(s.ctx, `
		UPDATE games
		SET steam_launch_options = ?
		WHERE id = ?
	`, normalizeSteamLaunchOptions(launchOptions), gameID)
	if err != nil {
		return fmt.Errorf("save Steam launch options: %w", err)
	}
	return nil
}

func normalizeSteamLaunchOptions(launchOptions string) string {
	return strings.TrimSpace(strings.ReplaceAll(launchOptions, "\x00", ""))
}

func (s *IntegrationService) persistSteamIdentities(items []SteamBatchImportItemResult) error {
	if len(items) == 0 {
		return nil
	}
	if s.db == nil {
		return fmt.Errorf("Steam integration service is not initialized")
	}

	valueRows := make([]string, 0, len(items))
	args := make([]interface{}, 0, len(items)*5)
	for _, item := range items {
		valueRows = append(valueRows, "(?, ?, ?, ?, ?)")
		args = append(
			args,
			item.GameID,
			item.Status.LaunchID,
			item.Status.LaunchKind,
			item.Status.UserID,
			item.Status.ProtonPrefix,
		)
	}
	_, err := s.db.ExecContext(s.ctx, fmt.Sprintf(`
		UPDATE games AS game
		SET steam_launch_id = identity.launch_id,
		    steam_launch_kind = identity.launch_kind,
		    steam_user_id = identity.user_id,
		    wine_prefix = CASE
		        WHEN identity.proton_prefix <> '' AND COALESCE(game.wine_prefix, '') = '' THEN identity.proton_prefix
		        ELSE game.wine_prefix
		    END,
		    launch_mode = 'steam'
		FROM (VALUES %s) AS identity(id, launch_id, launch_kind, user_id, proton_prefix)
		WHERE game.id = identity.id
	`, strings.Join(valueRows, ", ")), args...)
	if err != nil {
		return fmt.Errorf("save Steam launch identities: %w", err)
	}
	return nil
}

func normalizeSteamBatchGameIDs(gameIDs []string) []string {
	normalized := make([]string, 0, len(gameIDs))
	for _, gameID := range gameIDs {
		normalized = append(normalized, strings.TrimSpace(gameID))
	}
	return utils.UniqueNonEmptyStrings(normalized)
}

func steamLaunchStatusFromIntegrator(status integrator.SteamLaunchStatus) SteamLaunchStatus {
	return SteamLaunchStatus{
		State:          status.State,
		Ready:          status.Ready,
		SteamInstalled: status.SteamInstalled,
		SteamRunning:   status.SteamRunning,
		LaunchID:       status.LaunchID,
		LaunchKind:     status.LaunchKind,
		UserID:         status.UserID,
		ProtonPrefix:   status.ProtonPrefix,
	}
}

func steamCompatibilityInfoFromIntegrator(info integrator.SteamCompatibilityInfo) SteamCompatibilityInfo {
	tools := make([]SteamCompatibilityTool, 0, len(info.Tools))
	for _, tool := range info.Tools {
		tools = append(tools, SteamCompatibilityTool{
			Name:        tool.Name,
			DisplayName: tool.DisplayName,
			Path:        tool.Path,
			BuiltIn:     tool.BuiltIn,
		})
	}
	return SteamCompatibilityInfo{
		Supported:      info.Supported,
		SteamInstalled: info.SteamInstalled,
		SteamRoot:      info.SteamRoot,
		AppID:          info.AppID,
		ProtonPrefix:   info.ProtonPrefix,
		CurrentTool:    info.CurrentTool,
		DefaultTool:    info.DefaultTool,
		Tools:          tools,
	}
}
