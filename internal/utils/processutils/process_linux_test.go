//go:build linux

package processutils

import "testing"

func TestParseProcStatReturnsStateField(t *testing.T) {
	name, parentPID, fields, ok := parseProcStat("1234 (Game.exe) Z 1000 1234 1234 0 -1 4194304 1 2 3 4 5 6 7 8 20 0 1 0 123456")
	if !ok {
		t.Fatal("expected proc stat to parse")
	}
	if name != "Game.exe" {
		t.Fatalf("expected process name, got %q", name)
	}
	if parentPID != 1000 {
		t.Fatalf("expected parent PID 1000, got %d", parentPID)
	}
	if len(fields) == 0 || fields[0] != "Z" {
		t.Fatalf("expected zombie state in first field, got %#v", fields)
	}
}
