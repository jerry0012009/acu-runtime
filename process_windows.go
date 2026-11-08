package main

import (
	"os/exec"
	"syscall"
)

func hideProcess(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW: no console/taskbar entry for background tools.
}
