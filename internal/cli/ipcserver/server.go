package ipcserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"lunabox/internal/applog"
	"lunabox/internal/cli"
	"lunabox/internal/vo"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartServer 启动 IPC 服务器 (在 GUI 进程中运行)
func StartServer(app *cli.CoreApp) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	mux.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic in CLI handler: %v", r)
				applog.LogErrorf(app.Ctx, "%v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}()

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

	// /install: 接收来自新启动实例转发的 lunabox:// 安装请求
	mux.HandleFunc("/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req vo.InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		// 不直接开始下载，只推送事件让前端弹出确认窗口
		// 用户确认后前端调用 DownloadService.StartDownload
		runtime.EventsEmit(app.Ctx, "install:pending", req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(InstallResponse{TaskID: ""})
	})

	listener, port, err := chooseIPCListener()
	if err != nil {
		applog.LogErrorf(app.Ctx, "IPC Server failed to acquire port: %v", err)
		return
	}
	savePort(port)
	server := &http.Server{Handler: mux}

	applog.LogInfof(app.Ctx, "IPC Server starting on %s", listener.Addr().String())
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			applog.LogErrorf(app.Ctx, "IPC Server failed: %v", err)
		}
	}()
}
