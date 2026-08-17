//go:build linux

package processutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type processSnapshotEntry struct {
	Name      string
	PID       uint32
	ParentPID uint32
}

// ProcessDetails exposes Linux-only process metadata used by launcher
// detection. It intentionally keeps ProcessInfo unchanged for the UI-facing
// process picker while allowing Linux detectors to inspect /proc paths.
type ProcessDetails struct {
	ProcessInfo
	ParentPID        uint32
	ExecutablePath   string
	CurrentDirectory string
	CommandLine      []string
}

func StartProcess(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcessWithEnv(file, args, dir, nil)
}

func StartProcessWithEnv(file string, args []string, dir string, env []string) (*StartedProcess, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return nil, fmt.Errorf("executable path is empty")
	}

	cmd := exec.Command(file, args...)
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
		"bash": true, "cat": true, "dbus-daemon": true, "gnome-shell": true,
		"init": true, "kthreadd": true, "login": true, "sh": true,
		"sshd": true, "steam": true, "steamwebhelper": true, "systemd": true,
		"systemd-journal": true, "systemd-logind": true, "xwayland": true,
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
	if isZombieProcess(pid) {
		return false
	}
	err := syscall.Kill(int(pid), 0)
	return err == nil || err == syscall.EPERM
}

func isZombieProcess(pid uint32) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "stat"))
	if err != nil {
		return false
	}
	_, _, fields, ok := parseProcStat(string(stat))
	return ok && len(fields) > 0 && fields[0] == "Z"
}

func GetDescendantProcesses(parentPID uint32) ([]ProcessInfo, error) {
	details, err := GetDescendantProcessDetails(parentPID)
	if err != nil {
		return nil, err
	}

	descendants := make([]ProcessInfo, 0, len(details))
	for _, detail := range details {
		descendants = append(descendants, detail.ProcessInfo)
	}
	return descendants, nil
}

func GetDescendantProcessDetails(parentPID uint32) ([]ProcessDetails, error) {
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
	descendants := make([]ProcessDetails, 0)

	for len(queue) > 0 {
		currentPID := queue[0]
		queue = queue[1:]

		for _, child := range childrenByParent[currentPID] {
			if seen[child.PID] {
				continue
			}
			seen[child.PID] = true
			queue = append(queue, child.PID)
			descendants = append(descendants, processDetailsFromEntry(child))
		}
	}

	sortProcessDetails(descendants)
	return descendants, nil
}

func GetProcessCreationTime(pid uint32) (time.Time, error) {
	if pid == 0 {
		return time.Time{}, fmt.Errorf("invalid pid: 0")
	}

	stat, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "stat"))
	if err != nil {
		return time.Time{}, fmt.Errorf("read process stat: %w", err)
	}

	_, _, fields, ok := parseProcStat(string(stat))
	if !ok || len(fields) < 20 {
		return time.Time{}, fmt.Errorf("parse process stat for pid %d", pid)
	}

	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse process start time: %w", err)
	}
	bootTime, err := linuxBootTime()
	if err != nil {
		return time.Time{}, err
	}
	return bootTime.Add(time.Duration(startTicks) * time.Second / 100), nil
}

func GetProcessesByExecutableDir(rootDir string) ([]ProcessInfo, error) {
	details, err := GetProcessDetailsByExecutableDir(rootDir)
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0, len(details))
	for _, detail := range details {
		processes = append(processes, detail.ProcessInfo)
	}
	return processes, nil
}

func GetProcessDetailsByExecutableDir(rootDir string) ([]ProcessDetails, error) {
	normalizedRoot, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return nil, fmt.Errorf("normalize executable dir: %w", err)
	}

	entries, err := getProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessDetails, 0)
	seen := make(map[uint32]bool)
	for _, entry := range entries {
		detail := processDetailsFromEntry(entry)
		if seen[entry.PID] || !processDetailsMatchesRoot(detail, normalizedRoot) {
			continue
		}
		seen[entry.PID] = true
		processes = append(processes, detail)
	}

	sortProcessDetails(processes)
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
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("enumerate processes: %w", err)
	}

	processes := make([]processSnapshotEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid64, err := strconv.ParseUint(entry.Name(), 10, 32)
		if err != nil || pid64 == 0 {
			continue
		}

		statPath := filepath.Join("/proc", entry.Name(), "stat")
		stat, err := os.ReadFile(statPath)
		if err != nil {
			continue
		}
		name, parentPID, fields, ok := parseProcStat(string(stat))
		if !ok {
			continue
		}
		if len(fields) > 0 && fields[0] == "Z" {
			continue
		}
		if name == "" {
			if imagePath, ok := queryProcessImagePath(uint32(pid64)); ok {
				name = filepath.Base(imagePath)
			}
		}
		if name == "" {
			continue
		}

		processes = append(processes, processSnapshotEntry{
			Name:      name,
			PID:       uint32(pid64),
			ParentPID: parentPID,
		})
	}
	return processes, nil
}

