//go:build windows

package integrator

import (
	"context"
	"errors"
	"fmt"
	"lunabox/internal/models"
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/steamutils"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	steamLibraryPathPattern = regexp.MustCompile(`(?i)"path"\s*"([^"]+)"`)
	steamLoginUserPattern   = regexp.MustCompile(`(?is)"([0-9]{15,20})"\s*\{(.*?)\}`)
)

type steamNativeGame struct {
	AppID      string
	InstallDir string
}

type steamLoginUser struct {
	UserID      string
	AccountName string
	MostRecent  bool
	Timestamp   uint64
}

func resolveSteamPlatformTarget(_ context.Context, game models.Game) (SteamResult, error) {
	steamRoot, err := findSteamRoot()
	if err != nil {
		return SteamResult{
			Status: SteamLaunchStatus{State: SteamLaunchStateSteamNotInstalled},
		}, nil
	}

	steamRunning := isSteamRunning()
	baseStatus := SteamLaunchStatus{
		SteamInstalled: true,
		SteamRunning:   steamRunning,
	}
	nativeGame, found, err := findNativeSteamGame(steamRoot, game)
	if err != nil {
		return SteamResult{}, err
	}
	if found {
		baseStatus.State = SteamLaunchStateReady
		baseStatus.Ready = true
		baseStatus.LaunchID = nativeGame.AppID
		baseStatus.LaunchKind = "native"
		return SteamResult{Status: baseStatus}, nil
	}

	executable, ok := steamImportExecutable(game.Path)
	if !ok {
		baseStatus.State = SteamLaunchStateExecutableRequired
		return SteamResult{Status: baseStatus}, nil
	}

	userID, err := activeSteamUserID(steamRoot)
	if err != nil {
		baseStatus.State = SteamLaunchStateUserUnavailable
		return SteamResult{Status: baseStatus}, nil
	}
	baseStatus.UserID = userID

	shortcutsPath := steamShortcutsPath(steamRoot, userID)
	entries, err := loadSteamShortcuts(shortcutsPath)
	if err != nil {
		return SteamResult{}, err
	}
	storedLaunchID := ""
	if game.SteamLaunchKind == "shortcut" &&
		(strings.TrimSpace(game.SteamUserID) == "" || game.SteamUserID == userID) {
		storedLaunchID = game.SteamLaunchID
	}
	if appID, found := entries.Find(executable, storedLaunchID); found {
		baseStatus.State = SteamLaunchStateReady
		baseStatus.Ready = true
		baseStatus.LaunchID = steamutils.ShortcutLongID(appID)
		baseStatus.LaunchKind = "shortcut"
		return SteamResult{Status: baseStatus}, nil
	}

	if steamRunning {
		baseStatus.State = SteamLaunchStateSteamRunning
	} else {
		baseStatus.State = SteamLaunchStateNeedsImport
	}
	return SteamResult{Status: baseStatus}, nil
}

func importSteamPlatformShortcut(ctx context.Context, game models.Game) (SteamResult, error) {
	resolved, err := resolveSteamPlatformTarget(ctx, game)
	if err != nil {
		return resolved, err
	}
	if resolved.Status.Ready {
		if !resolved.Status.SteamRunning && resolved.Status.LaunchKind == "shortcut" {
			if appID, ok := steamutils.ShortcutAppIDFromLongID(resolved.Status.LaunchID); ok {
				if steamRoot, rootErr := findSteamRoot(); rootErr == nil {
					_ = importSteamShortcutArtwork(steamRoot, resolved.Status.UserID, appID, game)
				}
			}
		}
		return resolved, nil
	}
	if resolved.Status.State != SteamLaunchStateNeedsImport {
		return resolved, nil
	}

	steamRoot, err := findSteamRoot()
	if err != nil {
		return resolved, nil
	}
	executable, ok := steamImportExecutable(game.Path)
	if !ok {
		resolved.Status.State = SteamLaunchStateExecutableRequired
		return resolved, nil
	}
	userID := resolved.Status.UserID
	if userID == "" {
		userID, err = activeSteamUserID(steamRoot)
		if err != nil {
			resolved.Status.State = SteamLaunchStateUserUnavailable
			return resolved, nil
		}
	}

	shortcutsPath := steamShortcutsPath(steamRoot, userID)
	shortcuts, original, hasOriginal, err := readSteamShortcutFile(shortcutsPath)
	if err != nil {
		return SteamResult{}, err
	}
	if appID, found := shortcuts.Find(executable, game.SteamLaunchID); found {
		resolved.Status.State = SteamLaunchStateReady
		resolved.Status.Ready = true
		resolved.Status.LaunchID = steamutils.ShortcutLongID(appID)
		resolved.Status.LaunchKind = "shortcut"
		resolved.Status.UserID = userID
		if isSteamRunning() {
			resolved.Status.State = SteamLaunchStateSteamRunning
			resolved.Status.Ready = false
			resolved.Status.SteamRunning = true
			return resolved, nil
		}
		shortcuts.SetLaunchOptions(executable, resolved.Status.LaunchID, game.SteamLaunchOptions)
		backupPath, err := saveSteamShortcutFile(shortcutsPath, shortcuts, original, hasOriginal)
		if err != nil {
			return SteamResult{}, err
		}
		resolved.BackupPath = backupPath
		_ = importSteamShortcutArtwork(steamRoot, userID, appID, game)
		return resolved, nil
	}

	appID, err := shortcuts.Add(strings.TrimSpace(game.Name), executable, game.SteamLaunchOptions)
	if err != nil {
		return SteamResult{}, fmt.Errorf("append Steam shortcut: %w", err)
	}
	if isSteamRunning() {
		resolved.Status.State = SteamLaunchStateSteamRunning
		resolved.Status.SteamRunning = true
		return resolved, nil
	}

	backupPath, err := saveSteamShortcutFile(
		shortcutsPath,
		shortcuts,
		original,
		hasOriginal,
	)
	if err != nil {
		return SteamResult{}, err
	}

	resolved.Status = SteamLaunchStatus{
		State:          SteamLaunchStateReady,
		Ready:          true,
		SteamInstalled: true,
		SteamRunning:   false,
		LaunchID:       steamutils.ShortcutLongID(appID),
		LaunchKind:     "shortcut",
		UserID:         userID,
	}
	resolved.Imported = true
	resolved.BackupPath = backupPath
	_ = importSteamShortcutArtwork(steamRoot, userID, appID, game)
	return resolved, nil
}

func setSteamPlatformLaunchOptions(ctx context.Context, game models.Game) (SteamResult, error) {
	resolved, err := resolveSteamPlatformTarget(ctx, game)
	if err != nil {
		return SteamResult{}, err
	}
	if resolved.Status.Ready && resolved.Status.LaunchKind == "native" {
		return resolved, fmt.Errorf("原生 Steam 游戏的启动参数暂不支持直接修改，请在 Steam 属性中设置")
	}
	if resolved.Status.State == SteamLaunchStateNeedsImport {
		return importSteamPlatformShortcut(ctx, game)
	}
	if !resolved.Status.Ready || resolved.Status.LaunchKind != "shortcut" {
		return resolved, fmt.Errorf("该游戏尚未关联可修改启动参数的 Steam 快捷方式")
	}
	if isSteamRunning() {
		resolved.Status.State = SteamLaunchStateSteamRunning
		resolved.Status.Ready = false
		resolved.Status.SteamRunning = true
		return resolved, fmt.Errorf("Steam 正在运行，请先完全退出 Steam 后再保存启动参数")
	}

	steamRoot, err := findSteamRoot()
	if err != nil {
		return resolved, nil
	}
	executable, ok := steamImportExecutable(game.Path)
	if !ok {
		resolved.Status.State = SteamLaunchStateExecutableRequired
		return resolved, fmt.Errorf("Steam 启动参数需要可执行文件路径")
	}
	userID := strings.TrimSpace(resolved.Status.UserID)
	if userID == "" {
		userID, err = activeSteamUserID(steamRoot)
		if err != nil {
			resolved.Status.State = SteamLaunchStateUserUnavailable
			return resolved, nil
		}
	}

	shortcutsPath := steamShortcutsPath(steamRoot, userID)
	shortcuts, original, hasOriginal, err := readSteamShortcutFile(shortcutsPath)
	if err != nil {
		return SteamResult{}, err
	}
	appID, updated := shortcuts.SetLaunchOptions(executable, resolved.Status.LaunchID, game.SteamLaunchOptions)
	if !updated {
		return resolved, fmt.Errorf("未找到可更新启动参数的 Steam 快捷方式")
	}
	backupPath, err := saveSteamShortcutFile(shortcutsPath, shortcuts, original, hasOriginal)
	if err != nil {
		return SteamResult{}, err
	}
	resolved.Status.Ready = true
	resolved.Status.State = SteamLaunchStateReady
	resolved.Status.LaunchID = steamutils.ShortcutLongID(appID)
	resolved.Status.LaunchKind = "shortcut"
	resolved.Status.UserID = userID
	resolved.BackupPath = backupPath
	return resolved, nil
}

func importSteamPlatformShortcuts(_ context.Context, games []models.Game) (SteamBatchResult, error) {
	batch := SteamBatchResult{
		Items: make([]SteamBatchItemResult, len(games)),
	}
	if len(games) == 0 {
		return batch, nil
	}
	for index, game := range games {
		batch.Items[index].GameID = game.ID
	}

	steamRoot, err := findSteamRoot()
	if err != nil {
		for index := range batch.Items {
			batch.Items[index].Result.Status.State = SteamLaunchStateSteamNotInstalled
		}
		return batch, nil
	}

	steamRunning := isSteamRunning()
	nativeGames, err := listNativeSteamGames(steamRoot)
	if err != nil {
		return SteamBatchResult{}, err
	}
	type shortcutCandidate struct {
		executable string
		game       models.Game
		itemIndex  int
	}
	candidates := make([]shortcutCandidate, 0, len(games))
	for index, game := range games {
		status := SteamLaunchStatus{
			SteamInstalled: true,
			SteamRunning:   steamRunning,
		}
		if nativeGame, found := findNativeSteamGameInList(nativeGames, game); found {
			status.State = SteamLaunchStateReady
			status.Ready = true
			status.LaunchID = nativeGame.AppID
			status.LaunchKind = "native"
			batch.Items[index].Result.Status = status
			continue
		}

		executable, ok := steamImportExecutable(game.Path)
		if !ok {
			status.State = SteamLaunchStateExecutableRequired
			batch.Items[index].Result.Status = status
			continue
		}
		batch.Items[index].Result.Status = status
		candidates = append(candidates, shortcutCandidate{
			executable: executable,
			game:       game,
			itemIndex:  index,
		})
	}
	if len(candidates) == 0 {
		return batch, nil
	}

	userID, err := activeSteamUserID(steamRoot)
	if err != nil {
		for _, candidate := range candidates {
			status := batch.Items[candidate.itemIndex].Result.Status
			status.State = SteamLaunchStateUserUnavailable
			batch.Items[candidate.itemIndex].Result.Status = status
		}
		return batch, nil
	}

	shortcutsPath := steamShortcutsPath(steamRoot, userID)
	shortcuts, original, hasOriginal, err := readSteamShortcutFile(shortcutsPath)
	if err != nil {
		return SteamBatchResult{}, err
	}
	addedItemIndexes := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		item := &batch.Items[candidate.itemIndex]
		status := item.Result.Status
		status.UserID = userID

		storedLaunchID := ""
		if candidate.game.SteamLaunchKind == "shortcut" &&
			(strings.TrimSpace(candidate.game.SteamUserID) == "" ||
				candidate.game.SteamUserID == userID) {
			storedLaunchID = candidate.game.SteamLaunchID
		}
		if appID, found := shortcuts.Find(candidate.executable, storedLaunchID); found {
			status.State = SteamLaunchStateReady
			status.Ready = true
			status.LaunchID = steamutils.ShortcutLongID(appID)
			status.LaunchKind = "shortcut"
			item.Result.Status = status
			if !steamRunning {
				_ = importSteamShortcutArtwork(steamRoot, userID, appID, candidate.game)
			}
			continue
		}
		if steamRunning {
			status.State = SteamLaunchStateSteamRunning
			item.Result.Status = status
			continue
		}

		appID, addErr := shortcuts.Add(
			strings.TrimSpace(candidate.game.Name),
			candidate.executable,
			candidate.game.SteamLaunchOptions,
		)
		if addErr != nil {
			item.Err = fmt.Errorf("append Steam shortcut: %w", addErr)
			continue
		}
		status.State = SteamLaunchStateReady
		status.Ready = true
		status.LaunchID = steamutils.ShortcutLongID(appID)
		status.LaunchKind = "shortcut"
		item.Result.Status = status
		item.Result.Imported = true
		addedItemIndexes = append(addedItemIndexes, candidate.itemIndex)
	}
	if len(addedItemIndexes) == 0 {
		return batch, nil
	}

	if isSteamRunning() {
		for _, itemIndex := range addedItemIndexes {
			item := &batch.Items[itemIndex]
			item.Result.Imported = false
			item.Result.Status.State = SteamLaunchStateSteamRunning
			item.Result.Status.Ready = false
			item.Result.Status.SteamRunning = true
			item.Result.Status.LaunchID = ""
			item.Result.Status.LaunchKind = ""
		}
		return batch, nil
	}

	backupPath, err := saveSteamShortcutFile(
		shortcutsPath,
		shortcuts,
		original,
		hasOriginal,
	)
	if err != nil {
		return SteamBatchResult{}, err
	}
	batch.BackupPath = backupPath
	for _, itemIndex := range addedItemIndexes {
		item := &batch.Items[itemIndex]
		item.Result.BackupPath = backupPath
		if appID, ok := steamutils.ShortcutAppIDFromLongID(item.Result.Status.LaunchID); ok {
			_ = importSteamShortcutArtwork(steamRoot, userID, appID, games[itemIndex])
		}
	}
	return batch, nil
}

func findSteamRoot() (string, error) {
	candidates := make([]string, 0, 4)
	if key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		for _, valueName := range []string{"SteamPath", "InstallPath"} {
			if value, _, valueErr := key.GetStringValue(valueName); valueErr == nil {
				candidates = append(candidates, value)
			}
		}
		key.Close()
	}
	if programFilesX86 := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); programFilesX86 != "" {
		candidates = append(candidates, filepath.Join(programFilesX86, "Steam"))
	}
	if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
		candidates = append(candidates, filepath.Join(programFiles, "Steam"))
	}

	for _, candidate := range candidates {
		candidate = strings.ReplaceAll(strings.TrimSpace(candidate), "/", string(os.PathSeparator))
		if candidate == "" {
			continue
		}
		absolute, err := filepath.Abs(filepath.Clean(candidate))
		if err != nil {
			continue
		}
		if info, statErr := os.Stat(filepath.Join(absolute, "steam.exe")); statErr == nil && !info.IsDir() {
			return absolute, nil
		}
	}
	return "", fmt.Errorf("Steam is not installed")
}

func isSteamRunning() bool {
	_, err := processutils.GetProcessPIDByName("steam.exe")
	return err == nil
}

func steamImportExecutable(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", false
	}
	info, err := os.Stat(absolute)
	if err != nil || info.IsDir() {
		return "", false
	}
	return absolute, true
}

func findNativeSteamGame(steamRoot string, game models.Game) (steamNativeGame, bool, error) {
	nativeGames, err := listNativeSteamGames(steamRoot)
	if err != nil {
		return steamNativeGame{}, false, err
	}
	nativeGame, found := findNativeSteamGameInList(nativeGames, game)
	return nativeGame, found, nil
}

func findNativeSteamGameInList(nativeGames []steamNativeGame, game models.Game) (steamNativeGame, bool) {
	gameDirectories := steamGameDirectories(game)
	for _, nativeGame := range nativeGames {
		for _, gameDirectory := range gameDirectories {
			if steamDirectoryContains(nativeGame.InstallDir, gameDirectory) {
				return nativeGame, true
			}
		}
	}
	return steamNativeGame{}, false
}

func listNativeSteamGames(steamRoot string) ([]steamNativeGame, error) {
	libraries := []string{steamRoot}
	libraryFile, err := os.ReadFile(filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf"))
	if err == nil {
		for _, match := range steamLibraryPathPattern.FindAllStringSubmatch(string(libraryFile), -1) {
			if len(match) == 2 {
				libraries = append(libraries, unescapeSteamTextPath(match[1]))
			}
		}
	}

	seenLibraries := make(map[string]bool)
	nativeGames := make([]steamNativeGame, 0)
	for _, library := range libraries {
		absolute, err := filepath.Abs(filepath.Clean(library))
		if err != nil {
			continue
		}
		key := strings.ToLower(absolute)
		if seenLibraries[key] {
			continue
		}
		seenLibraries[key] = true

		manifests, globErr := filepath.Glob(filepath.Join(absolute, "steamapps", "appmanifest_*.acf"))
		if globErr != nil {
			return nil, fmt.Errorf("list Steam manifests: %w", globErr)
		}
		sort.Strings(manifests)
		for _, manifest := range manifests {
			data, readErr := os.ReadFile(manifest)
			if readErr != nil {
				continue
			}
			appID := steamTextValue(data, "appid")
			installDirName := steamTextValue(data, "installdir")
			if appID == "" || installDirName == "" {
				continue
			}
			nativeGames = append(nativeGames, steamNativeGame{
				AppID:      appID,
				InstallDir: filepath.Join(absolute, "steamapps", "common", installDirName),
			})
		}
	}
	return nativeGames, nil
}

func steamTextValue(data []byte, key string) string {
	pattern := regexp.MustCompile(`(?i)"` + regexp.QuoteMeta(key) + `"\s*"([^"]*)"`)
	match := pattern.FindSubmatch(data)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func unescapeSteamTextPath(value string) string {
	value = strings.ReplaceAll(value, `\\`, `\`)
	return strings.ReplaceAll(value, "/", string(os.PathSeparator))
}

func steamGameDirectories(game models.Game) []string {
	candidates := []string{game.GameDirectory}
	if game.Path != "" {
		if info, err := os.Stat(game.Path); err == nil && info.IsDir() {
			candidates = append(candidates, game.Path)
		} else {
			candidates = append(candidates, filepath.Dir(game.Path))
		}
	}
	result := make([]string, 0, len(candidates))
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		absolute, err := filepath.Abs(filepath.Clean(candidate))
		if err != nil {
			continue
		}
		key := strings.ToLower(absolute)
		if !seen[key] {
			seen[key] = true
			result = append(result, absolute)
		}
	}
	return result
}

func steamDirectoryContains(parent string, child string) bool {
	parentAbsolute, parentErr := filepath.Abs(filepath.Clean(parent))
	childAbsolute, childErr := filepath.Abs(filepath.Clean(child))
	if parentErr != nil || childErr != nil {
		return false
	}
	relative, err := filepath.Rel(parentAbsolute, childAbsolute)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)))
}

func activeSteamUserID(steamRoot string) (string, error) {
	if key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam\ActiveProcess`, registry.QUERY_VALUE); err == nil {
		if value, _, valueErr := key.GetIntegerValue("ActiveUser"); valueErr == nil && value != 0 {
			key.Close()
			return strconv.FormatUint(value, 10), nil
		}
		key.Close()
	}

	autoLoginUser := ""
	if key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		autoLoginUser, _, _ = key.GetStringValue("AutoLoginUser")
		key.Close()
	}

	loginUsers, err := os.ReadFile(filepath.Join(steamRoot, "config", "loginusers.vdf"))
	if err == nil {
		if userID, found := selectSteamLoginUser(parseSteamLoginUsers(loginUsers), autoLoginUser); found {
			return userID, nil
		}
	}

	userIDs := steamUserDataIDs(steamRoot)
	if userID, found := mostRecentlyModifiedSteamUserID(steamRoot, userIDs); found {
		return userID, nil
	}
	if len(userIDs) == 1 {
		return userIDs[0], nil
	}
	return "", fmt.Errorf("active Steam user is unavailable")
}

