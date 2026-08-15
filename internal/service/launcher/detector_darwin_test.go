//go:build darwin

package launcher

import (
	"lunabox/internal/utils/processutils"
	"testing"
)

func TestSelectDarwinSteamProcessPrefersSavedProcessAndSkipsHelpers(t *testing.T) {
	candidates := []processutils.ProcessInfo{
		{Name: "Game Helper", PID: 10},
		{Name: "FallbackGame", PID: 20},
		{Name: "SavedGame", PID: 30},
	}

	selected, ok := selectDarwinSteamProcess(candidates, "savedgame")
	if !ok || selected.PID != 30 {
		t.Fatalf("expected saved process, got %+v, %v", selected, ok)
	}
}

func TestSelectDarwinSteamProcessRejectsOnlyHelpers(t *testing.T) {
	candidates := []processutils.ProcessInfo{
		{Name: "steam_osx", PID: 10},
		{Name: "Game Helper", PID: 20},
	}
	if selected, ok := selectDarwinSteamProcess(candidates, ""); ok {
		t.Fatalf("expected no game process, got %+v", selected)
	}
}
