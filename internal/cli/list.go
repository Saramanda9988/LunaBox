package cli

import (
	"fmt"
	"lunabox/internal/applog"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func newListCmd(app *CoreApp) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all games in your library",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			applog.LogInfof(app.Ctx, "Getting games from database...")
			// 获取所有游戏
			games, err := app.GameService.GetGames()
			if err != nil {
				applog.LogErrorf(app.Ctx, "Failed to get games: %v", err)
				return err
			}

			applog.LogInfof(app.Ctx, "Retrieved %d games", len(games))

			if len(games) == 0 {
				fmt.Fprintln(w, "No games in your library.")
				fmt.Fprintln(w, "Add games using the GUI application first.")
				return nil
			}

			// 打印游戏列表
			// Total width: 1 (│) + 1 (space) + 12 (ID) + 1 (space) + 1 (│) + 1 (space) + 53 (Name) + 1 (space) + 1 (│) = 72 chars
			line := "┌" + strings.Repeat("─", 70) + "┐"
			midLine := "├" + strings.Repeat("─", 70) + "┤"
			bottomLine := "└" + strings.Repeat("─", 70) + "┘"

			fmt.Fprintf(w, "\nYour Game Library (%d games):\n\n", len(games))
			fmt.Fprintln(w, line)
			fmt.Fprintf(w, "│ %-12s │ %-53s │\n", "Short ID", "Name")
			fmt.Fprintln(w, midLine)

			for _, game := range games {
				// 只显示ID的前8位
				shortID := game.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}

				// 显示状态图标
				statusIcon := "·"
				switch game.Status {
				case "playing":
					statusIcon = "▶"
				case "completed":
					statusIcon = "✓"
				case "on_hold":
					statusIcon = "○"
				case "dropped":
					statusIcon = "✗"
				}

				// Calculate available width for name
				// Name column is 53 chars wide total.
				// Content is: statusIcon + " " + name
				// So name available width = 53 - width(statusIcon) - 1
				iconWidth := runewidth.StringWidth(statusIcon)
				nameAvailableWidth := 53 - iconWidth - 1

				// Truncate name if too long
				name := game.Name
				if runewidth.StringWidth(name) > nameAvailableWidth {
					name = runewidth.Truncate(name, nameAvailableWidth-3, "...")
				}

				// Calculate padding
				currentNameWidth := runewidth.StringWidth(name)
				padding := nameAvailableWidth - currentNameWidth
				if padding < 0 {
					padding = 0
				}

				fmt.Fprintf(w, "│ %-12s │ %s %s%s │\n", shortID, statusIcon, name, strings.Repeat(" ", padding))
			}

			fmt.Fprintln(w, bottomLine)
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Status Icons: ○ Not Started  ▶ Playing  ✓ Completed  ⏸ On Hold  ✗ Dropped")
			fmt.Fprintln(w)
			fmt.Fprintf(w, "Use 'lunacli start <game-id> or name' to start a game\n\n")
			return nil
		},
	}
}
