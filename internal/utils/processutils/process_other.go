//go:build !windows

package processutils

import (
	"context"
	"fmt"
	"time"
)

func CheckIfProcessRunning(processName string) (bool, error) {
	return false, unsupportedProcessError()
}

func GetRunningProcesses() ([]ProcessInfo, error) {
	return nil, unsupportedProcessError()
}

func GetProcessPIDByName(processName string) (uint32, error) {
	return 0, unsupportedProcessError()
}

func IsProcessRunningByPID(pid uint32, ctx context.Context) bool {
	return false
}

type ProcessMonitor struct{}

func NewProcessMonitor(pid uint32) *ProcessMonitor {
	return &ProcessMonitor{}
}

func (pm *ProcessMonitor) Start() (<-chan struct{}, error) {
	return nil, unsupportedProcessError()
}

func (pm *ProcessMonitor) Stop() {}

func (pm *ProcessMonitor) WaitForProcessExit(timeout time.Duration) bool {
	return true
}

func WaitForProcessExitAsync(pid uint32) (*ProcessMonitor, <-chan struct{}, error) {
	return nil, nil, unsupportedProcessError()
}

func unsupportedProcessError() error {
	return fmt.Errorf("process utilities are only supported on Windows")
}
