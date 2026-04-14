# wrkmon-go

A blazing fast TUI YouTube audio player built with Go and Bubble Tea. Search YouTube, build queues, and play audio — all from your terminal.

> v2.0 — Go rewrite of [wrkmon](https://github.com/Umar-Khan-Yousafzai/Wrkmon-TUI-Youtube)

## Install

### Linux (one-liner)
```bash
curl -fsSL https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/install.sh | bash
```

### Linux (.deb — Ubuntu/Debian/Pop!_OS)
```bash
# Download the .deb from the latest release, then:
sudo dpkg -i wrkmon-go_*.deb
sudo apt-get install -f   # auto-installs mpv + yt-dlp
```

### Windows (PowerShell — fully automatic)
```powershell
irm https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/install.ps1 | iex
```
The installer downloads wrkmon-go, yt-dlp.exe, and mpv (via winget or portable .7z) on your behalf — no manual `winget install` steps. Also wires up user PATH and registers the app in Add/Remove Programs.

### Windows (GUI wizard)
Download `wrkmon-go_<version>_windows_amd64.zip` from the [latest release](https://github.com/Umar-Khan-Yousafzai/wrkmon-go/releases/latest), extract, right-click `installer-gui.ps1` → **Run with PowerShell**. Shows a native WinForms wizard with progress bar and live install log.

### macOS
```bash
curl -fsSL https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/install.sh | bash
```

### Requirements

wrkmon-go needs these on your system:

- **[mpv](https://mpv.io)** — audio playback engine
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)** — YouTube stream resolver

The Windows installer provisions both automatically. On Linux/macOS the installer scripts check for these and guide you to the right package manager.

## Usage

```bash
wrkmon-go
```

Type to search YouTube. Use `/help` to see all commands.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Pause / Resume |
| `Left` / `Right` | Seek -/+ 5 seconds |
| `+` / `-` | Volume up / down |
| `n` / `p` | Next / Previous track |
| `a` | Add to queue (search view) |
| `Tab` / `Shift+Tab` | Cycle views |
| `Enter` | Play selected |
| `Esc` | Back to search |
| `q` | Quit |

### Slash Commands

| Command | Description |
|---------|-------------|
| `/search <query>` | Search YouTube |
| `/queue` | Show play queue |
| `/now` | Show now playing |
| `/history` | Show play history |
| `/pause` | Toggle pause |
| `/stop` | Stop playback |
| `/next` | Next track |
| `/prev` | Previous track |
| `/vol <0-100>` | Set volume |
| `/theme [name]` | Switch theme |
| `/clear` | Clear queue |
| `/help` | Show help |

### Themes

Three built-in themes: `opencode-mono` (default), `github-dark`, `warm-minimal`.

Switch with `/theme <name>`.

## Uninstall

### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/uninstall.sh | bash
```

### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/Umar-Khan-Yousafzai/wrkmon-go/main/scripts/uninstall.ps1 | iex
```

## Building from Source

```bash
git clone https://github.com/Umar-Khan-Yousafzai/wrkmon-go.git
cd wrkmon-go
make build
./wrkmon-go
```

## License

MIT
