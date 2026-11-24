//go:build darwin

package processutils

/*
#cgo darwin LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <libproc.h>
#include <stdint.h>
#include <stdlib.h>

// lunabox_visible_window_pids enumerates PIDs owning at least one visible,
// non-transparent, non-empty top-level (layer 0) on-screen window.
// Returns 1 on success (even if *out_count == 0) and 0 if CoreGraphics could
// not query the window server.
static int lunabox_visible_window_pids(uint32_t **out_pids, int *out_count) {
	*out_pids = NULL;
	*out_count = 0;

	CFArrayRef windowList = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID
	);
	if (windowList == NULL) {
		return 0;
	}

	CFIndex total = CFArrayGetCount(windowList);
	if (total <= 0) {
		CFRelease(windowList);
		return 1;
	}

	uint32_t *pids = (uint32_t *)malloc((size_t)total * sizeof(uint32_t));
	if (pids == NULL) {
		CFRelease(windowList);
		return 0;
	}

	int count = 0;
	for (CFIndex i = 0; i < total; i++) {
		CFDictionaryRef info = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);
		if (info == NULL) {
			continue;
		}

		// Standard application windows live on layer 0 (kCGNormalWindowLevel).
		// Menu bar items, overlays, and system chrome sit on other layers.
		CFNumberRef layerRef = (CFNumberRef)CFDictionaryGetValue(info, kCGWindowLayer);
		int32_t layer = -1;
		if (layerRef == NULL || !CFNumberGetValue(layerRef, kCFNumberSInt32Type, &layer) || layer != 0) {
			continue;
		}

		CFNumberRef alphaRef = (CFNumberRef)CFDictionaryGetValue(info, kCGWindowAlpha);
		double alpha = 1.0;
		if (alphaRef != NULL && CFNumberGetValue(alphaRef, kCFNumberDoubleType, &alpha) && alpha <= 0.0) {
			continue;
		}

		CFDictionaryRef boundsRef = (CFDictionaryRef)CFDictionaryGetValue(info, kCGWindowBounds);
		CGRect bounds;
		if (boundsRef != NULL && CGRectMakeWithDictionaryRepresentation(boundsRef, &bounds)) {
			if (bounds.size.width < 1.0 || bounds.size.height < 1.0) {
				continue;
			}
		}

		CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(info, kCGWindowOwnerPID);
		int32_t pid = 0;
		if (pidRef == NULL || !CFNumberGetValue(pidRef, kCFNumberSInt32Type, &pid) || pid <= 0) {
			continue;
		}
		pids[count++] = (uint32_t)pid;
	}

	CFRelease(windowList);
	if (count == 0) {
		free(pids);
		return 1;
	}
	*out_pids = pids;
	*out_count = count;
	return 1;
}
*/
import "C"

import (
	"context"
	"encoding/binary"
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

func StartProcessHidden(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcess(file, args, dir)
}

func StartProcessElevatedHidden(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcessElevated(file, args, dir)
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
	processes, err := GetProcessesByName(processName)
	if err != nil {
		return 0, err
	}
	if len(processes) == 0 {
		return 0, fmt.Errorf("process not found: %s", processName)
	}
	return processes[0].PID, nil
}

func GetProcessesByName(processName string) ([]ProcessInfo, error) {
	targetName := strings.TrimSpace(processName)
	if targetName == "" {
		return nil, fmt.Errorf("process name is empty")
	}

	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0, 1)
	seen := make(map[uint32]bool)
	for _, entry := range entries {
		if seen[entry.PID] {
			continue
		}
		matchedName, ok := matchProcessName(entry, targetName)
		if !ok {
			continue
		}
		seen[entry.PID] = true
		processes = append(processes, ProcessInfo{Name: matchedName, PID: entry.PID})
	}
	sortProcesses(processes)
	return processes, nil
}

func matchProcessName(entry processSnapshotEntry, targetName string) (string, bool) {
	if strings.EqualFold(entry.Name, targetName) {
		return entry.Name, true
	}
	// macOS kinfo_proc.p_comm is truncated to MAXCOMLEN (15 chars + NUL).
	// When the target name is longer and shares the truncated prefix, check
	// the full executable basename via proc_pidpath.
	if len(targetName) > len(entry.Name) && len(entry.Name) >= 15 &&
		strings.EqualFold(targetName[:len(entry.Name)], entry.Name) {
		if imagePath, ok := queryProcessImagePath(entry.PID); ok {
			base := filepath.Base(imagePath)
			if strings.EqualFold(base, targetName) {
				return base, true
			}
		}
	}
	return "", false
}

func IsProcessPresentByPID(pid uint32) bool {
	if pid == 0 {
		return false
	}
	err := syscall.Kill(int(pid), 0)
	return err == nil || err == syscall.EPERM
}

// GetProcessCommandInfo reads argv and environment from macOS KERN_PROCARGS2.
// This remains available for Wine processes whose executable image path cannot
// be resolved through proc_pidpath.
func GetProcessCommandInfo(pid uint32) (ProcessCommandInfo, error) {
	if pid == 0 {
		return ProcessCommandInfo{}, fmt.Errorf("process id is zero")
	}

	data, err := unix.SysctlRaw("kern.procargs2", int(pid))
	if err != nil {
		return ProcessCommandInfo{}, fmt.Errorf("read process arguments for pid %d: %w", pid, err)
	}
	info, err := parseKernProcArgs(data)
	if err != nil {
		return ProcessCommandInfo{}, fmt.Errorf("parse process arguments for pid %d: %w", pid, err)
	}
	return info, nil
}

