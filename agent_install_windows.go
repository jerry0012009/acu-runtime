package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
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

const chatGPTVerifyScript = `
$ErrorActionPreference='Stop'
Import-Module (Join-Path $PSHOME 'Modules\Microsoft.PowerShell.Security\Microsoft.PowerShell.Security.psd1') -ErrorAction Stop
$s=Get-AuthenticodeSignature -LiteralPath $env:ACU_AGENT_INSTALLER
if ($s.Status -ne 'Valid' -or $s.SignerCertificate.Subject -ne 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B') { throw 'Invalid official signature' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$z=[IO.Compression.ZipFile]::OpenRead($env:ACU_AGENT_INSTALLER)
try {
 $entry=$z.GetEntry('AppxManifest.xml')
 if (!$entry -or $entry.Length -gt 1048576) { throw 'Invalid manifest' }
 $reader=[IO.StreamReader]::new($entry.Open())
 try { [xml]$manifest=$reader.ReadToEnd() } finally { $reader.Dispose() }
 $identity=$manifest.Package.Identity
 if ($identity.Name -ne 'OpenAI.Codex' -or $identity.Publisher -ne $s.SignerCertificate.Subject -or $identity.ProcessorArchitecture -ne $env:ACU_AGENT_ARCH) { throw 'Invalid app identity' }
} finally { $z.Dispose() }
`

func chatGPTInstallerEnv(path string) []string {
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	return append(appendEnvironment("ACU_AGENT_INSTALLER", path), "ACU_AGENT_ARCH="+arch)
}

func verifyChatGPTPackage(ctx context.Context, path string) error {
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", chatGPTVerifyScript)
	command.Env = chatGPTInstallerEnv(path)
	hideProcess(command)
	if command.Run() != nil {
		return errors.New("ChatGPT 官方签名或应用身份验证失败，未执行安装；请联网重试")
	}
	return nil
}

func chatGPTInstalled() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$p=Get-AppxPackage -Name OpenAI.Codex; if (!$p -or $p.Publisher -ne 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B') { exit 1 }`)
	hideProcess(command)
	return command.Run() == nil
}

func installChatGPT(ctx context.Context, path string) error {
	// Installing packaged services needs admin rights. UAC remains a normal
	// Windows confirmation; no policy changes or app force-termination.
	dir, err := os.MkdirTemp("", "acu-chatgpt-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "install.ps1")
	content := `param([string]$PackagePath,[string]$Architecture,[string]$ExpectedUser)
$ErrorActionPreference='Stop'
if ([Security.Principal.WindowsIdentity]::GetCurrent().User.Value -ne $ExpectedUser) { exit 2 }
$env:ACU_AGENT_INSTALLER=$PackagePath
$env:ACU_AGENT_ARCH=$Architecture
` + chatGPTVerifyScript + `
try {
 $existing=Get-AppxPackage -Name OpenAI.Codex
 if ($existing -and [version]$existing.Version -ge [version]$identity.Version) { exit 0 }
 Add-AppxPackage -Path $PackagePath -ErrorAction Stop
 $p=Get-AppxPackage -Name OpenAI.Codex -ErrorAction Stop
 if (!$p -or $p.Publisher -ne 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B') { exit 3 }
} catch { exit 1 }
`
	if err := os.WriteFile(script, []byte(content), 0600); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `
$ErrorActionPreference='Stop'
$sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$argsList=@('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',('"'+$env:ACU_INSTALL_SCRIPT+'"'),'-PackagePath',('"'+$env:ACU_AGENT_INSTALLER+'"'),'-Architecture',$env:ACU_AGENT_ARCH,'-ExpectedUser',$sid)
try {
 $principal=[Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
 if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  & $env:ACU_INSTALL_SCRIPT -PackagePath $env:ACU_AGENT_INSTALLER -Architecture $env:ACU_AGENT_ARCH -ExpectedUser $sid
  exit $LASTEXITCODE
 }
 $p=Start-Process -FilePath powershell.exe -ArgumentList $argsList -Verb RunAs -WindowStyle Hidden -PassThru -Wait
 exit $p.ExitCode
} catch { exit 1 }
`)
	command.Env = append(chatGPTInstallerEnv(path), "ACU_INSTALL_SCRIPT="+script)
	hideProcess(command)
	if command.Run() != nil {
		return errors.New("ChatGPT 安装未完成。请保存并关闭 ChatGPT 后重试，并在系统提示中使用当前用户确认管理员权限；已有数据保留")
	}
	return nil
}

func openAgentURL(ctx context.Context, address string) error {
	command := exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", address)
	hideProcess(command)
	return command.Run()
}

func openInstalledAgent(ctx context.Context, id string) error {
	if id == "codex" {
		if running, err := codexDesktopOpen(); err != nil {
			return err
		} else if running {
			return nil
		}
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
