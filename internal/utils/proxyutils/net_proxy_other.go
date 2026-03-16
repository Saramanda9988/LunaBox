//go:build !windows

package proxyutils

func loadSystemProxySelection() (*ProxySelection, string, error) {
	return nil, "", nil
}
