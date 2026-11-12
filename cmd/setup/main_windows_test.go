//go:build windows && setup

package main

import (
	"strings"
	"testing"
)

func TestManifestPinsAreConsistentAcrossKernelUpdates(t *testing.T) {
	expected := kernelManifest{"v0.1.3935", "magpie-cli-windows-amd64.exe", strings.Repeat("a", 64)}
	if err := validateKernelManifest(expected, expected); err != nil {
		t.Fatal(err)
	}
	for _, actual := range []kernelManifest{{"v0.1.3909", expected.Name, expected.SHA256}, {expected.Version, "magpie-cli-windows-arm64.exe", expected.SHA256}, {expected.Version, expected.Name, strings.Repeat("b", 64)}} {
		if err := validateKernelManifest(actual, expected); err == nil {
			t.Fatalf("mismatched payload accepted: %+v", actual)
		}
	}
	if err := validateKernelManifest(expected, kernelManifest{}); err == nil {
		t.Fatal("missing build pins accepted")
	}
}
