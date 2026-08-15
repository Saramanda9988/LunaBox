package enums

type LaunchMode string

const (
	LaunchModeNormal        LaunchMode = "normal"
	LaunchModeSteam         LaunchMode = "steam"
	LaunchModeCompatibility LaunchMode = "compatibility"
)

var AllLaunchModes = []struct {
	Value  LaunchMode
	TSName string
}{
	{LaunchModeNormal, "NORMAL"},
	{LaunchModeSteam, "STEAM"},
	{LaunchModeCompatibility, "COMPATIBILITY"},
}

func NormalizeLaunchMode(mode LaunchMode) LaunchMode {
	switch mode {
	case LaunchModeSteam, LaunchModeCompatibility:
		return mode
	default:
		return LaunchModeNormal
	}
}
