package cli

import (
	"fmt"

	"lunabox/internal/protocol"

	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command for the CLI
func NewRootCmd(app *CoreApp) *cobra.Command {
	var showVersion bool
	var registerProtocol bool
	var unregisterProtocol bool
	var protocolExePath string

	cmd := &cobra.Command{
		Use:   "lunacli",
		Short: "LunaBox - Gal Game Manager",
		Long: `LunaBox - Gal Game Manager
Manage and play your gal games from the command line.`,
		SilenceErrors: true, // Errors are returned to caller
		SilenceUsage:  true, // Only show usage on flag errors
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case registerProtocol && unregisterProtocol:
				return fmt.Errorf("--register-protocol and --unregister-protocol cannot be used together")
			case registerProtocol:
				if err := protocol.RegisterURLScheme(protocolExePath); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol registered successfully")
				return nil
			case unregisterProtocol:
				if err := protocol.UnregisterURLScheme(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol unregistered")
				return nil
			case showVersion:
				printVersion(cmd.OutOrStdout(), app)
				return nil
			default:
				return cmd.Help()
			}
		},
	}

	// Disable mousetrap (prevents exit when GUI app double-clicked)
	cobra.MousetrapHelpText = ""

	cmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print the version number of LunaBox")
	cmd.Flags().BoolVar(&registerProtocol, "register-protocol", false, "Register the lunabox:// URL protocol handler")
	cmd.Flags().BoolVar(&unregisterProtocol, "unregister-protocol", false, "Unregister the lunabox:// URL protocol handler")
	cmd.Flags().StringVar(&protocolExePath, "exe", "", "Override the executable path used with --register-protocol")

	cmd.AddCommand(newStartCmd(app))
	cmd.AddCommand(newListCmd(app))
	cmd.AddCommand(newDetailCmd(app))
	cmd.AddCommand(newBackupCmd(app))
	cmd.AddCommand(newVersionCmd(app))
	cmd.AddCommand(newLunaCmd(app))
	cmd.AddCommand(newProtocolCmd(app))

	return cmd
}
