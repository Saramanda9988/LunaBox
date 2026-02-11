package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ConfigService struct {
	ctx         context.Context
	db          *sql.DB
	config      *appconf.AppConfig
	quitHandler func() // 安全退出回调
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
	localPath, err := utils.SaveBackgroundImage(selection)
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
	localPath, err := utils.SaveTempBackgroundImage(selection)
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
	localPath, err := utils.CropAndSaveBackgroundImage(srcPath, x, y, width, height)
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

	err := appconf.SaveConfig(&newConfig)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to save config: %v", err)
		return err
	}

	// 更新应用配置 in-memory
	s.config.BangumiAccessToken = newConfig.BangumiAccessToken
	s.config.VNDBAccessToken = newConfig.VNDBAccessToken
	s.config.Theme = newConfig.Theme
	s.config.Language = newConfig.Language
	s.config.SidebarOpen = newConfig.SidebarOpen
	s.config.CloseToTray = newConfig.CloseToTray
	s.config.AIProvider = newConfig.AIProvider
	s.config.AIBaseURL = newConfig.AIBaseURL
	s.config.AIAPIKey = newConfig.AIAPIKey
	s.config.AIModel = newConfig.AIModel
	s.config.AISystemPrompt = newConfig.AISystemPrompt
	s.config.CloudBackupEnabled = newConfig.CloudBackupEnabled
	s.config.CloudBackupProvider = newConfig.CloudBackupProvider
	s.config.BackupPassword = newConfig.BackupPassword
	s.config.BackupUserID = newConfig.BackupUserID
	s.config.S3Endpoint = newConfig.S3Endpoint
	s.config.S3Region = newConfig.S3Region
	s.config.S3Bucket = newConfig.S3Bucket
	s.config.S3AccessKey = newConfig.S3AccessKey
	s.config.S3SecretKey = newConfig.S3SecretKey
	s.config.CloudBackupRetention = newConfig.CloudBackupRetention
	s.config.RecordActiveTimeOnly = newConfig.RecordActiveTimeOnly
	// OneDrive OAuth
	s.config.OneDriveClientID = newConfig.OneDriveClientID
	s.config.OneDriveRefreshToken = newConfig.OneDriveRefreshToken
	// 备份相关配置
	s.config.AutoBackupDB = newConfig.AutoBackupDB
	s.config.AutoBackupGameSave = newConfig.AutoBackupGameSave
	s.config.AutoUploadToCloud = newConfig.AutoUploadToCloud
	s.config.LocalBackupRetention = newConfig.LocalBackupRetention
	s.config.LocalDBBackupRetention = newConfig.LocalDBBackupRetention
	s.config.LastFullBackupTime = newConfig.LastFullBackupTime
	s.config.PendingFullRestore = newConfig.PendingFullRestore
	s.config.RecordActiveTimeOnly = newConfig.RecordActiveTimeOnly
	s.config.CheckUpdateOnStartup = newConfig.CheckUpdateOnStartup
	// 更新相关配置
	s.config.UpdateCheckURL = newConfig.UpdateCheckURL
	s.config.LastUpdateCheck = newConfig.LastUpdateCheck
	s.config.SkipVersion = newConfig.SkipVersion
	// 背景图配置
	s.config.BackgroundImage = newConfig.BackgroundImage
	s.config.BackgroundBlur = newConfig.BackgroundBlur
	s.config.BackgroundOpacity = newConfig.BackgroundOpacity
	s.config.BackgroundEnabled = newConfig.BackgroundEnabled
	s.config.BackgroundHideGameCover = newConfig.BackgroundHideGameCover
	s.config.BackgroundIsLight = newConfig.BackgroundIsLight
	// 游戏相关配置
	s.config.LocaleEmulatorPath = newConfig.LocaleEmulatorPath
	s.config.MagpiePath = newConfig.MagpiePath
	s.config.AutoDetectGameProcess = newConfig.AutoDetectGameProcess
	// 时区相关配置
	s.config.TimeZone = newConfig.TimeZone
	return nil
}

// SetQuitHandler 设置安全退出回调
func (s *ConfigService) SetQuitHandler(handler func()) {
	s.quitHandler = handler
}

// SafeQuit 安全退出应用（绕过托盘最小化逻辑）
func (s *ConfigService) SafeQuit() {
	if s.quitHandler != nil {
		s.quitHandler()
	}
}
