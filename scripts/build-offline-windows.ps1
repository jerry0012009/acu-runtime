param([Parameter(Mandatory=$true)][string]$PackageDirectory)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$packageRoot = (Resolve-Path -LiteralPath $PackageDirectory).Path
Push-Location $root
try {
    $app = Join-Path $root 'dist/windows-amd64/ACU Runtime.exe'
    $setup = Join-Path $root 'dist/windows-amd64/ACU Runtime Setup.exe'
    if (!(Test-Path -LiteralPath $app) -or !(Test-Path -LiteralPath $setup)) { throw 'Build Windows installer first.' }
    $manifest = (& $app --agent-packages-json | Out-String | ConvertFrom-Json)
    $version = [regex]::Match((Get-Content update.go -Raw), 'const appVersion = "([^"]+)"').Groups[1].Value
    $bundle = Join-Path $root "dist/offline/ACU-Runtime-$version-windows-amd64-Offline"
    $packages = Join-Path $bundle 'agent-packages'
    New-Item -ItemType Directory -Force -Path $packages | Out-Null
    foreach ($agent in @('workbuddy', 'chatgpt')) {
        $pkg = $manifest.$agent
        $source = Join-Path $packageRoot $pkg.Name
        if (!(Test-Path -LiteralPath $source -PathType Leaf)) { throw "Missing unchanged official package: $($pkg.Name)" }
        if ($pkg.SHA256 -and (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant() -ne $pkg.SHA256) { throw 'WorkBuddy checksum mismatch' }
        $signature = Get-AuthenticodeSignature -LiteralPath $source
        if ($signature.Status -ne 'Valid') { throw "Invalid package signature: $agent" }
        if ($agent -eq 'chatgpt' -and $signature.SignerCertificate.Subject -ne 'CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B') { throw 'Invalid OpenAI publisher' }
        if ($agent -eq 'workbuddy' -and $signature.SignerCertificate.Subject -notmatch 'O=Tencent Technology \(Shenzhen\) Company Limited') { throw 'Invalid Tencent publisher' }
        Copy-Item -LiteralPath $source -Destination (Join-Path $packages $pkg.Name) -Force
    }
    Copy-Item -LiteralPath $setup -Destination (Join-Path $bundle 'ACU Runtime Setup.exe') -Force
    @'
解压整个文件夹，双击 ACU Runtime Setup.exe。
请保持 agent-packages 与安装器同级；安装器会保留原版官方应用包，供 Runtime 内点击安装时离线使用。
Windows 安装 ChatGPT 可能需要当前用户确认管理员权限。无需 Microsoft Store 网页安装器。
安装完成后，连接 ACU（登录或已有 Key），点击安装并配置所需 Agent。
没有 ChatGPT 账号时自动通过 ACU 内核使用 API 模式；云端账号功能仍需官方登录。
WorkBuddy 是否要求账号由官方客户端决定。正常模型推理和在线工具仍需网络。
所有第三方包保持原样，执行前还会验证摘要/官方签名；不关闭系统安全设置。
'@ | Set-Content -LiteralPath (Join-Path $bundle '使用说明.txt') -Encoding utf8
    $zip = "$bundle.zip"
    Compress-Archive -LiteralPath $bundle -DestinationPath $zip -CompressionLevel Fastest -Force
    Write-Output "Offline bundle: $zip"
} finally { Pop-Location }
