//go:build linux

package focusing

import (
	"errors"
	"testing"
)

func TestParseKDEForegroundProcessID(t *testing.T) {
	pid, ok := parseKDEForegroundProcessID("9959\n")
	if !ok || pid != 9959 {
		t.Fatalf("expected pid 9959, got %d (ok=%v)", pid, ok)
	}
}

func TestParseKDEForegroundProcessIDRejectsInvalidOutput(t *testing.T) {
	pid, ok := parseKDEForegroundProcessID("active window has no pid")
	if ok || pid != 0 {
		t.Fatalf("expected invalid output to be rejected, got %d (ok=%v)", pid, ok)
	}
}

func TestGetForegroundProcessIDUsesKDotoolOutput(t *testing.T) {
	restore := stubKDEActiveWindowPIDOutput(func() ([]byte, error) {
		return []byte("1234\n"), nil
	})
	defer restore()

	pid, ok := GetForegroundProcessID()
	if !ok || pid != 1234 {
		t.Fatalf("expected pid 1234, got %d (ok=%v)", pid, ok)
	}
}

func TestGetForegroundProcessIDFailsSoftlyWhenKDotoolFails(t *testing.T) {
	restore := stubKDEActiveWindowPIDOutput(func() ([]byte, error) {
		return nil, errors.New("kdotool failed")
	})
	defer restore()

	pid, ok := GetForegroundProcessID()
	if ok || pid != 0 {
		t.Fatalf("expected failed kdotool lookup to fall back, got %d (ok=%v)", pid, ok)
	}
}

func stubKDEActiveWindowPIDOutput(fn func() ([]byte, error)) func() {
	orig := kdeActiveWindowPIDOutput
	kdeActiveWindowPIDOutput = fn
	return func() {
		kdeActiveWindowPIDOutput = orig
	}
}
