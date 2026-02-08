package cli

import (
	"fmt"
	"lunabox/internal/applog"

	"github.com/spf13/cobra"
)

func newListCmd(app *CoreApp) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all games in your library",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
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
		},
	}
}
