//go:build linux

package integrator

import (
	"context"
	"errors"
	"fmt"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/steamutils"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const steamDefaultCompatibilityTool = "proton_experimental"

var (
	steamCompatibilityMappingEntryPattern = regexp.MustCompile(`(?is)"([0-9]+)"\s*\{(.*?)\}`)
	steamCompatibilityNamePattern         = regexp.MustCompile(`(?i)"name"\s*"([^"]*)"`)
	steamCompatibilityDisplayPattern      = regexp.MustCompile(`(?i)"display_name"\s*"([^"]*)"`)
	steamCompatibilityInternalPattern     = regexp.MustCompile(`(?i)"([^"]+)"\s*//\s*Internal name of this tool`)
	steamOfficialProtonNamePattern        = regexp.MustCompile(`(?i)^Proton\s+[0-9]+(\.[0-9]+)?`)
)

type steamVDFBlock struct {
	NameStart  int
	OpenBrace  int
	InnerStart int
	CloseBrace int
}

func getSteamPlatformCompatibilityInfo(ctx context.Context, game models.Game) (SteamCompatibilityInfo, error) {
	info := SteamCompatibilityInfo{Supported: true}
	steamRoot, err := findSteamRoot()
	if err != nil {
		info.SteamInstalled = false
		return info, nil
	}
	info.SteamInstalled = true
	info.SteamRoot = steamRoot

	appID, err := steamCompatibilityAppID(ctx, game)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	info.AppID = appID
	info.ProtonPrefix = steamCompatibilityProtonPrefix(steamRoot, appID)

	mapping, err := readSteamCompatibilityMapping(steamRoot)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	info.DefaultTool = mapping["0"]
	if info.DefaultTool == "" {
		info.DefaultTool = steamDefaultCompatibilityTool
	}
	info.CurrentTool = mapping[appID]
	info.Tools = steamCompatibilityTools(steamRoot, info.CurrentTool, info.DefaultTool)
	return info, nil
}

func setSteamPlatformCompatibilityTool(ctx context.Context, game models.Game, toolName string) (SteamCompatibilityInfo, error) {
	steamRoot, err := findSteamRoot()
	if err != nil {
		return SteamCompatibilityInfo{}, fmt.Errorf("Steam is not installed: %w", err)
	}
	appID, err := steamCompatibilityAppID(ctx, game)
	if err != nil {
		return SteamCompatibilityInfo{}, err
	}
	if appID == "" {
		return SteamCompatibilityInfo{}, fmt.Errorf("该游戏尚未关联 Steam")
	}
	if err := updateSteamCompatibilityTool(steamRoot, appID, toolName); err != nil {
		return SteamCompatibilityInfo{}, err
	}
	return getSteamPlatformCompatibilityInfo(ctx, game)
}

func steamCompatibilityAppID(ctx context.Context, game models.Game) (string, error) {
	if appID := steamCompatibilityStoredAppID(game); appID != "" {
		return appID, nil
	}

	result, err := resolveSteamPlatformTarget(ctx, game)
	if err != nil {
		return "", err
	}
	if !result.Status.Ready {
		return "", nil
	}
	return steamCompatibilityLaunchAppID(result.Status.LaunchKind, result.Status.LaunchID), nil
}

func steamCompatibilityStoredAppID(game models.Game) string {
	if appID := steamCompatibilityLaunchAppID(game.SteamLaunchKind, game.SteamLaunchID); appID != "" {
		return appID
	}
	if game.SourceType == enums.Steam {
		if appID := steamCompatibilityNativeAppID(game.SourceID); appID != "" {
			return appID
		}
	}
	return ""
}

func steamCompatibilityLaunchAppID(kind string, launchID string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "native":
		return steamCompatibilityNativeAppID(launchID)
	case "shortcut":
		return steamCompatibilityShortcutAppID(launchID)
	default:
		return ""
	}
}

func steamCompatibilityNativeAppID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	appID, err := strconv.ParseUint(value, 10, 32)
	if err != nil || appID == 0 {
		return ""
	}
	return strconv.FormatUint(appID, 10)
}

func steamCompatibilityShortcutAppID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if appID, ok := steamutils.ShortcutAppIDFromLongID(value); ok && appID != 0 {
		return strconv.FormatUint(uint64(appID), 10)
	}
	appID, err := strconv.ParseUint(value, 10, 32)
	if err != nil || appID == 0 {
		return ""
	}
	return strconv.FormatUint(appID, 10)
}

func steamCompatibilityProtonPrefix(steamRoot string, appID string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}

	appIDs := []string{appID}
	if value, err := strconv.ParseUint(appID, 10, 32); err == nil && value != 0 {
		appIDs = append(appIDs, steamutils.ShortcutLongID(uint32(value)))
	}
	return findSteamProtonPrefix(steamRoot, appIDs...)
}