func parseKernProcArgs(data []byte) (ProcessCommandInfo, error) {
	const argcSize = 4
	if len(data) < argcSize {
		return ProcessCommandInfo{}, fmt.Errorf("process arguments buffer is too short")
	}

	argc := int(int32(binary.NativeEndian.Uint32(data[:argcSize])))
	if argc < 0 {
		return ProcessCommandInfo{}, fmt.Errorf("invalid argument count: %d", argc)
	}

	executablePath, offset, ok := readCString(data, argcSize)
	if !ok {
		return ProcessCommandInfo{}, fmt.Errorf("executable path is not terminated")
	}
	for offset < len(data) && data[offset] == 0 {
		offset++
	}

	arguments := make([]string, 0, argc)
	for len(arguments) < argc {
		argument, next, terminated := readCString(data, offset)
		if !terminated {
			return ProcessCommandInfo{}, fmt.Errorf("argument %d is not terminated", len(arguments))
		}
		arguments = append(arguments, argument)
		offset = next
	}

	environment := make(map[string]string)
	for offset < len(data) {
		if data[offset] == 0 {
			offset++
			continue
		}
		value, next, terminated := readCString(data, offset)
		if !terminated {
			break
		}
		offset = next
		key, envValue, found := strings.Cut(value, "=")
		if found && key != "" {
			environment[key] = envValue
		}
	}

	return ProcessCommandInfo{
		ExecutablePath: executablePath,
		Arguments:      arguments,
		Environment:    environment,
	}, nil
}

func readCString(data []byte, offset int) (string, int, bool) {
	if offset < 0 || offset >= len(data) {
		return "", offset, false
	}
	for index := offset; index < len(data); index++ {
		if data[index] == 0 {
			return string(data[offset:index]), index + 1, true
		}
	}
	return "", offset, false
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

func IsProcessDescendant(rootPID uint32, candidatePID uint32) bool {
	if rootPID == 0 || candidatePID == 0 {
		return false
	}
	if rootPID == candidatePID {
		return true
	}
	descendants, err := GetDescendantProcesses(rootPID)
	if err != nil {
		return false
	}
	for _, process := range descendants {
		if process.PID == candidatePID {
			return true
		}
	}
	return false
}

// GetProcessCreationTime queries the process start time via kern.proc.pid.
func GetProcessCreationTime(pid uint32) (time.Time, error) {
	if pid == 0 {
		return time.Time{}, fmt.Errorf("process id is zero")
	}
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", int(pid))
	if err != nil {
		return time.Time{}, fmt.Errorf("query process %d info: %w", pid, err)
	}
	tv := kp.Proc.P_starttime
	if tv.Sec == 0 && tv.Usec == 0 {
		return time.Time{}, fmt.Errorf("process %d has zero start time", pid)
	}
	return time.Unix(int64(tv.Sec), int64(tv.Usec)*int64(time.Microsecond)), nil
}

func GetProcessesByExecutableDir(rootDir string) ([]ProcessInfo, error) {
	normalizedRoot, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return nil, fmt.Errorf("normalize executable dir: %w", err)
	}
	resolvedRoot := normalizedRoot
	if eval, evalErr := filepath.EvalSymlinks(normalizedRoot); evalErr == nil && strings.TrimSpace(eval) != "" {
		resolvedRoot = eval
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
		if !ok || (!isPathUnderDir(imagePath, normalizedRoot) && (resolvedRoot == normalizedRoot || !isPathUnderDir(imagePath, resolvedRoot))) {
			continue
		}

		name := entry.Name
		if base := strings.TrimSpace(filepath.Base(imagePath)); base != "" && base != "." && base != string(filepath.Separator) {
			name = base
		}
		seen[entry.PID] = true
		processes = append(processes, ProcessInfo{Name: name, PID: entry.PID})
	}

	sortProcesses(processes)
	return processes, nil
}

// VisibleWindowPIDs returns the set of PIDs currently owning at least one
// visible top-level window on macOS. The boolean reports whether CoreGraphics
// window enumeration succeeded.
func VisibleWindowPIDs() (map[uint32]bool, bool) {
	var rawPIDs *C.uint32_t
	var count C.int
	if C.lunabox_visible_window_pids(&rawPIDs, &count) == 0 {
		return nil, false
	}
	result := make(map[uint32]bool, int(count))
	if rawPIDs != nil {
		defer C.free(unsafe.Pointer(rawPIDs))
		slice := unsafe.Slice((*uint32)(unsafe.Pointer(rawPIDs)), int(count))
		for _, pid := range slice {
			if pid != 0 {
				result[pid] = true
			}
		}
	}
	return result, true
}

func FilterProcessesWithVisibleWindows(processes []ProcessInfo) []ProcessInfo {
	if len(processes) == 0 {
		return nil
	}
	windowPIDs, ok := VisibleWindowPIDs()
	if !ok {
		return nil
	}
	filtered := make([]ProcessInfo, 0, len(processes))
	for _, proc := range processes {
		if windowPIDs[proc.PID] {
			filtered = append(filtered, proc)
		}
	}
	return filtered
}

func HasVisibleTopLevelWindow(pid uint32) bool {
	if pid == 0 {
		return false
	}
	windowPIDs, ok := VisibleWindowPIDs()
	if !ok {
		return false
	}
	return windowPIDs[pid]
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
