// Package protocol handles the lunabox:// custom URL scheme.
package protocol

import (
	"fmt"
	"lunabox/internal/vo"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const Scheme = "lunabox"

// ParseURL parses a lunabox:// URI into an InstallRequest.
// Supports: lunabox://install?url=...&file_name=...&archive_format=...&checksum_algo=...&checksum=...&expires_at=...
func ParseURL(rawURL string) (*vo.InstallRequest, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != Scheme {
		return nil, fmt.Errorf("unexpected scheme %q (expected %q)", u.Scheme, Scheme)
	}
	if u.Host != "install" {
		return nil, fmt.Errorf("unsupported action %q (only \"install\" is supported)", u.Host)
	}

	q := u.Query()
	req := &vo.InstallRequest{
		URL:            q.Get("url"),
		FileName:       q.Get("file_name"),
		ArchiveFormat:  strings.ToLower(strings.TrimSpace(q.Get("archive_format"))),
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
	if strings.TrimSpace(req.FileName) == "" {
		return nil, fmt.Errorf("missing required parameter: file_name")
	}
	if req.ArchiveFormat == "" {
		return nil, fmt.Errorf("missing required parameter: archive_format")
	}
	if !isSupportedArchiveFormat(req.ArchiveFormat) {
		return nil, fmt.Errorf("unsupported archive_format: %s", req.ArchiveFormat)
	}

	if s := q.Get("size"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			req.Size = n
		} else {
			return nil, fmt.Errorf("invalid size: %w", err)
		}
		if req.Size < 0 {
			return nil, fmt.Errorf("size must be >= 0")
		}
	}

	if s := q.Get("expires_at"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
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
	}

	if req.ChecksumAlgo != "" || req.Checksum != "" {
		if req.ChecksumAlgo == "" || req.Checksum == "" {
			return nil, fmt.Errorf("checksum_algo and checksum must be provided together")
		}
		switch req.ChecksumAlgo {
		case "sha256", "blake3":
		default:
			return nil, fmt.Errorf("unsupported checksum_algo: %s", req.ChecksumAlgo)
		}
	}

	return req, nil
}

func isSupportedArchiveFormat(format string) bool {
	switch format {
	case "none", "zip", "rar", "7z", "tar", "tar.gz", "tar.bz2", "tar.xz", "tar.zst", "tgz", "tbz2", "txz", "tzst":
		return true
	default:
		return false
	}
}

// IsProtocolURL reports whether the string looks like a lunabox:// URL.
func IsProtocolURL(s string) bool {
	return len(s) > len(Scheme)+3 && s[:len(Scheme)+3] == Scheme+"://"
}
