//go:build darwin

package launcher

import (
	"fmt"
	"lunabox/backend/utils/processutils"
	"testing"
	"time"
)

func TestDarwinSteamSamplesScoreWindowsForegroundAndSavedProcess(t *testing.T) {
	dirCandidates := []processutils.ProcessInfo{
		{Name: "Game Helper", PID: 10},
		{Name: "BackgroundTool", PID: 20},
		{Name: "VisibleGame", PID: 30},
	}
	savedCandidates := []processutils.ProcessInfo{
		{Name: "ExternalSavedGame", PID: 40},
	}
	windowPIDs := map[uint32]bool{30: true, 40: true}

	samples := darwinSteamSamples(dirCandidates, savedCandidates, "externalsavedgame", 1, windowPIDs, true, 30, true)
	if len(samples) != 3 {
		t.Fatalf("expected helper to be skipped and 3 candidates returned, got %+v", samples)
	}
	saved, ok := savedNameCandidate(samples)
	if !ok || saved.Process.PID != 40 || !saved.Window || !saved.WindowKnown {
		t.Fatalf("expected external saved process with window evidence, got %+v, %v", saved, ok)
	}
	for _, sample := range samples {
		if sample.Relation != relationDescendantInDir || !sample.WindowKnown {
			t.Fatalf("expected primary relation with window channel enabled, got %+v", sample)
		}
		if sample.Window != (sample.Process.PID == 30 || sample.Process.PID == 40) {
			t.Fatalf("unexpected window flag for %+v", sample)
		}
		if sample.Foreground != (sample.Process.PID == 30) {
			t.Fatalf("expected only the frontmost app to be foreground, got %+v", sample)
		}
	}
}

func TestDarwinSteamSamplesFallbackWhenWindowEnumerationUnavailable(t *testing.T) {
	dirCandidates := []processutils.ProcessInfo{
		{Name: "GameApp", PID: 20},
	}
	samples := darwinSteamSamples(dirCandidates, nil, "", 1, nil, false, 20, true)
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %+v", samples)
	}
	if samples[0].WindowKnown || samples[0].Window {
		t.Fatalf("expected WindowKnown=false when enumeration fails, got %+v", samples[0])
	}
	if !samples[0].Foreground || !samples[0].ForegroundKnown {
		t.Fatalf("expected foreground evidence to remain active, got %+v", samples[0])
	}
}

func TestDarwinSteamSamplesRejectOnlyHelpers(t *testing.T) {
	candidates := []processutils.ProcessInfo{
		{Name: "steam_osx", PID: 10},
		{Name: "Game Helper", PID: 20},
	}
	if samples := darwinSteamSamples(candidates, nil, "", 1, nil, false, 0, false); len(samples) != 0 {
		t.Fatalf("expected no game process, got %+v", samples)
	}
}

func TestDarwinSteamSamplerFindsExternalSavedProcess(t *testing.T) {
	prevDir := darwinSteamProcessLookup
	prevByName := darwinProcessesByNameLookup
	prevWin := darwinVisibleWindowPIDs
	prevFg := darwinForegroundProcessID
	t.Cleanup(func() {
		darwinSteamProcessLookup = prevDir
		darwinProcessesByNameLookup = prevByName
		darwinVisibleWindowPIDs = prevWin
		darwinForegroundProcessID = prevFg
	})

	darwinSteamProcessLookup = func(string) ([]processutils.ProcessInfo, error) {
		return nil, nil
	}
	darwinProcessesByNameLookup = func(name string) ([]processutils.ProcessInfo, error) {
		if name != "RealGame" {
			return nil, nil
		}
		return []processutils.ProcessInfo{{Name: "RealGame", PID: 777}}, nil
	}
	darwinVisibleWindowPIDs = func() (map[uint32]bool, bool) {
		return map[uint32]bool{777: true}, true
	}
	darwinForegroundProcessID = func() (uint32, bool) {
		return 777, true
	}

	sampler := newDarwinSteamSampler(StagedProcessDetectionInput{
		GameID:           "g1",
		LaunchDir:        "/Applications/MyGame.app",
		SavedProcessName: "RealGame",
	}, nil)
	samples, err := sampler.sample()
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	if len(samples) != 1 || samples[0].Process.PID != 777 || !samples[0].SavedName || !samples[0].Window || !samples[0].Foreground {
		t.Fatalf("unexpected samples: %+v", samples)
	}
}

