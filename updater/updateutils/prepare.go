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
		return err
	}

	stageRoot := stagingDir(task)
	if err := os.RemoveAll(stageRoot); err != nil {
		return fmt.Errorf("reset staging directory: %w", err)
	}
	if err := os.MkdirAll(stageRoot, 0755); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}

	for _, file := range task.Files {
		if err := verifyFile(file.ArtifactPath, file.ArtifactSize, file.ArtifactSHA256); err != nil {
			return fmt.Errorf("verify artifact for %s: %w", file.Path, err)
		}

		outputPath := localPath(stageRoot, file.Path)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("create staging path for %s: %w", file.Path, err)
		}
		tempOutput := outputPath + ".tmp"
		_ = os.Remove(tempOutput)

		var err error
		switch file.Kind {
		case TaskFileKindPatch:
			sourcePath := localPath(task.AppDir, file.Path)
			if verifyErr := verifyFile(sourcePath, 0, file.SourceSHA256); verifyErr != nil {
				return fmt.Errorf("verify patch source %s: %w", file.Path, verifyErr)
			}
			err = ReconstructZstdPatch(sourcePath, file.ArtifactPath, tempOutput)
		case TaskFileKindFull:
			err = materializeFullArtifact(file.ArtifactPath, file.Compression, tempOutput)
		default:
			err = fmt.Errorf("unsupported task file kind: %s", file.Kind)
		}
		if err != nil {
			_ = os.Remove(tempOutput)
			return fmt.Errorf("prepare %s: %w", file.Path, err)
		}
		if err := verifyFile(tempOutput, file.TargetSize, file.TargetSHA256); err != nil {
			_ = os.Remove(tempOutput)
			return fmt.Errorf("verify reconstructed %s: %w", file.Path, err)
		}
		if requiresAuthenticode(file.Path) {
			if err := verifyAuthenticode(tempOutput); err != nil {
				_ = os.Remove(tempOutput)
				return fmt.Errorf("verify Authenticode signature for %s: %w", file.Path, err)
			}
		}
		if err := os.Rename(tempOutput, outputPath); err != nil {
			_ = os.Remove(tempOutput)
			return fmt.Errorf("finalize staged %s: %w", file.Path, err)
		}
	}

	return writePreparedMarker(task)
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
		return err
	}
	if err := verifyPreparedMarker(task); err != nil {
		return err
	}
	for _, file := range task.Files {
		stagedPath := localPath(stagingDir(task), file.Path)
		if err := verifyFile(stagedPath, file.TargetSize, file.TargetSHA256); err != nil {
			return fmt.Errorf("verify staged %s: %w", file.Path, err)
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
		return copyFile(artifactPath, outputPath)
	}
	if compression != ArtifactCompressionZstd {
		return fmt.Errorf("unsupported full artifact compression: %s", compression)
	}

	artifact, err := os.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("open full artifact: %w", err)
	}
	defer artifact.Close()
	decoder, err := zstd.NewReader(artifact, zstd.WithDecoderLowmem(true))
	if err != nil {
		return fmt.Errorf("open zstd decoder: %w", err)
	}
	defer decoder.Close()
	return writeDecodedFile(decoder, outputPath)
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
