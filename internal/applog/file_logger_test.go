package applog

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileLoggerFiltersBelowMinimumLevel(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "app.log")
	logger := newFileLogger(logPath, defaultMaxLogSize, defaultMaxLogBackups, slog.LevelInfo)
	fixedTime := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.Local)
	logger.now = func() time.Time { return fixedTime }

	slogLogger := logger.Slog()
	if slogLogger.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug level should be disabled")
	}
	if !slogLogger.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info level should be enabled")
	}

	slogLogger.Debug("wails debug")
	slogLogger.Info("wails info")
	logger.Debug("application debug")
	logger.Info("application info")

	logger.mu.Lock()
	err := logger.closeCurrentLocked()
	logger.mu.Unlock()
	if err != nil {
		t.Fatalf("close log file: %v", err)
	}

	contents, err := os.ReadFile(logger.activeFilePath(fixedTime.Format(logFileDateLayout)))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	logContents := string(contents)
	if strings.Contains(logContents, "debug") {
		t.Fatalf("log contains a filtered debug entry: %s", logContents)
	}
	if !strings.Contains(logContents, "wails info") {
		t.Fatalf("log is missing the Wails info entry: %s", logContents)
	}
	if !strings.Contains(logContents, "application info") {
		t.Fatalf("log is missing the application info entry: %s", logContents)
	}
}
