package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

func installedDesktopVersions(ctx context.Context) map[string]string {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$v=@{}; $p=Get-AppxPackage -Name OpenAI.Codex; if($p -and $p.Publisher -eq 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B'){$v.chatgpt=[string]$p.Version};foreach($f in @((Join-Path $env:LOCALAPPDATA 'Programs/WorkBuddy/WorkBuddy.exe'),(Join-Path $env:ProgramFiles 'WorkBuddy/WorkBuddy.exe'))){if(Test-Path -LiteralPath $f){$v.'workbuddy-desktop'=(Get-Item -LiteralPath $f).VersionInfo.ProductVersion;$v.'workbuddy-path'=[IO.Path]::GetDirectoryName($f);break}};$v|ConvertTo-Json -Compress`)
	hideProcess(command)
	b, err := command.Output()
	var result map[string]string
	if err == nil {
		json.Unmarshal(b, &result)
	}
	if dir := result["workbuddy-path"]; dir != "" {
		if version, commit := workBuddyBuildMetadata(dir); version != "" {
			result["workbuddy-desktop"] = version
			result["workbuddy-commit"] = commit
		}
		delete(result, "workbuddy-path")
	}
	return result
}
