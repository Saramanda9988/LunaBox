package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// DefaultOneDriveClientID TODO: 编译的时候使用github变量动态注入
const DefaultOneDriveClientID = ""

// 本地回调服务器配置
const (
	localCallbackPort = 23456
	localCallbackPath = "/callback"
	redirectURI       = "http://localhost:23456/callback"
)

// OneDriveConfig OneDrive 配置
type OneDriveConfig struct {
	ClientID     string
	RefreshToken string
}

// OneDriveClient OneDrive 客户端封装
type OneDriveClient struct {
	config      OneDriveConfig
	accessToken string
	tokenExpiry time.Time
	httpClient  *http.Client
}

// OneDriveTokenResponse OAuth token 响应
type OneDriveTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// OneDriveItem OneDrive 文件/文件夹项
type OneDriveItem struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Size             int64     `json:"size"`
	LastModifiedTime time.Time `json:"lastModifiedDateTime"`
}

// OneDriveListResponse OneDrive 列表响应
type OneDriveListResponse struct {
	Value    []OneDriveItem `json:"value"`
	NextLink string         `json:"@odata.nextLink,omitempty"`
}

// OneDriveUploadSession 大文件上传会话
type OneDriveUploadSession struct {
	UploadURL string `json:"uploadUrl"`
}

const (
	oneDriveAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	oneDriveTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	oneDriveAPIBase  = "https://graph.microsoft.com/v1.0/me/drive"
	// 小文件上传限制 4MB
	smallFileLimit = 4 * 1024 * 1024
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

// NewOneDriveClient 创建 OneDrive 客户端
func NewOneDriveClient(cfg OneDriveConfig) (*OneDriveClient, error) {
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultOneDriveClientID
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("OneDrive Client ID 未配置")
	}
	if cfg.RefreshToken == "" {
		return nil, fmt.Errorf("OneDrive 未授权")
	}

	client := &OneDriveClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	// 获取 access token
	if err := client.refreshAccessToken(context.Background()); err != nil {
		return nil, fmt.Errorf("获取 access token 失败: %w", err)
	}

	return client, nil
}

// GetOneDriveAuthURL 获取 OAuth 授权 URL
func GetOneDriveAuthURL(clientID string) string {
	if clientID == "" {
		clientID = DefaultOneDriveClientID
	}
	params := url.Values{
		"client_id":     {clientID},
		"response_type": {"code"},
		"redirect_uri":  {redirectURI},
		"scope":         {"Files.ReadWrite.All offline_access"},
		"response_mode": {"query"},
	}
	return oneDriveAuthURL + "?" + params.Encode()
}

// StartOneDriveAuthServer 启动本地回调服务器，等待授权回调
// 返回授权码或错误
func StartOneDriveAuthServer(ctx context.Context, timeout time.Duration) (string, error) {
	authServerMu.Lock()
	defer authServerMu.Unlock()

	// 创建结果通道
	authResultChan = make(chan OneDriveAuthResult, 1)

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc(localCallbackPath, handleOAuthCallback)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", localCallbackPort))
	if err != nil {
		return "", fmt.Errorf("无法启动本地服务器: %w", err)
	}

	authServer = &http.Server{
		Handler: mux,
	}

	// 启动服务器
	go func() {
		if err := authServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			authResultChan <- OneDriveAuthResult{Error: err.Error()}
		}
	}()

	// 等待结果或超时
	select {
	case result := <-authResultChan:
		// 关闭服务器
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

// ExchangeOneDriveCodeForToken 用授权码换取 token
func ExchangeOneDriveCodeForToken(ctx context.Context, clientID, code string) (*OneDriveTokenResponse, error) {
	if clientID == "" {
		clientID = DefaultOneDriveClientID
	}
	if clientID == "" {
		return nil, fmt.Errorf("OneDrive Client ID 未配置")
	}
	if code == "" {
		return nil, fmt.Errorf("授权码不能为空")
	}

	data := url.Values{
		"client_id":    {clientID},
		"redirect_uri": {redirectURI},
		"code":         {code},
		"grant_type":   {"authorization_code"},
	}

	bodyStr := data.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", oneDriveTokenURL, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应体用于调试
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp OneDriveTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &tokenResp, nil
}

// refreshAccessToken 刷新 access token
func (c *OneDriveClient) refreshAccessToken(ctx context.Context) error {
	data := url.Values{
		"client_id":     {c.config.ClientID},
		"refresh_token": {c.config.RefreshToken},
		"grant_type":    {"refresh_token"},
		"redirect_uri":  {redirectURI},
	}

	bodyStr := data.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", oneDriveTokenURL, strings.NewReader(bodyStr))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取响应体用于调试
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp OneDriveTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}

	if tokenResp.Error != "" {
		return fmt.Errorf("OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return nil
}

// GetNewRefreshToken 获取新的 refresh token（用于更新存储）
func (c *OneDriveClient) GetNewRefreshToken(ctx context.Context) (string, error) {
	data := url.Values{
		"client_id":     {c.config.ClientID},
		"refresh_token": {c.config.RefreshToken},
		"grant_type":    {"refresh_token"},
		"redirect_uri":  {redirectURI},
	}

	bodyStr := data.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", oneDriveTokenURL, strings.NewReader(bodyStr))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 读取响应体用于调试
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp OneDriveTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return tokenResp.RefreshToken, nil
}

// ensureValidToken 确保 token 有效
func (c *OneDriveClient) ensureValidToken(ctx context.Context) error {
	if time.Now().After(c.tokenExpiry) {
		return c.refreshAccessToken(ctx)
	}
	return nil
}

// UploadFile 上传文件到 OneDrive
func (c *OneDriveClient) UploadFile(ctx context.Context, remotePath string, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 小文件直接上传
	if stat.Size() <= smallFileLimit {
		return c.uploadSmallFile(ctx, remotePath, file)
	}

	// 大文件使用分片上传
	return c.uploadLargeFile(ctx, remotePath, file, stat.Size())
}

// uploadSmallFile 上传小文件 (< 4MB)
func (c *OneDriveClient) uploadSmallFile(ctx context.Context, remotePath string, file *os.File) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/root:%s:/content", oneDriveAPIBase, remotePath)

	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, file)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上传失败: %s", string(body))
	}

	return nil
}

