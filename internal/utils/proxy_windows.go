//go:build windows

package utils

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

func loadSystemProxySelection() (*ProxySelection, string, error) {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil, "", nil
		}
		return nil, "", err
	}
	defer key.Close()

	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil {
		enabled = 0
	}

	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		server = ""
	}
	server = strings.TrimSpace(server)

	autoConfigURL, _, err := key.GetStringValue("AutoConfigURL")
	if err != nil {
		autoConfigURL = ""
	}

	if enabled == 0 || server == "" {
		if strings.TrimSpace(autoConfigURL) != "" {
			return nil, "PAC detected", nil
		}
		return nil, "", nil
	}

	selection, err := parseWindowsProxyServer(server)
	if err != nil {
		return nil, "", err
	}
	selection.Source = "system"
	return selection, "", nil
}
