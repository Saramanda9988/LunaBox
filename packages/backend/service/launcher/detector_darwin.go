//go:build darwin

package launcher

import (
	"lunabox/backend/utils/processutils"
	"lunabox/backend/utils/timerutils/focusing"
	"os"
	"strings"
	"time"
)

var (
	darwinSteamProcessLookup           = processutils.GetProcessesByExecutableDir
	darwinProcessesByNameLookup        = processutils.GetProcessesByName
	darwinProcessPIDByNameLookup       = processutils.GetProcessPIDByName
	darwinDescendantProcessLookup      = processutils.GetDescendantProcesses
	darwinProcessCreationTime          = processutils.GetProcessCreationTime
	darwinVisibleWindowPIDs            = processutils.VisibleWindowPIDs
	darwinFilterVisibleWindowProcesses = processutils.FilterProcessesWithVisibleWindows
	darwinForegroundProcessID          = focusing.GetForegroundProcessID

	successorGraceDelays = []time.Duration{0, 1 * time.Second, 2 * time.Second, 3 * time.Second}
)

const successorStartupPhase = 60 * time.Second

// DetectStagedProcess keeps native macOS launch strategies on launcher-only
// monitoring. Steam uses its dedicated install-directory handoff below.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Staged process detection is not supported on macOS for game %s, using launcher process: %s (PID %d)", input.GameID, input.Launcher.Name, input.Launcher.PID)
	return resultForLauncher(input)
}

// DetectSteamDirectoryProcess resolves the game process Steam started from the
// game's install directory (or saved process name), scoring candidates using
// directory/name relation, CoreGraphics visible top-level windows, and the
// NSWorkspace frontmost application PID.
func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	return detectScoredProcess(input, scoredDetection{
		label:   "macOS Steam directory",
		steam:   true,
		sampler: newDarwinSteamSampler(input, logger),
	}, logger)
}

// darwinSteamSampler builds one tick of candidate evidence from the install
// directory, saved process name, CoreGraphics visible windows, and frontmost
// application PID.
type darwinSteamSampler struct {
	input     StagedProcessDetectionInput
	savedName string
	selfPID   uint32
}

func newDarwinSteamSampler(input StagedProcessDetectionInput, logger DetectionLogger) *darwinSteamSampler {
	savedName := ""
	if HasReliableSavedProcessName(input.SavedProcessName, "steam_osx") {
		savedName = strings.TrimSpace(input.SavedProcessName)
		logInfo(logger, "Game %s has saved process_name: %s, will commit to it as soon as it runs", input.GameID, savedName)
	}
	return &darwinSteamSampler{
		input:     input,
		savedName: savedName,
		selfPID:   uint32(os.Getpid()),
	}
}

func (s *darwinSteamSampler) sample() ([]candidateSample, error) {
	var dirProcesses []processutils.ProcessInfo
	if strings.TrimSpace(s.input.LaunchDir) != "" {
		var err error
		dirProcesses, err = darwinSteamProcessLookup(s.input.LaunchDir)
		if err != nil {
			return nil, err
		}
	}

	var savedProcesses []processutils.ProcessInfo
	if s.savedName != "" {
		if matched, err := darwinProcessesByNameLookup(s.savedName); err == nil {
			savedProcesses = matched
		}
	}

	if len(dirProcesses) == 0 && len(savedProcesses) == 0 {
		return nil, nil
	}

	windowPIDs, windowKnown := darwinVisibleWindowPIDs()
	foregroundPID, foregroundKnown := darwinForegroundProcessID()
	return darwinSteamSamples(
		dirProcesses,
		savedProcesses,
		s.savedName,
		s.selfPID,
		windowPIDs,
		windowKnown,
		foregroundPID,
		foregroundKnown,
	), nil
}

func darwinSteamSamples(
	dirProcesses []processutils.ProcessInfo,
	savedProcesses []processutils.ProcessInfo,
	savedProcessName string,
	selfPID uint32,
	windowPIDs map[uint32]bool,
	windowKnown bool,
	foregroundPID uint32,
	foregroundKnown bool,
) []candidateSample {
	savedName := strings.TrimSpace(savedProcessName)
	links := make(map[uint32]*processLinks)
	infos := make([]processutils.ProcessInfo, 0, len(dirProcesses)+len(savedProcesses))

	add := func(proc processutils.ProcessInfo) *processLinks {
		if proc.PID == 0 || proc.PID == selfPID {
			return nil
		}
		if link, ok := links[proc.PID]; ok {
			return link
		}
		if strings.EqualFold(proc.Name, "steam_osx") || IsLikelyHelperProcess(proc.Name) {
			return nil
		}
		link := &processLinks{DirIsPrimary: true}
		links[proc.PID] = link
		infos = append(infos, proc)
		return link
	}

	for _, proc := range dirProcesses {
		if link := add(proc); link != nil {
			link.InLaunchDir = true
			if savedName != "" && strings.EqualFold(proc.Name, savedName) {
				link.SavedName = true
			}
		}
	}
	for _, proc := range savedProcesses {
		if link := add(proc); link != nil {
			link.SavedName = true
		}
	}

	samples := make([]candidateSample, 0, len(infos))
	for _, proc := range infos {
		link := links[proc.PID]
		samples = append(samples, candidateSample{
			Process:         proc,
			SavedName:       link.SavedName,
			Relation:        link.relation(),
			Window:          windowKnown && windowPIDs[proc.PID],
			WindowKnown:     windowKnown,
			Foreground:      foregroundKnown && proc.PID == foregroundPID,
			ForegroundKnown: foregroundKnown,
		})
	}
	return samples
}

