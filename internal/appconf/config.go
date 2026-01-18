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
	LastDBBackupTime string `json:"last_db_backup_time,omitempty"` // 上次数据库备份时间
	PendingDBRestore string `json:"pending_db_restore,omitempty"`  // 待恢复的数据库备份路径（重启后执行）
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
		Language:               "zh",
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
		S3Region:               "",
		S3Bucket:               "",
		S3AccessKey:            "",
		S3SecretKey:            "",
		CloudBackupRetention:   5,
		OneDriveClientID:       "26fcab6e-41ea-49ff-8ec9-063983cae3ef",
		OneDriveRefreshToken:   "",
		LastDBBackupTime:       "",
		PendingDBRestore:       "",
		AutoBackupDB:           false,
		AutoBackupGameSave:     false,
		AutoUploadToCloud:      false,
		LocalBackupRetention:   10,
		LocalDBBackupRetention: 5,
		WindowWidth:            1230,
		WindowHeight:           800,
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
