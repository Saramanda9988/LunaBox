package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckIfProcessRunning 检查指定进程是否正在运行
func CheckIfProcessRunning(processName string) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", processName), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	outputStr := string(output)
	// 检查输出中是否包含进程名
	return strings.Contains(strings.ToLower(outputStr), strings.ToLower(processName)), nil
}
