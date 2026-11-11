package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"lunabox/backend/appconf"
)

func databaseTimeZone(t *testing.T, db *sql.DB) string {
	t.Helper()
	var tz string
	if err := db.QueryRow("SELECT current_setting('TimeZone')").Scan(&tz); err != nil {
		t.Fatalf("query time zone: %v", err)
	}
	return tz
}

// TestUpdateAppConfigAppliesTimeZone：修改时区立即作用于数据库；保存失败时恢复原来的时区。
func TestUpdateAppConfigAppliesTimeZone(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	config := &appconf.AppConfig{Theme: "light", Language: "zh-CN"}
	s := NewConfigService()
	s.Init(context.Background(), db, config)
	saveErr := error(nil)
	s.SetConfigSaverForTest(func(*appconf.AppConfig) error { return saveErr })

	next := *config
	next.TimeZone = "Asia/Tokyo"
	if err := s.UpdateAppConfig(next); err != nil {
		t.Fatalf("update config: %v", err)
	}
	if got := databaseTimeZone(t, db); got != "Asia/Tokyo" {
		t.Fatalf("database time zone = %q, want Asia/Tokyo", got)
	}

	saveErr = errors.New("disk full")
	next.TimeZone = "Europe/London"
	if err := s.UpdateAppConfig(next); err == nil {
		t.Fatal("update config succeeded although saving failed")
	}
	if got := databaseTimeZone(t, db); got != "Asia/Tokyo" {
		t.Fatalf("database time zone after failed save = %q, want Asia/Tokyo", got)
	}

	saveErr = nil
	next.TimeZone = "Not/AZone"
	if err := s.UpdateAppConfig(next); err == nil {
		t.Fatal("invalid time zone accepted")
	}
	if config.TimeZone != "Asia/Tokyo" {
		t.Fatalf("config time zone = %q after invalid update, want Asia/Tokyo", config.TimeZone)
	}
}
