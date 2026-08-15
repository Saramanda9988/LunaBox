//go:build !windows && !darwin && !linux

package launcher

import "lunabox/internal/utils/processutils"

// DetectStagedProcess keeps non-Windows staged detection conservative.
// macOS launch strategies normally use DetectionLauncherOnly; if a staged plan
// reaches this fallback, monitor the launcher PID directly.
func DetectStagedProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Staged process detection is not supported on this platform for game %s, using launcher process: %s (PID %d)", input.GameID, input.Launcher.Name, input.Launcher.PID)
	return resultForLauncher(input)
}

func DetectSteamDirectoryProcess(input StagedProcessDetectionInput, logger DetectionLogger) StagedProcessDetectionResult {
	logInfo(logger, "Steam directory detection is not supported on this platform for game %s", input.GameID)
	return promptProcessSelectionResult()
}

// DetectSuccessorProcess is Windows-only; on other platforms a monitored
// process exit always finalizes the play session.
func DetectSuccessorProcess(input SuccessorDetectionInput, logger DetectionLogger) (processutils.ProcessInfo, bool) {
	return processutils.ProcessInfo{}, false
}
