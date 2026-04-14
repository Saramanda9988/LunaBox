package cli

import (
	"fmt"

	"lunabox/internal/protocol"

	"github.com/spf13/cobra"
)

func newProtocolCmd(_ *CoreApp) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protocol",
		Short: "Manage the lunabox:// URL protocol handler",
	}

	cmd.AddCommand(newProtocolRegisterCmd())
	cmd.AddCommand(newProtocolUnregisterCmd())

	return cmd
}

func newProtocolRegisterCmd() *cobra.Command {
	var exePath string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register the lunabox:// URL protocol handler",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := protocol.RegisterURLScheme(exePath); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol registered successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&exePath, "exe", "", "Override the executable path used for the protocol handler")
	return cmd
}

func newProtocolUnregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unregister",
		Short: "Unregister the lunabox:// URL protocol handler",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := protocol.UnregisterURLScheme(); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol unregistered")
			return nil
		},
	}
}
