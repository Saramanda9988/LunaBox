package importer

import (
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/metadata"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const steamFullyInstalledFlag = 4

type SteamImporter struct {
	deps Dependencies
}

type SteamLocalGame struct {
	AppID        string   `json:"app_id"`
	Name         string   `json:"name"`
	InstallDir   string   `json:"install_dir"`
	LibraryPath  string   `json:"library_path"`
	ManifestPath string   `json:"manifest_path"`
	SizeOnDisk   int64    `json:"size_on_disk"`
	StateFlags   int      `json:"state_flags"`
	Executables  []string `json:"executables"`
	SelectedExe  string   `json:"selected_exe"`
	ProtonPrefix string   `json:"proton_prefix"`
}

type vdfNode struct {
	Value    string
	Children map[string]*vdfNode
}

func NewSteamImporter(deps Dependencies) *SteamImporter {
	return &SteamImporter{deps: deps}
}

func (s *SteamImporter) Preview() ([]PreviewGame, error) {
	games, err := s.ScanLocalGames()
	if err != nil {
		return nil, err
	}

	existingGames, _, _, err := s.deps.existingGames("PreviewSteamLocalImport")
	if err != nil {
		return nil, err
	}
	existingIndex := newExistingPreviewIndex(existingGames)

	previews := make([]PreviewGame, 0, len(games))
	for _, game := range games {
		conflict := previewConflict(existingIndex, game.Name, game.InstallDir, string(enums.Steam), game.AppID)
		exists := conflict.Type != ConflictTypeNone
		conflictType := conflict.Type
		if shouldUpdateLocalSteamLaunchFields(conflict) {
			exists = false
			conflictType = ConflictTypeNone
		}
		previews = append(previews, PreviewGame{
			Name:         game.Name,
			SourceType:   string(enums.Steam),
			SourceID:     game.AppID,
			Path:         game.InstallDir,
			Exists:       exists,
			ConflictType: conflictType,
			ExistingID:   conflict.Game.ID,
			ExistingName: conflict.Game.Name,
			AddTime:      time.Now(),
			HasPath:      game.InstallDir != "",
		})
	}
	return previews, nil
}

func (s *SteamImporter) Import(skipNoPath bool, samePathAction string, language string, getterOptions ...metadata.GetterOption) (ImportResult, error) {
	return s.ImportSelected(skipNoPath, samePathAction, nil, language, getterOptions...)
}

func (s *SteamImporter) ImportSelected(skipNoPath bool, samePathAction string, selections []vo.ImportSelection, language string, getterOptions ...metadata.GetterOption) (ImportResult, error) {
	result := newImportResult()
	samePathAction = NormalizeSamePathAction(samePathAction)
	selectionFilter := newImportSelectionFilter(selections)

	games, err := s.ScanLocalGames()
	if err != nil {
		return result, err
	}

	existingGames, existingNames, existingPaths, err := s.deps.existingGames("ImportFromSteamLocal")
	if err != nil {
		return result, err
	}

	existingIndex := newExistingPreviewIndex(existingGames)
	getter := metadata.NewSteamInfoGetterWithLanguage(language, getterOptions...)
	items := make([]ImportItem, 0, len(games))
	for _, localGame := range games {
		if !selectionFilter.includes(localGame.Name, localGame.InstallDir, string(enums.Steam), localGame.AppID) {
			continue
		}
		if skipNoPath && strings.TrimSpace(localGame.InstallDir) == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, localGame.Name+" (无路径)")
			continue
		}

		conflict := previewConflict(existingIndex, localGame.Name, localGame.InstallDir, string(enums.Steam), localGame.AppID)
		action := ImportActionCreate
		existingGameID := ""
		updateLocalLaunchFields := false
		if conflict.Type != ConflictTypeNone {
			if shouldUpdateLocalSteamLaunchFields(conflict) {
				action = ImportActionUpdateExisting
				existingGameID = conflict.Game.ID
				updateLocalLaunchFields = true
			} else if conflict.Type != ConflictTypeSamePath || !IsSamePathMergeAction(samePathAction) {
				result.Skipped++
				switch conflict.Type {
				case ConflictTypeSource:
					result.SkippedNames = append(result.SkippedNames, localGame.Name+" (元数据已存在: "+conflict.Game.Name+")")
				case ConflictTypeNameAndPath:
					result.SkippedNames = append(result.SkippedNames, localGame.Name+" (已存在)")
				default:
					result.SkippedNames = append(result.SkippedNames, localGame.Name+" (路径已存在: "+conflict.Game.Name+")")
				}
				continue
			} else {
				action = ImportActionUpdateExisting
				if samePathAction == SamePathActionMergeSessions {
					action = ImportActionMergeSessions
				}
				existingGameID = conflict.Game.ID
			}
		}

		game, tags := s.fetchSteamGameMetadata(getter, localGame)
		if TargetsExistingGame(action) {
			game.ID = existingGameID
		}

		source := vo.GameMetadataFromWebVO{
			Source: enums.Steam,
			Game:   game,
			Tags:   tags,
		}
		items = append(items, ImportItem{
			Source:                  source,
			DisplayName:             localGame.Name,
			Path:                    localGame.InstallDir,
			Action:                  action,
			ExistingGameID:          existingGameID,
			UpdateLocalLaunchFields: updateLocalLaunchFields,
		})
		if action == ImportActionCreate {
			updateExistingIndexes(existingNames, existingPaths, game, game.Name, localGame.InstallDir)
			existingIndex.byPath[normalizeImportPath(localGame.InstallDir)] = game
			existingIndex.bySource[previewSourceKey(string(enums.Steam), localGame.AppID)] = game
			existingIndex.byNamePath[previewNamePathKey(game.Name, localGame.InstallDir)] = game
		}
	}

	batchResult, err := addImportedItems(s.deps, items)
	if err != nil {
		return result, err
	}
	result.Success += batchResult.Success
	result.Skipped += batchResult.Skipped
	result.Failed += batchResult.Failed
	result.SessionsImported += batchResult.SessionsImported
	result.SkippedNames = append(result.SkippedNames, batchResult.SkippedNames...)
	result.FailedNames = append(result.FailedNames, batchResult.FailedNames...)
	return result, nil
}

