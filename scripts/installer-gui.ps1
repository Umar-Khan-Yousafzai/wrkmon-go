# wrkmon-go GUI installer (Windows, WinForms — no compilation required)
# Launch:
#   powershell -ExecutionPolicy Bypass -File installer-gui.ps1
# Or from irm | iex wrapper that saves + runs via Start-Process with -WindowStyle Hidden.

$ErrorActionPreference = "Stop"
try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

# ---------------------------- config ----------------------------
$Repo       = "Umar-Khan-Yousafzai/wrkmon-go"
$Binary     = "wrkmon-go.exe"
$InstallDir = "$env:LOCALAPPDATA\wrkmon-go"
$MpvDir     = Join-Path $InstallDir "mpv"

# ---------------------------- window ----------------------------
$form = New-Object System.Windows.Forms.Form
$form.Text = "wrkmon-go Setup"
$form.Size = New-Object System.Drawing.Size(560, 420)
$form.StartPosition = "CenterScreen"
$form.FormBorderStyle = "FixedDialog"
$form.MaximizeBox = $false
$form.MinimizeBox = $true
$form.BackColor = [System.Drawing.Color]::FromArgb(245, 247, 250)
$form.Font = New-Object System.Drawing.Font("Segoe UI", 9)

# Header band
$header = New-Object System.Windows.Forms.Panel
$header.Size = New-Object System.Drawing.Size(560, 72)
$header.Location = New-Object System.Drawing.Point(0, 0)
$header.BackColor = [System.Drawing.Color]::FromArgb(20, 27, 45)
$form.Controls.Add($header)

$title = New-Object System.Windows.Forms.Label
$title.Text = "wrkmon-go"
$title.ForeColor = [System.Drawing.Color]::White
$title.Font = New-Object System.Drawing.Font("Segoe UI Semibold", 16)
$title.AutoSize = $true
$title.Location = New-Object System.Drawing.Point(20, 10)
$header.Controls.Add($title)

$subtitle = New-Object System.Windows.Forms.Label
$subtitle.Text = "YouTube TUI Player — one-click installer"
$subtitle.ForeColor = [System.Drawing.Color]::FromArgb(180, 190, 210)
$subtitle.AutoSize = $true
$subtitle.Location = New-Object System.Drawing.Point(22, 42)
$header.Controls.Add($subtitle)

# Body
$body = New-Object System.Windows.Forms.Label
$body.Location = New-Object System.Drawing.Point(20, 88)
$body.Size = New-Object System.Drawing.Size(520, 56)
$body.Text = "This installer will download and configure wrkmon-go plus its dependencies (yt-dlp, mpv). Nothing else to install manually.`r`n`r`nDestination: $InstallDir"
$form.Controls.Add($body)

# Status label
$status = New-Object System.Windows.Forms.Label
$status.Location = New-Object System.Drawing.Point(20, 160)
$status.Size = New-Object System.Drawing.Size(520, 20)
$status.Text = "Ready"
$status.ForeColor = [System.Drawing.Color]::FromArgb(80, 90, 110)
$form.Controls.Add($status)

# Progress bar
$progress = New-Object System.Windows.Forms.ProgressBar
$progress.Location = New-Object System.Drawing.Point(20, 184)
$progress.Size = New-Object System.Drawing.Size(520, 14)
$progress.Minimum = 0
$progress.Maximum = 100
$progress.Value = 0
$form.Controls.Add($progress)

# Log textbox
$log = New-Object System.Windows.Forms.TextBox
$log.Multiline = $true
$log.ScrollBars = "Vertical"
$log.ReadOnly = $true
$log.Location = New-Object System.Drawing.Point(20, 212)
$log.Size = New-Object System.Drawing.Size(520, 110)
$log.Font = New-Object System.Drawing.Font("Consolas", 8)
$log.BackColor = [System.Drawing.Color]::White
$form.Controls.Add($log)

