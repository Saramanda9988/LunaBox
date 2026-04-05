//go:build windows

package apputils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ExportShortcutIconCache exports a local .ico cache file for use by .url shortcuts.
func ExportShortcutIconCache(sourcePath string, cacheKey string) (string, error) {
	trimmedSource := strings.TrimSpace(sourcePath)
	if trimmedSource == "" {
		return "", fmt.Errorf("icon source path is empty")
	}

	absSource, err := filepath.Abs(filepath.Clean(trimmedSource))
	if err != nil {
		return "", fmt.Errorf("normalize icon source path: %w", err)
	}

	info, err := os.Stat(absSource)
	if err != nil {
		return "", fmt.Errorf("stat icon source path: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("icon source path is a directory")
	}

	shortcutsDir, err := GetSubDir("shortcut-icons")
	if err != nil {
		return "", fmt.Errorf("get shortcut icon cache dir: %w", err)
	}

	safeKey := strings.TrimSpace(cacheKey)
	if safeKey == "" {
		safeKey = filepath.Base(absSource)
	}
	safeKey = sanitizeShortcutIconCacheKey(safeKey)
	if safeKey == "" {
		safeKey = "shortcut-icon"
	}

	destPath := filepath.Join(shortcutsDir, safeKey+".ico")
	switch strings.ToLower(filepath.Ext(absSource)) {
	case ".ico":
		if err := CopyFile(absSource, destPath); err != nil {
			return "", fmt.Errorf("copy icon file: %w", err)
		}
	case ".exe", ".dll":
		if err := extractAssociatedIconToICO(absSource, destPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported shortcut icon source: %s", filepath.Ext(absSource))
	}

	return destPath, nil
}

func sanitizeShortcutIconCacheKey(value string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\x00", "_",
	)
	result := strings.TrimSpace(replacer.Replace(value))
	result = strings.TrimSuffix(result, filepath.Ext(result))
	if len(result) > 120 {
		result = result[:120]
	}
	return strings.TrimSpace(result)
}

func extractAssociatedIconToICO(sourcePath string, destPath string) error {
	script := `
Add-Type -AssemblyName System.Drawing
$source = $env:LUNABOX_SHORTCUT_ICON_SOURCE
$dest = $env:LUNABOX_SHORTCUT_ICON_DEST
if ([string]::IsNullOrWhiteSpace($source)) { throw "icon source path is empty" }
if ([string]::IsNullOrWhiteSpace($dest)) { throw "icon destination path is empty" }
$icon = [System.Drawing.Icon]::ExtractAssociatedIcon($source)
if ($null -eq $icon) { throw "no associated icon found" }
$dir = Split-Path -Parent $dest
if (-not [string]::IsNullOrWhiteSpace($dir)) {
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
}
$stream = [System.IO.File]::Open($dest, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
try {
  $icon.Save($stream)
} finally {
  $stream.Dispose()
  $icon.Dispose()
}
`

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(),
		"LUNABOX_SHORTCUT_ICON_SOURCE="+sourcePath,
		"LUNABOX_SHORTCUT_ICON_DEST="+destPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			return fmt.Errorf("extract associated icon with PowerShell: %w (%s)", err, trimmedOutput)
		}
		return fmt.Errorf("extract associated icon with PowerShell: %w", err)
	}

	return nil
}
