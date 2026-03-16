package apputils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenDirectory 使用系统文件管理器打开指定目录
func OpenDirectory(dir string) error {
	if dir == "" {
		return os.ErrInvalid
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}

	return cmd.Start()
}

// OpenFileOrFolder 使用系统文件管理器打开文件或目录。如果是文件，尽量在资源管理器中选中它。
func OpenFileOrFolder(path string) error {
	if path == "" {
		return os.ErrNotExist
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return OpenDirectory(filepath.Dir(absPath))
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if info.IsDir() {
			cmd = exec.Command("explorer", absPath)
		} else {
			cmd = exec.Command("explorer", "/select,", absPath)
		}
	case "darwin":
		if info.IsDir() {
			cmd = exec.Command("open", absPath)
		} else {
			cmd = exec.Command("open", "-R", absPath)
		}
	default:
		dir := absPath
		if !info.IsDir() {
			dir = filepath.Dir(absPath)
		}
		cmd = exec.Command("xdg-open", dir)
	}

	return cmd.Start()
}
