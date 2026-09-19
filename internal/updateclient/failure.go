package updateclient

import (
	"errors"
	"os"
	"syscall"

	"lunabox/updater/updateutils"
)

// Normalized reasons reported to the update server. Raw error messages contain
// local absolute paths (user name, install directory), so they only ever reach
// the local log.
const (
	// failureReasonUpdaterNotStarted means the updater process never ran far
	// enough to classify its own failure. The usual cause is antivirus or EDR
	// blocking a freshly written executable from %TEMP%.
	failureReasonUpdaterNotStarted = "updater_not_started"
	failureReasonUACCancelled      = "uac_cancelled"
	failureReasonAccessDenied      = "access_denied"
	failureReasonUpdaterMissing    = "updater_missing"
	failureReasonLaunchFailed      = "launch_failed"
	failureReasonFallbackDownload  = "fallback_download_failed"
)

// ShellExecuteEx error codes, matched before the generic launch failure.
var (
	errorFileNotFound = syscall.Errno(2)
	errorPathNotFound = syscall.Errno(3)
	errorAccessDenied = syscall.Errno(5)
	errorCancelled    = syscall.Errno(1223)
)

// prepareFailureReason reports why the updater's prepare run failed, using the
// marker it writes into the transaction directory.
func prepareFailureReason(workDir string) string {
	kind, ok := updateutils.ReadPrepareFailure(workDir)
	if !ok {
		return failureReasonUpdaterNotStarted
	}
	return string(kind)
}

// launchFailureReason classifies a failure to start the updater in commit mode.
func launchFailureReason(err error) string {
	switch {
	case errors.Is(err, errorCancelled):
		return failureReasonUACCancelled
	case errors.Is(err, errorAccessDenied), errors.Is(err, os.ErrPermission):
		return failureReasonAccessDenied
	case errors.Is(err, errorFileNotFound), errors.Is(err, errorPathNotFound):
		return failureReasonUpdaterMissing
	default:
		return failureReasonLaunchFailed
	}
}
