package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBackupCmd(app *CoreApp) *cobra.Command {
	var gameQuery string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create local backup",
		// TODO: Add support for:
		//   -d, --database: Backup database
		//   -a, --all <path>: Full backup to specified path
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Check if at least one backup type is specified
			if gameQuery == "" {
				return fmt.Errorf("please specify backup type using flags (see --help)")
			}

			// Perform game save backup
			gameID, gameName, err := resolveGame(w, app, gameQuery)
			if err != nil {
				return err
			}

			backup, err := app.BackupService.CreateBackup(gameID)
			if err != nil {
				return err
			}

			fmt.Fprintln(w, "✓ Game save backup created successfully!")
			fmt.Fprintf(w, "Game: %s\n", gameName)
			fmt.Fprintf(w, "File: %s\n", backup.Name)
			fmt.Fprintf(w, "Size: %s\n", formatBytes(backup.Size))
			fmt.Fprintf(w, "Path: %s\n", backup.Path)
			return nil
		},
	}

	cmd.Flags().StringVarP(&gameQuery, "game", "g", "", "Backup game save (ID, ID prefix, or name - supports fuzzy matching)")
	return cmd
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
