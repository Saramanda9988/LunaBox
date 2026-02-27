//go:build !windows

package utils

func extractArchiveWithEmbedded7z(source, target string) (bool, error) {
	return false, nil
}
