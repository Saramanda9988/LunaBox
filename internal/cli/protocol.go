package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"lunabox/internal/protocol"
	"lunabox/internal/utils/apputils"

	"github.com/spf13/cobra"
)

func newProtocolCmd(_ *CoreApp) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protocol",
		Short: "Manage the portable lunabox:// URL protocol handler",
	}

	cmd.AddCommand(newProtocolRegisterCmd())
	cmd.AddCommand(newProtocolUnregisterCmd())
	return cmd
}

func newProtocolRegisterCmd() *cobra.Command {
	var exePath string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register lunabox:// for a portable build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !apputils.IsPortableMode() {
				return fmt.Errorf("installed builds manage lunabox:// through the Wails installer")
			}
			if runtime.GOOS != "windows" {
				return fmt.Errorf("portable protocol registration is only supported on Windows")
			}
			if exePath == "" {
				var err error
				exePath, err = siblingPortableGUIPath()
				if err != nil {
					return err
				}
			}
			if err := protocol.RegisterPortableURLScheme(exePath); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "lunabox:// protocol registered for the portable build")
			return nil
		},
	}

	cmd.Flags().StringVar(&exePath, "exe", "", "Override the portable executable path")
	return cmd
}

func siblingPortableGUIPath() (string, error) {
	cliPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get lunacli executable path: %w", err)
	}
	guiPath := filepath.Join(filepath.Dir(cliPath), "LunaBox.exe")
	info, err := os.Stat(guiPath)
	if err != nil {
		return "", fmt.Errorf("find portable LunaBox.exe next to lunacli: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("portable LunaBox.exe path is a directory: %s", guiPath)
	}
	return guiPath, nil
}

func newProtocolUnregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unregister",
		Short: "Unregister the portable lunabox:// handler",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !apputils.IsPortableMode() {
				return fmt.Errorf("installed builds manage lunabox:// through the Wails installer")
			}
			if err := protocol.UnregisterPortableURLScheme(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "portable lunabox:// protocol unregistered")
			return nil
		},
	}
}
