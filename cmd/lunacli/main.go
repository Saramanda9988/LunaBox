package main

import (
	"fmt"
	"os"

	"lunabox/internal/cli"
	"lunabox/internal/ipc"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Error: command required")
		fmt.Println("Usage: lunacli <command>")
		os.Exit(1)
	}

	// 1. 尝试通过 IPC 在 GUI 进程中运行命令
	if ipc.IsServerRunning() {
		err := ipc.RemoteRun(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 2. 如果 GUI 未运行，则在本地运行命令
	// 初始化 CoreApp (包含 DB 连接)
	app, err := cli.NewCoreApp()
	if err != nil {
		fmt.Printf("Failed to initialize: %v\n", err)
		os.Exit(1)
	}
	// 确保在退出时释放资源（如 DB 锁）
	defer app.Close()

	// 运行命令
	if err := cli.RunCommand(os.Stdout, app, args); err != nil {
		// RunCommand 已经打印了具体的错误信息，这里只处理退出码
		if err.Error() == "unknown command" {
			// 对于未知命令，Usage 已经打印了
		} else {
			// 其他错误已经在 RunCommand 内部通过 applog 记录了
		}
		os.Exit(1)
	}
}
