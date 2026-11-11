package imageutils

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"resty.dev/v3"
)

// AvatarRefreshInterval 是本地缓存头像的有效期：超过后下次读取资料时重新下载，
// 让用户在站点上换的头像最迟一天内同步过来。
const AvatarRefreshInterval = 24 * time.Hour

// GetAvatarDir returns the managed avatars directory path.
func GetAvatarDir() (string, error) {
	return ensureManagedImageDir("avatars")
}

func avatarBaseName(provider, userID string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "_" + strings.TrimSpace(userID)
}

// avatarLocalURL 返回头像的 /local/ 地址，带上文件修改时间作为版本号。
// 头像文件名固定（<provider>_<userID>.<ext>），换图后地址不变会让界面的图片缓存一直显示旧图，
// 所以每次重新下载都换一个 ?v=，读取本地文件时会忽略查询参数。
func avatarLocalURL(fileName string, modTime time.Time) string {
	return "/local/avatars/" + fileName + "?v=" + strconv.FormatInt(modTime.UnixMilli(), 10)
}

// findManagedAvatar 返回头像文件的路径、带版本号的地址和修改时间；没有缓存时都为空。
func findManagedAvatar(provider, userID string) (string, string, time.Time, error) {
	avatarDir, err := GetAvatarDir()
	if err != nil {
		return "", "", time.Time{}, err
	}

	baseName := avatarBaseName(provider, userID)
	for _, ext := range managedImageExtensions {
		fileName := baseName + ext
		absPath := filepath.Join(avatarDir, fileName)
		if info, statErr := os.Stat(absPath); statErr == nil && !info.IsDir() {
			return absPath, avatarLocalURL(fileName, info.ModTime()), info.ModTime(), nil
		}
	}

	return "", "", time.Time{}, nil
}

// FindManagedAvatarFile locates the managed local avatar file for a provider/user pair.
// The returned URL carries a ?v=<mtime> version so it changes whenever the file is replaced.
func FindManagedAvatarFile(provider, userID string) (string, string, error) {
	absPath, localURL, _, err := findManagedAvatar(provider, userID)
	return absPath, localURL, err
}

// ResolveManagedAvatar 返回头像的本地缓存地址，必要时重新下载：
// force 为 true（刚登录、重新授权）或缓存超过 AvatarRefreshInterval 时下载 sourceURL，否则直接用缓存。
// 下载失败时返回已有的缓存地址（可能为空）和错误，旧头像不会被删掉。
func ResolveManagedAvatar(client *http.Client, sourceURL, provider, userID string, force bool) (string, error) {
	_, cachedURL, modTime, err := findManagedAvatar(provider, userID)
	if err != nil {
		return "", err
	}
	if cachedURL != "" && !force && time.Since(modTime) < AvatarRefreshInterval {
		return cachedURL, nil
	}

	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return cachedURL, nil
	}

	localURL, err := DownloadAndSaveAvatarImageWithClient(client, sourceURL, provider, userID)
	if err != nil {
		return cachedURL, err
	}
	return localURL, nil
}

// RemoveManagedAvatar removes all managed local avatar files for a provider/user pair.
func RemoveManagedAvatar(provider, userID string) error {
	avatarDir, err := GetAvatarDir()
	if err != nil {
		return err
	}

	removeFilesWithBaseName(avatarDir, avatarBaseName(provider, userID))
	return nil
}

// DownloadAndSaveAvatarImage downloads a remote avatar and saves it to the managed avatar directory.
func DownloadAndSaveAvatarImage(imageURL, provider, userID string) (string, error) {
	return DownloadAndSaveAvatarImageWithClient(nil, imageURL, provider, userID)
}

// DownloadAndSaveAvatarImageWithClient downloads a remote avatar using the provided HTTP client and saves it locally.
// The image is written to a temporary file first, so a failed download keeps the previous avatar.
func DownloadAndSaveAvatarImageWithClient(client *http.Client, imageURL, provider, userID string) (string, error) {
	if strings.HasPrefix(imageURL, "/local/") || strings.HasPrefix(imageURL, "http://wails.localhost") {
		return imageURL, nil
	}
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return imageURL, nil
	}

	avatarDir, err := GetAvatarDir()
	if err != nil {
		return imageURL, err
	}

	var restyClient *resty.Client
	if client == nil {
		restyClient, err = newSystemImageRestyClient(30 * time.Second)
	} else {
		restyClient, err = newImageRestyClientWithHTTPClient(client)
	}
	if err != nil {
		return imageURL, fmt.Errorf("create avatar image download client: %w", err)
	}
	resp, err := newImageRequest(context.Background(), restyClient).
		SetResponseDoNotParse(true).
		Get(imageURL)
	if err != nil {
		return imageURL, err
	}
	if resp == nil || resp.Body == nil {
		return imageURL, fmt.Errorf("avatar image response is empty")
	}
	defer resp.Body.Close()

	if resp.StatusCode() != http.StatusOK {
		return imageURL, fmt.Errorf("failed to download avatar image: status %d", resp.StatusCode())
	}

	baseName := avatarBaseName(provider, userID)
	tmpPath := filepath.Join(avatarDir, baseName+".download")
	if err := saveHTTPBody(resp.Body, tmpPath); err != nil {
		return imageURL, err
	}

	ext := detectImageExtension(resp.Header().Get("Content-Type"), imageURL)
	destFileName := baseName + ext
	destPath := filepath.Join(avatarDir, destFileName)
	removeFilesWithBaseName(avatarDir, baseName)
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return imageURL, err
	}

	// 用当前时间作为修改时间，保证版本号变化（有些系统的 rename 不更新 mtime）。
	now := time.Now()
	_ = os.Chtimes(destPath, now, now)
	modTime := now
	if info, statErr := os.Stat(destPath); statErr == nil {
		modTime = info.ModTime()
	}
	return avatarLocalURL(destFileName, modTime), nil
}