func parseSteamLoginUsers(data []byte) []steamLoginUser {
	users := make([]steamLoginUser, 0)
	for _, match := range steamLoginUserPattern.FindAllSubmatch(data, -1) {
		if len(match) != 3 {
			continue
		}
		steamID64, err := strconv.ParseUint(string(match[1]), 10, 64)
		if err != nil {
			continue
		}
		userID := uint32(steamID64)
		if userID == 0 {
			continue
		}
		timestamp, _ := strconv.ParseUint(steamTextValue(match[2], "Timestamp"), 10, 64)
		users = append(users, steamLoginUser{
			UserID:      strconv.FormatUint(uint64(userID), 10),
			AccountName: steamTextValue(match[2], "AccountName"),
			MostRecent:  steamTextValue(match[2], "MostRecent") == "1",
			Timestamp:   timestamp,
		})
	}
	return users
}

func selectSteamLoginUser(users []steamLoginUser, autoLoginUser string) (string, bool) {
	if userID, found := newestSteamLoginUser(users, func(user steamLoginUser) bool {
		return user.MostRecent
	}); found {
		return userID, true
	}

	autoLoginUser = strings.TrimSpace(autoLoginUser)
	if autoLoginUser != "" {
		for _, user := range users {
			if strings.EqualFold(user.AccountName, autoLoginUser) {
				return user.UserID, true
			}
		}
	}

	if userID, found := newestSteamLoginUser(users, func(user steamLoginUser) bool {
		return user.Timestamp != 0
	}); found {
		return userID, true
	}
	if len(users) == 1 {
		return users[0].UserID, true
	}
	return "", false
}

