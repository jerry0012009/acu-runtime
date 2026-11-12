package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

func validateInstalledCodex(ctx context.Context, model string) error {
	executable, err := installedCodexProbeExecutable(ctx)
	if err != nil {
		return err
	}
	return probeCodexConfiguration(ctx, executable, model)
}

func installedCodexProbeExecutable(ctx context.Context) (string, error) {
	c := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$p=Get-AppxPackage -Name OpenAI.Codex | Where-Object Publisher -eq 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B' | Select-Object -First 1;if($p){[Console]::Write($p.InstallLocation)}`)
	hideProcess(c)
	b, err := c.Output()
	if err == nil && strings.TrimSpace(string(b)) != "" {
		return filepath.Join(strings.TrimSpace(string(b)), "app", "resources", "codex.exe"), nil
	}
	// CLI-only installations are supported too. A native executable is needed
	// for stdin/stdout RPC; shell wrappers are not configuration validators.
	if executable, e := exec.LookPath("codex.exe"); e == nil {
		return executable, nil
	}
	return "", errors.New("未找到可验证配置的 Codex，请先安装 ChatGPT 或 Codex CLI")
}
