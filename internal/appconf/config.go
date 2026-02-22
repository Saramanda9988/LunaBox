package appconf

import (
	"encoding/json"
	"log"
	"lunabox/internal/enums"
	"lunabox/internal/utils"
	"os"
	"path/filepath"
)

// AppConfig 应用配置结构体
type AppConfig struct {
	BangumiAccessToken string `json:"access_token,omitempty"`
	VNDBAccessToken    string `json:"vndb_access_token,omitempty"`
	Theme              string `json:"theme"`         // light or dark
	Language           string `json:"language"`      // zh, en, etc.
	SidebarOpen        bool   `json:"sidebar_open"`  // 侧边栏是否展开
	CloseToTray        bool   `json:"close_to_tray"` // 关闭时最小化到托盘
	// AI 配置
	AIProvider     string `json:"ai_provider,omitempty"`      // openai, deepseek, etc.
	AIBaseURL      string `json:"ai_base_url,omitempty"`      // API base URL
	AIAPIKey       string `json:"ai_api_key,omitempty"`       // API key
	AIModel        string `json:"ai_model,omitempty"`         // model name
	AISystemPrompt string `json:"ai_system_prompt,omitempty"` // AI 系统提示语
	// 云备份配置
	CloudBackupEnabled   bool   `json:"cloud_backup_enabled"`             // 是否启用云备份
	CloudBackupProvider  string `json:"cloud_backup_provider,omitempty"`  // 云备份提供商: s3, onedrive
	BackupPassword       string `json:"backup_password,omitempty"`        // 备份密码（用于生成 user-id 和加密）
	BackupUserID         string `json:"backup_user_id,omitempty"`         // 云端用户标识（由备份密码 hash 生成）
	S3Endpoint           string `json:"s3_endpoint,omitempty"`            // S3 兼容端点
	S3Region             string `json:"s3_region,omitempty"`              // S3 区域
	S3Bucket             string `json:"s3_bucket,omitempty"`              // S3 存储桶
	S3AccessKey          string `json:"s3_access_key,omitempty"`          // S3 Access Key
	S3SecretKey          string `json:"s3_secret_key,omitempty"`          // S3 Secret Key
	CloudBackupRetention int    `json:"cloud_backup_retention,omitempty"` // 云端保留备份数量
	// OneDrive OAuth 配置
	OneDriveClientID     string `json:"onedrive_client_id,omitempty"`     // OneDrive Client ID
	OneDriveRefreshToken string `json:"onedrive_refresh_token,omitempty"` // OneDrive Refresh Token（OAuth 授权后获得）
	// 数据库备份
	LastDBBackupTime   string `json:"last_db_backup_time,omitempty"`   // 上次数据库备份时间
	PendingDBRestore   string `json:"pending_db_restore,omitempty"`    // 待恢复的数据库备份路径（重启后执行）
	LastFullBackupTime string `json:"last_full_backup_time,omitempty"` // 上次全量数据备份时间
	PendingFullRestore string `json:"pending_full_restore,omitempty"`  // 待恢复的全量数据备份路径（重启后执行）
	// 自动备份配置
	AutoBackupDB          bool `json:"auto_backup_db"`                 // 退出时自动备份数据库
	AutoBackupGameSave    bool `json:"auto_backup_game_save"`          // 游戏退出时自动备份存档
	AutoUploadToCloud     bool `json:"auto_upload_to_cloud,omitempty"` // 已弃用，保留用于配置迁移
	AutoUploadDBToCloud   bool `json:"auto_upload_db_to_cloud"`        // 自动上传数据库备份到云端
	AutoUploadSaveToCloud bool `json:"auto_upload_game_save_to_cloud"` // 自动上传游戏存档备份到云端
	// 备份保留策略
	LocalBackupRetention   int `json:"local_backup_retention"`    // 本地游戏备份保留数量
	LocalDBBackupRetention int `json:"local_db_backup_retention"` // 本地数据库备份保留数量
	// 窗口尺寸记忆
	WindowWidth  int `json:"window_width"`  // 窗口宽度
	WindowHeight int `json:"window_height"` // 窗口高度
	// 活跃时间追踪配置
	RecordActiveTimeOnly bool `json:"record_active_time_only"` // 仅记录活跃游玩时长（窗口在前台时）
	// 自动更新配置
	CheckUpdateOnStartup bool   `json:"check_update_on_startup"`     // 启动时自动检查更新
	UpdateCheckURL       string `json:"update_check_url,omitempty"`  // 自定义更新检查 URL
	LastUpdateCheck      string `json:"last_update_check,omitempty"` // 上次更新检查时间
	SkipVersion          string `json:"skip_version,omitempty"`      // 跳过的版本号（用户选择忽略的更新）
	// 背景图配置
	BackgroundImage         string  `json:"background_image,omitempty"` // 自定义背景图路径
	BackgroundBlur          int     `json:"background_blur"`            // 背景模糊度 (0-20)
	BackgroundOpacity       float64 `json:"background_opacity"`         // 背景不透明度 (0-1)
	BackgroundEnabled       bool    `json:"background_enabled"`         // 是否启用自定义背景
	BackgroundHideGameCover bool    `json:"background_hide_game_cover"` // 启用自定义背景时隐藏首页游戏封面
	BackgroundIsLight       bool    `json:"background_is_light"`        // 记录自定义背景是不是浅色调
	// Locale Emulator 和 Magpie 配置
	LocaleEmulatorPath string `json:"locale_emulator_path,omitempty"` // Locale Emulator 可执行文件路径
	MagpiePath         string `json:"magpie_path,omitempty"`          // Magpie 可执行文件路径
	// 进程检测配置
	AutoDetectGameProcess bool `json:"auto_detect_game_process"` // 是否启用自动游戏进程检测（分阶段检测策略）
	// 时区配置
	TimeZone string `json:"time_zone,omitempty"` // 数据库使用的 IANA 时区名称（如 "Asia/Shanghai"）
	// 路径配置
	DownloadDir    string `json:"download_dir,omitempty"`     // 默认下载目录（空则使用 ~/Downloads/LunaBox）
	GameLibraryDir string `json:"game_library_dir,omitempty"` // 游戏库目录（信息展示用）
}

