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
