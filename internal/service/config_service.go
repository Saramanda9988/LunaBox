package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/autostart"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/archiveutils"
	"lunabox/internal/utils/imageutils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ConfigService struct {
	ctx                       context.Context
	db                        *sql.DB
	config                    *appconf.AppConfig
	quitHandler               func() // 安全退出回调
	configUpdateHook          func(appconf.AppConfig) error
	suppressInitialWindowShow bool
}

func NewConfigService() *ConfigService {
	return &ConfigService{}
}

func (s *ConfigService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *ConfigService) GetAppConfig() (appconf.AppConfig, error) {
	return *s.config, nil
}

func (s *ConfigService) SetSuppressInitialWindowShow(suppress bool) {
	s.suppressInitialWindowShow = suppress
}

func (s *ConfigService) ShouldShowMainWindowOnReady() bool {
	if !s.suppressInitialWindowShow {
		return true
	}

	if s.config == nil {
		return true
	}

	return strings.TrimSpace(s.config.TimeZone) == ""
}

// SelectDirectory 打开目录选择对话框
func (s *ConfigService) SelectDirectory(title string) (string, error) {
	selection, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "SelectDirectory failed: %v", err)
		return "", err
	}
	return selection, nil
}

// OpenDataDirectory 在系统文件管理器中打开 LunaBox 数据目录。
func (s *ConfigService) OpenDataDirectory() (string, error) {
	dataDir, err := apputils.GetDataDir()
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get data directory: %v", err)
		return "", fmt.Errorf("获取数据目录失败: %w", err)
	}

	if err := apputils.OpenDirectory(dataDir); err != nil {
		applog.LogErrorf(s.ctx, "failed to open data directory %s: %v", dataDir, err)
		return "", fmt.Errorf("打开数据目录失败: %w", err)
	}

	return dataDir, nil
}

// ExportLogsZip 将 logs 目录导出为 ZIP 压缩包。
func (s *ConfigService) ExportLogsZip() (string, error) {
	logDir, err := apputils.GetSubDir("logs")
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get logs directory: %v", err)
		return "", fmt.Errorf("获取日志目录失败: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	defaultFileName := fmt.Sprintf("lunabox_logs_%s.zip", timestamp)
	selection, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:           "导出日志 ZIP",
		DefaultFilename: defaultFileName,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "ZIP 压缩包 (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open log export save dialog: %v", err)
		return "", fmt.Errorf("选择日志导出位置失败: %w", err)
	}
	if selection == "" {
		return "", nil
	}
	if !strings.HasSuffix(strings.ToLower(selection), ".zip") {
		selection += ".zip"
	}

	tempDir, err := os.MkdirTemp("", "lunabox_logs_export_*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempZip := filepath.Join(tempDir, filepath.Base(selection))
	if _, err := archiveutils.ZipDirectory(logDir, tempZip); err != nil {
		applog.LogErrorf(s.ctx, "failed to zip logs directory %s: %v", logDir, err)
		return "", fmt.Errorf("压缩日志失败: %w", err)
	}
	if err := apputils.CopyFile(tempZip, selection); err != nil {
		applog.LogErrorf(s.ctx, "failed to copy log archive to %s: %v", selection, err)
		return "", fmt.Errorf("保存日志压缩包失败: %w", err)
	}

	return selection, nil
}

// SelectBackgroundImage 打开文件选择对话框选择背景图片，并保存到应用目录
func (s *ConfigService) SelectBackgroundImage() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择背景图片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片文件", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp"},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open file dialog: %v", err)
		return "", err
	}
	if selection == "" {
		return "", nil // 用户取消选择
	}

	// 将图片保存到应用目录
	localPath, err := imageutils.SaveBackgroundImage(selection)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to save background image: %v", err)
		return "", err
	}

	return localPath, nil
}

