//go:build !windows && !darwin && !linux

package archiveutils

func extractArchiveWithBundled7z(source, target string) (bool, error) {
	return false, nil
}
