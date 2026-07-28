package webdav

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"lunabox/internal/utils/proxyutils"
)

// Config WebDAV 配置
type Config struct {
	URL         string
	Username    string
	Password    string
	ProxyConfig proxyutils.ProxyConfigProvider
}

// Provider WebDAV 云存储提供商
type Provider struct {
	baseURL    *url.URL // 规范化后的服务地址（路径无尾部斜杠）
	username   string
	password   string
	httpClient *http.Client
}

// statusError 携带 HTTP 状态码的错误，用于判断是否需要补建父目录后重试
type statusError struct {
	StatusCode int
	Message    string
}

func (e *statusError) Error() string {
	return e.Message
}

// propfindBody 仅请求 resourcetype，用于区分文件与集合
const propfindBody = `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`

// multistatus PROPFIND 207 响应（按 local name 匹配，兼容不同命名空间前缀的服务端）
type multistatus struct {
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href      string        `xml:"href"`
	Propstats []davPropstat `xml:"propstat"`
}

type davPropstat struct {
	Prop struct {
		ResourceType struct {
			Collection *struct{} `xml:"collection"`
		} `xml:"resourcetype"`
	} `xml:"prop"`
}

func (r davResponse) isCollection() bool {
	for _, ps := range r.Propstats {
		if ps.Prop.ResourceType.Collection != nil {
			return true
		}
	}
	return false
}

// NewProvider 创建 WebDAV Provider
func NewProvider(cfg Config) (*Provider, error) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("WebDAV 服务地址未配置")
	}
	baseURL, err := url.Parse(rawURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("WebDAV 服务地址无效")
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("WebDAV 服务地址协议无效")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("WebDAV 服务地址格式无效")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	baseURL.RawPath = ""

	client, _, err := proxyutils.NewHTTPClientFromConfig(60*time.Second, cfg.ProxyConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 WebDAV HTTP 客户端失败: %w", err)
	}

	return &Provider{
		baseURL:    baseURL,
		username:   cfg.Username,
		password:   cfg.Password,
		httpClient: client,
	}, nil
}

// normalizeKey 统一 key 形态：正斜杠分隔、无首尾斜杠
func normalizeKey(key string) string {
	return strings.Trim(strings.ReplaceAll(key, "\\", "/"), "/")
}

// urlForKey 生成对象 URL，逐段做百分号编码
func (p *Provider) urlForKey(key string) string {
	var b strings.Builder
	b.WriteString(p.baseURL.String())
	key = normalizeKey(key)
	if key != "" {
		for _, seg := range strings.Split(key, "/") {
			b.WriteString("/")
			b.WriteString(url.PathEscape(seg))
		}
	}
	return b.String()
}

// urlForCollection 集合 URL 以斜杠结尾，避免服务端 301 重定向（自定义方法不会被自动跟随）
func (p *Provider) urlForCollection(key string) string {
	return p.urlForKey(key) + "/"
}

func (p *Provider) newRequest(ctx context.Context, method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if p.username != "" || p.password != "" {
		req.SetBasicAuth(p.username, p.password)
	}
	return req, nil
}

func responseError(action string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &statusError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("%s失败: 认证失败，请检查用户名和密码", action)}
	}
	return &statusError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("%s失败 (HTTP %d): %s", action, resp.StatusCode, msg)}
}

func (p *Provider) UploadFile(ctx context.Context, cloudPath, localPath string) error {
	err := p.putFile(ctx, cloudPath, localPath)
	if err == nil {
		return nil
	}
	// 409/404 通常表示父集合不存在，补建后重试一次
	var se *statusError
	if !errors.As(err, &se) || (se.StatusCode != http.StatusConflict && se.StatusCode != http.StatusNotFound) {
		return err
	}
	parent := path.Dir(normalizeKey(cloudPath))
	if parent == "." || parent == "/" {
		return err
	}
	if dirErr := p.EnsureDir(ctx, parent); dirErr != nil {
		return err
	}
	return p.putFile(ctx, cloudPath, localPath)
}

func (p *Provider) putFile(ctx context.Context, cloudPath, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	req, err := p.newRequest(ctx, http.MethodPut, p.urlForKey(cloudPath), file)
	if err != nil {
		return err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return responseError("上传", resp)
	}
	return nil
}

func (p *Provider) DownloadFile(ctx context.Context, cloudPath, localPath string) error {
	req, err := p.newRequest(ctx, http.MethodGet, p.urlForKey(cloudPath), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return responseError("下载", resp)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

func (p *Provider) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	dirKey := normalizeKey(prefix)

	req, err := p.newRequest(ctx, "PROPFIND", p.urlForCollection(dirKey), strings.NewReader(propfindBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出对象失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, responseError("列出对象", resp)
	}

	var ms multistatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("解析 PROPFIND 响应失败: %w", err)
	}

	requestPath := strings.TrimSuffix(p.baseURL.Path, "/")
	if dirKey != "" {
		requestPath = requestPath + "/" + dirKey
	}

	var keys []string
	for _, item := range ms.Responses {
		// 只返回文件，跳过集合（含请求的目录自身）
		if item.isCollection() {
			continue
		}
		itemPath, ok := p.hrefToPath(item.Href)
		if !ok {
			continue
		}
		if strings.TrimSuffix(itemPath, "/") == requestPath {
			continue
		}
		key := strings.TrimPrefix(itemPath, p.baseURL.Path)
		key = normalizeKey(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// hrefToPath 把 href（可能是绝对 URL 或编码后的路径）解析为解码后的路径
func (p *Provider) hrefToPath(href string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", false
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	return u.Path, true
}

func (p *Provider) DeleteObject(ctx context.Context, key string) error {
	req, err := p.newRequest(ctx, http.MethodDelete, p.urlForKey(key), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		return responseError("删除", resp)
	}
	return nil
}

func (p *Provider) TestConnection(ctx context.Context) error {
	req, err := p.newRequest(ctx, "PROPFIND", p.baseURL.String()+"/", strings.NewReader(propfindBody))
	if err != nil {
		return err
	}
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("连接测试失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return responseError("连接测试", resp)
	}
	return nil
}

func (p *Provider) EnsureDir(ctx context.Context, dirPath string) error {
	dirKey := normalizeKey(dirPath)
	if dirKey == "" {
		return nil
	}

	// 目录已存在时直接返回，避免逐级 MKCOL
	if exists, err := p.collectionExists(ctx, dirKey); err == nil && exists {
		return nil
	}

	current := ""
	for _, seg := range strings.Split(dirKey, "/") {
		if current == "" {
			current = seg
		} else {
			current = current + "/" + seg
		}
		if err := p.mkcol(ctx, current); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provider) collectionExists(ctx context.Context, dirKey string) (bool, error) {
	req, err := p.newRequest(ctx, "PROPFIND", p.urlForCollection(dirKey), strings.NewReader(propfindBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	return resp.StatusCode == http.StatusMultiStatus || resp.StatusCode == http.StatusOK, nil
}

func (p *Provider) mkcol(ctx context.Context, dirKey string) error {
	req, err := p.newRequest(ctx, "MKCOL", p.urlForCollection(dirKey), nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	defer resp.Body.Close()

	// 201 创建成功；405/301 表示目录已存在
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK, http.StatusMethodNotAllowed, http.StatusMovedPermanently:
		return nil
	default:
		return responseError("创建目录", resp)
	}
}

func (p *Provider) GetCloudPath(userID, subPath string) string {
	return fmt.Sprintf("LunaBox/v1/%s/%s", userID, subPath)
}
