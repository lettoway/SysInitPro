$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $projectRoot) { $projectRoot = Get-Location }

$lines = @("# SysInitPro v9.0 SHA-256 Integrity Manifest")
$files = Get-ChildItem -Path $projectRoot -Recurse -File | Where-Object {
    $_.FullName -notmatch '\\dist\\' -and
    $_.FullName -notmatch '\\build\\' -and
    $_.FullName -notmatch '\\\.git\\' -and
    $_.Name -ne "sysinit_manifest.sha256"
} | Sort-Object FullName

foreach ($f in $files) {
    $rel = $f.FullName.Substring($projectRoot.Length + 1).Replace("\", "/")
    $hash = (Get-FileHash -Path $f.FullName -Algorithm SHA256).Hash.ToLower()
    $lines += "$hash  $rel"
}

$manifestPath = Join-Path $projectRoot "sysinit_manifest.sha256"
[System.IO.File]::WriteAllText($manifestPath, ($lines -join "`r`n"), [System.Text.UTF8Encoding]::new($false))
Write-Host "[OK] 已刷新 sysinit_manifest.sha256（共 $($files.Count) 个文件）" -ForegroundColor Green