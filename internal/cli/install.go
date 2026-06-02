package cli

import (
	"fmt"

	"lunabox/internal/applog"
	"lunabox/internal/common/vo"

	"github.com/spf13/cobra"
)

func newInstallCmd(app *CoreApp) *cobra.Command {
	var title string
	var fileName string
	var archiveFormat string
	var startupPath string
	var metaSource string
	var metaID string
	var size int64
	var checksumAlgo string
	var checksum string

	cmd := &cobra.Command{
		Use:   "install <url>",
		Short: "Download and install a game from URL",
		Long: `Download a game from a URL, automatically extract, match metadata, and add to library.
Only URL and title are required. Other fields are inferred from the URL or use defaults.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			downloadURL := args[0]

			if title == "" {
				return fmt.Errorf("title is required (use --title)")
			}

			if app.DownloadService == nil {
				return fmt.Errorf("download service not available")
			}

			req := vo.InstallFromURLRequest{
				URL:           downloadURL,
				Title:         title,
				FileName:      fileName,
				ArchiveFormat: archiveFormat,
				StartupPath:   startupPath,
				MetaSource:    metaSource,
				MetaID:        metaID,
				Size:          size,
				ChecksumAlgo:  checksumAlgo,
				Checksum:      checksum,
			}

			applog.LogInfof(app.Ctx, "Installing game from URL: %s (title: %s)", downloadURL, title)
			fmt.Fprintf(w, "Downloading and installing: %s\n", title)
			fmt.Fprintf(w, "URL: %s\n", downloadURL)
			fmt.Fprintln(w, "Please wait...")

			result, err := app.DownloadService.InstallFromURL(req)
			if err != nil {
				applog.LogErrorf(app.Ctx, "Install from URL failed: %v", err)
				return err
			}

			fmt.Fprintln(w)
			fmt.Fprintf(w, "✓ Game installed successfully!\n")
			fmt.Fprintf(w, "Game: %s\n", result.GameName)
			if result.GameID != "" {
				fmt.Fprintf(w, "ID: %s\n", result.GameID)
			}
			fmt.Fprintf(w, "Path: %s\n", result.GamePath)

			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Game title (required)")
	cmd.Flags().StringVarP(&fileName, "file-name", "f", "", "Download file name (auto-inferred from URL)")
	cmd.Flags().StringVarP(&archiveFormat, "format", "F", "", "Archive format: zip/rar/7z/none etc. (auto-inferred)")
	cmd.Flags().StringVarP(&startupPath, "startup", "s", "", "Relative path to executable inside archive")
	cmd.Flags().StringVarP(&metaSource, "meta-source", "m", "", "Metadata source: bangumi/vndb/ymgal/steam")
	cmd.Flags().StringVarP(&metaID, "meta-id", "i", "", "Metadata ID from the source")
	cmd.Flags().Int64VarP(&size, "size", "S", 0, "File size in bytes (0 = unknown)")
	cmd.Flags().StringVar(&checksumAlgo, "checksum-algo", "", "Checksum algorithm: sha256/blake3")
	cmd.Flags().StringVar(&checksum, "checksum", "", "Checksum value (64 hex characters)")

	_ = cmd.MarkFlagRequired("title")

	return cmd
}
