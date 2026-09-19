package updateutils

import (
	"errors"
	"fmt"
	"testing"
)

func TestFailureKindOfFollowsWrappedErrors(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("prepare update: %w",
		withFailureKind(FailureKindSignature, fmt.Errorf("verify Authenticode signature for %s: %w", "LunaBox.exe", errors.New("untrusted root"))))
	if kind := FailureKindOf(err); kind != FailureKindSignature {
		t.Fatalf("expected %s, got %s", FailureKindSignature, kind)
	}
	if kind := FailureKindOf(errors.New("plain error")); kind != FailureKindUnknown {
		t.Fatalf("expected %s for an untagged error, got %s", FailureKindUnknown, kind)
	}
	if kind := FailureKindOf(nil); kind != FailureKindUnknown {
		t.Fatalf("expected %s for a nil error, got %s", FailureKindUnknown, kind)
	}
}

func TestPrepareFailureMarkerRoundTrip(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	task := &Task{WorkDir: workDir}
	if kind, ok := ReadPrepareFailure(workDir); ok || kind != FailureKindUnknown {
		t.Fatalf("expected a missing marker, got %s (%v)", kind, ok)
	}
	if err := WritePrepareFailure(task, withFailureKind(FailureKindPatchApply, errors.New("boom"))); err != nil {
		t.Fatal(err)
	}
	kind, ok := ReadPrepareFailure(workDir)
	if !ok || kind != FailureKindPatchApply {
		t.Fatalf("expected %s, got %s (%v)", FailureKindPatchApply, kind, ok)
	}
}
