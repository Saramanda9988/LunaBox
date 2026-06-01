//go:build !windows

package processutils

import "os/exec"

func ConfigureDetachedCommand(cmd *exec.Cmd) {}
