package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SaveCoverImage 保存封面图片到应用的封面目录
func SaveCoverImage(srcPath string, gameID string) (string, error) {
	// 获取应用程序目录
	appDir, err := GetDataDir()
	if err != nil {
		return "", err
	}

	// 获取封面保存目录
	coverDir := filepath.Join(appDir, "covers")
	if err := os.MkdirAll(coverDir, os.ModePerm); err != nil {
		return "", err
	}

	// 删除该 gameID 的旧封面文件（可能是不同扩展名）
	oldExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
	for _, ext := range oldExtensions {
		oldPath := filepath.Join(coverDir, gameID+ext)
		os.Remove(oldPath) // 忽略错误，文件可能不存在
	}

	// 获取源文件的扩展名
	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	// 生成目标文件名
	destFileName := fmt.Sprintf("%s%s", gameID, ext)
	destPath := filepath.Join(coverDir, destFileName)

	// 复制文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", err
	}

	// 返回相对路径或可访问的 URL
	return fmt.Sprintf("/local/covers/%s", destFileName), nil
}

// ResolveCoverPath 解析封面图片路径
func ResolveCoverPath(imagePath string, tempDir string) string {
	// PotatoVN 的图片路径格式通常是 ".\\Images\\xxx_cover" 或相对路径
	// 需要转换为绝对路径

	// 移除开头的 ".\" 或 "./"
	cleanPath := strings.TrimPrefix(imagePath, ".\\")
	cleanPath = strings.TrimPrefix(cleanPath, "./")

	// 替换反斜杠为正斜杠
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	// 构建完整路径
	fullPath := filepath.Join(tempDir, cleanPath)

	// 检查文件是否存在，可能需要添加扩展名
	extensions := []string{"", ".png", ".jpg", ".jpeg", ".webp", ".gif"}
	for _, ext := range extensions {
		testPath := fullPath + ext
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
	}

	// 记录未找到封面图片的情况
	return ""
}

// DownloadAndSaveCoverImage 下载远程图片并保存到本地
func DownloadAndSaveCoverImage(imageURL string, gameID string) (string, error) {
	// 如果是本地路径，直接返回
	if strings.HasPrefix(imageURL, "/local/") || strings.HasPrefix(imageURL, "http://wails.localhost") {
		return imageURL, nil
	}

	// 如果不是 http/https URL，返回原始路径
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return imageURL, nil
	}

	// 获取应用程序目录
	appDir, err := GetDataDir()
	if err != nil {
		return imageURL, err // 失败时返回原始 URL
	}

	// 获取封面保存目录
	coverDir := filepath.Join(appDir, "covers")
	if err := os.MkdirAll(coverDir, os.ModePerm); err != nil {
		return imageURL, err
	}

	// 下载图片
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(imageURL)
	if err != nil {
		return imageURL, err // 下载失败，返回原始 URL
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return imageURL, fmt.Errorf("failed to download image: status %d", resp.StatusCode)
	}

	// 从 Content-Type 或 URL 推断文件扩展名
	ext := ".jpg" // 默认扩展名
	contentType := resp.Header.Get("Content-Type")
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	default:
		// 尝试从 URL 获取扩展名
		if urlExt := filepath.Ext(imageURL); urlExt != "" {
			ext = urlExt
		}
	}

	// 删除该 gameID 的旧封面文件（可能是不同扩展名）
	oldExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
	for _, oldExt := range oldExtensions {
		oldPath := filepath.Join(coverDir, gameID+oldExt)
		os.Remove(oldPath) // 忽略错误
	}

	// 生成目标文件名
	destFileName := fmt.Sprintf("%s%s", gameID, ext)
	destPath := filepath.Join(coverDir, destFileName)

	// 保存文件
	destFile, err := os.Create(destPath)
	if err != nil {
		return imageURL, err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, resp.Body); err != nil {
		os.Remove(destPath) // 清理失败的文件
		return imageURL, err
	}

	// 返回本地路径
	return fmt.Sprintf("/local/covers/%s", destFileName), nil
}

// RenameTempCover 将临时封面图片重命名为正式的游戏ID
func RenameTempCover(tempCoverURL string, gameID string) (string, error) {
	// 从 URL 中提取文件名，格式如 /local/covers/temp_xxx.png
	if !strings.Contains(tempCoverURL, "/local/covers/temp_") {
		return tempCoverURL, nil
	}

	// 获取应用程序目录
	appDir, err := GetDataDir()
	if err != nil {
		return tempCoverURL, err
	}

	coverDir := filepath.Join(appDir, "covers")

	// 提取临时文件名
	parts := strings.Split(tempCoverURL, "/")
	tempFileName := parts[len(parts)-1] // temp_xxx.png
	ext := filepath.Ext(tempFileName)

	tempPath := filepath.Join(coverDir, tempFileName)
	newFileName := gameID + ext
	newPath := filepath.Join(coverDir, newFileName)

	// 删除该 gameID 的旧封面文件（可能是不同扩展名）
	oldExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
	for _, oldExt := range oldExtensions {
		oldPath := filepath.Join(coverDir, gameID+oldExt)
		os.Remove(oldPath) // 忽略错误
	}

	// 重命名文件
	if err := os.Rename(tempPath, newPath); err != nil {
		return tempCoverURL, err
	}

	return fmt.Sprintf("/local/covers/%s", newFileName), nil
}

// SaveBackgroundImage 保存背景图片到应用的背景目录
func SaveBackgroundImage(srcPath string) (string, error) {
	// 获取应用程序目录
	appDir, err := GetDataDir()
	if err != nil {
		return "", err
	}

	// 获取背景保存目录
	bgDir := filepath.Join(appDir, "backgrounds")
	if err := os.MkdirAll(bgDir, os.ModePerm); err != nil {
		return "", err
	}

	// 获取源文件的扩展名
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == "" {
		ext = ".png"
	}

	// 生成目标文件名 (使用固定名称，每次覆盖)
	destFileName := fmt.Sprintf("custom_bg%s", ext)
	destPath := filepath.Join(bgDir, destFileName)

	// 删除旧的背景文件（可能是不同扩展名）
	oldExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}
	for _, oldExt := range oldExtensions {
		oldPath := filepath.Join(bgDir, "custom_bg"+oldExt)
		os.Remove(oldPath) // 忽略错误，文件可能不存在
	}

	// 复制文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", err
	}

	// 返回可访问的 URL
	return fmt.Sprintf("/local/backgrounds/%s", destFileName), nil
}
