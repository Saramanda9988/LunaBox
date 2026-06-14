//go:build !windows && !darwin

package archiveutils

func extractArchiveWithBundled7z(source, target string) (bool, error) {
	return false, nil
}
