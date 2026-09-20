package identityutils

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"lunabox/internal/utils/apputils"
)

const (
	installationIDDirectory = "identity"
	installationIDFileName  = "installation-id"
)

var (
	installationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
	installationIDLocks   sync.Map
)

// LoadOrCreateInstallationID returns LunaBox's persistent anonymous installation
// identifier. Existing Umbra and update telemetry identifiers are adopted once
// so upgrades keep the same identity.
func LoadOrCreateInstallationID() (string, error) {
	configDir, err := apputils.GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve installation identity directory: %w", err)
	}
	legacyPaths := []string{filepath.Join(configDir, "umbra", "install-id")}
	if cacheDir, cacheErr := apputils.GetCacheDir(); cacheErr == nil {
		legacyPaths = append(legacyPaths, filepath.Join(cacheDir, "updates", installationIDFileName))
	}
	return loadOrCreateInstallationIDAt(
		filepath.Join(configDir, installationIDDirectory, installationIDFileName),
		legacyPaths...,
	)
}

// LoadOrCreateInstallationIDAt is the file-based form used by tests and tools
// that need an explicitly scoped identity file.
func LoadOrCreateInstallationIDAt(path string) (string, error) {
	return loadOrCreateInstallationIDAt(path)
}

func loadOrCreateInstallationIDAt(path string, legacyPaths ...string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", fmt.Errorf("installation identity file is empty")
	}
	lockValue, _ := installationIDLocks.LoadOrStore(path, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	if id, found, err := readInstallationID(path); err != nil {
		return "", fmt.Errorf("read installation identity: %w", err)
	} else if found {
		return id, nil
	}

	for _, legacyPath := range legacyPaths {
		id, found, err := readInstallationID(legacyPath)
		if err != nil || !found {
			continue
		}
		return saveInstallationID(path, id)
	}

	id, err := newInstallationID()
	if err != nil {
		return "", fmt.Errorf("generate installation identity: %w", err)
	}
	return saveInstallationID(path, id)
}

func readInstallationID(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	id := strings.TrimSpace(string(data))
	return id, installationIDPattern.MatchString(id), nil
}

func newInstallationID() (string, error) {
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func saveInstallationID(path string, id string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create installation identity directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("protect installation identity directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, installationIDFileName+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create installation identity file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		return "", fmt.Errorf("protect installation identity file: %w", err)
	}
	if _, err := tempFile.WriteString(id + "\n"); err != nil {
		_ = tempFile.Close()
		return "", fmt.Errorf("write installation identity: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return "", fmt.Errorf("sync installation identity: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("close installation identity: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		if existingID, found, readErr := readInstallationID(path); readErr == nil && found {
			return existingID, nil
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return "", fmt.Errorf("replace installation identity: %w", removeErr)
		}
		if retryErr := os.Rename(tempPath, path); retryErr != nil {
			return "", fmt.Errorf("save installation identity: %w", retryErr)
		}
	}
	return id, nil
}
