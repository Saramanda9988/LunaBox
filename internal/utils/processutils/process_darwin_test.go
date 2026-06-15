//go:build darwin

package processutils

import (
	"testing"
	"time"
)

func TestStartProcessReturnsExitSignal(t *testing.T) {
	started, err := StartProcess("/bin/sh", []string{"-c", "exit 0"}, "")
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	if started.PID == 0 {
		t.Fatal("expected pid")
	}
	if started.ExitChan == nil {
		t.Fatal("expected exit channel")
	}

	select {
	case <-started.ExitChan:
	case <-time.After(2 * time.Second):
		t.Fatal("expected exit channel to close")
	}
}
