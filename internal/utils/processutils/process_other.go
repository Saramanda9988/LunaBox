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

func IsProcessPresentByPID(pid uint32) bool {
	return false
}

func GetDescendantProcesses(parentPID uint32) ([]ProcessInfo, error) {
	return nil, unsupportedProcessError()
}

func GetProcessesByExecutableDir(rootDir string) ([]ProcessInfo, error) {
	return nil, unsupportedProcessError()
}

func StartProcess(file string, args []string, dir string) (*StartedProcess, error) {
	return nil, unsupportedProcessError()
}

func CloseProcessHandle(processHandle uintptr) error {
	return unsupportedProcessError()
}

func FilterProcessesWithVisibleWindows(processes []ProcessInfo) []ProcessInfo {
	return nil
}

func HasVisibleTopLevelWindow(pid uint32) bool {
	return false
}

func IsProcessRunningByPID(pid uint32, ctx context.Context) bool {
	return false
}

type ProcessMonitor struct{}

type SnapshotProcessMonitor struct{}

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

func WaitForProcessHandleExitAsync(pid uint32, processHandle uintptr) (*ProcessMonitor, <-chan struct{}, error) {
	return nil, nil, unsupportedProcessError()
}

func NewSnapshotProcessMonitor(pid uint32) *SnapshotProcessMonitor {
	return &SnapshotProcessMonitor{}
}

func (m *SnapshotProcessMonitor) Stop() {}

func (m *SnapshotProcessMonitor) ExitChan() <-chan struct{} {
	exitChan := make(chan struct{})
	close(exitChan)
	return exitChan
}

func WaitForProcessExitBySnapshotAsync(pid uint32) (*SnapshotProcessMonitor, <-chan struct{}) {
	monitor := NewSnapshotProcessMonitor(pid)
	return monitor, monitor.ExitChan()
}

func unsupportedProcessError() error {
	return fmt.Errorf("process utilities are only supported on Windows")
}
