//go:build !windows

package apputils

import "fmt"

func ExportShortcutIconCache(sourcePath string, cacheKey string) (string, error) {
	return "", fmt.Errorf("shortcut icon export is only supported on Windows")
}