// getConfigPath 获取配置文件路径
func getConfigPath() (string, error) {
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "appconf.json"), nil
}

func LoadConfig() (*AppConfig, error) {
	config := &AppConfig{
		BangumiAccessToken:     "",
		VNDBAccessToken:        "",
		Theme:                  "light",
		Language:               "zh-CN",
		SidebarOpen:            true,
		CloseToTray:            false,
		AIProvider:             "",
		AIBaseURL:              "",
		AIAPIKey:               "",
		AIModel:                "",
		AISystemPrompt:         string(enums.DefaultSystemPrompt),
		CloudBackupEnabled:     false,
		CloudBackupProvider:    "s3",
		BackupPassword:         "",
		BackupUserID:           "",
		S3Endpoint:             "",
		TimeZone:               "",
		S3Region:               "",
		S3Bucket:               "",
		S3AccessKey:            "",
		S3SecretKey:            "",
		CloudBackupRetention:   5,
		OneDriveClientID:       "26fcab6e-41ea-49ff-8ec9-063983cae3ef",
		OneDriveRefreshToken:   "",
		LastDBBackupTime:       "",
		PendingDBRestore:       "",
		LastFullBackupTime:     "",
		PendingFullRestore:     "",
		AutoBackupDB:           false,
		AutoBackupGameSave:     false,
		AutoUploadToCloud:      false,
		LocalBackupRetention:   10,
		LocalDBBackupRetention: 5,
		WindowWidth:            1230,
		WindowHeight:           800,
		RecordActiveTimeOnly:   false, // 默认关闭，向后兼容
		CheckUpdateOnStartup:   true,  // 默认开启启动时检查更新
		UpdateCheckURL:         "",
		LastUpdateCheck:        "",
		SkipVersion:            "",
		// 背景图配置默认值
		BackgroundImage:         "",
		BackgroundBlur:          10,   // 默认模糊度
		BackgroundOpacity:       0.85, // 默认不透明度
		BackgroundEnabled:       false,
		BackgroundHideGameCover: false, // 默认显示游戏封面
		BackgroundIsLight:       true,  // 默认是浅色调
		LocaleEmulatorPath:      "",
		MagpiePath:              "",
		AutoDetectGameProcess:   true, // 默认启用自动检测，保持向后兼容
		DownloadDir:             "",
		GameLibraryDir:          "",
	}

	// 获取配置文件路径
	configPath, err := getConfigPath()
	if err != nil {
		return config, err
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		err := SaveConfig(config)
		return config, err
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	// 解析配置
	if err := json.Unmarshal(data, config); err != nil {
		log.Printf("Failed to parse appconf file: %v", err)
		return config, err
	}

	// corner case: 如果有密码但没有 user_id（可能是旧版本的配置文件）
	if config.BackupPassword != "" && config.BackupUserID == "" {
		log.Printf("Fixing config: found password without user_id, clearing...")
		config.BackupPassword = ""
		if err := SaveConfig(config); err != nil {
			log.Printf("Failed to save fixed config: %v", err)
		} else {
			log.Printf("Config fixed: password clearing successfully")
		}
	}

	return config, err
}

func SaveConfig(config *AppConfig) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
