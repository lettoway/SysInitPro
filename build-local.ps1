param([switch]$SkipLinux)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $root) { $root = Get-Location }
$distDir = Join-Path $root "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null

Write-Host "=== [1/3] 校验工程文件与 SHA-256 完整性 ===" -ForegroundColor Cyan
$manifestFile = Join-Path $root "sysinit_manifest.sha256"
if (Test-Path $manifestFile) {
    foreach ($line in (Get-Content $manifestFile -Encoding UTF8)) {
        if ($line -match '^([a-fA-F0-9]{64})\s+\*?(.+)$') {
            $expected = $matches[1].ToLower()
            $rel = $matches[2].Trim()
            $target = Join-Path $root $rel
            if (-not (Test-Path $target)) { throw "缺少文件: $rel" }
            $actual = (Get-FileHash $target -Algorithm SHA256).Hash.ToLower()
            if ($actual -ne $expected) { throw "文件校验不通过: $rel" }
        }
    }
    Write-Host "[OK] SHA-256 完整性校验通过" -ForegroundColor Green
}

$goBin = Join-Path (go env GOPATH) "bin"
if ($env:Path -notlike "*$goBin*") { $env:Path += ";$goBin" }

Write-Host "=== [2/3] 编译 Windows (amd64) ===" -ForegroundColor Cyan
go mod tidy
wails build -platform windows/amd64 -webview2 download
if ($LASTEXITCODE -ne 0) { throw 'Wails Windows 编译失败' }
Stop-Process -Name "SysInitPro-Windows-amd64" -Force -ErrorAction SilentlyContinue
Copy-Item "build/bin/SysInitPro.exe" (Join-Path $distDir "SysInitPro-Windows-amd64.exe") -Force
Write-Host "[OK] Windows 编译完成 -> dist\SysInitPro-Windows-amd64.exe" -ForegroundColor Green

if ($SkipLinux) {
    Write-Host "[SKIP] 已跳过 Linux 编译 (-SkipLinux)" -ForegroundColor Yellow
    exit 0
}

Write-Host "=== [3/3] 通过 Docker 编译 Linux (amd64) ===" -ForegroundColor Cyan
$linuxCmd = 'set -e; apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y golang-go git build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev; export PATH=$PATH:$(go env GOPATH)/bin; go env -w GOPROXY=https://goproxy.cn,direct; go install github.com/wailsapp/wails/v2/cmd/wails@latest; wails build -platform linux/amd64 -tags webkit2_41; cp build/bin/SysInitPro /src/dist/SysInitPro-Linux-amd64'
docker run --rm -v "${root}:/src" -w /src ubuntu:24.04 bash -c $linuxCmd
Write-Host "[OK] Linux 编译完成 -> dist\SysInitPro-Linux-amd64" -ForegroundColor Green