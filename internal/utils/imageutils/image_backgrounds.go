package imageutils

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/gif"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// SaveBackgroundImage 保存背景图片到应用的背景目录
func SaveBackgroundImage(srcPath string) (string, error) {
	return saveManagedBackgroundCopy(srcPath, "custom_bg", "custom_bg_")
}

// SaveTempBackgroundImage 将用户选择的图片复制到应用的背景目录作为临时文件
func SaveTempBackgroundImage(srcPath string) (string, error) {
	return saveManagedBackgroundCopy(srcPath, "temp_bg", "temp_bg_")
}

// CropAndSaveBackgroundImage 裁剪并保存背景图片到应用的背景目录
// srcPath 可以是本地文件系统路径或 /local/backgrounds/xxx 格式的相对路径
func CropAndSaveBackgroundImage(srcPath string, x, y, width, height int) (string, error) {
	bgDir, err := ensureManagedImageDir("backgrounds")
	if err != nil {
		return "", err
	}

	actualPath := srcPath
	if strings.HasPrefix(srcPath, "/local/backgrounds/") {
		actualPath = filepath.Join(bgDir, strings.TrimPrefix(srcPath, "/local/backgrounds/"))
	}

	srcFile, err := os.Open(actualPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	img, format, err := image.Decode(srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	if x < 0 || y < 0 || x+width > bounds.Max.X || y+height > bounds.Max.Y {
		return "", fmt.Errorf("crop area out of image bounds")
	}

	subImager, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		return "", fmt.Errorf("image format does not support cropping")
	}
	croppedImg := subImager.SubImage(image.Rect(x, y, x+width, y+height))

	ext := strings.ToLower(filepath.Ext(actualPath))
	if ext == "" {
		ext = ".png"
		format = "png"
	}

	destFileName := fmt.Sprintf("custom_bg_%d%s", time.Now().UnixMilli(), ext)
	destPath := filepath.Join(bgDir, destFileName)
	removeFilesWithPrefixes(bgDir, "custom_bg_", "temp_bg_")

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(destFile, croppedImg, &jpeg.Options{Quality: 95})
	default:
		err = png.Encode(destFile, croppedImg)
	}
	if err != nil {
		_ = os.Remove(destPath)
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	return "/local/backgrounds/" + destFileName, nil
}
