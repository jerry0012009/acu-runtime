package main

import (
	"context"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func systemNetworkProxy(_ context.Context) (string, []string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return "", nil, false
	}
	defer key.Close()
	on, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || on == 0 {
		return "", nil, false
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return "", nil, false
	}
	exclusions, _, _ := key.GetStringValue("ProxyOverride")
	address := strings.TrimSpace(server)
	if strings.Contains(address, "=") {
		entries := make(map[string]string)
		for _, entry := range strings.Split(address, ";") {
			if scheme, value, ok := strings.Cut(entry, "="); ok {
				entries[strings.ToLower(strings.TrimSpace(scheme))] = strings.TrimSpace(value)
			}
		}
		address = ""
		for _, scheme := range []string{"https", "http", "socks"} {
			if value := entries[scheme]; value != "" {
				address = value
				if scheme == "socks" && !strings.Contains(value, "://") {
					address = "socks5://" + value
				}
				break
			}
		}
	}
	return address, strings.Split(exclusions, ";"), address != ""
}
