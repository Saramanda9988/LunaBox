//go:build windows

package protocol

import "testing"

func TestExtractExeFromCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "quoted", command: `"C:\Program Files\LunaBox\LunaBox.exe" "%1"`, want: `C:\Program Files\LunaBox\LunaBox.exe`},
		{name: "plain", command: `C:\LunaBox\LunaBox.exe "%1"`, want: `C:\LunaBox\LunaBox.exe`},
		{name: "empty", command: "  ", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := extractExeFromCommand(test.command); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
