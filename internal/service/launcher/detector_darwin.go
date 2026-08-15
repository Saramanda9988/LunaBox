//go:build darwin

package launcher

import (
	"lunabox/internal/utils/processutils"
	"strings"
	"time"
)

var darwinSteamProcessLookup = processutils.GetProcessesByExecutableDir

// DetectStagedProcess keeps native macOS launch strategies on launcher-only
// monitoring. Steam uses its dedicated install-directory handoff below.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Staged process detection is not supported on macOS for game %s, using launcher process: %s (PID %d)", input.GameID, input.Launcher.Name, input.Launcher.PID)
	return resultForLauncher(input)
}

func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	deadline := processDetectionDeadline(input)
	const checkInterval = time.Second
	logInfo(logger, "Starting macOS Steam directory detection for game %s until %s, install dir: %s", input.GameID, deadline.Format(time.RFC3339), input.LaunchDir)
	attempt := 0
	for {
		candidates, err := darwinSteamProcessLookup(input.LaunchDir)
		if err != nil {
			logWarning(processDetectionAttemptLogger(logger, attempt, 30), "Failed to enumerate macOS Steam game processes in %s for game %s: %v", input.LaunchDir, input.GameID, err)
		} else if process, ok := selectDarwinSteamProcess(candidates, input.SavedProcessName); ok {
			logInfo(logger, "Detected macOS Steam game process %s (PID %d) for game %s", process.Name, process.PID, input.GameID)
			return resultForExternalProcess(input, process, true)
		}

		attempt++
		if !waitForProcessDetection(deadline, checkInterval, input.Done) {
			break
		}
	}

	if processDetectionCancelled(input.Done) {
		return StagedProcessDetectionResult{}
	}
	logWarning(logger, "macOS Steam process detection timed out for game %s; monitoring the Steam launcher process", input.GameID)
	return resultForLauncher(input)
}

func selectDarwinSteamProcess(candidates []processutils.ProcessInfo, savedProcessName string) (processutils.ProcessInfo, bool) {
	filtered := make([]processutils.ProcessInfo, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.PID == 0 || strings.EqualFold(candidate.Name, "steam_osx") || IsLikelyHelperProcess(candidate.Name) {
			continue
		}
		if strings.TrimSpace(savedProcessName) != "" && strings.EqualFold(candidate.Name, savedProcessName) {
			return candidate, true
		}
		filtered = append(filtered, candidate)
	}
	if len(filtered) == 0 {
		return processutils.ProcessInfo{}, false
	}
	return filtered[0], true
}

func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	return processutils.ProcessInfo{}, false
}
