package service

import (
	"context"
	"lunabox/internal/utils/audioutils"
	"lunabox/internal/version"
	"runtime"
)

type VersionService struct {
	ctx context.Context
}

func NewVersionService() *VersionService {
	return &VersionService{}
}

//wails:ignore
func (s *VersionService) Init(ctx context.Context) {
	s.ctx = ctx
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

// GetGOOS 返回当前运行平台。
func (s *VersionService) GetGOOS() string {
	return runtime.GOOS
}

// SupportsBackgroundProcessMute reports whether the running operating system
// provides the per-process audio controls required by background game mute.
func (s *VersionService) SupportsBackgroundProcessMute() bool {
	return audioutils.IsProcessMuteSupported()
}

// GetVersionInfo 返回版本信息对象
func (s *VersionService) GetVersionInfo() map[string]string {
	return map[string]string{
		"version":   version.Version,
		"commit":    version.GitCommit,
		"buildTime": version.BuildTime,
		"buildMode": version.BuildMode,
		"goos":      runtime.GOOS,
	}
}
