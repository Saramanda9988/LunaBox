//go:build windows

package saveprobe

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorCapturesGameDirectoryWrite(t *testing.T) {
	directory := t.TempDir()
	collector, err := Start(directory)
	if err != nil {
		t.Fatalf("start collector: %v", err)
	}
	t.Cleanup(func() {
		_, _ = collector.Stop()
	})

	time.Sleep(100 * time.Millisecond)
	userDataDirectory := filepath.Join(directory, "UserData")
	if err := os.MkdirAll(userDataDirectory, 0o700); err != nil {
		t.Fatalf("create user data directory: %v", err)
	}
	path := filepath.Join(userDataDirectory, "AoKana018.sud")
	if err := os.WriteFile(path, []byte("save data"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	result, err := collector.Stop()
	if err != nil {
		t.Fatalf("stop collector: %v", err)
	}
	if result.ObservedEvents == 0 {
		t.Fatal("expected directory change events")
	}
	for _, candidate := range result.Candidates {
		if candidate.Path == path || candidate.Path == userDataDirectory {
			return
		}
	}
	t.Fatalf("expected %s in candidates, got %#v", path, result.Candidates)
}