// SelectAndCropBackgroundImage 打开文件选择对话框选择背景图片，复制到临时目录并返回 /local/ 路径供前端裁剪
func (s *ConfigService) SelectAndCropBackgroundImage() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择背景图片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片文件", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp"},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open file dialog: %v", err)
		return "", err
	}
	if selection == "" {
		return "", nil // 用户取消选择
	}

	// 将文件复制到临时目录，返回 /local/ 路径供前端使用
	localPath, err := imageutils.SaveTempBackgroundImage(selection)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to save temp background image: %v", err)
		return "", err
	}

	return localPath, nil
}

// SaveCroppedBackgroundImage 保存裁剪后的背景图片
// srcPath 应为 /local/backgrounds/temp_bg_xxx.png 格式的路径
func (s *ConfigService) SaveCroppedBackgroundImage(srcPath string, x, y, width, height int) (string, error) {
	if srcPath == "" {
		return "", fmt.Errorf("source path is empty")
	}

	// 裁剪并保存图片（会自动清理临时文件）
	localPath, err := imageutils.CropAndSaveBackgroundImage(srcPath, x, y, width, height)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to crop and save background image: %v", err)
		return "", err
	}

	return localPath, nil
}

func (s *ConfigService) UpdateAppConfig(newConfig appconf.AppConfig) error {
	if newConfig.Theme == "" || newConfig.Language == "" {
		applog.LogErrorf(s.ctx, "invalid config")
		return fmt.Errorf("invalid config")
	}

	appconf.SanitizeOneDriveOAuthConfig(&newConfig)
	newConfig.MCPPort = appconf.NormalizeMCPPort(newConfig.MCPPort)

	var previousConfig appconf.AppConfig
	if s.config != nil {
		previousConfig = *s.config
	}

	previousLaunchAtLogin := false
	if s.config != nil {
		previousLaunchAtLogin = s.config.LaunchAtLogin
	}

	shouldSyncLaunchAtLogin := newConfig.LaunchAtLogin != previousLaunchAtLogin || newConfig.LaunchAtLogin
	if shouldSyncLaunchAtLogin {
		if err := autostart.Sync(newConfig.LaunchAtLogin); err != nil {
			applog.LogErrorf(s.ctx, "failed to sync launch-at-login: %v", err)
			return fmt.Errorf("同步开机自启动失败: %w", err)
		}
	}

	err := appconf.SaveConfig(&newConfig)
	if err != nil {
		if shouldSyncLaunchAtLogin {
			if rollbackErr := autostart.Sync(previousLaunchAtLogin); rollbackErr != nil {
				applog.LogErrorf(s.ctx, "failed to rollback launch-at-login after save error: %v", rollbackErr)
			}
		}
		applog.LogErrorf(s.ctx, "failed to save config: %v", err)
		return err
	}

	if s.config != nil {
		*s.config = newConfig
	}

	if s.configUpdateHook != nil {
		if err := s.configUpdateHook(newConfig); err != nil {
			if saveErr := appconf.SaveConfig(&previousConfig); saveErr != nil {
				applog.LogErrorf(s.ctx, "failed to rollback config file after update hook error: %v", saveErr)
			}
			if s.config != nil {
				*s.config = previousConfig
			}
			if shouldSyncLaunchAtLogin {
				if rollbackErr := autostart.Sync(previousLaunchAtLogin); rollbackErr != nil {
					applog.LogErrorf(s.ctx, "failed to rollback launch-at-login after update hook error: %v", rollbackErr)
				}
			}
			if rollbackHookErr := s.configUpdateHook(previousConfig); rollbackHookErr != nil {
				applog.LogErrorf(s.ctx, "failed to rollback runtime config hook: %v", rollbackHookErr)
			}
			return err
		}
	}

	return nil
}

// SetQuitHandler 设置安全退出回调
func (s *ConfigService) SetQuitHandler(handler func()) {
	s.quitHandler = handler
}

func (s *ConfigService) SetConfigUpdateHook(hook func(appconf.AppConfig) error) {
	s.configUpdateHook = hook
}

// SafeQuit 安全退出应用（绕过托盘最小化逻辑）
func (s *ConfigService) SafeQuit() {
	if s.quitHandler != nil {
		s.quitHandler()
	}
}
