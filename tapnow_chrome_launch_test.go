package main

import (
	"reflect"
	"testing"
)

func TestTapnowChromeLaunchDoesNotClaimInternalNavigation(t *testing.T) {
	for _, address := range []string{"chrome://extensions/", "about:blank"} {
		if got := tapnowChromeLaunchArgs(address); !reflect.DeepEqual(got, []string{"--new-window", "about:blank"}) {
			t.Fatal(got)
		}
	}
	address := "chrome-extension://" + tapnowCompanionID + "/connect.html"
	if got := tapnowChromeLaunchArgs(address); got[1] != "https://app.tapnow.ai/" {
		t.Fatal(got)
	}
}
