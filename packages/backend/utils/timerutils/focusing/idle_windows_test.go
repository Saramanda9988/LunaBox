//go:build windows

package focusing

import "testing"

func TestIsGamepadActive(t *testing.T) {
	rest := xinputState{}
	cases := []struct {
		name string
		cur  xinputState
		want bool
	}{
		{"unchanged", rest, false},
		{"stick drift", xinputState{sThumbLX: 3000, sThumbRY: -4000}, false},
		{"trigger noise", xinputState{bLeftTrigger: 10}, false},
		{"button", xinputState{wButtons: 0x1000}, true},
		{"trigger pressed", xinputState{bRightTrigger: 200}, true},
		{"left stick", xinputState{sThumbLY: -20000}, true},
		{"right stick", xinputState{sThumbRX: 32767}, true},
	}
	for _, tc := range cases {
		if got := isGamepadActive(rest, tc.cur); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestGetIdleDurationAvailable(t *testing.T) {
	if _, ok := GetIdleDuration(); !ok {
		t.Fatal("expected GetLastInputInfo to be available")
	}
}
