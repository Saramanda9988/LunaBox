package ipccore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	ServerAddr  = "127.0.0.1"
	Port        = 56789
	PingTimeout = 500 * time.Millisecond
)

type CommandRequest struct {
	Args []string `json:"args"`
}

type CommandResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type endpointInfo struct {
	Port int `json:"port"`
}

func serverURLForPort(port int) string {
	return fmt.Sprintf("http://%s:%d", ServerAddr, port)
}

func endpointFilePath() string {
	return filepath.Join(os.TempDir(), "lunabox_ipc_endpoint.json")
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

func candidateServerURLs() []string {
	urls := make([]string, 0, 2)
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

func IsServerRunning() bool {
	_, ok := findRunningServerURL()
	return ok
}

func RemoteInstall(req interface{}) error {
	serverURL, ok := findRunningServerURL()
	if !ok {
		return fmt.Errorf("failed to connect to LunaBox: IPC server not running")
	}

	jsonBody, _ := json.Marshal(req)
	resp, err := http.Post(serverURL+"/install", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to connect to LunaBox: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LunaBox returned error status: %d", resp.StatusCode)
	}

	return nil
}

func RemoteRun(args []string) (string, error) {
	serverURL, ok := findRunningServerURL()
	if !ok {
		return "", fmt.Errorf("failed to connect to server: IPC server not running")
	}

	reqBody := CommandRequest{Args: args}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(serverURL+"/run", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned error status: %d", resp.StatusCode)
	}

	var cmdResp CommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&cmdResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if cmdResp.Error != "" {
		return cmdResp.Output, fmt.Errorf(cmdResp.Error)
	}

	return cmdResp.Output, nil
}
