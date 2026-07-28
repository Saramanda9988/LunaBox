package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/utils/proxyutils"
	"net/http"
	"strings"
	"time"

	"lunabox/internal/version"

	"golang.org/x/mod/semver"
	"lunabox/internal/wailsruntime"
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
	ctx     context.Context
	config  *ConfigService
	runtime wailsruntime.Runtime
}

// 默认更新检查 URL 列表（按优先级排序）
var defaultUpdateURLs = []string{
	"https://lunabox.pages.dev/version.json",   // 主地址
	"https://4update.netlify.app/version.json", // Netlify 备份（用户可修改）
}

func NewUpdateService() *UpdateService {
	return &UpdateService{runtime: wailsruntime.Unavailable()}
}

//wails:ignore
func (s *UpdateService) Init(ctx context.Context) {
	s.ctx = ctx
}

//wails:ignore
func (s *UpdateService) SetRuntime(runtime wailsruntime.Runtime) {
	if runtime != nil {
		s.runtime = runtime
	}
}

// SetConfigService 设置 ConfigService（用于读取和更新应用配置）。
//
//wails:ignore
func (s *UpdateService) SetConfigService(configService *ConfigService) {
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
		applog.LogError(s.ctx, "[UpdateService]获取应用配置失败: "+err.Error())
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
		updateInfo, lastErr = s.fetchUpdateInfo(url, &appConfig)
		if lastErr == nil {
			break
		}
		applog.LogWarningf(s.ctx, "Failed to fetch update info from %s: %v", url, lastErr)
	}

	if updateInfo == nil {
		applog.LogWarningf(s.ctx, "[UpdateService] failed to fetch update info from all sources: %v", lastErr)
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
func (s *UpdateService) fetchUpdateInfo(url string, appConfig *appconf.AppConfig) (*UpdateInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", version.UserAgent())

	client, _, err := proxyutils.NewHTTPClientFromConfig(10*time.Second, appConfig)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
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

// OpenDownloadURL 打开下载页面（已废弃，请在前端使用 @wailsio/runtime 的 Browser.OpenURL）。
func (s *UpdateService) OpenDownloadURL(url string) error {
	return s.runtime.OpenURL(url)
}

// compareVersions 比较两个版本号
// 返回 (true, nil) 表示 v1 < v2（即需要更新）
func compareVersions(v1, v2 string) (bool, error) {
	// 处理 dev 版本
	if strings.TrimPrefix(strings.TrimSpace(v1), "v") == "dev" {
		return false, nil // dev 版本不提示更新
	}
	if strings.TrimPrefix(strings.TrimSpace(v2), "v") == "dev" {
		return false, nil
	}

	normalizedV1, err := normalizeComparableVersion(v1)
	if err != nil {
		return false, err
	}
	normalizedV2, err := normalizeComparableVersion(v2)
	if err != nil {
		return false, err
	}

	return semver.Compare(normalizedV1, normalizedV2) < 0, nil
}

func normalizeComparableVersion(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	withoutPrefix := strings.TrimPrefix(trimmed, "v")

	// 兼容旧 autobuild 受滚动标签 dev-latest 影响生成的非法版本号。
	// 由于这类版本缺失正式基础版本，将其视为 0.0.0 的开发预发布版本。
	if strings.HasPrefix(withoutPrefix, "dev-latest-dev.") {
		withoutPrefix = "0.0.0-" + strings.TrimPrefix(withoutPrefix, "dev-latest-")
	}

	normalized := "v" + withoutPrefix
	if !semver.IsValid(normalized) {
		return "", fmt.Errorf("invalid version format: %s", trimmed)
	}

	return normalized, nil
}