func (s *SteamImporter) ScanLocalGames() ([]SteamLocalGame, error) {
	steamPath, err := findSteamInstallPath()
	if err != nil {
		return nil, err
	}

	libraryPaths, err := loadSteamLibraryPaths(steamPath)
	if err != nil {
		return nil, err
	}

	games := make([]SteamLocalGame, 0)
	seenAppIDs := make(map[string]bool)
	for _, libraryPath := range libraryPaths {
		manifestPaths, err := filepath.Glob(filepath.Join(libraryPath, "steamapps", "appmanifest_*.acf"))
		if err != nil {
			applog.LogWarningf(s.deps.Ctx, "Steam scan: failed to glob manifests in %s: %v", libraryPath, err)
			continue
		}
		sort.Strings(manifestPaths)
		for _, manifestPath := range manifestPaths {
			game, err := readSteamManifest(libraryPath, manifestPath)
			if err != nil {
				applog.LogWarningf(s.deps.Ctx, "Steam scan: failed to read manifest %s: %v", manifestPath, err)
				continue
			}
			if game.AppID == "" || seenAppIDs[game.AppID] || !isImportableSteamGame(game) {
				continue
			}
			seenAppIDs[game.AppID] = true
			games = append(games, game)
		}
	}

	sort.Slice(games, func(i, j int) bool {
		return strings.ToLower(games[i].Name) < strings.ToLower(games[j].Name)
	})
	return games, nil
}

