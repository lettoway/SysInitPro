# build-local.ps1 - SysInitPro v9.0 本地双端编译脚本 (自动补齐 Winget / Go / Wails / Windows虚拟化 / WSL 2 / Docker)
$ErrorActionPreference = "Stop"
$distDir = Join-Path (Get-Location) "dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null

function Refresh-EnvPath {
    $machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
    $localApp = [System.Environment]::GetEnvironmentVariable("LOCALAPPDATA", "User")
    if (-not $localApp) { $localApp = $env:LOCALAPPDATA }
    $wingetDir = Join-Path $localApp "Microsoft\WindowsApps"
    $env:Path = "$machinePath;$userPath;$wingetDir"
}

Write-Host "=== [0/4] 校验工程文件与 SHA-256 完整性 ===" -ForegroundColor Cyan
$manifestFile = Join-Path (Get-Location) "sysinit_manifest.sha256"
if (Test-Path $manifestFile) {
    Get-Content $manifestFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -and -not $line.StartsWith("#")) {
            $parts = $line -split "\s+", 2
            if ($parts.Count -eq 2) {
                $expected = $parts[0].ToLower()
                $targetFile = Join-Path (Get-Location) $parts[1]
                if (-not (Test-Path $targetFile)) {
                    Write-Host "[FAIL] 缺少文件: $($parts[1])" -ForegroundColor Red
                    exit 1
                }
                $actual = (Get-FileHash -Path $targetFile -Algorithm SHA256).Hash.ToLower()
                if ($actual -ne $expected) {
                    Write-Host "[FAIL] 哈希校验失败: $($parts[1])" -ForegroundColor Red
                    exit 1
                }
            }
        }
    }
    Write-Host "[OK] SHA-256 完整性校验通过" -ForegroundColor Green
}

Write-Host "=== [1/4] 检查 Winget 包管理器 ===" -ForegroundColor Cyan
if (-not (Get-Command "winget" -ErrorAction SilentlyContinue)) {
    Write-Host ">>> 未检测到 winget，正在从微软官方源自动下载并安装..." -ForegroundColor Yellow
    $ProgressPreference = 'SilentlyContinue'
    $vcLibsPath = Join-Path $env:TEMP "Microsoft.VCLibs.x64.appx"
    $wingetPath = Join-Path $env:TEMP "Microsoft.DesktopAppInstaller.msixbundle"
    try {
        Invoke-WebRequest -Uri "https://aka.ms/Microsoft.VCLibs.x64.14.00.Desktop.appx" -OutFile $vcLibsPath
        Invoke-WebRequest -Uri "https://aka.ms/getwinget" -OutFile $wingetPath
        Add-AppxPackage -Path $vcLibsPath -ErrorAction SilentlyContinue
        Add-AppxPackage -Path $wingetPath
        Refresh-EnvPath
        Write-Host "[OK] winget 安装完成" -ForegroundColor Green
    } catch {
        Write-Host "[错误] 自动安装 winget 失败: $_" -ForegroundColor Red
        exit 1
    } finally {
        Remove-Item $vcLibsPath, $wingetPath -Force -ErrorAction SilentlyContinue
    }
}

Write-Host "=== [2/4] 检查本机编译工具链 (Go & Wails) ===" -ForegroundColor Cyan
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host ">>> 未检测到 Go，正在通过 winget 自动静默安装 GoLang.Go..." -ForegroundColor Yellow
    winget install --id GoLang.Go --silent --accept-package-agreements --accept-source-agreements
    Refresh-EnvPath
    if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
        $env:Path += ";C:\Program Files\Go\bin"
    }
}

$goBin = Join-Path (go env GOPATH) "bin"
if ($env:Path -notlike "*$goBin*") { $env:Path += ";$goBin" }

if (-not (Get-Command "wails" -ErrorAction SilentlyContinue)) {
    Write-Host ">>> 未检测到 Wails CLI，正在自动安装..." -ForegroundColor Yellow
    go env -w GOPROXY=https://goproxy.cn,direct
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
}

Write-Host "=== [3/4] 开始本机原生编译 Windows (amd64) ===" -ForegroundColor Cyan
if (-not (Test-Path "go.mod")) { go mod init SysInitPro }
go get github.com/wailsapp/wails/v2@latest
go get golang.org/x/crypto/ssh@latest
go mod tidy

New-Item -ItemType Directory -Force -Path "build/windows" | Out-Null
$manifestXml = '<?xml version="1.0" encoding="UTF-8" standalone="yes"?><assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0"><trustInfo xmlns="urn:schemas-microsoft-com:asm.v3"><security><requestedPrivileges><requestedExecutionLevel level="requireAdministrator" uiAccess="false"/></requestedPrivileges></security></trustInfo></assembly>'
[System.IO.File]::WriteAllText((Join-Path (Get-Location) "build/windows/wails.exe.manifest"), $manifestXml, [System.Text.UTF8Encoding]::new($false))

