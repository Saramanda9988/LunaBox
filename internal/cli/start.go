package cli

import (
	"fmt"
	"io"
	"lunabox/internal/applog"
	"lunabox/internal/service/launcher"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newStartCmd(app *CoreApp) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <game>",
		Short: "Start a game by ID, alias or name (fuzzy match)",
		Args:  cobra.ExactArgs(1), // Expect exactly one argument
		RunE: func(cmd *cobra.Command, args []string) error {
			gameQuery := args[0]
			w := cmd.OutOrStdout()

			// 获取 flags
			useLE, _ := cmd.Flags().GetBool("le")
			useMagpie, _ := cmd.Flags().GetBool("magpie")
			runAsAdmin, _ := cmd.Flags().GetBool("admin")
			wineRunner, _ := cmd.Flags().GetString("wine-runner")
			wineArgs, _ := cmd.Flags().GetString("wine-args")
			winePrefix, _ := cmd.Flags().GetString("wine-prefix")

			// 解析游戏 ID
			applog.LogInfof(app.Ctx, "Looking for game: %s", gameQuery)
			gameID, gameName, err := resolveGame(w, app, gameQuery)
			if err != nil {
				applog.LogErrorf(app.Ctx, "Failed to find game: %v", err)
				return err
			}

			applog.LogInfof(app.Ctx, "Found game: %s (ID: %s)", gameName, gameID)

			// 构造启动选项
			launchOptions := launcher.LaunchOptions{}
			if cmd.Flags().Changed("le") {
				if useLE && app.Config.LocaleEmulatorPath == "" {
					return fmt.Errorf("Locale Emulator path is not configured")
				}
				launchOptions.UseLocaleEmulator = &useLE
			}
			if cmd.Flags().Changed("magpie") {
				if useMagpie && app.Config.MagpiePath == "" {
					return fmt.Errorf("Magpie path is not configured")
				}
				launchOptions.UseMagpie = &useMagpie
			}
			if cmd.Flags().Changed("admin") {
				launchOptions.RunAsAdmin = &runAsAdmin
			}
			if cmd.Flags().Changed("wine-runner") {
				wineRunner = strings.TrimSpace(wineRunner)
				launchOptions.WineRunner = &wineRunner
			}
			if cmd.Flags().Changed("wine-args") {
				launchOptions.WineArgs = &wineArgs
			}
			if cmd.Flags().Changed("wine-prefix") {
				launchOptions.WinePrefix = &winePrefix
			}

			if goruntime.GOOS == "darwin" {
				game, err := app.GameService.GetGameByID(gameID)
				if err != nil {
					return fmt.Errorf("failed to load game: %w", err)
				}
				ext := strings.ToLower(filepath.Ext(strings.TrimSpace(game.Path)))
				effectiveWineRunner := strings.TrimSpace(game.WineRunner)
				if launchOptions.WineRunner != nil {
					effectiveWineRunner = strings.TrimSpace(*launchOptions.WineRunner)
				}
				if (ext == ".exe" || ext == ".bat") && effectiveWineRunner == "" {
					return fmt.Errorf("this game uses a Windows executable on macOS; set --wine-runner system|crossover|custom or configure Wine in the game launch settings")
				}
			}

			logMsg := "Starting game..."
			if launchOptions.UseLocaleEmulator != nil || launchOptions.UseMagpie != nil || launchOptions.RunAsAdmin != nil {
				logMsg = fmt.Sprintf("Starting game with options (LE: %v, Magpie: %v, Admin: %v)...",
					boolPtrToString(launchOptions.UseLocaleEmulator),
					boolPtrToString(launchOptions.UseMagpie),
					boolPtrToString(launchOptions.RunAsAdmin))
			}
			applog.LogInfof(app.Ctx, "%s", logMsg)

			// 启动游戏
			success, err := app.StartService.StartGameWithOptions(gameID, launchOptions)
			if err != nil {
				applog.LogErrorf(app.Ctx, "Failed to start game: %v", err)
				return err
			}

			if !success {
				applog.LogErrorf(app.Ctx, "Game failed to start")
				return fmt.Errorf("game failed to start")
			}

			fmt.Fprintln(w, "Game started successfully!")
			fmt.Fprintln(w, "Recording play duration...")

			return nil
		},
	}

	cmd.Flags().BoolP("le", "l", false, "Start with Locale Emulator")
	cmd.Flags().BoolP("magpie", "m", false, "Start with Magpie")
	cmd.Flags().BoolP("admin", "a", false, "Start as administrator")
	cmd.Flags().String("wine-runner", "", "Override Wine runner on macOS/Linux: system, crossover, custom")
	cmd.Flags().String("wine-args", "", "Override Wine arguments on macOS/Linux")
	cmd.Flags().String("wine-prefix", "", "Override WINEPREFIX or CrossOver bottle on macOS/Linux")

	return cmd
}

func boolPtrToString(b *bool) string {
	if b == nil {
		return "default"
	}
	return fmt.Sprintf("%v", *b)
}

// resolveGame 解析游戏查询（ID / ID前缀 / 别名 / 名称模糊匹配）
func resolveGame(w io.Writer, app *CoreApp, query string) (gameID string, gameName string, err error) {
	// 1. 先尝试作为 ID 精确查找
	game, err := app.GameService.GetGameByID(query)
	if err == nil {
		return game.ID, game.Name, nil
	}

	// 2. 拉取完整游戏列表，用于短 ID 与别名匹配。
	games, err := app.GameService.ListAllGames()
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

	// 5. 名称或别名模糊匹配（包含查询字符串）
	var matches []struct {
		ID   string
		Name string
	}

	for _, g := range games {
		matched := strings.Contains(strings.ToLower(g.Name), queryLower)
		if !matched {
			for _, alias := range g.Aliases {
				if strings.Contains(strings.ToLower(alias), queryLower) {
					matched = true
					break
				}
			}
		}
		if matched {
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
