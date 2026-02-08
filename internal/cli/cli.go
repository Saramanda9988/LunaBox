package cli

import (
	"context"
	"database/sql"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/service"

	_ "github.com/duckdb/duckdb-go/v2"
)

// CoreApp CLI 模式的核心应用 (也可用于 GUI 传递 Context)
type CoreApp struct {
	Config         *appconf.AppConfig
	DB             *sql.DB
	Ctx            context.Context // Export Ctx
	GameService    *service.GameService
	StartService   *service.StartService
	SessionService *service.SessionService
	BackupService  *service.BackupService
	VersionService *service.VersionService
}

// RunCommand 执行 CLI 命令
// w:输出目标 (os.Stdout 或 http.ResponseWriter)
// app: 已初始化的 CoreApp
// args: 命令行参数 (不包含程序名)
func RunCommand(w io.Writer, app *CoreApp, args []string) error {
	rootCmd := NewRootCmd(app)
	rootCmd.SetOut(w)
	rootCmd.SetErr(w) // 将错误输出也重定向到 w，以便 IPC 可以捕获
	rootCmd.SetArgs(args)

	return rootCmd.Execute()
}
