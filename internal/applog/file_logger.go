package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailslogger "github.com/wailsapp/wails/v2/pkg/logger"
)

const (
	logFileTimestampLayout = "2006-01-02 15:04:05.000"
	logFileDateLayout      = "2006-01-02"
	defaultMaxLogSize      = 10 * 1024 * 1024
	defaultMaxLogBackups   = 5
)

var _ wailslogger.Logger = (*FileLogger)(nil)

// FileLogger 为 Wails 提供带时间戳和自动切分能力的日志实现。
type FileLogger struct {
	mu sync.Mutex

	dir      string
	baseName string
	ext      string

	maxSize    int64
	maxBackups int

	currentDate string
	currentFile *os.File
	currentSize int64

	now func() time.Time
	pid int
}

// NewFileLogger 创建带日期分片和大小轮转能力的文件日志。
func NewFileLogger(filename string) wailslogger.Logger {
	return newFileLogger(filename, defaultMaxLogSize, defaultMaxLogBackups)
}

func newFileLogger(filename string, maxSize int64, maxBackups int) *FileLogger {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filepath.Base(filename), ext)
	if base == "" {
		base = "app"
	}
	if ext == "" {
		ext = ".log"
	}

	return &FileLogger{
		dir:        filepath.Dir(filename),
		baseName:   base,
		ext:        ext,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		now:        time.Now,
		pid:        os.Getpid(),
	}
}

func (l *FileLogger) Print(message string) {
	l.write("PRINT", message)
}

func (l *FileLogger) Trace(message string) {
	l.write("TRACE", message)
}

func (l *FileLogger) Debug(message string) {
	l.write("DEBUG", message)
}

func (l *FileLogger) Info(message string) {
	l.write("INFO", message)
}

func (l *FileLogger) Warning(message string) {
	l.write("WARN", message)
}

func (l *FileLogger) Error(message string) {
	l.write("ERROR", message)
}

func (l *FileLogger) Fatal(message string) {
	l.write("FATAL", message)
	os.Exit(1)
}

func (l *FileLogger) write(level, message string) {
	now := l.now()
	entry := formatLogEntry(now, level, message, l.pid)
	entrySize := int64(len(entry))

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureWritable(now, entrySize); err != nil {
		reportFileLoggerError("prepare log file", err)
		return
	}

	if _, err := l.currentFile.WriteString(entry); err != nil {
		reportFileLoggerError("write log entry", err)
		return
	}

	l.currentSize += entrySize
}

func (l *FileLogger) ensureWritable(now time.Time, nextEntrySize int64) error {
	dateKey := now.Format(logFileDateLayout)

	if l.currentFile != nil && l.currentDate != dateKey {
		if err := l.closeCurrentLocked(); err != nil {
			return err
		}
	}

	if l.currentFile == nil {
		if err := l.openCurrentLocked(dateKey); err != nil {
			return err
		}
	}

	if l.maxSize > 0 && l.currentSize > 0 && l.currentSize+nextEntrySize > l.maxSize {
		if err := l.rotateBySizeLocked(dateKey); err != nil {
			return err
		}
	}

	return nil
}

func (l *FileLogger) openCurrentLocked(dateKey string) error {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	path := l.activeFilePath(dateKey)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("stat log file %s: %w", path, err)
	}

	l.currentFile = file
	l.currentDate = dateKey
	l.currentSize = info.Size()
	return nil
}

func (l *FileLogger) rotateBySizeLocked(dateKey string) error {
	if err := l.closeCurrentLocked(); err != nil {
		return err
	}

	activePath := l.activeFilePath(dateKey)
	if l.maxBackups > 0 {
		oldestPath := l.rotatedFilePath(dateKey, l.maxBackups)
		if err := os.Remove(oldestPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove rotated log %s: %w", oldestPath, err)
		}
		for idx := l.maxBackups - 1; idx >= 1; idx-- {
			src := l.rotatedFilePath(dateKey, idx)
			dst := l.rotatedFilePath(dateKey, idx+1)
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("rotate log %s -> %s: %w", src, dst, err)
			}
		}
		if err := os.Rename(activePath, l.rotatedFilePath(dateKey, 1)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate active log %s: %w", activePath, err)
		}
	} else {
		if err := os.Remove(activePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove active log %s: %w", activePath, err)
		}
	}

	return l.openCurrentLocked(dateKey)
}

func (l *FileLogger) closeCurrentLocked() error {
	if l.currentFile == nil {
		return nil
	}

	err := l.currentFile.Close()
	l.currentFile = nil
	l.currentDate = ""
	l.currentSize = 0
	if err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

func (l *FileLogger) activeFilePath(dateKey string) string {
	return filepath.Join(l.dir, fmt.Sprintf("%s-%s%s", l.baseName, dateKey, l.ext))
}

func (l *FileLogger) rotatedFilePath(dateKey string, index int) string {
	return filepath.Join(l.dir, fmt.Sprintf("%s-%s.%03d%s", l.baseName, dateKey, index, l.ext))
}

func formatLogEntry(now time.Time, level, message string, pid int) string {
	prefix := fmt.Sprintf("%s | %-5s | pid=%d | ", now.Format(logFileTimestampLayout), level, pid)
	lines := splitLogLines(message)
	continuationPrefix := strings.Repeat(" ", len(prefix))

	var builder strings.Builder
	for index, line := range lines {
		if index == 0 {
			builder.WriteString(prefix)
		} else {
			builder.WriteString(continuationPrefix)
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	return builder.String()
}

func splitLogLines(message string) []string {
	normalized := strings.ReplaceAll(message, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimRight(normalized, "\n")
	if normalized == "" {
		return []string{""}
	}
	return strings.Split(normalized, "\n")
}

func reportFileLoggerError(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s | ERROR | applog | failed to %s: %v\n", time.Now().Format(logFileTimestampLayout), action, err)
}
