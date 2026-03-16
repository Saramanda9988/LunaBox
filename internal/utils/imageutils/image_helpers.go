package imageutils

import (
	"io"
	"lunabox/internal/utils/apputils"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var managedImageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}

func ensureManagedImageDir(name string) (string, error) {
	appDir, err := apputils.GetDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(appDir, name)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	return dir, nil
}

func removeFilesWithBaseName(dir, baseName string) {
	for _, ext := range managedImageExtensions {
		_ = os.Remove(filepath.Join(dir, baseName+ext))
	}
}

func removeFilesWithPrefixes(dir string, prefixes ...string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(file.Name(), prefix) {
				_ = os.Remove(filepath.Join(dir, file.Name()))
				break
			}
		}
	}
}

func detectImageExtension(contentType, rawURL string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		if ext := strings.ToLower(filepath.Ext(rawURL)); ext != "" {
			return ext
		}
		return ".jpg"
	}
}

func saveHTTPBody(resp *http.Response, destPath string) error {
	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, resp.Body); err != nil {
		_ = os.Remove(destPath)
		return err
	}
	return nil
}

func saveManagedBackgroundCopy(srcPath, filePrefix string, cleanupPrefixes ...string) (string, error) {
	bgDir, err := ensureManagedImageDir("backgrounds")
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == "" {
		ext = ".png"
	}

	fileName := filePrefix + "_" + strconv.FormatInt(time.Now().UnixMilli(), 10) + ext
	destPath := filepath.Join(bgDir, fileName)

	removeFilesWithPrefixes(bgDir, cleanupPrefixes...)
	if err := apputils.CopyFile(srcPath, destPath); err != nil {
		return "", err
	}

	return "/local/backgrounds/" + fileName, nil
}
