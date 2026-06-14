package archiveutils

import (
	"fmt"
	"os"
	"path/filepath"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func resolveRepoLibFile(parts ...string) (string, bool) {
	candidates := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.Abs(exe); err == nil {
			candidates = append(candidates, filepath.Dir(abs))
		}
	}

	for _, start := range candidates {
		dir := start
		for i := 0; i < 8; i++ {
			pathParts := append([]string{dir, "lib"}, parts...)
			candidate := filepath.Join(pathParts...)
			if fileExists(candidate) {
				return candidate, true
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return "", false
}

func ensureExecutable(path string) error {
	if path == "" {
		return fmt.Errorf("executable path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", path)
	}
	if info.Mode().Perm()&0111 == 0 {
		return os.Chmod(path, info.Mode().Perm()|0755)
	}
	return nil
}
