package appconf

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// AppConfig 应用配置结构体
type AppConfig struct {
	BangumiAccessToken string `json:"access_token,omitempty"`
	VNDBAccessToken    string `json:"vndb_access_token,omitempty"`
	Theme              string `json:"theme"`    // light or dark
	Language           string `json:"language"` // zh, en, etc.
}

func LoadConfig() (*AppConfig, error) {
	config := &AppConfig{
		BangumiAccessToken: "",
		VNDBAccessToken:    "",
		Theme:              "light",
		Language:           "zh",
	}

	// 获取配置文件路径
	configPath := filepath.Join(".", "appconf.json")

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

	return config, err
}

func SaveConfig(config *AppConfig) error {
	configPath := filepath.Join(".", "appconf.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