func newestSteamLoginUser(users []steamLoginUser, include func(steamLoginUser) bool) (string, bool) {
	var selected steamLoginUser
	found := false
	for _, user := range users {
		if !include(user) || (found && user.Timestamp <= selected.Timestamp) {
			continue
		}
		selected = user
		found = true
	}
	return selected.UserID, found
}

func steamUserDataIDs(steamRoot string) []string {
	userDirectories, _ := os.ReadDir(filepath.Join(steamRoot, "userdata"))
	userIDs := make([]string, 0)
	for _, directory := range userDirectories {
		if !directory.IsDir() {
			continue
		}
		if value, parseErr := strconv.ParseUint(directory.Name(), 10, 32); parseErr == nil && value != 0 {
			userIDs = append(userIDs, directory.Name())
		}
	}
	sort.Strings(userIDs)
	return userIDs
}

func mostRecentlyModifiedSteamUserID(steamRoot string, userIDs []string) (string, bool) {
	selectedUserID := ""
	var selectedTime time.Time
	for _, userID := range userIDs {
		localConfigPath := filepath.Join(steamRoot, "userdata", userID, "config", "localconfig.vdf")
		info, err := os.Stat(localConfigPath)
		if err != nil || info.ModTime().Before(selectedTime) {
			continue
		}
		if info.ModTime().Equal(selectedTime) {
			selectedUserID = ""
			continue
		}
		selectedUserID = userID
		selectedTime = info.ModTime()
	}
	return selectedUserID, selectedUserID != ""
}

