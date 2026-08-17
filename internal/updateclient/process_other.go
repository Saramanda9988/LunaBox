//go:build !windows

package updateclient

import "os/exec"

func configureUpdateHelperCommand(command *exec.Cmd) {}