func (s *SteamImporter) fetchSteamGameMetadata(getter *metadata.SteamInfoGetter, localGame SteamLocalGame) (models.Game, []metadata.TagItem) {
	gameID := uuid.New().String()
	now := time.Now()
	game := models.Game{
		ID:              gameID,
		Name:            localGame.Name,
		Path:            localGame.InstallDir,
		GameDirectory:   localGame.InstallDir,
		LaunchMode:      enums.LaunchModeSteam,
		SteamLaunchID:   localGame.AppID,
		SteamLaunchKind: "native",
		WinePrefix:      localGame.ProtonPrefix,
		SourceType:      enums.Steam,
		SourceID:        localGame.AppID,
		CreatedAt:       now,
		CachedAt:        now,
		UpdatedAt:       now,
	}

	metaResult, err := getter.FetchMetadata(localGame.AppID, "")
	if err != nil {
		applog.LogWarningf(s.deps.Ctx, "ImportFromSteamLocal: failed to fetch Steam metadata for %s/%s: %v", localGame.AppID, localGame.Name, err)
		return game, nil
	}

	if metaResult.Game.Name != "" {
		game = metaResult.Game
		game.ID = gameID
		game.Path = localGame.InstallDir
		game.GameDirectory = localGame.InstallDir
		game.LaunchMode = enums.LaunchModeSteam
		game.SteamLaunchID = localGame.AppID
		game.SteamLaunchKind = "native"
		game.WinePrefix = localGame.ProtonPrefix
		game.SourceType = enums.Steam
		game.SourceID = localGame.AppID
		game.CreatedAt = now
		game.CachedAt = now
		game.UpdatedAt = now
	}
	return game, metaResult.Tags
}

func loadSteamLibraryPaths(steamPath string) ([]string, error) {
	libraryPaths := []string{steamPath}
	libraryFile := filepath.Join(steamPath, "steamapps", "libraryfolders.vdf")
	root, err := parseVDFFile(libraryFile)
	if err != nil {
		return uniqueExistingSteamLibraries(libraryPaths), nil
	}

	libraryFolders := child(root, "libraryfolders")
	if libraryFolders == nil {
		libraryFolders = root
	}
	for _, node := range libraryFolders.Children {
		if pathValue := strings.TrimSpace(node.Value); pathValue != "" {
			libraryPaths = append(libraryPaths, normalizeSteamPath(pathValue))
			continue
		}
		if pathNode := child(node, "path"); pathNode != nil && strings.TrimSpace(pathNode.Value) != "" {
			libraryPaths = append(libraryPaths, normalizeSteamPath(pathNode.Value))
		}
	}
	return uniqueExistingSteamLibraries(libraryPaths), nil
}

func uniqueExistingSteamLibraries(paths []string) []string {
	result := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		cleaned, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
		if err != nil || cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if seen[key] {
			continue
		}
		if info, err := os.Stat(filepath.Join(cleaned, "steamapps")); err == nil && info.IsDir() {
			seen[key] = true
			result = append(result, cleaned)
		}
	}
	return result
}

func readSteamManifest(libraryPath string, manifestPath string) (SteamLocalGame, error) {
	root, err := parseVDFFile(manifestPath)
	if err != nil {
		return SteamLocalGame{}, err
	}
	appState := child(root, "AppState")
	if appState == nil {
		appState = root
	}

	appID := nodeValue(appState, "appid")
	name := strings.TrimSpace(nodeValue(appState, "name"))
	installDirName := strings.TrimSpace(nodeValue(appState, "installdir"))
	stateFlags := parseInt(nodeValue(appState, "StateFlags"))
	sizeOnDisk := parseInt64(nodeValue(appState, "SizeOnDisk"))
	installDir := filepath.Join(libraryPath, "steamapps", "common", installDirName)

	executables := apputils.FindExecutables(installDir, steamExecutableExcludeKeywords())
	selectedExe := installDir
	if len(executables) > 0 {
		selectedExe = apputils.SelectBestExecutable(executables, name)
	}

	return SteamLocalGame{
		AppID:        strings.TrimSpace(appID),
		Name:         name,
		InstallDir:   installDir,
		LibraryPath:  libraryPath,
		ManifestPath: manifestPath,
		SizeOnDisk:   sizeOnDisk,
		StateFlags:   stateFlags,
		Executables:  executables,
		SelectedExe:  selectedExe,
		ProtonPrefix: steamLocalProtonPrefix(libraryPath, appID),
	}, nil
}

