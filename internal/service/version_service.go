package service

import (
	"lunabox/internal/version"
)

type VersionService struct {
}

func NewVersionService() *VersionService {
	return &VersionService{}
}

// GetVersion 返回版本号
func (s *VersionService) GetVersion() string {
	return version.Version
}

// GetFullVersion 返回完整版本信息
func (s *VersionService) GetFullVersion() string {
	return version.GetFullVersion()
}

// GetBuildMode 返回构建模式
func (s *VersionService) GetBuildMode() string {
	return version.BuildMode
}

// GetBuildTime 返回构建时间
func (s *VersionService) GetBuildTime() string {
	return version.BuildTime
}

// GetVersionInfo 返回版本信息对象
func (s *VersionService) GetVersionInfo() map[string]string {
	return map[string]string{
		"version":   version.Version,
		"commit":    version.GitCommit,
		"buildTime": version.BuildTime,
		"buildMode": version.BuildMode,
	}
}
