package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command for the CLI
func NewRootCmd(app *CoreApp) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lunacli",
		Short: "LunaBox - Gal Game Manager",
		Long: `LunaBox - Gal Game Manager
Manage and play your gal games from the command line.`,
		SilenceUsage: true, // Only show usage on flag errors
	}

	// Disable mousetrap (prevents exit when GUI app double-clicked)
	cobra.MousetrapHelpText = ""

	cmd.AddCommand(newStartCmd(app))
	cmd.AddCommand(newListCmd(app))

	return cmd
}
