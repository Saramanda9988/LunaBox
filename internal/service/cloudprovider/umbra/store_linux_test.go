//go:build linux

package umbra

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
)

func TestLinuxTokenStoreSavesAndRestoresCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials", "tokens.json")
	store := umbrsdk.NewFileTokenStore(path)
	want := &umbrsdk.TokenSet{
		AccessToken:  "access-secret",
		RefreshToken: "refresh-secret",
		TokenType:    "bearer",
	}

	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertLinuxCredentialPermissions(t, path)

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil || got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	if err := store.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	got, err = store.Load(context.Background())
	if err != nil || got != nil {
		t.Fatalf("Load() after Clear = %#v, %v", got, err)
	}
}

func TestLinuxDeviceStoreSavesAndRestoresCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials", "device.json")
	store := umbrsdk.NewFileDeviceStore(path)
	want := &umbrsdk.DeviceCredentials{
		DeviceID:     "dev-test",
		DeviceSecret: "device-secret",
	}

	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertLinuxCredentialPermissions(t, path)

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil || got.DeviceID != want.DeviceID || got.DeviceSecret != want.DeviceSecret {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func assertLinuxCredentialPermissions(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("credential file mode = %o, want 600", mode)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", filepath.Dir(path), err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("credential dir mode = %o, want 700", mode)
	}
}
