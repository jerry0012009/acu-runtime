param([Parameter(Mandatory)][string]$Installer, [ValidateSet('0.1.137','0.1.138','0.1.139')][string]$Version)
$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true' -or !$IsWindows) { throw 'Disposable Windows CI only' }
$installed = Join-Path $env:LOCALAPPDATA 'Programs/ACU Runtime'
if (Test-Path -LiteralPath $installed) { throw 'Expected clean installation directory' }
$installerPath = (Resolve-Path -LiteralPath $Installer).Path
Add-Type -Path (Join-Path $PSScriptRoot 'InstallerProbe.cs')
$oldTemp = $env:TEMP
$oldTmp = $env:TMP
$tempRoot = (New-Item -ItemType Directory -Path (Join-Path $env:RUNNER_TEMP ([guid]::NewGuid().ToString()))).FullName
$env:TEMP = $env:TMP = $tempRoot
$probe = [InstallerProbe]::new($tempRoot)
try {
    if ($Version -eq '0.1.137') {
        $setup = Start-Process -FilePath $installerPath -PassThru
        try {
            $deadline = [datetime]::UtcNow.AddSeconds(60)
            do {
                $setup.Refresh()
                if ($setup.HasExited) { throw "No error dialog: installer exited with $($setup.ExitCode)" }
                if ($setup.MainWindowTitle -eq 'ACU Runtime 安装未完成') { break }
                Start-Sleep -Milliseconds 100
            } while ([datetime]::UtcNow -lt $deadline)
            if ($setup.MainWindowTitle -ne 'ACU Runtime 安装未完成' -or !$probe.Locked) { throw 'Expected installer-owned failure dialog after fault injection' }
            $readDeadline = [datetime]::UtcNow.AddSeconds(5)
            do {
                $setup.Refresh()
                try { $body = [InstallerProbe]::ReadDialogBody($setup.MainWindowHandle); break }
                catch { if ([datetime]::UtcNow -gt $readDeadline) { throw }; Start-Sleep -Milliseconds 300 }
            } while ($true)
            if ($body.Length -ne 0) { throw "Dialog was not empty: $body" }
            Write-Host "Title='$($setup.MainWindowTitle)'; OK button present; body length=$($body.Length); first staged process exited 0; exclusive lock acquired."
            [InstallerProbe]::Accept($setup.MainWindowHandle)
            if (!$setup.WaitForExit(10000) -or $setup.ExitCode -ne 1 -or (Test-Path -LiteralPath $installed)) { throw 'Expected a failed first installation' }
        } finally { if (!$setup.HasExited) { $setup.Kill() } }
    } else {
        # No command-line flags. The same watcher remains armed throughout.
        & (Join-Path $PSScriptRoot 'smoke-windows-install.ps1') -Installer $installerPath -Version $Version -LegacyInstaller
        if ($probe.ObservedManifest) { throw 'Unexpected execution of staged Runtime' }
        Write-Host 'Same fault trigger armed; no staged Runtime process observed; clean installation ready.'
    }
} finally {
    try { $probe.Dispose() } finally {
        $env:TEMP = $oldTemp
        $env:TMP = $oldTmp
    }
}
