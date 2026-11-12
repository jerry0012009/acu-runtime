//go:build !windows && !darwin

package main

import "errors"

func installedRuntimeExecutable(string) bool { return false }
func startupEntryExists() bool               { return false }
func readStartup() (bool, bool)              { return false, false }
func writeStartup(string, bool) error        { return errors.New("此平台暂不支持自启动") }
