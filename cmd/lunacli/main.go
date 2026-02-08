package main

import (
	"fmt"
	"os"

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

	// 2. 如果 GUI 未运行，提示用户启动
	fmt.Println("Error: LunaBox application is not running.")
	fmt.Println("Please start LunaBox first to use CLI commands.")
	os.Exit(1)
}
