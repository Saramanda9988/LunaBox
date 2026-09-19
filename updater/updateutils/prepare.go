package updateutils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Prepare reconstructs and verifies every target file without modifying the
// application directory. It is safe to run while LunaBox is still running.
func Prepare(task *Task) error {
	if task == nil {
		return fmt.Errorf("update task is nil")
	}
	if err := task.Validate(); err != nil {
		return withFailureKind(FailureKindTaskInvalid, err)
	}

	stageRoot := stagingDir(task)
	if err := os.RemoveAll(stageRoot); err != nil {
		return withFailureKind(FailureKindStaging, fmt.Errorf("reset staging directory: %w", err))
	}
	if err := os.MkdirAll(stageRoot, 0755); err != nil {
		return withFailureKind(FailureKindStaging, fmt.Errorf("create staging directory: %w", err))
	}

	for _, file := range task.Files {
		outputPath := localPath(stageRoot, file.Path)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return withFailureKind(FailureKindStaging, fmt.Errorf("create staging path for %s: %w", file.Path, err))
		}
		tempOutput := outputPath + ".tmp"
		_ = os.Remove(tempOutput)

		var err error
		switch file.Kind {
		case TaskFileKindPatch:
			err = preparePatchChain(task, file, tempOutput)
		case TaskFileKindFull:
			if err := verifyFile(file.ArtifactPath, file.ArtifactSize, file.ArtifactSHA256); err != nil {
				return withFailureKind(FailureKindArtifact, fmt.Errorf("verify artifact for %s: %w", file.Path, err))
			}
			err = materializeFullArtifact(file.ArtifactPath, file.Compression, tempOutput)
		default:
			err = fmt.Errorf("unsupported task file kind: %s", file.Kind)
		}
		if err != nil {
			_ = os.Remove(tempOutput)
			return fmt.Errorf("prepare %s: %w", file.Path, err)
		}
		if requiresAuthenticode(file.Path) {
			if err := verifyAuthenticode(tempOutput); err != nil {
				_ = os.Remove(tempOutput)
				return withFailureKind(FailureKindSignature, fmt.Errorf("verify Authenticode signature for %s: %w", file.Path, err))
			}
		}
		if err := os.Rename(tempOutput, outputPath); err != nil {
			_ = os.Remove(tempOutput)
			return withFailureKind(FailureKindStaging, fmt.Errorf("finalize staged %s: %w", file.Path, err))
		}
	}

	return writePreparedMarker(task)
}

func preparePatchChain(task *Task, file TaskFile, firstOutput string) error {
	steps := file.PatchChain
	if len(steps) == 0 {
		steps = []TaskPatch{{
			ArtifactPath:   file.ArtifactPath,
			ArtifactSize:   file.ArtifactSize,
			ArtifactSHA256: file.ArtifactSHA256,
			Compression:    file.Compression,
			SourceSHA256:   file.SourceSHA256,
			TargetSHA256:   file.TargetSHA256,
			TargetSize:     file.TargetSize,
		}}
	}

	sourcePath := localPath(task.AppDir, file.Path)
	for index, step := range steps {
		if err := verifyFile(step.ArtifactPath, step.ArtifactSize, step.ArtifactSHA256); err != nil {
			return withFailureKind(FailureKindArtifact, fmt.Errorf("verify patch artifact step %d: %w", index+1, err))
		}
		if err := verifyFile(sourcePath, 0, step.SourceSHA256); err != nil {
			return withFailureKind(FailureKindSource, fmt.Errorf("verify patch source step %d: %w", index+1, err))
		}

		outputPath := firstOutput
		if index > 0 {
			outputPath = firstOutput + fmt.Sprintf(".%d", index)
		}
		_ = os.Remove(outputPath)
		if err := ReconstructZstdPatch(sourcePath, step.ArtifactPath, outputPath); err != nil {
			_ = os.Remove(outputPath)
			return withFailureKind(FailureKindPatchApply, fmt.Errorf("reconstruct patch step %d: %w", index+1, err))
		}
		if err := verifyFile(outputPath, step.TargetSize, step.TargetSHA256); err != nil {
			_ = os.Remove(outputPath)
			return withFailureKind(FailureKindPatchApply, fmt.Errorf("verify reconstructed patch step %d: %w", index+1, err))
		}
		if index > 0 {
			_ = os.Remove(sourcePath)
		}
		sourcePath = outputPath
	}
	if sourcePath != firstOutput {
		if err := os.Rename(sourcePath, firstOutput); err != nil {
			_ = os.Remove(sourcePath)
			return withFailureKind(FailureKindPatchApply, fmt.Errorf("finalize patch chain: %w", err))
		}
	}
	return nil
}

