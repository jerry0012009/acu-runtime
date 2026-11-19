package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

func validateInstalledCodex(ctx context.Context, model string) error {
	executable, desktop, err := installedCodexProbeExecutable(ctx)
	if err != nil {
		return err
	}
	err = probeCodexConfiguration(ctx, executable, model)
	if desktop && errors.Is(err, errCodexProbeStart) && ctx.Err() == nil {
		return errDesktopProbeUnavailable
	}
	return err
}

func installedCodexProbeExecutable(ctx context.Context) (string, bool, error) {
	c := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);$p=Get-AppxPackage -Name OpenAI.Codex | Where-Object Publisher -eq 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B' | Select-Object -First 1;if($p){[Console]::Write($p.InstallLocation)}`)
	hideProcess(c)
	b, err := c.Output()
	if dir := strings.TrimSpace(string(b)); err == nil && filepath.IsAbs(dir) {
		return filepath.Join(dir, "app", "resources", "codex.exe"), true, nil
	}
	// CLI-only installations are supported too. A native executable is needed
	// for stdin/stdout RPC; shell wrappers are not configuration validators.
	if executable, e := exec.LookPath("codex.exe"); e == nil {
		return executable, false, nil
	}
	return "", false, errors.New("未找到可验证配置的 Codex，请在 Runtime 中安装 ChatGPT")
}
