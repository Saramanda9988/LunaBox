package updateclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsUpdateWorkDir(t *testing.T) {
	workDir, err := os.MkdirTemp("", "LunaBox-update-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workDir) })

	if !isUpdateWorkDir(workDir) {
		t.Fatalf("expected update work directory to be accepted: %s", workDir)
	}
	if isUpdateWorkDir(filepath.Dir(workDir)) {
		t.Fatal("expected temp root to be rejected")
	}
	if isUpdateWorkDir(t.TempDir()) {
		t.Fatal("expected unrelated temporary directory to be rejected")
	}
}
