package httputils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lunabox/internal/utils/proxyutils"
	"lunabox/internal/version"
)

func TestNewClientSetsDefaultUserAgent(t *testing.T) {
	userAgents := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, proxyDescription, err := NewClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if proxyDescription != proxyutils.ProxyModeDirect {
		t.Fatalf("unexpected proxy description: %q", proxyDescription)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if got := <-userAgents; got != version.UserAgent() {
		t.Fatalf("unexpected default User-Agent: %q", got)
	}
	if got := req.Header.Get("User-Agent"); got != "" {
		t.Fatalf("original request was mutated: User-Agent=%q", got)
	}
}

func TestNewClientPreservesRequestUserAgent(t *testing.T) {
	userAgents := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _, err := NewClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
		UserAgent: "configured-agent",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("User-Agent", "request-agent")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	_ = resp.Body.Close()

	if got := <-userAgents; got != "request-agent" {
		t.Fatalf("request User-Agent was replaced: %q", got)
	}
}

func TestNewClientUsesConfiguredUserAgent(t *testing.T) {
	userAgents := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _, err := NewClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
		UserAgent: "configured-agent",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	_ = resp.Body.Close()

	if got := <-userAgents; got != "configured-agent" {
		t.Fatalf("unexpected configured User-Agent: %q", got)
	}
}

func TestNewRestyClientUsesApplicationSettings(t *testing.T) {
	userAgents := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		userAgents <- req.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, proxyDescription, err := NewRestyClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeDirect,
	})
	if err != nil {
		t.Fatalf("NewRestyClient failed: %v", err)
	}
	if proxyDescription != proxyutils.ProxyModeDirect {
		t.Fatalf("unexpected proxy description: %q", proxyDescription)
	}

	response, err := client.R().Get(server.URL)
	if err != nil {
		t.Fatalf("perform request: %v", err)
	}
	if response.StatusCode() != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", response.StatusCode())
	}
	if got := <-userAgents; got != version.UserAgent() {
		t.Fatalf("unexpected default User-Agent: %q", got)
	}
}

func TestNewRestyClientWithHTTPClientRejectsNil(t *testing.T) {
	client, err := NewRestyClientWithHTTPClient(nil, "")
	if err == nil {
		t.Fatal("expected nil HTTP client error")
	}
	if client != nil {
		t.Fatal("client should be nil after an error")
	}
}

func TestNewRestyClientUsesManualProxy(t *testing.T) {
	requests := make(chan *http.Request, 1)
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests <- req.Clone(req.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxyServer.Close()

	client, _, err := NewRestyClient(ClientOptions{
		Timeout:   time.Second,
		ProxyMode: proxyutils.ProxyModeManual,
		ProxyURL:  proxyServer.URL,
	})
	if err != nil {
		t.Fatalf("NewRestyClient failed: %v", err)
	}

	response, err := client.R().Get("http://resty-proxy-test.invalid/image.webp")
	if err != nil {
		t.Fatalf("perform proxied request: %v", err)
	}
	if response.StatusCode() != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", response.StatusCode())
	}

	proxied := <-requests
	if got := proxied.URL.String(); got != "http://resty-proxy-test.invalid/image.webp" {
		t.Fatalf("proxied URL = %q", got)
	}
	if got := proxied.Header.Get("User-Agent"); got != version.UserAgent() {
		t.Fatalf("proxied User-Agent = %q", got)
	}
}
