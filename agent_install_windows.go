package main

import (
	"context"
	"errors"
	"os/exec"
)

func installWorkBuddy(ctx context.Context, path string) error {
	// The official Electron/NSIS per-user installer supports /S. Windows
	// Authenticode is checked in addition to the pinned SHA-256 before launch.
	verify := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$s=Get-AuthenticodeSignature -LiteralPath $env:ACU_AGENT_INSTALLER; if ($s.Status -ne 'Valid' -or $s.SignerCertificate.Subject -notmatch 'O=Tencent Technology \(Shenzhen\) Company Limited') { exit 1 }`)
	verify.Env = appendEnvironment("ACU_AGENT_INSTALLER", path)
	hideProcess(verify)
	if err := verify.Run(); err != nil {
		return errors.New("WorkBuddy 官方签名验证失败，未执行安装")
	}
	command := exec.CommandContext(ctx, path, "/S")
	hideProcess(command)
	if err := command.Run(); err != nil {
		return errors.New("WorkBuddy 安装未完成；如系统拒绝安装，请检查系统提示后重试")
	}
	return nil
}

func openChatGPTInstaller(ctx context.Context) error {
	// Use the documented Microsoft installer; do not distribute an unverified
	// third-party repack. Domestic users use WorkBuddy's first-party CDN instead.
	return openAgentURL(ctx, "https://get.microsoft.com/installer/download/9PLM9XGG6VKS?cid=website_cta_psi")
}

func openAgentURL(ctx context.Context, address string) error {
	command := exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", address)
	hideProcess(command)
	return command.Run()
}

func openInstalledAgent(ctx context.Context, id string) error {
	if id == "codex" {
		command := exec.Command("explorer.exe", `shell:AppsFolder\OpenAI.Codex_2p2nqsd0c76g0!App`)
		if err := command.Start(); err != nil {
			return err
		}
		go command.Wait()
		return nil
	}
	for _, path := range workBuddyExecutableCandidates() {
		if regularFile(path) {
			command := exec.Command(path)
			if err := command.Start(); err != nil {
				return err
			}
			go command.Wait()
			return nil
		}
	}
	return errors.New("请从开始菜单打开 WorkBuddy")
}

func workBuddyInstalled() bool {
	for _, path := range workBuddyExecutableCandidates() {
		if regularFile(path) {
			return true
		}
	}
	return false
}