wails build -platform windows/amd64 -webview2 download
Copy-Item "build/bin/SysInitPro.exe" (Join-Path $distDir "SysInitPro-Windows-amd64.exe") -Force
Write-Host "[OK] Windows 版本编译完成 -> dist\SysInitPro-Windows-amd64.exe" -ForegroundColor Green

Write-Host "=== [4/4] 检查 Windows 虚拟化功能、WSL 2、Docker 并编译 Linux (amd64) ===" -ForegroundColor Cyan
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
$requiredFeatures = @("Microsoft-Windows-Subsystem-Linux", "VirtualMachinePlatform")
$needReboot = $false

foreach ($feat in $requiredFeatures) {
    $state = (Get-WindowsOptionalFeature -Online -FeatureName $feat -ErrorAction SilentlyContinue).State
    if ($state -ne "Enabled") {
        if (-not $isAdmin) {
            Write-Host "[错误] Windows 功能 [$feat] 未开启，请以管理员身份重新运行 PowerShell 以自动开启。" -ForegroundColor Red
            exit 1
        }
        Write-Host ">>> 检测到 Windows 功能 [$feat] 未开启，正在自动启用..." -ForegroundColor Yellow
        dism.exe /online /enable-feature /featurename:$feat /all /norestart | Out-Null
        if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 3010) {
            Write-Host "[OK] 已启用 [$feat]" -ForegroundColor Green
            $needReboot = $true
        } else {
            Write-Host "[错误] 启用 [$feat] 失败 (退出码: $LASTEXITCODE)" -ForegroundColor Red
            exit 1
        }
    }
}

$wslReady = $false
if (Get-Command "wsl" -ErrorAction SilentlyContinue) {
    wsl --status *> $null
    if ($LASTEXITCODE -eq 0) { $wslReady = $true }
}

if (-not $wslReady) {
    Write-Host ">>> 正在下载并安装 WSL 2 内核组件..." -ForegroundColor Yellow
    try {
        wsl --install --no-distribution
        $needReboot = $true
    } catch {
        Write-Host "[错误] 自动安装 WSL 2 失败（请确认以管理员身份运行 PowerShell）: $_" -ForegroundColor Red
        exit 1
    }
}

if ($needReboot) {
    Write-Host "[提示] 已启用 WSL 2 / 虚拟机平台功能，必须重启电脑后才能启动 Docker Desktop！" -ForegroundColor Yellow
    Write-Host "[OK] Windows 产物已就绪: $distDir\SysInitPro-Windows-amd64.exe" -ForegroundColor Green
    Write-Host "请重启电脑后，再次运行 .\build-local.ps1 完成 Linux 版本编译。" -ForegroundColor Cyan
    exit 0
}

if (-not (Get-Command "docker" -ErrorAction SilentlyContinue)) {
    Write-Host ">>> 未检测到 Docker，正在通过 winget 自动静默安装 Docker Desktop..." -ForegroundColor Yellow
    winget install --id Docker.DockerDesktop --silent --accept-package-agreements --accept-source-agreements
    Refresh-EnvPath
    $dockerExe = "$env:ProgramFiles\Docker\Docker\Docker Desktop.exe"
    if (Test-Path $dockerExe) {
        Start-Process $dockerExe
        Write-Host ">>> 正在等待 Docker 引擎启动..." -ForegroundColor Yellow
        Start-Sleep -Seconds 15
    }
}

$dockerReady = $false
try {
    docker info *> $null
    if ($LASTEXITCODE -eq 0) { $dockerReady = $true }
} catch {}

if (-not $dockerReady) {
    Write-Host "[提示] Docker 引擎尚未就绪，请先手动启动 Docker Desktop 后重新运行本脚本。" -ForegroundColor Yellow
    Write-Host "[OK] Windows 产物已就绪: $distDir\SysInitPro-Windows-amd64.exe" -ForegroundColor Green
    exit 0
}

$workDir = (Get-Location).Path
$linuxCmd = 'set -e; apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y golang-go git build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev; export PATH=$PATH:$(go env GOPATH)/bin; go env -w GOPROXY=https://goproxy.cn,direct; go install github.com/wailsapp/wails/v2/cmd/wails@latest; wails build -platform linux/amd64 -tags webkit2_41; cp build/bin/SysInitPro /src/dist/SysInitPro-Linux-amd64'
docker run --rm -v "${workDir}:/src" -w /src ubuntu:24.04 bash -c $linuxCmd

Write-Host "[OK] Linux 版本编译完成 -> dist\SysInitPro-Linux-amd64" -ForegroundColor Green
Write-Host "[OK] 本地双端编译全部完成！产物目录: $distDir" -ForegroundColor Green