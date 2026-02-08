package ipc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lunabox/internal/applog"
	"lunabox/internal/cli"
)

const (
	Port        = 56789
	ServerAddr  = "127.0.0.1"
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

// StartServer 启动 IPC 服务器 (在 GUI 进程中运行)
func StartServer(app *cli.CoreApp) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	mux.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CommandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// 捕获输出
		var outputBuf bytes.Buffer
		err := cli.RunCommand(&outputBuf, app, req.Args)

		resp := CommandResponse{
			Output: outputBuf.String(),
		}
		if err != nil {
			resp.Error = err.Error()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", ServerAddr, Port),
		Handler: mux,
	}

	applog.LogInfof(app.Ctx, "IPC Server starting on %s", server.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applog.LogErrorf(app.Ctx, "IPC Server failed: %v", err)
		}
	}()
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
