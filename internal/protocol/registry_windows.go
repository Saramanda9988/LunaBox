//go:build windows

package protocol

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// RegisterURLScheme registers lunabox:// in HKCU (no admin required).
// exePath should be the absolute path to LunaBox.exe.
func RegisterURLScheme(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
		exePath, _ = filepath.Abs(exePath)
	}

	root, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\`+Scheme,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("create registry key: %w", err)
	}
	defer root.Close()

	if err := root.SetStringValue("", "URL:LunaBox Protocol"); err != nil {
		return err
	}
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}

	cmdKey, _, err := registry.CreateKey(root, `shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create command key: %w", err)
	}
	defer cmdKey.Close()

	// Windows replaces %1 with the full lunabox:// URI at invocation time.
	command := fmt.Sprintf(`"%s" "%%1"`, exePath)
	return cmdKey.SetStringValue("", command)
}

// UnregisterURLScheme removes the lunabox:// protocol handler from HKCU.
func UnregisterURLScheme() error {
	paths := []string{
		`Software\Classes\` + Scheme + `\shell\open\command`,
		`Software\Classes\` + Scheme + `\shell\open`,
		`Software\Classes\` + Scheme + `\shell`,
		`Software\Classes\` + Scheme,
	}
	for _, p := range paths {
		err := registry.DeleteKey(registry.CURRENT_USER, p)
		if err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("delete key %s: %w", p, err)
		}
	}
	return nil
}
