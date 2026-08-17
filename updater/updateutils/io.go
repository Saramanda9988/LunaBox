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
)

func LoadTask(taskPath string) (*Task, error) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, fmt.Errorf("read update task: %w", err)
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("decode update task: %w", err)
	}
	if err := task.Validate(); err != nil {
		return nil, err
	}
	if !pathWithin(task.WorkDir, taskPath) {
		return nil, fmt.Errorf("update task must be inside work_dir")
	}
	return &task, nil
}

func WriteTask(taskPath string, task *Task) error {
	if task == nil {
		return fmt.Errorf("update task is nil")
	}
	if err := task.Validate(); err != nil {
		return err
	}
	if !pathWithin(task.WorkDir, taskPath) {
		return fmt.Errorf("update task must be inside work_dir")
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("encode update task: %w", err)
	}
	return writeFileAtomic(taskPath, data, 0600)
}

func FileSHA256(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func verifyFile(filePath string, expectedSize int64, expectedSHA256 string) error {
	actualSHA256, actualSize, err := FileSHA256(filePath)
	if err != nil {
		return err
	}
	if expectedSize > 0 && actualSize != expectedSize {
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, actualSize)
	}
	if !strings.EqualFold(actualSHA256, expectedSHA256) {
		return fmt.Errorf("sha256 mismatch: expected %s, got %s", expectedSHA256, actualSHA256)
	}
	return nil
}

func copyFile(source string, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
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

func writeFileAtomic(filePath string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, mode); err != nil {
		return err
	}
	if err := renameReplace(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}
