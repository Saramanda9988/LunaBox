//go:build darwin && !cgo

package audioutils

func IsProcessMuteSupported() bool {
	return false
}

func SetProcessMuted(processID uint32, muted bool) (bool, error) {
	return false, nil
}