func steamShortcutsPath(steamRoot string, userID string) string {
	return filepath.Join(steamRoot, "userdata", userID, "config", "shortcuts.vdf")
}

func loadSteamShortcuts(path string) (*steamutils.ShortcutFile, error) {
	shortcuts, _, _, err := readSteamShortcutFile(path)
	return shortcuts, err
}

func readSteamShortcutFile(path string) (*steamutils.ShortcutFile, []byte, bool, error) {
	original, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return steamutils.NewShortcutFile(), nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("read Steam shortcuts: %w", err)
	}
	shortcuts, err := steamutils.ParseShortcutFile(original)
	if err != nil {
		return nil, nil, false, fmt.Errorf("parse Steam shortcuts: %w", err)
	}
	return shortcuts, original, true, nil
}

func saveSteamShortcutFile(
	path string,
	shortcuts *steamutils.ShortcutFile,
	original []byte,
	hasOriginal bool,
) (string, error) {
	encoded, err := shortcuts.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("encode Steam shortcuts: %w", err)
	}

	configDirectory := filepath.Dir(path)
	if err := os.MkdirAll(configDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create Steam user config directory: %w", err)
	}

	backupPath := ""
	if hasOriginal {
		backupPath = path + ".lunabox.bak"
		if err := os.WriteFile(backupPath, original, 0o644); err != nil {
			return "", fmt.Errorf("back up Steam shortcuts: %w", err)
		}
	}

	tempFile, err := os.CreateTemp(configDirectory, "shortcuts-*.vdf.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary Steam shortcuts file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.Write(encoded); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("write temporary Steam shortcuts file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return "", fmt.Errorf("flush temporary Steam shortcuts file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("close temporary Steam shortcuts file: %w", err)
	}
	if err := replaceSteamShortcutsFile(tempPath, path); err != nil {
		return "", fmt.Errorf("replace Steam shortcuts file: %w", err)
	}
	return backupPath, nil
}

func replaceSteamShortcutsFile(source string, target string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		sourcePtr,
		targetPtr,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
