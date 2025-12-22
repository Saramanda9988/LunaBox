package onedrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// OneDrive 常量
const (
	oneDriveAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	oneDriveTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	oneDriveAPIBase  = "https://graph.microsoft.com/v1.0/me/drive"
	smallFileLimit   = 4 * 1024 * 1024 // 4MB
	redirectURI      = "http://localhost:23456/callback"
)

// OneDriveConfig OneDrive 配置
type OneDriveConfig struct {
	ClientID     string
	RefreshToken string
}

// OneDriveProvider OneDrive 云存储提供商
type OneDriveProvider struct {
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
	ID   string `json:"id"`
	Name string `json:"name"`
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

// NewOneDriveProvider 创建 OneDrive Provider
func NewOneDriveProvider(cfg OneDriveConfig) (*OneDriveProvider, error) {
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("OneDrive Client ID 未配置")
	}
	if cfg.RefreshToken == "" {
		return nil, fmt.Errorf("OneDrive 未授权")
	}

	provider := &OneDriveProvider{
		config:     cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	if err := provider.refreshAccessToken(context.Background()); err != nil {
		return nil, fmt.Errorf("获取 access token 失败: %w", err)
	}

	return provider, nil
}

// GetOneDriveAuthURL 获取 OAuth 授权 URL
func GetOneDriveAuthURL(clientID string) string {
	params := url.Values{
		"client_id":     {clientID},
		"response_type": {"code"},
		"redirect_uri":  {redirectURI},
		"scope":         {"Files.ReadWrite.AppFolder offline_access"},
		"response_mode": {"query"},
	}
	return oneDriveAuthURL + "?" + params.Encode()
}

// ExchangeOneDriveCodeForToken 用授权码换取 token
func ExchangeOneDriveCodeForToken(ctx context.Context, clientID, code string) (*OneDriveTokenResponse, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", oneDriveTokenURL, strings.NewReader(data.Encode()))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp OneDriveTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	return &tokenResp, nil
}

// refreshAccessToken 刷新 access token
func (p *OneDriveProvider) refreshAccessToken(ctx context.Context) error {
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"refresh_token": {p.config.RefreshToken},
		"grant_type":    {"refresh_token"},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", oneDriveTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp OneDriveTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if tokenResp.Error != "" {
		return fmt.Errorf("OAuth 错误 %s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	p.accessToken = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)
	return nil
}

// ensureValidToken 确保 token 有效
func (p *OneDriveProvider) ensureValidToken(ctx context.Context) error {
	if time.Now().After(p.tokenExpiry) {
		return p.refreshAccessToken(ctx)
	}
	return nil
}

func (p *OneDriveProvider) UploadFile(ctx context.Context, cloudPath, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	if stat.Size() <= smallFileLimit {
		return p.uploadSmallFile(ctx, cloudPath, file)
	}
	return p.uploadLargeFile(ctx, cloudPath, file, stat.Size())
}

// uploadSmallFile 上传小文件 (< 4MB)
func (p *OneDriveProvider) uploadSmallFile(ctx context.Context, remotePath string, file *os.File) error {
	if err := p.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/special/approot:%s:/content", oneDriveAPIBase, remotePath)
	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, file)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := p.httpClient.Do(req)
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
func (p *OneDriveProvider) uploadLargeFile(ctx context.Context, remotePath string, file *os.File, fileSize int64) error {
	if err := p.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	// 创建上传会话
	sessionURL := fmt.Sprintf("%s/special/approot:%s:/createUploadSession", oneDriveAPIBase, remotePath)
	sessionReq, err := http.NewRequestWithContext(ctx, "POST", sessionURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	sessionReq.Header.Set("Authorization", "Bearer "+p.accessToken)
	sessionReq.Header.Set("Content-Type", "application/json")

	sessionResp, err := p.httpClient.Do(sessionReq)
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

		resp, err := p.httpClient.Do(req)
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

func (p *OneDriveProvider) DownloadFile(ctx context.Context, cloudPath, localPath string) error {
	if err := p.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(cloudPath, "/") {
		cloudPath = "/" + cloudPath
	}

	apiURL := fmt.Sprintf("%s/special/approot:%s:/content", oneDriveAPIBase, cloudPath)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
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

func (p *OneDriveProvider) ListObjects(ctx context.Context, folderPath string) ([]string, error) {
	if err := p.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(folderPath, "/") {
		folderPath = "/" + folderPath
	}
	folderPath = strings.TrimSuffix(folderPath, "/")

	apiURL := fmt.Sprintf("%s/special/approot:%s:/children", oneDriveAPIBase, folderPath)

	var allItems []string
	for apiURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.accessToken)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

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

func (p *OneDriveProvider) DeleteObject(ctx context.Context, remotePath string) error {
	if err := p.ensureValidToken(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/special/approot:%s", oneDriveAPIBase, remotePath)
	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
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

func (p *OneDriveProvider) TestConnection(ctx context.Context) error {
	if err := p.ensureValidToken(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", oneDriveAPIBase, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.httpClient.Do(req)
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

func (p *OneDriveProvider) EnsureDir(ctx context.Context, folderPath string) error {
	if err := p.ensureValidToken(ctx); err != nil {
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
		checkURL := fmt.Sprintf("%s/special/approot:%s", oneDriveAPIBase, currentPath)
		checkReq, _ := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
		checkReq.Header.Set("Authorization", "Bearer "+p.accessToken)
		checkResp, err := p.httpClient.Do(checkReq)
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
			createURL = fmt.Sprintf("%s/special/approot/children", oneDriveAPIBase)
		} else {
			createURL = fmt.Sprintf("%s/special/approot:%s:/children", oneDriveAPIBase, parentPath)
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
		req.Header.Set("Authorization", "Bearer "+p.accessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
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

func (p *OneDriveProvider) GetCloudPath(userID, subPath string) string {
	return fmt.Sprintf("/LunaBox/v1/%s/%s", userID, subPath)
}
