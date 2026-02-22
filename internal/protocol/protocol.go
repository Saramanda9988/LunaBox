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
// Supports: lunabox://install?url=...&title=...&vndb=...&size=...
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
		URL:    q.Get("url"),
		Title:  q.Get("title"),
		VndbID: q.Get("vndb"),
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
