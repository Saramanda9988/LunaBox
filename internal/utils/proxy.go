package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	DownloadProxyModeSystem = "system"
	DownloadProxyModeManual = "manual"
	DownloadProxyModeDirect = "direct"
)

type ProxySelection struct {
	HTTPProxy  *url.URL
	HTTPSProxy *url.URL
	AllProxy   *url.URL
	Source     string
}

func (s *ProxySelection) HasProxy() bool {
	return s != nil && (s.HTTPProxy != nil || s.HTTPSProxy != nil || s.AllProxy != nil)
}

func (s *ProxySelection) Proxy(req *http.Request) (*url.URL, error) {
	if s == nil || req == nil || req.URL == nil {
		return nil, nil
	}

	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		if s.HTTPProxy != nil {
			return s.HTTPProxy, nil
		}
	case "https":
		if s.HTTPSProxy != nil {
			return s.HTTPSProxy, nil
		}
	}

	if s.AllProxy != nil {
		return s.AllProxy, nil
	}
	if s.HTTPSProxy != nil {
		return s.HTTPSProxy, nil
	}
	return s.HTTPProxy, nil
}

func (s *ProxySelection) AllowedDialTargets() map[string]struct{} {
	targets := make(map[string]struct{})
	for _, proxyURL := range []*url.URL{s.HTTPProxy, s.HTTPSProxy, s.AllProxy} {
		target := proxyDialTarget(proxyURL)
		if target == "" {
			continue
		}
		targets[target] = struct{}{}
	}
	return targets
}

func (s *ProxySelection) Description() string {
	if !s.HasProxy() {
		return DownloadProxyModeDirect
	}

	parts := make([]string, 0, 3)
	if s.HTTPProxy != nil {
		parts = append(parts, "http="+s.HTTPProxy.Redacted())
	}
	if s.HTTPSProxy != nil {
		parts = append(parts, "https="+s.HTTPSProxy.Redacted())
	}
	if s.AllProxy != nil {
		parts = append(parts, "all="+s.AllProxy.Redacted())
	}
	if s.Source == "" {
		return strings.Join(parts, ", ")
	}
	return fmt.Sprintf("%s (%s)", s.Source, strings.Join(parts, ", "))
}

func ResolveDownloadProxy(mode string, manualURL string) (*ProxySelection, string, error) {
	switch normalizeProxyMode(mode) {
	case DownloadProxyModeDirect:
		return nil, DownloadProxyModeDirect, nil
	case DownloadProxyModeManual:
		proxyURL, err := parseProxyURL(manualURL, "http")
		if err != nil {
			return nil, "", err
		}
		if proxyURL == nil {
			return nil, DownloadProxyModeManual + " (empty)", nil
		}
		selection := &ProxySelection{
			HTTPProxy:  proxyURL,
			HTTPSProxy: proxyURL,
			AllProxy:   proxyURL,
			Source:     "manual",
		}
		return selection, selection.Description(), nil
	default:
		systemSelection, systemNote, err := loadSystemProxySelection()
		if err != nil {
			return nil, "", err
		}
		if systemSelection != nil && systemSelection.HasProxy() {
			return systemSelection, systemSelection.Description(), nil
		}

		envSelection, err := loadEnvironmentProxySelection()
		if err != nil {
			return nil, "", err
		}
		if envSelection.HasProxy() {
			return envSelection, envSelection.Description(), nil
		}
		if systemNote != "" {
			return nil, DownloadProxyModeSystem + " (" + systemNote + ", no static proxy)", nil
		}
		return nil, DownloadProxyModeSystem + " (no proxy)", nil
	}
}

func normalizeProxyMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", DownloadProxyModeSystem:
		return DownloadProxyModeSystem
	case DownloadProxyModeManual:
		return DownloadProxyModeManual
	case DownloadProxyModeDirect:
		return DownloadProxyModeDirect
	default:
		return DownloadProxyModeSystem
	}
}

func parseProxyURL(raw string, defaultScheme string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if !strings.Contains(value, "://") {
		value = defaultScheme + "://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("parse proxy url: %w", err)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("proxy host is required")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
	}
}

func proxyDialTarget(proxyURL *url.URL) string {
	if proxyURL == nil {
		return ""
	}

	host := proxyURL.Hostname()
	if host == "" {
		return ""
	}

	port := proxyURL.Port()
	if port == "" {
		switch strings.ToLower(proxyURL.Scheme) {
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}

	return net.JoinHostPort(host, port)
}

func parseWindowsProxyServer(raw string) (*ProxySelection, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return &ProxySelection{Source: "system"}, nil
	}
	if !strings.Contains(value, "=") {
		proxyURL, err := parseProxyURL(value, "http")
		if err != nil {
			return nil, err
		}
		return &ProxySelection{
			HTTPProxy:  proxyURL,
			HTTPSProxy: proxyURL,
			Source:     "system",
		}, nil
	}

	selection := &ProxySelection{Source: "system"}
	for _, entry := range strings.Split(value, ";") {
		if err := applyWindowsProxyEntry(selection, entry); err != nil {
			return nil, err
		}
	}
	return selection, nil
}

func applyWindowsProxyEntry(selection *ProxySelection, entry string) error {
	name, value, ok := strings.Cut(strings.TrimSpace(entry), "=")
	if !ok || value == "" {
		return nil
	}

	key := strings.ToLower(strings.TrimSpace(name))
	defaultScheme := "http"
	if key == "socks" || key == "socks5" {
		defaultScheme = "socks5"
	}

	proxyURL, err := parseProxyURL(value, defaultScheme)
	if err != nil {
		return err
	}

	switch key {
	case "http":
		selection.HTTPProxy = proxyURL
	case "https":
		selection.HTTPSProxy = proxyURL
	case "socks", "socks5":
		selection.AllProxy = proxyURL
	default:
		selection.AllProxy = proxyURL
	}
	return nil
}

func loadEnvironmentProxySelection() (*ProxySelection, error) {
	httpProxy, err := proxyFromEnvironment("http://example.invalid")
	if err != nil {
		return nil, err
	}
	httpsProxy, err := proxyFromEnvironment("https://example.invalid")
	if err != nil {
		return nil, err
	}

	return &ProxySelection{
		HTTPProxy:  httpProxy,
		HTTPSProxy: httpsProxy,
		Source:     "environment",
	}, nil
}

func proxyFromEnvironment(rawURL string) (*url.URL, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build proxy probe request: %w", err)
	}
	proxyURL, err := http.ProxyFromEnvironment(req)
	if err != nil {
		return nil, fmt.Errorf("resolve proxy from environment: %w", err)
	}
	return proxyURL, nil
}
