package apputils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyDir 递归复制目录
func CopyDir(src, dst string) error {
	return CopyDirWithProgress(src, dst, nil)
}

// CopyDirWithProgress 同 CopyDir，每写入一段数据就以写入的字节数调用 onWrite（可为 nil）。
func CopyDirWithProgress(src, dst string, onWrite func(n int64)) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDirWithProgress(srcPath, dstPath, onWrite); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath, onWrite); err != nil {
			return err
		}
	}

	return nil
}

// CopyFile 复制单个文件
func CopyFile(src, dst string) error {
	return copyFile(src, dst, nil)
}

func copyFile(src, dst string, onWrite func(n int64)) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	var w io.Writer = dstFile
	if onWrite != nil {
		w = countingWriter{w: dstFile, onWrite: onWrite}
	}
	_, err = io.Copy(w, srcFile)
	return err
}

// countingWriter 把写入转给 w，并以每次写入的字节数调用 onWrite。
type countingWriter struct {
	w       io.Writer
	onWrite func(n int64)
}

func (c countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.onWrite(int64(n))
	return n, err
}
