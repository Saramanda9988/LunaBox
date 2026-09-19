package updateutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FailureKind is the normalized failure reason reported to the update server.
// Raw error messages can contain local absolute paths, so they stay in the local
// log; only these kinds leave the machine.
type FailureKind string

const (
	FailureKindUnknown     FailureKind = "unknown"
	FailureKindTaskInvalid FailureKind = "task_invalid"
	FailureKindArtifact    FailureKind = "artifact_invalid"
	FailureKindDecode      FailureKind = "artifact_decode_failed"
	FailureKindSource      FailureKind = "source_mismatch"
	FailureKindPatchApply  FailureKind = "patch_apply_failed"
	FailureKindSignature   FailureKind = "signature_invalid"
	FailureKindStaging     FailureKind = "staging_failed"
	FailureKindMarker      FailureKind = "prepared_marker_invalid"
	FailureKindWait        FailureKind = "wait_process_failed"
	FailureKindReplace     FailureKind = "target_replace_failed"
	FailureKindJournal     FailureKind = "journal_failed"
	FailureKindRollback    FailureKind = "rollback_failed"
	FailureKindRestart     FailureKind = "restart_failed"
)

type failureKindError struct {
	kind FailureKind
	err  error
}

func (e *failureKindError) Error() string {
	return e.err.Error()
}

func (e *failureKindError) Unwrap() error {
	return e.err
}

// withFailureKind tags an error with its normalized category. Tag the innermost
// error only: the outer %w wrappers preserve it and FailureKindOf reads back the
// outermost tag.
func withFailureKind(kind FailureKind, err error) error {
	if err == nil {
		return nil
	}
	return &failureKindError{kind: kind, err: err}
}

// FailureKindOf reports the normalized failure category of an error chain.
func FailureKindOf(err error) FailureKind {
	var target *failureKindError
	if errors.As(err, &target) {
		return target.kind
	}
	return FailureKindUnknown
}

// PrepareFailureFileName is the marker the updater writes when "prepare" fails.
// LunaBox reads it to report the normalized reason; its absence means the
// updater process itself never got far enough to classify the failure.
const PrepareFailureFileName = "prepare-failure.json"

type prepareFailure struct {
	Kind FailureKind `json:"kind"`
}

// WritePrepareFailure records the normalized reason of a failed prepare run
// inside the transaction directory.
func WritePrepareFailure(task *Task, err error) error {
	if task == nil {
		return fmt.Errorf("update task is nil")
	}
	data, encodeErr := json.MarshalIndent(prepareFailure{Kind: FailureKindOf(err)}, "", "  ")
	if encodeErr != nil {
		return fmt.Errorf("encode prepare failure: %w", encodeErr)
	}
	path := filepath.Join(task.WorkDir, PrepareFailureFileName)
	if writeErr := writeFileAtomic(path, data, 0600); writeErr != nil {
		return fmt.Errorf("write prepare failure: %w", writeErr)
	}
	return nil
}

// ReadPrepareFailure reads the normalized reason written by WritePrepareFailure.
// The reported flag is false when no marker exists, which means the updater
// process never ran far enough to classify its own failure.
func ReadPrepareFailure(workDir string) (FailureKind, bool) {
	data, err := os.ReadFile(filepath.Join(workDir, PrepareFailureFileName))
	if err != nil {
		return FailureKindUnknown, false
	}
	var marker prepareFailure
	if err := json.Unmarshal(data, &marker); err != nil || marker.Kind == "" {
		return FailureKindUnknown, false
	}
	return marker.Kind, true
}
