package main

func tapnowChromeLaunchArgs(address string) []string {
	if address == "chrome://extensions/" {
		address = "about:blank"
	}
	return []string{"--new-window", address}
}
