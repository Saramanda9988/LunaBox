package proxyutils

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

// systemProxyRefreshInterval bounds how stale a system proxy lookup may be.
// System proxy settings can change while the application is running (for
// example a proxy client toggling the Windows system proxy), so long-lived
// HTTP clients must not freeze the selection made at construction time.
const systemProxyRefreshInterval = 2 * time.Second

// Resolver supplies the proxy for each request. Manual and direct modes are
// fixed at construction; system mode is re-read from the OS periodically so
// proxies enabled or disabled after startup are picked up.
type Resolver struct {
	dynamic bool
	now     func() time.Time
	load    func() (*ProxySelection, string, error)

	mu           sync.Mutex
	current      *ProxySelection
	expiresAt    time.Time
	knownTargets map[string]struct{}
}

// NewResolver resolves the proxy once (so configuration errors surface
// immediately) and returns a resolver plus a description of that initial
// selection.
func NewResolver(mode string, manualURL string) (*Resolver, string, error) {
	return newResolver(mode, manualURL, time.Now, func() (*ProxySelection, string, error) {
		return ResolveProxy(ProxyModeSystem, "")
	})
}

func newResolver(
	mode string,
	manualURL string,
	now func() time.Time,
	loadSystem func() (*ProxySelection, string, error),
) (*Resolver, string, error) {
	dynamic := normalizeProxyMode(mode) == ProxyModeSystem

	var (
		selection *ProxySelection
		desc      string
		err       error
	)
	if dynamic {
		selection, desc, err = loadSystem()
	} else {
		selection, desc, err = ResolveProxy(mode, manualURL)
	}
	if err != nil {
		return nil, "", err
	}

	r := &Resolver{
		dynamic:      dynamic,
		now:          now,
		load:         loadSystem,
		current:      selection,
		expiresAt:    now().Add(systemProxyRefreshInterval),
		knownTargets: make(map[string]struct{}),
	}
	r.rememberTargetsLocked(selection)
	return r, desc, nil
}

// Selection returns the current proxy selection, refreshing system settings
// when the cached lookup has expired. A failed refresh keeps the previous
// selection.
func (r *Resolver) Selection() *ProxySelection {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.dynamic {
		return r.current
	}
	now := r.now()
	if now.Before(r.expiresAt) {
		return r.current
	}
	if selection, _, err := r.load(); err == nil {
		r.current = selection
		r.rememberTargetsLocked(selection)
	}
	r.expiresAt = now.Add(systemProxyRefreshInterval)
	return r.current
}

// Proxy is suitable for http.Transport.Proxy.
func (r *Resolver) Proxy(req *http.Request) (*url.URL, error) {
	return r.Selection().Proxy(req)
}

// IsProxyDialTarget reports whether address (host:port) is, or recently was,
// a proxy endpoint chosen by this resolver. Previously seen endpoints stay
// allowed so a request whose proxy was picked just before a refresh can still
// dial it.
func (r *Resolver) IsProxyDialTarget(address string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.knownTargets[address]
	return ok
}

func (r *Resolver) rememberTargetsLocked(selection *ProxySelection) {
	if selection == nil {
		return
	}
	for target := range selection.AllowedDialTargets() {
		r.knownTargets[target] = struct{}{}
	}
}
