param([Parameter(Mandatory=$true)][string]$Installer, [string]$PreviousInstaller)
$ErrorActionPreference = 'Stop'
$target = Join-Path $env:LOCALAPPDATA 'Programs/ACU Runtime'
if (Test-Path -LiteralPath $target) { throw 'Clean runner required: Runtime already exists' }
function Wait-InstalledRuntime([string]$Version) {
    $deadline = (Get-Date).AddSeconds(90)
    do {
        foreach ($app in @(Get-Process -Name 'ACU Runtime' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq (Join-Path $target 'ACU Runtime.exe') })) {
            foreach ($connection in @(Get-NetTCPConnection -OwningProcess $app.Id -State Listen -ErrorAction SilentlyContinue | Where-Object LocalAddress -eq '127.0.0.1')) {
                try {
                    $candidate = 'http://127.0.0.1:' + $connection.LocalPort
                    $status = Invoke-RestMethod -Uri ($candidate + '/runtime/status') -TimeoutSec 3
                    if ($status.version -eq $Version -and $status.desktop -and $status.ready) { return @{origin=$candidate; process=$app} }
                } catch {}
            }
        }
        Start-Sleep -Milliseconds 500
    } until ((Get-Date) -gt $deadline)
    throw "Installed Runtime $Version failed to reach ready state"
}
$configHashes = @{}
if ($PreviousInstaller) {
    $previous = Start-Process -FilePath (Resolve-Path -LiteralPath $PreviousInstaller).Path -ArgumentList @('--target', ('"' + $target + '"')) -WindowStyle Hidden -PassThru
    if (!$previous.WaitForExit(60000) -or $previous.ExitCode -ne 0) { throw 'Previous release installation failed' }
    $old = Wait-InstalledRuntime '0.1.56'
    $codexConfig = Join-Path $env:USERPROFILE '.codex/config.toml'
    if (!(Test-Path -LiteralPath $codexConfig)) {
        New-Item -ItemType Directory -Force (Split-Path $codexConfig -Parent) | Out-Null
        '[mcp_servers.upgrade_preservation_fixture]`ncommand = "preserved-by-runtime-upgrade"' -replace '`n', "`n" | Set-Content -LiteralPath $codexConfig -Encoding utf8NoBOM
    }
    foreach ($file in @((Join-Path $env:USERPROFILE '.codex/config.toml'), (Join-Path $env:USERPROFILE '.codex/auth.json'))) {
        if (Test-Path -LiteralPath $file) { $configHashes[$file] = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash }
    }
    $html = (Invoke-WebRequest -Uri $old.origin -TimeoutSec 5).Content
    $token = [regex]::Match($html, 'name="acu-token" content="([^"]+)"').Groups[1].Value
    # Exercise the same exact-process handoff used by the in-app updater.
    $setup = Start-Process -FilePath (Resolve-Path -LiteralPath $Installer).Path -ArgumentList @('--wait-pid', $old.process.Id, '--target', ('"' + $target + '"')) -WindowStyle Hidden -PassThru
    Invoke-RestMethod -Uri ($old.origin + '/runtime/desktop/quit') -Method Post -Headers @{'X-ACU-Token'=$token;Origin=$old.origin} -ContentType 'application/json' -Body '{}' -TimeoutSec 5 | Out-Null
} else {
    $setup = Start-Process -FilePath (Resolve-Path -LiteralPath $Installer).Path -ArgumentList @('--target', ('"' + $target + '"')) -WindowStyle Hidden -PassThru
}
if (!$setup.WaitForExit(60000) -or $setup.ExitCode -ne 0) { throw 'Runtime first installation failed' }
$expected = [regex]::Match((Get-Content update.go -Raw), 'const appVersion = "([^"]+)"').Groups[1].Value
$origin = $null
try {
    $deadline = (Get-Date).AddSeconds(90)
    do {
        $apps = @(Get-Process -Name 'ACU Runtime' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq (Join-Path $target 'ACU Runtime.exe') })
        foreach ($app in $apps) {
            foreach ($connection in @(Get-NetTCPConnection -OwningProcess $app.Id -State Listen -ErrorAction SilentlyContinue | Where-Object LocalAddress -eq '127.0.0.1')) {
                try {
                    $candidate = 'http://127.0.0.1:' + $connection.LocalPort
                    $status = Invoke-RestMethod -Uri ($candidate + '/runtime/status') -TimeoutSec 3
                    if ($status.version -eq $expected -and $status.desktop -and $status.ready) { $origin = $candidate; break }
                } catch {}
            }
            if ($origin) { break }
        }
        if (!$origin) { Start-Sleep -Milliseconds 500 }
    } until ($origin -or (Get-Date) -gt $deadline)
    if (!$origin) { throw 'Installed native Runtime failed to reach ready state' }
    foreach ($file in $configHashes.Keys) {
        if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash -ne $configHashes[$file]) { throw 'Runtime upgrade changed an existing Codex configuration or identity' }
    }
    $kernel = Invoke-RestMethod 'http://127.0.0.1:3425/' -TimeoutSec 5
    $kernelExpected = [regex]::Match((Get-Content kernel_install.go -Raw), 'const kernelVersion = "v([^"]+)"').Groups[1].Value
    if ($kernel.version -ne $kernelExpected) { throw 'Updated Runtime is still serving the previous kernel' }
    $html = (Invoke-WebRequest -UseBasicParsing -Uri $origin -TimeoutSec 5).Content
    $token = [regex]::Match($html, 'name="acu-token" content="([^"]+)"').Groups[1].Value
    if (!$token) { throw 'Installed Runtime did not serve session UI' }
    Invoke-RestMethod -Uri ($origin + '/runtime/desktop/quit') -Method Post -Headers @{'X-ACU-Token'=$token} -ContentType 'application/json' -Body '{}' -TimeoutSec 5 | Out-Null
    Write-Output "Clean native Runtime installation verified: $expected, ready=true"
} finally {
    # Only this disposable runner's freshly installed Runtime is stopped.
    Get-Process -Name 'ACU Runtime' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq (Join-Path $target 'ACU Runtime.exe') } | Stop-Process -ErrorAction SilentlyContinue
}
