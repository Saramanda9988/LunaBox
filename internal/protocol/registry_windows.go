//go:build windows

package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const portableRegistryPath = `Software\Classes\` + Scheme

// RegisterPortableURLScheme registers lunabox:// for a Windows portable build.
// Packaged builds use Wails' protocol configuration instead.
func RegisterPortableURLScheme(exePath string) error {
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}
	}

	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("normalize executable path: %w", err)
	}

	root, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		portableRegistryPath,
		registry.CREATE_SUB_KEY|registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("create protocol registry key: %w", err)
	}
	defer root.Close()

	if err := root.SetStringValue("", "URL:LunaBox Protocol"); err != nil {
		return fmt.Errorf("set protocol description: %w", err)
	}
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return fmt.Errorf("mark URL protocol: %w", err)
	}

	commandKey, _, err := registry.CreateKey(root, `shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create protocol command key: %w", err)
	}
	defer commandKey.Close()

	command := fmt.Sprintf(`"%s" "%%1"`, absPath)
	if err := commandKey.SetStringValue("", command); err != nil {
		return fmt.Errorf("set protocol command: %w", err)
	}
	return nil
}

// GetRegisteredURLSchemeExe returns the executable currently registered for
// lunabox:// in the current user's registry. An empty path means unregistered.
func GetRegisteredURLSchemeExe() (string, error) {
	commandKey, err := registry.OpenKey(
		registry.CURRENT_USER,
		portableRegistryPath+`\shell\open\command`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", nil
		}
		return "", fmt.Errorf("open protocol command key: %w", err)
	}
	defer commandKey.Close()

	command, _, err := commandKey.GetStringValue("")
	if err != nil {
		if err == registry.ErrNotExist {
			return "", nil
		}
		return "", fmt.Errorf("read protocol command: %w", err)
	}

	return extractExeFromCommand(command), nil
}

func extractExeFromCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if strings.HasPrefix(command, `"`) {
		if end := strings.Index(command[1:], `"`); end >= 0 {
			return command[1 : 1+end]
		}
		return strings.TrimPrefix(command, `"`)
	}
	if index := strings.IndexAny(command, " \t"); index >= 0 {
		return command[:index]
	}
	return command
}

// UnregisterPortableURLScheme removes the current-user lunabox:// association.
func UnregisterPortableURLScheme() error {
	if err := deleteRegistryTree(registry.CURRENT_USER, portableRegistryPath); err != nil {
		return fmt.Errorf("delete protocol registry key: %w", err)
	}
	return nil
}

func deleteRegistryTree(root registry.Key, path string) error {
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}

	subKeys, readErr := key.ReadSubKeyNames(-1)
	closeErr := key.Close()
	if readErr != nil {
		return fmt.Errorf("read protocol registry subkeys: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close protocol registry key: %w", closeErr)
	}

	for _, subKey := range subKeys {
		if err := deleteRegistryTree(root, path+`\`+subKey); err != nil {
			return err
		}
	}

	if err := registry.DeleteKey(root, path); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}
