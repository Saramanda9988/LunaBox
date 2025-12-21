package onedrive

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// 本地回调服务器配置
const (
	localCallbackPort = 23456
	localCallbackPath = "/callback"
)

// OneDriveAuthResult OAuth 授权结果
type OneDriveAuthResult struct {
	Code  string
	Error string
}

// 全局授权回调通道
var (
	authResultChan chan OneDriveAuthResult
	authServer     *http.Server
	authServerMu   sync.Mutex
)

// StartOneDriveAuthServer 启动本地回调服务器，等待授权回调
func StartOneDriveAuthServer(ctx context.Context, timeout time.Duration) (string, error) {
	authServerMu.Lock()
	defer authServerMu.Unlock()

	authResultChan = make(chan OneDriveAuthResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(localCallbackPath, handleOAuthCallback)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localCallbackPort))
	if err != nil {
		return "", fmt.Errorf("无法启动本地服务器: %w", err)
	}

	authServer = &http.Server{Handler: mux}

	go func() {
		if err := authServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			authResultChan <- OneDriveAuthResult{Error: err.Error()}
		}
	}()

	select {
	case result := <-authResultChan:
		authServer.Shutdown(context.Background())
		if result.Error != "" {
			return "", fmt.Errorf("授权失败: %s", result.Error)
		}
		return result.Code, nil
	case <-time.After(timeout):
		authServer.Shutdown(context.Background())
		return "", fmt.Errorf("授权超时")
	case <-ctx.Done():
		authServer.Shutdown(context.Background())
		return "", ctx.Err()
	}
}

// handleOAuthCallback 处理 OAuth 回调
func handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	errorMsg := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	if errorMsg != "" {
		authResultChan <- OneDriveAuthResult{Error: fmt.Sprintf("%s: %s", errorMsg, errorDesc)}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>%s: %s</p><p>您可以关闭此窗口。</p>
			</body></html>`, errorMsg, errorDesc)
		return
	}

	if code == "" {
		authResultChan <- OneDriveAuthResult{Error: "未收到授权码"}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>未收到授权码</p><p>您可以关闭此窗口。</p>
			</body></html>`)
		return
	}

	authResultChan <- OneDriveAuthResult{Code: code}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权成功</title></head><body>
		<h1>授权成功！</h1><p>您可以关闭此窗口并返回应用。</p>
		<script>window.close();</script>
		</body></html>`)
}