# Buttons
$btnInstall = New-Object System.Windows.Forms.Button
$btnInstall.Text = "Install"
$btnInstall.Size = New-Object System.Drawing.Size(100, 30)
$btnInstall.Location = New-Object System.Drawing.Point(330, 340)
$btnInstall.BackColor = [System.Drawing.Color]::FromArgb(38, 110, 235)
$btnInstall.ForeColor = [System.Drawing.Color]::White
$btnInstall.FlatStyle = "Flat"
$btnInstall.FlatAppearance.BorderSize = 0
$form.Controls.Add($btnInstall)

$btnCancel = New-Object System.Windows.Forms.Button
$btnCancel.Text = "Cancel"
$btnCancel.Size = New-Object System.Drawing.Size(100, 30)
$btnCancel.Location = New-Object System.Drawing.Point(440, 340)
$btnCancel.FlatStyle = "Flat"
$btnCancel.BackColor = [System.Drawing.Color]::FromArgb(220, 225, 232)
$form.Controls.Add($btnCancel)

$btnOpen = New-Object System.Windows.Forms.Button
$btnOpen.Text = "Open Folder"
$btnOpen.Size = New-Object System.Drawing.Size(100, 30)
$btnOpen.Location = New-Object System.Drawing.Point(220, 340)
$btnOpen.FlatStyle = "Flat"
$btnOpen.BackColor = [System.Drawing.Color]::FromArgb(220, 225, 232)
$btnOpen.Visible = $false
$form.Controls.Add($btnOpen)

# ---------------------------- helpers ----------------------------
function UI-Log([string]$msg) {
    $ts = (Get-Date).ToString("HH:mm:ss")
    $log.AppendText("[$ts] $msg`r`n")
    [System.Windows.Forms.Application]::DoEvents()
}

function UI-Status([string]$msg, [int]$pct) {
    $status.Text = $msg
    $progress.Value = [Math]::Max(0, [Math]::Min(100, $pct))
    [System.Windows.Forms.Application]::DoEvents()
}

function UI-Fail([string]$msg) {
    UI-Status "Installation failed." 100
    UI-Log "ERROR: $msg"
    $progress.ForeColor = [System.Drawing.Color]::Red
    [System.Windows.Forms.MessageBox]::Show($form, $msg, "wrkmon-go Setup", "OK", "Error") | Out-Null
    $btnInstall.Enabled = $true
    $btnInstall.Text = "Retry"
    $btnCancel.Text = "Close"
}

function Invoke-Download {
    param([string]$Url, [string]$OutFile, [string]$Label)
    for ($i = 1; $i -le 3; $i++) {
        try {
            Invoke-WebRequest -Uri $Url -OutFile $OutFile -UseBasicParsing -UserAgent "wrkmon-installer"
            return
        } catch {
            if ($i -eq 3) { throw "$Label download failed: $($_.Exception.Message)" }
            UI-Log "$Label attempt $i failed — retrying..."
            Start-Sleep -Seconds ([Math]::Pow(2, $i))
        }
    }
}

function Add-UserPath {
    param([string]$Dir)
    $key  = "HKCU:\Environment"
    $cur  = (Get-ItemProperty -Path $key -Name Path -ErrorAction SilentlyContinue).Path
    if (-not $cur) { $cur = "" }
    $parts = @($cur -split ";" | Where-Object { $_ -ne "" })
    if ($parts -notcontains $Dir) {
        Set-ItemProperty -Path $key -Name Path -Value ((($parts + $Dir) -join ";")) -Type ExpandString
        $env:Path = "$env:Path;$Dir"
        return $true
    }
    return $false
}

function Broadcast-EnvChange {
    Add-Type -Namespace Win32Setup -Name Native -ErrorAction SilentlyContinue -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    $result = [UIntPtr]::Zero
    [void][Win32Setup.Native]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, "Environment", 2, 5000, [ref]$result)
}

