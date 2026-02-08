package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/service"
)

// CoreApp CLI 模式的核心应用 (也可用于 GUI 传递 Context)
type CoreApp struct {
	Config         *appconf.AppConfig
	DB             *sql.DB
	Ctx            context.Context // Export Ctx
	GameService    *service.GameService
	StartService   *service.StartService
	SessionService *service.SessionService
	BackupService  *service.BackupService
}

// RunCommand 执行 CLI 命令
// w:输出目标 (os.Stdout 或 http.ResponseWriter)
// app: 已初始化的 CoreApp
// args: 命令行参数 (不包含程序名)
func RunCommand(w io.Writer, app *CoreApp, args []string) error {
	if len(args) == 0 {
		printUsage(w)
		return nil
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "start":
		return runStartCommand(w, app, cmdArgs)
	case "list":
		return runListCommand(w, app, cmdArgs)
	case "help", "--help", "-h":
		printUsage(w)
		return nil
	default:
		fmt.Fprintf(w, "Unknown command: %s\n\n", command)
		printUsage(w)
		return fmt.Errorf("unknown command: %s", command)
	}
}

// printUsage 打印使用帮助
func printUsage(w io.Writer) {
	usage :=
		`LunaBox - Gal Game Manager

Usage: lunacli <command> [options]

Commands:
  start <game>     Start a game by ID, alias or name (fuzzy match)
  list             List all games in your library
  help             Show this help message

Examples:
  lunacli start my-gal          # Start game by ID or name
  lunacli start "Wonderful Everyday"      # Start by full name (with spaces)
  lunacli list                  # List all games
`
	fmt.Fprint(w, usage)
}

// runStartCommand 执行 start 命令
func runStartCommand(w io.Writer, app *CoreApp, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(w, "Error: game ID or name required")
		fmt.Fprintln(w, "Usage: lunacli start <game>")
		return fmt.Errorf("game ID or name required")
	}

	gameQuery := args[0]

	// 解析游戏 ID
	applog.LogInfof(app.Ctx, "Looking for game: %s", gameQuery)
	gameID, gameName, err := resolveGame(w, app, gameQuery)
	if err != nil {
		applog.LogFatalf(app.Ctx, "Failed to find game: %v", err)
		return err
	}

	applog.LogInfof(app.Ctx, "Found game: %s (ID: %s)", gameName, gameID)
	applog.LogInfof(app.Ctx, "Starting game...")

	// 启动游戏
	success, err := app.StartService.StartGameWithTracking(gameID)
	if err != nil {
		applog.LogFatalf(app.Ctx, "Failed to start game: %v", err)
		return err
	}

	if !success {
		applog.LogFatalf(app.Ctx, "Game failed to start")
		return fmt.Errorf("game failed to start")
	}

	fmt.Fprintln(w, "Game started successfully!")
	// 注意：这里不监控进程，因为在 IPC 模式下这会阻塞 response
	// TODO: 考虑是否需要在本地模式下监控

	return nil
}