func steamCompatibilityTools(steamRoot string, extraNames ...string) []SteamCompatibilityTool {
	toolsByName := make(map[string]SteamCompatibilityTool)
	addTool := func(tool SteamCompatibilityTool) {
		tool.Name = strings.TrimSpace(tool.Name)
		tool.DisplayName = strings.TrimSpace(tool.DisplayName)
		if tool.Name == "" {
			return
		}
		if tool.DisplayName == "" {
			tool.DisplayName = tool.Name
		}
		if _, exists := toolsByName[tool.Name]; exists {
			return
		}
		toolsByName[tool.Name] = tool
	}

	addTool(SteamCompatibilityTool{
		Name:        steamDefaultCompatibilityTool,
		DisplayName: "Proton Experimental",
		BuiltIn:     true,
	})
	for _, tool := range steamOfficialCompatibilityTools(steamRoot) {
		addTool(tool)
	}
	for _, tool := range steamCustomCompatibilityTools(steamRoot) {
		addTool(tool)
	}
	for _, name := range extraNames {
		name = strings.TrimSpace(name)
		if name != "" {
			addTool(SteamCompatibilityTool{Name: name, DisplayName: name})
		}
	}

	tools := make([]SteamCompatibilityTool, 0, len(toolsByName))
	for _, tool := range toolsByName {
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool {
		left := strings.ToLower(tools[i].DisplayName)
		right := strings.ToLower(tools[j].DisplayName)
		if left == right {
			return tools[i].Name < tools[j].Name
		}
		return left < right
	})
	return tools
}

func steamOfficialCompatibilityTools(steamRoot string) []SteamCompatibilityTool {
	libraries := steamLibraryRoots(steamRoot)
	tools := make([]SteamCompatibilityTool, 0)
	for _, library := range libraries {
		manifests, err := filepath.Glob(filepath.Join(library, "steamapps", "appmanifest_*.acf"))
		if err != nil {
			continue
		}
		sort.Strings(manifests)
		for _, manifest := range manifests {
			data, err := os.ReadFile(manifest)
			if err != nil {
				continue
			}
			appID := steamTextValue(data, "appid")
			name := steamTextValue(data, "name")
			installDir := steamTextValue(data, "installdir")
			if !isSteamOfficialCompatibilityTool(appID, name) {
				continue
			}
			tools = append(tools, SteamCompatibilityTool{
				Name:        steamOfficialCompatibilityToolName(appID, name),
				DisplayName: name,
				Path:        filepath.Join(library, "steamapps", "common", installDir),
				BuiltIn:     true,
			})
		}
	}
	return tools
}

func isSteamOfficialCompatibilityTool(appID string, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	return steamOfficialProtonNamePattern.MatchString(name) ||
		appID == "1493710" ||
		appID == "2180100"
}

func steamOfficialCompatibilityToolName(appID string, displayName string) string {
	switch strings.TrimSpace(appID) {
	case "1493710":
		return "proton_experimental"
	case "2180100":
		return "proton_hotfix"
	}
	name := strings.ToLower(strings.TrimSpace(displayName))
	if dot := strings.Index(name, "."); dot >= 0 {
		name = name[:dot]
	}
	name = strings.Join(strings.Fields(name), "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return strings.TrimSpace(displayName)
	}
	return name
}

func steamCustomCompatibilityTools(steamRoot string) []SteamCompatibilityTool {
	root := filepath.Join(steamRoot, "compatibilitytools.d")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	tools := make([]SteamCompatibilityTool, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		tool, ok := readSteamCustomCompatibilityTool(path)
		if ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

func readSteamCustomCompatibilityTool(path string) (SteamCompatibilityTool, bool) {
	data, err := os.ReadFile(filepath.Join(path, "compatibilitytool.vdf"))
	if err != nil {
		return SteamCompatibilityTool{}, false
	}
	name := steamRegexpValue(steamCompatibilityInternalPattern, data)
	if name == "" {
		return SteamCompatibilityTool{}, false
	}
	displayName := steamRegexpValue(steamCompatibilityDisplayPattern, data)
	return SteamCompatibilityTool{
		Name:        name,
		DisplayName: displayName,
		Path:        path,
	}, true
}

func steamRegexpValue(pattern *regexp.Regexp, data []byte) string {
	match := pattern.FindSubmatch(data)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func readSteamCompatibilityMapping(steamRoot string) (map[string]string, error) {
	data, err := os.ReadFile(steamCompatibilityConfigPath(steamRoot))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Steam config: %w", err)
	}
	block, ok := findSteamVDFBlock(string(data), "CompatToolMapping")
	if !ok {
		return map[string]string{}, nil
	}
	return parseSteamCompatibilityMapping(string(data[block.InnerStart:block.CloseBrace])), nil
}

func parseSteamCompatibilityMapping(content string) map[string]string {
	mapping := make(map[string]string)
	for _, match := range steamCompatibilityMappingEntryPattern.FindAllStringSubmatch(content, -1) {
		if len(match) != 3 {
			continue
		}
		appID := strings.TrimSpace(match[1])
		name := steamRegexpValue(steamCompatibilityNamePattern, []byte(match[2]))
		if appID != "" && name != "" {
			mapping[appID] = name
		}
	}
	return mapping
}

func updateSteamCompatibilityTool(steamRoot string, appID string, toolName string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return fmt.Errorf("Steam app ID is required")
	}
	configPath := steamCompatibilityConfigPath(steamRoot)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read Steam config: %w", err)
	}

	content := string(data)
	mappingBlock, hasMapping := findSteamVDFBlock(content, "CompatToolMapping")
	mapping := map[string]string{}
	if hasMapping {
		mapping = parseSteamCompatibilityMapping(content[mappingBlock.InnerStart:mappingBlock.CloseBrace])
	}
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		if !hasMapping {
			return nil
		}
		delete(mapping, appID)
	} else {
		mapping[appID] = toolName
	}

	updated, err := replaceSteamCompatibilityMapping(content, mapping, hasMapping, mappingBlock)
	if err != nil {
		return err
	}
	if updated == content {
		return nil
	}
	return writeSteamCompatibilityConfig(configPath, updated)
}

