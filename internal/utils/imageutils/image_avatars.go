package imageutils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetAvatarDir returns the managed avatars directory path.
func GetAvatarDir() (string, error) {
	return ensureManagedImageDir("avatars")
}

func avatarBaseName(provider, userID string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "_" + strings.TrimSpace(userID)
}

// FindManagedAvatarFile locates the managed local avatar file for a provider/user pair.
func FindManagedAvatarFile(provider, userID string) (string, string, error) {
	avatarDir, err := GetAvatarDir()
	if err != nil {
		return "", "", err
	}

	baseName := avatarBaseName(provider, userID)
	for _, ext := range managedImageExtensions {
		fileName := baseName + ext
		absPath := filepath.Join(avatarDir, fileName)
		if _, statErr := os.Stat(absPath); statErr == nil {
			return absPath, "/local/avatars/" + fileName, nil
		}
	}

	return "", "", nil
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

	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Get(imageURL)
	if err != nil {
		return imageURL, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return imageURL, fmt.Errorf("failed to download avatar image: status %d", resp.StatusCode)
	}

	baseName := avatarBaseName(provider, userID)
	removeFilesWithBaseName(avatarDir, baseName)

	ext := detectImageExtension(resp.Header.Get("Content-Type"), imageURL)
	destFileName := baseName + ext
	destPath := filepath.Join(avatarDir, destFileName)
	if err := saveHTTPBody(resp, destPath); err != nil {
		return imageURL, err
	}

	return "/local/avatars/" + destFileName, nil
}
