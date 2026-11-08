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
    & $GoExecutable build -tags setup -ldflags '-H windowsgui' -o $installer ./cmd/setup
    if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
    Write-Output "Installer: $installer"
} finally {
    Pop-Location
}
