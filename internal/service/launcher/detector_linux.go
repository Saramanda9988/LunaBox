//go:build linux

package launcher

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lunabox/internal/utils/processutils"
)

func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	launcher := input.Launcher
	deadline := processDetectionDeadline(input)
	reliableSavedProcess := HasReliableSavedProcessName(input.SavedProcessName, input.LauncherExeName)

	if reliableSavedProcess {
		logInfo(logger, "Game %s has saved process_name: %s, will search for it during process detection", input.GameID, input.SavedProcessName)
	}

	logInfo(logger, "Starting Linux staged detection for game %s until %s, launcher: %s (PID %d)", input.GameID, deadline.Format(time.RFC3339), launcher.Name, launcher.PID)
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
		if detected, ok := detectLaunchedGameProcess(input, attemptLogger); ok && detected.PID != launcher.PID {
			if attemptLogger == nil {
				logInfo(logger, "Detected Linux game process for game %s: %s (PID %d)", input.GameID, detected.Name, detected.PID)
			}
			return resultForExternalProcess(input, detected, true)
		}
		if !processutils.IsProcessPresentByPID(launcher.PID) && !launcherExitLogged {
			logInfo(logger, "Launcher %s exited during process detection; continuing to scan for the actual game process", input.LauncherExeName)
			launcherExitLogged = true
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
	reliableSavedProcess := HasReliableSavedProcessName(input.SavedProcessName, "steam")
	logInfo(logger, "Starting Linux Steam directory detection for game %s until %s, install dir: %s", input.GameID, deadline.Format(time.RFC3339), input.LaunchDir)
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
		if detected, ok := detectProcessInSteamDir(input, attemptLogger); ok {
			if attemptLogger == nil {
				logInfo(logger, "Detected Linux Steam game process for game %s: %s (PID %d)", input.GameID, detected.Name, detected.PID)
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

func detectLaunchedDescendantProcess(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	descendants, err := processutils.GetDescendantProcessDetails(input.Launcher.PID)
	if err != nil {
		logWarning(logger, "Failed to enumerate descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
		return processutils.ProcessInfo{}, false
	}

	candidates := linuxCandidatesFromDetails(descendants, true, false)
	return pickLinuxProcessCandidate(candidates, input, "descendant", logger)
}

func detectProcessInLaunchDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := launchDirProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return pickLinuxProcessCandidate(candidates, input, "launch dir", logger)
}

func detectProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamGameProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return pickLinuxProcessCandidate(candidates, input, "Steam game", logger)
}

func detectSingleStableProcessInSteamDir(input StagedProcessDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	candidates, err := steamGameProcessCandidates(input, logger)
	if err != nil || len(candidates) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return pickLinuxProcessCandidate(candidates, input, "Steam game", logger)
}

type linuxProcessCandidate struct {
	detail         processutils.ProcessDetails
	fromDescendant bool
	fromDirectory  bool
}

type scoredLinuxProcessCandidate struct {
	candidate linuxProcessCandidate
	score     int
}

func linuxCandidatesFromDetails(details []processutils.ProcessDetails, fromDescendant bool, fromDirectory bool) []linuxProcessCandidate {
	candidates := make([]linuxProcessCandidate, 0, len(details))
	for _, detail := range details {
		candidates = append(candidates, linuxProcessCandidate{
			detail:         detail,
			fromDescendant: fromDescendant,
			fromDirectory:  fromDirectory,
		})
	}
	return candidates
}

func linuxMergeProcessCandidates(groups ...[]linuxProcessCandidate) []linuxProcessCandidate {
	byPID := make(map[uint32]linuxProcessCandidate)
	for _, group := range groups {
		for _, candidate := range group {
			pid := candidate.detail.PID
			if pid == 0 {
				continue
			}
			existing, ok := byPID[pid]
			if !ok {
				byPID[pid] = candidate
				continue
			}
			existing.fromDescendant = existing.fromDescendant || candidate.fromDescendant
			existing.fromDirectory = existing.fromDirectory || candidate.fromDirectory
			if existing.detail.ExecutablePath == "" {
				existing.detail.ExecutablePath = candidate.detail.ExecutablePath
			}
			if existing.detail.CurrentDirectory == "" {
				existing.detail.CurrentDirectory = candidate.detail.CurrentDirectory
			}
			if len(existing.detail.CommandLine) == 0 {
				existing.detail.CommandLine = candidate.detail.CommandLine
			}
			byPID[pid] = existing
		}
	}

	candidates := make([]linuxProcessCandidate, 0, len(byPID))
	for _, candidate := range byPID {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].detail.PID < candidates[j].detail.PID
	})
	return candidates
}