func steamLocalProtonPrefix(libraryPath string, appID string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}
	prefix := filepath.Join(libraryPath, "steamapps", "compatdata", appID, "pfx")
	if info, err := os.Stat(prefix); err == nil && info.IsDir() {
		return prefix
	}
	return ""
}

func isImportableSteamGame(game SteamLocalGame) bool {
	if game.StateFlags&steamFullyInstalledFlag == 0 {
		return false
	}
	appID, err := strconv.ParseUint(strings.TrimSpace(game.AppID), 10, 32)
	if err != nil || appID == 0 {
		return false
	}
	if strings.TrimSpace(game.Name) == "" || strings.TrimSpace(game.InstallDir) == "" {
		return false
	}
	if info, err := os.Stat(game.InstallDir); err != nil || !info.IsDir() {
		return false
	}

	name := strings.ToLower(strings.TrimSpace(game.Name))
	if isSteamRuntimeOrCompatibilityTool(game.AppID, name) ||
		strings.Contains(name, "dedicated server") {
		return false
	}
	return true
}

func isSteamRuntimeOrCompatibilityTool(appID string, normalizedName string) bool {
	appID = strings.TrimSpace(appID)
	if appID != "" {
		if _, ok := steamRuntimeAndCompatibilityToolAppIDs[appID]; ok {
			return true
		}
	}

	name := strings.ToLower(strings.TrimSpace(normalizedName))
	if name == "" {
		return false
	}
	return strings.Contains(name, "steamworks common redistributables") ||
		strings.Contains(name, "redistributable") ||
		strings.HasPrefix(name, "steam linux runtime") ||
		isProtonCompatibilityToolName(name)
}

func isProtonCompatibilityToolName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "proton" {
		return true
	}
	for _, prefix := range []string{
		"proton experimental",
		"proton hotfix",
		"proton battleye runtime",
		"proton easyanticheat runtime",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	version, ok := strings.CutPrefix(name, "proton ")
	if !ok || version == "" {
		return false
	}
	return version[0] >= '0' && version[0] <= '9'
}

var steamRuntimeAndCompatibilityToolAppIDs = map[string]struct{}{
	"228980":  {}, // Steamworks Common Redistributables
	"858280":  {}, // Proton 3.7
	"961940":  {}, // Proton 3.16
	"1054830": {}, // Proton 4.2
	"1070560": {}, // Steam Linux Runtime
	"1113280": {}, // Proton 4.11
	"1161040": {}, // Proton BattlEye Runtime
	"1245040": {}, // Proton 5.0
	"1391110": {}, // Steam Linux Runtime 2.0 (soldier)
	"1420170": {}, // Proton 5.13
	"1493710": {}, // Proton Experimental
	"1580130": {}, // Proton 6.3
	"1628350": {}, // Steam Linux Runtime 3.0 (sniper)
	"1826330": {}, // Proton EasyAntiCheat Runtime
	"1887720": {}, // Proton 7.0
	"2180100": {}, // Proton Hotfix
	"2348590": {}, // Proton 8.0
	"2805730": {}, // Proton 9.0
}

func steamExecutableExcludeKeywords() []string {
	return []string{
		"unins", "setup", "config", "patch", "update", "crashpad",
		"vc_redist", "dxwebsetup", "directx", "vcredist", "dotnet",
		"redistributable", "installer", "launcher_helper", "crashreporter",
		"updater", "uninstall", "删除", "卸载",
	}
}

