// mygo 用 cgo 运行 MyGo 的 CLI（go tool mygo）：MyGo 在 CGO_ENABLED 未设置时用 CGO_ENABLED=0 构建，
// 而 DuckDB 需要 cgo（docs/cli.md 的 Environment variables）。参数原样交给 go tool mygo：
//
//	go run ./tools/mygo dev
//	go run ./tools/mygo dev -lang en-US
//
// 在能写 `CGO_ENABLED=1 go tool mygo dev` 的 shell 里效果相同；它让 pnpm（Windows 上用 cmd 运行脚本）也能用。
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

func main() {
	if os.Getenv("CGO_ENABLED") == "" {
		os.Setenv("CGO_ENABLED", "1")
	}
	cmd := exec.Command("go", append([]string{"tool", "mygo"}, os.Args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Ctrl+C 交给 mygo：它先等应用退出再结束，这里等它。
	signal.Ignore(os.Interrupt)
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
