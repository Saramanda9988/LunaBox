package proxyutils

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestResolverPicksUpSystemProxyEnabledAfterStartup(t *testing.T) {
	now := time.Unix(0, 0)
	var system *ProxySelection
	loads := 0

	resolver, _, err := newResolver(ProxyModeSystem, "", func() time.Time { return now }, func() (*ProxySelection, string, error) {
		loads++
		return system, "", nil
	})
	if err != nil {
		t.Fatalf("newResolver() error = %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.test/", nil)
	if got, _ := resolver.Proxy(req); got != nil {
		t.Fatalf("Proxy() before enabling = %v, want direct", got)
	}

	// The user turns on a (non-TUN) system proxy while the app is running.
	proxyURL, _ := url.Parse("http://127.0.0.1:7890")
	system = &ProxySelection{HTTPProxy: proxyURL, HTTPSProxy: proxyURL, Source: "system"}

	if got, _ := resolver.Proxy(req); got != nil {
		t.Fatalf("Proxy() within refresh interval = %v, want cached direct", got)
	}

	now = now.Add(systemProxyRefreshInterval)
	got, _ := resolver.Proxy(req)
	if got == nil || got.String() != "http://127.0.0.1:7890" {
		t.Fatalf("Proxy() after refresh = %v, want http://127.0.0.1:7890", got)
	}
	if !resolver.IsProxyDialTarget("127.0.0.1:7890") {
		t.Fatal("IsProxyDialTarget() = false for current system proxy")
	}

	// Disabling the proxy again is also followed, while the old endpoint stays dialable.
	system = nil
	now = now.Add(systemProxyRefreshInterval)
	if got, _ := resolver.Proxy(req); got != nil {
		t.Fatalf("Proxy() after disabling = %v, want direct", got)
	}
	if !resolver.IsProxyDialTarget("127.0.0.1:7890") {
		t.Fatal("IsProxyDialTarget() forgot previously used proxy endpoint")
	}
	if loads != 3 {
		t.Fatalf("system proxy loads = %d, want 3", loads)
	}
}

func TestResolverManualModeIsStatic(t *testing.T) {
	resolver, _, err := newResolver(ProxyModeManual, "http://127.0.0.1:8080", time.Now, func() (*ProxySelection, string, error) {
		t.Fatal("manual mode must not read system proxy")
		return nil, "", nil
	})
	if err != nil {
		t.Fatalf("newResolver() error = %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example.test/", nil)
	if got, _ := resolver.Proxy(req); got == nil || got.Host != "127.0.0.1:8080" {
		t.Fatalf("Proxy() = %v, want manual proxy", got)
	}
}
