//go:build desktop

package main

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestDesktopIcon(t *testing.T) {
	icon, err := png.Decode(bytes.NewReader(desktopIcon()))
	if err != nil {
		t.Fatal(err)
	}
	if icon.Bounds().Dx() != 64 || icon.Bounds().Dy() != 64 {
		t.Fatal("unexpected native icon size")
	}
	red, green, blue, _ := icon.At(29, 11).RGBA()
	if red>>8 != 235 || green>>8 != 141 || blue>>8 != 34 {
		t.Fatal("native icon does not match the ACU amber palette")
	}
}

func TestLoginWindowBootstrapsBeforeNavigatingToTheWebsite(t *testing.T) {
	options := accountWindowOptions(loginTicket{Site: "https://acu.best", State: "fixture-nonce", Expires: time.Now().Add(time.Minute)})
	if options.HTML == "" || options.JS == "" || options.URL != "" {
		t.Fatal("remote login must start in an HTML bootstrap, not queue ExecJS on a remote URL")
	}
	if !strings.Contains(options.HTML, "wails:runtime:ready") || !strings.Contains(options.HTML, "https://acu.best/sign-in") || !strings.Contains(options.JS, "acu-account-ready") || strings.Contains(options.JS, "__ACU_LOGIN_TICKET__") {
		t.Fatal("native bridge bootstrap or document-start helper is missing")
	}
}
