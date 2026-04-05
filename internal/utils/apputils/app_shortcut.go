package apputils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InternetShortcut struct {
	URL       string
	IconFile  string
	IconIndex int
}

// WriteInternetShortcut writes a Windows .url shortcut file.
func WriteInternetShortcut(path string, shortcut InternetShortcut) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("shortcut path is empty")
	}
	if strings.TrimSpace(shortcut.URL) == "" {
		return fmt.Errorf("shortcut url is empty")
	}

	if !strings.EqualFold(filepath.Ext(trimmedPath), ".url") {
		trimmedPath += ".url"
	}

	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0755); err != nil {
		return fmt.Errorf("create shortcut directory: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("[InternetShortcut]\r\n")
	builder.WriteString("URL=")
	builder.WriteString(shortcut.URL)
	builder.WriteString("\r\n")

	iconFile := strings.TrimSpace(shortcut.IconFile)
	if iconFile != "" {
		builder.WriteString("IconFile=")
		builder.WriteString(iconFile)
		builder.WriteString("\r\n")
		builder.WriteString(fmt.Sprintf("IconIndex=%d\r\n", shortcut.IconIndex))
	}

	if err := os.WriteFile(trimmedPath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("write internet shortcut: %w", err)
	}

	return nil
}