func requiresAuthenticode(managedPath string) bool {
	return strings.EqualFold(managedPath, "LunaBox.exe") ||
		strings.EqualFold(managedPath, "LunaBoxUpdater.exe") ||
		strings.EqualFold(managedPath, "lunacli.exe")
}

func ValidatePrepared(task *Task) error {
	if task == nil {
		return fmt.Errorf("update task is nil")
	}
	if err := task.Validate(); err != nil {
		return withFailureKind(FailureKindTaskInvalid, err)
	}
	if err := verifyPreparedMarker(task); err != nil {
		return withFailureKind(FailureKindMarker, err)
	}
	for _, file := range task.Files {
		stagedPath := localPath(stagingDir(task), file.Path)
		if err := verifyFile(stagedPath, file.TargetSize, file.TargetSHA256); err != nil {
			return withFailureKind(FailureKindStaging, fmt.Errorf("verify staged %s: %w", file.Path, err))
		}
	}
	return nil
}

// ReconstructZstdPatch decodes a zstd --patch-from artifact using the exact
// source file as its raw dictionary.
func ReconstructZstdPatch(sourcePath string, patchPath string, outputPath string) error {
	dictionary, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read patch source: %w", err)
	}
	patch, err := os.Open(patchPath)
	if err != nil {
		return fmt.Errorf("open patch: %w", err)
	}
	defer patch.Close()

	decoder, err := zstd.NewReader(patch, zstd.WithDecoderDictRaw(0, dictionary), zstd.WithDecoderLowmem(true))
	if err != nil {
		return fmt.Errorf("open zstd patch decoder: %w", err)
	}
	defer decoder.Close()
	return writeDecodedFile(decoder, outputPath)
}

func materializeFullArtifact(artifactPath string, compression string, outputPath string) error {
	if compression == ArtifactCompressionNone {
		return withFailureKind(FailureKindStaging, copyFile(artifactPath, outputPath))
	}
	if compression != ArtifactCompressionZstd {
		return withFailureKind(FailureKindArtifact, fmt.Errorf("unsupported full artifact compression: %s", compression))
	}

	artifact, err := os.Open(artifactPath)
	if err != nil {
		return withFailureKind(FailureKindArtifact, fmt.Errorf("open full artifact: %w", err))
	}
	defer artifact.Close()
	decoder, err := zstd.NewReader(artifact, zstd.WithDecoderLowmem(true))
	if err != nil {
		return withFailureKind(FailureKindDecode, fmt.Errorf("open zstd decoder: %w", err))
	}
	defer decoder.Close()
	return withFailureKind(FailureKindDecode, writeDecodedFile(decoder, outputPath))
}

func writeDecodedFile(reader io.Reader, outputPath string) error {
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, reader)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func preparedMarkerPath(task *Task) string {
	return filepath.Join(task.WorkDir, "prepared.json")
}

type preparedMarker struct {
	TransactionID string `json:"transaction_id"`
	TargetVersion string `json:"target_version"`
	TaskSHA256    string `json:"task_sha256"`
}

func writePreparedMarker(task *Task) error {
	taskSHA256, err := taskFingerprint(task)
	if err != nil {
		return err
	}
	marker := preparedMarker{
		TransactionID: task.TransactionID,
		TargetVersion: task.TargetVersion,
		TaskSHA256:    taskSHA256,
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("encode prepared update marker: %w", err)
	}
	return writeFileAtomic(preparedMarkerPath(task), data, 0600)
}

func verifyPreparedMarker(task *Task) error {
	data, err := os.ReadFile(preparedMarkerPath(task))
	if err != nil {
		return fmt.Errorf("prepared update marker is missing: %w", err)
	}
	var marker preparedMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return fmt.Errorf("decode prepared update marker: %w", err)
	}
	if marker.TransactionID != task.TransactionID || marker.TargetVersion != task.TargetVersion {
		return fmt.Errorf("prepared update marker does not match task")
	}
	taskSHA256, err := taskFingerprint(task)
	if err != nil {
		return err
	}
	if !strings.EqualFold(marker.TaskSHA256, taskSHA256) {
		return fmt.Errorf("prepared update task was modified")
	}
	return nil
}

func taskFingerprint(task *Task) (string, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("encode update task fingerprint: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
