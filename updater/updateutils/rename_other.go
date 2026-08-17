//go:build !windows

package updateutils

import "os"

func renameReplace(source string, destination string) error {
	return os.Rename(source, destination)
}
