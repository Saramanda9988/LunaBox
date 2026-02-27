package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/mholt/archives"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ExtractArchive 通用解压函数（支持 zip/rar/7z/tar 及常见压缩归档组合）
// 返回值 extracted 表示是否完整解压成功。
// extracted=false 且 err!=nil 表示可回退的解压失败（例如目标平台非法文件名），可由上层进入手动解压模式。
func ExtractArchive(source, target string) (extracted bool, err error) {
	if err := os.MkdirAll(target, 0755); err != nil {
		return false, fmt.Errorf("create target dir: %w", err)
	}

	fsys, err := archives.FileSystem(context.Background(), source, nil)
	if err != nil {
		return false, fmt.Errorf("open archive filesystem: %w", err)
	}

	cleanTarget := filepath.Clean(target)
	recoverableStopErr := errors.New("recoverable extract stop")
	var recoverableErr error

	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}

		safeRelativePath := sanitizeArchiveRelativePath(path)
		if safeRelativePath == "" {
			return nil
		}

		targetPath := filepath.Join(cleanTarget, safeRelativePath)
		cleanPath := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanPath, cleanTarget+string(os.PathSeparator)) {
			return fmt.Errorf("illegal extracted path: %s", path)
		}

		if d.IsDir() {
			return os.MkdirAll(cleanPath, 0755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
			return err
		}

		srcFile, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		perm := info.Mode().Perm()
		if perm == 0 {
			perm = 0644
		}

		dstFile, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			if isRecoverableExtractPathError(err) {
				recoverableErr = fmt.Errorf("open %s: %w", cleanPath, err)
				return recoverableStopErr
			}
			return err
		}

		_, copyErr := io.Copy(dstFile, srcFile)
		closeErr := dstFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, recoverableStopErr) {
			if recoverableErr == nil {
				recoverableErr = err
			}
			return false, fmt.Errorf("extract archive entries: %w", recoverableErr)
		}
		return false, fmt.Errorf("extract archive entries: %w", err)
	}

	return true, nil
}

func isRecoverableExtractPathError(err error) bool {
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}

	if errors.Is(pathErr.Err, syscall.EINVAL) {
		return true
	}

	if runtime.GOOS == "windows" {
		if errors.Is(pathErr.Err, syscall.Errno(123)) {
			return true
		}
	}

	return false
}

func sanitizeArchiveRelativePath(raw string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}

	parts := strings.Split(normalized, "/")
	safeParts := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || trimmed == "." {
			continue
		}
		if trimmed == ".." {
			return ""
		}

		safePart := sanitizeArchivePathSegment(trimmed)
		if safePart == "" || safePart == "." || safePart == ".." {
			continue
		}
		safeParts = append(safeParts, safePart)
	}

	if len(safeParts) == 0 {
		return ""
	}

	return filepath.Join(safeParts...)
}

func sanitizeArchivePathSegment(segment string) string {
	segment = decodeArchiveSegmentWithGBKFallback(segment)
	segment = strings.ToValidUTF8(segment, "_")
	segment = strings.ReplaceAll(segment, string(utf8.RuneError), "_")

	cleaned := strings.Map(func(r rune) rune {
		if r < 32 {
			return '_'
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*', '\x00':
			return '_'
		default:
			return r
		}
	}, segment)

	if runtime.GOOS == "windows" {
		cleaned = strings.Trim(cleaned, " .")
		if cleaned == "" {
			cleaned = "_"
		}
		switch strings.ToUpper(cleaned) {
		case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
			cleaned = "_" + cleaned
		}
	}

	return cleaned
}

func decodeArchiveSegmentWithGBKFallback(segment string) string {
	if segment == "" {
		return ""
	}

	if utf8.ValidString(segment) && !strings.ContainsRune(segment, utf8.RuneError) {
		return segment
	}

	decoded, _, err := transform.String(simplifiedchinese.GBK.NewDecoder(), segment)
	if err != nil {
		return segment
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" {
		return segment
	}

	return decoded
}
