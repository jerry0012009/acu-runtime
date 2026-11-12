param([string]$GoExecutable = 'go')
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
    $architecture = (& $GoExecutable env GOARCH).Trim()
    if ($LASTEXITCODE -ne 0 -or $architecture -notin @('amd64', 'arm64')) { throw 'Unsupported Windows architecture' }
    $output = Join-Path $root "dist/windows-$architecture"
    $kernelDirectory = Join-Path $output 'kernel'
    New-Item -ItemType Directory -Force -Path $kernelDirectory | Out-Null
    $application = Join-Path $output 'ACU Runtime.exe'
    & $GoExecutable build -tags desktop,production -ldflags '-H windowsgui' -o $application .
    if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
    $search = (& $application --search-runtime-release-json | Out-String | ConvertFrom-Json)
    $searchArchive = Join-Path $root 'web/search-runtime.archive'
    if (!(Test-Path -LiteralPath $searchArchive) -or (Get-FileHash -LiteralPath $searchArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $search.sha256) {
        python scripts/download-release.py astral-sh/uv $search.version $search.name $search.sha256 $searchArchive
        if ($LASTEXITCODE -ne 0) { throw 'Search runtime download or checksum failed' }
    }
    & $GoExecutable build -tags desktop,production -ldflags '-H windowsgui' -o $application .
    if ($LASTEXITCODE -ne 0) { throw 'Bundled desktop build failed' }
    $release = (& $application --kernel-release-json | Out-String | ConvertFrom-Json)
    if (!$release.sha256 -or $release.sha256 -notmatch '^[0-9a-f]{64}$') { throw 'Invalid kernel manifest' }
    $kernel = Join-Path $kernelDirectory 'acu-kernel.exe'
    if (!(Test-Path -LiteralPath $kernel) -or (Get-FileHash -LiteralPath $kernel -Algorithm SHA256).Hash.ToLowerInvariant() -ne $release.sha256) {
        python scripts/download-release.py yetone/magpie $release.version $release.name $release.sha256 $kernel
        if ($LASTEXITCODE -ne 0) { throw 'Kernel download or checksum failed' }
    }
    $payload = Join-Path $root 'cmd/setup/payload.zip'
    Compress-Archive -LiteralPath $application, $kernelDirectory, (Join-Path $root 'web/THIRD-PARTY-NOTICES.txt') -DestinationPath $payload -Force
    $installer = Join-Path $output 'ACU Runtime Setup.exe'
    $setupLinker = "-H windowsgui -X main.setupKernelVersion=$($release.version) -X main.setupKernelName=$($release.name) -X main.setupKernelSHA256=$($release.sha256)"
    & $GoExecutable build -tags setup -ldflags $setupLinker -o $installer ./cmd/setup
    if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
    $verifyOutput = Join-Path $output 'installer-verify.stdout.log'
    $verifyError = Join-Path $output 'installer-verify.stderr.log'
    $verification = Start-Process -FilePath $installer -ArgumentList '--verify-only' -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput $verifyOutput -RedirectStandardError $verifyError
    if (Test-Path -LiteralPath $verifyOutput) { Get-Content -LiteralPath $verifyOutput -Encoding utf8 }
    if ($verification.ExitCode -ne 0) {
        $detail = Get-Content -LiteralPath $verifyError -Encoding utf8 -Raw
        throw "Embedded installer payload validation failed: $detail"
    }
    Write-Output "Installer: $installer"
} finally {
    Pop-Location
}