func parseVDFFile(path string) (*vdfNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read VDF file: %w", err)
	}
	tokens, err := tokenizeVDF(string(data))
	if err != nil {
		return nil, err
	}
	root := &vdfNode{Children: make(map[string]*vdfNode)}
	if err := parseVDFObject(root, tokens, 0); err != nil {
		return nil, err
	}
	return root, nil
}

func parseVDFObject(target *vdfNode, tokens []string, start int) error {
	for i := start; i < len(tokens); {
		token := tokens[i]
		if token == "}" {
			return nil
		}
		if token == "{" {
			return fmt.Errorf("unexpected VDF object start")
		}
		key := token
		i++
		if i >= len(tokens) {
			return fmt.Errorf("missing VDF value for key %s", key)
		}
		if tokens[i] == "{" {
			childNode := &vdfNode{Children: make(map[string]*vdfNode)}
			next, err := parseVDFObjectWithIndex(childNode, tokens, i+1)
			if err != nil {
				return err
			}
			target.Children[key] = childNode
			i = next
			continue
		}
		if tokens[i] == "}" {
			return fmt.Errorf("missing VDF value for key %s", key)
		}
		target.Children[key] = &vdfNode{Value: tokens[i]}
		i++
	}
	return nil
}

func parseVDFObjectWithIndex(target *vdfNode, tokens []string, start int) (int, error) {
	for i := start; i < len(tokens); {
		token := tokens[i]
		if token == "}" {
			return i + 1, nil
		}
		if token == "{" {
			return i, fmt.Errorf("unexpected VDF object start")
		}
		key := token
		i++
		if i >= len(tokens) {
			return i, fmt.Errorf("missing VDF value for key %s", key)
		}
		if tokens[i] == "{" {
			childNode := &vdfNode{Children: make(map[string]*vdfNode)}
			next, err := parseVDFObjectWithIndex(childNode, tokens, i+1)
			if err != nil {
				return next, err
			}
			target.Children[key] = childNode
			i = next
			continue
		}
		if tokens[i] == "}" {
			return i, fmt.Errorf("missing VDF value for key %s", key)
		}
		target.Children[key] = &vdfNode{Value: tokens[i]}
		i++
	}
	return len(tokens), nil
}

func tokenizeVDF(input string) ([]string, error) {
	tokens := make([]string, 0)
	for i := 0; i < len(input); {
		ch := input[i]
		if ch == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}
		if ch == '{' || ch == '}' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		if isVDFWhitespace(ch) {
			i++
			continue
		}
		if ch == '"' {
			value, next, err := readVDFQuoted(input, i+1)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, value)
			i = next
			continue
		}
		start := i
		for i < len(input) && !isVDFWhitespace(input[i]) && input[i] != '{' && input[i] != '}' {
			i++
		}
		tokens = append(tokens, input[start:i])
	}
	return tokens, nil
}

func readVDFQuoted(input string, start int) (string, int, error) {
	var builder strings.Builder
	for i := start; i < len(input); i++ {
		ch := input[i]
		if ch == '"' {
			return builder.String(), i + 1, nil
		}
		if ch == '\\' && i+1 < len(input) {
			next := input[i+1]
			if next == '"' || next == '\\' {
				builder.WriteByte(next)
				i++
				continue
			}
		}
		builder.WriteByte(ch)
	}
	return "", len(input), fmt.Errorf("unterminated VDF quoted string")
}

func isVDFWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

func child(node *vdfNode, key string) *vdfNode {
	if node == nil {
		return nil
	}
	if item, ok := node.Children[key]; ok {
		return item
	}
	lowerKey := strings.ToLower(key)
	for name, item := range node.Children {
		if strings.ToLower(name) == lowerKey {
			return item
		}
	}
	return nil
}

func nodeValue(node *vdfNode, key string) string {
	if item := child(node, key); item != nil {
		return strings.TrimSpace(item.Value)
	}
	return ""
}

func parseInt(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	return value
}

func parseInt64(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value
}

func normalizeSteamPath(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "/", string(os.PathSeparator))
}
