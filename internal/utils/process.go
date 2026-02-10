package utils

import (
	"context"
	"fmt"
	"lunabox/internal/applog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	SYNCHRONIZE               = 0x00100000
	WAIT_OBJECT_0             = 0
	STILL_ACTIVE              = 259
	WAIT_TIMEOUT              = 258
	WAIT_FAILED               = 0xFFFFFFFF
	INFINITE                  = 0xFFFFFFFF
	PROCESS_QUERY_INFORMATION = 0x0400
	TH32CS_SNAPPROCESS        = 0x00000002
	MAX_PATH                  = 260
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procWaitForSingleObject      = kernel32.NewProc("WaitForSingleObject")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
)

// ProcessInfo 进程信息
type ProcessInfo struct {
	Name string `json:"name"` // 进程名
	PID  uint32 `json:"pid"`  // 进程ID
}

// PROCESSENTRY32W Windows API 进程快照结构体
type PROCESSENTRY32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [MAX_PATH]uint16
}

// CheckIfProcessRunning 检查指定进程是否正在运行
// 使用 Windows API 代替 tasklist，避免编码问题
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

// GetRunningProcesses 获取系统中正在运行的进程列表
// 只返回有意义的exe进程，过滤掉系统进程和常见的无关进程
// 使用 Windows API 代替 tasklist，避免编码和语言问题
func GetRunningProcesses() ([]ProcessInfo, error) {
	// 使用tasklist获取进程列表，CSV格式便于解析
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	// 需要过滤的系统进程和常见无关进程
	systemProcesses := map[string]bool{
		"system":                      true,
		"registry":                    true,
		"smss.exe":                    true,
		"csrss.exe":                   true,
		"wininit.exe":                 true,
		"services.exe":                true,
		"lsass.exe":                   true,
		"winlogon.exe":                true,
		"fontdrvhost.exe":             true,
		"dwm.exe":                     true,
		"svchost.exe":                 true,
		"sihost.exe":                  true,
		"taskhostw.exe":               true,
		"explorer.exe":                true,
		"runtimebroker.exe":           true,
		"searchhost.exe":              true,
		"startmenuexperiencehost.exe": true,
		"textinputhost.exe":           true,
		"ctfmon.exe":                  true,
		"conhost.exe":                 true,
		"dllhost.exe":                 true,
		"spoolsv.exe":                 true,
		"searchindexer.exe":           true,
		"securityhealthservice.exe":   true,
		"securityhealthsystray.exe":   true,
		"smartscreen.exe":             true,
		"applicationframehost.exe":    true,
		"windowsterminal.exe":         true,
		"cmd.exe":                     true,
		"powershell.exe":              true,
		"pwsh.exe":                    true,
		"taskmgr.exe":                 true,
		"systemsettings.exe":          true,
		"lockapp.exe":                 true,
		"shellexperiencehost.exe":     true,
		"wudfhost.exe":                true,
		"dashost.exe":                 true,
		"wmiprvse.exe":                true,
		"mpcmdrun.exe":                true,
		"audiodg.exe":                 true,
		"unsecapp.exe":                true,
	}

	lines := strings.Split(string(output), "\n")
	processMap := make(map[string]ProcessInfo) // 使用map去重，只保留每个进程名的第一个实例

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// CSV格式: "Image Name","PID","Session Name","Session#","Mem Usage"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		// 去除引号
		name := strings.Trim(parts[0], "\"")
		pidStr := strings.Trim(parts[1], "\"")

		// 跳过系统进程
		if systemProcesses[strings.ToLower(name)] {
			continue
		}

		// 只保留.exe文件
		if !strings.HasSuffix(strings.ToLower(name), ".exe") {
			continue
		}

		pid, err := strconv.ParseUint(pidStr, 10, 32)
		if err != nil {
			continue
		}

		// 跳过PID为0或4的系统进程
		if pid == 0 || pid == 4 {
			continue
		}

		// 如果该进程名还没有记录，则添加
		if _, exists := processMap[name]; !exists {
			processMap[name] = ProcessInfo{
				Name: name,
				PID:  uint32(pid),
			}
		}
	}

	// 转换为切片
	processes := make([]ProcessInfo, 0, len(processMap))
	for _, proc := range processMap {
		processes = append(processes, proc)
	}

	return processes, nil
}

// GetProcessPIDByName 根据进程名获取PID
// 如果有多个同名进程，返回第一个找到的
// 使用 Windows API (CreateToolhelp32Snapshot) 代替 tasklist，避免编码和语言问题
func GetProcessPIDByName(processName string) (uint32, error) {
	// 创建进程快照
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(
		uintptr(TH32CS_SNAPPROCESS),
		0,
	)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return 0, fmt.Errorf("failed to create process snapshot: %w", err)
	}
	defer procCloseHandle.Call(snapshot)

	// 准备进程信息结构体
	var pe32 PROCESSENTRY32W
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	// 获取第一个进程
	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
	if ret == 0 {
		return 0, fmt.Errorf("failed to get first process")
	}

	// 转换进程名为小写以进行不区分大小写的比较
	targetName := strings.ToLower(processName)

	// 遍历所有进程
	for {
		// 从 UTF-16 转换进程名
		exeName := syscall.UTF16ToString(pe32.ExeFile[:])

		// 不区分大小写比较
		if strings.EqualFold(exeName, targetName) {
			return pe32.ProcessID, nil
		}

		// 获取下一个进程
		ret, _, _ := procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
		if ret == 0 {
			break
		}
	}

	return 0, fmt.Errorf("process not found: %s", processName)
}

