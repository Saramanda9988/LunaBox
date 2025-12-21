package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SaveCoverImage 保存封面图片到应用的封面目录
func SaveCoverImage(srcPath string, gameID string) (string, error) {
	// 获取应用程序目录
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	appDir := filepath.Dir(execPath)

	// 获取封面保存目录
	coverDir := filepath.Join(appDir, "covers")
	if err := os.MkdirAll(coverDir, os.ModePerm); err != nil {
		return "", err
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
