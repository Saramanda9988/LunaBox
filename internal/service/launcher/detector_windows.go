//go:build windows

package launcher

import (
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/timerutils/focusing"
	"strings"
	"time"
)

// DetectStagedProcess resolves the actual game process behind a Windows launcher.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	launcher := input.Launcher
	deadline := processDetectionDeadline(input)
	reliableSavedProcess := HasReliableSavedProcessName(input.SavedProcessName, input.LauncherExeName)

	if reliableSavedProcess {
		logInfo(logger, "Game %s has saved process_name: %s, will search for it during process detection", input.GameID, input.SavedProcessName)
	}

	logInfo(logger, "Starting staged detection for game %s until %s, launcher: %s (PID %d)", input.GameID, deadline.Format(time.RFC3339), launcher.Name, launcher.PID)
	if !waitForProcessDetection(deadline, 5*time.Second, input.Done) {
		return StagedProcessDetectionResult{}
	}

	savedLookupFailureLogged := false
	launcherExitLogged := false
	attempt := 0
	for {
		attemptLogger := processDetectionAttemptLogger(logger, attempt, 15)
		if reliableSavedProcess {
			pid, err := processutils.GetProcessPIDByName(input.SavedProcessName)
			if err == nil {
				logInfo(logger, "Found saved process %s with PID %d", input.SavedProcessName, pid)
				return StagedProcessDetectionResult{
					ProcessID:           pid,
					ProcessName:         input.SavedProcessName,
					CloseLauncherHandle: true,
				}
			}
			if !savedLookupFailureLogged {
				logWarning(logger, "Saved process %s is not running yet: %v; continuing detection", input.SavedProcessName, err)
				savedLookupFailureLogged = true
			}
		}

		if detected, ok := detectVisibleGameProcess(input, attemptLogger); ok {
			if attemptLogger == nil {
				logInfo(logger, "Detected game process for game %s during staged detection: %s (PID %d)", input.GameID, detected.Name, detected.PID)
			}
			return resultForExternalProcess(input, detected, true)
		}

		if !processutils.IsProcessPresentByPID(launcher.PID) {
			if !launcherExitLogged {
				logInfo(logger, "Launcher %s exited during process detection; continuing to scan for the actual game process", input.LauncherExeName)
				launcherExitLogged = true
			}
			if detected, ok := detectLaunchedGameProcess(input, attemptLogger); ok {
				if attemptLogger == nil {
					logInfo(logger, "Detected game process for game %s after launcher exit: %s (PID %d)", input.GameID, detected.Name, detected.PID)
				}
				return resultForExternalProcess(input, detected, true)
			}
		}

		attempt++
		if !waitForProcessDetection(deadline, 2*time.Second, input.Done) {
			break
		}
	}

	if processDetectionCancelled(input.Done) {
		return StagedProcessDetectionResult{}
	}
	if processutils.IsProcessPresentByPID(launcher.PID) {
		logInfo(logger, "Process detection timed out for game %s; monitoring launcher %s (PID %d)", input.GameID, input.LauncherExeName, launcher.PID)
		return resultForLauncher(input)
	}
	logWarning(logger, "Process detection timed out for game %s after launcher %s exited", input.GameID, input.LauncherExeName)
	return promptProcessSelectionResult()
}

func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	deadline := processDetectionDeadline(input)
	reliableSavedProcess := HasReliableSavedProcessName(input.SavedProcessName, "steam.exe")
	logInfo(logger, "Starting Steam directory detection for game %s until %s, install dir: %s", input.GameID, deadline.Format(time.RFC3339), input.LaunchDir)
	if !waitForProcessDetection(deadline, 5*time.Second, input.Done) {
		return StagedProcessDetectionResult{}
	}

	savedLookupFailureLogged := false
	attempt := 0
	for {
		attemptLogger := processDetectionAttemptLogger(logger, attempt, 15)
		if reliableSavedProcess {
			pid, err := processutils.GetProcessPIDByName(input.SavedProcessName)
			if err == nil {
				logInfo(logger, "Found saved Steam game process %s with PID %d", input.SavedProcessName, pid)
				return StagedProcessDetectionResult{
					ProcessID:           pid,
					ProcessName:         input.SavedProcessName,
					CloseLauncherHandle: true,
				}
			}
			if !savedLookupFailureLogged {
				logWarning(logger, "Saved Steam game process %s is not running yet: %v; continuing detection", input.SavedProcessName, err)
				savedLookupFailureLogged = true
			}
		}
		if detected, ok := detectVisibleProcessInSteamDir(input, attemptLogger); ok {
			if attemptLogger == nil {
				logInfo(logger, "Detected Steam game process for game %s: %s (PID %d)", input.GameID, detected.Name, detected.PID)
			}
			return resultForSteamProcess(input, detected)
		}
		attempt++
		if !waitForProcessDetection(deadline, 2*time.Second, input.Done) {
			break
		}
	}

	if processDetectionCancelled(input.Done) {
		return StagedProcessDetectionResult{}
	}
	if detected, ok := detectSingleStableProcessInSteamDir(input, logger); ok {
		return resultForSteamProcess(input, detected)
	}

	logWarning(logger, "Steam directory detection failed for game %s, requiring manual process selection", input.GameID)
	return promptProcessSelectionResult()
}