// IsProcessRunningByPID 检查指定PID的进程是否仍在运行
// 使用 Windows API OpenProcess + GetExitCodeProcess，避免 tasklist 的编码问题
func IsProcessRunningByPID(pid uint32, ctx context.Context) bool {
	// 尝试打开进程句柄
	handle, _, err := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_INFORMATION),
		0,
		uintptr(pid),
	)

	// 无法打开句柄，进程不存在
	if handle == 0 {
		applog.LogWarningf(ctx, "%s | [PROCESS_CHECK] PID %d NOT running (OpenProcess failed: %v)", time.Now(), pid, err)
		return false
	}
	defer procCloseHandle.Call(handle)

	// 检查进程退出码
	var exitCode uint32
	procGetExitCodeProcess := kernel32.NewProc("GetExitCodeProcess")
	ret, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&exitCode)))

	if ret == 0 {
		// GetExitCodeProcess 失败，假设进程不存在
		applog.LogWarningf(ctx, "%s | [PROCESS_CHECK] PID %d NOT running (GetExitCodeProcess failed)", time.Now(), pid)
		return false
	}

	// STILL_ACTIVE = 259，表示进程仍在运行
	if exitCode == STILL_ACTIVE {
		applog.LogWarningf(ctx, "%s | [PROCESS_CHECK] PID %d IS running", time.Now(), pid)
		return true
	}

	// 进程已退出
	applog.LogWarningf(ctx, "%s | [PROCESS_CHECK] PID %d NOT running (exit code: %d)", time.Now(), pid, exitCode)
	return false
}

//====================ProcessMonitor====================

// ProcessMonitor 进程监控器
// 使用 WaitForSingleObject 实现事件驱动的进程退出检测
type ProcessMonitor struct {
	mu            sync.Mutex
	pid           uint32
	processHandle uintptr
	running       bool
	stopChan      chan struct{}
	exitChan      chan struct{} // 进程退出通知
}

// NewProcessMonitor 创建进程监控器
func NewProcessMonitor(pid uint32) *ProcessMonitor {
	return &ProcessMonitor{
		pid:      pid,
		stopChan: make(chan struct{}),
		exitChan: make(chan struct{}),
	}
}

// Start 开始监控进程
// 返回一个 channel，当进程退出时会被关闭
func (pm *ProcessMonitor) Start() (<-chan struct{}, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return pm.exitChan, nil
	}

	// 打开进程句柄
	handle, _, err := procOpenProcess.Call(
		uintptr(SYNCHRONIZE|PROCESS_QUERY_INFORMATION),
		0,
		uintptr(pm.pid),
	)
	if handle == 0 {
		return nil, fmt.Errorf("failed to open process %d: %v", pm.pid, err)
	}

	pm.processHandle = handle
	pm.running = true

	// 启动监控 goroutine
	go pm.monitorLoop()

	return pm.exitChan, nil
}

// monitorLoop 监控循环
func (pm *ProcessMonitor) monitorLoop() {
	defer pm.cleanup()

	// 使用带超时的 WaitForSingleObject
	// 每秒检查一次是否需要停止，避免无法响应 Stop 调用
	for {
		select {
		case <-pm.stopChan:
			return
		default:
			// 等待进程退出，超时1秒
			ret, _, _ := procWaitForSingleObject.Call(
				pm.processHandle,
				uintptr(1000), // 1秒超时
			)

			switch ret {
			case WAIT_OBJECT_0:
				// 进程已退出
				close(pm.exitChan)
				return
			case WAIT_TIMEOUT:
				// 超时，继续等待
				continue
			case WAIT_FAILED:
				// 等待失败（可能进程句柄无效）
				close(pm.exitChan)
				return
			default:
				// 其他情况，继续等待
				continue
			}
		}
	}
}

// cleanup 清理资源
func (pm *ProcessMonitor) cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.processHandle != 0 {
		procCloseHandle.Call(pm.processHandle)
		pm.processHandle = 0
	}
	pm.running = false
}

// Stop 停止监控
func (pm *ProcessMonitor) Stop() {
	pm.mu.Lock()
	if !pm.running {
		pm.mu.Unlock()
		return
	}
	pm.mu.Unlock()

	// 发送停止信号
	select {
	case <-pm.stopChan:
		// 已经关闭
	default:
		close(pm.stopChan)
	}
}

// WaitForProcessExit 等待进程退出（阻塞）
// timeout: 最大等待时间，0 表示无限等待
// 返回: true 表示进程已退出，false 表示超时或被取消
func (pm *ProcessMonitor) WaitForProcessExit(timeout time.Duration) bool {
	exitChan, err := pm.Start()
	if err != nil {
		return true // 无法打开进程，认为已退出
	}

	if timeout == 0 {
		<-exitChan
		return true
	}

	select {
	case <-exitChan:
		return true
	case <-time.After(timeout):
		pm.Stop()
		return false
	}
}

// WaitForProcessExitAsync 异步等待进程退出
// 返回一个 channel，当进程退出时会被关闭
// 调用者需要在不再需要时调用 Stop()
func WaitForProcessExitAsync(pid uint32) (*ProcessMonitor, <-chan struct{}, error) {
	pm := NewProcessMonitor(pid)
	exitChan, err := pm.Start()
	if err != nil {
		return nil, nil, err
	}
	return pm, exitChan, nil
}
