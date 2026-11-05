//go:build linux

package focusing

import (
	"errors"
	"testing"
	"time"
)

func TestParseIdleMillis(t *testing.T) {
	cases := map[string]uint64{
		"1234\n":            1234,
		"(uint64 98765,)\n": 98765,
	}
	for input, want := range cases {
		got, ok := parseIdleMillis(input)
		if !ok || got != want {
			t.Fatalf("parseIdleMillis(%q) = %d, %v; want %d", input, got, ok, want)
		}
	}
	if _, ok := parseIdleMillis("Error: not supported"); ok {
		t.Fatal("expected error output to be rejected")
	}
}

func TestLinuxIdleStateCachesAndBacksOff(t *testing.T) {
	orig := runLinuxIdleCommand
	defer func() { runLinuxIdleCommand = orig }()

	calls := 0
	fail := false
	runLinuxIdleCommand = func(name string, args ...string) ([]byte, error) {
		calls++
		if fail || name != "gdbus" {
			return nil, errors.New("unavailable")
		}
		return []byte("(uint64 3000,)"), nil
	}

	var state linuxIdleState
	now := time.Unix(1000, 0)
	if idle, ok := state.get(now); !ok || idle != 3*time.Second {
		t.Fatalf("unexpected idle %v %v", idle, ok)
	}
	state.get(now.Add(time.Second))
	if calls != 1 {
		t.Fatalf("expected cached result, got %d calls", calls)
	}

	fail = true
	if _, ok := state.get(now.Add(linuxIdleCacheTTL)); ok {
		t.Fatal("expected failure once all sources fail")
	}
	before := calls
	if _, ok := state.get(now.Add(linuxIdleCacheTTL + time.Second)); ok || calls != before {
		t.Fatal("expected unsupported result to be cached")
	}
}
