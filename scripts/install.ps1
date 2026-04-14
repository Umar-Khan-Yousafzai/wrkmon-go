# wrkmon-go installer for Windows — fully automatic, no user intervention needed.
# Usage: irm https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

# Win 11 24H2 .NET Framework TLS regression workaround; no-op on patched systems.
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}

$Repo       = "Umar-Khan-Yousafzai/wrkmon-go"
$Binary     = "wrkmon-go.exe"
$InstallDir = "$env:LOCALAPPDATA\wrkmon-go"

# Detect whether we were launched via `irm | iex` — if so, use `break` not `exit`
# so the user's terminal isn't nuked on a fatal error.
$IsIex = $MyInvocation.MyCommand.Path -eq $null

function Write-Info($msg) { Write-Host "[*] $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "[+] $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "[!] $msg" -ForegroundColor Yellow }
function Write-Fail($msg) {
    Write-Host "[x] $msg" -ForegroundColor Red
    if ($IsIex) { $Global:LASTEXITCODE = 1; break } else { exit 1 }
}

# --- ExecutionPolicy guard (Scoop pattern: abort with guidance, don't override GP) ---
$policy = Get-ExecutionPolicy
if ($policy -notin @('Unrestricted','RemoteSigned','Bypass')) {
    Write-Fail "PowerShell execution policy is '$policy'. Run: Set-ExecutionPolicy RemoteSigned -Scope CurrentUser"
}

Write-Host ""
Write-Host "              _                          " -ForegroundColor Cyan
Write-Host " __      __ _ | | __ _ __   ___  _ __    " -ForegroundColor Cyan
Write-Host " \ \ /\ / /| '__|| |/ /| '_ \ / _ \| '_ \   " -ForegroundColor Cyan
Write-Host "  \ V  V / | |   |   < | | | | (_) | | | |  " -ForegroundColor Cyan
Write-Host "   \_/\_/  |_|   |_|\_\|_| |_|\___/|_| |_|  " -ForegroundColor Cyan
Write-Host ""
Write-Host "  YouTube TUI Player — Windows Installer (auto-install)"
Write-Host ""

# --- Helpers ---
function Invoke-Download {
    param([string]$Url, [string]$OutFile, [string]$Label)
    $max = 3
    for ($i = 1; $i -le $max; $i++) {
        try {
            Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -UserAgent "wrkmon-installer"
            return
        } catch {
            if ($i -eq $max) { Write-Fail "$Label download failed after $max attempts: $($_.Exception.Message)" }
            $wait = [Math]::Pow(2, $i)
            Write-Warn "$Label download attempt $i failed; retrying in ${wait}s..."
            Start-Sleep -Seconds $wait
        }
    }
}

function Add-UserPath {
    # Safe append that preserves REG_EXPAND_SZ (avoids dotnet/runtime#1442 PATH corruption).
    param([string]$Dir)
    $key  = "HKCU:\Environment"
    $cur  = (Get-ItemProperty -Path $key -Name Path -ErrorAction SilentlyContinue).Path
    if (-not $cur) { $cur = "" }
    $parts = @($cur -split ";" | Where-Object { $_ -ne "" })
    if ($parts -notcontains $Dir) {
        $new = (($parts + $Dir) -join ";")
        # Keep type as ExpandString so %VAR% references survive.
        Set-ItemProperty -Path $key -Name Path -Value $new -Type ExpandString
        $env:Path = "$env:Path;$Dir"
        return $true
    }
    return $false
}

function Broadcast-EnvChange {
    # Tell running processes (Explorer, new shells) that Environment changed — no logoff needed.
    Add-Type -Namespace Win32 -Name NativeMethods -ErrorAction SilentlyContinue -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    $HWND_BROADCAST    = [IntPtr]0xffff
    $WM_SETTINGCHANGE  = 0x001A
    $SMTO_ABORTIFHUNG  = 2
    $result = [UIntPtr]::Zero
    [void][Win32.NativeMethods]::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, "Environment", $SMTO_ABORTIFHUNG, 5000, [ref]$result)
}

# --- Detect architecture ---
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
Write-Info "Detected: windows/$Arch"

# --- Ensure install dir ---
if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

# --- 1. Download wrkmon-go.exe ---
Write-Info "Fetching latest wrkmon-go release..."
try {
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "wrkmon-installer" }
    $tag = $rel.tag_name
} catch {
    Write-Fail "Could not fetch latest release. Check https://github.com/$Repo/releases"
}
Write-Ok "Latest version: $tag"

$asset = "wrkmon-go-windows-$Arch.exe"
$dlUrl = "https://github.com/$Repo/releases/download/$tag/$asset"
$tmp   = [System.IO.Path]::GetTempFileName() + ".exe"

Write-Info "Downloading $asset..."
Invoke-Download -Url $dlUrl -OutFile $tmp -Label $asset
Move-Item -Path $tmp -Destination "$InstallDir\$Binary" -Force
# Strip Mark of the Web so SmartScreen doesn't flash-close the binary.
Unblock-File -Path "$InstallDir\$Binary" -ErrorAction SilentlyContinue
Write-Ok "Installed: $InstallDir\$Binary"

# --- 2. Provision yt-dlp.exe (direct download, bypasses winget bugs) ---
# This lands next to wrkmon-go.exe so wrkmon-go's tier-2 bundled locator picks it up automatically.
$ytDlpExe = "$InstallDir\yt-dlp.exe"
$ytDlpAsset = switch ($Arch) {
    "arm64" { "yt-dlp_arm64.exe" }
    default { "yt-dlp.exe" }
}
$ytDlpUrl = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$ytDlpAsset"

