package updateutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultWaitTimeout = 10 * time.Minute
	replaceRetryWindow = 10 * time.Second
	replaceRetryDelay  = 250 * time.Millisecond
)

type preExitCommitError struct {
	err error
}

type unsafeRestartCommitError struct {
	err error
}

func (e *preExitCommitError) Error() string {
	return e.err.Error()
}

func (e *preExitCommitError) Unwrap() error {
	return e.err
}

func (e *unsafeRestartCommitError) Error() string {
	return e.err.Error()
}

func (e *unsafeRestartCommitError) Unwrap() error {
	return e.err
}

// ShouldRestartAfterCommit reports whether LunaBox has exited far enough for
// the updater to safely start it again. Validation and wait failures leave the
// original process running and must not create a second instance.
func ShouldRestartAfterCommit(err error) bool {
	var preExitErr *preExitCommitError
	var unsafeRestartErr *unsafeRestartCommitError
	return err == nil || (!errors.As(err, &preExitErr) && !errors.As(err, &unsafeRestartErr))
}

type transactionJournal struct {
	TransactionID string                    `json:"transaction_id"`
	Status        string                    `json:"status"`
	Entries       []transactionJournalEntry `json:"entries"`
}

type transactionJournalEntry struct {
	Path          string `json:"path"`
	SwapPath      string `json:"swap_path"`
	BackupPath    string `json:"backup_path"`
	TargetExisted bool   `json:"target_existed"`
	Attempted     bool   `json:"attempted"`
	Applied       bool   `json:"applied"`
}

// Commit waits for LunaBox to exit and replaces only the managed files listed
// in the prepared task. Replacements are journaled and rolled back in reverse
// order if any file is locked or cannot be replaced.
func Commit(task *Task) error {
	if task == nil {
		return &preExitCommitError{err: fmt.Errorf("update task is nil")}
	}
	if err := task.Validate(); err != nil {
		return &preExitCommitError{err: err}
	}
	if err := ValidatePrepared(task); err != nil {
		return &preExitCommitError{err: err}
	}

	timeout := defaultWaitTimeout
	if task.WaitTimeout > 0 {
		timeout = time.Duration(task.WaitTimeout) * time.Second
	}
	if err := waitForProcessExit(task.WaitPID, timeout); err != nil {
		return &preExitCommitError{err: fmt.Errorf("wait for LunaBox to exit: %w", err)}
	}

	journal, err := newTransactionJournal(task)
	if err != nil {
		return err
	}
	journalPath := filepath.Join(task.WorkDir, "transaction.json")
	if previous, loadErr := loadJournal(journalPath); loadErr == nil && previous.Status == "applying" {
		if err := validateRecoveryJournal(journal, previous); err != nil {
			return err
		}
		if rollbackErr := rollbackTransaction(task, previous); rollbackErr != nil {
			return &unsafeRestartCommitError{err: fmt.Errorf("recover interrupted update: %w", rollbackErr)}
		}
	}
	if err := writeJournal(journalPath, journal); err != nil {
		return fmt.Errorf("write transaction journal: %w", err)
	}

	for i := range journal.Entries {
		entry := &journal.Entries[i]
		targetPath := localPath(task.AppDir, entry.Path)
		_, statErr := os.Stat(targetPath)
		entry.TargetExisted = statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			return rollbackOrError(task, journal, fmt.Errorf("inspect replacement target %s: %w", entry.Path, statErr))
		}
		entry.Attempted = true
		if err := writeJournal(journalPath, journal); err != nil {
			return rollbackOrError(task, journal, fmt.Errorf("persist replacement intent for %s: %w", entry.Path, err))
		}

		stagedPath := localPath(stagingDir(task), entry.Path)
		if err := copyFile(stagedPath, entry.SwapPath); err != nil {
			return rollbackOrError(task, journal, fmt.Errorf("stage replacement for %s: %w", entry.Path, err))
		}

		targetExisted, err := replaceFileWithRetry(targetPath, entry.SwapPath, entry.BackupPath)
		if targetExisted != entry.TargetExisted {
			entry.TargetExisted = targetExisted
			err = fmt.Errorf("target existence changed during update")
		}
		if err != nil {
			_ = os.Remove(entry.SwapPath)
			rollbackErr := rollbackTransaction(task, journal)
			if rollbackErr != nil {
				return &unsafeRestartCommitError{err: fmt.Errorf("replace %s: %w; rollback failed: %v", entry.Path, err, rollbackErr)}
			}
			return fmt.Errorf("replace %s: %w", entry.Path, err)
		}

		entry.Applied = true
		if err := writeJournal(journalPath, journal); err != nil {
			rollbackErr := rollbackTransaction(task, journal)
			if rollbackErr != nil {
				return &unsafeRestartCommitError{err: fmt.Errorf("persist transaction after %s: %w; rollback failed: %v", entry.Path, err, rollbackErr)}
			}
			return fmt.Errorf("persist transaction after %s: %w", entry.Path, err)
		}
	}

	journal.Status = "complete"
	if err := writeJournal(journalPath, journal); err != nil {
		return fmt.Errorf("complete transaction journal: %w", err)
	}
	metadataWarning := ""
	if err := updateInstallMetadata(task.BuildMode, task.TargetVersion); err != nil {
		metadataWarning = fmt.Sprintf("files updated but install metadata failed: %v", err)
	}
	cleanupTransaction(journal)
	_ = os.Remove(journalPath)
	cleanupPreparedFiles(task)
	_ = writeResult(task, true, "", metadataWarning)
	return nil
}

