//go:build !windows

package main

import "context"

func prepareKernelDetection(context.Context, string) error { return nil }
