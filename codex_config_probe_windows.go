package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

func validateInstalledCodex(ctx context.Context, model string) error {
	c := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$p=Get-AppxPackage -Name OpenAI.Codex | Where-Object Publisher -eq 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B' | Select-Object -First 1;if($p){[Console]::Write($p.InstallLocation)}`)
	hideProcess(c)
	b, err := c.Output()
	if err != nil || strings.TrimSpace(string(b)) == "" {
		return errors.New("未找到官方 ChatGPT 桌面端，未应用 Codex 接入")
	}
	return probeCodexConfiguration(ctx, filepath.Join(strings.TrimSpace(string(b)), "app", "resources", "codex.exe"), model)
}
