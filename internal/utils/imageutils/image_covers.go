package imageutils

import (
	"fmt"
	"lunabox/internal/utils/apputils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SaveCoverImage 保存封面图片到应用的封面目录
func SaveCoverImage(srcPath string, gameID string) (string, error) {
	coverDir, err := ensureManagedImageDir("covers")
	if err != nil {
		return "", err
	}

	removeFilesWithBaseName(coverDir, gameID)

	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == "" {
		ext = ".png"
	}

	destFileName := gameID + ext
	destPath := filepath.Join(coverDir, destFileName)
	if err := apputils.CopyFile(srcPath, destPath); err != nil {
		return "", err
	}

	return "/local/covers/" + destFileName, nil
}

// ResolveCoverPath 解析封面图片路径
func ResolveCoverPath(imagePath string, tempDir string) string {
	cleanPath := strings.TrimPrefix(imagePath, ".\\")
	cleanPath = strings.TrimPrefix(cleanPath, "./")
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	fullPath := filepath.Join(tempDir, cleanPath)
	for _, ext := range []string{"", ".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		testPath := fullPath + ext
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
	}

	return ""
}

// DownloadAndSaveCoverImage 下载远程图片并保存到本地
func DownloadAndSaveCoverImage(imageURL string, gameID string) (string, error) {
	if strings.HasPrefix(imageURL, "/local/") || strings.HasPrefix(imageURL, "http://wails.localhost") {
		return imageURL, nil
	}
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return imageURL, nil
	}

	coverDir, err := ensureManagedImageDir("covers")
	if err != nil {
		return imageURL, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(imageURL)
	if err != nil {
		return imageURL, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return imageURL, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	removeFilesWithBaseName(coverDir, gameID)

	ext := detectImageExtension(resp.Header.Get("Content-Type"), imageURL)
	destFileName := gameID + ext
	destPath := filepath.Join(coverDir, destFileName)
	if err := saveHTTPBody(resp, destPath); err != nil {
		return imageURL, err
	}

	return "/local/covers/" + destFileName, nil
}

// RenameTempCover 将临时封面图片重命名为正式的游戏ID
func RenameTempCover(tempCoverURL string, gameID string) (string, error) {
	if !strings.Contains(tempCoverURL, "/local/covers/temp_") {
		return tempCoverURL, nil
	}

	coverDir, err := ensureManagedImageDir("covers")
	if err != nil {
		return tempCoverURL, err
	}

	tempFileName := filepath.Base(tempCoverURL)
	ext := strings.ToLower(filepath.Ext(tempFileName))
	if ext == "" {
		ext = ".png"
	}

	tempPath := filepath.Join(coverDir, tempFileName)
	newFileName := gameID + ext
	newPath := filepath.Join(coverDir, newFileName)

	removeFilesWithBaseName(coverDir, gameID)
	if err := os.Rename(tempPath, newPath); err != nil {
		return tempCoverURL, err
	}

	return "/local/covers/" + newFileName, nil
}