func pickLinuxProcessCandidate(candidates []linuxProcessCandidate, input StagedProcessDetectionInput, source string, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	targetNames := linuxTargetProcessNames(input, candidates)
	scored := make([]scoredLinuxProcessCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		score := linuxProcessCandidateScore(candidate, input, targetNames)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredLinuxProcessCandidate{
			candidate: candidate,
			score:     score,
		})
	}

	if len(scored) == 0 {
		return processutils.ProcessInfo{}, false
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].candidate.detail.PID < scored[j].candidate.detail.PID
		}
		return scored[i].score > scored[j].score
	})

	top := scored[0]
	if len(scored) > 1 && scored[1].score == top.score {
		tied := make([]processutils.ProcessInfo, 0, len(scored))
		for _, candidate := range scored {
			if candidate.score != top.score {
				break
			}
			tied = append(tied, candidate.candidate.detail.ProcessInfo)
		}
		logInfo(logger, "Multiple %s candidates found for game %s with same confidence, requiring manual selection: %s", source, input.GameID, FormatProcessCandidates(tied))
		return processutils.ProcessInfo{}, false
	}

	proc := top.candidate.detail.ProcessInfo
	logInfo(logger, "Auto-detected %s process for game %s: %s (PID %d, score %d)", source, input.GameID, proc.Name, proc.PID, top.score)
	return proc, true
}

func linuxProcessCandidateScore(candidate linuxProcessCandidate, input StagedProcessDetectionInput, targetNames []string) int {
	detail := candidate.detail
	if detail.PID == 0 || detail.PID == input.Launcher.PID || linuxIsLikelyHelperProcess(detail) {
		return 0
	}
	nameScore := linuxProcessNameMatchScore(detail.Name, targetNames)
	pathScore := linuxProcessPathScore(detail, input.LaunchDir)
	if nameScore == 0 && pathScore == 0 {
		return 0
	}

	score := nameScore + pathScore
	if candidate.fromDescendant {
		score += 20
	}
	if candidate.fromDirectory {
		score += 20
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(detail.Name)), ".exe") {
		score += 10
	}
	return score
}

func linuxProcessNameMatchScore(processName string, targetNames []string) int {
	processName = strings.ToLower(strings.TrimSpace(processName))
	if processName == "" {
		return 0
	}
	for _, targetName := range targetNames {
		targetName = strings.ToLower(strings.TrimSpace(filepath.Base(targetName)))
		if targetName == "" {
			continue
		}
		if processName == targetName {
			return 120
		}
		if processName == linuxTruncatedCommName(targetName) {
			return 100
		}
		withoutExt := strings.TrimSuffix(targetName, filepath.Ext(targetName))
		if withoutExt != "" && processName == withoutExt {
			return 80
		}
	}
	return 0
}

func linuxTruncatedCommName(name string) string {
	name = strings.TrimSpace(name)
	if len([]byte(name)) <= 15 {
		return strings.ToLower(name)
	}
	return strings.ToLower(string([]byte(name)[:15]))
}

func linuxProcessPathScore(detail processutils.ProcessDetails, launchDir string) int {
	score := 0
	if linuxPathUnderDir(detail.ExecutablePath, launchDir) {
		score += 70
	}
	if linuxPathUnderDir(detail.CurrentDirectory, launchDir) {
		score += 50
	}
	for _, arg := range detail.CommandLine {
		if linuxArgReferencesDir(arg, launchDir) {
			score += 40
			break
		}
	}
	return score
}

func linuxTargetProcessNames(input StagedProcessDetectionInput, candidates []linuxProcessCandidate) []string {
	names := make([]string, 0, 4)
	seen := make(map[string]bool)
	add := func(name string) {
		name = strings.TrimSpace(filepath.Base(linuxNormalizeArgumentPath(name)))
		if name == "" || IsLikelyHelperProcess(name) {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		names = append(names, name)
	}

	add(input.SavedProcessName)
	add(input.Launcher.Name)
	for _, candidate := range candidates {
		for _, arg := range candidate.detail.CommandLine {
			if !linuxArgReferencesDir(arg, input.LaunchDir) {
				continue
			}
			name := filepath.Base(linuxNormalizeArgumentPath(arg))
			switch strings.ToLower(filepath.Ext(name)) {
			case ".exe", ".bat":
				add(name)
			}
		}
	}
	return names
}

func linuxIsLikelyHelperProcess(detail processutils.ProcessDetails) bool {
	if IsLikelyHelperProcess(detail.Name) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(detail.Name))
	if name == "bash" || name == "sh" || name == "python" || strings.HasPrefix(name, "python") {
		return linuxCommandLineContainsProton(detail.CommandLine)
	}
	return false
}

func linuxCommandLineContainsProton(commandLine []string) bool {
	for _, arg := range commandLine {
		arg = strings.ToLower(filepath.ToSlash(arg))
		if strings.Contains(arg, "/proton") || strings.Contains(arg, "proton ") || strings.HasSuffix(arg, "proton") {
			return true
		}
	}
	return false
}

func linuxNormalizeArgumentPath(arg string) string {
	arg = strings.Trim(strings.TrimSpace(arg), "\"'")
	arg = strings.ReplaceAll(arg, "\\", "/")
	if len(arg) >= 3 && arg[1] == ':' && arg[2] == '/' {
		arg = arg[2:]
	}
	return arg
}

