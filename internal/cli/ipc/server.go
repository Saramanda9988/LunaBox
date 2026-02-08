package ipc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"lunabox/internal/applog"
	"lunabox/internal/cli"
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
