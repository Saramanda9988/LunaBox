package onedrive

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"html"
	"net"
	"net/http"
	"time"
)

const (
	localCallbackPort = 23456
	localCallbackPath = "/callback"
)

// OneDriveAuthResult OAuth 授权结果
type OneDriveAuthResult struct {
	Code  string
	Error string
}

type oneDriveAuthSession struct {
	resultChan  chan OneDriveAuthResult
	server      *http.Server
	listener    net.Listener
	state       string
	redirectURI string
}

func newOneDriveAuthSession() (*oneDriveAuthSession, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localCallbackPort))
	if err != nil {
		return nil, fmt.Errorf("无法启动本地服务器: %w", err)
	}

	state, err := generateOAuthState()
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("生成授权状态失败: %w", err)
	}

	session := &oneDriveAuthSession{
		resultChan:  make(chan OneDriveAuthResult, 1),
		listener:    listener,
		state:       state,
		redirectURI: legacyRedirectURI,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(localCallbackPath, session.handleOAuthCallback)

	session.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := session.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			session.trySendResult(OneDriveAuthResult{Error: err.Error()})
		}
	}()

	return session, nil
}

func generateOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *oneDriveAuthSession) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	_ = s.listener.Close()
}

func (s *oneDriveAuthSession) trySendResult(result OneDriveAuthResult) {
	select {
	case s.resultChan <- result:
	default:
	}
}

// StartOneDriveAuthFlow 启动本地回调服务器并完成 OneDrive OAuth 授权
func StartOneDriveAuthFlow(ctx context.Context, clientID string, timeout time.Duration, openURL func(string) error) (string, string, error) {
	if clientID == "" {
		return "", "", fmt.Errorf("OneDrive Client ID 未配置")
	}

	session, err := newOneDriveAuthSession()
	if err != nil {
		return "", "", err
	}
	defer session.shutdown()

	if err := openURL(buildOneDriveAuthURL(clientID, session.redirectURI, session.state)); err != nil {
		return "", "", fmt.Errorf("打开授权页面失败: %w", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-session.resultChan:
		if result.Error != "" {
			return "", "", fmt.Errorf("授权失败: %s", result.Error)
		}
		return result.Code, session.redirectURI, nil
	case <-timer.C:
		return "", "", fmt.Errorf("授权超时")
	case <-ctx.Done():
		return "", "", ctx.Err()
	}
}

func (s *oneDriveAuthSession) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>请求方法无效</p><p>您可以关闭此窗口。</p>
			</body></html>`)
		return
	}

	if !isLoopbackRequest(r.RemoteAddr) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>回调来源无效</p><p>请返回应用后重试。</p>
			</body></html>`)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorMsg := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	if subtle.ConstantTimeCompare([]byte(state), []byte(s.state)) != 1 {
		s.trySendResult(OneDriveAuthResult{Error: "授权状态校验失败"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>授权状态校验失败</p><p>请返回应用后重试。</p>
			</body></html>`)
		return
	}

	if errorMsg != "" {
		errorMsg = html.EscapeString(errorMsg)
		errorDesc = html.EscapeString(errorDesc)
		s.trySendResult(OneDriveAuthResult{Error: fmt.Sprintf("%s: %s", errorMsg, errorDesc)})
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>%s: %s</p><p>您可以关闭此窗口。</p>
			</body></html>`, errorMsg, errorDesc)
		return
	}

	if code == "" {
		s.trySendResult(OneDriveAuthResult{Error: "未收到授权码"})
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权失败</title></head><body>
			<h1>授权失败</h1><p>未收到授权码</p><p>您可以关闭此窗口。</p>
			</body></html>`)
		return
	}

	s.trySendResult(OneDriveAuthResult{Code: code})
	fmt.Fprint(w, `<!DOCTYPE html><html><head><title>授权成功</title></head><body>
		<h1>授权成功！</h1><p>您可以关闭此窗口并返回应用。</p>
		<script>window.close();</script>
		</body></html>`)
}

func isLoopbackRequest(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