func parseProcStat(stat string) (string, uint32, []string, bool) {
	stat = strings.TrimSpace(stat)
	open := strings.Index(stat, "(")
	close := strings.LastIndex(stat, ")")
	if open < 0 || close <= open || close+2 >= len(stat) {
		return "", 0, nil, false
	}

	name := strings.TrimSpace(stat[open+1 : close])
	fields := strings.Fields(strings.TrimSpace(stat[close+1:]))
	if len(fields) < 2 {
		return "", 0, nil, false
	}
	ppid, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return "", 0, nil, false
	}
	return name, uint32(ppid), fields, true
}

func queryProcessImagePath(pid uint32) (string, bool) {
	path, err := os.Readlink(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "exe"))
	if err != nil || strings.TrimSpace(path) == "" {
		return "", false
	}
	return strings.TrimSuffix(path, " (deleted)"), true
}

func queryProcessCurrentDirectory(pid uint32) string {
	path, err := os.Readlink(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "cwd"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(path)
}

func readProcessCommandLine(pid uint32) []string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "cmdline"))
	if err != nil || len(raw) == 0 {
		return nil
	}
	parts := strings.Split(string(raw), "\x00")
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
		}
	}
	return args
}

func GetProcessCommandInfo(pid uint32) (ProcessCommandInfo, error) {
	if pid == 0 {
		return ProcessCommandInfo{}, fmt.Errorf("process id is zero")
	}

	info := ProcessCommandInfo{
		Arguments:   readProcessCommandLine(pid),
		Environment: readProcessEnvironment(pid),
	}
	if imagePath, ok := queryProcessImagePath(pid); ok {
		info.ExecutablePath = imagePath
	}
	if len(info.Arguments) == 0 && info.ExecutablePath == "" {
		return ProcessCommandInfo{}, fmt.Errorf("read process command info for pid %d", pid)
	}
	return info, nil
}

func readProcessEnvironment(pid uint32) map[string]string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "environ"))
	if err != nil || len(raw) == 0 {
		return nil
	}
	environment := make(map[string]string)
	for _, part := range strings.Split(string(raw), "\x00") {
		key, value, ok := strings.Cut(part, "=")
		if ok && key != "" {
			environment[key] = value
		}
	}
	return environment
}

func processDetailsFromEntry(entry processSnapshotEntry) ProcessDetails {
	detail := ProcessDetails{
		ProcessInfo: ProcessInfo{
			Name: entry.Name,
			PID:  entry.PID,
		},
		ParentPID:        entry.ParentPID,
		CurrentDirectory: queryProcessCurrentDirectory(entry.PID),
		CommandLine:      readProcessCommandLine(entry.PID),
	}
	if imagePath, ok := queryProcessImagePath(entry.PID); ok {
		detail.ExecutablePath = imagePath
	}
	return detail
}

func processMatchesRoot(pid uint32, rootDir string) bool {
	if imagePath, ok := queryProcessImagePath(pid); ok && isPathUnderDir(imagePath, rootDir) {
		return true
	}

	pidText := strconv.FormatUint(uint64(pid), 10)
	if cwdPath, err := os.Readlink(filepath.Join("/proc", pidText, "cwd")); err == nil && isPathUnderDir(cwdPath, rootDir) {
		return true
	}

	cmdline, err := os.ReadFile(filepath.Join("/proc", pidText, "cmdline"))
	if err != nil || len(cmdline) == 0 {
		return false
	}
	for _, raw := range strings.Split(string(cmdline), "\x00") {
		if processArgMatchesRoot(raw, rootDir) {
			return true
		}
	}
	return false
}

func processDetailsMatchesRoot(detail ProcessDetails, rootDir string) bool {
	if strings.TrimSpace(rootDir) == "" {
		return false
	}
	if detail.ExecutablePath != "" && isPathUnderDir(detail.ExecutablePath, rootDir) {
		return true
	}
	if detail.CurrentDirectory != "" && isPathUnderDir(detail.CurrentDirectory, rootDir) {
		return true
	}
	for _, arg := range detail.CommandLine {
		if processArgMatchesRoot(arg, rootDir) {
			return true
		}
	}
	return false
}

func processArgMatchesRoot(arg string, rootDir string) bool {
	arg = strings.Trim(strings.TrimSpace(arg), "\"'")
	if arg == "" {
		return false
	}
	if filepath.IsAbs(arg) {
		return isPathUnderDir(arg, rootDir)
	}
	return strings.Contains(filepath.ToSlash(arg), filepath.ToSlash(rootDir))
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

func linuxBootTime() (time.Time, error) {
	stat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, fmt.Errorf("read /proc/stat: %w", err)
	}
	for _, line := range strings.Split(string(stat), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "btime" {
			seconds, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return time.Time{}, fmt.Errorf("parse boot time: %w", err)
			}
			return time.Unix(seconds, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("boot time not found")
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

func sortProcessDetails(processes []ProcessDetails) {
	sort.Slice(processes, func(i, j int) bool {
		left := strings.ToLower(processes[i].Name)
		right := strings.ToLower(processes[j].Name)
		if left == right {
			return processes[i].PID < processes[j].PID
		}
		return left < right
	})
}
