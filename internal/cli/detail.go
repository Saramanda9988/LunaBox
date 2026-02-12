package cli

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
)

func newDetailCmd(app *CoreApp) *cobra.Command {
	return &cobra.Command{
		Use:   "detail <game>",
		Short: "Show detailed information for a game",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			gameQuery := args[0]

			gameID, _, err := resolveGame(w, app, gameQuery)
			if err != nil {
				return err
			}

			game, err := app.GameService.GetGameByID(gameID)
			if err != nil {
				return err
			}

			fmt.Fprintln(w)
			fmt.Fprintf(w, "Name: %s\n", game.Name)
			fmt.Fprintf(w, "ID: %s\n", game.ID)
			fmt.Fprintf(w, "Status: %s\n", formatGameStatus(string(game.Status)))
			fmt.Fprintf(w, "Source: %s\n", emptyAsNA(string(game.SourceType)))
			fmt.Fprintf(w, "Company: %s\n", emptyAsNA(game.Company))
			fmt.Fprintf(w, "Launch Path: %s\n", emptyAsNA(game.Path))
			fmt.Fprintf(w, "Save Path: %s\n", emptyAsNA(game.SavePath))
			fmt.Fprintf(w, "Process Name: %s\n", emptyAsNA(game.ProcessName))
			fmt.Fprintf(w, "Use Locale Emulator: %t\n", game.UseLocaleEmulator)
			fmt.Fprintf(w, "Use Magpie: %t\n", game.UseMagpie)
			fmt.Fprintf(w, "Created At: %s\n", game.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Fprintf(w, "Cached At: %s\n", game.CachedAt.Format("2006-01-02 15:04:05"))

			// Format summary with word wrap
			summary := strings.TrimSpace(game.Summary)
			if summary == "" {
				fmt.Fprintf(w, "Summary: N/A\n")
			} else {
				fmt.Fprintf(w, "Summary:\n%s\n", wrapText(summary, 70, "  "))
			}
			return nil
		},
	}
}

func formatGameStatus(status string) string {
	switch status {
	case "not_started":
		return "Not Started"
	case "playing":
		return "Playing"
	case "completed":
		return "Completed"
	case "on_hold":
		return "On Hold"
	case "dropped":
		return "Dropped"
	default:
		if status == "" {
			return "N/A"
		}
		return status
	}
}

func emptyAsNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return "N/A"
	}
	return value
}

// wrapText wraps text to specified width, respecting CJK characters
func wrapText(text string, maxWidth int, indent string) string {
	if text == "" {
		return ""
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentWidth := 0
	indentWidth := runewidth.StringWidth(indent)

	// Split by existing newlines first
	paragraphs := strings.Split(text, "\n")

	for pIdx, paragraph := range paragraphs {
		if pIdx > 0 {
			result.WriteString("\n")
		}

		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		words := strings.Fields(paragraph)
		currentWidth = 0
		currentLine.Reset()

		for _, word := range words {
			wordWidth := runewidth.StringWidth(word)
			spaceWidth := 1

			// If this is the first word on the line, add indent
			lineWidth := currentWidth
			if currentWidth == 0 {
				lineWidth = indentWidth
			}

			// Check if adding this word would exceed the max width
			if currentWidth > 0 && lineWidth+spaceWidth+wordWidth > maxWidth {
				// Write current line and start a new one
				result.WriteString(indent)
				result.WriteString(currentLine.String())
				result.WriteString("\n")
				currentLine.Reset()
				currentWidth = 0
			}

			// Add word to current line
			if currentWidth > 0 {
				currentLine.WriteString(" ")
				currentWidth += spaceWidth
			}
			currentLine.WriteString(word)
			currentWidth += wordWidth
		}

		// Write remaining content
		if currentLine.Len() > 0 {
			result.WriteString(indent)
			result.WriteString(currentLine.String())
		}
	}

	return result.String()
}
