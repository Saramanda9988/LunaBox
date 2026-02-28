package ipcclient

import (
	"fmt"

	"lunabox/internal/cli/ipccore"
)

// IsServerRunning 检查 Server 是否在运行
func IsServerRunning() bool {
	return ipccore.IsServerRunning()
}

// RemoteInstall 将 InstallRequest 转发给运行中的 GUI 处理
func RemoteInstall(req interface{}) error {
	return ipccore.RemoteInstall(req)
}

// RemoteRun 在远程 Server 上执行命令
func RemoteRun(args []string) error {
	output, err := ipccore.RemoteRun(args)
	fmt.Print(output)
	return err
}
