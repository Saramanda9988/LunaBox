//go:build darwin

package processutils

/*
#include <libproc.h>
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

type processSnapshotEntry struct {
	Name      string
	PID       uint32
	ParentPID uint32
}

func StartProcess(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcessWithEnv(file, args, dir, nil)
}

func StartProcessWithEnv(file string, args []string, dir string, env []string) (*StartedProcess, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return nil, fmt.Errorf("executable path is empty")
	}

	var cmd *exec.Cmd
	if strings.EqualFold(filepath.Ext(file), ".app") {
		openArgs := []string{"-W", file}
		if len(args) > 0 {
			openArgs = append(openArgs, "--args")
			openArgs = append(openArgs, args...)
		}
		cmd = exec.Command("open", openArgs...)
	} else {
		cmd = exec.Command(file, args...)
	}
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	pid := uint32(cmd.Process.Pid)
	exitChan := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exitChan)
	}()

	return &StartedProcess{PID: pid, ExitChan: exitChan}, nil
}

func StartProcessElevated(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcess(file, args, dir)
}

func CloseProcessHandle(processHandle uintptr) error {
	return nil
}

func CheckIfProcessRunning(processName string) (bool, error) {
	_, err := GetProcessPIDByName(processName)
	if err != nil {
		if strings.Contains(err.Error(), "process not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GetRunningProcesses() ([]ProcessInfo, error) {
	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}

	systemProcesses := map[string]bool{
		"kernel_task":        true,
		"launchd":            true,
		"syslogd":            true,
		"runningboardd":      true,
		"windowserver":       true,
		"coreservicesd":      true,
		"distnoted":          true,
		"cfprefsd":           true,
		"mds":                true,
		"mds_stores":         true,
		"spotlight":          true,
		"trustd":             true,
		"tccd":               true,
		"loginwindow":        true,
		"controlcenter":      true,
		"notificationcenter": true,
	}

	processMap := make(map[string]ProcessInfo)
	for _, entry := range entries {
		nameLower := strings.ToLower(strings.TrimSpace(entry.Name))
		if nameLower == "" || systemProcesses[nameLower] || entry.PID == 0 {
			continue
		}
		if _, exists := processMap[nameLower]; !exists {
			processMap[nameLower] = ProcessInfo{Name: entry.Name, PID: entry.PID}
		}
	}

	processes := make([]ProcessInfo, 0, len(processMap))
	for _, proc := range processMap {
		processes = append(processes, proc)
	}
	sortProcesses(processes)
	return processes, nil
}

func GetProcessPIDByName(processName string) (uint32, error) {
	targetName := strings.TrimSpace(processName)
	if targetName == "" {
		return 0, fmt.Errorf("process name is empty")
	}

	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name, targetName) {
			return entry.PID, nil
		}
	}

	return 0, fmt.Errorf("process not found: %s", processName)
}

func IsProcessPresentByPID(pid uint32) bool {
	if pid == 0 {
		return false
	}
	err := syscall.Kill(int(pid), 0)
	return err == nil || err == syscall.EPERM
}

func GetDescendantProcesses(parentPID uint32) ([]ProcessInfo, error) {
	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}

	childrenByParent := make(map[uint32][]processSnapshotEntry)
	for _, entry := range entries {
		childrenByParent[entry.ParentPID] = append(childrenByParent[entry.ParentPID], entry)
	}

	seen := map[uint32]bool{parentPID: true}
	queue := []uint32{parentPID}
	descendants := make([]ProcessInfo, 0)

	for len(queue) > 0 {
		currentPID := queue[0]
		queue = queue[1:]

		for _, child := range childrenByParent[currentPID] {
			if seen[child.PID] {
				continue
			}
			seen[child.PID] = true
			queue = append(queue, child.PID)
			descendants = append(descendants, ProcessInfo{Name: child.Name, PID: child.PID})
		}
	}

	sortProcesses(descendants)
	return descendants, nil
}

func GetProcessesByExecutableDir(rootDir string) ([]ProcessInfo, error) {
	normalizedRoot, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return nil, fmt.Errorf("normalize executable dir: %w", err)
	}

	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0)
	seen := make(map[uint32]bool)
	for _, entry := range entries {
		if seen[entry.PID] {
			continue
		}
		imagePath, ok := queryProcessImagePath(entry.PID)
		if !ok || !isPathUnderDir(imagePath, normalizedRoot) {
			continue
		}

		seen[entry.PID] = true
		processes = append(processes, ProcessInfo{Name: entry.Name, PID: entry.PID})
	}

	sortProcesses(processes)
	return processes, nil
}

func FilterProcessesWithVisibleWindows(processes []ProcessInfo) []ProcessInfo {
	return processes
}

func HasVisibleTopLevelWindow(pid uint32) bool {
	return IsProcessPresentByPID(pid)
}

func IsProcessRunningByPID(pid uint32, ctx context.Context) bool {
	return IsProcessPresentByPID(pid)
}

type ProcessMonitor struct {
	stopOnce sync.Once
	stopChan chan struct{}
	exitChan chan struct{}
}

type SnapshotProcessMonitor struct {
	stopOnce sync.Once
	stopChan chan struct{}
	exitChan chan struct{}
}

func NewProcessMonitor(pid uint32) *ProcessMonitor {
	monitor := &ProcessMonitor{
		stopChan: make(chan struct{}),
		exitChan: make(chan struct{}),
	}
	go monitor.poll(pid)
	return monitor
}

func (pm *ProcessMonitor) poll(pid uint32) {
	defer close(pm.exitChan)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !IsProcessPresentByPID(pid) {
				return
			}
		case <-pm.stopChan:
			return
		}
	}
}

func (pm *ProcessMonitor) Start() (<-chan struct{}, error) {
	return pm.exitChan, nil
}

func (pm *ProcessMonitor) Stop() {
	if pm == nil {
		return
	}
	pm.stopOnce.Do(func() {
		close(pm.stopChan)
	})
}

func (pm *ProcessMonitor) WaitForProcessExit(timeout time.Duration) bool {
	if timeout == 0 {
		<-pm.exitChan
		return true
	}
	select {
	case <-pm.exitChan:
		return true
	case <-time.After(timeout):
		pm.Stop()
		return false
	}
}

func WaitForProcessExitAsync(pid uint32) (*ProcessMonitor, <-chan struct{}, error) {
	if pid == 0 {
		return nil, nil, fmt.Errorf("process id is zero")
	}
	monitor := NewProcessMonitor(pid)
	return monitor, monitor.exitChan, nil
}

func WaitForProcessHandleExitAsync(pid uint32, processHandle uintptr) (*ProcessMonitor, <-chan struct{}, error) {
	return WaitForProcessExitAsync(pid)
}

func NewSnapshotProcessMonitor(pid uint32) *SnapshotProcessMonitor {
	monitor := &SnapshotProcessMonitor{
		stopChan: make(chan struct{}),
		exitChan: make(chan struct{}),
	}
	go func() {
		defer close(monitor.exitChan)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !IsProcessPresentByPID(pid) {
					return
				}
			case <-monitor.stopChan:
				return
			}
		}
	}()
	return monitor
}

func (m *SnapshotProcessMonitor) Stop() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

func (m *SnapshotProcessMonitor) ExitChan() <-chan struct{} {
	if m == nil {
		exitChan := make(chan struct{})
		close(exitChan)
		return exitChan
	}
	return m.exitChan
}

func WaitForProcessExitBySnapshotAsync(pid uint32) (*SnapshotProcessMonitor, <-chan struct{}) {
	monitor := NewSnapshotProcessMonitor(pid)
	return monitor, monitor.ExitChan()
}

func getProcessSnapshotEntries() ([]processSnapshotEntry, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("enumerate processes: %w", err)
	}

	entries := make([]processSnapshotEntry, 0, len(kps))
	for _, kp := range kps {
		pid := uint32(kp.Proc.P_pid)
		if pid == 0 {
			continue
		}

		name := strings.TrimSpace(byteArrayToString(kp.Proc.P_comm[:]))
		if name == "" {
			if imagePath, ok := queryProcessImagePath(pid); ok {
				name = filepath.Base(imagePath)
			}
		}
		if name == "" {
			continue
		}

		entries = append(entries, processSnapshotEntry{
			Name:      name,
			PID:       pid,
			ParentPID: uint32(kp.Eproc.Ppid),
		})
	}
	return entries, nil
}

func queryProcessImagePath(pid uint32) (string, bool) {
	var buffer [4096]byte
	n, err := procPIDPath(int32(pid), unsafe.Pointer(&buffer[0]), uint32(len(buffer)))
	if err != nil || n <= 0 {
		return "", false
	}
	return strings.TrimRight(string(buffer[:n]), "\x00"), true
}

func isPathUnderDir(path string, rootDir string) bool {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(rootDir, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func sortProcesses(processes []ProcessInfo) {
	sort.Slice(processes, func(i, j int) bool {
		left := strings.ToLower(processes[i].Name)
		right := strings.ToLower(processes[j].Name)
		if left == right {
			return processes[i].PID < processes[j].PID
		}
		return left < right
	})
}

func byteArrayToString(bytes []byte) string {
	for i, b := range bytes {
		if b == 0 {
			return string(bytes[:i])
		}
	}
	return string(bytes)
}

func procPIDPath(pid int32, buffer unsafe.Pointer, bufferSize uint32) (int32, error) {
	ret := C.proc_pidpath(C.int(pid), buffer, C.uint32_t(bufferSize))
	if ret == 0 {
		return 0, fmt.Errorf("proc_pidpath returned empty path")
	}
	return int32(ret), nil
}
