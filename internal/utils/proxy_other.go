//go:build !windows

package utils

func loadSystemProxySelection() (*ProxySelection, string, error) {
	return nil, "", nil
}
