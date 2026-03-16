package archiveutils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipDirectory 压缩目录
func ZipDirectory(source, target string) (int64, error) {
	return createZipArchive(target, func(archive *zip.Writer) error {
		return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			if relPath == "." {
				return nil
			}

			return addPathToZip(archive, path, relPath, info)
		})
	})
}

// ZipFileOrDirectory 压缩单个文件或整个目录
func ZipFileOrDirectory(source, target string) (int64, error) {
	info, err := os.Stat(source)
	if err != nil {
		return 0, fmt.Errorf("源路径不存在: %w", err)
	}

	if info.IsDir() {
		return ZipDirectory(source, target)
	}

	return createZipArchive(target, func(archive *zip.Writer) error {
		return addPathToZip(archive, source, filepath.Base(source), info)
	})
}

// UnzipFile 解压文件
func UnzipFile(source, target string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	return extractZipFiles(reader.File, target)
}

// ExtractZip 解压 ZIP 文件到指定目录
func ExtractZip(zipReader *zip.ReadCloser, destDir string) error {
	if zipReader == nil {
		return fmt.Errorf("zip reader is nil")
	}
	return extractZipFiles(zipReader.File, destDir)
}

// UnzipForRestore 解压文件用于恢复（与 UnzipFile 相同，保留兼容性）
func UnzipForRestore(src, dest string) error {
	return UnzipFile(src, dest)
}

func createZipArchive(target string, writeEntries func(*zip.Writer) error) (int64, error) {
	zipFile, err := os.Create(target)
	if err != nil {
		return 0, err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	if err := writeEntries(archive); err != nil {
		_ = archive.Close()
		return 0, err
	}
	if err := archive.Close(); err != nil {
		return 0, err
	}

	stat, err := os.Stat(target)
	if err != nil {
		return 0, err
	}
	return stat.Size(), nil
}

func addPathToZip(archive *zip.Writer, sourcePath, archivePath string, info os.FileInfo) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = filepath.ToSlash(archivePath)
	if info.IsDir() {
		header.Name += "/"
	} else {
		header.Method = zip.Deflate
	}

	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(writer, file)
	return err
}

func extractZipFiles(files []*zip.File, destDir string) error {
	cleanTarget, err := filepath.Abs(filepath.Clean(destDir))
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := extractZipFile(file, cleanTarget); err != nil {
			return err
		}
	}

	return nil
}

func extractZipFile(file *zip.File, cleanTarget string) error {
	cleanPath, err := safeJoinWithinBase(cleanTarget, file.Name)
	if err != nil {
		return err
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(cleanPath, file.Mode())
	}

	if err := os.MkdirAll(filepath.Dir(cleanPath), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func safeJoinWithinBase(baseDir, relativePath string) (string, error) {
	fullPath := filepath.Join(baseDir, relativePath)
	cleanPath := filepath.Clean(fullPath)
	if cleanPath != baseDir && !strings.HasPrefix(cleanPath, baseDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("非法的文件路径: %s", relativePath)
	}
	return cleanPath, nil
}
