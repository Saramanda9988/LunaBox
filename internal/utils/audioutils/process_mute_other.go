//go:build !windows && !darwin

package audioutils

func IsProcessMuteSupported() bool {
	return false
}

// SetProcessMuted is a platform-compatible placeholder. Background game mute
// is currently exposed only on supported desktop platforms.
func SetProcessMuted(processID uint32, muted bool) (bool, error) {
	return false, nil
}
