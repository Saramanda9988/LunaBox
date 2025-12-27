package version

// 版本信息，通过 ldflags 在编译时注入
var (
	Version   = "dev"      // 版本号，如 1.0.0
	GitCommit = "unknown"  // Git commit hash
	BuildTime = "unknown"  // 构建时间
	BuildMode = "portable" // 构建模式：portable 或 installer
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
