//go:build !desktop

package main

import (
	"context"
	"errors"
)

const desktopEnabled = false

func openBillingBrowser(string) error {
	return errors.New("desktop shell unavailable")
}

func runDesktop(context.Context, *server, string, int, <-chan struct{}, func()) error {
	return errors.New("this developer service build has no native shell; build with -tags desktop,production")
}
