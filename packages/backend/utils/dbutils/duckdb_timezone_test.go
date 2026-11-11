package dbutils

import (
	"context"
	"database/sql"
	"testing"
)

// TestSetDuckDBTimeZoneAppliesToEveryConnection：设置后连接池里同时持有的每个连接都是新时区。
func TestSetDuckDBTimeZoneAppliesToEveryConnection(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	check := func(want string) {
		t.Helper()
		var conns []*sql.Conn
		defer func() {
			for _, c := range conns {
				_ = c.Close()
			}
		}()
		for range 4 {
			c, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			conns = append(conns, c)
		}
		for i, c := range conns {
			var tz string
			if err := c.QueryRowContext(ctx, "SELECT current_setting('TimeZone')").Scan(&tz); err != nil {
				t.Fatal(err)
			}
			if tz != want {
				t.Fatalf("connection %d time zone = %q, want %q", i, tz, want)
			}
		}
	}

	if err := SetDuckDBTimeZone(ctx, db, ""); err != nil {
		t.Fatal(err)
	}
	check("UTC")
	if err := SetDuckDBTimeZone(ctx, db, "Asia/Tokyo"); err != nil {
		t.Fatal(err)
	}
	check("Asia/Tokyo")
	if err := SetDuckDBTimeZone(ctx, db, "Not/AZone'"); err == nil {
		t.Fatal("invalid time zone accepted")
	}
	check("Asia/Tokyo")
}
