//go:build windows

package archiveutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const createNoWindowFlag uint32 = 0x08000000

func extractArchiveWithBundled7z(source, target string) (bool, error) {
	exePath, workDir, err := resolveBundled7z()
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

	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}

	return false, fmt.Errorf("bundled 7z extract failed: %w; output=%s", runErr, strings.TrimSpace(string(output)))
}

func resolveBundled7z() (string, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("resolve executable path: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", "", fmt.Errorf("resolve absolute executable path: %w", err)
	}

	baseDir := filepath.Dir(exe)
	for _, dir := range []string{
		filepath.Join(baseDir, "7z"),
		baseDir,
	} {
		exePath := filepath.Join(dir, "7z.exe")
		dllPath := filepath.Join(dir, "7z.dll")
		if fileExists(exePath) && fileExists(dllPath) {
			return exePath, dir, nil
		}
	}

	if exePath, ok := resolveRepoLibFile("win"+runtime.GOARCH, "7z", "7z.exe"); ok {
		dir := filepath.Dir(exePath)
		if fileExists(filepath.Join(dir, "7z.dll")) {
			return exePath, dir, nil
		}
	}

	return "", "", fmt.Errorf("bundled 7z.exe/7z.dll not found")
}
