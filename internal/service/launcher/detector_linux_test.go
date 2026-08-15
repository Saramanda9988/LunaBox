//go:build linux

package launcher

import (
	"testing"

	"lunabox/internal/utils/processutils"
)

func TestLinuxProcessCandidatePrefersTruncatedGameCommFromProtonTree(t *testing.T) {
	gamePath := "/home/u/Games/Escu/haison_fd2_gemini2.5pro.exe"
	input := StagedProcessDetectionInput{
		GameID:          "game",
		Launcher:        LaunchedProcessInfo{PID: 100, Name: "steam"},
		LauncherExeName: "steam",
		LaunchDir:       "/home/u/Games/Escu",
	}
	candidates := []linuxProcessCandidate{
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "pv-adverb", PID: 201},
				CommandLine:  []string{"/home/u/.steam/steamapps/common/Proton/proton", "waitforexitandrun", gamePath},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "python3", PID: 202},
				CommandLine:  []string{"/home/u/.steam/steamapps/common/Proton 11.0/proton", "waitforexitandrun", gamePath},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "steam.exe", PID: 203},
				CommandLine:  []string{gamePath},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "xalia.exe", PID: 204},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "haison_fd2_gemi", PID: 205},
			},
			fromDescendant: true,
		},
	}

	proc, ok := pickLinuxProcessCandidate(candidates, input, "Steam game", nil)
	if !ok {
		t.Fatal("expected Linux detector to select the truncated game process")
	}
	if proc.PID != 205 || proc.Name != "haison_fd2_gemi" {
		t.Fatalf("expected game process PID 205, got %+v", proc)
	}
}

func TestLinuxProcessCandidateMatchesWineLauncherDisplayNameTruncation(t *testing.T) {
	input := StagedProcessDetectionInput{
		GameID:          "game",
		Launcher:        LaunchedProcessInfo{PID: 100, Name: "haison_fd2_gemini2.5pro.exe"},
		LauncherExeName: "wine",
		LaunchDir:       "/home/u/Games/Escu",
	}
	candidates := []linuxProcessCandidate{
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "wineserver", PID: 201},
			},
			fromDescendant: true,
		},
		{
			detail: processutils.ProcessDetails{
				ProcessInfo: processutils.ProcessInfo{Name: "haison_fd2_gemi", PID: 202},
			},
			fromDescendant: true,
		},
	}

	proc, ok := pickLinuxProcessCandidate(candidates, input, "descendant", nil)
	if !ok {
		t.Fatal("expected Wine detector to select the truncated child process")
	}
	if proc.PID != 202 {
		t.Fatalf("expected game process PID 202, got %+v", proc)
	}
}
