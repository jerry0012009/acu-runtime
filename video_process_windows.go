package main

import "os/exec"

func startVideoCommand(cmd *exec.Cmd) (func(), error) { return startOwnedCommand(cmd) }
