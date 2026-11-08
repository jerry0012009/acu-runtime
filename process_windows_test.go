package main

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestBackgroundProcessHasNoConsoleAndPreservesAttributes(t *testing.T) {
	command := exec.Command("unused")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200}
	hideProcess(command)
	if !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&0x08000000 == 0 || command.SysProcAttr.CreationFlags&0x00000200 == 0 {
		t.Fatal("background launch exposes a console or loses existing process attributes")
	}
}
