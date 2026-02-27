//go:build windows

package utils

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const createNoWindowFlag uint32 = 0x08000000

var (
	//go:embed 7z/7z.exe
	embedded7zExe []byte
	//go:embed 7z/7z.dll
	embedded7zDLL []byte

	embedded7zOnce sync.Once
	embedded7zPath string
	embedded7zDir  string
	embedded7zErr  error
)

func extractArchiveWithEmbedded7z(source, target string) (bool, error) {
	exePath, workDir, err := ensureEmbedded7zFiles()
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return false, fmt.Errorf("create 7z output dir: %w", err)
	}

	args := []string{"x", "-y", "-aoa", "-mcp=936", "-o" + target, source}
	cmd := exec.Command(exePath, args...)
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindowFlag,
	}

	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		return true, nil
	}

	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		if exitCode == 1 {
			return true, nil
		}
	}

	return false, fmt.Errorf("embedded 7z extract failed: %w; output=%s", runErr, strings.TrimSpace(string(output)))
}

func ensureEmbedded7zFiles() (string, string, error) {
	embedded7zOnce.Do(func() {
		if len(embedded7zExe) == 0 {
			embedded7zErr = fmt.Errorf("embedded 7z.exe is empty")
			return
		}
		if len(embedded7zDLL) == 0 {
			embedded7zErr = fmt.Errorf("embedded 7z.dll is empty")
			return
		}

		tempDir, err := os.MkdirTemp("", "lunabox-7z-*")
		if err != nil {
			embedded7zErr = fmt.Errorf("create temp dir for embedded 7z: %w", err)
			return
		}

		exePath := filepath.Join(tempDir, "7z.exe")
		dllPath := filepath.Join(tempDir, "7z.dll")

		if err := os.WriteFile(exePath, embedded7zExe, 0755); err != nil {
			embedded7zErr = fmt.Errorf("write embedded 7z.exe: %w", err)
			return
		}

		if err := os.WriteFile(dllPath, embedded7zDLL, 0644); err != nil {
			embedded7zErr = fmt.Errorf("write embedded 7z.dll: %w", err)
			return
		}

		embedded7zPath = exePath
		embedded7zDir = tempDir
	})

	if embedded7zErr != nil {
		return "", "", embedded7zErr
	}

	return embedded7zPath, embedded7zDir, nil
}
