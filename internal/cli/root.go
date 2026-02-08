package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command for the CLI
func NewRootCmd(app *CoreApp) *cobra.Command {
	var showVersion bool

	cmd := &cobra.Command{
		Use:   "lunacli",
		Short: "LunaBox - Gal Game Manager",
		Long: `LunaBox - Gal Game Manager
Manage and play your gal games from the command line.`,
		SilenceErrors: true, // Errors are returned to caller
		SilenceUsage:  true, // Only show usage on flag errors
		Run: func(cmd *cobra.Command, args []string) {
			if showVersion {
				printVersion(cmd.OutOrStdout(), app)
				return
			}
			cmd.Help()
		},
	}

	// Disable mousetrap (prevents exit when GUI app double-clicked)
	cobra.MousetrapHelpText = ""

	cmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print the version number of LunaBox")

	cmd.AddCommand(newStartCmd(app))
	cmd.AddCommand(newListCmd(app))
	cmd.AddCommand(newVersionCmd(app))

	return cmd
}