function Register-Uninstaller {
    param([string]$Version)
    $key = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\wrkmon-go"
    if (-not (Test-Path $key)) { New-Item -Path $key -Force | Out-Null }
    Set-ItemProperty -Path $key -Name "DisplayName"     -Value "wrkmon-go"
    Set-ItemProperty -Path $key -Name "DisplayVersion"  -Value $Version
    Set-ItemProperty -Path $key -Name "Publisher"       -Value "Umar Khan Yousafzai"
    Set-ItemProperty -Path $key -Name "URLInfoAbout"    -Value "https://github.com/$Repo"
    Set-ItemProperty -Path $key -Name "InstallLocation" -Value $InstallDir
    Set-ItemProperty -Path $key -Name "UninstallString" -Value "powershell -ExecutionPolicy Bypass -File `"$InstallDir\uninstall.ps1`""
    Set-ItemProperty -Path $key -Name "NoModify"        -Value 1 -Type DWord
    Set-ItemProperty -Path $key -Name "NoRepair"        -Value 1 -Type DWord
}

function Install-MpvViaWinget {
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { return $false }
    UI-Log "Trying winget install shinchiro.mpv..."
    try {
        & winget install --id shinchiro.mpv --exact --silent --accept-package-agreements --accept-source-agreements --disable-interactivity 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq -1978335189) {
            Start-Sleep -Seconds 2
            if (Get-Command mpv -ErrorAction SilentlyContinue) { return $true }
        }
    } catch {}
    return $false
}

function Install-MpvPortable {
    param([string]$Arch)
    if (-not (Test-Path $MpvDir)) { New-Item -ItemType Directory -Path $MpvDir -Force | Out-Null }
    $mpvArch = if ($Arch -eq "arm64") { "aarch64" } else { "x86_64" }
    UI-Log "Querying shinchiro latest mpv release..."
    $mpvRel = Invoke-RestMethod -Uri "https://api.github.com/repos/shinchiro/mpv-winbuild-cmake/releases/latest" -Headers @{ "User-Agent" = "wrkmon-installer" }
    $mpvAsset = $mpvRel.assets | Where-Object { $_.name -match "^mpv-$mpvArch-\d{8}-git-[a-f0-9]+\.7z$" } | Select-Object -First 1
    if (-not $mpvAsset) { throw "No matching mpv asset for $mpvArch" }
    $mpv7z = Join-Path $env:TEMP $mpvAsset.name
    UI-Log "Downloading $($mpvAsset.name) ($([Math]::Round($mpvAsset.size / 1MB, 1)) MB)..."
    Invoke-Download -Url $mpvAsset.browser_download_url -OutFile $mpv7z -Label "mpv"

    $sevenZr = Join-Path $env:TEMP "wrkmon-7zr.exe"
    if (-not (Test-Path $sevenZr)) {
        UI-Log "Downloading 7zr.exe (required for .7z extraction)..."
        Invoke-Download -Url "https://www.7-zip.org/a/7zr.exe" -OutFile $sevenZr -Label "7zr"
    }
    UI-Log "Extracting mpv to $MpvDir..."
    & $sevenZr x $mpv7z "-o$MpvDir" -y | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "7zr extraction failed (exit $LASTEXITCODE)" }
    if (-not (Test-Path (Join-Path $MpvDir "mpv.exe"))) { throw "mpv.exe not found after extraction" }
    Unblock-File -Path (Join-Path $MpvDir "mpv.exe") -ErrorAction SilentlyContinue
    Remove-Item $mpv7z -ErrorAction SilentlyContinue
    return $true
}

# ---------------------------- run ----------------------------
function Run-Install {
    $btnInstall.Enabled = $false
    $btnInstall.Text = "Installing..."
    $progress.ForeColor = [System.Drawing.Color]::FromArgb(38, 110, 235)

    try {
        UI-Status "Checking prerequisites..." 2
        $policy = Get-ExecutionPolicy
        if ($policy -notin @('Unrestricted','RemoteSigned','Bypass')) {
            throw "PowerShell execution policy is '$policy'. Open an admin PowerShell and run: Set-ExecutionPolicy RemoteSigned -Scope CurrentUser"
        }
        $arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
        UI-Log "Architecture: windows/$arch"

        if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

        # --- Fetch wrkmon-go ---
        UI-Status "Fetching wrkmon-go release info..." 8
        $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "wrkmon-installer" }
        $tag = $rel.tag_name
        UI-Log "Latest wrkmon-go: $tag"

        UI-Status "Downloading wrkmon-go..." 18
        $asset = "wrkmon-go-windows-$arch.exe"
        $tmp = [System.IO.Path]::GetTempFileName() + ".exe"
        Invoke-Download -Url "https://github.com/$Repo/releases/download/$tag/$asset" -OutFile $tmp -Label $asset
        Move-Item -Path $tmp -Destination (Join-Path $InstallDir $Binary) -Force
        Unblock-File -Path (Join-Path $InstallDir $Binary) -ErrorAction SilentlyContinue
        UI-Log "Installed wrkmon-go.exe"

        # --- yt-dlp direct download (bypasses winget bugs) ---
        UI-Status "Downloading yt-dlp..." 40
        $ytAsset = if ($arch -eq "arm64") { "yt-dlp_arm64.exe" } else { "yt-dlp.exe" }
        $ytTmp   = [System.IO.Path]::GetTempFileName() + ".exe"
        Invoke-Download -Url "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$ytAsset" -OutFile $ytTmp -Label "yt-dlp"
        Move-Item -Path $ytTmp -Destination (Join-Path $InstallDir "yt-dlp.exe") -Force
        Unblock-File -Path (Join-Path $InstallDir "yt-dlp.exe") -ErrorAction SilentlyContinue
        UI-Log "Installed yt-dlp.exe"

        # --- mpv ---
        UI-Status "Installing mpv..." 55
        $mpvDone = $false
        if (Get-Command mpv -ErrorAction SilentlyContinue) {
            UI-Log "mpv already on PATH — skipping"
            $mpvDone = $true
        } elseif (Install-MpvViaWinget) {
            UI-Log "mpv installed via winget"
            $mpvDone = $true
        } else {
            UI-Log "winget unavailable or refused — falling back to portable mpv"
            if (Install-MpvPortable -Arch $arch) {
                Add-UserPath -Dir $MpvDir | Out-Null
                UI-Log "Installed portable mpv ($MpvDir)"
                $mpvDone = $true
            }
        }
        if (-not $mpvDone) { throw "Failed to install mpv via any method" }

        # --- PATH + uninstaller ---
        UI-Status "Wiring up PATH + Add/Remove Programs..." 85
        if (Add-UserPath -Dir $InstallDir) { UI-Log "Added $InstallDir to user PATH" }
        Broadcast-EnvChange
        UI-Log "Broadcast WM_SETTINGCHANGE (new shells will see the new PATH)"

        # Copy uninstall.ps1 next to the binary so Add/Remove Programs can invoke it.
        $unSrc = Join-Path $PSScriptRoot "uninstall.ps1"
        if (Test-Path $unSrc) { Copy-Item $unSrc (Join-Path $InstallDir "uninstall.ps1") -Force }
        Register-Uninstaller -Version $tag

        # --- Create Start Menu shortcut ---
        try {
            $startDir = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
            $lnkPath  = Join-Path $startDir "wrkmon-go.lnk"
            $wsh = New-Object -ComObject WScript.Shell
            $sc  = $wsh.CreateShortcut($lnkPath)
            $sc.TargetPath = Join-Path $InstallDir $Binary
            $sc.WorkingDirectory = $InstallDir
            $sc.Description = "wrkmon-go — YouTube TUI Player"
            $sc.Save()
            UI-Log "Added Start Menu shortcut"
        } catch { UI-Log "(shortcut creation skipped: $($_.Exception.Message))" }

        UI-Status "All done." 100
        UI-Log "Installation complete. Run 'wrkmon-go' from any new terminal."
        $btnInstall.Enabled = $true
        $btnInstall.Text = "Launch wrkmon-go"
        $btnInstall.Add_Click({ Start-Process (Join-Path $InstallDir $Binary); $form.Close() }.GetNewClosure())
        $btnCancel.Text = "Close"
        $btnOpen.Visible = $true
    } catch {
        UI-Fail $_.Exception.Message
    }
}

$btnInstall.Add_Click({ Run-Install })
$btnCancel.Add_Click({ $form.Close() })
$btnOpen.Add_Click({ Start-Process explorer.exe $InstallDir })

[void]$form.ShowDialog()