func rollbackOrError(task *Task, journal *transactionJournal, operationErr error) error {
	if rollbackErr := rollbackTransaction(task, journal); rollbackErr != nil {
		return &unsafeRestartCommitError{err: fmt.Errorf("%w; rollback failed: %v", operationErr, rollbackErr)}
	}
	return operationErr
}

func replaceFileWithRetry(targetPath string, replacementPath string, backupPath string) (bool, error) {
	deadline := time.Now().Add(replaceRetryWindow)
	for {
		targetExisted, err := replaceFile(targetPath, replacementPath, backupPath)
		if err == nil || time.Now().After(deadline) {
			return targetExisted, err
		}
		time.Sleep(replaceRetryDelay)
	}
}

func Restart(task *Task) error {
	if task == nil {
		return fmt.Errorf("update task is nil")
	}
	restartPath := localPath(task.AppDir, task.RestartPath)
	command := exec.Command(restartPath, task.RestartArgs...)
	command.Dir = task.AppDir
	if err := configureRestartCommand(command); err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("restart LunaBox: %w", err)
	}
	return command.Process.Release()
}

type UpdateResult struct {
	TransactionID string `json:"transaction_id"`
	TargetVersion string `json:"target_version"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	Warning       string `json:"warning,omitempty"`
	FinishedAt    string `json:"finished_at"`
}

func WriteResult(task *Task, success bool, errorMessage string) error {
	return writeResult(task, success, errorMessage, "")
}

func writeResult(task *Task, success bool, errorMessage string, warningMessage string) error {
	result := UpdateResult{
		TransactionID: task.TransactionID,
		TargetVersion: task.TargetVersion,
		Success:       success,
		Error:         errorMessage,
		Warning:       warningMessage,
		FinishedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(task.WorkDir, "result.json"), data, 0600)
}

func newTransactionJournal(task *Task) (*transactionJournal, error) {
	journal := &transactionJournal{
		TransactionID: task.TransactionID,
		Status:        "applying",
		Entries:       make([]transactionJournalEntry, 0, len(task.Files)),
	}
	// LunaBox.exe is deliberately replaced last. A crash before that point leaves
	// the old GUI executable available to report or retry the update.
	appendEntry := func(file TaskFile) error {
		targetPath := localPath(task.AppDir, file.Path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		base := filepath.Base(targetPath)
		journal.Entries = append(journal.Entries, transactionJournalEntry{
			Path:       file.Path,
			SwapPath:   filepath.Join(filepath.Dir(targetPath), "."+base+"."+task.TransactionID+".new"),
			BackupPath: filepath.Join(filepath.Dir(targetPath), "."+base+"."+task.TransactionID+".bak"),
		})
		return nil
	}
	for _, file := range task.Files {
		if !stringsEqualFold(file.Path, "LunaBox.exe") {
			if err := appendEntry(file); err != nil {
				return nil, err
			}
		}
	}
	for _, file := range task.Files {
		if stringsEqualFold(file.Path, "LunaBox.exe") {
			if err := appendEntry(file); err != nil {
				return nil, err
			}
		}
	}
	return journal, nil
}

func rollbackTransaction(task *Task, journal *transactionJournal) error {
	if journal == nil {
		return nil
	}
	var rollbackErrors []error
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		entry := journal.Entries[i]
		if !entry.Attempted {
			continue
		}
		targetPath := localPath(task.AppDir, entry.Path)
		if entry.TargetExisted {
			if _, err := os.Stat(entry.BackupPath); os.IsNotExist(err) {
				_ = os.Remove(entry.SwapPath)
				continue
			} else if err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("inspect backup for %s: %w", entry.Path, err))
			} else if err := restoreBackup(targetPath, entry.BackupPath); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", entry.Path, err))
			}
		} else {
			// os.Rename consumes the swap path when a previously absent target is
			// installed. Its absence therefore tells recovery that the target may
			// have been created even if the final journal write never happened.
			if _, err := os.Stat(entry.SwapPath); os.IsNotExist(err) {
				if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("remove new %s: %w", entry.Path, err))
				}
			}
		}
		_ = os.Remove(entry.SwapPath)
	}
	rollbackErr := errors.Join(rollbackErrors...)
	if rollbackErr == nil {
		cleanupTransaction(journal)
	}
	return rollbackErr
}

func cleanupTransaction(journal *transactionJournal) {
	if journal == nil {
		return
	}
	for _, entry := range journal.Entries {
		_ = os.Remove(entry.SwapPath)
		_ = os.Remove(entry.BackupPath)
	}
}

func cleanupPreparedFiles(task *Task) {
	if task == nil {
		return
	}
	_ = os.RemoveAll(stagingDir(task))
	_ = os.RemoveAll(filepath.Join(task.WorkDir, "artifacts"))
}

func loadJournal(journalPath string) (*transactionJournal, error) {
	data, err := os.ReadFile(journalPath)
	if err != nil {
		return nil, err
	}
	var journal transactionJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return nil, err
	}
	return &journal, nil
}

func writeJournal(journalPath string, journal *transactionJournal) error {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(journalPath, data, 0600)
}

func validateRecoveryJournal(expected *transactionJournal, actual *transactionJournal) error {
	if expected == nil || actual == nil || expected.TransactionID != actual.TransactionID || len(expected.Entries) != len(actual.Entries) {
		return fmt.Errorf("transaction journal does not match update task")
	}
	for i := range expected.Entries {
		if expected.Entries[i].Path != actual.Entries[i].Path ||
			expected.Entries[i].SwapPath != actual.Entries[i].SwapPath ||
			expected.Entries[i].BackupPath != actual.Entries[i].BackupPath {
			return fmt.Errorf("transaction journal entry %d does not match update task", i)
		}
	}
	return nil
}

func stringsEqualFold(a string, b string) bool {
	return strings.EqualFold(a, b)
}
