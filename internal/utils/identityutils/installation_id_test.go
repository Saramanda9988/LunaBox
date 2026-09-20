package identityutils

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateInstallationIDAtPersistsRandomValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity", installationIDFileName)

	first, err := LoadOrCreateInstallationIDAt(path)
	if err != nil {
		t.Fatal(err)
	}
	random, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil || len(random) != 18 {
		t.Fatalf("installation id %q is not an 18-byte random value", first)
	}
	second, err := LoadOrCreateInstallationIDAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("installation id changed from %q to %q", first, second)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("installation identity file mode = %o, want 600", mode)
		}
	}
}

func TestLoadOrCreateInstallationIDAtReplacesInvalidValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), installationIDFileName)
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	id, err := LoadOrCreateInstallationIDAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if id == "invalid" || !installationIDPattern.MatchString(id) {
		t.Fatalf("invalid installation id was not replaced: %q", id)
	}
}

func TestLoadOrCreateInstallationIDAtAdoptsLegacyValue(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "identity", installationIDFileName)
	legacyPath := filepath.Join(root, "umbra", "install-id")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	const legacyID = "UmbraLegacyInstallID_123"
	if err := os.WriteFile(legacyPath, []byte(legacyID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	id, err := loadOrCreateInstallationIDAt(path, legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if id != legacyID {
		t.Fatalf("installation id = %q, want legacy value %q", id, legacyID)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(persisted) != legacyID+"\n" {
		t.Fatalf("persisted installation id = %q", persisted)
	}
}