// runListCommand 执行 list 命令
func runListCommand(w io.Writer, app *CoreApp, args []string) error {
	fmt.Fprintln(w, "Starting list command...")

	applog.LogInfof(app.Ctx, "Getting games from database...")
	// 获取所有游戏
	games, err := app.GameService.GetGames()
	if err != nil {
		applog.LogFatalf(app.Ctx, "Failed to get games: %v", err)
		return err
	}

	applog.LogInfof(app.Ctx, "Retrieved %d games", len(games))

	if len(games) == 0 {
		fmt.Fprintln(w, "No games in your library.")
		fmt.Fprintln(w, "Add games using the GUI application first.")
		return nil
	}

	// 打印游戏列表
	fmt.Fprintf(w, "\nYour Game Library (%d games):\n\n", len(games))
	fmt.Fprintln(w, "┌────────────────────────────────────────────────────────────────────┐")
	fmt.Fprintf(w, "│ %-12s │ %-53s │\n", "Short ID", "Name")
	fmt.Fprintln(w, "├────────────────────────────────────────────────────────────────────┤")

	for _, game := range games {
		// 只显示ID的前8位
		shortID := game.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// 截断过长的名称
		name := game.Name
		if len(name) > 51 {
			name = name[:48] + "..."
		}

		// 显示状态图标
		statusIcon := "○"
		switch game.Status {
		case "playing":
			statusIcon = "▶"
		case "completed":
			statusIcon = "✓"
		case "on_hold":
			statusIcon = "⏸"
		case "dropped":
			statusIcon = "✗"
		}

		fmt.Fprintf(w, "│ %-12s │ %s %-51s │\n", shortID, statusIcon, name)
	}

	fmt.Fprintln(w, "└────────────────────────────────────────────────────────────────────┘")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Status Icons: ○ Not Started  ▶ Playing  ✓ Completed  ⏸ On Hold  ✗ Dropped")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Use 'lunacli start <game-id>' to start a game\n\n")
	return nil
}

// resolveGame 解析游戏查询（ID / ID前缀 / 别名 / 名称模糊匹配）
func resolveGame(w io.Writer, app *CoreApp, query string) (gameID string, gameName string, err error) {
	// 1. 先尝试作为 ID 精确查找
	game, err := app.GameService.GetGameByID(query)
	if err == nil {
		return game.ID, game.Name, nil
	}

	// 2. 获取所有游戏用于后续匹配
	games, err := app.GameService.GetGames()
	if err != nil {
		return "", "", fmt.Errorf("failed to get games: %w", err)
	}

	queryLower := strings.ToLower(query)

	// 3. 尝试作为 ID 前缀匹配（支持短ID）
	var idPrefixMatches []struct {
		ID   string
		Name string
	}
	for _, g := range games {
		if strings.HasPrefix(strings.ToLower(g.ID), queryLower) {
			idPrefixMatches = append(idPrefixMatches, struct {
				ID   string
				Name string
			}{g.ID, g.Name})
		}
	}

	// 如果ID前缀只有一个匹配，直接使用
	if len(idPrefixMatches) == 1 {
		return idPrefixMatches[0].ID, idPrefixMatches[0].Name, nil
	}

	// 如果ID前缀有多个匹配，提示用户
	if len(idPrefixMatches) > 1 {
		fmt.Fprintf(w, "\nMultiple games found with ID prefix '%s':\n\n", query)
		for i, match := range idPrefixMatches {
			shortID := match.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			fmt.Fprintf(w, "  %d. %s (ID: %s)\n", i+1, match.Name, shortID)
		}
		fmt.Fprintln(w)
		return "", "", fmt.Errorf("please use a longer ID prefix to match exactly one game")
	}

	// 4. 作为名称精确匹配（不区分大小写）
	for _, g := range games {
		if strings.ToLower(g.Name) == queryLower {
			return g.ID, g.Name, nil
		}
	}

	// 5. 模糊匹配（包含查询字符串）
	var matches []struct {
		ID   string
		Name string
	}

	for _, g := range games {
		if strings.Contains(strings.ToLower(g.Name), queryLower) {
			matches = append(matches, struct {
				ID   string
				Name string
			}{g.ID, g.Name})
		}
	}

	if len(matches) == 0 {
		return "", "", fmt.Errorf("no game found matching: %s", query)
	}

	if len(matches) == 1 {
		return matches[0].ID, matches[0].Name, nil
	}

	// 多个匹配结果，提示用户
	fmt.Fprintf(w, "\nMultiple games found matching '%s':\n\n", query)
	for i, match := range matches {
		shortID := match.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Fprintf(w, "  %d. %s (ID: %s)\n", i+1, match.Name, shortID)
	}
	fmt.Fprintln(w)
	return "", "", fmt.Errorf("please use the exact game ID or refine your search")
}
