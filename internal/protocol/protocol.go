// Package protocol handles the lunabox:// custom URL scheme.
package protocol

import (
	"fmt"
	"lunabox/internal/vo"
	"net/url"
	"strconv"
)

const Scheme = "lunabox"

// ParseURL parses a lunabox:// URI into an InstallRequest.
// Supports: lunabox://install?url=...&title=...&download_source=Shionlib&source=vndb&meta_id=V2920&size=...
// Legacy compat: &vndb=V2920 is treated as source=vndb&meta_id=V2920
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
		Title:          q.Get("title"),
		DownloadSource: q.Get("download_source"),
		MetaSource:     q.Get("source"),
		MetaID:         q.Get("meta_id"),
	}
	// 兼容旧版 ?vndb=V2920 写法
	if req.MetaID == "" && req.MetaSource == "" {
		if vndb := q.Get("vndb"); vndb != "" {
			req.MetaSource = "vndb"
			req.MetaID = vndb
		}
	}
	if req.URL == "" {
		return nil, fmt.Errorf("missing required parameter: url")
	}
	if s := q.Get("size"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			req.Size = n
		}
	}
	return req, nil
}

// IsProtocolURL reports whether the string looks like a lunabox:// URL.
func IsProtocolURL(s string) bool {
	return len(s) > len(Scheme)+3 && s[:len(Scheme)+3] == Scheme+"://"
}
