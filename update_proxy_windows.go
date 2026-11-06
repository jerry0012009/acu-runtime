package main

import (
	"golang.org/x/sys/windows/registry"
	"net/url"
)

func systemUpdateProxy(scheme string) *url.URL {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil
	}
	value, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return nil
	}
	return parseSystemProxy(value, scheme)
}