func TestDarwinDetectSuccessorPrefersWindowedDirectoryProcessAndRejectsStale(t *testing.T) {
	prevDir := darwinSteamProcessLookup
	prevDesc := darwinDescendantProcessLookup
	prevCreated := darwinProcessCreationTime
	prevFilterWin := darwinFilterVisibleWindowProcesses
	prevFg := darwinForegroundProcessID
	prevDelays := successorGraceDelays
	t.Cleanup(func() {
		darwinSteamProcessLookup = prevDir
		darwinDescendantProcessLookup = prevDesc
		darwinProcessCreationTime = prevCreated
		darwinFilterVisibleWindowProcesses = prevFilterWin
		darwinForegroundProcessID = prevFg
		successorGraceDelays = prevDelays
	})

	sessionStart := time.Now()
	successorGraceDelays = []time.Duration{0}

	darwinSteamProcessLookup = func(string) ([]processutils.ProcessInfo, error) {
		return []processutils.ProcessInfo{
			{Name: "StaleTool", PID: 100},
			{Name: "BackgroundDaemon", PID: 200},
			{Name: "MainGame", PID: 300},
		}, nil
	}
	darwinDescendantProcessLookup = func(uint32) ([]processutils.ProcessInfo, error) {
		return nil, nil
	}
	darwinProcessCreationTime = func(pid uint32) (time.Time, error) {
		if pid == 100 {
			return sessionStart.Add(-10 * time.Minute), nil
		}
		return sessionStart.Add(2 * time.Second), nil
	}
	darwinForegroundProcessID = func() (uint32, bool) {
		return 0, false
	}
	darwinFilterVisibleWindowProcesses = func(procs []processutils.ProcessInfo) []processutils.ProcessInfo {
		out := make([]processutils.ProcessInfo, 0, 1)
		for _, proc := range procs {
			if proc.PID == 300 {
				out = append(out, proc)
			}
		}
		return out
	}

	got, ok := DetectSuccessorProcess(SuccessorDetectionInput{
		GameID:            "g1",
		ExitedPID:         50,
		ExitedProcessName: "Launcher",
		LaunchDir:         "/Applications/MainGame.app",
		SessionStart:      sessionStart,
		SelfPID:           1,
	}, nil)
	if !ok || got.PID != 300 || got.Name != "MainGame" {
		t.Fatalf("DetectSuccessorProcess() = %+v, %v; want MainGame (PID 300)", got, ok)
	}
}

func TestDarwinDetectSuccessorFallsBackToSameNameProcess(t *testing.T) {
	prevDir := darwinSteamProcessLookup
	prevDesc := darwinDescendantProcessLookup
	prevPIDByName := darwinProcessPIDByNameLookup
	prevCreated := darwinProcessCreationTime
	prevDelays := successorGraceDelays
	t.Cleanup(func() {
		darwinSteamProcessLookup = prevDir
		darwinDescendantProcessLookup = prevDesc
		darwinProcessPIDByNameLookup = prevPIDByName
		darwinProcessCreationTime = prevCreated
		successorGraceDelays = prevDelays
	})

	sessionStart := time.Now()
	successorGraceDelays = []time.Duration{0}

	darwinSteamProcessLookup = func(string) ([]processutils.ProcessInfo, error) { return nil, nil }
	darwinDescendantProcessLookup = func(uint32) ([]processutils.ProcessInfo, error) { return nil, nil }
	darwinProcessPIDByNameLookup = func(name string) (uint32, error) {
		if name == "ReExecGame" {
			return 404, nil
		}
		return 0, fmt.Errorf("not found")
	}
	darwinProcessCreationTime = func(uint32) (time.Time, error) {
		return sessionStart.Add(time.Second), nil
	}

	got, ok := DetectSuccessorProcess(SuccessorDetectionInput{
		GameID:            "g1",
		ExitedPID:         400,
		ExitedProcessName: "ReExecGame",
		LaunchDir:         "/tmp/game",
		SessionStart:      sessionStart,
		SelfPID:           1,
	}, nil)
	if !ok || got.PID != 404 || got.Name != "ReExecGame" {
		t.Fatalf("DetectSuccessorProcess() = %+v, %v; want ReExecGame (PID 404)", got, ok)
	}
}
