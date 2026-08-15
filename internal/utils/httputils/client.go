package httputils

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
	"resty.dev/v3"
)

// ClientOptions configures a standard application HTTP client.
type ClientOptions struct {
	Timeout     time.Duration
	ProxyConfig proxyutils.ProxyConfigProvider
	ProxyMode   string
	ProxyURL    string
	UserAgent   string
}

// NewClient creates an HTTP client with application proxy settings and a
// default User-Agent. ProxyConfig takes precedence over ProxyMode and ProxyURL.
func NewClient(options ClientOptions) (*http.Client, string, error) {
	var (
		client           *http.Client
		proxyDescription string
		err              error
	)
	if options.ProxyConfig != nil {
		client, proxyDescription, err = proxyutils.NewHTTPClientFromConfig(options.Timeout, options.ProxyConfig)
	} else {
		client, proxyDescription, err = proxyutils.NewHTTPClient(options.Timeout, options.ProxyMode, options.ProxyURL)
	}
	if err != nil {
		return nil, "", err
	}

	userAgent := resolveUserAgent(options.UserAgent)
	client.Transport = &defaultUserAgentTransport{
		base:      client.Transport,
		userAgent: userAgent,
	}
	return client, proxyDescription, nil
}

// NewRestyClient creates a Resty client with the same application proxy,
// timeout, and User-Agent behavior as NewClient. Retry conditions remain under
// the caller's control so each service can apply its own HTTP semantics.
func NewRestyClient(options ClientOptions) (*resty.Client, string, error) {
	httpClient, proxyDescription, err := NewClient(options)
	if err != nil {
		return nil, "", err
	}
	restyClient, err := NewRestyClientWithHTTPClient(httpClient, options.UserAgent)
	if err != nil {
		return nil, "", err
	}
	return restyClient, proxyDescription, nil
}

// NewRestyClientWithHTTPClient wraps an existing standard client with Resty.
// The caller retains ownership of the supplied client and its transport.
func NewRestyClientWithHTTPClient(httpClient *http.Client, userAgent string) (*resty.Client, error) {
	if httpClient == nil {
		return nil, errors.New("HTTP client is nil")
	}
	client := resty.NewWithClient(httpClient)
	client.SetHeader("User-Agent", resolveUserAgent(userAgent))
	return client, nil
}

func resolveUserAgent(userAgent string) string {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		return version.UserAgent()
	}
	return userAgent
}

type defaultUserAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *defaultUserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	if cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", t.userAgent)
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func (t *defaultUserAgentTransport) CloseIdleConnections() {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if closer, ok := base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
