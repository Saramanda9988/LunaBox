// Package protocol handles the lunabox:// custom URL scheme.
package protocol

import (
	"fmt"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/vo"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const Scheme = "lunabox"
const (
	ActionInstall = "install"
	ActionLaunch  = "launch"
)

func parseProtocolURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, Scheme) {
		return nil, fmt.Errorf("unexpected scheme %q (expected %q)", u.Scheme, Scheme)
	}
	return u, nil
}

// ParseAction returns the supported lunabox:// action name.
func ParseAction(rawURL string) (string, error) {
	u, err := parseProtocolURL(rawURL)
	if err != nil {
		return "", err
	}

	action := strings.ToLower(strings.TrimSpace(u.Host))
	switch action {
	case ActionInstall, ActionLaunch:
		return action, nil
	default:
		return "", fmt.Errorf("unsupported action %q", u.Host)
	}
}

// ParseURL parses a lunabox://install URI into an InstallRequest.
// Supports: lunabox://install?url=...&file_name=...&archive_format=...&checksum_algo=...&checksum=...&expires_at=...
func ParseURL(rawURL string) (*vo.InstallRequest, error) {
	return ParseInstallURL(rawURL)
}

// ParseInstallURL parses a lunabox://install URI into an InstallRequest.
func ParseInstallURL(rawURL string) (*vo.InstallRequest, error) {
	u, err := parseProtocolURL(rawURL)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(u.Host, ActionInstall) {
		return nil, fmt.Errorf("unsupported action %q (only %q is supported)", u.Host, ActionInstall)
	}

	q := u.Query()
	req := &vo.InstallRequest{
		URL:            q.Get("url"),
		FileName:       q.Get("file_name"),
		ArchiveFormat:  downloadutils.NormalizeArchiveFormat(q.Get("archive_format")),
		StartupPath:    q.Get("startup_path"),
		Title:          q.Get("title"),
		DownloadSource: q.Get("download_source"),
		MetaSource:     q.Get("source"),
		MetaID:         q.Get("meta_id"),
		ChecksumAlgo:   strings.ToLower(strings.TrimSpace(q.Get("checksum_algo"))),
		Checksum:       strings.ToLower(strings.TrimSpace(q.Get("checksum"))),
	}
	if req.StartupPath == "" {
		req.StartupPath = q.Get("launch_path")
	}
	if req.MetaSource == "" {
		req.MetaSource = q.Get("meta_source")
	}

	if req.URL == "" {
		return nil, fmt.Errorf("missing required parameter: url")
	}
	if err := downloadutils.ValidateDownloadURL(req.URL); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.FileName) == "" {
		return nil, fmt.Errorf("missing required parameter: file_name")
	}
	if req.ArchiveFormat == "" {
		return nil, fmt.Errorf("missing required parameter: archive_format")
	}
	if !downloadutils.IsSupportedArchiveFormat(req.ArchiveFormat) {
		return nil, fmt.Errorf("unsupported archive_format: %s", req.ArchiveFormat)
	}

	sizeValue := strings.TrimSpace(q.Get("size"))
	if sizeValue == "" {
		return nil, fmt.Errorf("missing required parameter: size")
	}
	if n, err := strconv.ParseInt(sizeValue, 10, 64); err == nil {
		req.Size = n
	} else {
		return nil, fmt.Errorf("invalid size: %w", err)
	}
	if req.Size <= 0 {
		return nil, fmt.Errorf("size must be > 0")
	}

	expiresAtValue := strings.TrimSpace(q.Get("expires_at"))
	if expiresAtValue == "" {
		return nil, fmt.Errorf("missing required parameter: expires_at")
	}
	n, err := strconv.ParseInt(expiresAtValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid expires_at: %w", err)
	}
	req.ExpiresAt = n
	if req.ExpiresAt <= 0 {
		return nil, fmt.Errorf("expires_at must be > 0")
	}
	if req.ExpiresAt <= time.Now().Unix() {
		return nil, fmt.Errorf("install request expired")
	}

	if req.ChecksumAlgo == "" {
		return nil, fmt.Errorf("missing required parameter: checksum_algo")
	}
	if req.Checksum == "" {
		return nil, fmt.Errorf("missing required parameter: checksum")
	}
	if err := downloadutils.ValidateChecksumFields(req.ChecksumAlgo, req.Checksum); err != nil {
		return nil, err
	}

	return req, nil
}

// ParseLaunchURL parses a lunabox://launch URI into a ProtocolLaunchRequest.
func ParseLaunchURL(rawURL string) (*vo.ProtocolLaunchRequest, error) {
	u, err := parseProtocolURL(rawURL)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(u.Host, ActionLaunch) {
		return nil, fmt.Errorf("unsupported action %q (only %q is supported)", u.Host, ActionLaunch)
	}

	gameID := strings.TrimSpace(u.Query().Get("game_id"))
	if gameID == "" {
		return nil, fmt.Errorf("missing required parameter: game_id")
	}

	return &vo.ProtocolLaunchRequest{
		GameID: gameID,
		RawURL: rawURL,
	}, nil
}

// BuildLaunchURL returns a lunabox://launch URI for the given game ID.
func BuildLaunchURL(gameID string) (string, error) {
	trimmedID := strings.TrimSpace(gameID)
	if trimmedID == "" {
		return "", fmt.Errorf("game_id is required")
	}

	values := url.Values{}
	values.Set("game_id", trimmedID)
	return (&url.URL{
		Scheme:   Scheme,
		Host:     ActionLaunch,
		RawQuery: values.Encode(),
	}).String(), nil
}

// IsProtocolURL reports whether the string looks like a lunabox:// URL.
func IsProtocolURL(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), Scheme+"://")
}
