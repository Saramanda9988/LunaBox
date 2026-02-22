package ipc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

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

// RemoteInstall 将 InstallRequest 转发给运行中的 GUI 处理
func RemoteInstall(req interface{}) error {
	jsonBody, _ := json.Marshal(req)
	resp, err := http.Post(ServerURL+"/install", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to connect to LunaBox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LunaBox returned error status: %d", resp.StatusCode)
	}
	return nil
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
