package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	ServerAddr  = "127.0.0.1"
	Port        = 56789
	PortMax     = 56820
	PingTimeout = 500 * time.Millisecond
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

func candidateServerURLs() []string {
	urls := make([]string, 0, 3)
	seen := make(map[string]struct{})
	add := func(u string) {
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}

	if savedPort, ok := readSavedPort(); ok {
		add(serverURLForPort(savedPort))
	}
	add(serverURLForPort(Port))
	return urls
}

func pingServer(url string) bool {
	client := http.Client{Timeout: PingTimeout}
	resp, err := client.Get(url + "/ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func findRunningServerURL() (string, bool) {
	for _, url := range candidateServerURLs() {
		if pingServer(url) {
			return url, true
		}
	}
	return "", false
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

type CommandRequest struct {
	Args []string `json:"args"`
}

type CommandResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// InstallResponse IPC /install 响应
type InstallResponse struct {
	TaskID string `json:"task_id,omitempty"`
	Error  string `json:"error,omitempty"`
}
