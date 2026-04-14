# wrkmon-go uninstaller for Windows
$ErrorActionPreference = "Stop"
$InstallDir = "$env:LOCALAPPDATA\wrkmon-go"

function Write-Info($msg) { Write-Host "[*] $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "[+] $msg" -ForegroundColor Green }

Write-Host ""
Write-Info "Uninstalling wrkmon-go..."

# Remove wrkmon-go.exe + bundled yt-dlp.exe + portable mpv/ (if present).
foreach ($f in @("wrkmon-go.exe", "yt-dlp.exe")) {
    $p = Join-Path $InstallDir $f
    if (Test-Path $p) { Remove-Item $p -Force; Write-Ok "Removed $p" }
}
$mpvDir = Join-Path $InstallDir "mpv"
if (Test-Path $mpvDir) { Remove-Item $mpvDir -Recurse -Force; Write-Ok "Removed $mpvDir" }

# Strip InstallDir and $InstallDir\mpv from user PATH using registry-direct write
# (preserves REG_EXPAND_SZ — avoids dotnet/runtime#1442 PATH corruption).
$key = "HKCU:\Environment"
$cur = (Get-ItemProperty -Path $key -Name Path -ErrorAction SilentlyContinue).Path
if ($cur) {
    $parts = @($cur -split ";" | Where-Object { $_ -ne "" -and $_ -ne $InstallDir -and $_ -ne $mpvDir })
    $new   = $parts -join ";"
    if ($new -ne $cur) {
        Set-ItemProperty -Path $key -Name Path -Value $new -Type ExpandString
        Write-Ok "Removed wrkmon-go entries from PATH"
    }
}

# Drop empty install dir.
if ((Test-Path $InstallDir) -and ((Get-ChildItem $InstallDir -Force).Count -eq 0)) {
    Remove-Item $InstallDir -Force
}

# Ask about config/data.
Write-Host ""
$ConfigDir = "$env:APPDATA\wrkmon-go"
$DataDir   = "$env:LOCALAPPDATA\wrkmon-go"
$reply = Read-Host "[*] Remove config and data? ($ConfigDir) [y/N]"
if ($reply -eq "y" -or $reply -eq "Y") {
    if (Test-Path $ConfigDir) { Remove-Item $ConfigDir -Recurse -Force }
    if (Test-Path $DataDir)   { Remove-Item $DataDir -Recurse -Force }
    Write-Ok "Removed config and data"
}

Write-Host ""
Write-Ok "wrkmon-go uninstalled."
Write-Host ""
