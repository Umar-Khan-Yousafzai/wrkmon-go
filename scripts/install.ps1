# wrkmon-go installer for Windows
# Usage: irm https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = "Umar-Khan-Yousafzai/wrkmon-go"
$Binary = "wrkmon-go.exe"
$InstallDir = "$env:LOCALAPPDATA\wrkmon-go"

function Write-Info($msg) { Write-Host "[*] $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "[+] $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "[!] $msg" -ForegroundColor Yellow }
function Write-Fail($msg) { Write-Host "[x] $msg" -ForegroundColor Red; exit 1 }

Write-Host ""
Write-Host "              _                          " -ForegroundColor Cyan
Write-Host " __      __ _ | | __ _ __   ___  _ __    " -ForegroundColor Cyan
Write-Host " \ \ /\ / /| '__|| |/ /| '_ \ / _ \| '_ \   " -ForegroundColor Cyan
Write-Host "  \ V  V / | |   |   < | | | | (_) | | | |  " -ForegroundColor Cyan
Write-Host "   \_/\_/  |_|   |_|\_\|_| |_|\___/|_| |_|  " -ForegroundColor Cyan
Write-Host ""
Write-Host "  YouTube TUI Player - Windows Installer"
Write-Host ""

# Detect architecture
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
Write-Info "Detected: windows/$Arch"

# Get latest release
Write-Info "Fetching latest release..."
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "wrkmon-installer" }
    $Tag = $Release.tag_name
} catch {
    Write-Fail "Could not fetch latest release. Check https://github.com/$Repo/releases"
}
Write-Ok "Latest version: $Tag"

# Download binary
$Asset = "wrkmon-go-windows-$Arch.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Tag/$Asset"

Write-Info "Downloading $Asset..."
$TmpFile = [System.IO.Path]::GetTempFileName() + ".exe"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TmpFile -UseBasicParsing
} catch {
    Write-Fail "Download failed. Check https://github.com/$Repo/releases for available binaries."
}
Write-Ok "Downloaded successfully"

# Install binary
Write-Info "Installing to $InstallDir..."
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}
Move-Item -Path $TmpFile -Destination "$InstallDir\$Binary" -Force
Write-Ok "Installed: $InstallDir\$Binary"

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Info "Adding to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Ok "Added $InstallDir to PATH (restart terminal to take effect)"
} else {
    Write-Ok "Already in PATH"
}

# Check dependencies
Write-Host ""
Write-Info "Checking dependencies..."

$Missing = $false

# Check mpv
if (Get-Command mpv -ErrorAction SilentlyContinue) {
    Write-Ok "mpv found: $((Get-Command mpv).Source)"
} else {
    $Missing = $true
    Write-Warn "mpv NOT found"
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Warn "  Install:  winget install mpv"
    } elseif (Get-Command choco -ErrorAction SilentlyContinue) {
        Write-Warn "  Install:  choco install mpv"
    } elseif (Get-Command scoop -ErrorAction SilentlyContinue) {
        Write-Warn "  Install:  scoop install mpv"
    } else {
        Write-Warn "  Install from: https://mpv.io/installation/"
    }
}

# Check yt-dlp
if (Get-Command yt-dlp -ErrorAction SilentlyContinue) {
    Write-Ok "yt-dlp found: $((Get-Command yt-dlp).Source)"
} else {
    $Missing = $true
    Write-Warn "yt-dlp NOT found"
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Warn "  Install:  winget install yt-dlp"
    } elseif (Get-Command pip -ErrorAction SilentlyContinue) {
        Write-Warn "  Install:  pip install yt-dlp"
    } else {
        Write-Warn "  Install from: https://github.com/yt-dlp/yt-dlp#installation"
    }
}

Write-Host ""
if (-not $Missing) {
    Write-Ok "All dependencies satisfied!"
    Write-Host ""
    Write-Host "  Run 'wrkmon-go' to start." -ForegroundColor Green
} else {
    Write-Warn "Install missing dependencies above, then run: wrkmon-go"
}
Write-Host ""
