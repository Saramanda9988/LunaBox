package main

import (
	"fmt"
	"os"
	"slices"

	"lunabox/internal/cli"
	"lunabox/internal/cli/ipcclient"
)

// localOnlyCommands 必须在本地运行、不转发给 GUI 的命令。
// 这些命令仍然走 Cobra，只是执行位置留在当前进程。
var localOnlyCommands = map[string]bool{
	"luna-sama": true,
	"protocol":  true,
}

func main() {
	args := os.Args[1:]

	// Help / 本地命令不需要 GUI 进程，直接在当前进程执行。
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
	if len(args) == 0 {
		return true
	}

	if localOnlyCommands[args[0]] || args[0] == "help" {
		return true
	}

	return slices.Contains(args, "--help") ||
		slices.Contains(args, "-h") ||
		slices.Contains(args, "--register-protocol") ||
		slices.Contains(args, "--unregister-protocol")
}

func runLocalCommand(args []string) error {
	return cli.RunCommand(os.Stdout, &cli.CoreApp{}, args)
}