func linuxArgReferencesDir(arg string, rootDir string) bool {
	arg = linuxNormalizeArgumentPath(arg)
	if arg == "" {
		return false
	}
	if linuxPathUnderDir(arg, rootDir) {
		return true
	}
	rootDir = filepath.ToSlash(strings.TrimSpace(rootDir))
	return rootDir != "" && strings.Contains(filepath.ToSlash(arg), rootDir)
}

func linuxPathUnderDir(path string, rootDir string) bool {
	path = linuxNormalizeArgumentPath(path)
	rootDir = strings.TrimSpace(rootDir)
	if path == "" || rootDir == "" || !filepath.IsAbs(path) {
		return false
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(filepath.Clean(rootDir))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func launchDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]linuxProcessCandidate, error) {
	details, err := processutils.GetProcessDetailsByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate processes in launch dir %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}

	filtered := linuxCandidatesFromDetails(details, false, true)
	if len(filtered) == 0 {
		logInfo(logger, "No process candidate found in launch dir %s for game %s", input.LaunchDir, input.GameID)
	}
	return filtered, nil
}

func steamGameProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]linuxProcessCandidate, error) {
	candidateGroups := make([][]linuxProcessCandidate, 0, 2)
	if descendants, err := processutils.GetDescendantProcessDetails(input.Launcher.PID); err == nil {
		candidateGroups = append(candidateGroups, linuxCandidatesFromDetails(descendants, true, false))
	} else {
		logWarning(logger, "Failed to enumerate Steam descendant processes for launcher %s (PID %d): %v", input.LauncherExeName, input.Launcher.PID, err)
	}

	dirCandidates, err := steamDirProcessCandidates(input, logger)
	if err != nil {
		candidates := linuxMergeProcessCandidates(candidateGroups...)
		if len(candidates) > 0 {
			return candidates, nil
		}
		return candidates, err
	}
	candidateGroups = append(candidateGroups, dirCandidates)

	candidates := linuxMergeProcessCandidates(candidateGroups...)
	if len(candidates) == 0 {
		logInfo(logger, "No Steam game process found for game %s under launcher %s (PID %d) or install dir %s", input.GameID, input.LauncherExeName, input.Launcher.PID, input.LaunchDir)
	}
	return candidates, nil
}

func steamDirProcessCandidates(input StagedProcessDetectionInput, logger DetectionLogger) ([]linuxProcessCandidate, error) {
	details, err := processutils.GetProcessDetailsByExecutableDir(input.LaunchDir)
	if err != nil {
		logWarning(logger, "Failed to enumerate Steam game processes in %s for game %s: %v", input.LaunchDir, input.GameID, err)
		return nil, err
	}

	filtered := linuxCandidatesFromDetails(details, false, true)
	if len(filtered) == 0 {
		logInfo(logger, "No Steam process candidate found in install dir %s for game %s", input.LaunchDir, input.GameID)
	}
	return filtered, nil
}

var successorGraceDelays = []time.Duration{0, 1 * time.Second, 2 * time.Second, 3 * time.Second}

const successorStartupPhase = 60 * time.Second

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
	return processutils.ProcessInfo{}, false
}

func findSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	var dirCandidates []processutils.ProcessInfo
	if strings.TrimSpace(input.LaunchDir) != "" {
		if dirProcs, err := processutils.GetProcessesByExecutableDir(input.LaunchDir); err == nil {
			dirCandidates = filterSuccessorCandidates(dirProcs, input, logger)
		}
	}

	descendantPIDs := make(map[uint32]bool)
	if descendants, err := processutils.GetDescendantProcesses(input.ExitedPID); err == nil {
		for _, proc := range descendants {
			descendantPIDs[proc.PID] = true
		}
	}

	descendantDirCandidates := make([]processutils.ProcessInfo, 0, len(dirCandidates))
	for _, proc := range dirCandidates {
		if descendantPIDs[proc.PID] {
			descendantDirCandidates = append(descendantDirCandidates, proc)
		}
	}
	if proc, ok := pickSuccessorCandidate(descendantDirCandidates); ok {
		return proc, true
	}

	if proc, ok := pickSuccessorCandidate(dirCandidates); ok {
		return proc, true
	}

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
		if IsLikelyHelperProcess(proc.Name) || !startedWithinSession(proc, input, logger) {
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
	created, err := processutils.GetProcessCreationTime(proc.PID)
	if err != nil {
		logInfo(logger, "Cannot read creation time of successor candidate %s (PID %d) for game %s, keeping it: %v", proc.Name, proc.PID, input.GameID, err)
		return true
	}
	return !created.Before(input.SessionStart.Add(-2 * time.Second))
}

func pickSuccessorCandidate(candidates []processutils.ProcessInfo) (processutils.ProcessInfo, bool) {
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
