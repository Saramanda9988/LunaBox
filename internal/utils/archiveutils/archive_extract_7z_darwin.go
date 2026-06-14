//go:build darwin

package archiveutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func extractArchiveWithBundled7z(source, target string) (bool, error) {
	toolPath, err := resolveBundled7zz()
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return false, fmt.Errorf("create 7zz output dir: %w", err)
	}

	if err := ensureExecutable(toolPath); err != nil {
		return false, fmt.Errorf("prepare bundled 7zz: %w", err)
	}

	args := []string{"x", "-y", "-aoa", "-o" + target, source}
	cmd := exec.Command(toolPath, args...)
	cmd.Dir = filepath.Dir(toolPath)

	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		return true, nil
	}

	if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}

	return false, fmt.Errorf("bundled 7zz extract failed: %w; output=%s", runErr, strings.TrimSpace(string(output)))
}

func resolveBundled7zz() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve absolute executable path: %w", err)
	}

	if appRoot, ok := darwinAppBundleRoot(exe); ok {
		candidate := filepath.Join(appRoot, "Contents", "Resources", "bin", "7zz")
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	baseDir := filepath.Dir(exe)
	for _, candidate := range []string{
		filepath.Join(baseDir, "bin", "7zz"),
		filepath.Join(baseDir, "7zz"),
	} {
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	if toolPath, ok := resolveRepoLibFile("macarm64", "7z", "7zz"); ok {
		return toolPath, nil
	}

	return "", fmt.Errorf("bundled 7zz not found")
}

func darwinAppBundleRoot(exe string) (string, bool) {
	const marker = ".app/Contents/MacOS/"
	idx := strings.Index(exe, marker)
	if idx < 0 {
		return "", false
	}
	return exe[:idx+len(".app")], true
}
