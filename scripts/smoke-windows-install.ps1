param([Parameter(Mandatory=$true)][string]$Installer)
$ErrorActionPreference = 'Stop'
$target = Join-Path $env:LOCALAPPDATA 'Programs/ACU Runtime'
if (Test-Path -LiteralPath $target) { throw 'Clean runner required: Runtime already exists' }
$setup = Start-Process -FilePath (Resolve-Path -LiteralPath $Installer).Path -ArgumentList @('--target', ('"' + $target + '"')) -WindowStyle Hidden -PassThru
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
    $html = (Invoke-WebRequest -UseBasicParsing -Uri $origin -TimeoutSec 5).Content
    $token = [regex]::Match($html, 'name="acu-token" content="([^"]+)"').Groups[1].Value
    if (!$token) { throw 'Installed Runtime did not serve session UI' }
    Invoke-RestMethod -Uri ($origin + '/runtime/desktop/quit') -Method Post -Headers @{'X-ACU-Token'=$token} -ContentType 'application/json' -Body '{}' -TimeoutSec 5 | Out-Null
    Write-Output "Clean native Runtime installation verified: $expected, ready=true"
} finally {
    # Only this disposable runner's freshly installed Runtime is stopped.
    Get-Process -Name 'ACU Runtime' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq (Join-Path $target 'ACU Runtime.exe') } | Stop-Process -ErrorAction SilentlyContinue
}
