package launcher

import (
	"fmt"
	"lunabox/internal/utils/processutils"
	"runtime"
	"strings"
	"time"
)

type DetectionLogger interface {
	Infof(format string, args ...any)
	Warningf(format string, args ...any)
}

type LaunchedProcessInfo struct {
	PID  uint32
	Name string
}

type StagedProcessDetectionInput struct {
	GameID            string
	Launcher          LaunchedProcessInfo
	LauncherExeName   string
	LaunchDir         string
	SavedProcessName  string
	DetectionDeadline time.Time
	Done              <-chan struct{}
}

type StagedProcessDetectionResult struct {
	ProcessID               uint32
	ProcessName             string
	UseLauncherHandle       bool
	CloseLauncherHandle     bool
	RequireProcessSelection bool
	PersistProcessName      string
}

// SuccessorDetectionInput describes an exited monitored process so the
// platform layer can look for a process that took over from it (splash window
// spawning the real game, launcher hand-off, self re-exec, in-game restart).
type SuccessorDetectionInput struct {
	GameID            string
	ExitedPID         uint32
	ExitedProcessName string
	LaunchDir         string
	SavedProcessName  string
	// SessionStart guards against PID reuse: a successor must have been
	// created after the play session began.
	SessionStart time.Time
	// SelfPID excludes the host app itself from candidate processes.
	SelfPID uint32
}

func resultForLauncher(input StagedProcessDetectionInput) StagedProcessDetectionResult {
	result := StagedProcessDetectionResult{
		ProcessID:         input.Launcher.PID,
		ProcessName:       input.Launcher.Name,
		UseLauncherHandle: true,
	}
	if ShouldPersistLauncherProcessName(input.SavedProcessName) {
		result.PersistProcessName = strings.TrimSpace(input.LauncherExeName)
	}
	return result
}

func resultForExternalProcess(input StagedProcessDetectionInput, proc processutils.ProcessInfo, closeLauncher bool) StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		ProcessID:           proc.PID,
		ProcessName:         proc.Name,
		CloseLauncherHandle: closeLauncher,
		PersistProcessName:  ProcessNameForPersistence(input.LauncherExeName, proc.Name),
	}
}

func promptProcessSelectionResult() StagedProcessDetectionResult {
	return StagedProcessDetectionResult{
		CloseLauncherHandle:     true,
		RequireProcessSelection: true,
	}
}

func HasReliableSavedProcessName(savedProcessName string, launcherExeName string) bool {
	saved := strings.TrimSpace(savedProcessName)
	if saved == "" || strings.EqualFold(saved, strings.TrimSpace(launcherExeName)) {
		return false
	}
	return IsPersistableProcessName(saved)
}

func ShouldPersistLauncherProcessName(savedProcessName string) bool {
	saved := strings.TrimSpace(savedProcessName)
	return saved == "" || !IsPersistableProcessName(saved)
}

func ProcessNameForPersistence(launcherExeName string, detectedProcessName string) string {
	detected := strings.TrimSpace(detectedProcessName)
	if IsPersistableProcessName(detected) {
		return detected
	}
	launcher := strings.TrimSpace(launcherExeName)
	if IsPersistableProcessName(launcher) {
		return launcher
	}
	return ""
}

func IsPersistableProcessName(processName string) bool {
	name := strings.ToLower(strings.TrimSpace(processName))
	if name == "" || IsLikelyHelperProcess(name) {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.HasSuffix(name, ".exe")
	}
	return true
}

func IsLikelyHelperProcess(processName string) bool {
	name := strings.ToLower(strings.TrimSpace(processName))
	if name == "" {
		return true
	}
	switch name {
	case "conhost.exe",
		"crashpad_handler.exe",
		"crashreporter.exe",
		"cef_server.exe",
		"cefsharp.browsersubprocess.exe",
		"steam",
		"steam.exe",
		"steamwebhelper",
		"werfault.exe",
		"crashpad_handler",
		"crashreporter",
		"plugin-container",
		"proton",
		"pressure-vessel",
		"pv-bwrap",
		"pv-adverb",
		"reaper",
		"srt-bwrap",
		"steam-runtime-launcher-service",
		"gameoverlayui",
		"wine",
		"wine-preloader",
		"wine64",
		"wine64-preloader",
		"wineboot",
		"wineserver",
		"winedevice.exe",
		"winemenubuilder",
		"explorer.exe",
		"plugplay.exe",
		"services.exe",
		"rpcss.exe",
		"svchost.exe",
		"tabtip.exe",
		"xalia.exe":
		return true
	default:
		return strings.Contains(name, " helper") ||
			strings.Contains(name, "helper (") ||
			strings.Contains(name, "crashpad") ||
			strings.Contains(name, "crash reporter") ||
			strings.Contains(name, "pressure-vessel")
	}
}

func FormatProcessCandidates(processes []processutils.ProcessInfo) string {
	if len(processes) == 0 {
		return "(none)"
	}

	parts := make([]string, 0, len(processes))
	for _, proc := range processes {
		parts = append(parts, fmt.Sprintf("%s(PID %d)", proc.Name, proc.PID))
	}
	return strings.Join(parts, ", ")
}

func logInfo(logger DetectionLogger, format string, args ...any) {
	if logger != nil {
		logger.Infof(format, args...)
	}
}

func logWarning(logger DetectionLogger, format string, args ...any) {
	if logger != nil {
		logger.Warningf(format, args...)
	}
}

const fallbackProcessDetectionTimeout = time.Minute

func processDetectionDeadline(input StagedProcessDetectionInput) time.Time {
	if !input.DetectionDeadline.IsZero() {
		return input.DetectionDeadline
	}
	return time.Now().Add(fallbackProcessDetectionTimeout)
}

func processDetectionCancelled(done <-chan struct{}) bool {
	if done == nil {
		return false
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func processDetectionAttemptLogger(logger DetectionLogger, attempt int, every int) DetectionLogger {
	if attempt == 0 || every <= 1 || attempt%every == 0 {
		return logger
	}
	return nil
}

func waitForProcessDetection(deadline time.Time, interval time.Duration, done <-chan struct{}) bool {
	remaining := time.Until(deadline)
	if remaining <= 0 || processDetectionCancelled(done) {
		return false
	}
	if interval > remaining {
		interval = remaining
	}

	timer := time.NewTimer(interval)
	defer timer.Stop()
	if done == nil {
		<-timer.C
		return true
	}

	select {
	case <-timer.C:
		return true
	case <-done:
		return false
	}
}
