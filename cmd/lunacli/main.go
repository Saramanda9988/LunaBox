package main

import (
	"fmt"
	"os"

	"lunabox/internal/cli/ipcclient"
)

// localOnlyCommands 必须在本地运行、不转发给 GUI 的命令。
// 其余命令统一要求 GUI 进程在线，以保持语义一致。
var localOnlyCommands = map[string]bool{
	"luna-sama": true,
}

func main() {
	args := os.Args[1:]

	// 仅保留真正必须在当前进程执行的本地命令。
	if shouldRunLocally(args) {
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

func shouldRunLocally(args []string) bool {
	return len(args) > 0 && localOnlyCommands[args[0]]
}
