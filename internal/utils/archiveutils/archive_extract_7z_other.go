//go:build !windows

package archiveutils

func extractArchiveWithEmbedded7z(source, target string) (bool, error) {
	return false, nil
}
