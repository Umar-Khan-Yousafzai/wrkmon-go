# wrkmon-go

A blazing fast TUI YouTube audio player built with Go and Bubble Tea. Search YouTube, build queues, and play audio — all from your terminal.

> v2.0 — Go rewrite of [wrkmon](https://github.com/Umar-Khan-Yousafzai/Wrkmon-TUI-Youtube)

## Features

- **Search & queue** — search YouTube, queue results, play/pause/seek/skip, history and playlists.
- **Downloads & lyrics** — save tracks as MP3, fetch lyrics for the current track.
- **18-band equalizer** — presets (`bass`, `flat`, `pop`, `rock`, `treble`, `vocal`) or per-band custom gains, applied live via mpv, persisted across restarts. See `/eq` below.
- **Focus mode** — `/focus` swaps the whole screen for a fake htop/build-log/test-runner overlay (v1-style disguise); music keeps playing underneath, any key brings the player back.
- **Media keys** — hardware Play/Pause/Next/Prev control the player. Full support on Linux (MPRIS, works with `playerctl` and desktop widgets); Windows registers the media hotkeys (play/pause/next/prev); macOS gets play/pause via mpv's own media-key passthrough. Opt out with `media_keys = false` in config.
- **`--version`** — `wrkmon-go --version` (or `-v` / `version`) prints the build version and exits immediately.

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
| `/lyrics` | Show lyrics for the current track |
| `/download [list]` | Download current track as MP3, or list downloads |
| `/eq [preset\|off\|show\|band <1-18> <dB>]` | Equalizer: apply a preset, disable, show status, or set one band's gain |
| `/focus` | Fake work-screen overlay; any key returns |
| `/help` | Show help |

### Themes

Three built-in themes: `opencode-mono` (default), `github-dark`, `warm-minimal`.

Switch with `/theme <name>`.

### Equalizer

18-band EQ backed by mpv's `superequalizer` audio filter:

```
/eq bass            # apply the bass preset (also: flat, pop, rock, treble, vocal)
/eq show             # show the active preset (or custom bands) and enabled state
/eq band 3 4.5       # set band 3 to +4.5 dB — switches to "custom"
/eq off              # disable EQ processing (settings are kept, not cleared)
```

Settings persist across restarts (`eq_preset`, `eq_gains`, `eq_enabled` in the config file).

### Focus Mode

`/focus` replaces the whole screen with a fake dev-tool overlay (a process monitor, a build log, or a test runner — picked at random), v1-style. Music keeps playing behind it. Press any key (other than Ctrl+C, which still quits) to return to the player — the overlay itself carries no track info, so it can't be dismissed by typing another `/focus`.

### Media Keys

Hardware media keys drive playback when available:

| Platform | Support |
|----------|---------|
| Linux | Full — MPRIS over D-Bus (`org.mpris.MediaPlayer2.wrkmon_go`); works with `playerctl`, GNOME/KDE media widgets, etc. |
| Windows | Play/Pause/Next/Prev via `RegisterHotKey`; no now-playing metadata surface (no SMTC). |
| macOS | Play/Pause only, via mpv's own `--input-media-keys=yes`; Next/Prev are not queue-aware on this platform. |

Set `media_keys = false` in the config file to opt out.

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
