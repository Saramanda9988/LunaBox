package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lunabox/internal/version"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// UpdateInfo 版本信息结构
type UpdateInfo struct {
	Version     string            `json:"version"`      // 版本号，如 1.2.0
	ReleaseDate string            `json:"release_date"` // 发布日期，如 2024-01-15
	Changelog   []string          `json:"changelog"`    // 更新日志内容数组
	Downloads   map[string]string `json:"downloads"`    // 下载链接字典：github, gitee 等
}

// UpdateCheckResult 更新检查结果
type UpdateCheckResult struct {
	HasUpdate   bool              `json:"has_update"`   // 是否有更新
	CurrentVer  string            `json:"current_ver"`  // 当前版本
	LatestVer   string            `json:"latest_ver"`   // 最新版本
	ReleaseDate string            `json:"release_date"` // 发布日期
	Changelog   []string          `json:"changelog"`    // 更新日志内容
	Downloads   map[string]string `json:"downloads"`    // 下载链接
}

// UpdateService 更新服务
type UpdateService struct {
	ctx    context.Context
	config *ConfigService
	client *http.Client
}

// 默认更新检查 URL 列表（按优先级排序）
var defaultUpdateURLs = []string{
	"https://lunabox.pages.dev/version.json",   // 主地址
	"https://4update.netlify.app/version.json", // Netlify 备份（用户可修改）
}

func NewUpdateService() *UpdateService {
	return &UpdateService{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *UpdateService) Init(ctx context.Context, configService *ConfigService) {
	s.ctx = ctx
	s.config = configService
}

// CheckForUpdates 手动检查更新（忽略跳过版本设置，总是检查最新版本）
func (s *UpdateService) CheckForUpdates() (*UpdateCheckResult, error) {
	return s.checkUpdates(false)
}

// CheckForUpdatesOnStartup 启动时自动检查更新
func (s *UpdateService) CheckForUpdatesOnStartup() (*UpdateCheckResult, error) {
	return s.checkUpdates(true)
}

// checkUpdates 检查更新的核心逻辑
// isAutoCheck: true 表示启动时自动检查，会检查频率限制和跳过版本
// isAutoCheck: false 表示手动检查，忽略跳过版本（因为在调用前已清空）
func (s *UpdateService) checkUpdates(isAutoCheck bool) (*UpdateCheckResult, error) {
	// 获取应用配置（手动检查时，SkipVersion 已在 CheckForUpdates 中被清空）
	appConfig, err := s.config.GetAppConfig()
	if err != nil {
		runtime.LogError(s.ctx, "[UpdateService]获取应用配置失败: "+err.Error())
		return nil, fmt.Errorf("failed to get app config: %w", err)
	}

	// 如果是启动时自动检查且未启用，直接返回
	if isAutoCheck && !appConfig.CheckUpdateOnStartup {
		return nil, nil
	}

	// 限制启动时检查的频率（最多每天一次）
	if isAutoCheck {
		if appConfig.LastUpdateCheck != "" {
			lastCheck, err := time.Parse(time.RFC3339, appConfig.LastUpdateCheck)
			if err == nil && time.Since(lastCheck) < 24*time.Hour {
				// 24小时内已检查过，跳过
				return nil, nil
			}
		}
	}

	// 获取更新检查 URL
	urls := s.getUpdateURLs(appConfig.UpdateCheckURL)

	// 尝试从各个 URL 获取版本信息
	var updateInfo *UpdateInfo
	var lastErr error
	for _, url := range urls {
		updateInfo, lastErr = s.fetchUpdateInfo(url)
		if lastErr == nil {
			break
		}
		runtime.LogWarningf(s.ctx, "Failed to fetch update info from %s: %v", url, lastErr)
	}

	if updateInfo == nil {
		runtime.LogWarningf(s.ctx, "[UpdateService] failed to fetch update info from all sources: %v", lastErr)
		return nil, fmt.Errorf("[UpdateService] failed to fetch update info from all sources: %w", lastErr)
	}

	// 更新最后检查时间
	s.updateLastCheckTime()

	// 比较版本
	currentVer := version.Version
	hasUpdate, err := compareVersions(currentVer, updateInfo.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to compare versions: %w", err)
	}

	// 只有自动检查时才检查跳过版本（手动检查时 SkipVersion 已被清空）
	if isAutoCheck && hasUpdate {
		skipVersionNormalized := strings.TrimSpace(strings.TrimPrefix(appConfig.SkipVersion, "v"))
		latestVersionNormalized := strings.TrimSpace(strings.TrimPrefix(updateInfo.Version, "v"))
		if skipVersionNormalized != "" && skipVersionNormalized == latestVersionNormalized {
			hasUpdate = false
		}
	}

	result := &UpdateCheckResult{
		HasUpdate:   hasUpdate,
		CurrentVer:  currentVer,
		LatestVer:   updateInfo.Version,
		ReleaseDate: updateInfo.ReleaseDate,
		Changelog:   updateInfo.Changelog,
		Downloads:   updateInfo.Downloads,
	}

	return result, nil
}

// getUpdateURLs 获取更新检查 URL 列表
func (s *UpdateService) getUpdateURLs(customURL string) []string {
	if customURL != "" {
		return []string{customURL}
	}
	return defaultUpdateURLs
}

// fetchUpdateInfo 从指定 URL 获取版本信息
func (s *UpdateService) fetchUpdateInfo(url string) (*UpdateInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "LunaBox-Updater/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info UpdateInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	// 验证必填字段
	if info.Version == "" {
		return nil, fmt.Errorf("missing version field")
	}

	return &info, nil
}

// updateLastCheckTime 更新最后检查时间
func (s *UpdateService) updateLastCheckTime() {
	appConfig, err := s.config.GetAppConfig()
	if err != nil {
		return
	}
	appConfig.LastUpdateCheck = time.Now().Format(time.RFC3339)
	s.config.UpdateAppConfig(appConfig)
}

// SkipVersion 跳过指定版本的更新
func (s *UpdateService) SkipVersion(ver string) error {
	appConfig, err := s.config.GetAppConfig()
	if err != nil {
		return err
	}
	// 统一移除 v 前缀，确保存储格式一致
	appConfig.SkipVersion = strings.TrimSpace(strings.TrimPrefix(ver, "v"))
	return s.config.UpdateAppConfig(appConfig)
}

// OpenDownloadURL 打开下载页面（已废弃，请在前端使用 runtime.BrowserOpenURL）
func (s *UpdateService) OpenDownloadURL(url string) error {
	runtime.BrowserOpenURL(s.ctx, url)
	return nil
}

// compareVersions 比较两个版本号
// 返回 (true, nil) 表示 v1 < v2（即需要更新）
func compareVersions(v1, v2 string) (bool, error) {
	// 移除可能的 'v' 前缀
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// 处理 dev 版本
	if v1 == "dev" {
		return false, nil // dev 版本不提示更新
	}
	if v2 == "dev" {
		return false, nil
	}

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int

		if i < len(parts1) {
			_, err := fmt.Sscanf(parts1[i], "%d", &n1)
			if err != nil {
				return false, fmt.Errorf("invalid version format: %s", v1)
			}
		}
		if i < len(parts2) {
			_, err := fmt.Sscanf(parts2[i], "%d", &n2)
			if err != nil {
				return false, fmt.Errorf("invalid version format: %s", v2)
			}
		}

		if n1 < n2 {
			return true, nil // v1 < v2，需要更新
		}
		if n1 > n2 {
			return false, nil // v1 > v2，不需要更新
		}
	}

	return false, nil // 版本相同
}
