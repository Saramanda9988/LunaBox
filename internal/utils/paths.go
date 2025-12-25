package utils

import (
	"os"
	"path/filepath"
	"sync"
)

// buildMode 由编译时 ldflags 注入
// "portable" = 便携版，数据放在程序目录
// "installer" = 安装版，数据放在用户目录
var buildMode = "portable" // 默认为便携版

const appName = "LunaBox"

var (
	dataDir   string
	cacheDir  string
	configDir string
	initOnce  sync.Once
	initErr   error
)

// initDirs 初始化所有目录路径
func initDirs() error {
	initOnce.Do(func() {
		if buildMode == "installer" {
			// 安装版：使用系统标准目录
			initErr = initInstallerDirs()
		} else {
			// 便携版：使用程序目录
			initErr = initPortableDirs()
		}
	})
	return initErr
}

// initPortableDirs 初始化便携版目录（程序目录）
func initPortableDirs() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execDir := filepath.Dir(execPath)
	dataDir = execDir
	cacheDir = execDir
	configDir = execDir
	return nil
}

// initInstallerDirs 初始化安装版目录（系统标准目录）
func initInstallerDirs() error {
	// 配置目录: %APPDATA%\LunaBox (Windows) 或 ~/.config/LunaBox (Linux/Mac)
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	configDir = filepath.Join(userConfigDir, appName)

	// 缓存目录: %LOCALAPPDATA%\LunaBox (Windows) 或 ~/.cache/LunaBox (Linux/Mac)
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cacheDir = filepath.Join(userCacheDir, appName)

	// 数据目录：使用配置目录（数据库、备份等重要数据）
	dataDir = configDir

	return nil
}

// GetDataDir 获取数据目录（数据库、备份、上传的封面图片等）
func GetDataDir() (string, error) {
	if err := initDirs(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", err
	}
	return dataDir, nil
}

// GetCacheDir 获取缓存目录
func GetCacheDir() (string, error) {
	if err := initDirs(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return cacheDir, nil
}

// GetConfigDir 获取配置目录
func GetConfigDir() (string, error) {
	if err := initDirs(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return configDir, nil
}

// GetSubDir 获取子目录并确保目录存在
func GetSubDir(subPath string) (string, error) {
	base, err := GetDataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, subPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// GetCacheSubDir 获取缓存子目录并确保目录存在
func GetCacheSubDir(subPath string) (string, error) {
	base, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, subPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// IsPortableMode 返回是否为便携模式
func IsPortableMode() bool {
	return buildMode == "portable"
}

// GetBuildMode 返回当前构建模式
func GetBuildMode() string {
	return buildMode
}
