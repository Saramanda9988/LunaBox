package utils

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

// ExtractArchive 通用解压函数（支持 zip/rar/7z/tar 及常见压缩归档组合）
func ExtractArchive(source, target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	fsys, err := archives.FileSystem(context.Background(), source, nil)
	if err != nil {
		return fmt.Errorf("open archive filesystem: %w", err)
	}

	cleanTarget := filepath.Clean(target)

	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}

		targetPath := filepath.Join(cleanTarget, filepath.FromSlash(path))
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
		return fmt.Errorf("extract archive entries: %w", err)
	}

	return nil
}
