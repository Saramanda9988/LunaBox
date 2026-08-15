package version

import "strings"

const (
	userAgentPrefix = "Saramanda9988/LunaBox/"
	userAgentSuffix = " (desktop) (https://github.com/Saramanda9988/LunaBox)"
)

// 版本信息，通过 ldflags 在编译时注入
var (
	Version                     = "1.1.1"                // 版本号，如 1.0.0
	GitCommit                   = "unknown"              // Git commit hash
	BuildTime                   = "unknown"              // 构建时间
	BuildMode                   = "portable"             // 构建模式：portable 或 installer
	BangumiOAuthClientID        = ""                     // Bangumi OAuth Client ID
	BangumiOAuthClientSecret    = ""                     // Bangumi OAuth Client Secret
	HikarinagiOAuthClientID     = "hkn_r3H8xRovRYSSbwP0" // Hikarinagi public/native OAuth Client ID
	HikarinagiOAuthClientSecret = ""                     // Hikarinagi OAuth Client Secret
	UmbraOAuthClientID          = ""                     // Umbra public/native OAuth Client ID
	UmbraRegistrationToken      = ""                     // Umbra device installation/registration token
	TouchGalAPIToken            = ""                     // TouchGAL API Bearer token
)

// GetVersion 返回版本信息
func GetVersion() string {
	return Version
}

// GetFullVersion 返回完整版本信息
func GetFullVersion() string {
	return Version + " (" + GitCommit + ")"
}

// GetBuildMode 返回构建模式
func GetBuildMode() string {
	return BuildMode
}

// UserAgent 返回包含当前构建版本的应用 User-Agent。
func UserAgent() string {
	appVersion := strings.TrimSpace(Version)
	if appVersion == "" {
		appVersion = "unknown"
	}
	return userAgentPrefix + appVersion + userAgentSuffix
}
