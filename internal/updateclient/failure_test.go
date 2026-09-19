package updateclient

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"lunabox/updater/updateutils"
)

func TestPrepareFailureReason(t *testing.T) {
	workDir := t.TempDir()
	if reason := prepareFailureReason(workDir); reason != failureReasonUpdaterNotStarted {
		t.Fatalf("expected %s without a marker, got %s", failureReasonUpdaterNotStarted, reason)
	}

	markerPath := filepath.Join(workDir, updateutils.PrepareFailureFileName)
	if err := os.WriteFile(markerPath, []byte(`{"kind":"signature_invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if reason := prepareFailureReason(workDir); reason != string(updateutils.FailureKindSignature) {
		t.Fatalf("expected %s, got %s", updateutils.FailureKindSignature, reason)
	}
}

func TestLaunchFailureReason(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		expected string
	}{
		{"cancelled elevation", fmt.Errorf("start updater commit: %w", syscall.Errno(1223)), failureReasonUACCancelled},
		{"access denied", syscall.Errno(5), failureReasonAccessDenied},
		{"permission sentinel", os.ErrPermission, failureReasonAccessDenied},
		{"missing executable", syscall.Errno(2), failureReasonUpdaterMissing},
		{"unknown", errors.New("boom"), failureReasonLaunchFailed},
	}

	for _, testCase := range cases {
		if reason := launchFailureReason(testCase.err); reason != testCase.expected {
			t.Errorf("%s: expected %s, got %s", testCase.name, testCase.expected, reason)
		}
	}
}
