package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// pluginDB 为单个插件提供惰性打开的独立 SQLite 数据库（<DataDir>/plugins-data/<id>/plugin.db）。
// 未调用 ctx.db.* 的插件不会创建数据库文件，也没有额外内存开销。
type pluginDB struct {
	p      *Plugin
	path   string
	mu     sync.Mutex
	db     *sql.DB
	closed bool
}

func newPluginDB(p *Plugin) *pluginDB {
	return &pluginDB{
		p:    p,
		path: filepath.Join(p.DataDir, "plugin.db"),
	}
}

func (d *pluginDB) open() (*sql.DB, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil, fmt.Errorf("插件数据库已关闭")
	}
	if d.db != nil {
		return d.db, nil
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return nil, fmt.Errorf("创建插件数据库目录失败: %w", err)
	}
	dsn := d.path + "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开插件 SQLite 数据库失败: %w", err)
	}
	// 单连接串行写避免 SQLITE_BUSY，同时查询均在后台 goroutine 异步执行，不阻塞 UI 或插件线程。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化插件 SQLite 数据库失败: %w", err)
	}
	d.db = db
	d.p.scope.Effect(func() {
		d.Close()
	})
	return db, nil
}

func (d *pluginDB) Close() {
	d.mu.Lock()
	d.closed = true
	db := d.db
	d.db = nil
	d.mu.Unlock()
	if db != nil {
		_ = db.Close()
	}
}

func decodeSQLParams(raw json.RawMessage) ([]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var params []any
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("SQL 参数必须是数组: %w", err)
	}
	return params, nil
}

func scanSQLRows(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = v
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Plugin) buildDBAPI() map[string]any {
	return map[string]any{
		"exec": p.asyncNative("db.exec", func(ctx context.Context, args []json.RawMessage) (any, error) {
			var query string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &query)
			}
			query = strings.TrimSpace(query)
			if query == "" {
				return nil, fmt.Errorf("db.exec 需要非空 SQL 语句")
			}
			var params []any
			if len(args) > 1 {
				var err error
				if params, err = decodeSQLParams(args[1]); err != nil {
					return nil, err
				}
			}
			db, err := p.db.open()
			if err != nil {
				return nil, err
			}
			res, err := db.ExecContext(ctx, query, params...)
			if err != nil {
				return nil, err
			}
			affected, _ := res.RowsAffected()
			lastID, _ := res.LastInsertId()
			return map[string]any{
				"rowsAffected": affected,
				"lastInsertId": lastID,
			}, nil
		}),
		"query": p.asyncNative("db.query", func(ctx context.Context, args []json.RawMessage) (any, error) {
			var query string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &query)
			}
			query = strings.TrimSpace(query)
			if query == "" {
				return nil, fmt.Errorf("db.query 需要非空 SQL 语句")
			}
			var params []any
			if len(args) > 1 {
				var err error
				if params, err = decodeSQLParams(args[1]); err != nil {
					return nil, err
				}
			}
			db, err := p.db.open()
			if err != nil {
				return nil, err
			}
			rows, err := db.QueryContext(ctx, query, params...)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			return scanSQLRows(rows)
		}),
		"queryOne": p.asyncNative("db.queryOne", func(ctx context.Context, args []json.RawMessage) (any, error) {
			var query string
			if len(args) > 0 {
				_ = json.Unmarshal(args[0], &query)
			}
			query = strings.TrimSpace(query)
			if query == "" {
				return nil, fmt.Errorf("db.queryOne 需要非空 SQL 语句")
			}
			var params []any
			if len(args) > 1 {
				var err error
				if params, err = decodeSQLParams(args[1]); err != nil {
					return nil, err
				}
			}
			db, err := p.db.open()
			if err != nil {
				return nil, err
			}
			rows, err := db.QueryContext(ctx, query, params...)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			items, err := scanSQLRows(rows)
			if err != nil {
				return nil, err
			}
			if len(items) == 0 {
				return json.RawMessage("null"), nil
			}
			return items[0], nil
		}),
		"transaction": p.asyncNative("db.transaction", func(ctx context.Context, args []json.RawMessage) (any, error) {
			type stmtInput struct {
				SQL    string `json:"sql"`
				Params []any  `json:"params,omitempty"`
			}
			var stmts []stmtInput
			if len(args) > 0 && len(args[0]) > 0 {
				if err := json.Unmarshal(args[0], &stmts); err != nil {
					return nil, fmt.Errorf("db.transaction 需要语句数组: %w", err)
				}
			}
			db, err := p.db.open()
			if err != nil {
				return nil, err
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return nil, err
			}
			results := make([]map[string]any, 0, len(stmts))
			for _, st := range stmts {
				q := strings.TrimSpace(st.SQL)
				if q == "" {
					continue
				}
				res, err := tx.ExecContext(ctx, q, st.Params...)
				if err != nil {
					_ = tx.Rollback()
					return nil, err
				}
				affected, _ := res.RowsAffected()
				lastID, _ := res.LastInsertId()
				results = append(results, map[string]any{
					"rowsAffected": affected,
					"lastInsertId": lastID,
				})
			}
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return results, nil
		}),
	}
}
