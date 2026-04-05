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

// RemoteLaunch 将 ProtocolLaunchRequest 转发给运行中的 GUI 处理
func RemoteLaunch(req interface{}) error {
	return ipccore.RemoteLaunch(req)
}

// RemoteRun 在远程 Server 上执行命令
func RemoteRun(args []string) error {
	output, err := ipccore.RemoteRun(args)
	fmt.Print(output)
	return err
}