// uploadLargeFile 上传大文件 (> 4MB)
func (c *OneDriveClient) uploadLargeFile(ctx context.Context, remotePath string, file *os.File, fileSize int64) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	// 创建上传会话
	sessionURL := fmt.Sprintf("%s/root:%s:/createUploadSession", oneDriveAPIBase, remotePath)
	sessionReq, err := http.NewRequestWithContext(ctx, "POST", sessionURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	sessionReq.Header.Set("Authorization", "Bearer "+c.accessToken)
	sessionReq.Header.Set("Content-Type", "application/json")

	sessionResp, err := c.httpClient.Do(sessionReq)
	if err != nil {
		return err
	}
	defer sessionResp.Body.Close()

	var session OneDriveUploadSession
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		return err
	}

	// 分片上传 (10MB 每片)
	const chunkSize = 10 * 1024 * 1024
	buffer := make([]byte, chunkSize)
	var offset int64 = 0

	for offset < fileSize {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		end := offset + int64(n) - 1
		contentRange := fmt.Sprintf("bytes %d-%d/%d", offset, end, fileSize)

		req, err := http.NewRequestWithContext(ctx, "PUT", session.UploadURL, bytes.NewReader(buffer[:n]))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Range", contentRange)
		req.Header.Set("Content-Length", fmt.Sprintf("%d", n))

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("分片上传失败: %d", resp.StatusCode)
		}

		offset += int64(n)
	}

	return nil
}

// DownloadFile 从 OneDrive 下载文件
func (c *OneDriveClient) DownloadFile(ctx context.Context, remotePath string, localPath string) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/root:%s:/content", oneDriveAPIBase, remotePath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("下载失败: %s", string(body))
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// ListObjects 列出指定路径下的文件
func (c *OneDriveClient) ListObjects(ctx context.Context, folderPath string) ([]string, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(folderPath, "/") {
		folderPath = "/" + folderPath
	}
	folderPath = strings.TrimSuffix(folderPath, "/")

	apiURL := fmt.Sprintf("%s/root:%s:/children", oneDriveAPIBase, folderPath)

	var allItems []string
	for apiURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.accessToken)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		// 处理 404（文件夹不存在）
		if resp.StatusCode == 404 {
			resp.Body.Close()
			return []string{}, nil
		}

		var listResp OneDriveListResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, item := range listResp.Value {
			allItems = append(allItems, folderPath+"/"+item.Name)
		}

		apiURL = listResp.NextLink
	}

	return allItems, nil
}

// DeleteObject 删除文件
func (c *OneDriveClient) DeleteObject(ctx context.Context, remotePath string) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/root:%s", oneDriveAPIBase, remotePath)

	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除失败: %s", string(body))
	}

	return nil
}

// TestConnection 测试连接
func (c *OneDriveClient) TestConnection(ctx context.Context) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", oneDriveAPIBase, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("连接测试失败: %s", string(body))
	}

	return nil
}

// CreateFolder 创建文件夹（递归创建）
func (c *OneDriveClient) CreateFolder(ctx context.Context, folderPath string) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(folderPath, "/") {
		folderPath = "/" + folderPath
	}

	parts := strings.Split(strings.Trim(folderPath, "/"), "/")
	currentPath := ""

	for _, part := range parts {
		parentPath := currentPath
		if parentPath == "" {
			parentPath = "/"
		}
		currentPath = currentPath + "/" + part

		// 检查是否存在
		checkURL := fmt.Sprintf("%s/root:%s", oneDriveAPIBase, currentPath)
		checkReq, _ := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
		checkReq.Header.Set("Authorization", "Bearer "+c.accessToken)
		checkResp, err := c.httpClient.Do(checkReq)
		if err == nil && checkResp.StatusCode == 200 {
			checkResp.Body.Close()
			continue
		}
		if checkResp != nil {
			checkResp.Body.Close()
		}

		// 创建文件夹
		var createURL string
		if parentPath == "/" {
			createURL = fmt.Sprintf("%s/root/children", oneDriveAPIBase)
		} else {
			createURL = fmt.Sprintf("%s/root:%s:/children", oneDriveAPIBase, parentPath)
		}

		body := map[string]interface{}{
			"name":   part,
			"folder": map[string]interface{}{},
		}
		bodyBytes, _ := json.Marshal(body)

		req, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 && resp.StatusCode != 409 {
			return fmt.Errorf("创建文件夹失败: %d", resp.StatusCode)
		}
	}

	return nil
}
