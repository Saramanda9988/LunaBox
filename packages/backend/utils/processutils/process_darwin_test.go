//go:build darwin

package processutils

import (
	"encoding/binary"
	"os"
	"slices"
	"testing"
	"time"
)

func TestParseKernProcArgs(t *testing.T) {
	data := make([]byte, 4)
	binary.NativeEndian.PutUint32(data, 3)
	data = append(data, "/Applications/CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine"...)
	data = append(data, 0, 0, 0)
	for _, value := range []string{
		"wine",
		`Z:\Users\u\games\Game.exe`,
		"--windowed",
		"CX_BOTTLE=test1",
		"WINEDEBUG=-all",
	} {
		data = append(data, value...)
		data = append(data, 0)
	}

	info, err := parseKernProcArgs(data)
	if err != nil {
		t.Fatalf("parse process arguments: %v", err)
	}
	if info.ExecutablePath != "/Applications/CrossOver.app/Contents/SharedSupport/CrossOver/bin/wine" {
		t.Fatalf("unexpected executable path: %q", info.ExecutablePath)
	}
	wantArguments := []string{"wine", `Z:\Users\u\games\Game.exe`, "--windowed"}
	if !slices.Equal(info.Arguments, wantArguments) {
		t.Fatalf("unexpected arguments: %#v", info.Arguments)
	}
	if info.Environment["CX_BOTTLE"] != "test1" || info.Environment["WINEDEBUG"] != "-all" {
		t.Fatalf("unexpected environment: %#v", info.Environment)
	}
}

func TestGetProcessCommandInfoCurrentProcess(t *testing.T) {
	info, err := GetProcessCommandInfo(uint32(os.Getpid()))
	if err != nil {
		t.Fatalf("get current process command info: %v", err)
	}
	if len(info.Arguments) == 0 {
		t.Fatal("expected current process arguments")
	}
}

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

func TestGetProcessCreationTimeCurrentProcess(t *testing.T) {
	created, err := GetProcessCreationTime(uint32(os.Getpid()))
	if err != nil {
		t.Fatalf("get creation time: %v", err)
	}
	if created.IsZero() || created.After(time.Now().Add(time.Minute)) {
		t.Fatalf("unexpected creation time: %v", created)
	}
	if _, err := GetProcessCreationTime(0); err == nil {
		t.Fatal("expected error for pid 0")
	}
}

func TestGetProcessesByExecutableDirCurrentProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	dir := exe[:len(exe)-len(os.Args[0])]
	if dir == "" {
		dir = "/"
	}
	// Resolve directory cleanly using filepath.Dir via os.Stat-safe parent.
	procs, err := GetProcessesByExecutableDir(exe)
	if err != nil {
		t.Fatalf("GetProcessesByExecutableDir: %v", err)
	}
	selfPID := uint32(os.Getpid())
	found := false
	for _, proc := range procs {
		if proc.PID == selfPID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected current process (PID %d) in %+v", selfPID, procs)
	}
}

func TestVisibleWindowHelpers(t *testing.T) {
	if HasVisibleTopLevelWindow(0) {
		t.Fatal("pid 0 should not have a visible window")
	}
	if got := FilterProcessesWithVisibleWindows(nil); len(got) != 0 {
		t.Fatalf("expected empty slice for nil input, got %+v", got)
	}
	// Smoke-test CoreGraphics enumeration; should not panic in GUI or headless runs.
	pids, ok := VisibleWindowPIDs()
	if ok && pids[0] {
		t.Fatal("visible window map must not contain pid 0")
	}
}