Write-Info "Downloading yt-dlp ($ytDlpAsset, standalone PyInstaller)..."
$ytDlpTmp = [System.IO.Path]::GetTempFileName() + ".exe"
Invoke-Download -Url $ytDlpUrl -OutFile $ytDlpTmp -Label "yt-dlp"
Move-Item -Path $ytDlpTmp -Destination $ytDlpExe -Force
Unblock-File -Path $ytDlpExe -ErrorAction SilentlyContinue
Write-Ok "Installed: $ytDlpExe"

# --- 3. Provision mpv ---
$mpvDir = "$InstallDir\mpv"
$mpvExe = "$mpvDir\mpv.exe"

function Install-MpvViaWinget {
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { return $false }
    Write-Info "Installing mpv via winget (shinchiro.mpv)..."
    try {
        & winget install --id shinchiro.mpv --exact --silent --accept-package-agreements --accept-source-agreements --disable-interactivity 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq -1978335189) {
            # -1978335189 = APPINSTALLER_CLI_ERROR_UPDATE_NOT_APPLICABLE (already installed)
            Start-Sleep -Seconds 2
            if (Get-Command mpv -ErrorAction SilentlyContinue) { return $true }
        }
    } catch {}
    return $false
}

function Install-MpvPortable {
    # Fallback: download shinchiro 7z + bootstrap 7zr.exe to extract.
    Write-Info "Installing mpv as portable build (no winget)..."
    if (-not (Test-Path $mpvDir)) { New-Item -ItemType Directory -Path $mpvDir -Force | Out-Null }

    $mpvArch = switch ($Arch) {
        "arm64" { "aarch64" }
        default { "x86_64" }
    }

    Write-Info "Querying shinchiro latest release..."
    try {
        $mpvRel = Invoke-RestMethod -Uri "https://api.github.com/repos/shinchiro/mpv-winbuild-cmake/releases/latest" -Headers @{ "User-Agent" = "wrkmon-installer" }
    } catch {
        Write-Fail "Could not query mpv release list: $($_.Exception.Message)"
    }
    $mpvAsset = $mpvRel.assets | Where-Object { $_.name -match "^mpv-$mpvArch-\d{8}-git-[a-f0-9]+\.7z$" } | Select-Object -First 1
    if (-not $mpvAsset) { Write-Fail "No matching mpv asset found for $mpvArch" }

    $mpv7z  = [System.IO.Path]::Combine($env:TEMP, $mpvAsset.name)
    Write-Info "Downloading $($mpvAsset.name) ($([Math]::Round($mpvAsset.size / 1MB, 1)) MB)..."
    Invoke-Download -Url $mpvAsset.browser_download_url -OutFile $mpv7z -Label "mpv"

    # Bootstrap 7zr.exe (standalone 7-Zip extractor, ~500 KB) to unpack the .7z.
    $sevenZr = "$env:TEMP\wrkmon-7zr.exe"
    if (-not (Test-Path $sevenZr)) {
        Write-Info "Downloading 7zr.exe (required for .7z extraction)..."
        Invoke-Download -Url "https://www.7-zip.org/a/7zr.exe" -OutFile $sevenZr -Label "7zr"
    }

    Write-Info "Extracting mpv to $mpvDir..."
    & $sevenZr x $mpv7z "-o$mpvDir" -y | Out-Null
    if ($LASTEXITCODE -ne 0) { Write-Fail "7zr extraction failed (exit $LASTEXITCODE)" }
    if (-not (Test-Path $mpvExe)) { Write-Fail "mpv.exe not found after extraction" }

    Unblock-File -Path $mpvExe -ErrorAction SilentlyContinue
    Remove-Item $mpv7z -ErrorAction SilentlyContinue
    return $true
}

$mpvReady = $false
if (Get-Command mpv -ErrorAction SilentlyContinue) {
    Write-Ok "mpv already on PATH: $((Get-Command mpv).Source)"
    $mpvReady = $true
} elseif (Install-MpvViaWinget) {
    Write-Ok "mpv installed via winget"
    $mpvReady = $true
} elseif (Install-MpvPortable) {
    Write-Ok "mpv installed: $mpvExe"
    # Add portable mpv dir to PATH so `mpv.exe` resolves for wrkmon-go's subprocess.
    if (Add-UserPath -Dir $mpvDir) { Write-Ok "Added $mpvDir to PATH" }
    $mpvReady = $true
}
if (-not $mpvReady) { Write-Fail "mpv installation failed" }

# --- 4. PATH + environment broadcast ---
if (Add-UserPath -Dir $InstallDir) {
    Write-Ok "Added $InstallDir to PATH"
} else {
    Write-Ok "Already in PATH"
}
Broadcast-EnvChange
Write-Ok "Broadcast WM_SETTINGCHANGE (new terminals will pick up PATH)"

# --- 5. Done ---
Write-Host ""
Write-Ok "All done. Run: wrkmon-go"
Write-Host ""
Write-Host "  Binary:  $InstallDir\$Binary" -ForegroundColor DarkGray
Write-Host "  yt-dlp:  $ytDlpExe" -ForegroundColor DarkGray
Write-Host "  mpv:     $(if (Test-Path $mpvExe) { $mpvExe } else { '(via winget)' })" -ForegroundColor DarkGray
Write-Host ""
