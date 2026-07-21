//go:build !windows

package updateutils

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		process, err := os.FindProcess(pid)
		if err != nil {
			return nil
		}
		err = process.Signal(syscall.Signal(0))
		if err != nil {
			return nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func replaceFile(targetPath string, replacementPath string, backupPath string) (bool, error) {
	_, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return false, os.Rename(replacementPath, targetPath)
	}
	if err != nil {
		return false, err
	}
	if err := os.Rename(targetPath, backupPath); err != nil {
		return true, err
	}
	if err := os.Rename(replacementPath, targetPath); err != nil {
		_ = os.Rename(backupPath, targetPath)
		return true, err
	}
	return true, nil
}

func restoreBackup(targetPath string, backupPath string) error {
	_ = os.Remove(targetPath)
	return os.Rename(backupPath, targetPath)
}

func updateInstallMetadata(buildMode string, version string) error {
	return nil
}

func configureRestartCommand(command *exec.Cmd) error {
	return nil
}
