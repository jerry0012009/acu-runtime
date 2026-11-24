package main

import (
	"context"
	"os/exec"
	"strings"
)

func systemNetworkProxy(ctx context.Context) (string, []string, bool) {
	output, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
	if err != nil {
		return "", nil, false
	}
	values := map[string]string{}
	var exclusions []string
	inExclusions := false
	for _, line := range strings.Split(string(output), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), " : ")
		if name == "ExceptionsList" {
			inExclusions = true
		} else if name == "}" {
			inExclusions = false
		} else if ok && inExclusions {
			exclusions = append(exclusions, strings.TrimSpace(value))
		} else if ok {
			values[name] = strings.TrimSpace(value)
		}
	}
	for _, kind := range []string{"HTTPS", "HTTP", "SOCKS"} {
		if values[kind+"Enable"] != "1" {
			continue
		}
		scheme := "http"
		if kind == "SOCKS" {
			scheme = "socks5"
		}
		if address := systemProxyAddress(values[kind+"Proxy"], values[kind+"Port"], scheme); address != "" {
			return address, exclusions, true
		}
	}
	return "", nil, false
}
