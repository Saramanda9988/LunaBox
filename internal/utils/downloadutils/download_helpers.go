package downloadutils

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
)

func NormalizeArchiveFormat(format string) string {
	return strings.ToLower(strings.TrimSpace(format))
}

func IsSupportedArchiveFormat(format string) bool {
	switch NormalizeArchiveFormat(format) {
	case "none", "zip", "rar", "7z", "tar", "tar.gz", "tar.bz2", "tar.xz", "tar.zst", "tgz", "tbz2", "txz", "tzst":
		return true
	default:
		return false
	}
}

func TrimArchiveSuffixByFormat(name string, format string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ""
	}

	lower := strings.ToLower(trimmedName)
	var suffixes []string
	switch NormalizeArchiveFormat(format) {
	case "zip":
		suffixes = []string{".zip"}
	case "rar":
		suffixes = []string{".rar"}
	case "7z":
		suffixes = []string{".7z"}
	case "tar":
		suffixes = []string{".tar"}
	case "tar.gz", "tgz":
		suffixes = []string{".tar.gz", ".tgz"}
	case "tar.bz2", "tbz2":
		suffixes = []string{".tar.bz2", ".tbz2"}
	case "tar.xz", "txz":
		suffixes = []string{".tar.xz", ".txz"}
	case "tar.zst", "tzst":
		suffixes = []string{".tar.zst", ".tzst"}
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(trimmedName[:len(trimmedName)-len(suffix)])
		}
	}

	return strings.TrimSuffix(trimmedName, filepath.Ext(trimmedName))
}

func SanitizeFileName(name string) string {
	invalid := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|', '\x00'}
	result := []rune(name)
	for i, c := range result {
		for _, inv := range invalid {
			if c == inv {
				result[i] = '_'
				break
			}
		}
	}
	s := string(result)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func SanitizeDownloadedFileName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	base := filepath.Base(trimmed)
	if base == "." || base == ".." {
		return ""
	}
	safe := strings.TrimSpace(SanitizeFileName(base))
	if safe == "" || safe == "." || safe == ".." {
		return ""
	}
	return safe
}

func BuildExpectedExtractDir(downloadedPath string, fileName string, archiveFormat string, title string) string {
	if strings.TrimSpace(downloadedPath) == "" {
		return ""
	}

	format := NormalizeArchiveFormat(archiveFormat)
	if format == "none" || !IsSupportedArchiveFormat(format) {
		return ""
	}

	baseName := TrimArchiveSuffixByFormat(strings.TrimSpace(fileName), format)
	baseName = SanitizeFileName(baseName)
	if baseName == "" {
		baseName = SanitizeFileName(title)
	}
	if baseName == "" {
		baseName = "game"
	}

	return filepath.Join(filepath.Dir(downloadedPath), baseName)
}

func ValidateDownloadURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	if isBlockedHostname(host) {
		return fmt.Errorf("url host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("url host resolves to a blocked address")
	}
	return nil
}

func ValidateChecksumFields(algo string, checksum string) error {
	trimmedAlgo := strings.ToLower(strings.TrimSpace(algo))
	trimmedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	if trimmedAlgo == "" || trimmedChecksum == "" {
		return fmt.Errorf("checksum_algo and checksum are required")
	}

	if _, err := hex.DecodeString(trimmedChecksum); err != nil {
		return fmt.Errorf("checksum must be lowercase hex")
	}

	switch trimmedAlgo {
	case "sha256", "blake3":
		if len(trimmedChecksum) != 64 {
			return fmt.Errorf("%s checksum must be 64 hex characters", trimmedAlgo)
		}
	default:
		return fmt.Errorf("unsupported checksum_algo: %s", algo)
	}

	return nil
}

func isBlockedHostname(host string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(host))
	return trimmed == "localhost" || strings.HasSuffix(trimmed, ".localhost")
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified()
}
