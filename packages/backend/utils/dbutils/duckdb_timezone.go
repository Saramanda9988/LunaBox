package dbutils

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// SetDuckDBTimeZone 把整个数据库（连接池里已有与以后新建的连接）的时区设为 timeZone（IANA 名称，为空时 UTC）。
// 必须用 SET GLOBAL：不带作用域的 SET TimeZone 只改执行它的那一个连接，连接池里其余连接仍是系统时区。
func SetDuckDBTimeZone(ctx context.Context, db *sql.DB, timeZone string) error {
	timeZone = strings.TrimSpace(timeZone)
	if timeZone == "" {
		timeZone = "UTC"
	}
	quoted := "'" + strings.ReplaceAll(timeZone, "'", "''") + "'"
	if _, err := db.ExecContext(ctx, "SET GLOBAL TimeZone = "+quoted); err != nil {
		return fmt.Errorf("set DuckDB time zone %q: %w", timeZone, err)
	}
	return nil
}
