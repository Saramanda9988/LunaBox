package ipcserver

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"lunabox/internal/cli/ipccore"
)

const (
	ServerAddr = "127.0.0.1"
	Port       = 56789
	PortMax    = 56820
)

func serverURLForPort(port int) string {
	return fmt.Sprintf("http://%s:%d", ServerAddr, port)
}

func endpointFilePath() string {
	return filepath.Join(os.TempDir(), "lunabox_ipc_endpoint.json")
}

type endpointInfo struct {
	Port int `json:"port"`
}

func readSavedPort() (int, bool) {
	content, err := os.ReadFile(endpointFilePath())
	if err != nil {
		return 0, false
	}
	var info endpointInfo
	if err := json.Unmarshal(content, &info); err != nil {
		return 0, false
	}
	if info.Port <= 0 {
		return 0, false
	}
	return info.Port, true
}

func savePort(port int) {
	if port <= 0 {
		return
	}
	content, err := json.Marshal(endpointInfo{Port: port})
	if err != nil {
		return
	}
	_ = os.WriteFile(endpointFilePath(), content, 0644)
}

func chooseIPCListener() (net.Listener, int, error) {
	ports := make([]int, 0, (PortMax-Port)+2)
	if savedPort, ok := readSavedPort(); ok {
		ports = append(ports, savedPort)
	}
	for p := Port; p <= PortMax; p++ {
		ports = append(ports, p)
	}

	seen := make(map[int]struct{})
	for _, p := range ports {
		if p <= 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}

		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ServerAddr, p))
		if err == nil {
			return ln, p, nil
		}
	}

	return nil, 0, fmt.Errorf("no available ipc port in range %d-%d", Port, PortMax)
}

type CommandRequest = ipccore.CommandRequest
type CommandResponse = ipccore.CommandResponse

// InstallResponse IPC /install 响应
type InstallResponse struct {
	TaskID string `json:"task_id,omitempty"`
	Error  string `json:"error,omitempty"`
}
