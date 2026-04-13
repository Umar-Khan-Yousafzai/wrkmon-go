# wrkmon-go uninstaller for Windows
$ErrorActionPreference = "Stop"
$InstallDir = "$env:LOCALAPPDATA\wrkmon-go"

Write-Host ""
Write-Host "[*] Uninstalling wrkmon-go..." -ForegroundColor Cyan

# Remove binary
if (Test-Path "$InstallDir\wrkmon-go.exe") {
    Remove-Item "$InstallDir\wrkmon-go.exe" -Force
    Write-Host "[+] Removed $InstallDir\wrkmon-go.exe" -ForegroundColor Green
}

# Remove from PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -like "*$InstallDir*") {
    $NewPath = ($UserPath.Split(";") | Where-Object { $_ -ne $InstallDir }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    Write-Host "[+] Removed from PATH" -ForegroundColor Green
}

# Remove install dir if empty
if ((Test-Path $InstallDir) -and ((Get-ChildItem $InstallDir).Count -eq 0)) {
    Remove-Item $InstallDir -Force
}

# Ask about config/data
Write-Host ""
$ConfigDir = "$env:APPDATA\wrkmon-go"
$DataDir = "$env:LOCALAPPDATA\wrkmon-go"
$reply = Read-Host "[*] Remove config and data? ($ConfigDir) [y/N]"
if ($reply -eq "y" -or $reply -eq "Y") {
    if (Test-Path $ConfigDir) { Remove-Item $ConfigDir -Recurse -Force }
    if (Test-Path $DataDir) { Remove-Item $DataDir -Recurse -Force }
    Write-Host "[+] Removed config and data" -ForegroundColor Green
}

Write-Host ""
Write-Host "[+] wrkmon-go uninstalled." -ForegroundColor Green
Write-Host ""
