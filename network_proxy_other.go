//go:build !windows && !darwin

package main

import "context"

func systemNetworkProxy(context.Context) (string, []string, bool) {
	return "", nil, false
}