func resultForSteamProcess(input StagedProcessDetectionInput, proc processutils.ProcessInfo) StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		ProcessID:           proc.PID,
		ProcessName:         proc.Name,
		CloseLauncherHandle: true,
		PersistProcessName:  ProcessNameForPersistence("", proc.Name),
	}
}

func detectLaunchedGameProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	if proc, ok := detectLaunchedDescendantProcess(input, logger); ok {
		return proc, true
	}
	return detectProcessInLaunchDir(input, logger)
}

func detectVisibleGameProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	if proc, ok := detectVisibleLaunchedDescendantProcess(input, logger); ok && proc.PID != input.Launcher.PID {
		logInfo(logger, "Detected visible game window process for game %s: %s (PID %d), launcher: %s (PID %d)", input.GameID, proc.Name, proc.PID, input.LauncherExeName, input.Launcher.PID)
		return proc, true
	}
	if proc, ok := detectVisibleProcessInLaunchDir(input, logger); ok && proc.PID != input.Launcher.PID {
		logInfo(logger, "Detected visible game window process for game %s: %s (PID %d), launcher: %s (PID %d)", input.GameID, proc.Name, proc.PID, input.LauncherExeName, input.Launcher.PID)
		return proc, true
	}
	return processutils.ProcessInfo{}, false
}

func detectVisibleLaunchedDescendantProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	descendants, err := processutils.GetDescendantProcesses(input.Launcher.PID)
	if err != nil {
		logWarning(logger, "Failed to enumerate visible descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
		return processutils.ProcessInfo{}, false
	}
	if len(descendants) == 0 {
		return processutils.ProcessInfo{}, false
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}
	if len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID {
				logInfo(logger, "Auto-detected foreground visible-window descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible-window descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	logInfo(logger, "Multiple visible-window descendant candidates found for game %s, requiring more evidence: %s", input.GameID, FormatProcessCandidates(windowCandidates))
	return processutils.ProcessInfo{}, false
}

func detectLaunchedDescendantProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	descendants, err := processutils.GetDescendantProcesses(input.Launcher.PID)
	if err != nil {
		logWarning(logger, "Failed to enumerate descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
		return processutils.ProcessInfo{}, false
	}
	if len(descendants) == 0 {
		logInfo(logger, "No descendant process found for launcher %s (PID %d)", input.LauncherExeName, input.Launcher.PID)
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range descendants {
			if proc.PID == foregroundPID && !IsLikelyHelperProcess(proc.Name) {
				logInfo(logger, "Auto-detected foreground descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	candidates := make([]processutils.ProcessInfo, 0, len(descendants))
	for _, proc := range descendants {
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		candidates = append(candidates, proc)
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible-window descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	if len(candidates) == 1 {
		proc := candidates[0]
		if IsPersistableProcessName(proc.Name) {
			logInfo(logger, "Auto-detected stable descendant process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
			return proc, true
		}
		logInfo(logger, "Single non-exe descendant process found for game %s without visible window, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
		return processutils.ProcessInfo{}, false
	}

	if len(candidates) > 1 {
		logInfo(logger, "Multiple descendant process candidates found for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
	} else {
		logInfo(logger, "Only helper descendant processes found for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(descendants))
	}
	return processutils.ProcessInfo{}, false
}

func detectVisibleProcessInLaunchDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := launchDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID && proc.PID != input.Launcher.PID {
				logInfo(logger, "Auto-detected foreground visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	nonLauncherWindowCandidates := make([]processutils.ProcessInfo, 0, len(windowCandidates))
	for _, proc := range windowCandidates {
		if proc.PID == input.Launcher.PID {
			continue
		}
		nonLauncherWindowCandidates = append(nonLauncherWindowCandidates, proc)
	}

	if len(nonLauncherWindowCandidates) == 1 {
		proc := nonLauncherWindowCandidates[0]
		logInfo(logger, "Auto-detected visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	if len(nonLauncherWindowCandidates) > 1 {
		logInfo(logger, "Multiple visible-window launch dir candidates found for game %s, requiring more evidence: %s", input.GameID, FormatProcessCandidates(nonLauncherWindowCandidates))
		return processutils.ProcessInfo{}, false
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Only launcher has a visible window for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	return processutils.ProcessInfo{}, false
}

func detectVisibleProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range windowCandidates {
			if proc.PID == foregroundPID {
				logInfo(logger, "Auto-detected foreground Steam game process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible Steam game process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	logInfo(logger, "Multiple visible Steam game candidates found for game %s: %s", input.GameID, FormatProcessCandidates(windowCandidates))
	return processutils.ProcessInfo{}, false
}

func detectSingleStableProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	if len(candidates) == 1 && IsPersistableProcessName(candidates[0].Name) {
		proc := candidates[0]
		logInfo(logger, "Auto-detected single stable Steam game process for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}
	logInfo(logger, "Steam game process candidates require manual selection for game %s: %s", input.GameID, FormatProcessCandidates(candidates))
	return processutils.ProcessInfo{}, false
}

func detectProcessInLaunchDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := launchDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range candidates {
			if proc.PID == foregroundPID && !IsLikelyHelperProcess(proc.Name) {
				logInfo(logger, "Auto-detected foreground process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
				return proc, true
			}
		}
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		proc := windowCandidates[0]
		logInfo(logger, "Auto-detected visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	nonLauncherWindowCandidates := make([]processutils.ProcessInfo, 0, len(windowCandidates))
	for _, proc := range windowCandidates {
		if proc.PID == input.Launcher.PID {
			continue
		}
		nonLauncherWindowCandidates = append(nonLauncherWindowCandidates, proc)
	}
	if len(nonLauncherWindowCandidates) == 1 {
		proc := nonLauncherWindowCandidates[0]
		logInfo(logger, "Auto-detected non-launcher visible-window process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
		return proc, true
	}

	if len(candidates) == 1 {
		proc := candidates[0]
		if IsPersistableProcessName(proc.Name) {
			logInfo(logger, "Auto-detected stable process in launch dir for game %s: %s (PID %d)", input.GameID, proc.Name, proc.PID)
			return proc, true
		}
		logInfo(logger, "Single non-exe launch dir process found for game %s without visible window, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
		return processutils.ProcessInfo{}, false
	}

	if len(candidates) > 1 {
		logInfo(logger, "Multiple launch dir process candidates found for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
	}
	return processutils.ProcessInfo{}, false
}

func launchDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]processutils.ProcessInfo, error) {
	candidates, err := processutils.GetProcessesByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate processes in launch dir %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}
	if len(candidates) == 0 {
		logInfo(logger, "No running process found in launch dir %s for game %s", input.LaunchDir, input.GameID)
		return nil, nil
	}

	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, proc := range candidates {
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}

	if len(filtered) == 0 {
		logInfo(logger, "Only helper processes found in launch dir for game %s, requiring manual selection: %s", input.GameID, FormatProcessCandidates(candidates))
	}
	return filtered, nil
}

// successorGraceDelays paces successor detection after a monitored process
// exits during the session start-up phase. The immediate check catches the
// common splash→game hand-off where the real game is spawned before the splash
// closes; the later checks cover the exec gap where the old process dies
// slightly before its successor appears.
var successorGraceDelays = []time.Duration{0, 1 * time.Second, 2 * time.Second, 3 * time.Second}

// successorStartupPhase bounds the start-up window in which splash/launcher
// hand-offs normally happen. It matches the minimum session duration: sessions
// shorter than this are deleted anyway, so the grace waits cannot distort any
// recorded play time.
const successorStartupPhase = 60 * time.Second

// DetectSuccessorProcess looks for a process that took over from an exited
// monitored process: a splash window that spawned the real game, a launcher
// that was mistakenly treated as the game, a self re-exec with the same exe
// name, or an in-game restart. It returns false when the exit looks like a
// genuine game shutdown.
func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	delays := successorGraceDelays
	if !input.SessionStart.IsZero() && time.Since(input.SessionStart) >= successorStartupPhase {
		// Past the start-up phase the game's window/process structure is
		// stable: a genuine exit leaves the game directory without processes.
		// A single immediate check keeps hand-off support for in-game restarts
		// without delaying session finalization.
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
	// A successor must live under the game's directory or reuse a known process
	// name. Bare descent from the exited process is NOT enough: games commonly
	// spawn a browser (survey/official site) right before exiting, and that
	// child must not be mistaken for the game.
	var dirCandidates []processutils.ProcessInfo
	if strings.TrimSpace(input.LaunchDir) != "" {
		if dirProcs, err := processutils.GetProcessesByExecutableDir(input.LaunchDir); err == nil {
			dirCandidates = filterSuccessorCandidates(dirProcs, input, logger)
		}
	}

	descendantPIDs := make(map[uint32]bool)
	if descendants, err := processutils.GetDescendantProcesses(input.ExitedPID); err == nil {
		// Snapshots keep the parent PID of the exited process, so children it
		// spawned before dying are still discoverable.
		for _, proc := range descendants {
			descendantPIDs[proc.PID] = true
		}
	}

	// Strongest evidence first: spawned by the exited process AND running from
	// the game's directory (splash window → real game).
	descendantDirCandidates := make([]processutils.ProcessInfo, 0, len(dirCandidates))
	for _, proc := range dirCandidates {
		if descendantPIDs[proc.PID] {
			descendantDirCandidates = append(descendantDirCandidates, proc)
		}
	}
	if proc, ok := pickSuccessorCandidate(descendantDirCandidates); ok {
		return proc, true
	}

	// Then any process from the game's directory (launcher hand-off without a
	// parent-child relationship, e.g. via a broker process).
	if proc, ok := pickSuccessorCandidate(dirCandidates); ok {
		return proc, true
	}

	// Finally same-name matches anywhere: covers re-exec where the real game
	// reuses the exe name but runs from an unpacked location.
	for _, name := range successorNameCandidates(input) {
		pid, err := processutils.GetProcessPIDByName(name)
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
		if IsLikelyHelperProcess(proc.Name) {
			continue
		}
		if !startedWithinSession(proc, input, logger) {
			continue
		}
		candidates = append(candidates, proc)
	}
	return candidates
}

// startedWithinSession rejects candidates created before the play session
// began (stale processes or PID reuse). Creation time can be unreadable for
// elevated games when the app itself is not elevated; keep those candidates so
// admin games do not lose hand-off support.
func startedWithinSession(proc processutils.ProcessInfo, input SuccessorDetectionInput, logger DetectionLogger) bool {
	if input.SessionStart.IsZero() {
		return true
	}
	created, err := processutils.GetProcessCreationTime(proc.PID)
	if err != nil {
		logInfo(logger, "Cannot read creation time of successor candidate %s (PID %d) for game %s, keeping it: %v", proc.Name, proc.PID, input.GameID, err)
		return true
	}
	return !created.Before(input.SessionStart.Add(-2 * time.Second))
}

func pickSuccessorCandidate(candidates []processutils.ProcessInfo) (processutils.ProcessInfo, bool) {
	if len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}

	if foregroundPID, ok := focusing.GetForegroundProcessID(); ok {
		for _, proc := range candidates {
			if proc.PID == foregroundPID {
				return proc, true
			}
		}
	}

	windowCandidates := processutils.FilterProcessesWithVisibleWindows(candidates)
	if len(windowCandidates) == 1 {
		return windowCandidates[0], true
	}
	if len(windowCandidates) > 1 {
		// Ambiguous: wait for a later grace attempt instead of guessing.
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

func steamDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]processutils.ProcessInfo, error) {
	candidates, err := processutils.GetProcessesByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate Steam game processes in %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}

	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, proc := range candidates {
		if strings.EqualFold(proc.Name, "steam.exe") || IsLikelyHelperProcess(proc.Name) {
			continue
		}
		filtered = append(filtered, proc)
	}
	if len(filtered) == 0 {
		logInfo(logger, "No non-Steam game process found in install dir %s for game %s", input.LaunchDir, input.GameID)
	}
	return filtered, nil
}
