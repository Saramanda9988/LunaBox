package ipcclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	ServerURL   = "http://127.0.0.1:56789"
	PingTimeout = 500 * time.Millisecond
)

type CommandRequest struct {
	Args []string `json:"args"`
}

type CommandResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// IsServerRunning 检查 Server 是否在运行
func IsServerRunning() bool {
	client := http.Client{
		Timeout: PingTimeout,
	}
	resp, err := client.Get(ServerURL + "/ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// RemoteRun 在远程 Server 上执行命令
func RemoteRun(args []string) error {
	reqBody := CommandRequest{Args: args}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(ServerURL+"/run", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned error status: %d", resp.StatusCode)
	}

	var cmdResp CommandResponse
	if err := json.NewDecoder(resp.Body).Decode(&cmdResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// 打印输出
	fmt.Print(cmdResp.Output)

	if cmdResp.Error != "" {
		return fmt.Errorf(cmdResp.Error)
	}

	return nil
}
