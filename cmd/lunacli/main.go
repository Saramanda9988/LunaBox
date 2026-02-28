package main

import (
	"fmt"
	"os"

	"lunabox/internal/cli/ipcclient"
)

// localOnlyCommands 必须在本地运行、不转发给 GUI 的命令
var localOnlyCommands = map[string]bool{
	"luna-sama":             true,
	"protocol":              true,
	"--register-protocol":   true,
	"--unregister-protocol": true,
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("Error: command required")
		fmt.Println("Usage: lunacli <command>")
		os.Exit(1)
	}

	// 本地命令：不需要 GUI 进程，直接在当前进程执行
	if localOnlyCommands[args[0]] {
		if err := runLocalCommand(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 其余命令：必须有 GUI 进程（通过 IPC 执行）
	if ipcclient.IsServerRunning() {
		if err := ipcclient.RemoteRun(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("Error: LunaBox application is not running.")
	fmt.Println("Please start LunaBox first to use CLI commands.")
	os.Exit(1)
}
