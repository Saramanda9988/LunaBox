package utils

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessInfo 进程信息
type ProcessInfo struct {
	Name string `json:"name"` // 进程名
	PID  uint32 `json:"pid"`  // 进程ID
}

// GetRunningProcesses 获取系统中正在运行的进程列表
// 只返回有意义的exe进程，过滤掉系统进程和常见的无关进程
func GetRunningProcesses() ([]ProcessInfo, error) {
	// 使用tasklist获取进程列表，CSV格式便于解析
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
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
func GetProcessPIDByName(processName string) (uint32, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", processName), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || strings.Contains(outputStr, "No tasks are running") {
		return 0, fmt.Errorf("process not found: %s", processName)
	}

	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		name := strings.Trim(parts[0], "\"")
		if strings.EqualFold(name, processName) {
			pidStr := strings.Trim(parts[1], "\"")
			pid, err := strconv.ParseUint(pidStr, 10, 32)
			if err != nil {
				continue
			}
			return uint32(pid), nil
		}
	}

	return 0, fmt.Errorf("process not found: %s", processName)
}

// IsProcessRunningByPID 检查指定PID的进程是否仍在运行
func IsProcessRunningByPID(pid uint32) bool {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := strings.TrimSpace(string(output))
	// 如果输出为空或包含"No tasks"说明进程不存在
	if outputStr == "" || strings.Contains(outputStr, "No tasks are running") {
		return false
	}

	return true
}
