package main

import "strings"

func tapnowChromeLaunchArgs(address string) []string {
	if address == "chrome://extensions/" {
		address = "about:blank"
	}
	if strings.HasPrefix(address, "chrome-extension://") {
		address = "https://app.tapnow.ai/"
	}
	return []string{"--new-window", address}
}
