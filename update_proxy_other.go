//go:build !windows

package main

import "net/url"

func systemUpdateProxy(string) *url.URL { return nil }