func replaceSteamCompatibilityMapping(content string, mapping map[string]string, hasMapping bool, mappingBlock steamVDFBlock) (string, error) {
	blockText := formatSteamCompatibilityMapping(mapping)
	if hasMapping {
		return content[:mappingBlock.NameStart] + blockText + content[mappingBlock.CloseBrace+1:], nil
	}
	steamBlock, ok := findSteamVDFBlock(content, "Steam")
	if !ok {
		return "", fmt.Errorf("Steam config does not contain a Steam block")
	}
	return content[:steamBlock.InnerStart] + "\n" + blockText + content[steamBlock.InnerStart:], nil
}

func formatSteamCompatibilityMapping(mapping map[string]string) string {
	ids := make([]string, 0, len(mapping))
	for id, name := range mapping {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, leftErr := strconv.ParseUint(ids[i], 10, 64)
		right, rightErr := strconv.ParseUint(ids[j], 10, 64)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return ids[i] < ids[j]
	})

	var builder strings.Builder
	builder.WriteString("\t\t\t\t\"CompatToolMapping\"\n")
	builder.WriteString("\t\t\t\t{\n")
	for _, id := range ids {
		priority := "250"
		if id == "0" {
			priority = "75"
		}
		builder.WriteString("\t\t\t\t\t\"")
		builder.WriteString(escapeSteamVDFString(id))
		builder.WriteString("\"\n")
		builder.WriteString("\t\t\t\t\t{\n")
		builder.WriteString("\t\t\t\t\t\t\"name\"\t\t\"")
		builder.WriteString(escapeSteamVDFString(mapping[id]))
		builder.WriteString("\"\n")
		builder.WriteString("\t\t\t\t\t\t\"config\"\t\t\"\"\n")
		builder.WriteString("\t\t\t\t\t\t\"priority\"\t\t\"")
		builder.WriteString(priority)
		builder.WriteString("\"\n")
		builder.WriteString("\t\t\t\t\t}\n")
	}
	builder.WriteString("\t\t\t\t}")
	return builder.String()
}

func findSteamVDFBlock(content string, name string) (steamVDFBlock, bool) {
	needle := "\"" + name + "\""
	start := 0
	for {
		index := strings.Index(content[start:], needle)
		if index < 0 {
			return steamVDFBlock{}, false
		}
		nameStart := start + index
		openBrace := nextSteamVDFOpenBrace(content, nameStart+len(needle))
		if openBrace >= 0 {
			closeBrace := matchingSteamVDFCloseBrace(content, openBrace)
			if closeBrace >= 0 {
				return steamVDFBlock{
					NameStart:  nameStart,
					OpenBrace:  openBrace,
					InnerStart: openBrace + 1,
					CloseBrace: closeBrace,
				}, true
			}
		}
		start = nameStart + len(needle)
	}
}

func nextSteamVDFOpenBrace(content string, offset int) int {
	for index := offset; index < len(content); index++ {
		switch content[index] {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			return index
		default:
			return -1
		}
	}
	return -1
}

func matchingSteamVDFCloseBrace(content string, openBrace int) int {
	depth := 0
	inQuote := false
	escaped := false
	for index := openBrace; index < len(content); index++ {
		char := content[index]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inQuote = false
			}
			continue
		}
		switch char {
		case '"':
			inQuote = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func writeSteamCompatibilityConfig(path string, content string) error {
	directory := filepath.Dir(path)
	tempFile, err := os.CreateTemp(directory, "config-*.vdf.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Steam config: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temporary Steam config: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("flush temporary Steam config: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary Steam config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace Steam config: %w", err)
	}
	return nil
}

func steamCompatibilityConfigPath(steamRoot string) string {
	return filepath.Join(steamRoot, "config", "config.vdf")
}

func escapeSteamVDFString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
