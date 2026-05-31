package imageutils

import (
	"fmt"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/proxyutils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetCoverDir returns the managed covers directory path.
func GetCoverDir() (string, error) {
	return ensureManagedImageDir("covers")
}

// SaveCoverImage 保存封面图片到应用的封面目录
func SaveCoverImage(srcPath string, gameID string) (string, error) {
	coverDir, err := GetCoverDir()
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

// SaveCoverImageBytes 保存封面图片字节到应用的封面目录。
func SaveCoverImageBytes(data []byte, gameID string, contentType string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("cover image data is empty")
	}

	ext, ok := imageExtensionFromContentType(contentType)
	if !ok {
		return "", fmt.Errorf("unsupported cover image type: %s", contentType)
	}

	coverDir, err := GetCoverDir()
	if err != nil {
		return "", err
	}

	removeFilesWithBaseName(coverDir, gameID)

	destFileName := gameID + ext
	destPath := filepath.Join(coverDir, destFileName)
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		_ = os.Remove(destPath)
		return "", err
	}

	return "/local/covers/" + destFileName, nil
}

// FindManagedCoverFile locates the managed local cover file for a game ID.
func FindManagedCoverFile(gameID string) (string, string, error) {
	coverDir, err := GetCoverDir()
	if err != nil {
		return "", "", err
	}

	for _, ext := range managedImageExtensions {
		fileName := gameID + ext
		absPath := filepath.Join(coverDir, fileName)
		if _, statErr := os.Stat(absPath); statErr == nil {
			return absPath, "/local/covers/" + fileName, nil
		}
	}

	return "", "", nil
}

// RemoveManagedCover removes all managed local cover files for a game ID.
func RemoveManagedCover(gameID string) error {
	coverDir, err := GetCoverDir()
	if err != nil {
		return err
	}

	removeFilesWithBaseName(coverDir, gameID)
	return nil
}

// PrepareManagedCoverDestination clears old cover variants and returns the target absolute path and local URL.
func PrepareManagedCoverDestination(gameID, ext string) (string, string, error) {
	coverDir, err := GetCoverDir()
	if err != nil {
		return "", "", err
	}

	if ext == "" {
		ext = ".jpg"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	ext = strings.ToLower(ext)

	removeFilesWithBaseName(coverDir, gameID)

	fileName := gameID + ext
	return filepath.Join(coverDir, fileName), "/local/covers/" + fileName, nil
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
	return DownloadAndSaveCoverImageWithClient(nil, imageURL, gameID)
}

func DownloadAndSaveCoverImageWithProxy(imageURL string, gameID string, proxyMode string, proxyURL string) (string, error) {
	if isLocalOrUnsupportedImageURL(imageURL) {
		return imageURL, nil
	}

	client, err := newImageHTTPClient(30*time.Second, proxyMode, proxyURL)
	if err != nil {
		return imageURL, fmt.Errorf("create cover image download client: %w", err)
	}
	return DownloadAndSaveCoverImageWithClient(client, imageURL, gameID)
}

func DownloadAndSaveCoverImageWithProxyConfig(imageURL string, gameID string, proxyConfig proxyutils.ProxyConfigProvider) (string, error) {
	if isLocalOrUnsupportedImageURL(imageURL) {
		return imageURL, nil
	}

	client, err := newImageHTTPClientFromConfig(30*time.Second, proxyConfig)
	if err != nil {
		return imageURL, fmt.Errorf("create cover image download client: %w", err)
	}
	return DownloadAndSaveCoverImageWithClient(client, imageURL, gameID)
}

func DownloadAndSaveCoverImageWithClient(client *http.Client, imageURL string, gameID string) (string, error) {
	if isLocalOrUnsupportedImageURL(imageURL) {
		return imageURL, nil
	}

	coverDir, err := GetCoverDir()
	if err != nil {
		return imageURL, err
	}

	if client == nil {
		var clientErr error
		client, clientErr = newSystemImageHTTPClient(30 * time.Second)
		if clientErr != nil {
			return imageURL, fmt.Errorf("create cover image download client: %w", clientErr)
		}
	}
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

func isLocalOrUnsupportedImageURL(imageURL string) bool {
	return strings.HasPrefix(imageURL, "/local/") ||
		strings.HasPrefix(imageURL, "http://wails.localhost") ||
		(!strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://"))
}

// RenameTempCover 将临时封面图片重命名为正式的游戏ID
func RenameTempCover(tempCoverURL string, gameID string) (string, error) {
	if !strings.Contains(tempCoverURL, "/local/covers/temp_") {
		return tempCoverURL, nil
	}

	coverDir, err := GetCoverDir()
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
