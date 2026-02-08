package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newVersionCmd(app *CoreApp) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the version number of LunaBox",
		Long:    `All software has versions. This is LunaBox's`,
		Aliases: []string{"v"},
		Run: func(cmd *cobra.Command, args []string) {
			printVersion(cmd.OutOrStdout(), app)
		},
	}
}

func printVersion(w io.Writer, app *CoreApp) {
	// Use VersionService if available
	if app.VersionService != nil {
		info := app.VersionService.GetVersionInfo()
		fmt.Fprintf(w, "LunaBox v%s\n", info["version"])
		fmt.Fprintf(w, "Commit: %s\n", info["commit"])
		fmt.Fprintf(w, "Build Time: %s\n", info["buildTime"])
		fmt.Fprintf(w, "Build Mode: %s\n", info["buildMode"])
	} else {
		// Fallback (should not happen in normal flow)
		fmt.Fprintln(w, "Version information unavailable")
	}
}