// DetectSuccessorProcess looks for a macOS process that took over from an
// exited monitored process (e.g. a launcher or splash window handing off to
// the real game, or an in-game restart).
func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	delays := successorGraceDelays
	if !input.SessionStart.IsZero() && time.Since(input.SessionStart) >= successorStartupPhase {
		delays = successorGraceDelays[:1]
	}

	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}
		if proc, ok := findSuccessorProcess(input, logger); ok {
			logInfo(logger, "Detected successor process for game %s on attempt %d: %s (PID %d), previous: %s (PID %d)", input.GameID, attempt+1, proc.Name, proc.PID, input.ExitedProcessName, input.ExitedPID)
			return proc, true
		}
	}
	logInfo(logger, "No successor process found for game %s after %s (PID %d) exited, treating as game shutdown", input.GameID, input.ExitedProcessName, input.ExitedPID)
	return processutils.ProcessInfo{}, false
}

func findSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	var dirCandidates []processutils.ProcessInfo
	if strings.TrimSpace(input.LaunchDir) != "" {
		if dirProcs, err := darwinSteamProcessLookup(input.LaunchDir); err == nil {
			dirCandidates = filterSuccessorCandidates(dirProcs, input, logger)
		}
	}

	descendantPIDs := make(map[uint32]bool)
	if descendants, err := darwinDescendantProcessLookup(input.ExitedPID); err == nil {
		for _, proc := range descendants {
			descendantPIDs[proc.PID] = true
		}
	}

	// Strongest evidence first: spawned by the exited process AND running from
	// the game's directory.
	descendantDirCandidates := make([]processutils.ProcessInfo, 0, len(dirCandidates))
	for _, proc := range dirCandidates {
		if descendantPIDs[proc.PID] {
			descendantDirCandidates = append(descendantDirCandidates, proc)
		}
	}
	if proc, ok := pickSuccessorCandidate(descendantDirCandidates); ok {
		return proc, true
	}

	// Then any process from the game's directory (e.g. LaunchServices / open
	// broker hand-off without a direct parent-child link).
	if proc, ok := pickSuccessorCandidate(dirCandidates); ok {
		return proc, true
	}

	// Finally same-name matches anywhere for self re-exec / unpacked locations.
	for _, name := range successorNameCandidates(input) {
		pid, err := darwinProcessPIDByNameLookup(name)
		if err != nil || pid == 0 || pid == input.ExitedPID || pid == input.SelfPID {
			continue
		}
		proc := processutils.ProcessInfo{Name: name, PID: pid}
		if startedWithinSession(proc, input, logger) {
			return proc, true
		}
	}

	return processutils.ProcessInfo{}, false
}

func filterSuccessorCandidates(processes []processutils.ProcessInfo, input SuccessorDetectionInput, logger DetectionLogger) []processutils.ProcessInfo {
	candidates := make([]processutils.ProcessInfo, 0, len(processes))
	for _, proc := range processes {
		if proc.PID == 0 || proc.PID == input.ExitedPID || proc.PID == input.SelfPID {
			continue
		}
		if strings.EqualFold(proc.Name, "steam_osx") || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		if !startedWithinSession(proc, input, logger) {
			continue
		}
		candidates = append(candidates, proc)
	}
	return candidates
}

func startedWithinSession(proc processutils.ProcessInfo, input SuccessorDetectionInput, logger DetectionLogger) bool {
	if input.SessionStart.IsZero() {
		return true
	}
	created, err := darwinProcessCreationTime(proc.PID)
	if err != nil {
		logInfo(logger, "Cannot read creation time of successor candidate %s (PID %d) for game %s, keeping it: %v", proc.Name, proc.PID, input.GameID, err)
		return true
	}
	return !created.Before(input.SessionStart.Add(-sessionStartTolerance))
}

func pickSuccessorCandidate(candidates []processutils.ProcessInfo) (processutils.ProcessInfo, bool) {
	if len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := darwinForegroundProcessID(); ok {
		for _, proc := range candidates {
			if proc.PID == foregroundPID {
				return proc, true
			}
		}
	}

	windowCandidates := darwinFilterVisibleWindowProcesses(candidates)
	if len(windowCandidates) == 1 {
		return windowCandidates[0], true
	}
	if len(windowCandidates) > 1 {
		return processutils.ProcessInfo{}, false
	}

	if len(candidates) == 1 && IsPersistableProcessName(candidates[0].Name) {
		return candidates[0], true
	}
	return processutils.ProcessInfo{}, false
}

func successorNameCandidates(input SuccessorDetectionInput) []string {
	names := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	for _, name := range []string{input.SavedProcessName, input.ExitedProcessName} {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || !IsPersistableProcessName(trimmed) {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, trimmed)
	}
	return names
}
